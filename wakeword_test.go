package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestWakeWordListenDisparaCapturaTrasEvento(t *testing.T) {
	fake := &fakeRunner{}
	voice := &VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("qué hora es"), nil },
	}
	w := &WakeWordInput{events: bufio.NewScanner(strings.NewReader("DETECTED\n")), voice: voice}
	got, err := w.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got != "qué hora es" {
		t.Fatalf("esperaba el comando capturado tras el wake, fue %q", got)
	}
	// tras el wake, se grabó (rec) y transcribió (whisper) — el primer comando es la captura
	if len(fake.calls) < 1 || fake.calls[0][0] != "timeout" || fake.calls[0][2] != "rec" {
		t.Fatalf("esperaba captura (timeout rec …) tras el wake, fue %v", fake.calls)
	}
}

func TestWakeWordListenSidecarMuertoReporta(t *testing.T) {
	// stdout cerrado/vacío = el sidecar murió → Scan false sin error → error descriptivo (falla ruidosa)
	w := &WakeWordInput{events: bufio.NewScanner(strings.NewReader("")), voice: &VoiceInput{Runner: &fakeRunner{}}}
	if _, err := w.Listen(); err == nil {
		t.Fatal("sidecar muerto (stdout cerrado) debía devolver error, no salida limpia")
	}
}

func TestNewWakeWordInputSinCmdError(t *testing.T) {
	if _, err := NewWakeWordInput("", &VoiceInput{}); err == nil {
		t.Fatal("sin ASTRO_WAKE_CMD debía dar error")
	}
}
