package main

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func fakeChat(reply string, err error) chatFunc {
	return func(system, user string) (string, error) { return reply, err }
}

func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(chat, acts, NewRuleInterpreter(acts))
}

func TestLLMMapeaAccionDelRegistro(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"pausar"}`, nil)).Interpret("che poné pausa")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba pausar; a=%v err=%v", a, err)
	}
}

func TestLLMOpenAbreLaApp(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox"}`, nil)).Interpret("abrí el navegador")
	if err != nil || a == nil {
		t.Fatalf("esperaba acción; a=%v err=%v", a, err)
	}
	if _, err := a.Run(fake); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

// Seguridad: aunque el LLM devuelva un arg peligroso, isSafeAppName lo frena → fallback → ErrNoEntiendo.
func TestLLMOpenInyeccionRechazada(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox; rm -rf ~"}`, nil)).Interpret("abrí firefox; rm -rf ~")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("el arg peligroso debía rechazarse; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiChatFalla(t *testing.T) {
	a, err := newLLM(fakeChat("", fmt.Errorf("sin red"))).Interpret("pausá")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba fallback a reglas → pausar; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiJSONInvalido(t *testing.T) {
	a, err := newLLM(fakeChat("no soy json", nil)).Interpret("hola")
	if err != nil || a == nil || a.Name != "saludar" {
		t.Fatalf("esperaba fallback → saludar; a=%v err=%v", a, err)
	}
}
