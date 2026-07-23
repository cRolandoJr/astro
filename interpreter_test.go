package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func testInterpreter() Interpreter { return NewRuleInterpreter(buildActions(time.Now)) }

// Table-test: toda frase de ejemplo resuelve a una acción NO nil (guard del must-fix #1).
func TestCadaReglaResuelveAccion(t *testing.T) {
	cmds := []string{"hola", "qué hora es", "pausá", "siguiente", "subí el volumen",
		"bajá el volumen", "silencio", "dormí", "despertá", "abrí firefox"}
	in := testInterpreter()
	for _, c := range cmds {
		a, err := in.Interpret(c)
		if err != nil || a == nil {
			t.Errorf("%q → acción nil o error: a=%v err=%v", c, a, err)
		}
	}
}

func TestNoEntiende(t *testing.T) {
	if _, err := testInterpreter().Interpret("xyzzy"); !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("esperaba ErrNoEntiendo, fue %v", err)
	}
}

func TestAbriSinAppNoEntiende(t *testing.T) {
	if _, err := testInterpreter().Interpret("abrí"); !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("esperaba ErrNoEntiendo, fue %v", err)
	}
}

// TestAbriRechazaInyeccion: nombres de app con metacaracteres de shell o espacios
// extra deben rechazarse antes de llegar a openAppAction (C1: inyección de comandos).
func TestAbriRechazaInyeccion(t *testing.T) {
	cmds := []string{
		"abrí firefox; rm -rf ~",
		"abrí firefox | tee x",
		"abrí firefox foo",
	}
	in := testInterpreter()
	for _, c := range cmds {
		if _, err := in.Interpret(c); !errors.Is(err, ErrNoEntiendo) {
			t.Errorf("%q → esperaba ErrNoEntiendo (rechazo), fue %v", c, err)
		}
	}
}

// TestAbriAppValidaFunciona: el caso legítimo (nombre de app simple) sigue andando.
func TestAbriAppValidaFunciona(t *testing.T) {
	a, err := testInterpreter().Interpret("abrí firefox")
	if err != nil || a == nil {
		t.Fatalf("esperaba acción válida, fue a=%v err=%v", a, err)
	}
}

// TestAbriConPuntuacion: whisper agrega punto final ("Abrí firefox.") y eso no debe
// filtrarse al nombre de app que llega a hyprctl (must-fix del review).
func TestAbriConPuntuacion(t *testing.T) {
	in := testInterpreter()
	a, err := in.Interpret("Abrí firefox.")
	if err != nil || a == nil {
		t.Fatalf("esperaba acción válida, fue a=%v err=%v", a, err)
	}
	fake := &fakeRunner{}
	if _, err := a.Run(fake); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v (sin el punto), fue %v", want, got)
	}

	pausar, err := testInterpreter().Interpret("pausá.")
	if err != nil || pausar == nil || pausar.Name != "pausar" {
		t.Fatalf("esperaba acción 'pausar', fue a=%v err=%v", pausar, err)
	}
}
