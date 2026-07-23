package main

import "math"

// Fact es un hecho recordado + su embedding (persistimos el vector para no re-embeber al boot).
type Fact struct {
	Text string    `json:"text"`
	Vec  []float32 `json:"vector"`
}

// MemoryStore guarda y recupera hechos. Es el seam: hoy FileMemory (archivo + coseno),
// mañana se podría enchufar Chroma sin tocar el resto.
type MemoryStore interface {
	Remember(text string) error
	Recall(query string, k int) ([]string, error)
}

// embedFunc convierte texto en vector. Seam: real por HTTP (embed.go) o fake en tests.
type embedFunc func(text string) ([]float32, error)

// cosine mide similitud entre dos vectores (1 = iguales, 0 = ortogonales). Devuelve 0 si las
// dimensiones no coinciden o alguna norma es cero (no paniquea con vectores raros/viejos).
func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
