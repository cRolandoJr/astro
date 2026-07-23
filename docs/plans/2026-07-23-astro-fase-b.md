# Astro · Fase B — Plan de Implementación (voz: te escucha y te habla)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Enter → Astro graba → transcribe (Whisper) → interpreta/actúa (pipeline de Fase A) → cambia la
cara → **dice la respuesta en voz** (Piper). Todo orquestando binarios vía `Runner`, testeable con `fakeRunner`.

**Architecture:** Se agrega `InputSource` (interfaz; `VoiceInput` + `StdinInput`) y `PiperVoice` (struct), y
`Runner` gana `RunWithInput` (stdin, para Piper). `Interpreter`/`Action`/`Display` de Fase A no se tocan.

**Tech Stack:** Go 1.26; externos: whisper.cpp, piper-tts, alsa-utils (`arecord`), pipewire (`pw-play`).
Spec: `docs/specs/2026-07-23-astro-fase-b-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nombres descriptivos.
- Nada destructivo; **solo stdlib** en Go. Módulo `github.com/cRolandoJr/astro`, raíz del repo.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- **Regla de abstracción (de Fase A):** interfaz solo si hay ≥2 impls hoy. `InputSource` sí (Voice+Stdin);
  la voz es struct (`PiperVoice`, una impl).

## Preparación del entorno (una vez, para el E2E de la Task 4)

No hace falta para las Tasks 1-3 (solo Go). Para probar de verdad en la Task 4:
- Herramientas (rápido: `nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils`; permanente:
  agregarlas al flake — momento de enseñanza Nix cuando lleguemos).
- Modelo whisper `small`: bajar `ggml-small.bin` (script `download-ggml-model.sh small` de whisper.cpp, o
  fetch del release ggerganov/whisper.cpp) → guardar fuera del repo.
- Voz Piper español: bajar un `.onnx` **+ su `.json`** `es_ES` (releases de rhasspy/piper-voices) → fuera del
  repo. El `.json` va **al lado** del `.onnx` con el mismo nombre (`<voz>.onnx.json`); piper lo carga solo.
- Env: `ASTRO_WHISPER_BIN`, `ASTRO_WHISPER_MODEL`, `ASTRO_PIPER_BIN`, `ASTRO_PIPER_VOICE`, `ASTRO_EWW_CONFIG`.

---

### Task 1: `Runner.RunWithInput` (stdin para Piper)

**Files:** Modify `runner.go`, `fake_test.go`; Create test in `runner_test.go`.
**Produces:** `Runner` gana `RunWithInput(stdin string, name string, args ...string) (string, error)`; `ExecRunner` y `fakeRunner` la implementan; `fakeRunner` guarda el stdin (`lastInput()`).

- [ ] **Step 1: Test que falla (agregar a `runner_test.go`)**

`cat` sin args ecoa su stdin → verificamos que el stdin se pasa de verdad.

```go
func TestExecRunnerRunWithInputPipesStdin(t *testing.T) {
	out, err := ExecRunner{}.RunWithInput("hola\n", "cat")
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if out != "hola" {
		t.Fatalf("esperaba %q, obtuve %q", "hola", out)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run RunWithInput` → FALLA (método no existe).

- [ ] **Step 3: Extender `runner.go`**

Agregá el método al interface y a `ExecRunner`:

```go
type Runner interface {
	Run(name string, args ...string) (string, error)
	RunWithInput(stdin string, name string, args ...string) (string, error)
}

// RunWithInput corre el comando alimentando 'stdin' por la entrada estándar (para
// binarios como piper que leen el texto por stdin).
func (ExecRunner) RunWithInput(stdin string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
```

- [ ] **Step 4: Extender `fakeRunner` (`fake_test.go`)**

Agregá el campo `inputs` y el método; reusa `Run` para registrar la llamada:

```go
// (agregar campo al struct fakeRunner)
	inputs []string // stdin de cada RunWithInput

// (agregar métodos)
func (f *fakeRunner) RunWithInput(stdin string, name string, args ...string) (string, error) {
	f.inputs = append(f.inputs, stdin)
	return f.Run(name, args...)
}

func (f *fakeRunner) lastInput() string {
	if len(f.inputs) == 0 {
		return ""
	}
	return f.inputs[len(f.inputs)-1]
}
```

- [ ] **Step 5: Correr TODO — pasa**

Run: `nix shell nixpkgs#go --command go test ./...` → PASS (nuevo + los 11 previos siguen verdes:
`fakeRunner` ahora satisface el `Runner` extendido).

- [ ] **Step 6: Commit**

```bash
git add runner.go runner_test.go fake_test.go
git commit -m "feat: Runner.RunWithInput (stdin) — habilita TTS por piper"
```

---

### Task 2: `InputSource` + `VoiceInput` + `StdinInput`

**Files:** Create `input.go`, `input_test.go`
**Produces:** `InputSource interface { Listen() (string, error) }`; `VoiceInput` (con `capture()` testeable y `cleanTranscript` puro); `StdinInput`.

- [ ] **Step 1: Tests que fallan (`input_test.go`)**

`cleanTranscript` es función pura (limpia la salida de whisper). `capture()` usa `fakeRunner` + un
`ReadFile` inyectado (whisper escribe a un `.txt`; así no dependemos de stderr ni de audio real).

```go
package main

import (
	"reflect"
	"testing"
)

func TestCleanTranscript(t *testing.T) {
	cases := map[string]string{
		"  Hola Astro.\n":            "Hola Astro.",
		"[BLANK_AUDIO]":              "",
		"[música] pausá":            "pausá",
		"\n  qué hora es \n":         "qué hora es",
	}
	for raw, want := range cases {
		if got := cleanTranscript(raw); got != want {
			t.Errorf("cleanTranscript(%q) = %q, esperaba %q", raw, got, want)
		}
	}
}

func TestVoiceInputCaptureArmaComandosYLimpia(t *testing.T) {
	fake := &fakeRunner{}
	v := VoiceInput{
		Runner: fake, WhisperBin: "whisper-cpp", WhisperModel: "/m/small.bin",
		RecSeconds: 4, WavPath: "/tmp/astro-in.wav",
		ReadFile: func(string) ([]byte, error) { return []byte("  pausá\n"), nil },
	}
	text, err := v.capture()
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if text != "pausá" {
		t.Fatalf("esperaba transcripción limpia 'pausá', fue %q", text)
	}
	// 2 comandos: arecord y whisper
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos (arecord, whisper), hubo %d: %v", len(fake.calls), fake.calls)
	}
	if fake.calls[0][0] != "arecord" {
		t.Errorf("primer comando debería ser arecord, fue %v", fake.calls[0])
	}
	wantWhisper := []string{"whisper-cpp", "-m", "/m/small.bin", "-f", "/tmp/astro-in.wav", "-l", "es", "-nt", "-otxt", "-of", "/tmp/astro-in"}
	if !reflect.DeepEqual(fake.calls[1], wantWhisper) {
		t.Errorf("comando whisper mal armado:\n got  %v\n want %v", fake.calls[1], wantWhisper)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'CleanTranscript|VoiceInputCapture'` → FALLA.

- [ ] **Step 3: `input.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// InputSource entrega el próximo comando como texto. Interfaz porque hay dos impls
// reales: VoiceInput (voz) y StdinInput (teclado, para debug). Swappable por env en main.
type InputSource interface {
	Listen() (text string, err error)
}

// ---- StdinInput: leer una línea (útil para debuggear el pipeline sin micrófono) ----
type StdinInput struct{ scanner *bufio.Scanner }

func NewStdinInput() *StdinInput { return &StdinInput{scanner: bufio.NewScanner(os.Stdin)} }

func (s *StdinInput) Listen() (string, error) {
	fmt.Print("> ")
	if !s.scanner.Scan() {
		return "", io.EOF
	}
	return s.scanner.Text(), nil
}

// ---- VoiceInput: Enter para hablar → grabar → transcribir ----
type VoiceInput struct {
	Runner       Runner
	WhisperBin   string
	WhisperModel string
	RecSeconds   int
	WavPath      string                          // ej. /tmp/astro-in.wav
	ReadFile     func(string) ([]byte, error)    // default os.ReadFile; inyectable en tests
	trigger      *bufio.Scanner
}

func NewVoiceInput(r Runner, whisperBin, whisperModel string, recSeconds int) *VoiceInput {
	return &VoiceInput{
		Runner: r, WhisperBin: whisperBin, WhisperModel: whisperModel,
		RecSeconds: recSeconds, WavPath: "/tmp/astro-in.wav",
		trigger: bufio.NewScanner(os.Stdin),
	}
}

func (v *VoiceInput) Listen() (string, error) {
	fmt.Print("[Enter] para hablar (Ctrl+D para salir) ")
	if !v.trigger.Scan() {
		return "", io.EOF
	}
	return v.capture()
}

// capture graba y transcribe. Es la parte testeable (sin el Enter interactivo).
func (v *VoiceInput) capture() (string, error) {
	secs := v.RecSeconds
	if secs <= 0 {
		secs = 4
	}
	fmt.Printf("🎙️  grabando %ds… hablá ahora\n", secs)
	if _, err := v.Runner.Run("arecord", "-q", "-d", strconv.Itoa(secs),
		"-f", "S16_LE", "-r", "16000", "-c", "1", v.WavPath); err != nil {
		return "", fmt.Errorf("no pude grabar (¿arecord instalado?): %w", err)
	}
	// whisper escribe la transcripción a <of>.txt (-nt: sin timestamps).
	ofPrefix := strings.TrimSuffix(v.WavPath, ".wav")
	if _, err := v.Runner.Run(v.WhisperBin, "-m", v.WhisperModel, "-f", v.WavPath,
		"-l", "es", "-nt", "-otxt", "-of", ofPrefix); err != nil {
		return "", fmt.Errorf("no pude transcribir (¿whisper/modelo?): %w", err)
	}
	readFile := v.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	raw, err := readFile(ofPrefix + ".txt")
	if err != nil {
		return "", fmt.Errorf("no pude leer la transcripción: %w", err)
	}
	return cleanTranscript(string(raw)), nil
}

// cleanTranscript limpia la salida de whisper: saca marcadores entre corchetes
// ([BLANK_AUDIO], [música], etc.), espacios y saltos de línea sobrantes.
func cleanTranscript(raw string) string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = stripBrackets(line)
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// stripBrackets remueve tramos "[...]" (marcadores no-verbales de whisper).
func stripBrackets(s string) string {
	for {
		i := strings.IndexByte(s, '[')
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], ']')
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+1:]
	}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -run 'CleanTranscript|VoiceInputCapture' -v` → PASS.
Verificá también `nix shell nixpkgs#go --command go test ./...` (todo verde).

- [ ] **Step 5: Commit**

```bash
git add input.go input_test.go
git commit -m "feat: InputSource + VoiceInput (grabar+Whisper) + StdinInput"
```

---

### Task 3: `PiperVoice` (TTS)

**Files:** Create `voice.go`, `voice_test.go`
**Produces:** `PiperVoice{ Runner, PiperBin, Voice, WavPath }` con `Say(text string) error`. (Struct, no interfaz — una impl.)

- [ ] **Step 1: Test que falla (`voice_test.go`)**

```go
package main

import (
	"reflect"
	"testing"
)

func TestPiperVoiceSayPasaTextoPorStdinYReproduce(t *testing.T) {
	fake := &fakeRunner{}
	p := PiperVoice{Runner: fake, PiperBin: "piper", Voice: "/v/es.onnx", WavPath: "/tmp/astro-out.wav"}

	if err := p.Say("hola"); err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	// piper recibe el texto por stdin
	if fake.lastInput() != "hola" {
		t.Errorf("esperaba 'hola' por stdin de piper, fue %q", fake.lastInput())
	}
	// 2 comandos: piper (síntesis) y pw-play (reproducción)
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos, hubo %d: %v", len(fake.calls), fake.calls)
	}
	wantPiper := []string{"piper", "--model", "/v/es.onnx", "--output_file", "/tmp/astro-out.wav"}
	if !reflect.DeepEqual(fake.calls[0], wantPiper) {
		t.Errorf("comando piper:\n got  %v\n want %v", fake.calls[0], wantPiper)
	}
	if fake.calls[1][0] != "pw-play" {
		t.Errorf("segundo comando debería ser pw-play, fue %v", fake.calls[1])
	}
}

func TestPiperVoiceSayVacioNoHaceNada(t *testing.T) {
	fake := &fakeRunner{}
	if err := (PiperVoice{Runner: fake, PiperBin: "piper"}).Say(""); err != nil {
		t.Fatalf("no debería fallar con texto vacío: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("texto vacío no debería llamar a nada, hubo %v", fake.calls)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run PiperVoice` → FALLA.

- [ ] **Step 3: `voice.go`**

```go
package main

import "fmt"

// PiperVoice dice un texto en voz alta: piper (texto por stdin) → wav → pw-play.
// Struct (no interfaz): una sola implementación hoy. Si mañana la voz sale del robot,
// en Go extraemos la interfaz sin tocar esto.
type PiperVoice struct {
	Runner   Runner
	PiperBin string
	Voice    string // ruta al .onnx
	WavPath  string // ej. /tmp/astro-out.wav
}

func (p PiperVoice) Say(text string) error {
	if text == "" {
		return nil
	}
	if _, err := p.Runner.RunWithInput(text, p.PiperBin, "--model", p.Voice, "--output_file", p.WavPath); err != nil {
		return fmt.Errorf("no pude sintetizar la voz (¿piper/voz?): %w", err)
	}
	if _, err := p.Runner.Run("pw-play", p.WavPath); err != nil {
		return fmt.Errorf("no pude reproducir el audio: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command go test ./... -run PiperVoice -v` → PASS. Y `go test ./...` todo verde.

- [ ] **Step 5: Commit**

```bash
git add voice.go voice_test.go
git commit -m "feat: PiperVoice — TTS (piper por stdin + pw-play)"
```

---

### Task 4: `main` — cablear voz + E2E

**Files:** Modify `main.go`
**Consumes:** `InputSource`, `VoiceInput`, `StdinInput`, `PiperVoice`, y todo lo de Fase A.

- [ ] **Step 1: Reescribir `main.go`**

Concepto: el loop ahora pide el comando a un `InputSource` (voz o stdin según env), y al final **habla**
la respuesta con `PiperVoice`. El pipeline del medio (interpretar → actuar → cara) es idéntico a Fase A.

```go
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

func main() {
	runner := ExecRunner{}
	var display Display = EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	// Salida de voz (si no hay piper configurado, degradamos a solo-texto).
	voice := PiperVoice{
		Runner: runner, PiperBin: envOr("ASTRO_PIPER_BIN", "piper"),
		Voice: os.Getenv("ASTRO_PIPER_VOICE"), WavPath: "/tmp/astro-out.wav",
	}

	// Entrada: voz por default, stdin si ASTRO_INPUT=stdin (para debug).
	var input InputSource
	if os.Getenv("ASTRO_INPUT") == "stdin" {
		input = NewStdinInput()
	} else {
		secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS"))
		input = NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cpp"),
			os.Getenv("ASTRO_WHISPER_MODEL"), secs)
	}

	_ = display.Show(Neutral)
	fmt.Println("Astro está despierto.")

	for {
		text, err := input.Listen()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "entrada:", err)
			continue
		}
		if text == "" {
			respond(display, voice, Pensativo, "No te escuché, repetí.")
			continue
		}

		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			respond(display, voice, Pensativo, "No te entendí.")
			continue
		}

		reply, err := action.Run(runner)
		if err != nil {
			_ = display.Show(Neutral)
			fmt.Println("Ups:", err) // error de ejecución: dev-facing, solo pantalla
			continue
		}
		respond(display, voice, action.Face, reply)
	}
	fmt.Println("\nChau 👋")
}

// respond muestra la cara, IMPRIME el texto y lo dice en voz alta. Imprimir siempre
// (aunque el TTS esté off) es la degradación que promete la spec §7: si no puede
// hablar, al menos responde por pantalla. Se usa en las 3 ramas de usuario.
func respond(d Display, v PiperVoice, face Expression, text string) {
	_ = d.Show(face)
	fmt.Println(text)
	say(v, text)
}

// say habla, pero si el TTS falla no corta el flujo (ya se mostró el texto).
func say(v PiperVoice, text string) {
	if err := v.Say(text); err != nil {
		fmt.Fprintln(os.Stderr, "(voz off:", err, ")")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 3: Smoke sin audio (modo stdin, para confirmar que el cableado no rompió nada)**

```bash
printf 'hola\nqué hora es\nxyz\n' | ASTRO_INPUT=stdin nix shell nixpkgs#go --command go run . 2>/dev/null
```
Expected: saludo, hora, "no te entendí", "Chau". (El `say` intentará piper y fallará callado si no está — OK.)

- [ ] **Step 4: E2E de voz (manual — requiere micrófono/parlantes; lo hace el usuario)**

Instalar/env (ver "Preparación del entorno") y:
```bash
export ASTRO_EWW_CONFIG=~/projects/astro/eww
export ASTRO_WHISPER_MODEL=/ruta/ggml-small.bin
export ASTRO_PIPER_VOICE=/ruta/es_ES-....onnx
eww --config ~/projects/astro/eww open astro   # (en otra terminal)
nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#go \
  --command go run .
```
Apretás Enter, decís "hola" / "qué hora es" / "pausá" → Astro transcribe, actúa, cambia la cara y
**responde en voz alta**. Ajustar `ASTRO_WHISPER_BIN`/`ASTRO_PIPER_BIN` si los binarios se llaman distinto.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: main — entrada por voz (Whisper) + respuesta hablada (Piper)"
```

---

## Definition of Done (Fase B)

1. `go build/vet/test ./...` verdes (tests nuevos de Runner stdin, VoiceInput/cleanTranscript, PiperVoice).
2. Smoke `ASTRO_INPUT=stdin` funciona (cableado intacto).
3. E2E de voz (manual): Enter → hablás → transcribe → actúa → cara → **responde hablado**.
4. Audio vacío / no entendido → cara `pensativo` + pedido de repetir, sin cortar el loop.
5. Falta binario/modelo → mensaje claro (arecord/whisper/piper), sin crash; TTS que falla degrada a texto.
6. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Cobertura de spec

§3 arquitectura → Tasks 1-4; §4 flujo → Tasks 2-4; §7 errores → Task 4 (vacío/no-entendido/TTS-degrada) +
Tasks 2-3 (mensajes de binario faltante); §8 testing → cleanTranscript puro + capture/Say con fake; §10 files → todas.

## Auto-revisión (hecha)

- **stderr de whisper:** se evita usando `-otxt -of` (whisper escribe a archivo) + `ReadFile` inyectado,
  en vez de parsear stdout+stderr mezclados de `CombinedOutput`. Testeable sin audio.
- **`RunWithInput` en el interface** obliga a `fakeRunner` a implementarlo (Task 1) → los 11 tests de Fase A
  siguen compilando/verdes. Verificado en el orden de tasks (Task 1 toca fake_test.go).
- **Tipos consistentes:** `Runner.RunWithInput(stdin,name,args...)`, `InputSource.Listen()(string,error)`,
  `PiperVoice.Say(string)error` usados igual en todas las tasks y en main.
- **Nombres de binarios por env** (`ASTRO_WHISPER_BIN`/`ASTRO_PIPER_BIN`) → no rompe si en tu Nix se
  llaman distinto (`whisper-cli`, `piper-tts`, etc.).
