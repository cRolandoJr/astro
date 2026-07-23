package main

import (
	"strings"
	"testing"
	"time"
)

func fixedClock() time.Time { return time.Date(2026, 7, 23, 9, 5, 0, 0, time.UTC) }

func TestSaludarReply(t *testing.T) {
	a := buildActions(fixedClock)["saludar"]
	reply, err := a.Run(&fakeRunner{})
	if err != nil || reply == "" {
		t.Fatalf("esperaba saludo sin error; reply=%q err=%v", reply, err)
	}
}

func TestHoraUsaElReloj(t *testing.T) {
	a := buildActions(fixedClock)["hora"]
	reply, err := a.Run(&fakeRunner{})
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if !strings.Contains(reply, "09:05") {
		t.Fatalf("esperaba que incluyera 09:05, fue %q", reply)
	}
}
