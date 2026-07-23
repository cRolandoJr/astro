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

// chatFunc habla con el LLM: recibe prompt de sistema + texto del usuario, devuelve la
// respuesta cruda (el JSON). Es un seam: real por HTTP (Task 3) o fake en tests.
type chatFunc func(system, user string) (string, error)

// LLMInterpreter elige una acción del registro usando un LLM. Si el LLM falla o devuelve
// algo raro, cae al 'fallback' (el RuleInterpreter). Implementa Interpreter.
type LLMInterpreter struct {
	chat     chatFunc
	actions  map[string]*Action
	fallback Interpreter
}

func NewLLMInterpreter(chat chatFunc, actions map[string]*Action, fallback Interpreter) *LLMInterpreter {
	return &LLMInterpreter{chat: chat, actions: actions, fallback: fallback}
}

func (li *LLMInterpreter) Interpret(text string) (*Action, error) {
	reply, err := li.chat(li.systemPrompt(), text)
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
		return sayAction(choice.Say), nil
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // arg peligroso → no por acá
		}
		return wrap(openAppAction(choice.Arg), choice.Say), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		return wrap(a, choice.Say), nil
	}
	return li.fallback.Interpret(text) // ni acción ni say → reglas
}

// systemPrompt arma el menú de acciones para el LLM (orden estable).
func (li *LLMInterpreter) systemPrompt() string {
	var b strings.Builder
	b.WriteString("Sos Astro, un asistente de escritorio con voz. El usuario te habla en español. ")
	b.WriteString("Respondé SOLO un JSON: {\"action\":\"<opcional>\",\"arg\":\"<opcional>\",\"say\":\"<respuesta hablada>\"}. ")
	b.WriteString("Si es un COMANDO, elegí un `action` del menú y un `say` corto de confirmación. ")
	b.WriteString("Si es CHARLA o una pregunta, usá action:\"none\" y contestá en `say`. ")
	b.WriteString("El `say` se lee en voz alta: que sea BREVE (1-2 frases), natural y en español. Menú:\n")
	names := make([]string, 0, len(li.actions))
	for n := range li.actions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %s\n", n, li.actions[n].Desc)
	}
	b.WriteString("- open (arg = nombre de la app): abrir una app o programa\n")
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
	return func(system, user string) (string, error) {
		body, _ := json.Marshal(map[string]any{
			"model":           model,
			"temperature":     0,
			"response_format": map[string]string{"type": "json_object"},
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
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
