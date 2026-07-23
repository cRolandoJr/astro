# Astro · Fase J — Plan de Implementación (activación por hotkey)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** `SUPER+SHIFT+A` activa a Astro (sin Enter, sin wake-word pago): un FIFO dispara la escucha con la VAD que ya existe.

**Architecture:** `HotkeyInput` (nuevo `InputSource`) abre un FIFO `O_RDWR` (no EOF entre disparos) y en `Listen()` bloquea hasta una línea → `voice.capture()`. El bind de Hyprland escribe al FIFO. Reusa el seam disparo→captura de Fase H; interfaz `Interpret` sin cambios.

**Tech Stack:** Go, stdlib (`syscall.Mkfifo`, `os`, `bufio`). Bind en los dotfiles. Spec: `docs/specs/2026-07-23-astro-fase-j-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; **solo stdlib**.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Interfaz `InputSource`/`Interpret` sin cambios; el Enter (default VoiceInput) sigue disponible.
- Cada tarea termina **compilando y con todos los tests verdes**.

---

### Task 1: `hotkey.go` — `HotkeyInput` (InputSource)

**Files:** Create `hotkey.go`, `hotkey_test.go`
**Interfaces:**
- Consumes: `VoiceInput` + `capture()` (input.go), `InputSource`.
- Produces: `HotkeyInput` (implementa `InputSource`); `NewHotkeyInput(fifoPath string, voice *VoiceInput) (*HotkeyInput, error)`; `(*HotkeyInput).Close() error`.

- [ ] **Step 1: Tests que fallan (`hotkey_test.go`)**

```go
package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestHotkeyListenDisparaCaptura(t *testing.T) {
	fake := &fakeRunner{}
	voice := &VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("qué hora es"), nil },
	}
	h := &HotkeyInput{events: bufio.NewScanner(strings.NewReader("go\n")), voice: voice}
	got, err := h.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got != "qué hora es" {
		t.Fatalf("esperaba el comando capturado tras el disparo, fue %q", got)
	}
	if len(fake.calls) < 1 || fake.calls[0][0] != "timeout" || fake.calls[0][2] != "rec" {
		t.Fatalf("esperaba captura (timeout rec …) tras el disparo, fue %v", fake.calls)
	}
}

func TestNewHotkeyInputSinFifoError(t *testing.T) {
	if _, err := NewHotkeyInput("", &VoiceInput{}); err == nil {
		t.Fatal("sin ASTRO_TRIGGER_FIFO debía dar error")
	}
}

func TestHotkeyCloseNilSafe(t *testing.T) {
	h := &HotkeyInput{events: bufio.NewScanner(strings.NewReader(""))}
	if err := h.Close(); err != nil {
		t.Fatalf("Close debía ser nil-safe, fue %v", err)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestHotkey|TestNewHotkey'` → FALLA.

- [ ] **Step 3: Implementar `hotkey.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
)

// HotkeyInput espera un disparo por un FIFO (lo escribe un bind de Hyprland) y ahí captura el
// comando reusando la grabación por voz. Implementa InputSource.
type HotkeyInput struct {
	events *bufio.Scanner // líneas del FIFO: una por activación del atajo
	voice  *VoiceInput    // captura el comando tras el disparo (VAD + whisper)
	fifo   *os.File       // el FIFO abierto O_RDWR (nil si se inyectó el scanner en tests)
	path   string         // ruta del FIFO, para borrarlo en Close
}

// NewHotkeyInput crea el FIFO si falta y lo abre O_RDWR. Abrir con lectura+escritura mantiene un
// escritor propio abierto → el Scanner nunca recibe EOF entre disparos (el FIFO no se "termina").
func NewHotkeyInput(fifoPath string, voice *VoiceInput) (*HotkeyInput, error) {
	if fifoPath == "" {
		return nil, fmt.Errorf("modo hotkey: falta ASTRO_TRIGGER_FIFO (ruta del FIFO)")
	}
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("no pude crear el FIFO %q: %w", fifoPath, err)
	}
	f, err := os.OpenFile(fifoPath, os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("no pude abrir el FIFO %q: %w", fifoPath, err)
	}
	return &HotkeyInput{events: bufio.NewScanner(f), voice: voice, fifo: f, path: fifoPath}, nil
}

func (h *HotkeyInput) Listen() (string, error) {
	fmt.Println("⌨️  esperando el atajo (SUPER+SHIFT+A)…")
	if !h.events.Scan() {
		if err := h.events.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("el FIFO de disparo se cerró")
	}
	fmt.Println("👂 ¡te escucho!")
	return h.voice.capture()
}

// Close cierra el FIFO y lo borra. main lo llama con defer y en el handler de señal. Nil-safe.
func (h *HotkeyInput) Close() error {
	if h.fifo != nil {
		_ = h.fifo.Close()
	}
	if h.path != "" {
		_ = os.Remove(h.path)
	}
	return nil
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS
(nuevos + todo lo previo; `HotkeyInput` queda como tipo sin consumidor hasta Task 2 — compila igual).

- [ ] **Step 5: Commit**

```bash
git add hotkey.go hotkey_test.go
git commit -m "feat: HotkeyInput — FIFO O_RDWR dispara capture(); Close borra el FIFO"
```

---

### Task 2: wiring en `main.go` + `run.sh`

**Files:** Modify `main.go`, `run.sh`

- [ ] **Step 1: `main.go` — caso `hotkey` + ciclo de vida**

En la declaración de vars de entrada, junto a `var wake *WakeWordInput`, agregá:
```go
	var hotkey *HotkeyInput
```
En el `switch os.Getenv("ASTRO_INPUT")`, agregá el caso (antes del `default`):
```go
	case "hotkey":
		h, err := NewHotkeyInput(envOr("ASTRO_TRIGGER_FIFO", "/tmp/astro-trigger.fifo"), newVoice(runner))
		if err != nil {
			fmt.Fprintln(os.Stderr, "modo hotkey:", err)
			return
		}
		hotkey = h
		defer hotkey.Close()
		input = hotkey
```
En la goroutine del handler de señal, cerrá también el hotkey (junto al `if wake != nil`):
```go
		if hotkey != nil {
			_ = hotkey.Close()
		}
```

- [ ] **Step 2: `run.sh` — hotkey por default**

Agregá, junto a los otros `export` de config (antes del `exec nix shell`):
```sh
# Activación por voz sin Enter: apretás SUPER+SHIFT+A (bind de Hyprland) y Astro escucha una vez.
export ASTRO_INPUT=hotkey
```

- [ ] **Step 3: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 4: El bind de Hyprland (lo aplica el usuario/controlador aparte — dotfiles)**

En `~/projects/dotfiles/hypr/.config/hypr/configs/binds.conf` agregar:
```
bind = SUPER SHIFT, A, exec, timeout 0.3 bash -c 'echo go > /tmp/astro-trigger.fifo' || true
```
Luego `hyprctl reload`. (Es la fuente declarativa symlinkeada — editar el origen.)

- [ ] **Step 5: E2E (manual — necesita entorno gráfico + micrófono; lo corre el usuario)**

```bash
cd ~/projects/astro && git switch fase-j && astro
# en otra ventana/foco: apretá SUPER+SHIFT+A → Astro debe mostrar "👂 ¡te escucho!" y grabar el comando
# sin apretar: no pasa nada. Con Astro apagado: el bind no cuelga Hyprland (timeout-guard).
```

- [ ] **Step 6: Commit**

```bash
git add main.go run.sh
git commit -m "feat: modo hotkey (ASTRO_INPUT=hotkey) — wiring + default en run.sh"
```

---

## Definition of Done (Fase J)

1. `go build/vet/test ./...` verdes (Listen dispara capture; sin FIFO → error; Close nil-safe; + todo lo previo).
2. E2E: `SUPER+SHIFT+A` activa la escucha y ejecuta el comando; sin apretar, nada; Astro apagado no cuelga Hyprland.
3. Interfaz `InputSource`/`Interpret`/voz/memoria/visión intactas; Enter sigue disponible.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib (Go).

## Auto-revisión (hecha)

- **Cobertura del spec:** `HotkeyInput` + FIFO O_RDWR + Close → Task 1; wiring + run.sh → Task 2; bind → Task 2 Step 4.
- **Compila en cada tarea:** Task 1 crea tipo nuevo sin consumidor (compila); Task 2 lo cablea. Nada roto entre tareas.
- **Seam testeable:** tests construyen `HotkeyInput` con `bufio.Scanner` sobre `strings.Reader` + `VoiceInput{fakeRunner}` → `Listen()` sin FIFO ni micrófono (white-box, como `WakeWordInput`).
- **Nil-safe:** `TestHotkeyCloseNilSafe`; sin FIFO → error al construir (`TestNewHotkeyInputSinFifoError`).
- **No EOF entre disparos:** `O_RDWR` mantiene un escritor abierto (documentado). El bind con `timeout-guard` evita colgar Hyprland si Astro está apagado.
- **No rompe lo previo:** el `switch` agrega `hotkey` sin tocar `stdin`/`wake`/`default`; `newVoice` reusado.
- **YAGNI:** `HotkeyInput` propio (no se generaliza con `WakeWordInput`, que quedó sin uso); FIFO (no socket/señal).
