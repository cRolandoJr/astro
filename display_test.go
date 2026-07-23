package main

import (
	"reflect"
	"testing"
)

func TestEwwFaceShowPasaConfigYVariable(t *testing.T) {
	fake := &fakeRunner{}
	face := EwwFace{Runner: fake, ConfigDir: "/cfg"}

	if err := face.Show(Feliz); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	got := fake.lastCall()
	want := []string{"eww", "--config", "/cfg", "update", "astro_face=feliz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, obtuve %v", want, got)
	}
}
