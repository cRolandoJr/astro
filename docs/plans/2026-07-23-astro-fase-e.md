# Astro · Fase E — Plan de Implementación (memoria semántica persistente)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Astro recuerda hechos entre sesiones (`recordar "X"`) y los usa al responder, recuperándolos por
similitud coseno (RAG en Go puro, embeddings de Gemini).

**Architecture:** `memory.go` (tipos + `cosine` + `FileMemory`) es el corazón; `embed.go` habla con la API
de embeddings; `LLMInterpreter` recupera top-K y los inyecta al `systemPrompt`, y rutea el action `recordar`.
La interfaz `Interpreter` y `MemoryStore` (seam para futura migración a Chroma) no cambian el resto del daemon.

**Tech Stack:** Go, stdlib. Embeddings: Gemini `text-embedding-004` (endpoint OpenAI-compat, misma key que Fase C).
Spec: `docs/specs/2026-07-23-astro-fase-e-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; **solo stdlib**.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Nil-safe: `mem == nil` (ej. `ASTRO_BRAIN=rules`) → sin memoria, sin panic. Errores anti-verde-falso (log a stderr).
- No re-embeber al arranque: el vector se persiste junto al texto.
- Cada tarea termina **compilando y con todos los tests verdes** (incluidos A/B/C/D).

---

### Task 1: `memory.go` — tipos + `cosine`

**Files:** Create `memory.go`, `memory_test.go`
**Produces:** `Fact`, `MemoryStore` interface, `embedFunc`, `cosine(a, b []float32) float32`.

- [ ] **Step 1: Tests que fallan (`memory_test.go`)**

```go
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
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestCosine` → FALLA (cosine no existe).

- [ ] **Step 3: Implementar `memory.go`**

```go
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
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./... -run TestCosine'` → PASS.

- [ ] **Step 5: Commit**

```bash
git add memory.go memory_test.go
git commit -m "feat: memoria — tipos Fact/MemoryStore/embedFunc + cosine"
```

---

### Task 2: `FileMemory` — cargar / Remember / Recall

**Files:** Modify `memory.go`, `memory_test.go`
**Interfaces:**
- Consumes: `Fact`, `embedFunc`, `cosine` (Task 1).
- Produces: `NewFileMemory(path string, embed embedFunc) (*FileMemory, error)`; métodos `Remember(text string) error`, `Recall(query string, k int) ([]string, error)` (implementa `MemoryStore`).

- [ ] **Step 1: Tests que fallan (agregar a `memory_test.go`)**

Agregá los imports `"fmt"`, `"os"`, `"path/filepath"`, `"reflect"` al bloque de imports del test (junto a `"testing"`), y:

```go
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
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestFileMemory` → FALLA (NewFileMemory no existe).

- [ ] **Step 3: Implementar `FileMemory` en `memory.go`**

Agregá los imports `"encoding/json"`, `"fmt"`, `"os"`, `"path/filepath"`, `"sort"` a `memory.go` (junto al `"math"` ya presente), y:

```go
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
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS (Task 1 + Task 2 + previos).

- [ ] **Step 5: Commit**

```bash
git add memory.go memory_test.go
git commit -m "feat: FileMemory — persistencia JSON (texto+vector) + Recall por coseno"
```

---

### Task 3: `embed.go` — cliente de embeddings HTTP

**Files:** Create `embed.go`, `embed_test.go`
**Interfaces:**
- Produces: `httpEmbed(baseURL, apiKey, model string) embedFunc`.

- [ ] **Step 1: Tests que fallan (`embed_test.go`)**

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHttpEmbedParseaYAutoriza(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer srv.Close()

	vec, err := httpEmbed(srv.URL, "secreto", "text-embedding-004")("hola")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 3 || vec[0] != 0.1 {
		t.Fatalf("vector mal parseado: %v", vec)
	}
	if gotAuth != "Bearer secreto" {
		t.Fatalf("faltó Authorization: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"input":"hola"`) || !strings.Contains(gotBody, `"model":"text-embedding-004"`) {
		t.Fatalf("body inesperado: %s", gotBody)
	}
}

func TestHttpEmbedErrorHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := httpEmbed(srv.URL, "k", "m")("x"); err == nil {
		t.Fatal("esperaba error en HTTP 403")
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestHttpEmbed` → FALLA.

- [ ] **Step 3: Implementar `embed.go`**

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpEmbed devuelve un embedFunc que pega al endpoint OpenAI-compatible /embeddings.
// Mismo patrón que httpChat: net/http, sin SDK.
func httpEmbed(baseURL, apiKey, model string) embedFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(text string) ([]float32, error) {
		body, _ := json.Marshal(map[string]any{"model": model, "input": text})
		url := strings.TrimRight(baseURL, "/") + "/embeddings"
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("embeddings HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Data) == 0 {
			return nil, fmt.Errorf("respuesta embeddings inesperada: %s", string(raw))
		}
		return out.Data[0].Embedding, nil
	}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS.

- [ ] **Step 5: Commit**

```bash
git add embed.go embed_test.go
git commit -m "feat: httpEmbed — cliente de embeddings OpenAI-compat"
```

---

### Task 4: `llm.go` — recuperar, inyectar al prompt y rutear `recordar`

**Files:** Modify `llm.go`, `llm_test.go`, `main.go` (solo para que compile)
**Interfaces:**
- Consumes: `MemoryStore` (Task 1).
- Produces: `NewLLMInterpreter(chat chatFunc, actions map[string]*Action, fallback Interpreter, mem MemoryStore, topK int) *LLMInterpreter`; `memoryWriteAction(mem MemoryStore, text string) *Action`.

- [ ] **Step 1: Tests que fallan (agregar a `llm_test.go`)**

Agregá `"strings"` al bloque de imports de `llm_test.go`, y:

```go
// fakeMemory: MemoryStore de test. Registra lo guardado y devuelve un Recall fijo.
type fakeMemory struct {
	remembered []string
	recall     []string
	recallErr  error
}

func (f *fakeMemory) Remember(text string) error {
	f.remembered = append(f.remembered, text)
	return nil
}
func (f *fakeMemory) Recall(query string, k int) ([]string, error) {
	return f.recall, f.recallErr
}

func newLLMWithMem(chat chatFunc, mem MemoryStore) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(chat, acts, NewRuleInterpreter(acts), mem, 5)
}

func TestLLMRecordarGuardaElHecho(t *testing.T) {
	mem := &fakeMemory{}
	a, err := newLLMWithMem(fakeChat(`{"action":"recordar","arg":"uso NixOS","say":"Dale, anotado."}`, nil), mem).
		Interpret("acordate que uso NixOS")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(&fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Dale, anotado." {
		t.Fatalf("esperaba el say, fue %q", reply)
	}
	if len(mem.remembered) != 1 || mem.remembered[0] != "uso NixOS" {
		t.Fatalf("esperaba guardar 'uso NixOS', fue %v", mem.remembered)
	}
}

func TestLLMRecordarSinMemVaAFallback(t *testing.T) {
	// mem nil → 'recordar' no disponible → fallback (reglas → ErrNoEntiendo).
	a, err := newLLM(fakeChat(`{"action":"recordar","arg":"algo","say":"ok"}`, nil)).Interpret("xyzzy")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("sin memoria, recordar debe caer al fallback; a=%v err=%v", a, err)
	}
}

func TestLLMInyectaHechosAlPrompt(t *testing.T) {
	mem := &fakeMemory{recall: []string{"el usuario usa NixOS"}}
	var seenSystem string
	chat := func(system, user string) (string, error) {
		seenSystem = system
		return `{"action":"none","say":"ok"}`, nil
	}
	if _, err := newLLMWithMem(chat, mem).Interpret("qué distro uso"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenSystem, "el usuario usa NixOS") {
		t.Fatalf("el prompt debía incluir el hecho recuperado; prompt=%q", seenSystem)
	}
}

func TestLLMRecallErrorNoRompeElTurno(t *testing.T) {
	mem := &fakeMemory{recallErr: fmt.Errorf("embed caído")}
	a, err := newLLMWithMem(fakeChat(`{"action":"none","say":"hola"}`, nil), mem).Interpret("hola")
	if err != nil || a == nil {
		t.Fatalf("un fallo de Recall no debe romper el turno; a=%v err=%v", a, err)
	}
}

// #3 (mentor): recordar con arg vacío no debe guardar un hecho vacío (no matchea → fallback).
func TestLLMRecordarArgVacioNoGuarda(t *testing.T) {
	mem := &fakeMemory{}
	_, _ = newLLMWithMem(fakeChat(`{"action":"recordar","arg":"","say":"ok"}`, nil), mem).Interpret("acordate")
	if len(mem.remembered) != 0 {
		t.Fatalf("no debe guardar un hecho vacío; guardó %v", mem.remembered)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestLLMRecordar` → FALLA (compila mal: firma vieja de NewLLMInterpreter).

- [ ] **Step 3: Modificar `llm.go`**

3a. Struct + constructor:
```go
type LLMInterpreter struct {
	chat     chatFunc
	actions  map[string]*Action
	fallback Interpreter
	mem      MemoryStore // opcional (nil = sin memoria)
	topK     int
}

func NewLLMInterpreter(chat chatFunc, actions map[string]*Action, fallback Interpreter, mem MemoryStore, topK int) *LLMInterpreter {
	if topK <= 0 {
		topK = 5
	}
	return &LLMInterpreter{chat: chat, actions: actions, fallback: fallback, mem: mem, topK: topK}
}
```

3b. `Interpret` — recuperar al inicio, pasar `facts` al prompt, y rutear `recordar`. Reemplazá el cuerpo entero por:
```go
func (li *LLMInterpreter) Interpret(text string) (*Action, error) {
	var facts []string
	if li.mem != nil {
		f, err := li.mem.Recall(text, li.topK)
		if err != nil { // recuperar falló → sigo sin hechos (no rompo el turno)
			fmt.Fprintln(os.Stderr, "(memoria falló al recuperar, sigo sin hechos:", err, ")")
		} else {
			facts = f
		}
	}
	reply, err := li.chat(li.systemPrompt(facts), text)
	if err != nil {
		fmt.Fprintln(os.Stderr, "(cerebro LLM falló, uso reglas:", err, ")")
		return li.fallback.Interpret(text)
	}
	var choice struct {
		Action string `json:"action"`
		Arg    string `json:"arg"`
		Say    string `json:"say"`
	}
	if jerr := json.Unmarshal([]byte(extractJSON(reply)), &choice); jerr != nil {
		fmt.Fprintln(os.Stderr, "(respuesta LLM no-JSON, uso reglas:", reply, ")")
		return li.fallback.Interpret(text)
	}
	if (choice.Action == "" || choice.Action == "none") && choice.Say != "" {
		return sayAction(choice.Say), nil
	}
	if choice.Action == "recordar" && li.mem != nil && choice.Arg != "" {
		return wrap(memoryWriteAction(li.mem, choice.Arg), choice.Say), nil
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text)
		}
		return wrap(openAppAction(choice.Arg), choice.Say), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		return wrap(a, choice.Say), nil
	}
	return li.fallback.Interpret(text)
}
```

3c. `systemPrompt` — ahora recibe `facts`, inyecta el bloque de hechos y menciona `recordar` si hay memoria. Reemplazá la firma y el cuerpo por:
```go
// systemPrompt arma el prompt: instrucciones + hechos recuperados (si hay) + menú (orden estable).
func (li *LLMInterpreter) systemPrompt(facts []string) string {
	var b strings.Builder
	b.WriteString("Sos Astro, un asistente de escritorio con voz. El usuario te habla en español. ")
	b.WriteString("Respondé SOLO un JSON: {\"action\":\"<opcional>\",\"arg\":\"<opcional>\",\"say\":\"<respuesta hablada>\"}. ")
	b.WriteString("Si es un COMANDO, elegí un `action` del menú y un `say` corto de confirmación. ")
	b.WriteString("Si es CHARLA o una pregunta, usá action:\"none\" y contestá en `say`. ")
	b.WriteString("El `say` se lee en voz alta: que sea BREVE (1-2 frases), natural y en español.\n")
	if len(facts) > 0 {
		b.WriteString("Esto es lo que sé del usuario (usalo si viene al caso):\n")
		for _, f := range facts {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("Menú:\n")
	names := make([]string, 0, len(li.actions))
	for n := range li.actions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %s\n", n, li.actions[n].Desc)
	}
	b.WriteString("- open (arg = nombre de la app): abrir una app o programa\n")
	if li.mem != nil {
		b.WriteString("- recordar (arg = el hecho a recordar): guardá algo que el usuario te pide recordar\n")
	}
	return b.String()
}
```

3d. Agregá `memoryWriteAction` al final de `llm.go`:
```go
// memoryWriteAction guarda 'text' en la memoria. No toca el Runner (como sayAction); el say
// hablado lo pone wrap() con lo que redactó el LLM.
func memoryWriteAction(mem MemoryStore, text string) *Action {
	return &Action{Name: "recordar", Face: Feliz,
		Run: func(r Runner) (string, error) {
			if err := mem.Remember(text); err != nil {
				return "", err
			}
			return "", nil
		}}
}
```

- [ ] **Step 4: Actualizar callers para que compile**

En `llm_test.go`, el helper `newLLM` (Fase C) pasa `nil, 5`:
```go
func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(chat, acts, NewRuleInterpreter(acts), nil, 5)
}
```
En `main.go`, la llamada de `NewLLMInterpreter` (interina — Task 5 le enchufa la memoria real):
```go
		interpreter = NewLLMInterpreter(
			httpChat(os.Getenv("ASTRO_LLM_URL"), os.Getenv("ASTRO_LLM_KEY"),
				envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			acts, rules, nil, 5)
```

- [ ] **Step 5: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS (nuevos + A/B/C/D intactos: los tests de Fase D sin `say` y de Fase C siguen verdes porque el ruteo previo no cambió).

- [ ] **Step 6: Commit**

```bash
git add llm.go llm_test.go main.go
git commit -m "feat: LLMInterpreter recupera hechos al prompt + rutea 'recordar' (mem nil-safe)"
```

---

### Task 5: `main.go` — armar la memoria real + E2E

**Files:** Modify `main.go`

- [ ] **Step 1: Implementar `buildMemory` y enchufarla**

Agregá `"path/filepath"` a los imports de `main.go`. Reemplazá el bloque `if os.Getenv("ASTRO_BRAIN") != "rules"` por:
```go
	if os.Getenv("ASTRO_BRAIN") != "rules" { // default: cerebro LLM (con fallback a reglas)
		llmURL := os.Getenv("ASTRO_LLM_URL")
		llmKey := os.Getenv("ASTRO_LLM_KEY")
		embed := httpEmbed(llmURL, llmKey, envOr("ASTRO_EMBED_MODEL", "text-embedding-004"))
		topK, _ := strconv.Atoi(os.Getenv("ASTRO_MEM_TOPK"))
		interpreter = NewLLMInterpreter(
			httpChat(llmURL, llmKey, envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			acts, rules, buildMemory(embed), topK)
	}
```
Y agregá el helper (junto a `envOr`):
```go
// buildMemory arma la memoria persistente. Si el archivo no se puede cargar, loguea y devuelve
// nil (Astro sigue funcionando, sin memoria) — nunca aborta el daemon.
func buildMemory(embed embedFunc) MemoryStore {
	path := os.Getenv("ASTRO_MEM_PATH")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "(sin HOME, memoria off:", err, ")")
			return nil
		}
		path = filepath.Join(home, ".local", "share", "astro", "memory.json")
	}
	mem, err := NewFileMemory(path, embed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "(no pude cargar la memoria, sigo sin ella:", err, ")")
		return nil
	}
	return mem
}
```

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 3: E2E con Gemini (manual — necesita la key; lo corre el usuario)**

```bash
export ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai
export ASTRO_LLM_KEY='TU_KEY'
export ASTRO_LLM_MODEL=gemini-flash-latest
cd ~/projects/astro
# 1) guardar un hecho
printf 'acordate que uso NixOS y odio el purple\n' | ASTRO_INPUT=stdin nix shell nixpkgs#go --command go run .
cat ~/.local/share/astro/memory.json   # debe tener {text, vector[768]}
# 2) en otra corrida, preguntar algo relacionado
printf 'qué colores no me gustan?\n' | ASTRO_INPUT=stdin nix shell nixpkgs#go --command go run .
```
Expected: (1) persiste el hecho con su vector; (2) Astro responde mencionando que odiás el purple (recuperado por coseno, sin re-embeber al boot). NOTA: si `/embeddings` da 404 por el nombre del modelo, probar `ASTRO_EMBED_MODEL=gemini-embedding-001` (mismo patrón que el `gemini-flash-latest` de Fase C).

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wiring de memoria persistente (buildMemory + embeddings) en main"
```

---

## Definition of Done (Fase E)

1. `go build/vet/test ./...` verdes (cosine, FileMemory Remember/Recall/round-trip/corrupto/vacío, httpEmbed, ruteo `recordar`, inyección de hechos; + A/B/C/D intactos).
2. E2E: `recordar "X"` persiste en `memory.json` (texto+vector); reiniciar y preguntar algo relacionado → Astro responde usando el hecho.
3. Nil-safe (`ASTRO_BRAIN=rules` → sin memoria); fallback/seguridad previos intactos.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Cobertura del spec:** §3 componentes → Tasks 1-4; §4 persistencia → Task 2 (`persist` atómico); §5 embeddings → Task 3; wiring §3 `main` → Task 5; §6 errores → tests de corrupto/vacío/Recall-error + guard de `main` para `Remember` error.
- **Compila en cada tarea:** Task 4 actualiza el helper `newLLM` y la llamada de `main.go` a la firma nueva (interina `nil, 5`) → nada queda roto entre tareas; Task 5 enchufa la memoria real.
- **Sin re-embeber al boot:** `TestFileMemoryPersisteSinReembebir` lo verifica por conteo de llamadas (aserción real).
- **Nil-safe:** `TestLLMRecordarSinMemVaAFallback` + guardas `li.mem != nil` en Interpret/systemPrompt; `buildMemory` devuelve `nil` de interfaz (no typed-nil) ante error.
- **Anti-verde-falso:** Recall-error loguea y sigue (`TestLLMRecallErrorNoRompeElTurno`); tests no inventan infra (usan `fakeEmbed`/`fakeMemory`/`httptest`).
- **Tipos consistentes:** `embedFunc`/`MemoryStore` definidos en Task 1 y usados con las mismas firmas en Tasks 2-5.
