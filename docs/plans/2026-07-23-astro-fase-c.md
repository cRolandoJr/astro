# Astro · Fase C — Plan de Implementación (el cerebro: LLM entiende lenguaje natural)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Reemplazar el intérprete de reglas por un **`LLMInterpreter`** (misma interfaz) que, dado tu
texto en lenguaje natural, le pregunta a un LLM (OpenAI-compat, JSON mode) cuál de **las acciones que ya
existen** usar. Con **fallback a las reglas** si el LLM falla, y **seguridad intacta** (`isSafeAppName`).

**Architecture:** `LLMInterpreter{chat, actions, fallback}` implementa `Interpret`. El `chat` es un seam
inyectable (real por HTTP / fake en tests). El resto (acciones, cara, voz, chirps) no se toca.

**Tech Stack:** Go 1.26, `net/http` + `encoding/json` (stdlib, sin SDK externo). LLM: Gemini free tier
(OpenAI-compat) por default, model-agnostic por env. Spec: `docs/specs/2026-07-23-astro-fase-c-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nombres descriptivos.
- Nada destructivo; **solo stdlib**. Módulo `github.com/cRolandoJr/astro`, raíz del repo.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Seguridad: el LLM solo elige del menú; `open` pasa por `isSafeAppName`. La key va por env, **nunca al repo**.

## Preparación (para el E2E de la Task 3 — el código/tests NO la necesitan)

Key gratis de Gemini: https://aistudio.google.com/apikey (sin tarjeta). Envs:
```
ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai
ASTRO_LLM_KEY=AIza...        # tu key
ASTRO_LLM_MODEL=gemini-2.5-flash
```

---

### Task 1: `Desc` en `Action` (el menú que ve el LLM)

**Files:** Modify `action.go`, `actions.go`; add test in `actions_test.go`.
**Produces:** `Action` gana campo `Desc string`; cada acción del registro tiene una `Desc` no vacía.

- [ ] **Step 1: Test que falla (`actions_test.go`)**

```go
func TestAccionesTienenDescripcion(t *testing.T) {
	for name, a := range buildActions(time.Now) {
		if a.Desc == "" {
			t.Errorf("acción %q sin Desc (la necesita el menú del LLM)", name)
		}
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestAccionesTienenDescripcion` → FALLA (no existe `Desc` / vacío).

- [ ] **Step 3: Agregar `Desc` a `action.go`**

```go
type Action struct {
	Name string
	Desc string // qué hace, en una línea — para el menú que ve el LLM
	Face Expression
	Run  func(r Runner) (reply string, err error)
}
```

- [ ] **Step 4: Completar `Desc` en `actions.go`**

En `buildActions`, agregá `Desc` a cada acción:
```go
	add(&Action{Name: "saludar", Desc: "saludar o responder un saludo", Face: Feliz,
		Run: func(r Runner) (string, error) { return "¡Hola! Soy Astro.", nil }})
	add(&Action{Name: "hora", Desc: "decir la hora actual", Face: Neutral,
		Run: func(r Runner) (string, error) { return "Son las " + now().Format("15:04") + ".", nil }})
	add(&Action{Name: "dormir", Desc: "poner a Astro a dormir", Face: Dormido,
		Run: func(r Runner) (string, error) { return "Me duermo… 💤", nil }})
	add(&Action{Name: "despertar", Desc: "despertar a Astro", Face: Neutral,
		Run: func(r Runner) (string, error) { return "¡Ya estoy despierto!", nil }})
```
Y en `addDesktopActions`:
```go
	add(&Action{Name: "pausar", Desc: "pausar o reanudar la música o el video", Face: Feliz, Run: playerctl("play-pause", "Listo.")})
	add(&Action{Name: "siguiente", Desc: "pasar a la siguiente pista", Face: Feliz, Run: playerctl("next", "Siguiente.")})
	add(&Action{Name: "subir_volumen", Desc: "subir el volumen", Face: Neutral, Run: volume("5%+", "Subí el volumen.")})
	add(&Action{Name: "bajar_volumen", Desc: "bajar el volumen", Face: Neutral, Run: volume("5%-", "Bajé el volumen.")})
	add(&Action{Name: "mute", Desc: "silenciar o quitar el silencio", Face: Neutral, Run: func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"); err != nil {
			return "", fmt.Errorf("no pude silenciar: %w", err)
		}
		return "Mute.", nil
	}})
```
(Y en `openAppAction`, agregá `Desc: "abrir " + app` por consistencia — aunque `open` se describe aparte en el menú.)

- [ ] **Step 5: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./...` → PASS (nuevo + todos los previos).

- [ ] **Step 6: Commit**

```bash
git add action.go actions.go actions_test.go
git commit -m "feat: Action.Desc — descripción de cada acción para el menú del LLM"
```

---

### Task 2: `LLMInterpreter` (lógica + parseo + fallback, con chat fake)

**Files:** Create `llm.go`, `llm_test.go`
**Produces:** `chatFunc`; `LLMInterpreter{chat, actions, fallback}`; `NewLLMInterpreter(...)`; helpers `systemPrompt`, `extractJSON`.

- [ ] **Step 1: Tests que fallan (`llm_test.go`)**

```go
package main

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func fakeChat(reply string, err error) chatFunc {
	return func(system, user string) (string, error) { return reply, err }
}

func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(chat, acts, NewRuleInterpreter(acts))
}

func TestLLMMapeaAccionDelRegistro(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"pausar"}`, nil)).Interpret("che poné pausa")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba pausar; a=%v err=%v", a, err)
	}
}

func TestLLMOpenAbreLaApp(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox"}`, nil)).Interpret("abrí el navegador")
	if err != nil || a == nil {
		t.Fatalf("esperaba acción; a=%v err=%v", a, err)
	}
	if _, err := a.Run(fake); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

// Seguridad: aunque el LLM devuelva un arg peligroso, isSafeAppName lo frena → fallback → ErrNoEntiendo.
func TestLLMOpenInyeccionRechazada(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox; rm -rf ~"}`, nil)).Interpret("abrí firefox; rm -rf ~")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("el arg peligroso debía rechazarse; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiChatFalla(t *testing.T) {
	a, err := newLLM(fakeChat("", fmt.Errorf("sin red"))).Interpret("pausá")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba fallback a reglas → pausar; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiJSONInvalido(t *testing.T) {
	a, err := newLLM(fakeChat("no soy json", nil)).Interpret("hola")
	if err != nil || a == nil || a.Name != "saludar" {
		t.Fatalf("esperaba fallback → saludar; a=%v err=%v", a, err)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestLLM` → FALLA (no existe `LLMInterpreter`).

- [ ] **Step 3: `llm.go` (la lógica; el chat real va en Task 3)**

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// chatFunc habla con el LLM: recibe prompt de sistema + texto del usuario, devuelve la
// respuesta cruda (el JSON). Es un seam: real por HTTP (Task 3) o fake en tests.
type chatFunc func(system, user string) (string, error)

// LLMInterpreter elige una acción del registro usando un LLM. Si el LLM falla o devuelve
// algo raro, cae al 'fallback' (el RuleInterpreter). Implementa Interpreter.
type LLMInterpreter struct {
	chat     chatFunc
	actions  map[string]*Action
	fallback Interpreter
}

func NewLLMInterpreter(chat chatFunc, actions map[string]*Action, fallback Interpreter) *LLMInterpreter {
	return &LLMInterpreter{chat: chat, actions: actions, fallback: fallback}
}

func (li *LLMInterpreter) Interpret(text string) (*Action, error) {
	reply, err := li.chat(li.systemPrompt(), text)
	if err != nil {
		// Logueamos: un fallback silencioso daría "verde falso" (las reglas rescatan
		// comandos con keyword y no te enterarías de que el LLM está muerto).
		fmt.Fprintln(os.Stderr, "(cerebro LLM falló, uso reglas:", err, ")")
		return li.fallback.Interpret(text)
	}
	var choice struct {
		Action string `json:"action"`
		Arg    string `json:"arg"`
	}
	if jerr := json.Unmarshal([]byte(extractJSON(reply)), &choice); jerr != nil {
		fmt.Fprintln(os.Stderr, "(respuesta LLM no-JSON, uso reglas:", reply, ")")
		return li.fallback.Interpret(text)
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // arg peligroso → no por acá
		}
		return openAppAction(choice.Arg), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		return a, nil
	}
	return li.fallback.Interpret(text) // acción desconocida / "none" → reglas
}

// systemPrompt arma el menú de acciones para el LLM (orden estable).
func (li *LLMInterpreter) systemPrompt() string {
	var b strings.Builder
	b.WriteString("Sos Astro, un asistente de escritorio. El usuario te habla en español. ")
	b.WriteString("Elegí UNA acción de la lista según lo que pide. Respondé SOLO un JSON: ")
	b.WriteString(`{"action":"<nombre>","arg":"<opcional>"}`)
	b.WriteString(". Si ninguna aplica, usá \"none\". Acciones:\n")
	names := make([]string, 0, len(li.actions))
	for n := range li.actions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "- %s: %s\n", n, li.actions[n].Desc)
	}
	b.WriteString("- open (arg = nombre de la app): abrir una app o programa\n")
	return b.String()
}

// extractJSON toma el primer objeto {...} del texto (por si el modelo mete texto alrededor).
func extractJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j < i {
		return s
	}
	return s[i : j+1]
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -run TestLLM -v 2>&1 | grep -E "^(--- |ok|FAIL)"` → todos PASS. Y `go test ./...` completo verde.

- [ ] **Step 5: Commit**

```bash
git add llm.go llm_test.go
git commit -m "feat: LLMInterpreter — menú + JSON + mapeo a acciones + fallback a reglas (seguridad intacta)"
```

---

### Task 3: chat real (HTTP) + wiring en `main`

**Files:** Modify `llm.go` (agregar `httpChat`), `main.go`
**Produces:** `httpChat(baseURL, apiKey, model) chatFunc`; `main` elige intérprete por `ASTRO_BRAIN` y aplica el guard de error genérico.

- [ ] **Step 1: Agregar `httpChat` a `llm.go`**

Concepto: POST a un endpoint OpenAI-compat. Solo stdlib (`net/http`). No lleva test unitario (es I/O);
se valida con build + el E2E. Agregá los imports `bytes`, `io`, `net/http`, `time`.

```go
// httpChat devuelve un chatFunc que pega a un endpoint OpenAI-compatible (chat completions).
func httpChat(baseURL, apiKey, model string) chatFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(system, user string) (string, error) {
		body, _ := json.Marshal(map[string]any{
			"model":           model,
			"temperature":     0,
			"response_format": map[string]string{"type": "json_object"},
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
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
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, string(raw))
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
			return "", fmt.Errorf("respuesta LLM inesperada: %s", string(raw))
		}
		return out.Choices[0].Message.Content, nil
	}
}
```

- [ ] **Step 2: Wiring en `main.go`**

Reemplazá la línea del intérprete:
```go
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))
```
por:
```go
	acts := buildActions(time.Now)
	rules := NewRuleInterpreter(acts)
	var interpreter Interpreter = rules
	if os.Getenv("ASTRO_BRAIN") != "rules" { // default: cerebro LLM (con fallback a reglas)
		interpreter = NewLLMInterpreter(
			httpChat(os.Getenv("ASTRO_LLM_URL"), os.Getenv("ASTRO_LLM_KEY"),
				envOr("ASTRO_LLM_MODEL", "gemini-2.5-flash")),
			acts, rules)
	}
```

Y el **guard de error genérico** (regla gatillada del mentor): reemplazá la rama
```go
		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			chirps.Play("confused")
			respond(face, voice, Pensativo, "No te entendí.")
			continue
		}
```
por (cualquier error, no solo ErrNoEntiendo → sin panic):
```go
		action, err := interpreter.Interpret(text)
		if err != nil { // ErrNoEntiendo o cualquier error del cerebro: reacciona y sigue, sin crash
			chirps.Play("confused")
			respond(face, voice, Pensativo, "No te entendí.")
			continue
		}
```

- [ ] **Step 3: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 4: Smoke sin LLM (modo reglas, cableado intacto)**

```bash
printf 'hola\nxyz\n' | ASTRO_INPUT=stdin ASTRO_BRAIN=rules nix shell nixpkgs#go --command go run . 2>/dev/null
```
Expected: saludo, "No te entendí.", "Chau" (las reglas siguen funcionando).

- [ ] **Step 5: E2E con Gemini (manual — necesita la key gratis)**

```bash
export ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai
export ASTRO_LLM_KEY=AIza...
export ASTRO_LLM_MODEL=gemini-2.5-flash
# Frases que las REGLAS no pueden resolver (sin keyword) → prueban que el LLM actúa de verdad:
printf 'dale play a la música\nhacé menos ruido\nabrime el firefox\n' | ASTRO_INPUT=stdin \
  nix shell nixpkgs#go --command go run .
```
Expected: interpreta natural → pausar, bajar/mute, abrir firefox. **Dejá stderr visible (sin `2>/dev/null`):**
si ves `(cerebro LLM falló…)` o "No te entendí" en todas, el LLM NO está funcionando (key/modelo/red) —
no te confíes de un verde falso.

- [ ] **Step 6: Commit**

```bash
git add llm.go main.go
git commit -m "feat: chat HTTP OpenAI-compat + wiring (ASTRO_BRAIN=llm por default, guard de error genérico)"
```

---

## Definition of Done (Fase C)

1. `go build/vet/test ./...` verdes (tests de LLMInterpreter con chat fake, incluido el de seguridad).
2. `ASTRO_BRAIN=rules` (o sin key): sigue andando con reglas (fallback). Smoke stdin OK.
3. E2E con Gemini: le hablás natural y elige la acción correcta.
4. Error del LLM / acción fuera del menú → pensativo + chirp, **sin crash** (guard genérico).
5. `isSafeAppName` frena un `arg` peligroso del LLM (test que lo prueba).
6. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Seguridad del arg del LLM:** cubierta — `open` pasa por `isSafeAppName`; test `TestLLMOpenInyeccionRechazada`.
- **Guard del mentor (Fase B):** aplicado — `main` trata cualquier error como "no entendí", nunca `nil.Run()`.
- **Solo stdlib:** el LLM se llama por `net/http`, sin SDK. `map[string]any` es Go 1.18+ (tenemos 1.26).
- **Fallback reusa Fase A:** el `RuleInterpreter` no se tira; es la red de seguridad → nada desperdiciado.
- **Tipos consistentes:** `chatFunc(system,user)(string,error)`, `Interpret(string)(*Action,error)` en todas las tasks.
- **JSON mode defensivo:** aunque el proveedor no honre `response_format`, el prompt pide JSON y `extractJSON`
  + `fallback` cubren respuestas con texto alrededor o inválidas.
