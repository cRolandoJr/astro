# Astro · Fase H — Plan de Implementación (activación por voz / wake-word)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Decir "Astro" activa la escucha (sin teclado): un sidecar de wake-word emite un evento y Astro captura el comando con la VAD que ya tiene.

**Architecture:** `WakeWordInput` (nuevo `InputSource`) espera una línea `DETECTED` del stdout de un sidecar (proceso externo, motor de wake) y ahí llama a `VoiceInput.capture()`. El Go se acopla solo a ese contrato de stdout; el motor (Porcupine/openWakeWord) vive en un script Python aparte. Opt-in por `ASTRO_INPUT=wake`.

**Tech Stack:** Go, stdlib (`os/exec` directo para el sidecar long-lived — no encaja en `Runner`). Sidecar Python (Porcupine) como proceso aparte. Spec: `docs/specs/2026-07-23-astro-fase-h-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; **solo stdlib** en el Go (el sidecar es proceso aparte).
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Interfaz `InputSource`/`Interpret` sin cambios; el Enter (VoiceInput) sigue como default; wake es opt-in.
- Fallas ruidosas (sidecar muerto → `Listen` error → main corta y reporta). El sidecar se cierra al salir.
- Cada tarea termina **compilando y con todos los tests verdes** (A/B/C/D/E/F/G incluidos).

---

### Task 1: `wakeword.go` — `WakeWordInput` (InputSource)

**Files:** Create `wakeword.go`, `wakeword_test.go`
**Interfaces:**
- Consumes: `VoiceInput` + su `capture()` (Fase F), `InputSource` (input.go).
- Produces: `WakeWordInput` (implementa `InputSource`); `NewWakeWordInput(wakeCmd string, voice *VoiceInput) (*WakeWordInput, error)`; `(*WakeWordInput).Close() error`.

- [ ] **Step 1: Tests que fallan (`wakeword_test.go`)**

```go
package main

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWakeWordListenDisparaCapturaTrasEvento(t *testing.T) {
	fake := &fakeRunner{}
	voice := &VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("qué hora es"), nil },
	}
	w := &WakeWordInput{events: bufio.NewScanner(strings.NewReader("DETECTED\n")), voice: voice}
	got, err := w.Listen()
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if got != "qué hora es" {
		t.Fatalf("esperaba el comando capturado tras el wake, fue %q", got)
	}
	// tras el wake, se grabó (rec) y transcribió (whisper) — el primer comando es la captura
	if len(fake.calls) < 1 || fake.calls[0][0] != "timeout" || fake.calls[0][2] != "rec" {
		t.Fatalf("esperaba captura (timeout rec …) tras el wake, fue %v", fake.calls)
	}
}

func TestWakeWordListenSidecarMuertoDevuelveEOF(t *testing.T) {
	// stdout cerrado/vacío = el sidecar murió → Scan false sin error → EOF (main corta limpio)
	w := &WakeWordInput{events: bufio.NewScanner(strings.NewReader("")), voice: &VoiceInput{Runner: &fakeRunner{}}}
	if _, err := w.Listen(); !errors.Is(err, io.EOF) {
		t.Fatalf("sidecar muerto debía devolver io.EOF, fue %v", err)
	}
}

func TestNewWakeWordInputSinCmdError(t *testing.T) {
	if _, err := NewWakeWordInput("", &VoiceInput{}); err == nil {
		t.Fatal("sin ASTRO_WAKE_CMD debía dar error")
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestWakeWord|TestNewWakeWord'` → FALLA.

- [ ] **Step 3: Implementar `wakeword.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// WakeWordInput espera a que un sidecar (motor de wake-word) emita "DETECTED" por stdout y ahí
// captura el comando reusando la grabación por voz. Implementa InputSource.
type WakeWordInput struct {
	events *bufio.Scanner // stdout del sidecar: una línea por activación
	voice  *VoiceInput    // captura el comando tras el wake (VAD + whisper)
	cmd    *exec.Cmd      // el sidecar, para cerrarlo (nil si se inyectó el scanner en tests)
}

// NewWakeWordInput lanza el sidecar (wakeCmd, ej. "python .../wake.py") vía sh -c (permite flags)
// y toma su stdout como stream de eventos. El sidecar es long-lived → os/exec directo, no Runner.
func NewWakeWordInput(wakeCmd string, voice *VoiceInput) (*WakeWordInput, error) {
	if wakeCmd == "" {
		return nil, fmt.Errorf("modo wake: falta ASTRO_WAKE_CMD (comando del sidecar)")
	}
	cmd := exec.Command("sh", "-c", wakeCmd)
	cmd.Stderr = os.Stderr // los logs del sidecar (stderr) quedan visibles
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("no pude tomar el stdout del sidecar: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no pude lanzar el sidecar de wake (%q): %w", wakeCmd, err)
	}
	return &WakeWordInput{events: bufio.NewScanner(out), voice: voice, cmd: cmd}, nil
}

func (w *WakeWordInput) Listen() (string, error) {
	fmt.Println("💤 esperando \"Astro\"…")
	if !w.events.Scan() {
		if err := w.events.Err(); err != nil {
			return "", err
		}
		return "", io.EOF // el sidecar cerró su stdout (murió) → fin del loop
	}
	fmt.Println("👂 ¡te escucho!")
	return w.voice.capture()
}

// Close mata el sidecar. main lo llama con defer y en el handler de señal (para no dejarlo colgado).
func (w *WakeWordInput) Close() error {
	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}
	return w.cmd.Process.Kill()
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 5: Commit**

```bash
git add wakeword.go wakeword_test.go
git commit -m "feat: WakeWordInput — espera 'DETECTED' del sidecar → capture(); Close mata el sidecar"
```

---

### Task 2: wiring en `main.go` + sidecar de referencia + E2E

**Files:** Modify `main.go`, `secrets.env.example`; Create `scripts/wake.py`

- [ ] **Step 1: `main.go` — extraer `newVoice` y agregar el modo wake**

Reemplazá el bloque de selección de entrada (el `if os.Getenv("ASTRO_INPUT") == "stdin" { … } else { … }`) por:
```go
	var input InputSource
	var wake *WakeWordInput
	switch os.Getenv("ASTRO_INPUT") {
	case "stdin":
		input = NewStdinInput()
	case "wake":
		w, err := NewWakeWordInput(os.Getenv("ASTRO_WAKE_CMD"), newVoice(runner))
		if err != nil {
			fmt.Fprintln(os.Stderr, "modo wake:", err)
			return
		}
		wake = w
		defer wake.Close()
		input = wake
	default:
		input = newVoice(runner)
	}
```
Agregá el helper (junto a `envOr`):
```go
// newVoice arma la entrada por voz (grabación por silencio + whisper) desde el entorno.
func newVoice(runner Runner) *VoiceInput {
	secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS"))
	vi := NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cli"),
		os.Getenv("ASTRO_WHISPER_MODEL"), secs)
	vi.SilencePct = os.Getenv("ASTRO_REC_SILENCE_PCT")
	vi.TrailSec = os.Getenv("ASTRO_REC_TRAIL_SEC")
	return vi
}
```
En la goroutine del handler de señal, cerrá también el sidecar (agregá la línea del `if wake != nil`):
```go
	go func() {
		<-sigCh
		_ = face.Close()
		if wake != nil {
			_ = wake.Close()
		}
		os.Exit(0)
	}()
```

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS
(nada cambia de comportamiento salvo el nuevo modo `wake`; default sigue Enter).

- [ ] **Step 3: Crear el sidecar de referencia `scripts/wake.py`**

```python
#!/usr/bin/env python3
"""Sidecar de wake-word para Astro (motor: Porcupine).
Escucha el micrófono y emite 'DETECTED' por STDOUT al oír la palabra "Astro".
Los logs van a STDERR (stdout es SOLO el contrato de eventos que lee el daemon Go).

Requisitos (los provee el usuario, no el repo):
  - pip install pvporcupine pvrecorder   (o vía nix)
  - PICOVOICE_ACCESS_KEY  (key free de Picovoice)
  - ASTRO_WAKE_PPN=/ruta/astro.ppn  (modelo custom "Astro" generado en la consola de Picovoice)
Uso: PICOVOICE_ACCESS_KEY=... ASTRO_WAKE_PPN=... python scripts/wake.py
"""
import os
import sys

import pvporcupine
from pvrecorder import PvRecorder


def main():
    key = os.environ["PICOVOICE_ACCESS_KEY"]
    ppn = os.environ["ASTRO_WAKE_PPN"]
    porcupine = pvporcupine.create(access_key=key, keyword_paths=[ppn])
    recorder = PvRecorder(frame_length=porcupine.frame_length)
    recorder.start()
    print("wake: escuchando 'Astro'…", file=sys.stderr, flush=True)
    try:
        while True:
            if porcupine.process(recorder.read()) >= 0:
                print("DETECTED", flush=True)  # STDOUT: el evento que lee Astro
    except KeyboardInterrupt:
        pass
    finally:
        recorder.stop()
        recorder.delete()
        porcupine.delete()


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: Documentar el secreto en `secrets.env.example`**

Agregá al final de `secrets.env.example`:
```sh
# Modo wake-word (ASTRO_INPUT=wake) — solo si usás activación por voz:
# export PICOVOICE_ACCESS_KEY='tu-access-key-free-de-picovoice'
# export ASTRO_WAKE_PPN="$HOME/modelos/astro.ppn"   # modelo custom generado en la consola de Picovoice
# export ASTRO_WAKE_CMD='python '"$PWD"'/scripts/wake.py'
```

- [ ] **Step 5: E2E (manual — necesita micrófono + key + modelo + python; lo corre el usuario)**

```bash
# instalar el motor (elegí uno): nix-shell -p python3Packages.pvporcupine  (o pip en un venv)
cd ~/projects/astro && git switch fase-h
# en secrets.env: descomentar/pegar PICOVOICE_ACCESS_KEY, ASTRO_WAKE_PPN, ASTRO_WAKE_CMD
export ASTRO_INPUT=wake
# ...los ASTRO_LLM_* / ASTRO_PIPER_* / etc. de siempre...
nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#sox nixpkgs#grim \
  nixpkgs#python3Packages.pvporcupine nixpkgs#python3Packages.pvrecorder nixpkgs#go --command go run .
# decir "Astro" → debería mostrar "¡te escucho!" y grabar el comando; sin hablar, nada;
# matar el sidecar → el loop corta con mensaje.
```
NOTA: si `pvporcupine`/`pvrecorder` no están en nixpkgs con esos nombres, usar un venv con `pip install`.
Si `rec` (captura) no engancha con el sidecar activo (contención de micro), es el gatillo para pausar el
sidecar durante la captura (spec §6).

- [ ] **Step 6: Commit**

```bash
git add main.go scripts/wake.py secrets.env.example
git commit -m "feat: modo wake (ASTRO_INPUT=wake) — wiring + sidecar Porcupine de referencia"
```

---

## Definition of Done (Fase H)

1. `go build/vet/test ./...` verdes (WakeWordInput dispara `capture` tras el evento; sidecar muerto → EOF; sin cmd → error; + A-G intactos).
2. E2E: "Astro" activa la escucha y el comando se ejecuta; sin hablar no pasa nada; matar el sidecar corta el loop con mensaje.
3. Interfaz `InputSource`/`Interpret`/voz/memoria/visión intactas; el Enter (default) sigue funcionando.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib en el Go.

## Auto-revisión (hecha)

- **Cobertura del spec:** `WakeWordInput` + seam + Close → Task 1; wiring `ASTRO_INPUT=wake` + ciclo de vida + sidecar de referencia + secreto → Task 2.
- **Seam testeable:** los tests construyen `WakeWordInput` con un `bufio.Scanner` sobre `strings.Reader` + `VoiceInput{fakeRunner}` → `Listen()` verificado sin micrófono ni motor (white-box, mismo paquete, como el resto del repo).
- **Long-lived vía os/exec directo:** justificado (un stream continuo no encaja en `Runner.Run`, que corre-hasta-terminar); sigue siendo stdlib.
- **Fallas ruidosas:** sidecar muerto → `Scan` false → `io.EOF` → `main` corta y reporta (no silenciosa). Sin cmd → error al construir → `return` (no arranca a medias).
- **Ciclo de vida:** `defer wake.Close()` (salida normal) + cierre en el handler de señal (Ctrl+C) → no queda el sidecar colgado tomando el micro. `Close` es nil-safe (cmd/Process nil).
- **No rompe lo previo:** el `default` del switch reusa el `newVoice` extraído (mismo comportamiento Enter); `stdin` igual; solo se agrega `wake`.
- **Bordes diferidos (spec):** re-trigger durante captura y contención de micro → validar/gatillar en E2E, no código especulativo ahora.
