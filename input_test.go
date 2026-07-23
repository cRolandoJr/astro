package main

import (
	"reflect"
	"testing"
)

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

func TestVoiceInputCaptureArmaComandosYLimpia(t *testing.T) {
	fake := &fakeRunner{}
	v := VoiceInput{
		Runner: fake, WhisperBin: "whisper-cpp", WhisperModel: "/m/small.bin",
		RecSeconds: 4, WavPath: "/tmp/astro-in.wav",
		ReadFile: func(string) ([]byte, error) { return []byte("  pausá\n"), nil },
	}
	text, err := v.capture()
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if text != "pausá" {
		t.Fatalf("esperaba transcripción limpia 'pausá', fue %q", text)
	}
	// 2 comandos: arecord y whisper
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos (arecord, whisper), hubo %d: %v", len(fake.calls), fake.calls)
	}
	if fake.calls[0][0] != "arecord" {
		t.Errorf("primer comando debería ser arecord, fue %v", fake.calls[0])
	}
	wantWhisper := []string{"whisper-cpp", "-m", "/m/small.bin", "-f", "/tmp/astro-in.wav", "-l", "es", "-nt", "-otxt", "-of", "/tmp/astro-in"}
	if !reflect.DeepEqual(fake.calls[1], wantWhisper) {
		t.Errorf("comando whisper mal armado:\n got  %v\n want %v", fake.calls[1], wantWhisper)
	}
}
