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

func TestQueSuenaDiceLaCancion(t *testing.T) {
	acts := buildActions(time.Now)
	fake := &fakeRunner{output: "Nate Gentile - Mi PC Linux"}
	reply, err := acts["que_suena"].Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Está sonando: Nate Gentile - Mi PC Linux." {
		t.Fatalf("fue %q", reply)
	}
	if got := fake.lastCall(); got[0] != "playerctl" || got[1] != "metadata" {
		t.Fatalf("esperaba playerctl metadata, fue %v", got)
	}
}

func TestBateriaParseaPorcentaje(t *testing.T) {
	acts := buildActions(time.Now)
	// upower -e → device; upower -i <dev> → info con "percentage:"
	fake := &fakeRunner{output: "/org/freedesktop/UPower/devices/battery_BAT0\n    percentage:          87%\n"}
	reply, err := acts["bateria"].Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Batería al 87%." {
		t.Fatalf("fue %q", reply)
	}
	// N1: ata que la 1ª llamada enumera y la 2ª pide info del device encontrado
	if len(fake.calls) != 2 || !reflect.DeepEqual(fake.calls[0], []string{"upower", "-e"}) {
		t.Fatalf("esperaba upower -e primero, fue %v", fake.calls)
	}
	if len(fake.calls[1]) < 3 || !strings.Contains(fake.calls[1][2], "battery_BAT0") {
		t.Fatalf("esperaba upower -i <dev battery>, fue %v", fake.calls[1])
	}
}

func TestFechaEnEspanol(t *testing.T) {
	fija := func() time.Time { return time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC) } // viernes 24 de julio
	acts := buildActions(fija)
	reply, _ := acts["fecha"].Run(&fakeRunner{})
	if reply != "Hoy es viernes 24 de julio." {
		t.Fatalf("fue %q", reply)
	}
}

func TestClimaVacioAmable(t *testing.T) {
	acts := buildActions(time.Now)
	reply, err := acts["clima"].Run(&fakeRunner{output: ""})
	if err != nil || reply != "No pude ver el clima." {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}

func TestAgendaPrimerEventoNoVacio(t *testing.T) {
	acts := buildActions(time.Now)
	// khal puede meter una línea vacía/encabezado; tomamos la 1ª NO vacía
	fake := &fakeRunner{output: "\n10:00 Reunión con el equipo\n12:00 Almuerzo\n"}
	reply, _ := acts["agenda"].Run(fake)
	if reply != "Próximo: 10:00 Reunión con el equipo." {
		t.Fatalf("fue %q", reply)
	}
}
