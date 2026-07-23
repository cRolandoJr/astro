# Astro · Fase A — Plan de Implementación

> **Para quien ejecuta:** SUB-SKILL REQUERIDA: usar superpowers:subagent-driven-development
> (recomendado) o superpowers:executing-plans, tarea por tarea. Los pasos usan checkbox `- [ ]`.

**Goal:** Que Astro exista en la PC (sin hardware): cara en un widget de eww + escribís un comando por
terminal → hace la acción real en la laptop + cambia de cara + responde por texto.

**Architecture:** Daemon en Go, paquete `main` en la raíz del repo. Cinco piezas chicas conectadas por
interfaces donde hay varias implementaciones (`Display`, `Interpreter`, `Runner`) y structs simples donde
no las hay (`Action`, `Expression`). El "cerebro" no sabe de dónde viene el texto ni cómo se pinta la cara.

**Tech Stack:** Go 1.22+, eww (widget), Python (genera los PNG de las caras), utilidades Wayland/Hyprland
(`hyprctl`, `playerctl`, `wpctl`). Spec: `docs/specs/2026-07-23-astro-fase-a-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios y mensajes al usuario en **español**.
- Nombres descriptivos; cortos solo en scope mínimo (`i`, receptor de método).
- **Nada destructivo:** solo las acciones curadas; sin ejecución de shell arbitrario.
- TDD: test que falla → implementación mínima → test que pasa → commit. Commits frecuentes.
- Módulo Go: `github.com/cRolandoJr/astro`. Código en la raíz `~/projects/astro/`.
- Go se obtiene con `nix shell nixpkgs#go` (rápido) o agregándolo al flake (permanente).

---

## File Structure

```
~/projects/astro/
  go.mod
  expression.go      — tipo Expression (las caras)
  runner.go          — Runner (interfaz) + ExecRunner (real)
  runner_test.go     — test de ExecRunner
  fake_test.go       — fakeRunner compartido por los tests
  display.go         — Display (interfaz) + EwwFace
  display_test.go
  action.go          — struct Action
  actions.go         — registro de acciones + openAppAction
  actions_test.go
  interpreter.go     — Interpreter (interfaz) + RuleInterpreter
  interpreter_test.go
  main.go            — wiring + loop de lectura
  tools/genfaces.py  — genera eww/faces/*.png
  eww/eww.yuck       — widget de la cara
  eww/eww.scss       — estilo mínimo
  eww/faces/*.png    — caras (generadas)
```

---

### Task 1: Setup + Expression + Runner

**Files:**
- Create: `go.mod`, `expression.go`, `runner.go`, `runner_test.go`, `fake_test.go`

**Interfaces:**
- Produces: `type Expression string` (+ constantes); `type Runner interface { Run(name string, args ...string) (string, error) }`; `ExecRunner`; y para tests `fakeRunner` con campos `calls [][]string`, `output string`, `err error` y método `lastCall() []string`.

- [ ] **Step 1: Inicializar el módulo**

Concepto: `go.mod` declara el módulo y la versión de Go. Es la raíz del proyecto Go.

```bash
cd ~/projects/astro
nix shell nixpkgs#go --command go mod init github.com/cRolandoJr/astro
```
Expected: crea `go.mod` con `module github.com/cRolandoJr/astro` y `go 1.2x`.

- [ ] **Step 2: Escribir `expression.go`**

Concepto: un tipo `string` nombrado. Lo usamos como "enum" que mapea directo al nombre del PNG y al comando de eww.

```go
package main

// Expression es el gesto que muestra Astro. Es string para mapear directo al
// nombre del PNG y al comando de eww (faces/<expr>.png).
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
```

- [ ] **Step 3: Escribir el test que falla (`runner_test.go`)**

Concepto: `ExecRunner` corre un comando real. Lo probamos con `echo`, inofensivo.

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

- [ ] **Step 4: Correr el test — debe fallar**

Run: `nix shell nixpkgs#go --command go test ./... -run TestExecRunner`
Expected: FALLA (no compila: `ExecRunner` no existe).

- [ ] **Step 5: Escribir `runner.go`**

Concepto: interfaz `Runner` (para inyectar un falso en tests) + implementación real con `os/exec`. `CombinedOutput` junta stdout+stderr y espera a que termine.

```go
package main

import (
	"os/exec"
	"strings"
)

// Runner ejecuta comandos del sistema. Es interfaz para poder inyectar un runner
// falso en los tests y NO abrir programas de verdad al testear.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// ExecRunner corre comandos reales del SO.
type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
```

- [ ] **Step 6: Escribir el `fakeRunner` (`fake_test.go`)**

Concepto: implementa `Runner` pero en vez de ejecutar, **registra** qué se pidió. Así los tests verifican el comando sin efectos reales. (Va en un archivo `_test.go` para que no entre en el binario final.)

```go
package main

// fakeRunner registra las llamadas en vez de ejecutarlas. Verifica QUÉ comando se
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

- [ ] **Step 7: Correr el test — debe pasar**

Run: `nix shell nixpkgs#go --command go test ./... -run TestExecRunner -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add go.mod expression.go runner.go runner_test.go fake_test.go
git commit -m "feat: Runner (interfaz + real + fake) y tipo Expression"
```

---

### Task 2: Display (eww)

**Files:**
- Create: `display.go`, `display_test.go`

**Interfaces:**
- Consumes: `Runner`, `Expression`, `fakeRunner.lastCall()`.
- Produces: `type Display interface { Show(expr Expression) error }`; `EwwFace{ Runner Runner }`.

- [ ] **Step 1: Test que falla (`display_test.go`)**

Concepto: mostrar una cara = decirle a eww qué variable poner. Verificamos el comando exacto con el fake.

```go
package main

import (
	"reflect"
	"testing"
)

func TestEwwFaceShowUpdatesVariable(t *testing.T) {
	fake := &fakeRunner{}
	face := EwwFace{Runner: fake}

	if err := face.Show(Feliz); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}

	got := fake.lastCall()
	want := []string{"eww", "update", "astro_face=feliz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, obtuve %v", want, got)
	}
}
```

- [ ] **Step 2: Correr — debe fallar**

Run: `nix shell nixpkgs#go --command go test ./... -run TestEwwFace`
Expected: FALLA (no compila: `EwwFace` no existe).

- [ ] **Step 3: Escribir `display.go`**

Concepto: `Display` es **interfaz** porque tiene varias implementaciones — hoy `EwwFace` (pantalla), mañana `ESP32Display` (robot). Mismo contrato. `%w` envuelve el error para no perder la causa.

```go
package main

import "fmt"

// Display muestra la cara de Astro. Interfaz porque tiene varias implementaciones:
// hoy eww en la pantalla, mañana el ESP32 en el robot. Mismo contrato.
type Display interface {
	Show(expr Expression) error
}

// EwwFace actualiza una variable de eww para que el widget muestre faces/<expr>.png.
type EwwFace struct {
	Runner Runner
}

func (e EwwFace) Show(expr Expression) error {
	_, err := e.Runner.Run("eww", "update", fmt.Sprintf("astro_face=%s", expr))
	if err != nil {
		return fmt.Errorf("no pude actualizar la cara: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Correr — debe pasar**

Run: `nix shell nixpkgs#go --command go test ./... -run TestEwwFace -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add display.go display_test.go
git commit -m "feat: Display + EwwFace (misma interfaz que usará el ESP32)"
```

---

### Task 3: Action + registro (acciones simples)

**Files:**
- Create: `action.go`, `actions.go`, `actions_test.go`

**Interfaces:**
- Consumes: `Runner`, `Expression`.
- Produces: `type Action struct { Name string; Face Expression; Run func(r Runner) (string, error) }`; `func buildActions(now func() time.Time) map[string]*Action` (claves: `"saludar"`, `"hora"`, `"dormir"`; más en Task 5).

- [ ] **Step 1: Escribir `action.go`**

Concepto (importante): `Action` es **struct con un campo función**, NO interfaz. Todas las acciones tienen la misma forma y una sola implementación → meter una interfaz sería abstracción sin pagar (YAGNI). La interfaz se gana donde SÍ hay varias implementaciones (Display, Interpreter). Ésta es la regla: abstraé cuando hay dos, no "por si acaso".

```go
package main

// Action es una capacidad de Astro. Struct (no interfaz) porque todas tienen la
// misma forma y una sola implementación: la interfaz se gana donde hay varias.
type Action struct {
	Name string
	Face Expression
	Run  func(r Runner) (reply string, err error)
}
```

- [ ] **Step 2: Test que falla (`actions_test.go`)**

Concepto: `now` se **inyecta** para testear la hora sin depender del reloj real (mismo patrón que el Runner falso).

```go
package main

import (
	"testing"
	"time"
)

func fixedClock() time.Time {
	return time.Date(2026, 7, 23, 9, 5, 0, 0, time.UTC)
}

func TestSaludarReply(t *testing.T) {
	actions := buildActions(fixedClock)
	reply, err := actions["saludar"].Run(&fakeRunner{})
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if reply == "" {
		t.Fatal("esperaba un saludo no vacío")
	}
}

func TestHoraUsaElReloj(t *testing.T) {
	actions := buildActions(fixedClock)
	reply, err := actions["hora"].Run(&fakeRunner{})
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if want := "09:05"; !contains(reply, want) {
		t.Fatalf("esperaba que incluyera %q, fue %q", want, reply)
	}
}

func contains(s, sub string) bool { return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 3: Correr — debe fallar**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestSaludar|TestHora'`
Expected: FALLA (`buildActions` no existe).

- [ ] **Step 4: Escribir `actions.go` (parte 1: acciones simples)**

```go
package main

import "time"

// buildActions arma el registro. 'now' se inyecta para testear la hora sin el reloj real.
func buildActions(now func() time.Time) map[string]*Action {
	actions := map[string]*Action{}
	add := func(a *Action) { actions[a.Name] = a }

	add(&Action{
		Name: "saludar", Face: Feliz,
		Run: func(r Runner) (string, error) { return "¡Hola! Soy Astro.", nil },
	})
	add(&Action{
		Name: "hora", Face: Neutral,
		Run: func(r Runner) (string, error) {
			return "Son las " + now().Format("15:04") + ".", nil
		},
	})
	add(&Action{
		Name: "dormir", Face: Dormido,
		Run: func(r Runner) (string, error) { return "Me duermo… 💤", nil },
	})
	return actions
}
```

- [ ] **Step 5: Correr — debe pasar**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestSaludar|TestHora' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add action.go actions.go actions_test.go
git commit -m "feat: Action (struct) + registro con saludar/hora/dormir"
```

---

### Task 4: Interpreter (reglas)

**Files:**
- Create: `interpreter.go`, `interpreter_test.go`

**Interfaces:**
- Consumes: `map[string]*Action`, `Action`.
- Produces: `type Interpreter interface { Interpret(text string) (*Action, error) }`; `NewRuleInterpreter(actions map[string]*Action) *RuleInterpreter`; `var ErrNoEntiendo error`.

- [ ] **Step 1: Test que falla (`interpreter_test.go`)**

```go
package main

import (
	"errors"
	"testing"
	"time"
)

func newTestInterpreter() Interpreter {
	return NewRuleInterpreter(buildActions(time.Now))
}

func TestInterpretaSaludo(t *testing.T) {
	action, err := newTestInterpreter().Interpret("hola astro")
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if action.Name != "saludar" {
		t.Fatalf("esperaba 'saludar', fue %q", action.Name)
	}
}

func TestNoEntiende(t *testing.T) {
	_, err := newTestInterpreter().Interpret("xyzzy")
	if !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("esperaba ErrNoEntiendo, fue %v", err)
	}
}
```

- [ ] **Step 2: Correr — debe fallar**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestInterpreta|TestNoEntiende'`
Expected: FALLA (`NewRuleInterpreter` no existe).

- [ ] **Step 3: Escribir `interpreter.go`**

Concepto: `Interpreter` es **interfaz** — hoy reglas, en la Fase 5 un LLM, sin tocar el resto. La regla de "abrí X" arma un `Action` al vuelo con una **closure** que captura el nombre de la app (así `Action` sigue simple, sin parámetros).

```go
package main

import (
	"errors"
	"strings"
)

// ErrNoEntiendo se devuelve cuando ningún patrón matchea.
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
	t := strings.ToLower(strings.TrimSpace(text))
	switch {
	case containsAny(t, "hola", "buenas"):
		return ri.actions["saludar"], nil
	case containsAny(t, "hora"):
		return ri.actions["hora"], nil
	case containsAny(t, "paus", "seguí", "reproduc"):
		return ri.actions["pausar"], nil
	case containsAny(t, "subí", "sube", "más volumen"):
		return ri.actions["subir_volumen"], nil
	case containsAny(t, "dormí", "dormite", "chau"):
		return ri.actions["dormir"], nil
	case strings.HasPrefix(t, "abrí") || strings.HasPrefix(t, "abri"):
		parts := strings.SplitN(t, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return nil, ErrNoEntiendo
		}
		return openAppAction(strings.TrimSpace(parts[1])), nil
	default:
		return nil, ErrNoEntiendo
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
```

Nota: las claves `"pausar"`/`"subir_volumen"` y `openAppAction` se agregan en la Task 5; este archivo
compila igual porque solo las referencia (y `openAppAction` se define en la Task 5 antes de correr `main`).
Para que los tests de esta task compilen, la Task 5 debe hacerse a continuación (o comentar esos `case`
temporalmente). El plan asume orden 4→5.

- [ ] **Step 4: (después de Task 5) Correr — debe pasar**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestInterpreta|TestNoEntiende' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add interpreter.go interpreter_test.go
git commit -m "feat: Interpreter (interfaz) + RuleInterpreter por palabras clave"
```

---

### Task 5: Acciones de escritorio (media, volumen, abrir)

**Files:**
- Modify: `actions.go` (agregar acciones), `actions_test.go` (agregar tests)

**Interfaces:**
- Consumes: `Runner`, `Action`.
- Produces: en `buildActions` las claves `"pausar"`, `"subir_volumen"`; y `func openAppAction(app string) *Action`.

- [ ] **Step 1: Tests que fallan (agregar a `actions_test.go`)**

Concepto: verificamos el **comando exacto** con el fake, sin abrir nada real. Abrir apps va por `hyprctl dispatch exec` (no bloquea y respeta tus reglas de Hyprland — igual que tus binds).

```go
func TestPausarLlamaPlayerctl(t *testing.T) {
	actions := buildActions(fixedClock)
	fake := &fakeRunner{}
	if _, err := actions["pausar"].Run(fake); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	got := fake.lastCall()
	if len(got) < 2 || got[0] != "playerctl" || got[1] != "play-pause" {
		t.Fatalf("esperaba playerctl play-pause, fue %v", got)
	}
}

func TestAbrirUsaHyprctl(t *testing.T) {
	fake := &fakeRunner{}
	action := openAppAction("firefox")
	if _, err := action.Run(fake); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	got := fake.lastCall()
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if len(got) != 4 || got[3] != "firefox" || got[0] != "hyprctl" {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}
```

- [ ] **Step 2: Correr — debe fallar**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestPausar|TestAbrir'`
Expected: FALLA (`openAppAction` / clave `"pausar"` no existen).

- [ ] **Step 3: Agregar acciones a `actions.go`**

Dentro de `buildActions`, antes del `return`, agregar:

```go
	add(&Action{
		Name: "pausar", Face: Feliz,
		Run: func(r Runner) (string, error) {
			if _, err := r.Run("playerctl", "play-pause"); err != nil {
				return "", fmt.Errorf("no pude controlar la música: %w", err)
			}
			return "Listo.", nil
		},
	})
	add(&Action{
		Name: "subir_volumen", Face: Neutral,
		Run: func(r Runner) (string, error) {
			if _, err := r.Run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "5%+"); err != nil {
				return "", fmt.Errorf("no pude cambiar el volumen: %w", err)
			}
			return "Subí el volumen.", nil
		},
	})
```

Y al final del archivo, la acción dinámica de abrir (fuera de `buildActions`):

```go
// openAppAction arma una acción al vuelo que abre 'app'. Usa `hyprctl dispatch exec`
// porque no bloquea y respeta las reglas de Hyprland (igual que tus keybinds).
func openAppAction(app string) *Action {
	return &Action{
		Name: "abrir_" + app, Face: Curioso,
		Run: func(r Runner) (string, error) {
			if _, err := r.Run("hyprctl", "dispatch", "exec", app); err != nil {
				return "", fmt.Errorf("no pude abrir %s: %w", app, err)
			}
			return "Abriendo " + app + ".", nil
		},
	}
}
```

Agregar `"fmt"` al import de `actions.go`.

- [ ] **Step 4: Correr TODO — debe pasar (incluye los tests de Task 4)**

Run: `nix shell nixpkgs#go --command go test ./... -v`
Expected: PASS (runner, display, actions, interpreter).

- [ ] **Step 5: Commit**

```bash
git add actions.go actions_test.go
git commit -m "feat: acciones de escritorio (media, volumen, abrir vía hyprctl)"
```

---

### Task 6: main (wiring + loop)

**Files:**
- Create: `main.go`

**Interfaces:**
- Consumes: `ExecRunner`, `EwwFace`, `buildActions`, `NewRuleInterpreter`, `Interpreter`, `Display`, `ErrNoEntiendo`.

- [ ] **Step 1: Escribir `main.go`**

Concepto: acá se ve la **inyección de dependencias** — armamos las piezas reales y las conectamos. El loop lee stdin, interpreta, ejecuta, muestra cara y responde. `main` es fino: la lógica ya está testeada en las piezas.

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
	var display Display = EwwFace{Runner: runner}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	_ = display.Show(Neutral) // arranca neutral

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
			fmt.Println("No te entendí. Probá: hola · qué hora es · pausá · subí volumen · abrí firefox · dormí")
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
	fmt.Println("\nChau 👋")
}
```

- [ ] **Step 2: Compila y tests siguen verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go test ./...'`
Expected: compila sin errores; PASS.

- [ ] **Step 3: Verificación manual (sin eww todavía → los `Show` van a fallar callados, está OK)**

Run: `nix shell nixpkgs#go --command go run .`
Escribir: `hola` ↵ , `qué hora es` ↵ , `xyz` ↵ , `abrí kitty` ↵ , Ctrl+D.
Expected: responde el saludo, la hora, el mensaje de "no te entendí", y abre kitty. (Las caras aún no se ven hasta la Task 7; el error de `eww` se ignora con `_`.)

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: main — wiring + loop de comandos por terminal"
```

---

### Task 7: Widget de eww + caras

**Files:**
- Create: `tools/genfaces.py`, `eww/eww.yuck`, `eww/eww.scss`, `eww/faces/*.png` (generados)

- [ ] **Step 1: Copiar el generador de caras a `tools/genfaces.py`**

Concepto: dibuja las 13 caras 128×64 (blanco sobre negro) por primitivas y las guarda como PNG. Es el mismo enfoque que va a usar el firmware. (Contenido completo del script: reutilizar el generador ya validado de la sesión — ver `docs/specs` / historial; produce `neutral, parpadeo, pensativo, feliz, curioso, sorprendido, bostezo, dormido, mareado, guino, amor, triste, enojado`.) El script debe escribir a `eww/faces/<nombre>.png`.

- [ ] **Step 2: Generar los PNG**

Run: `nix shell nixpkgs#python3 --command python3 tools/genfaces.py`
Expected: crea `eww/faces/neutral.png` … `eww/faces/enojado.png` (13 archivos).
Verificar: `ls eww/faces/` muestra los 13.

- [ ] **Step 3: Escribir `eww/eww.yuck`**

Concepto: una ventana `astro` con una variable `astro_face`; la imagen apunta a `faces/${astro_face}.png`. El daemon cambia la variable con `eww update astro_face=...`.

```lisp
(defvar astro_face "neutral")

(defwidget face []
  (box :class "astro"
    (image :path "${EWW_CONFIG_DIR}/faces/${astro_face}.png"
           :image-width 256 :image-height 128)))

(defwindow astro
  :monitor 0
  :geometry (geometry :x "20px" :y "20px" :width "280px" :height "160px" :anchor "top right")
  :stacking "fg"
  (face))
```

- [ ] **Step 4: Escribir `eww/eww.scss`**

```scss
.astro {
  background-color: #060b12;
  border-radius: 14px;
  padding: 10px;
}
```

- [ ] **Step 5: Abrir el widget y verificar en vivo**

Run (apuntando eww a la config del repo):
```bash
eww --config ~/projects/astro/eww open astro
```
Luego, en otra terminal:
```bash
eww --config ~/projects/astro/eww update astro_face=feliz
```
Expected: aparece el widget con la cara `neutral` y cambia a `feliz`.

> Nota: `main.go`/`EwwFace` llaman a `eww` sin `--config`. Para la Fase A, o exportás
> `EWW_CONFIG=~/projects/astro/eww`, o dejás esa config como la default de eww. (Decisión de
> cableado fino; anotarla, no bloquea la lógica.)

- [ ] **Step 6: Verificación end-to-end**

Con el widget abierto, correr `go run .` y escribir comandos: la cara del widget debe cambiar
(`feliz` al saludar, `pensativo` si no entiende, `dormido` al decir "dormí").

- [ ] **Step 7: Commit**

```bash
git add tools/genfaces.py eww/
git commit -m "feat: widget de eww + caras generadas; Astro reacciona en pantalla"
```

---

## Definition of Done (Fase A)

1. `go build ./...` y `go test ./...` verdes.
2. `go run .` + el widget de eww: cada comando de la spec §5 produce efecto real + cara + reply.
3. Comando desconocido → cara `pensativo` + ayuda, sin cortar el loop.
4. Todo respeta las convenciones (ids inglés, comentarios español, nada destructivo).

## Notas de auto-revisión (hechas)

- **Cobertura de spec:** §3 arquitectura → Tasks 1-6; §5 acciones → Tasks 3,5; §6 errores → Task 6
  (ErrNoEntiendo + reply de acción fallida); §7 testing → Runner/Display/Action/Interpreter con fake;
  §8 estructura → File Structure; cara/eww → Task 7. Sin huecos.
- **Dependencia de orden 4↔5:** el `interpreter.go` referencia `"pausar"`, `"subir_volumen"` y
  `openAppAction` que se crean en la Task 5 → **ejecutar 4 y 5 juntas** (o 5 antes de correr los tests de 4).
  Marcado explícito en Task 4, Step 3.
- **Consistencia de tipos:** `Runner.Run(name, args...)`, `Display.Show(Expression)`,
  `Interpreter.Interpret(string)(*Action,error)`, `Action{Name,Face,Run}` — usados igual en todas las tasks.
- **Pendiente menor (no bloquea):** el path de config de eww que usa `EwwFace` (Task 7, Step 5) —
  decisión de cableado, anotada.
