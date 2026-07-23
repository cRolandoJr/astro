package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestHotkeyListenDisparaCaptura(t *testing.T) {
	fake := &fakeRunner{}
	voice := &VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("qué hora es"), nil },
	}
	h := &HotkeyInput{events: bufio.NewScanner(strings.NewReader("go\n")), voice: voice}
	got, err := h.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got != "qué hora es" {
		t.Fatalf("esperaba el comando capturado tras el disparo, fue %q", got)
	}
	if len(fake.calls) < 1 || fake.calls[0][0] != "timeout" || fake.calls[0][2] != "rec" {
		t.Fatalf("esperaba captura (timeout rec …) tras el disparo, fue %v", fake.calls)
	}
}

func TestNewHotkeyInputSinFifoError(t *testing.T) {
	if _, err := NewHotkeyInput("", &VoiceInput{}); err == nil {
		t.Fatal("sin ASTRO_TRIGGER_FIFO debía dar error")
	}
}

func TestHotkeyCloseNilSafe(t *testing.T) {
	h := &HotkeyInput{events: bufio.NewScanner(strings.NewReader(""))}
	if err := h.Close(); err != nil {
		t.Fatalf("Close debía ser nil-safe, fue %v", err)
	}
}
