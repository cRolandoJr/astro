package main

import (
	"errors"
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
