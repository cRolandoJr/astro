package main

import "testing"

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
