package main

import (
	"reflect"
	"testing"
)

func newTestChirps(enabled bool) Chirps {
	return Chirps{
		Runner: nil, Enabled: enabled, WavPath: "/tmp/astro-chirp.wav",
		Specs: map[string]string{"wake": "synth 0.18 sine 500-1400 vol 0.5"},
	}
}

func TestChirpsPlayGeneraYReproduce(t *testing.T) {
	fake := &fakeRunner{}
	c := newTestChirps(true)
	c.Runner = fake

	c.Play("wake")

	// 2 comandos: sox (genera el wav) y pw-play (lo reproduce)
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos, hubo %d: %v", len(fake.calls), fake.calls)
	}
	wantSox := []string{"sox", "-n", "/tmp/astro-chirp.wav", "synth", "0.18", "sine", "500-1400", "vol", "0.5"}
	if !reflect.DeepEqual(fake.calls[0], wantSox) {
		t.Errorf("comando sox:\n got  %v\n want %v", fake.calls[0], wantSox)
	}
	if fake.calls[1][0] != "pw-play" {
		t.Errorf("segundo comando debería ser pw-play, fue %v", fake.calls[1])
	}
}

func TestChirpsDeshabilitadoNoHaceNada(t *testing.T) {
	fake := &fakeRunner{}
	c := newTestChirps(false)
	c.Runner = fake
	c.Play("wake")
	if len(fake.calls) != 0 {
		t.Fatalf("deshabilitado no debería llamar a nada, hubo %v", fake.calls)
	}
}

func TestChirpsKindDesconocidoNoHaceNada(t *testing.T) {
	fake := &fakeRunner{}
	c := newTestChirps(true)
	c.Runner = fake
	c.Play("inexistente")
	if len(fake.calls) != 0 {
		t.Fatalf("kind desconocido no debería llamar a nada, hubo %v", fake.calls)
	}
}
