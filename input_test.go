package main

import (

	"testing"
)

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
		ReadFile:  func(string) ([]byte, error) { return []byte("hola"), nil },
		RecordVAD: func(string, int, float64) error { return nil },
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
