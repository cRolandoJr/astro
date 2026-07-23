package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fakeChat(reply string, err error) chatFunc {
	return func(system string, history []Exchange, user string) (string, error) { return reply, err }
}

func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts)})
}

// fakeMemory: MemoryStore de test. Registra lo guardado y devuelve un Recall fijo.
type fakeMemory struct {
	remembered []string
	recall     []string
	recallErr  error
}

func (f *fakeMemory) Remember(text string) error {
	f.remembered = append(f.remembered, text)
	return nil
}
func (f *fakeMemory) Recall(query string, k int) ([]string, error) {
	return f.recall, f.recallErr
}

func newLLMWithMem(chat chatFunc, mem MemoryStore) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts), Mem: mem})
}

func TestLLMRecordarGuardaElHecho(t *testing.T) {
	mem := &fakeMemory{}
	a, err := newLLMWithMem(fakeChat(`{"action":"recordar","arg":"uso NixOS","say":"Dale, anotado."}`, nil), mem).
		Interpret("acordate que uso NixOS")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(&fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Dale, anotado." {
		t.Fatalf("esperaba el say, fue %q", reply)
	}
	if len(mem.remembered) != 1 || mem.remembered[0] != "uso NixOS" {
		t.Fatalf("esperaba guardar 'uso NixOS', fue %v", mem.remembered)
	}
}

func TestLLMRecordarSinMemVaAFallback(t *testing.T) {
	// mem nil → 'recordar' no disponible → fallback (reglas → ErrNoEntiendo).
	a, err := newLLM(fakeChat(`{"action":"recordar","arg":"algo","say":"ok"}`, nil)).Interpret("xyzzy")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("sin memoria, recordar debe caer al fallback; a=%v err=%v", a, err)
	}
}

func TestLLMInyectaHechosAlPrompt(t *testing.T) {
	mem := &fakeMemory{recall: []string{"el usuario usa NixOS"}}
	var seenSystem string
	chat := func(system string, history []Exchange, user string) (string, error) {
		seenSystem = system
		return `{"action":"none","say":"ok"}`, nil
	}
	if _, err := newLLMWithMem(chat, mem).Interpret("qué distro uso"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenSystem, "el usuario usa NixOS") {
		t.Fatalf("el prompt debía incluir el hecho recuperado; prompt=%q", seenSystem)
	}
}

func TestLLMRecallErrorNoRompeElTurno(t *testing.T) {
	mem := &fakeMemory{recallErr: fmt.Errorf("embed caído")}
	a, err := newLLMWithMem(fakeChat(`{"action":"none","say":"hola"}`, nil), mem).Interpret("hola")
	if err != nil || a == nil {
		t.Fatalf("un fallo de Recall no debe romper el turno; a=%v err=%v", a, err)
	}
}

// #3 (mentor): recordar con arg vacío no debe guardar un hecho vacío (no matchea → fallback).
func TestLLMRecordarArgVacioNoGuarda(t *testing.T) {
	mem := &fakeMemory{}
	_, _ = newLLMWithMem(fakeChat(`{"action":"recordar","arg":"","say":"ok"}`, nil), mem).Interpret("acordate")
	if len(mem.remembered) != 0 {
		t.Fatalf("no debe guardar un hecho vacío; guardó %v", mem.remembered)
	}
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

func TestLLMActionConSayHablaElSay(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"pausar","say":"Dale, te pausé."}`, nil)).Interpret("poné pausa")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Dale, te pausé." {
		t.Fatalf("esperaba el say del LLM, fue %q", reply)
	}
	if got := fake.lastCall(); len(got) < 1 || got[0] != "playerctl" {
		t.Fatalf("esperaba que corriera playerctl, fue %v", got)
	}
}

func TestLLMCharlaSoloHabla(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"none","say":"Estoy bien, ¿y vos?"}`, nil)).Interpret("cómo estás")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Estoy bien, ¿y vos?" {
		t.Fatalf("esperaba la charla, fue %q", reply)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("la charla no debe ejecutar comandos, hubo %v", fake.calls)
	}
}

func TestLLMSayPuroSinAction(t *testing.T) {
	a, err := newLLM(fakeChat(`{"say":"un chiste corto"}`, nil)).Interpret("contame un chiste")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, _ := a.Run(&fakeRunner{})
	if reply != "un chiste corto" {
		t.Fatalf("fue %q", reply)
	}
}
