package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
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
	}
	if jerr := json.Unmarshal([]byte(extractJSON(reply)), &choice); jerr != nil {
		fmt.Fprintln(os.Stderr, "(respuesta LLM no-JSON, uso reglas:", reply, ")")
		return li.fallback.Interpret(text)
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // arg peligroso → no por acá
		}
		return openAppAction(choice.Arg), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		return a, nil
	}
	return li.fallback.Interpret(text) // acción desconocida / "none" → reglas
}

// systemPrompt arma el menú de acciones para el LLM (orden estable).
func (li *LLMInterpreter) systemPrompt() string {
	var b strings.Builder
	b.WriteString("Sos Astro, un asistente de escritorio. El usuario te habla en español. ")
	b.WriteString("Elegí UNA acción de la lista según lo que pide. Respondé SOLO un JSON: ")
	b.WriteString(`{"action":"<nombre>","arg":"<opcional>"}`)
	b.WriteString(". Si ninguna aplica, usá \"none\". Acciones:\n")
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
