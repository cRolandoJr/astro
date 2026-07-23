package main

import (
	"reflect"
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

func TestSubirVolumenLlamaWpctl(t *testing.T) {
	fake := &fakeRunner{}
	if _, err := buildActions(fixedClock)["subir_volumen"].Run(fake); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	want := []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "5%+"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestAbrirUsaHyprctl(t *testing.T) {
	fake := &fakeRunner{}
	if _, err := openAppAction("firefox").Run(fake); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestAccionesTienenDescripcion(t *testing.T) {
	for name, a := range buildActions(time.Now) {
		if a.Desc == "" {
			t.Errorf("acción %q sin Desc (la necesita el menú del LLM)", name)
		}
	}
}
