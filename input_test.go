package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"testing"
)

func TestIsTimeout(t *testing.T) {
	if !isTimeout(exec.Command("sh", "-c", "exit 124").Run()) {
		t.Error("exit 124 (tope de timeout) debe reconocerse como timeout, no como falla")
	}
	if isTimeout(exec.Command("sh", "-c", "exit 1").Run()) {
		t.Error("exit 1 no debe contarse como timeout")
	}
	if isTimeout(nil) {
		t.Error("nil no es timeout")
	}
}

func TestIsRealUtterance(t *testing.T) {
	real := []string{"qué hora es", "poné pausa", "sí", "no", "abrí firefox", "reproducí música"}
	junk := []string{"", "   ", ".", "...", "Gracias.", "gracias", "Adiós", "muchas gracias",
		"Subtítulos", "[BLANK_AUDIO]", "[silencio]"}
	for _, s := range real {
		if !isRealUtterance(s) {
			t.Errorf("isRealUtterance(%q) = false, quiero true", s)
		}
	}
	for _, s := range junk {
		if isRealUtterance(s) {
			t.Errorf("isRealUtterance(%q) = true, quiero false (vacío/ruido/alucinación)", s)
		}
	}
}

func TestCleanTranscript(t *testing.T) {
	cases := map[string]string{
		"  Hola Astro.\n":    "Hola Astro.",
		"[BLANK_AUDIO]":      "",
		"[música] pausá":     "pausá",
		"\n  qué hora es \n": "qué hora es",
	}
	for raw, want := range cases {
		if got := cleanTranscript(raw); got != want {
			t.Errorf("cleanTranscript(%q) = %q, esperaba %q", raw, got, want)
		}
	}
}

func TestVoiceCaptureGrabaHastaSilencio(t *testing.T) {
	fake := &fakeRunner{}
	v := VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("hola astro"), nil },
	}
	got, err := v.capture()
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if got != "hola astro" {
		t.Fatalf("esperaba la transcripción, fue %q", got)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos (rec, whisper), hubo %d: %v", len(fake.calls), fake.calls)
	}
	// comando 0: timeout <max> rec ... silence ...
	wantRec := []string{"timeout", "30", "rec", "-q", "-c", "1", "-r", "16000", "/tmp/astro-in.wav",
		"silence", "1", "0.1", "3%", "1", "1.5", "3%"}
	if !reflect.DeepEqual(fake.calls[0], wantRec) {
		t.Fatalf("comando de grabación:\n esperaba %v\n fue      %v", wantRec, fake.calls[0])
	}
	// comando 1: whisper (armado que se conserva de la versión anterior)
	wantWhisper := []string{"whisper-cli", "-m", "m.bin", "-f", "/tmp/astro-in.wav", "-l", "es", "-nt", "-otxt", "-of", "/tmp/astro-in"}
	if !reflect.DeepEqual(fake.calls[1], wantWhisper) {
		t.Fatalf("comando whisper:\n esperaba %v\n fue      %v", wantWhisper, fake.calls[1])
	}
}

func TestVoiceCaptureErrorSiRecFalla(t *testing.T) {
	fake := &fakeRunner{err: fmt.Errorf("device busy")}
	v := VoiceInput{Runner: fake, WavPath: "/tmp/x.wav"}
	if _, err := v.capture(); err == nil {
		t.Fatal("esperaba error si rec falla")
	}
}

type fakeDisplay struct {
	opened, closed bool
	shown          []Expression
}

func (f *fakeDisplay) Open() error             { f.opened = true; return nil }
func (f *fakeDisplay) Close() error            { f.closed = true; return nil }
func (f *fakeDisplay) Show(e Expression) error { f.shown = append(f.shown, e); return nil }

func TestVoiceCaptureAbreYMuestraCaraEscuchando(t *testing.T) {
	disp := &fakeDisplay{}
	v := VoiceInput{
		Runner: &fakeRunner{}, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30, Face: disp,
		ReadFile: func(string) ([]byte, error) { return []byte("hola"), nil },
	}
	if _, err := v.capture(); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if !disp.opened {
		t.Fatal("esperaba que capture abriera la cara al empezar a escuchar")
	}
	if len(disp.shown) == 0 || disp.shown[0] != Curioso {
		t.Fatalf("esperaba mostrar 'Curioso' al escuchar, fue %v", disp.shown)
	}
}
