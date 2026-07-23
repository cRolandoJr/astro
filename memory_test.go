package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCosineIdenticos(t *testing.T) {
	v := []float32{1, 2, 3}
	if got := cosine(v, v); got < 0.999 || got > 1.001 {
		t.Fatalf("vectores iguales → ~1, fue %v", got)
	}
}

func TestCosineOrtogonales(t *testing.T) {
	if got := cosine([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Fatalf("ortogonales → 0, fue %v", got)
	}
}

func TestCosineDimsDistintas(t *testing.T) {
	if got := cosine([]float32{1, 0}, []float32{1, 0, 0}); got != 0 {
		t.Fatalf("dims distintas → 0, fue %v", got)
	}
}

func TestCosineVectorCero(t *testing.T) {
	if got := cosine([]float32{0, 0}, []float32{1, 1}); got != 0 {
		t.Fatalf("vector cero → 0, fue %v", got)
	}
}

// fakeEmbed: embedder determinista para tests; cuenta llamadas (para verificar que NO se
// re-embebe al reabrir), puede devolver error (para el camino de fallo) y mapea textos a vectores.
type fakeEmbed struct {
	calls int
	err   error
	table map[string][]float32
}

func (f *fakeEmbed) fn(text string) ([]float32, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if v, ok := f.table[text]; ok {
		return v, nil
	}
	return []float32{0, 0, 0}, nil
}

func TestFileMemoryRememberYRecall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.json")
	fe := &fakeEmbed{table: map[string][]float32{
		"uso nixos":    {1, 0, 0},
		"me gusta go":  {0, 1, 0},
		"linux distro": {0.9, 0.1, 0}, // consulta parecida a "uso nixos"
	}}
	m, _ := NewFileMemory(path, fe.fn)
	if err := m.Remember("uso nixos"); err != nil {
		t.Fatal(err)
	}
	if err := m.Remember("me gusta go"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Recall("linux distro", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "uso nixos" {
		t.Fatalf("esperaba el hecho más cercano 'uso nixos', fue %v", got)
	}
}

func TestFileMemoryPersisteSinReembebir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.json")
	fe := &fakeEmbed{table: map[string][]float32{"uso nixos": {1, 0, 0}}}
	m, _ := NewFileMemory(path, fe.fn)
	if err := m.Remember("uso nixos"); err != nil {
		t.Fatal(err)
	}
	llamadasTrasGuardar := fe.calls
	// Reabrir desde el mismo archivo NO debe re-embeber (el vector ya está persistido).
	m2, err := NewFileMemory(path, fe.fn)
	if err != nil {
		t.Fatal(err)
	}
	if len(m2.facts) != 1 || m2.facts[0].Text != "uso nixos" {
		t.Fatalf("el hecho no sobrevivió al reinicio: %v", m2.facts)
	}
	if fe.calls != llamadasTrasGuardar {
		t.Fatalf("reabrir no debe re-embeber; llamadas subieron de %d a %d", llamadasTrasGuardar, fe.calls)
	}
}

func TestFileMemoryVacioNoLlamaRed(t *testing.T) {
	fe := &fakeEmbed{}
	m, _ := NewFileMemory(filepath.Join(t.TempDir(), "mem.json"), fe.fn)
	got, err := m.Recall("cualquier cosa", 5)
	if err != nil || got != nil {
		t.Fatalf("memoria vacía → nil sin error; got=%v err=%v", got, err)
	}
	if fe.calls != 0 {
		t.Fatalf("memoria vacía no debe embeber; hubo %d llamadas", fe.calls)
	}
}

func TestFileMemoryArchivoCorrupto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.json")
	if err := os.WriteFile(path, []byte("{no soy json valido"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewFileMemory(path, (&fakeEmbed{}).fn)
	if err != nil {
		t.Fatalf("archivo corrupto no debe ser error fatal: %v", err)
	}
	if len(m.facts) != 0 {
		t.Fatalf("corrupto → memoria vacía, fue %v", m.facts)
	}
}

// #1 (mentor): el spec §6 promete que un embed caído devuelve error y NO muta facts.
func TestFileMemoryEmbedFallaNoMuta(t *testing.T) {
	fe := &fakeEmbed{err: fmt.Errorf("embed caído")}
	m, _ := NewFileMemory(filepath.Join(t.TempDir(), "m.json"), fe.fn)
	if err := m.Remember("x"); err == nil {
		t.Fatal("embed caído debe devolver error")
	}
	if len(m.facts) != 0 {
		t.Fatalf("no debe mutar facts si falló el embed, fue %v", m.facts)
	}
}

// #2 (mentor): si persist falla, el hecho ya appendeado se revierte. Forzamos el fallo con un
// path cuyo directorio padre es un ARCHIVO → MkdirAll (dentro de persist) falla.
func TestFileMemoryPersistFallaRevierte(t *testing.T) {
	dir := t.TempDir()
	archivo := filepath.Join(dir, "soy-archivo")
	if err := os.WriteFile(archivo, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(archivo, "mem.json") // el "dir" padre es en realidad un archivo
	fe := &fakeEmbed{table: map[string][]float32{"hecho": {1, 0, 0}}}
	m, _ := NewFileMemory(path, fe.fn)
	if err := m.Remember("hecho"); err == nil {
		t.Fatal("persist debía fallar (el dir padre es un archivo)")
	}
	if len(m.facts) != 0 {
		t.Fatalf("si persist falla, el hecho se revierte; facts=%v", m.facts)
	}
}

// #4 (mentor): k>len se recorta (sin panic) y el resultado viene ordenado por coseno desc.
func TestFileMemoryRecallOrdenYClamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.json")
	fe := &fakeEmbed{table: map[string][]float32{
		"cerca": {1, 0, 0},
		"medio": {0.5, 0.5, 0},
		"lejos": {0, 1, 0},
		"query": {0.9, 0.1, 0}, // cerca > medio > lejos
	}}
	m, _ := NewFileMemory(path, fe.fn)
	for _, txt := range []string{"cerca", "medio", "lejos"} {
		if err := m.Remember(txt); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.Recall("query", 5) // k=5 > 3 hechos → clamp a 3
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cerca", "medio", "lejos"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v (clamp k>len + orden por coseno), fue %v", want, got)
	}
}
