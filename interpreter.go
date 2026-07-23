package main

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNoEntiendo = errors.New("no entendí")

type Interpreter interface {
	Interpret(text string) (*Action, error)
}

// RuleInterpreter matchea por palabras clave. En la Fase 5 lo reemplaza uno con LLM.
type RuleInterpreter struct {
	actions map[string]*Action
}

func NewRuleInterpreter(actions map[string]*Action) *RuleInterpreter {
	return &RuleInterpreter{actions: actions}
}

func (ri *RuleInterpreter) Interpret(text string) (*Action, error) {
	t := normalize(text)
	switch {
	case containsAny(t, "hola", "buenas"):
		return ri.named("saludar")
	case containsAny(t, "hora"):
		return ri.named("hora")
	case containsAny(t, "pausa", "segui", "reproduc"):
		return ri.named("pausar")
	case containsAny(t, "siguiente", "proxima", "next"):
		return ri.named("siguiente")
	case containsAny(t, "subi", "sube", "mas volumen"):
		return ri.named("subir_volumen")
	case containsAny(t, "baja", "menos volumen"):
		return ri.named("bajar_volumen")
	case containsAny(t, "mute", "silencio", "callate"):
		return ri.named("mute")
	case containsAny(t, "dormi", "chau"):
		return ri.named("dormir")
	case containsAny(t, "despert"):
		return ri.named("despertar")
	case strings.HasPrefix(t, "abri"):
		parts := strings.SplitN(t, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return nil, ErrNoEntiendo
		}
		app := strings.TrimSpace(parts[1])
		if !isSafeAppName(app) {
			return nil, ErrNoEntiendo
		}
		return openAppAction(app), nil
	default:
		return nil, ErrNoEntiendo
	}
}

// named busca la acción con coma-ok: si la clave no está (bug de registro), devuelve
// ErrNoEntiendo en vez de un *Action nil que reventaría en main.
func (ri *RuleInterpreter) named(name string) (*Action, error) {
	a, ok := ri.actions[name]
	if !ok {
		return nil, fmt.Errorf("acción %q no registrada: %w", name, ErrNoEntiendo)
	}
	return a, nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// normalize: minúsculas, sin espacios extra, sin tildes (comandos de terminal a menudo
// van sin tilde) y sin puntuación final (whisper le agrega punto a "abrí firefox").
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u").Replace(s)
	return strings.TrimRight(s, " .,;:!?¡¿")
}

// isSafeAppName valida que el nombre de app a pasar a `hyprctl dispatch exec` sea un
// token simple: sin espacios ni metacaracteres de shell (C1: inyección de comandos).
// Solo permite letras, dígitos, punto, guion bajo y guion medio.
func isSafeAppName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
