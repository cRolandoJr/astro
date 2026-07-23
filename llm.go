package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Exchange es un turno de la charla (lo que dijo el usuario y lo que respondió Astro).
type Exchange struct{ User, Assistant string }

// chatFunc habla con el LLM: system + historial de turnos previos + la frase actual → JSON crudo.
type chatFunc func(system string, history []Exchange, user string) (string, error)

// LLMInterpreter elige una acción del registro usando un LLM. Si el LLM falla o devuelve
// algo raro, cae al 'fallback' (el RuleInterpreter). Implementa Interpreter.
type LLMInterpreter struct {
	chat         chatFunc
	actions      map[string]*Action
	fallback     Interpreter
	mem          MemoryStore // opcional (nil = sin memoria persistente)
	topK         int
	now          func() time.Time
	history      []Exchange
	historyTurns int
	idleWindow   time.Duration
	lastTurn     time.Time
}

// LLMConfig junta la config del intérprete (creció más allá de lo que conviene posicional).
type LLMConfig struct {
	Chat         chatFunc
	Actions      map[string]*Action
	Fallback     Interpreter
	Mem          MemoryStore      // opcional (nil = sin memoria persistente)
	TopK         int              // <=0 → 5
	Now          func() time.Time // nil → time.Now
	HistoryTurns int              // <=0 → 6
	IdleWindow   time.Duration    // <=0 → 5 min
}

func NewLLMInterpreter(cfg LLMConfig) *LLMInterpreter {
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HistoryTurns <= 0 {
		cfg.HistoryTurns = 6
	}
	if cfg.IdleWindow <= 0 {
		cfg.IdleWindow = 5 * time.Minute
	}
	return &LLMInterpreter{
		chat: cfg.Chat, actions: cfg.Actions, fallback: cfg.Fallback, mem: cfg.Mem,
		topK: cfg.TopK, now: cfg.Now, historyTurns: cfg.HistoryTurns, idleWindow: cfg.IdleWindow,
	}
}

func (li *LLMInterpreter) Interpret(text string) (*Action, error) {
	now := li.now()
	// Reset por inactividad: si pasó demasiado desde la última frase, es charla nueva.
	if !li.lastTurn.IsZero() && now.Sub(li.lastTurn) > li.idleWindow {
		li.history = nil
	}
	li.lastTurn = now
	var facts []string
	if li.mem != nil {
		f, err := li.mem.Recall(text, li.topK)
		if err != nil { // recuperar falló → sigo sin hechos (no rompo el turno)
			fmt.Fprintln(os.Stderr, "(memoria falló al recuperar, sigo sin hechos:", err, ")")
		} else {
			facts = f
		}
	}
	reply, err := li.chat(li.systemPrompt(facts), li.history, text)
	if err != nil {
		// Logueamos: un fallback silencioso daría "verde falso" (las reglas rescatan
		// comandos con keyword y no te enterarías de que el LLM está muerto).
		fmt.Fprintln(os.Stderr, "(cerebro LLM falló, uso reglas:", err, ")")
		return li.fallback.Interpret(text)
	}
	var choice struct {
		Action string `json:"action"`
		Arg    string `json:"arg"`
		Say    string `json:"say"`
	}
	if jerr := json.Unmarshal([]byte(extractJSON(reply)), &choice); jerr != nil {
		fmt.Fprintln(os.Stderr, "(respuesta LLM no-JSON, uso reglas:", reply, ")")
		return li.fallback.Interpret(text)
	}
	// Charla pura: sin acción (o "none") pero con say → solo hablar.
	if (choice.Action == "" || choice.Action == "none") && choice.Say != "" {
		li.remember(text, choice.Say)
		return sayAction(choice.Say), nil
	}
	if choice.Action == "recordar" && li.mem != nil && choice.Arg != "" {
		li.remember(text, choice.Say)
		return wrap(memoryWriteAction(li.mem, choice.Arg), choice.Say), nil
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // rechazo de seguridad → NO se registra
		}
		li.remember(text, choice.Say)
		return wrap(openAppAction(choice.Arg), choice.Say), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		li.remember(text, choice.Say)
		return wrap(a, choice.Say), nil
	}
	return li.fallback.Interpret(text) // acción desconocida → NO se registra
}

// systemPrompt arma el prompt: instrucciones + hechos recuperados (si hay) + menú (orden estable).
func (li *LLMInterpreter) systemPrompt(facts []string) string {
	var b strings.Builder
	b.WriteString("Sos Astro, un asistente de escritorio con voz. El usuario te habla en español. ")
	b.WriteString("Respondé SOLO un JSON: {\"action\":\"<opcional>\",\"arg\":\"<opcional>\",\"say\":\"<respuesta hablada>\"}. ")
	b.WriteString("Si es un COMANDO, elegí un `action` del menú y un `say` corto de confirmación. ")
	b.WriteString("Si es CHARLA o una pregunta, usá action:\"none\" y contestá en `say`. ")
	b.WriteString("El `say` se lee en voz alta: que sea BREVE (1-2 frases), natural y en español.\n")
	if len(facts) > 0 {
		b.WriteString("Esto es lo que sé del usuario (usalo si viene al caso):\n")
		for _, f := range facts {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("Menú:\n")
	names := make([]string, 0, len(li.actions))
	for n := range li.actions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %s\n", n, li.actions[n].Desc)
	}
	b.WriteString("- open (arg = nombre de la app): abrir una app o programa\n")
	if li.mem != nil {
		b.WriteString("- recordar (arg = el hecho a recordar): guardá algo que el usuario te pide recordar\n")
	}
	return b.String()
}

// extractJSON toma el primer objeto {...} del texto (por si el modelo mete texto alrededor).
func extractJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j < i {
		return s
	}
	return s[i : j+1]
}

// httpChat devuelve un chatFunc que pega a un endpoint OpenAI-compatible (chat completions).
func httpChat(baseURL, apiKey, model string) chatFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(system string, history []Exchange, user string) (string, error) {
		messages := []map[string]string{{"role": "system", "content": system}}
		for _, ex := range history {
			messages = append(messages, map[string]string{"role": "user", "content": ex.User})
			messages = append(messages, map[string]string{"role": "assistant", "content": ex.Assistant})
		}
		messages = append(messages, map[string]string{"role": "user", "content": user})
		body, _ := json.Marshal(map[string]any{
			"model":           model,
			"temperature":     0,
			"response_format": map[string]string{"type": "json_object"},
			"messages":        messages,
		})
		url := strings.TrimRight(baseURL, "/") + "/chat/completions"
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
			return "", fmt.Errorf("respuesta LLM inesperada: %s", string(raw))
		}
		return out.Choices[0].Message.Content, nil
	}
}

// wrap devuelve una acción que corre el efecto de 'base' pero habla 'say' (lo que
// redactó el LLM) en vez de la frase fija. Si say=="", devuelve base tal cual (compat).
func wrap(base *Action, say string) *Action {
	if say == "" {
		return base
	}
	return &Action{Name: base.Name, Desc: base.Desc, Face: base.Face,
		Run: func(r Runner) (string, error) {
			if _, err := base.Run(r); err != nil {
				return "", err
			}
			return say, nil
		}}
}

// sayAction devuelve una acción que SOLO habla 'say' (charla; no toca la máquina).
func sayAction(say string) *Action {
	return &Action{Name: "decir", Face: Feliz,
		Run: func(r Runner) (string, error) { return say, nil }}
}

// memoryWriteAction guarda 'text' en la memoria. No toca el Runner (como sayAction); el say
// hablado lo pone wrap() con lo que redactó el LLM.
func memoryWriteAction(mem MemoryStore, text string) *Action {
	return &Action{Name: "recordar", Face: Feliz,
		Run: func(r Runner) (string, error) {
			if err := mem.Remember(text); err != nil {
				return "", err
			}
			return "", nil
		}}
}

// remember agrega el turno al historial efímero y lo recorta a la ventana de N.
func (li *LLMInterpreter) remember(user, assistant string) {
	li.history = append(li.history, Exchange{User: user, Assistant: assistant})
	if len(li.history) > li.historyTurns {
		li.history = li.history[len(li.history)-li.historyTurns:]
	}
}
