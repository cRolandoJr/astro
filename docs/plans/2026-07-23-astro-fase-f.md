# Astro · Fase F — Plan de Implementación (conversación natural)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Grabar por silencio (VAD, corta al callarte) y mantener el hilo de la charla en curso (contexto multi-turno).

**Architecture:** Parte A en `input.go` (sox `rec` + `silence` con tope `timeout`, reemplaza `arecord`).
Parte B en `llm.go`: `chatFunc` gana historial de mensajes; `LLMInterpreter` guarda una ventana rodante
efímera con reset por inactividad. `NewLLMInterpreter` pasa a un **config struct** (cruza el umbral de params
que el mentor marcó). La interfaz `Interpreter`, la voz, la cara, la seguridad y la memoria persistente no cambian.

**Tech Stack:** Go, stdlib. `sox`/`rec`, `timeout` (coreutils) y `whisper` como binarios externos vía `Runner`
(ya verificado que `rec` engancha con PipeWire). Spec: `docs/specs/2026-07-23-astro-fase-f-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; **solo stdlib**.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- La interfaz `Interpret(text) (*Action, error)` NO cambia; seguridad de `open` intacta; memoria persistente (Fase E) intacta.
- Cada tarea termina **compilando y con todos los tests verdes** (incluidos A/B/C/D/E).

---

### Task 1: Grabación por silencio (VAD) en `input.go`

**Files:** Modify `input.go`, `input_test.go` (⚠️ **ya existe** — NO crear)
**Produces:** `VoiceInput.capture()` graba con `sox rec` + `silence` (tope duro con `timeout`).

- [ ] **Step 1: Modificar los tests (`input_test.go` ya existe)**

El archivo ya tiene `TestCleanTranscript` (**dejalo intacto**) y `TestVoiceInputCaptureArmaComandosYLimpia`
(usa `RecSeconds`/`arecord` — hay que reemplazarlo). Pasos:
- Agregá `"fmt"` al bloque de imports (`reflect` y `testing` ya están).
- **Reemplazá** `TestVoiceInputCaptureArmaComandosYLimpia` por los dos tests de abajo. **Conservá** `TestCleanTranscript`.

```go
func TestVoiceCaptureGrabaHastaSilencio(t *testing.T) {
	fake := &fakeRunner{}
	v := VoiceInput{
		Runner: fake, WhisperBin: "whisper-cli", WhisperModel: "m.bin",
		WavPath: "/tmp/astro-in.wav", MaxSeconds: 30,
		ReadFile: func(string) ([]byte, error) { return []byte("hola astro"), nil },
	}
	got, err := v.capture()
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if got != "hola astro" {
		t.Fatalf("esperaba la transcripción, fue %q", got)
	}
	if len(fake.calls) != 2 {
		t.Fatalf("esperaba 2 comandos (rec, whisper), hubo %d: %v", len(fake.calls), fake.calls)
	}
	// comando 0: timeout <max> rec ... silence ...
	wantRec := []string{"timeout", "30", "rec", "-q", "-c", "1", "-r", "16000", "/tmp/astro-in.wav",
		"silence", "1", "0.1", "3%", "1", "1.5", "3%"}
	if !reflect.DeepEqual(fake.calls[0], wantRec) {
		t.Fatalf("comando de grabación:\n esperaba %v\n fue      %v", wantRec, fake.calls[0])
	}
	// comando 1: whisper (armado que se conserva de la versión anterior)
	wantWhisper := []string{"whisper-cli", "-m", "m.bin", "-f", "/tmp/astro-in.wav", "-l", "es", "-nt", "-otxt", "-of", "/tmp/astro-in"}
	if !reflect.DeepEqual(fake.calls[1], wantWhisper) {
		t.Fatalf("comando whisper:\n esperaba %v\n fue      %v", wantWhisper, fake.calls[1])
	}
}

func TestVoiceCaptureErrorSiRecFalla(t *testing.T) {
	fake := &fakeRunner{err: fmt.Errorf("device busy")}
	v := VoiceInput{Runner: fake, WavPath: "/tmp/x.wav"}
	if _, err := v.capture(); err == nil {
		t.Fatal("esperaba error si rec falla")
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestVoiceCapture` → FALLA (aún usa `arecord`, campos viejos).

- [ ] **Step 3: Modificar `input.go`**

3a. En el struct `VoiceInput`, reemplazá el campo `RecSeconds int` por:
```go
	MaxSeconds int    // tope duro de grabación (seg); el silence corta antes en uso normal
	SilencePct string // umbral de silencio (ej "3%"); vacío → default
	TrailSec   string // silencio de cola antes de cortar (ej "1.5"); vacío → default
```

3b. En `NewVoiceInput`, cambiá el parámetro `recSeconds` por `maxSeconds` y el campo:
```go
func NewVoiceInput(r Runner, whisperBin, whisperModel string, maxSeconds int) *VoiceInput {
	return &VoiceInput{
		Runner: r, WhisperBin: whisperBin, WhisperModel: whisperModel,
		MaxSeconds: maxSeconds, WavPath: "/tmp/astro-in.wav",
		trigger: bufio.NewScanner(os.Stdin),
	}
}
```

3c. Reemplazá el cuerpo de `capture()` (desde `secs := v.RecSeconds` hasta el `Run("arecord", …)` inclusive) por:
```go
	maxSecs := v.MaxSeconds
	if maxSecs <= 0 {
		maxSecs = 30
	}
	thresh := v.SilencePct
	if thresh == "" {
		thresh = "3%"
	}
	trail := v.TrailSec
	if trail == "" {
		trail = "1.5"
	}
	fmt.Println("🎙️  hablá… (corta sola al callarte)")
	// rec (sox) graba hasta el silencio de cola; 'timeout' es el tope duro si el umbral nunca
	// detecta silencio (ruido constante) → no graba infinito. rec sale 0 al cortar por silencio.
	if _, err := v.Runner.Run("timeout", strconv.Itoa(maxSecs),
		"rec", "-q", "-c", "1", "-r", "16000", v.WavPath,
		"silence", "1", "0.1", thresh, "1", trail, thresh); err != nil {
		return "", fmt.Errorf("no pude grabar (¿sox/rec + timeout/coreutils + PipeWire?): %w", err)
	}
```
(El resto de `capture()` —whisper, ReadFile, cleanTranscript, `entendí: %q`— queda **igual**.)

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 5: Commit**

```bash
git add input.go input_test.go
git commit -m "feat: grabación por silencio (sox rec + silence, tope timeout) en vez de arecord fijo"
```

---

### Task 2: `chatFunc` con historial + `LLMConfig` (refactor, sin cambio de comportamiento)

**Files:** Modify `llm.go`, `llm_test.go`, `main.go`
**Interfaces:**
- Produces: `type Exchange struct{ User, Assistant string }`; `type LLMConfig struct{…}`;
  `NewLLMInterpreter(cfg LLMConfig) *LLMInterpreter`; `chatFunc = func(system string, history []Exchange, user string) (string, error)`.

> Refactor puro: se agrega el historial al seam pero `Interpret` lo pasa **vacío** todavía → comportamiento
> idéntico. La verificación es que **toda la suite previa sigue verde** con la firma nueva.

- [ ] **Step 1: Cambiar el seam en `llm.go`**

1a. `chatFunc` y el tipo `Exchange`:
```go
// Exchange es un turno de la charla (lo que dijo el usuario y lo que respondió Astro).
type Exchange struct{ User, Assistant string }

// chatFunc habla con el LLM: system + historial de turnos previos + la frase actual → JSON crudo.
type chatFunc func(system string, history []Exchange, user string) (string, error)
```

1b. Struct + config + constructor (reemplazan al struct y `NewLLMInterpreter` de Fase E):
```go
type LLMInterpreter struct {
	chat         chatFunc
	actions      map[string]*Action
	fallback     Interpreter
	mem          MemoryStore
	topK         int
	now          func() time.Time
	history      []Exchange
	historyTurns int
	idleWindow   time.Duration
	lastTurn     time.Time
}

// LLMConfig junta la config del intérprete (creció más allá de lo que conviene posicional).
type LLMConfig struct {
	Chat         chatFunc
	Actions      map[string]*Action
	Fallback     Interpreter
	Mem          MemoryStore     // opcional (nil = sin memoria persistente)
	TopK         int             // <=0 → 5
	Now          func() time.Time // nil → time.Now
	HistoryTurns int             // <=0 → 6
	IdleWindow   time.Duration   // <=0 → 5 min
}

func NewLLMInterpreter(cfg LLMConfig) *LLMInterpreter {
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HistoryTurns <= 0 {
		cfg.HistoryTurns = 6
	}
	if cfg.IdleWindow <= 0 {
		cfg.IdleWindow = 5 * time.Minute
	}
	return &LLMInterpreter{
		chat: cfg.Chat, actions: cfg.Actions, fallback: cfg.Fallback, mem: cfg.Mem,
		topK: cfg.TopK, now: cfg.Now, historyTurns: cfg.HistoryTurns, idleWindow: cfg.IdleWindow,
	}
}
```

1c. En `Interpret`, la llamada al chat pasa el historial (vacío por ahora):
```go
	reply, err := li.chat(li.systemPrompt(facts), li.history, text)
```
(el resto de `Interpret` queda igual en esta tarea).

1d. `httpChat` arma el array de mensajes con el historial:
```go
func httpChat(baseURL, apiKey, model string) chatFunc {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(system string, history []Exchange, user string) (string, error) {
		messages := []map[string]string{{"role": "system", "content": system}}
		for _, ex := range history {
			messages = append(messages, map[string]string{"role": "user", "content": ex.User})
			messages = append(messages, map[string]string{"role": "assistant", "content": ex.Assistant})
		}
		messages = append(messages, map[string]string{"role": "user", "content": user})
		body, _ := json.Marshal(map[string]any{
			"model":           model,
			"temperature":     0,
			"response_format": map[string]string{"type": "json_object"},
			"messages":        messages,
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

- [ ] **Step 2: Actualizar callers para que compile (`llm_test.go` + `main.go`)**

2a. `llm_test.go` — `fakeChat` con la firma nueva y los helpers construyen `LLMConfig`:
```go
func fakeChat(reply string, err error) chatFunc {
	return func(system string, history []Exchange, user string) (string, error) { return reply, err }
}

func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts)})
}

func newLLMWithMem(chat chatFunc, mem MemoryStore) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts), Mem: mem})
}
```

2b. En `llm_test.go`, actualizá el closure inline de `TestLLMInyectaHechosAlPrompt` a la firma nueva:
```go
	chat := func(system string, history []Exchange, user string) (string, error) {
		seenSystem = system
		return `{"action":"none","say":"ok"}`, nil
	}
```

2c. `main.go` — construir con `LLMConfig` (reemplaza la llamada posicional de Fase E):
```go
		interpreter = NewLLMInterpreter(LLMConfig{
			Chat: httpChat(llmURL, llmKey, envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			Actions: acts, Fallback: rules, Mem: buildMemory(embed), TopK: topK, Now: time.Now,
		})
```
(dejá `topK` como se lee hoy; `HistoryTurns`/`IdleWindow` se agregan por env en Task 4).

- [ ] **Step 3: Correr — pasa (comportamiento idéntico)**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS
(todos los tests A/B/C/D/E verdes sin cambios: el historial va vacío, mismo resultado que antes).

- [ ] **Step 4: Commit**

```bash
git add llm.go llm_test.go main.go
git commit -m "refactor: chatFunc con historial + LLMConfig (sin cambio de comportamiento; historial vacío)"
```

---

### Task 3: Contexto multi-turno (threading + registro + reset)

**Files:** Modify `llm.go`, `llm_test.go`
**Interfaces:**
- Consumes: `Exchange`, `LLMConfig`, campos `history/now/historyTurns/idleWindow/lastTurn` (Task 2).
- Produces: `LLMInterpreter.remember(user, assistant string)`; `Interpret` con reset por inactividad + registro de turno.

- [ ] **Step 1: Tests que fallan (agregar a `llm_test.go`)**

```go
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newLLMClock(chat chatFunc, clock *fakeClock) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		Now: clock.now, IdleWindow: 5 * time.Minute})
}

func TestLLMPasaHistorialAlSegundoTurno(t *testing.T) {
	var seen []Exchange
	chat := func(system string, history []Exchange, user string) (string, error) {
		seen = history
		return `{"action":"none","say":"ok"}`, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(chat, clock)
	li.Interpret("subí el volumen") // turno 1: historial vacío
	clock.t = clock.t.Add(10 * time.Second)
	li.Interpret("un poco más") // turno 2: debe ver el turno 1
	if len(seen) != 1 || seen[0].User != "subí el volumen" || seen[0].Assistant != "ok" {
		t.Fatalf("el 2do turno debía ver el 1ro; fue %v", seen)
	}
}

func TestLLMResetPorInactividad(t *testing.T) {
	var seen []Exchange
	chat := func(system string, history []Exchange, user string) (string, error) {
		seen = history
		return `{"action":"none","say":"ok"}`, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(chat, clock)
	li.Interpret("primero")
	clock.t = clock.t.Add(6 * time.Minute) // pasó el umbral de 5 min
	li.Interpret("segundo")                // → reset, historial vacío
	if len(seen) != 0 {
		t.Fatalf("tras >5min el historial se resetea; fue %v", seen)
	}
}

func TestLLMHistorialRecortaAVentana(t *testing.T) {
	chat := func(system string, history []Exchange, user string) (string, error) {
		return `{"action":"none","say":"ok"}`, nil
	}
	acts := buildActions(time.Now)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		Now: clock.now, HistoryTurns: 2})
	for i := 0; i < 4; i++ {
		li.Interpret("frase")
	}
	if len(li.history) != 2 {
		t.Fatalf("la ventana debía recortar a 2 turnos, fue %d", len(li.history))
	}
}

func TestLLMNoRegistraTurnoRechazado(t *testing.T) {
	// open con arg peligroso → cae al fallback → NO debe quedar en el historial (spec §4).
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(fakeChat(`{"action":"open","arg":"firefox; rm -rf ~","say":"abriendo"}`, nil), clock)
	li.Interpret("abrí firefox; rm -rf ~")
	if len(li.history) != 0 {
		t.Fatalf("un turno rechazado por seguridad no debe registrarse; historial=%v", li.history)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestLLMPasaHistorial|TestLLMReset|TestLLMHistorial'` → FALLA
(historial nunca se llena; `remember` no existe).

- [ ] **Step 3: Implementar en `llm.go`**

3a. Al inicio de `Interpret`, antes de todo, el reset por inactividad + marca de tiempo:
```go
func (li *LLMInterpreter) Interpret(text string) (*Action, error) {
	now := li.now()
	// Reset por inactividad: si pasó demasiado desde la última frase, es charla nueva.
	if !li.lastTurn.IsZero() && now.Sub(li.lastTurn) > li.idleWindow {
		li.history = nil
	}
	li.lastTurn = now
```
(el resto sigue: recall de memoria, chat, parseo…).

3b. Registrá el turno **solo en los returns genuinos** (no en los dos fallbacks post-parseo, que van a
reglas — así no se contamina el historial con turnos rechazados; spec §4). Reemplazá el bloque de ruteo
(desde `if (choice.Action == "" || choice.Action == "none") …` hasta el `return li.fallback.Interpret(text)`
final) por:
```go
	if (choice.Action == "" || choice.Action == "none") && choice.Say != "" {
		li.remember(text, choice.Say)
		return sayAction(choice.Say), nil
	}
	if choice.Action == "recordar" && li.mem != nil && choice.Arg != "" {
		li.remember(text, choice.Say)
		return wrap(memoryWriteAction(li.mem, choice.Arg), choice.Say), nil
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // rechazo de seguridad → NO se registra
		}
		li.remember(text, choice.Say)
		return wrap(openAppAction(choice.Arg), choice.Say), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		li.remember(text, choice.Say)
		return wrap(a, choice.Say), nil
	}
	return li.fallback.Interpret(text) // acción desconocida → NO se registra
```

3c. Agregá el helper al final de `llm.go`:
```go
// remember agrega el turno al historial efímero y lo recorta a la ventana de N.
func (li *LLMInterpreter) remember(user, assistant string) {
	li.history = append(li.history, Exchange{User: user, Assistant: assistant})
	if len(li.history) > li.historyTurns {
		li.history = li.history[len(li.history)-li.historyTurns:]
	}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS (nuevos + A/B/C/D/E intactos).

- [ ] **Step 5: Commit**

```bash
git add llm.go llm_test.go
git commit -m "feat: contexto multi-turno — historial rodante + reset por inactividad"
```

---

### Task 4: Wiring de envs en `main.go` + E2E

**Files:** Modify `main.go`

- [ ] **Step 1: Envs de grabación (VAD) + de historial**

1a. Donde se arma el `VoiceInput` (rama `ASTRO_INPUT != stdin`), seteá los campos de silencio desde env:
```go
		secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS")) // ahora = tope duro
		vi := NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cli"),
			os.Getenv("ASTRO_WHISPER_MODEL"), secs)
		vi.SilencePct = os.Getenv("ASTRO_REC_SILENCE_PCT") // vacío → default en capture()
		vi.TrailSec = os.Getenv("ASTRO_REC_TRAIL_SEC")
		input = vi
```

1b. En el `LLMConfig` (Task 2), agregá historial por env:
```go
		idleMin, _ := strconv.Atoi(os.Getenv("ASTRO_HISTORY_IDLE_MIN"))
		historyTurns, _ := strconv.Atoi(os.Getenv("ASTRO_HISTORY_TURNS"))
		interpreter = NewLLMInterpreter(LLMConfig{
			Chat: httpChat(llmURL, llmKey, envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			Actions: acts, Fallback: rules, Mem: buildMemory(embed), TopK: topK, Now: time.Now,
			HistoryTurns: historyTurns, IdleWindow: time.Duration(idleMin) * time.Minute,
		})
```
(env ausente → `0` → el constructor aplica los defaults 6 turnos / 5 min).

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 3: E2E (manual — necesita la key + micrófono; lo corre el usuario)**

```bash
cd ~/projects/astro && git switch fase-f
astro   # (o los exports de siempre + run)
```
Probar: (a) una frase **corta** y una **larga** → graba completo y corta sola al callarte (no a los 4s);
(b) *"subí el volumen"* → *"un poco más"* → lo toma como seguimiento; (c) esperar >5 min y hablar → charla nueva.

**Gatillo — SOLO si en (a) Astro se APAGA con ruido de fondo** (el tope `timeout` mata `rec` con exit 124 →
`capture` devuelve error → `main` hace `break` → sale). Recién ahí, traducir el 124 a turno vacío para que el
loop siga ("No te escuché") en vez de apagarse. En `capture`, en el error de `rec`:
```go
var ee *exec.ExitError
if errors.As(err, &ee) && ee.ExitCode() == 124 {
	return "", nil // tope alcanzado → main dice "No te escuché" y sigue, no apaga
}
```
(agrega imports `errors` + `os/exec` en `input.go`; y un test del chequeo con un `*exec.ExitError` real de
`exec.Command("sh", "-c", "exit 124").Run()`). Si (a) NO apaga Astro, no se toca — no metemos código sin evidencia.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wiring de envs de grabación (VAD) e historial multi-turno en main"
```

---

## Definition of Done (Fase F)

1. `go build/vet/test ./...` verdes (VAD rec, multi-turno, reset, recorte de ventana; + A/B/C/D/E intactos).
2. E2E: graba hasta el silencio (corta sola, corto o largo); seguimientos entendidos; reset tras 5 min.
3. Interfaz `Interpret` / voz / cara / seguridad / memoria persistente intactas.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Cobertura del spec:** Parte A → Task 1; seam+config → Task 2; comportamiento multi-turno → Task 3; envs+E2E → Task 4.
- **Compila en cada tarea:** Task 1 no toca la firma de `NewVoiceInput` (solo renombra campo interno). Task 2 actualiza TODOS los call sites (`fakeChat`, helpers, closure inline, `main`) → verde como refactor no-op. Task 3 enciende el comportamiento. Task 4 solo agrega envs.
- **Tope duro (anti-cuelgue):** `timeout <max>` antepuesto; en uso normal `rec` corta por silencio (exit 0). Si nunca hay silencio (ruido sostenido > umbral), `timeout` mata `rec` (exit 124) → `capture` devuelve error → `main` hace `break` y **Astro sale** (mismo path que un fallo de `arecord` hoy; NO reintenta). Aceptado como borde; si el E2E lo muestra, se aplica el fix gatillado (Task 4 Step 3).
- **Reset determinista:** `fakeClock` inyectado → los tests de inactividad no esperan tiempo real.
- **No verde-falso:** los tests capturan el historial REAL pasado al chat y afirman su contenido; el reset y el recorte se verifican por longitud/contenido, no por "no crashea".
- **Config struct:** cruza el umbral de params que el mentor marcó (Fase E: "6º parámetro → LLMConfig"); Fase F suma `now/historyTurns/idleWindow`, así que el salto está justificado, no especulativo.
- **Sin tocar Fase E/seguridad:** el recall de memoria y el guard de `open`/`recordar` quedan igual; el historial es un canal aparte (mensajes), los hechos siguen en el `systemPrompt`.
