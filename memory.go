package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

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

// FileMemory guarda los hechos en un archivo JSON (texto+vector) y busca por coseno en RAM.
// Un solo goroutine (el loop principal) lo usa → sin mutex.
type FileMemory struct {
	path  string
	embed embedFunc
	facts []Fact
}

// NewFileMemory carga los hechos guardados a RAM. Archivo ausente = memoria vacía (no es
// error: es la primera vez). Archivo corrupto = log + vacío (no rompe el arranque del daemon).
func NewFileMemory(path string, embed embedFunc) (*FileMemory, error) {
	m := &FileMemory{path: path, embed: embed}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		// El "directorio" padre puede ser en realidad un archivo (ENOTDIR, no ENOENT):
		// también cuenta como "no hay memoria todavía", no como error fatal de lectura.
		if info, statErr := os.Stat(filepath.Dir(path)); statErr == nil && !info.IsDir() {
			return m, nil
		}
		return nil, fmt.Errorf("no pude leer la memoria: %w", err)
	}
	if err := json.Unmarshal(raw, &m.facts); err != nil {
		fmt.Fprintln(os.Stderr, "(memoria corrupta, arranco vacío:", err, ")")
		m.facts = nil
	}
	return m, nil
}

// Remember embebe el texto y lo persiste (texto+vector). Si el embed o el guardado fallan,
// no deja el hecho a medias en RAM.
func (m *FileMemory) Remember(text string) error {
	vec, err := m.embed(text)
	if err != nil {
		return fmt.Errorf("no pude embeber el hecho: %w", err)
	}
	m.facts = append(m.facts, Fact{Text: text, Vec: vec})
	if err := m.persist(); err != nil {
		m.facts = m.facts[:len(m.facts)-1] // revierto si no pude guardar
		return err
	}
	return nil
}

// Recall embebe la query y devuelve los k textos más parecidos por coseno. Sin hechos no
// llama a la red (devuelve nil).
func (m *FileMemory) Recall(query string, k int) ([]string, error) {
	if len(m.facts) == 0 {
		return nil, nil
	}
	q, err := m.embed(query)
	if err != nil {
		return nil, fmt.Errorf("no pude embeber la consulta: %w", err)
	}
	type scored struct {
		text  string
		score float32
	}
	ranked := make([]scored, len(m.facts))
	for i, f := range m.facts {
		ranked[i] = scored{f.Text, cosine(q, f.Vec)}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if k > len(ranked) {
		k = len(ranked)
	}
	out := make([]string, k)
	for i := 0; i < k; i++ {
		out[i] = ranked[i].text
	}
	return out, nil
}

// persist reescribe el archivo completo de forma atómica (temp + rename) para no corromper
// la memoria si se corta a mitad de escritura.
func (m *FileMemory) persist() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(m.facts)
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}
