# Astro · Fase A — Plan de Implementación (v2, post-auditoría)

> **Para quien ejecuta:** SUB-SKILL REQUERIDA: superpowers:subagent-driven-development (recomendado)
> o superpowers:executing-plans, tarea por tarea. Los pasos usan checkbox `- [ ]`.

**Goal:** Que Astro exista en la PC (sin hardware): cara en un widget de eww + escribís un comando por
terminal → hace la acción real en la laptop + cambia de cara + responde por texto.

**Architecture:** Daemon en Go, paquete `main` en la raíz del repo. Interfaces donde hay (o habrá muy
pronto) varias implementaciones: `Runner` (real + fake de test hoy), `Display` (eww hoy, ESP32 después),
`Interpreter` (reglas hoy, LLM en Fase 5). Struct simple donde hay una sola forma: `Action`, `Expression`.

**Tech Stack:** Go 1.22+, eww, Python (genera PNGs), utilidades Hyprland/Wayland (`hyprctl`, `playerctl`, `wpctl`).

> **Nota de diseño (opción A, elegida):** `Display`/`Interpreter` se dejan como interfaces **por el
> objetivo de aprendizaje declarado en la spec (dependency inversion)**. Honestidad: en Go las interfaces
> son implícitas, así que en un proyecto de trabajo el reflejo idiomático sería usar el tipo concreto y
> *extraer la interfaz gratis* recién cuando aparece el 2º implementador. Acá la tenemos a la vista a
> propósito, para aprender el patrón. `Runner` sí se gana la interfaz hoy (el fake es 2º impl real).

## Global Constraints

- Identificadores en **inglés**; comentarios y mensajes al usuario en **español**.
- Nombres descriptivos; cortos solo en scope mínimo. Nada destructivo; sin shell arbitrario.
- TDD: test que falla → mínimo → pasa → commit. Módulo: `github.com/cRolandoJr/astro`, raíz `~/projects/astro/`.
- Go vía `nix shell nixpkgs#go`. Sin dependencias externas (stdlib only).

---

## File Structure

```
~/projects/astro/
  go.mod
  expression.go / runner.go / display.go / action.go / actions.go / interpreter.go / main.go
  *_test.go (runner, fake, display, actions, interpreter)
  tools/genfaces.py
  eww/eww.yuck / eww/eww.scss / eww/faces/*.png
```

---

### Task 1: Setup + Expression + Runner

**Files:** Create `go.mod`, `expression.go`, `runner.go`, `runner_test.go`, `fake_test.go`
**Produces:** `Expression` (+constantes); `Runner interface { Run(name string, args ...string)(string,error) }`; `ExecRunner`; test-helper `fakeRunner{calls [][]string; output string; err error}` con `lastCall() []string`.

- [ ] **Step 1: Inicializar módulo**

```bash
cd ~/projects/astro
nix shell nixpkgs#go --command go mod init github.com/cRolandoJr/astro
```
Expected: crea `go.mod`.

- [ ] **Step 2: `expression.go`**

```go
package main

// Expression es el gesto de Astro. String para mapear directo al PNG y al comando de eww.
type Expression string

const (
	Neutral     Expression = "neutral"
	Feliz       Expression = "feliz"
	Curioso     Expression = "curioso"
	Pensativo   Expression = "pensativo"
	Sorprendido Expression = "sorprendido"
	Dormido     Expression = "dormido"
	Triste      Expression = "triste"
	Mareado     Expression = "mareado"
)
// (parpadeo, bostezo, guino, amor, enojado también existen como PNG — se usan en fases
//  con voz/estados; por eso hay 13 PNGs y 8 constantes en Fase A.)
```

- [ ] **Step 3: Test que falla (`runner_test.go`)**

```go
package main

import "testing"

func TestExecRunnerRunsCommand(t *testing.T) {
	out, err := ExecRunner{}.Run("echo", "hola")
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if out != "hola" {
		t.Fatalf("esperaba %q, obtuve %q", "hola", out)
	}
}
```

- [ ] **Step 4: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestExecRunner`
Expected: FALLA (no compila).

- [ ] **Step 5: `runner.go`**

```go
package main

import (
	"os/exec"
	"strings"
)

// Runner ejecuta comandos del sistema. Interfaz para inyectar un fake en tests y NO
// abrir programas de verdad al testear.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// ExecRunner corre comandos reales. CombinedOutput junta stdout+stderr y espera el fin.
type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
```

- [ ] **Step 6: `fake_test.go`**

```go
package main

// fakeRunner registra las llamadas en vez de ejecutarlas: verifica QUÉ comando se
// habría corrido, sin efectos reales.
type fakeRunner struct {
	calls  [][]string
	output string
	err    error
}

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.output, f.err
}

func (f *fakeRunner) lastCall() []string {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}
```

- [ ] **Step 7: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -run TestExecRunner -v` → PASS.

- [ ] **Step 8: Commit**

```bash
git add go.mod expression.go runner.go runner_test.go fake_test.go
git commit -m "feat: Runner (interfaz + real + fake) y tipo Expression"
```

---

### Task 2: Display (eww) — con ConfigDir  [incorpora must-fix #2]

**Files:** Create `display.go`, `display_test.go`
**Produces:** `Display interface { Show(Expression) error }`; `EwwFace{ Runner Runner; ConfigDir string }`.

> **Must-fix #2:** el widget se abre con `eww --config <dir>`; si `Show` corre `eww update` **sin**
> `--config`, le habla a otro daemon y la cara nunca cambia. Solución: `EwwFace` guarda el `ConfigDir`
> y siempre pasa `--config`. No dependemos de ninguna env var de eww (usamos el flag, que es seguro).

- [ ] **Step 1: Test que falla (`display_test.go`)**

```go
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
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestEwwFace` → FALLA.

- [ ] **Step 3: `display.go`**

```go
package main

import "fmt"

// Display muestra la cara. Interfaz porque tendrá varias implementaciones: hoy EwwFace
// (pantalla), mañana ESP32Display (robot). Mismo contrato. (Ver nota de diseño del plan:
// la mantenemos como interfaz por aprendizaje; en Go se podría extraer gratis después.)
type Display interface {
	Show(expr Expression) error
}

// EwwFace actualiza una variable de eww para que el widget muestre faces/<expr>.png.
// ConfigDir apunta a la config de eww del repo; si está vacío, usa la default de eww.
type EwwFace struct {
	Runner    Runner
	ConfigDir string
}

func (e EwwFace) Show(expr Expression) error {
	args := []string{"update", fmt.Sprintf("astro_face=%s", expr)}
	if e.ConfigDir != "" {
		args = append([]string{"--config", e.ConfigDir}, args...)
	}
	if _, err := e.Runner.Run("eww", args...); err != nil {
		return fmt.Errorf("no pude actualizar la cara: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -run TestEwwFace -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add display.go display_test.go
git commit -m "feat: Display + EwwFace con ConfigDir (fix: eww update llega al widget correcto)"
```

---

### Task 3: Action + registro simple

**Files:** Create `action.go`, `actions.go`, `actions_test.go`
**Produces:** `Action{ Name string; Face Expression; Run func(Runner)(string,error) }`; `buildActions(now func() time.Time) map[string]*Action` con claves `saludar`, `hora`, `dormir`, `despertar` (las de escritorio se agregan en Task 4).

- [ ] **Step 1: `action.go`**

```go
package main

// Action es una capacidad de Astro. Struct con campo func (NO interfaz): todas tienen la
// misma forma y una sola implementación → interfaz sería abstracción sin pagar (YAGNI).
// La interfaz se gana donde hay varias implementaciones (Display, Interpreter, Runner).
type Action struct {
	Name string
	Face Expression
	Run  func(r Runner) (reply string, err error)
}
```

- [ ] **Step 2: Test que falla (`actions_test.go`)**

Concepto: `now` se inyecta para testear la hora sin depender del reloj real. Uso `strings.Contains`
de la stdlib (no reinvento un buscador).

```go
package main

import (
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
```

- [ ] **Step 3: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestSaludar|TestHora'` → FALLA.

- [ ] **Step 4: `actions.go` (parte 1)**

```go
package main

import "time"

// buildActions arma el registro. 'now' inyectado para testear la hora.
func buildActions(now func() time.Time) map[string]*Action {
	actions := map[string]*Action{}
	add := func(a *Action) { actions[a.Name] = a }

	add(&Action{Name: "saludar", Face: Feliz,
		Run: func(r Runner) (string, error) { return "¡Hola! Soy Astro.", nil }})
	add(&Action{Name: "hora", Face: Neutral,
		Run: func(r Runner) (string, error) { return "Son las " + now().Format("15:04") + ".", nil }})
	add(&Action{Name: "dormir", Face: Dormido,
		Run: func(r Runner) (string, error) { return "Me duermo… 💤", nil }})
	add(&Action{Name: "despertar", Face: Neutral,
		Run: func(r Runner) (string, error) { return "¡Ya estoy despierto!", nil }})

	addDesktopActions(add) // definidas en Task 4 (mismo archivo)
	return actions
}
```

> Nota: `addDesktopActions` se crea en la Task 4. Para que Task 3 compile por sí sola, en Task 3
> agregá temporalmente `func addDesktopActions(add func(*Action)) {}` (stub vacío) al final de
> `actions.go`; la Task 4 lo reemplaza por la versión real.

- [ ] **Step 5: Agregar stub + correr — pasa**

Agregá al final de `actions.go`: `func addDesktopActions(add func(*Action)) {}`
Run: `nix shell nixpkgs#go --command go test ./... -run 'TestSaludar|TestHora' -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add action.go actions.go actions_test.go
git commit -m "feat: Action (struct) + registro con saludar/hora/dormir/despertar"
```

---

### Task 4: Acciones de escritorio + Interpreter  [fusiona ex-4 y ex-5; incorpora must-fix #1]

**Files:** Modify `actions.go`, `actions_test.go`; Create `interpreter.go`, `interpreter_test.go`
**Produces:** `addDesktopActions` (claves `pausar`, `siguiente`, `subir_volumen`, `bajar_volumen`, `mute`); `openAppAction(app string) *Action`; `Interpreter interface { Interpret(string)(*Action,error) }`; `NewRuleInterpreter(map[string]*Action) *RuleInterpreter`; `ErrNoEntiendo`.

> **Must-fix #1:** el lookup por clave (`ri.actions["pausar"]`) devuelve `nil` **sin error** si la clave
> falta → `main` haría `nil.Run()` → panic → se corta el loop (rompe spec §6). Solución: helper con
> *coma-ok* que devuelve `ErrNoEntiendo` en vez de nil. El table-test de abajo cubre esta clase de bug.

- [ ] **Step 1: Reemplazar el stub por las acciones reales (`actions.go`)**

Reemplazá `func addDesktopActions(add func(*Action)) {}` por:

```go
import "fmt" // agregar al bloque de imports de actions.go (junto a "time")

// addDesktopActions registra las acciones que tocan el escritorio.
func addDesktopActions(add func(*Action)) {
	add(&Action{Name: "pausar", Face: Feliz, Run: playerctl("play-pause", "Listo.")})
	add(&Action{Name: "siguiente", Face: Feliz, Run: playerctl("next", "Siguiente.")})
	add(&Action{Name: "subir_volumen", Face: Neutral, Run: volume("5%+", "Subí el volumen.")})
	add(&Action{Name: "bajar_volumen", Face: Neutral, Run: volume("5%-", "Bajé el volumen.")})
	add(&Action{Name: "mute", Face: Neutral, Run: func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"); err != nil {
			return "", fmt.Errorf("no pude silenciar: %w", err)
		}
		return "Mute.", nil
	}})
}

func playerctl(cmd, ok string) func(Runner) (string, error) {
	return func(r Runner) (string, error) {
		if _, err := r.Run("playerctl", cmd); err != nil {
			return "", fmt.Errorf("no pude controlar la música: %w", err)
		}
		return ok, nil
	}
}

func volume(delta, ok string) func(Runner) (string, error) {
	return func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", delta); err != nil {
			return "", fmt.Errorf("no pude cambiar el volumen: %w", err)
		}
		return ok, nil
	}
}

// openAppAction arma una acción al vuelo que abre 'app'. Usa `hyprctl dispatch exec` porque
// no bloquea y respeta tus reglas de Hyprland (igual que tus keybinds).
func openAppAction(app string) *Action {
	return &Action{Name: "abrir_" + app, Face: Curioso, Run: func(r Runner) (string, error) {
		if _, err := r.Run("hyprctl", "dispatch", "exec", app); err != nil {
			return "", fmt.Errorf("no pude abrir %s: %w", app, err)
		}
		return "Abriendo " + app + ".", nil
	}}
}
```

- [ ] **Step 2: Tests de acciones y del interpreter (agregar a `actions_test.go` y crear `interpreter_test.go`)**

En `actions_test.go`:
```go
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
```
(agregá `"reflect"` al import de `actions_test.go`.)

`interpreter_test.go`:
```go
package main

import (
	"errors"
	"testing"
	"time"
)

func testInterpreter() Interpreter { return NewRuleInterpreter(buildActions(time.Now)) }

// Table-test: toda frase de ejemplo resuelve a una acción NO nil (guard del must-fix #1).
func TestCadaReglaResuelveAccion(t *testing.T) {
	cmds := []string{"hola", "qué hora es", "pausá", "siguiente", "subí el volumen",
		"bajá el volumen", "silencio", "dormí", "despertá", "abrí firefox"}
	in := testInterpreter()
	for _, c := range cmds {
		a, err := in.Interpret(c)
		if err != nil || a == nil {
			t.Errorf("%q → acción nil o error: a=%v err=%v", c, a, err)
		}
	}
}

func TestNoEntiende(t *testing.T) {
	if _, err := testInterpreter().Interpret("xyzzy"); !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("esperaba ErrNoEntiendo, fue %v", err)
	}
}

func TestAbriSinAppNoEntiende(t *testing.T) {
	if _, err := testInterpreter().Interpret("abrí"); !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("esperaba ErrNoEntiendo, fue %v", err)
	}
}
```

- [ ] **Step 3: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./...` → FALLA (`NewRuleInterpreter`/`ErrNoEntiendo` no existen).

- [ ] **Step 4: `interpreter.go`**

Concepto: `normalize` saca acentos (para que `segui`/`dormi` sin tilde matcheen) — sin dependencias,
con un `strings.NewReplacer`. `named` es el lookup coma-ok (must-fix #1). La regla `abrí X` arma la
acción al vuelo con una closure que captura la app.

```go
package main

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNoEntiendo = errors.New("no entendí")

type Interpreter interface {
	Interpret(text string) (*Action, error)
}

// RuleInterpreter matchea por palabras clave. En la Fase 5 lo reemplaza uno con LLM.
type RuleInterpreter struct {
	actions map[string]*Action
}

func NewRuleInterpreter(actions map[string]*Action) *RuleInterpreter {
	return &RuleInterpreter{actions: actions}
}

func (ri *RuleInterpreter) Interpret(text string) (*Action, error) {
	t := normalize(text)
	switch {
	case containsAny(t, "hola", "buenas"):
		return ri.named("saludar")
	case containsAny(t, "hora"):
		return ri.named("hora")
	case containsAny(t, "pausa", "segui", "reproduc"):
		return ri.named("pausar")
	case containsAny(t, "siguiente", "proxima", "next"):
		return ri.named("siguiente")
	case containsAny(t, "subi", "sube", "mas volumen"):
		return ri.named("subir_volumen")
	case containsAny(t, "baja", "menos volumen"):
		return ri.named("bajar_volumen")
	case containsAny(t, "mute", "silencio", "callate"):
		return ri.named("mute")
	case containsAny(t, "dormi", "chau"):
		return ri.named("dormir")
	case containsAny(t, "despert"):
		return ri.named("despertar")
	case strings.HasPrefix(t, "abri"):
		parts := strings.SplitN(t, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return nil, ErrNoEntiendo
		}
		return openAppAction(strings.TrimSpace(parts[1])), nil
	default:
		return nil, ErrNoEntiendo
	}
}

// named busca la acción con coma-ok: si la clave no está (bug de registro), devuelve
// ErrNoEntiendo en vez de un *Action nil que reventaría en main.
func (ri *RuleInterpreter) named(name string) (*Action, error) {
	a, ok := ri.actions[name]
	if !ok {
		return nil, fmt.Errorf("acción %q no registrada: %w", name, ErrNoEntiendo)
	}
	return a, nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// normalize: minúsculas, sin espacios extra y sin tildes (comandos de terminal a menudo van sin tilde).
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u").Replace(s)
}
```

- [ ] **Step 5: Correr TODO — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -v` → PASS (runner, display, actions, interpreter).

- [ ] **Step 6: Commit**

```bash
git add actions.go actions_test.go interpreter.go interpreter_test.go
git commit -m "feat: acciones de escritorio + Interpreter (coma-ok anti-nil, normaliza acentos)"
```

---

### Task 5: main (wiring + loop)

**Files:** Create `main.go`
**Consumes:** todo lo anterior.

- [ ] **Step 1: `main.go`**

Concepto: inyección de dependencias explícita + loop. `ConfigDir` sale de la env `ASTRO_EWW_CONFIG`.
Chequeo `scanner.Err()` al salir (idiom completo de lectura de stdin).

```go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"
)

func main() {
	runner := ExecRunner{}
	var display Display = EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	_ = display.Show(Neutral)

	fmt.Println("Astro está despierto. Escribí un comando (Ctrl+D para salir).")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := scanner.Text()
		if text == "" {
			continue
		}

		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			_ = display.Show(Pensativo)
			fmt.Println("No te entendí. Probá: hola · qué hora es · pausá · siguiente · subí/bajá/silencio · abrí firefox · dormí · despertá")
			continue
		}

		reply, err := action.Run(runner)
		if err != nil {
			_ = display.Show(Neutral)
			fmt.Println("Ups:", err)
			continue
		}
		_ = display.Show(action.Face)
		fmt.Println(reply)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error leyendo stdin:", err)
	}
	fmt.Println("\nChau 👋")
}
```

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go test ./...'` → OK + PASS.

- [ ] **Step 3: Verificación manual (sin eww aún; los Show fallan callados, OK)**

Run: `nix shell nixpkgs#go --command go run .`
Probar: `hola`, `qué hora es`, `xyz`, `abrí kitty`, Ctrl+D.
Expected: saludo, hora, "no te entendí", abre kitty, "Chau".

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: main — wiring + loop de comandos por terminal"
```

---

### Task 6: Widget de eww + caras

**Files:** Create `tools/genfaces.py`, `eww/eww.yuck`, `eww/eww.scss`, `eww/faces/*.png`

- [ ] **Step 1: `tools/genfaces.py` (auto-contenido — dibuja las 13 caras por primitivas)**

```python
#!/usr/bin/env python3
# Genera eww/faces/<nombre>.png (128x64, blanco sobre negro) por primitivas.
import struct, zlib, math, os
W, H = 128, 64
OUT = os.path.join(os.path.dirname(__file__), "..", "eww", "faces")

def newg(): return [[0]*W for _ in range(H)]
def px(g,x,y):
    if 0<=x<W and 0<=y<H: g[y][x]=1
def stamp(g,x,y,t=3):
    r=t//2
    for dx in range(-r,t-r):
        for dy in range(-r,t-r): px(g,x+dx,y+dy)
def line(g,x0,y0,x1,y1,t=3):
    dx=abs(x1-x0); dy=-abs(y1-y0); sx=1 if x0<x1 else -1; sy=1 if y0<y1 else -1; e=dx+dy
    while True:
        stamp(g,x0,y0,t)
        if x0==x1 and y0==y1: break
        e2=2*e
        if e2>=dy: e+=dy; x0+=sx
        if e2<=dx: e+=dx; y0+=sy
def hline(g,x0,x1,y,t=3):
    for x in range(x0,x1+1): stamp(g,x,y,t)
def fcircle(g,cx,cy,r):
    for y in range(cy-r,cy+r+1):
        for x in range(cx-r,cx+r+1):
            if (x-cx)**2+(y-cy)**2<=r*r: px(g,x,y)
def ocircle(g,cx,cy,r,t=2):
    a=0
    while a<360:
        stamp(g,round(cx+r*math.cos(math.radians(a))),round(cy+r*math.sin(math.radians(a))),t); a+=3
def ftri(g,p0,p1,p2):
    def ar(a,b,c): return (b[0]-a[0])*(c[1]-a[1])-(c[0]-a[0])*(b[1]-a[1])
    xs=[p[0] for p in(p0,p1,p2)]; ys=[p[1] for p in(p0,p1,p2)]
    for y in range(min(ys),max(ys)+1):
        for x in range(min(xs),max(xs)+1):
            w0=ar(p1,p2,(x,y)); w1=ar(p2,p0,(x,y)); w2=ar(p0,p1,(x,y))
            if (w0>=0 and w1>=0 and w2>=0) or (w0<=0 and w1<=0 and w2<=0): px(g,x,y)
def rr(x,y,x0,y0,x1,y1,r):
    if x<x0 or x>x1 or y<y0 or y>y1: return False
    cx=x0+r if x<x0+r else (x1-r if x>x1-r else None)
    cy=y0+r if y<y0+r else (y1-r if y>y1-r else None)
    if cx is not None and cy is not None: return (x-cx)**2+(y-cy)**2<=r*r
    return True
def fill_rr(g,x0,y0,x1,y1,r):
    for y in range(y0,y1+1):
        for x in range(x0,x1+1):
            if rr(x,y,x0,y0,x1,y1,r): g[y][x]=1
def outline_rr(g,x0,y0,x1,y1,r,t):
    for y in range(H):
        for x in range(W):
            if rr(x,y,x0,y0,x1,y1,r) and not rr(x,y,x0+t,y0+t,x1-t,y1-t,r-t): g[y][x]=1
def parab(g,x0,x1,ymid,yend,t=3):
    cx=(x0+x1)/2; half=(x1-x0)/2
    for x in range(x0,x1+1): stamp(g,x,round(ymid+(yend-ymid)*((x-cx)/half)**2),t)
def frame(g): outline_rr(g,8,6,119,57,10,2)
L,R=42,86

def faces():
    f={}
    def neutral():
        g=newg();frame(g);fill_rr(g,32,20,52,32,4);fill_rr(g,76,20,96,32,4);hline(g,50,78,46,3);return g
    def parpadeo():
        g=newg();frame(g);hline(g,32,52,26,2);hline(g,76,96,26,2);hline(g,50,78,46,3);return g
    def pensativo():
        g=newg();frame(g);fcircle(g,42,23,4);fcircle(g,86,23,4);line(g,78,17,94,19,2);hline(g,48,64,47,2)
        for dx in (0,6,12): stamp(g,98+dx,44,2)
        return g
    def feliz():
        g=newg();frame(g)
        for e in (L,R): line(g,e-10,30,e,20,3); line(g,e,20,e+10,30,3)
        parab(g,46,82,50,42,3);return g
    def guino():
        g=newg();frame(g);fill_rr(g,32,20,52,32,4);line(g,76,30,86,20,3);line(g,86,20,96,30,3);parab(g,46,82,50,42,3);return g
    def amor():
        g=newg();frame(g)
        for e in (L,R): fcircle(g,e-4,24,4);fcircle(g,e+4,24,4);ftri(g,(e-8,26),(e+8,26),(e,34))
        parab(g,48,80,49,43,3);return g
    def sorprendido():
        g=newg();frame(g);fcircle(g,42,26,9);fcircle(g,86,26,9);fcircle(g,64,46,5);return g
    def curioso():
        g=newg();frame(g);fcircle(g,42,26,7);fcircle(g,86,26,7);line(g,76,15,96,18,2);fcircle(g,64,46,3);return g
    def bostezo():
        g=newg();frame(g);hline(g,34,50,24,2);hline(g,78,94,24,2);ocircle(g,64,46,8,2);return g
    def dormido():
        g=newg();frame(g);hline(g,32,52,28,3);hline(g,76,96,28,3);hline(g,58,70,46,2)
        line(g,100,14,112,14,2);line(g,112,14,100,22,2);line(g,100,22,112,22,2);return g
    def triste():
        g=newg();frame(g);fill_rr(g,34,27,50,34,3);fill_rr(g,78,27,94,34,3);line(g,34,26,50,20,2);line(g,78,20,94,26,2);fcircle(g,38,41,3);parab(g,48,80,45,51,3);return g
    def enojado():
        g=newg();frame(g);line(g,34,20,52,28,3);line(g,94,20,76,28,3);fill_rr(g,36,30,50,36,3);fill_rr(g,78,30,92,36,3);parab(g,50,78,43,48,3);return g
    def mareado():
        g=newg();frame(g)
        for e in (L,R): line(g,e-9,19,e+9,33,3);line(g,e+9,19,e-9,33,3)
        for x in range(46,83): stamp(g,x,round(46+2.4*math.sin((x-46)/3.0)),2)
        return g
    for n,fn in [("neutral",neutral),("parpadeo",parpadeo),("pensativo",pensativo),("feliz",feliz),
                 ("guino",guino),("amor",amor),("sorprendido",sorprendido),("curioso",curioso),
                 ("bostezo",bostezo),("dormido",dormido),("triste",triste),("enojado",enojado),
                 ("mareado",mareado)]:
        f[n]=fn()
    return f

def write_png(path,g,SC=4):
    ON=(207,240,255); OFF=(6,11,18); ow,oh=W*SC,H*SC; raw=bytearray()
    for y in range(oh):
        raw.append(0); sy=y//SC
        for x in range(ow): raw+=bytes(ON if g[sy][x//SC] else OFF)
    def ch(t,d): c=t+d; return struct.pack('>I',len(d))+c+struct.pack('>I',zlib.crc32(c)&0xffffffff)
    png=b'\x89PNG\r\n\x1a\n'+ch(b'IHDR',struct.pack('>IIBBBBB',ow,oh,8,2,0,0,0))+ch(b'IDAT',zlib.compress(bytes(raw),9))+ch(b'IEND',b'')
    open(path,'wb').write(png)

os.makedirs(OUT, exist_ok=True)
for name,g in faces().items():
    write_png(os.path.join(OUT, name+".png"), g)
print("caras generadas en", os.path.normpath(OUT))
```

- [ ] **Step 2: Generar**

Run: `nix shell nixpkgs#python3 --command python3 tools/genfaces.py`
Verificar: `ls eww/faces/` → 13 PNGs (neutral … mareado).

- [ ] **Step 3: `eww/eww.yuck`**

```lisp
(defvar astro_face "neutral")
(defwidget face []
  (box :class "astro"
    (image :path "${EWW_CONFIG_DIR}/faces/${astro_face}.png" :image-width 256 :image-height 128)))
(defwindow astro
  :monitor 0
  :geometry (geometry :x "20px" :y "20px" :width "280px" :height "160px" :anchor "top right")
  :stacking "fg"
  (face))
```

- [ ] **Step 4: `eww/eww.scss`**

```scss
.astro { background-color: #060b12; border-radius: 14px; padding: 10px; }
```

- [ ] **Step 5: Verificar el mecanismo de config de eww y abrir el widget**

Run: `eww --help | grep -i config` (confirmá que existe el flag `--config`; NO dependemos de env vars de eww).
Luego:
```bash
eww --config ~/projects/astro/eww open astro
eww --config ~/projects/astro/eww update astro_face=feliz   # debe cambiar la cara
```
Expected: aparece el widget (cara neutral) y cambia a feliz.

- [ ] **Step 6: End-to-end**

```bash
export ASTRO_EWW_CONFIG=~/projects/astro/eww
nix shell nixpkgs#go --command go run .
```
Escribir comandos: la cara del widget cambia (feliz al saludar, pensativo si no entiende, dormido con "dormí").

- [ ] **Step 7: Commit**

```bash
git add tools/genfaces.py eww/
git commit -m "feat: widget de eww + caras generadas; Astro reacciona en pantalla"
```

---

## Definition of Done (Fase A)

1. `go build ./...` y `go test ./...` verdes.
2. Con el widget abierto y `ASTRO_EWW_CONFIG` seteado: cada comando (hola, hora, pausá, siguiente,
   subí/bajá/silencio, abrí X, dormí, despertá) hace efecto real + cambia la cara + responde.
3. Comando desconocido → cara `pensativo` + ayuda, sin cortar el loop (garantizado por el coma-ok + table-test).
4. Convenciones respetadas (ids inglés, comentarios español, nada destructivo).

## Cobertura de spec (honesta)

- §5 acciones: se cubren **todas** las de la tabla — hola, hora, abrir, pausá, **siguiente**, subir/**bajar**/**mute**, dormir/**despertar**. (En v1 faltaban las de *cursiva*; agregadas.)
- §3 arquitectura → Tasks 1-5; §6 errores → Task 4 (coma-ok) + Task 5 (loop no corta); §7 tests → fakes + table-test; §8 estructura → File Structure; cara → Task 6.

## Cambios vs v1 (auditoría del mentor Go aplicada)

- must-fix #1 (panic por nil): helper `named` coma-ok + table-test guard.
- must-fix #2 (eww config): `EwwFace.ConfigDir` + `--config` siempre; env `ASTRO_EWW_CONFIG` en main; verificación del flag en Task 6.
- Fusionadas ex-Task 4 y 5 (acople de compilación resuelto).
- Tests agregados: wpctl args, `abrí` vacío, table-test de todas las reglas.
- `genfaces.py` inline (auto-contenido) + nota enum(8)/PNG(13).
- Acentos normalizados; `strings.Contains` en tests; `scanner.Err()` en main.
- Cobertura §5 completada (siguiente/bajar/mute/despertar); claim de "sin huecos" corregido a inventario real.
- Decisión A: `Display`/`Interpreter` quedan interfaces, con nota honesta de que es por pedagogía.
