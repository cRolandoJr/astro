# Astro · Fase G — Plan de Implementación (visión de pantalla)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Astro ve el monitor que le pidas (`grim -o`) y responde sobre él con un LLM multimodal (Gemini flash), solo cuando se lo pedís.

**Architecture:** `vision.go` (cliente HTTP multimodal, imagen base64). `llm.go` rutea un action `mirar` (como `open`/`recordar`): captura con `grim` → llamada de visión → habla el texto. El LLM elige el monitor desde la lista que `main` arma con `hyprctl monitors`. Interfaz `Interpreter` sin cambios.

**Tech Stack:** Go, stdlib. `grim` (captura, binario externo vía `Runner`), `hyprctl monitors -j` (lista), Gemini flash multimodal (OpenAI-compat). Spec: `docs/specs/2026-07-23-astro-fase-g-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; **solo stdlib**.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Nil-safe: `vision == nil` → `mirar` cae a fallback (sin panic). Interfaz `Interpret` sin cambios; voz/memoria/multi-turno/seguridad intactas.
- `mirar` es **stateless** (no entra al historial). Privacidad: solo el monitor pedido se manda a Gemini, solo al pedirlo.
- Cada tarea termina **compilando y con todos los tests verdes** (A/B/C/D/E/F incluidos).

---

### Task 1: `vision.go` — cliente de visión HTTP

**Files:** Create `vision.go`, `vision_test.go`
**Produces:** `type visionFunc func(question, imagePath string) (string, error)`; `httpVision(baseURL, apiKey, model string) visionFunc`.

- [ ] **Step 1: Tests que fallan (`vision_test.go`)**

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHttpVisionMandaImagenYParsea(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(img, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	var gotBody, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Veo una terminal con un error."}}]}`)
	}))
	defer srv.Close()

	ans, err := httpVision(srv.URL, "secreto", "gemini-flash-latest")("¿qué ves?", img)
	if err != nil {
		t.Fatal(err)
	}
	if ans != "Veo una terminal con un error." {
		t.Fatalf("respuesta mal parseada: %q", ans)
	}
	if gotAuth != "Bearer secreto" {
		t.Fatalf("faltó Authorization: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"type":"image_url"`) || !strings.Contains(gotBody, "data:image/png;base64,") {
		t.Fatalf("el body debía incluir la imagen base64; fue %s", gotBody)
	}
	if !strings.Contains(gotBody, "¿qué ves?") {
		t.Fatalf("el body debía incluir la pregunta; fue %s", gotBody)
	}
}

func TestHttpVisionErrorSiNoHayImagen(t *testing.T) {
	if _, err := httpVision("http://x", "k", "m")("q", "/no/existe.png"); err == nil {
		t.Fatal("esperaba error si la imagen no existe")
	}
}

func TestHttpVisionErrorHTTP(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "s.png")
	if err := os.WriteFile(img, []byte{1}, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := httpVision(srv.URL, "k", "m")("q", img); err == nil {
		t.Fatal("esperaba error en HTTP 403")
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestHttpVision` → FALLA.

- [ ] **Step 3: Implementar `vision.go`**

```go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// visionFunc mira una imagen y responde una pregunta sobre ella. Seam: real por HTTP o fake en tests.
type visionFunc func(question, imagePath string) (string, error)

// httpVision manda la imagen (base64) + la pregunta a un endpoint OpenAI-compatible multimodal
// (chat/completions con content de tipo image_url). Timeout más largo: la visión es lenta + sube imagen.
func httpVision(baseURL, apiKey, model string) visionFunc {
	client := &http.Client{Timeout: 40 * time.Second}
	return func(question, imagePath string) (string, error) {
		raw, err := os.ReadFile(imagePath)
		if err != nil {
			return "", fmt.Errorf("no pude leer la captura: %w", err)
		}
		dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "system", "content": "Sos Astro. Respondé BREVE (1-2 frases), en español, para leer en voz alta, sobre lo que ves en la imagen."},
				{"role": "user", "content": []any{
					map[string]string{"type": "text", "text": question},
					map[string]any{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
				}},
			},
		})
		url := strings.TrimRight(baseURL, "/") + "/chat/completions"
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("visión HTTP %d: %s", resp.StatusCode, string(out))
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil || len(parsed.Choices) == 0 {
			return "", fmt.Errorf("respuesta visión inesperada: %s", string(out))
		}
		return parsed.Choices[0].Message.Content, nil
	}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS.

- [ ] **Step 5: Commit**

```bash
git add vision.go vision_test.go
git commit -m "feat: httpVision — cliente multimodal (imagen base64 → chat/completions)"
```

---

### Task 2: `llm.go` — action `mirar` + `lookAction` + prompt

**Files:** Modify `llm.go`, `llm_test.go`
**Interfaces:**
- Consumes: `visionFunc` (Task 1).
- Produces: `LLMConfig.Vision`/`.Monitors`; ruteo de `mirar`; `lookAction(vision visionFunc, output, question string) *Action`.

- [ ] **Step 1: Tests que fallan (agregar a `llm_test.go`)**

```go
func TestLLMMirarCapturaYPregunta(t *testing.T) {
	fake := &fakeRunner{}
	var gotQuestion, gotPath string
	vision := func(question, imagePath string) (string, error) {
		gotQuestion, gotPath = question, imagePath
		return "Veo una terminal.", nil
	}
	acts := buildActions(time.Now)
	li := NewLLMInterpreter(LLMConfig{Chat: fakeChat(`{"action":"mirar","arg":"HDMI-A-1"}`, nil),
		Actions: acts, Fallback: NewRuleInterpreter(acts), Vision: vision})
	a, err := li.Interpret("mirá el de la derecha, ¿qué ves?")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Veo una terminal." {
		t.Fatalf("esperaba la respuesta de visión, fue %q", reply)
	}
	wantGrim := []string{"grim", "-o", "HDMI-A-1", "/tmp/astro-screen.png"}
	if !reflect.DeepEqual(fake.calls[0], wantGrim) {
		t.Fatalf("grim mal armado:\n esperaba %v\n fue      %v", wantGrim, fake.calls[0])
	}
	if gotQuestion != "mirá el de la derecha, ¿qué ves?" {
		t.Fatalf("la pregunta a visión debía ser la frase del usuario, fue %q", gotQuestion)
	}
	if gotPath != "/tmp/astro-screen.png" {
		t.Fatalf("path a visión: %q", gotPath)
	}
	if len(li.history) != 0 {
		t.Fatalf("mirar es stateless, no debe registrar historial; fue %v", li.history)
	}
}

func TestLLMMirarSinOutputCapturaTodo(t *testing.T) {
	fake := &fakeRunner{}
	vision := func(question, imagePath string) (string, error) { return "ok", nil }
	acts := buildActions(time.Now)
	li := NewLLMInterpreter(LLMConfig{Chat: fakeChat(`{"action":"mirar","arg":""}`, nil),
		Actions: acts, Fallback: NewRuleInterpreter(acts), Vision: vision})
	a, _ := li.Interpret("mirá la pantalla")
	if _, err := a.Run(fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"grim", "/tmp/astro-screen.png"} // sin -o
	if !reflect.DeepEqual(fake.calls[0], want) {
		t.Fatalf("sin monitor → grim sin -o; esperaba %v, fue %v", want, fake.calls[0])
	}
}

func TestLLMMirarSinVisionVaAFallback(t *testing.T) {
	// sin Vision configurado → 'mirar' cae al fallback (no promete lo que no puede)
	a, err := newLLM(fakeChat(`{"action":"mirar","arg":"","say":"ok"}`, nil)).Interpret("mirá la pantalla")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("sin visión, mirar debe caer al fallback; a=%v err=%v", a, err)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestLLMMirar` → FALLA (`Vision` no existe; `mirar` no ruteado).

- [ ] **Step 3: Modificar `llm.go`**

3a. En `LLMConfig` agregá dos campos:
```go
	Vision   visionFunc // opcional (nil = sin visión)
	Monitors string     // lista de monitores para el prompt (vacía = sin resolución de nombres)
```
En el struct `LLMInterpreter` agregá:
```go
	vision   visionFunc
	monitors string
```
En `NewLLMInterpreter`, en el `return &LLMInterpreter{…}` agregá: `vision: cfg.Vision, monitors: cfg.Monitors,`.

3b. En `Interpret`, agregá el ruteo de `mirar` **antes** del bloque `if choice.Action == "open"`:
```go
	if choice.Action == "mirar" && li.vision != nil {
		return lookAction(li.vision, choice.Arg, text), nil // stateless: no se registra en historial
	}
```

3c. En `systemPrompt`, después del bloque `if li.mem != nil { … recordar … }` (y antes del `return`):
```go
	if li.vision != nil {
		b.WriteString("- mirar (arg = nombre del monitor; vacío = el enfocado): mirá la pantalla y respondé sobre lo que hay\n")
		if li.monitors != "" {
			b.WriteString("Monitores (elegí el name; x menor = más a la izquierda; si no aclarás, el enfocado): " + li.monitors + "\n")
		}
	}
```

3d. Agregá `lookAction` al final de `llm.go` (junto a `memoryWriteAction`):
```go
// lookAction captura un monitor con grim y le pregunta a la visión sobre la imagen.
func lookAction(vision visionFunc, output, question string) *Action {
	return &Action{Name: "mirar", Face: Neutral,
		Run: func(r Runner) (string, error) {
			const path = "/tmp/astro-screen.png"
			args := []string{"-o", output, path}
			if output == "" {
				args = []string{path} // sin -o: toda la pantalla (fallback)
			}
			if _, err := r.Run("grim", args...); err != nil {
				return "", fmt.Errorf("no pude capturar la pantalla (¿grim?): %w", err)
			}
			return vision(question, path)
		}}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS
(nuevos + A/B/C/D/E/F intactos: los tests sin `Vision` no rutean `mirar`, `main.go` compila con los campos nuevos sin setear).

- [ ] **Step 5: Commit**

```bash
git add llm.go llm_test.go
git commit -m "feat: action 'mirar' — grim -o <monitor> + visión; ruteo nil-safe, stateless"
```

---

### Task 3: `main.go` + `run.sh` — monitores, wiring de visión + E2E

**Files:** Modify `main.go`, `run.sh`

- [ ] **Step 1: `main.go` — lista de monitores + wiring**

1a. Agregá `"encoding/json"` a los imports de `main.go`.

1b. En la rama LLM (`ASTRO_BRAIN != "rules"`), después de armar `embed`/`buildMemory`, agregá visión y monitores al `LLMConfig`:
```go
		vision := httpVision(llmURL, llmKey, envOr("ASTRO_VISION_MODEL", envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")))
		interpreter = NewLLMInterpreter(LLMConfig{
			Chat: httpChat(llmURL, llmKey, envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			Actions: acts, Fallback: rules, Mem: buildMemory(embed), TopK: topK, Now: time.Now,
			HistoryTurns: historyTurns, IdleWindow: time.Duration(idleMin) * time.Minute,
			Vision: vision, Monitors: buildMonitorsPrompt(runner),
		})
```
(mantené las líneas `idleMin`/`historyTurns` que ya están arriba de ese bloque).

1c. Agregá el helper (junto a `buildMemory`):
```go
// buildMonitorsPrompt lee los monitores de Hyprland para que el LLM resuelva "el de la derecha" →
// nombre de salida. Si falla (no Hyprland / no hyprctl), devuelve "" (mirar captura toda la pantalla).
func buildMonitorsPrompt(r Runner) string {
	out, err := r.Run("hyprctl", "monitors", "-j")
	if err != nil {
		return ""
	}
	var mons []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		X           int    `json:"x"`
		Focused     bool   `json:"focused"`
	}
	if err := json.Unmarshal([]byte(out), &mons); err != nil {
		return ""
	}
	var parts []string
	for _, m := range mons {
		p := fmt.Sprintf("%s — %s, x=%d", m.Name, m.Description, m.X)
		if m.Focused {
			p += " (enfocado)"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}
```

- [ ] **Step 2: `run.sh` — agregar grim al nix shell**

En la línea `exec nix shell …`, agregá `nixpkgs#grim`:
```sh
exec nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#sox nixpkgs#grim nixpkgs#go \
  --command go run .
```

- [ ] **Step 3: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 4: E2E (manual — necesita la key + pantalla; lo corre el usuario)**

```bash
cd ~/projects/astro && git switch fase-g
astro
# decir: "mirá la pantalla y decime qué ves"
# (multi-monitor) "mirá el de la derecha, ¿qué hay?"
```
Expected: captura + describe lo que hay; con varios monitores, captura el pedido; sin aclarar, el enfocado.
NOTA: si `/chat/completions` con imagen da error de modelo, probar otro en `ASTRO_VISION_MODEL` (mismo patrón de volatilidad que el chat).

- [ ] **Step 5: Commit**

```bash
git add main.go run.sh
git commit -m "feat: wiring de visión (httpVision + monitores de hyprctl) en main; grim en run.sh"
```

---

## Definition of Done (Fase G)

1. `go build/vet/test ./...` verdes (httpVision con httptest, ruteo `mirar` con grim+visión fake, sin-output→captura todo, nil-safe sin visión; + A/B/C/D/E/F intactos).
2. E2E: describe la pantalla pedida; elige monitor por voz; default al enfocado.
3. Interfaz `Interpret` / voz / memoria / multi-turno / seguridad intactas; `mirar` stateless.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Cobertura del spec:** §4 `vision.go` → Task 1; ruteo `mirar` + `lookAction` + prompt → Task 2; monitores + wiring + grim → Task 3.
- **Compila en cada tarea:** Task 1 archivos nuevos; Task 2 agrega campos a `LLMConfig` (main compila sin setearlos, valen cero); Task 3 los setea. Nada roto entre tareas.
- **Nil-safe:** `TestLLMMirarSinVisionVaAFallback` cubre `vision==nil`→fallback; `buildMonitorsPrompt` devuelve `""` ante fallo (sin panic).
- **Stateless:** `TestLLMMirarCapturaYPregunta` verifica `len(li.history)==0` tras un `mirar`.
- **Anti-verde-falso:** el test de `httpVision` afirma que el body lleva `image_url` + base64 + la pregunta (no solo "no crashea"); el de ruteo verifica el comando `grim` real y que la pregunta = la frase del usuario.
- **Privacidad/seguridad:** `mirar` solo lee y habla; no ejecuta nada; captura solo el monitor pedido.
- **Tipos consistentes:** `visionFunc` (Task 1) usado con la misma firma en `LLMConfig`/`lookAction`/tests.
