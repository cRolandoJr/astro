package main

import (
	"reflect"
	"testing"
)

func TestPiperVoiceSayPasaTextoPorStdinYReproduce(t *testing.T) {
	fake := &fakeRunner{}
	p := PiperVoice{Runner: fake, PiperBin: "piper", Voice: "/v/es.onnx", WavPath: "/tmp/astro-out.wav"}

	if err := p.Say("hola"); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	// piper recibe el texto por stdin
	if fake.lastInput() != "hola" {
		t.Errorf("esperaba 'hola' por stdin de piper, fue %q", fake.lastInput())
	}
	// 2 comandos: piper (síntesis) y pw-play (reproducción)
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos, hubo %d: %v", len(fake.calls), fake.calls)
	}
	wantPiper := []string{"piper", "--model", "/v/es.onnx", "--output_file", "/tmp/astro-out.wav"}
	if !reflect.DeepEqual(fake.calls[0], wantPiper) {
		t.Errorf("comando piper:\n got  %v\n want %v", fake.calls[0], wantPiper)
	}
	if fake.calls[1][0] != "pw-play" {
		t.Errorf("segundo comando debería ser pw-play, fue %v", fake.calls[1])
	}
}

func TestPiperVoiceSayVacioNoHaceNada(t *testing.T) {
	fake := &fakeRunner{}
	if err := (PiperVoice{Runner: fake, PiperBin: "piper"}).Say(""); err != nil {
		t.Fatalf("no debería fallar con texto vacío: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("texto vacío no debería llamar a nada, hubo %v", fake.calls)
	}
}

func TestPiperVoiceSayConEfectoAplicaSox(t *testing.T) {
	fake := &fakeRunner{}
	p := PiperVoice{
		Runner: fake, PiperBin: "piper", Voice: "/v/es.onnx",
		WavPath: "/tmp/astro-out.wav", FxPath: "/tmp/astro-fx.wav",
		FxArgs: []string{"pitch", "200", "tremolo", "20"},
	}
	if err := p.Say("hola"); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	// 3 comandos: piper (síntesis) → sox (efecto) → pw-play (del wav con efecto)
	if len(fake.calls) != 3 {
		t.Fatalf("esperaba 3 comandos con efecto, hubo %d: %v", len(fake.calls), fake.calls)
	}
	wantSox := []string{"sox", "/tmp/astro-out.wav", "/tmp/astro-fx.wav", "pitch", "200", "tremolo", "20"}
	if !reflect.DeepEqual(fake.calls[1], wantSox) {
		t.Errorf("comando sox:\n got  %v\n want %v", fake.calls[1], wantSox)
	}
	wantPlay := []string{"pw-play", "/tmp/astro-fx.wav"}
	if !reflect.DeepEqual(fake.calls[2], wantPlay) {
		t.Errorf("debería reproducir el wav con efecto:\n got  %v\n want %v", fake.calls[2], wantPlay)
	}
}
