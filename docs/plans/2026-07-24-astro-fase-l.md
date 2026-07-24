# Astro · Fase L — Ejecutor genérico con confirmación · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que Astro pueda proponer y correr un comando de shell arbitrario para lo que ningún tool cubre, pero solo tras una confirmación de dos turnos y con un backstop que rechaza patrones catastróficos.

**Architecture:** Un action `ejecutar` (el LLM compone el comando en `arg`). El `LLMInterpreter` gana estado `pendingCmd`: al recibir `ejecutar` NO corre nada — guarda el comando y pide confirmar. El turno siguiente, al TOPE de `Interpret` (antes del LLM), un sí/no resuelve el pendiente: afirmativo → `sh -c <cmd>`; negativo/ambiguo → no corre (default seguro). Un gate `ASTRO_EXEC` (off por default) habilita todo; con él apagado `ejecutar` cae al fallback.

**Tech Stack:** Go (solo stdlib). Reusa `normalize` de `interpreter.go`, `Runner`, `sayAction`, el patrón de ruteo de `Interpret`, y los fakes de test (`fakeChat`, `fakeRunner`, `fakeClock`).

## Global Constraints

- Identificadores en **inglés**; comentarios y mensajes hablados en **español**.
- **Solo stdlib** de Go (sin dependencias externas).
- **Nada destructivo sin confirmación**: `ejecutar` jamás corre en el mismo turno que se propone; el default ante duda es NO correr.
- La firma de `Interpret(text string) (*Action, error)` **no cambia**.
- `ejecutar` es **stateless respecto al historial** (no llama a `remember`, como `mirar`).
- La confirmación se resuelve **antes** de llamar al LLM.
- Con `ASTRO_EXEC` apagado, `ejecutar` se comporta como acción inexistente (fallback).

---

### Task 1: Estado, config y helpers de confirmación/backstop

**Files:**
- Modify: `llm.go` (campos `execEnabled`/`pendingCmd` en `LLMInterpreter`; `ExecEnabled` en `LLMConfig`; wiring en `NewLLMInterpreter`; helpers nuevos; import `unicode`)
- Test: `llm_test.go` (tests de los helpers + helper de construcción `newLLMExec`)

**Interfaces:**
- Consumes: `normalize(s string) string` (de `interpreter.go`); `LLMConfig`, `LLMInterpreter`, `NewLLMInterpreter` (de `llm.go`).
- Produces: `func confirmationVerdict(text string) string` (devuelve `"yes"`/`"no"`/`"other"`), `func isCatastrophic(cmd string) bool`; campo `LLMConfig.ExecEnabled bool`; campos `execEnabled bool`, `pendingCmd string` en `LLMInterpreter`; helper de test `newLLMExec(chat chatFunc) *LLMInterpreter`.

**Diseño de la confirmación (regla de seguridad, corregida tras auditoría del mentor):** una respuesta cuenta como sí/no SOLO si la frase **entera** es una confirmación — es decir, cada palabra es afirmativa, negativa o una muletilla. Si aparece **cualquier palabra ajena** (ej. `"ok mostrame la hora"`), es un pedido nuevo, no una confirmación → devuelve `"other"` → el flujo la reprocesa y NO corre el pendiente. Esto cierra el hueco de que una muletilla suelta (`ok`/`dale`) al inicio de un pedido nuevo dispare el comando. El negativo gana sobre el afirmativo (ante `"no, dale"` → `"no"`).

- [ ] **Step 1: Escribir los tests de los helpers**

En `llm_test.go`, agregá al final:

```go
func TestConfirmationVerdict(t *testing.T) {
	yes := []string{"sí", "si", "Dale", "dale, hacelo", "confirmo", "ok", "correlo", "sí, dale", "obvio"}
	no := []string{"no", "No", "cancelá", "cancelalo", "dejá", "olvidalo", "no, dale", "mejor no"}
	// "other" = pedido nuevo (aunque arranque con muletilla) o frase que no es sí/no puro → NO corre.
	other := []string{"ok mostrame la hora", "dale contame un chiste", "qué hora es", "", "hola cómo estás"}
	for _, s := range yes {
		if got := confirmationVerdict(s); got != "yes" {
			t.Errorf("confirmationVerdict(%q) = %q, quiero yes", s, got)
		}
	}
	for _, s := range no {
		if got := confirmationVerdict(s); got != "no" {
			t.Errorf("confirmationVerdict(%q) = %q, quiero no", s, got)
		}
	}
	for _, s := range other {
		if got := confirmationVerdict(s); got != "other" {
			t.Errorf("confirmationVerdict(%q) = %q, quiero other", s, got)
		}
	}
}

func TestIsCatastrophic(t *testing.T) {
	bad := []string{
		"rm -rf /", "rm -rf ~", "rm  -rf   /home", "RM -RF /", "sudo rm algo",
		"dd if=/dev/zero of=/dev/sda", "mkfs.ext4 /dev/sda1", ":(){ :|:& };:",
		"echo x > /dev/sda", "cat x > /dev/nvme0n1", "shutdown now", "reboot", "chmod -R / 777",
	}
	ok := []string{
		"echo hola", "ls -la ~/Descargas | wc -l", "git add .",
		"rm archivo.txt", "rm -rf node_modules", "grep -r foo .", "cat log 2> /dev/null",
	}
	for _, c := range bad {
		if !isCatastrophic(c) {
			t.Errorf("isCatastrophic(%q) = false, quiero true", c)
		}
	}
	for _, c := range ok {
		if isCatastrophic(c) {
			t.Errorf("isCatastrophic(%q) = true, quiero false", c)
		}
	}
}
```

Notas: `"rm -rf node_modules"` es relativo (no arranca en `/` ni `~`) → NO catastrófico; se confirma como cualquier borrado. `"cat log 2> /dev/null"` es benigno (redirect a `/dev/null`, no a un block device) → NO catastrófico; por eso el backstop matchea `> /dev/sd`/`> /dev/nvme`, **no** `/dev/` a secas. Este host es NVMe, de ahí el patrón `nvme`.

- [ ] **Step 2: Correr los tests para verlos fallar**

Run: `cd ~/projects/astro && go test -run 'TestConfirmationVerdict|TestIsCatastrophic' .`
Expected: FAIL — `undefined: confirmationVerdict` / `undefined: isCatastrophic`.

- [ ] **Step 3: Implementar los helpers**

En `llm.go`, agregá `"unicode"` al bloque de imports (en orden alfabético, después de `"time"`). Luego agregá al final del archivo:

```go
// affirmatives/negatives/fillers: vocabulario de una confirmación sí/no (ya normalizado: "sí"→"si",
// "cancelá"→"cancela"). fillers = muletillas que no cambian el sentido ("por favor", "che", "mejor no").
var affirmatives = map[string]bool{
	"si": true, "dale": true, "confirmo": true, "confirmar": true, "confirma": true,
	"hacelo": true, "hazlo": true, "correlo": true, "corre": true, "ok": true, "okay": true,
	"obvio": true, "claro": true, "correcto": true, "exacto": true,
}
var negatives = map[string]bool{
	"no": true, "cancela": true, "cancelalo": true, "cancelar": true,
	"deja": true, "dejalo": true, "olvidalo": true, "nada": true,
}
var fillers = map[string]bool{
	"che": true, "bueno": true, "ya": true, "por": true, "favor": true,
	"eh": true, "que": true, "eso": true, "lo": true, "mejor": true,
}

// confirmationVerdict clasifica una respuesta de confirmación en "yes"/"no"/"other". Solo es sí/no si
// la frase ENTERA es una confirmación (cada token es afirmativo/negativo/muletilla); cualquier palabra
// ajena → "other" (es un pedido nuevo, NO una confirmación) → default seguro: no corre. El negativo
// gana sobre el afirmativo (ante "no, dale" → "no").
func confirmationVerdict(text string) string {
	toks := strings.FieldsFunc(normalize(text), func(r rune) bool { return !unicode.IsLetter(r) })
	neg, aff := false, false
	for _, t := range toks {
		switch {
		case negatives[t]:
			neg = true
		case affirmatives[t]:
			aff = true
		case fillers[t]:
			// muletilla: se ignora
		default:
			return "other" // palabra ajena → pedido nuevo, no una confirmación pura
		}
	}
	switch {
	case neg:
		return "no"
	case aff:
		return "yes"
	default:
		return "other" // vacío o solo muletillas
	}
}

// isCatastrophic es un BACKSTOP: rechaza patrones que borran todo o rompen el sistema, aunque el
// usuario confirme. NO es airtight (una denylist es gato-y-ratón); la guarda real es la confirmación.
func isCatastrophic(cmd string) bool {
	c := strings.ToLower(strings.Join(strings.Fields(cmd), " ")) // colapsa espacios, a minúsculas
	bad := []string{
		"rm -rf /", "rm -fr /", "rm -rf ~", "rm -fr ~", "rm -rf /home",
		"dd if=", "dd of=", "mkfs", "sudo ", ":(){",
		"> /dev/sd", "> /dev/nvme", "of=/dev/sd", "of=/dev/nvme",
		"shutdown", "reboot", "chmod -r /", "chown -r /",
	}
	for _, b := range bad {
		if strings.Contains(c, b) {
			return true
		}
	}
	return false
}
```

Después agregá el campo a `LLMConfig` (junto a los otros, tras `Monitors`):

```go
	Monitors     string           // lista de monitores para el prompt (vacía = sin resolución de nombres)
	ExecEnabled  bool             // habilita el action `ejecutar` (shell con confirmación); default off
```

Y los campos a `LLMInterpreter` (tras `monitors`):

```go
	vision       visionFunc
	monitors     string
	execEnabled  bool
	pendingCmd   string // comando `ejecutar` propuesto, esperando confirmación (vacío = ninguno)
```

Y el wiring en `NewLLMInterpreter`: **REEMPLAZÁ** la línea que hoy dice (es la última del `return &LLMInterpreter{...}`, `llm.go:70`):

```go
		vision: cfg.Vision, monitors: cfg.Monitors,
```

por esta versión extendida (NO la dupliques — agregá solo `execEnabled`):

```go
		vision: cfg.Vision, monitors: cfg.Monitors, execEnabled: cfg.ExecEnabled,
```

Finalmente, el helper de test en `llm_test.go` (cerca de `newLLM`):

```go
func newLLMExec(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		ArgActions: buildArgActions(), ExecEnabled: true})
}
```

- [ ] **Step 4: Correr los tests para verlos pasar**

Run: `cd ~/projects/astro && go test -run 'TestConfirmationVerdict|TestIsCatastrophic' .`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
cd ~/projects/astro
git add llm.go llm_test.go
git commit -m "feat(astro): estado ejecutar + confirmationVerdict (frase-entera) y backstop catastrófico (Fase L Task 1)"
```

---

### Task 2: Ruteo de `ejecutar` (propuesta, rechazo catastrófico, gate off) + `execAction`

**Files:**
- Modify: `llm.go` (rama `ejecutar` en `Interpret`; función `execAction`)
- Test: `llm_test.go`

**Interfaces:**
- Consumes: `isCatastrophic`, `sayAction`, `LLMInterpreter.execEnabled`, `LLMInterpreter.pendingCmd`, `Runner` (de Task 1 + existentes).
- Produces: `func execAction(cmd string) *Action`; comportamiento de ruteo para `choice.Action == "ejecutar"`.

- [ ] **Step 1: Escribir los tests del ruteo (propuesta / catastrófico / gate off)**

En `llm_test.go`:

```go
func TestEjecutarProponeNoCorre(t *testing.T) {
	li := newLLMExec(fakeChat(`{"action":"ejecutar","arg":"ls ~/Descargas | wc -l","say":"ok"}`, nil))
	a, err := li.Interpret("cuántos archivos hay en Descargas")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if li.pendingCmd != "ls ~/Descargas | wc -l" {
		t.Fatalf("pendingCmd = %q, quiero el comando propuesto", li.pendingCmd)
	}
	fake := &fakeRunner{}
	say, err := a.Run(fake)
	if err != nil {
		t.Fatalf("error corriendo la propuesta: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("la propuesta NO debe correr nada, corrió: %v", fake.calls)
	}
	if !strings.Contains(say, "ls ~/Descargas | wc -l") || !strings.Contains(say, "confirmo") {
		t.Errorf("say = %q, quiero que muestre el comando y pida confirmar", say)
	}
}

func TestEjecutarCatastroficoRechaza(t *testing.T) {
	li := newLLMExec(fakeChat(`{"action":"ejecutar","arg":"rm -rf /","say":"ok"}`, nil))
	a, err := li.Interpret("borrá todo")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if li.pendingCmd != "" {
		t.Fatalf("un comando catastrófico NO debe quedar pendiente, pendingCmd = %q", li.pendingCmd)
	}
	say, _ := a.Run(&fakeRunner{})
	if !strings.Contains(strings.ToLower(say), "peligroso") {
		t.Errorf("say = %q, quiero un rechazo", say)
	}
}

func TestEjecutarGateOffCaeAFallback(t *testing.T) {
	// Sin ExecEnabled: ejecutar no existe → fallback (reglas). No debe armar pendiente.
	acts := buildActions(time.Now)
	li := NewLLMInterpreter(LLMConfig{
		Chat:     fakeChat(`{"action":"ejecutar","arg":"echo hola","say":"ok"}`, nil),
		Actions:  acts, Fallback: NewRuleInterpreter(acts),
	})
	_, _ = li.Interpret("hacé cualquier cosa rara")
	if li.pendingCmd != "" {
		t.Fatalf("con el gate apagado NO debe armar pendiente, pendingCmd = %q", li.pendingCmd)
	}
}
```

- [ ] **Step 2: Correr los tests para verlos fallar**

Run: `cd ~/projects/astro && go test -run 'TestEjecutar' .`
Expected: FAIL — `TestEjecutarProponeNoCorre` falla porque `pendingCmd` queda vacío (todavía no hay rama `ejecutar`; cae al fallback).

- [ ] **Step 3: Implementar `execAction` y la rama de ruteo**

En `llm.go`, agregá `execAction` y el helper `clip` (cerca de `lookAction`):

```go
// execAction corre un comando de shell YA CONFIRMADO por el usuario. Es el único punto del sistema
// con shell arbitrario, y solo se llega acá tras la confirmación de dos turnos. Va envuelto en
// `timeout 60`: como el daemon es un solo goroutine, un comando colgado (sleep, algo que lee stdin)
// lo congelaría entero (mismo criterio que el curl de `clima`). En fallo HABLA el error con err=nil,
// porque main.go a los errores de Run los imprime en pantalla (Println), no los dice por voz.
func execAction(cmd string) *Action {
	return &Action{Name: "ejecutar", Face: Neutral,
		Run: func(r Runner) (string, error) {
			out, err := r.Run("timeout", "60", "sh", "-c", cmd)
			out = strings.TrimSpace(out)
			if err != nil {
				if out == "" {
					return "El comando falló.", nil
				}
				return "Falló: " + clip(out, 140), nil
			}
			switch {
			case out == "":
				return "Listo.", nil
			case len(out) > 200:
				return "Listo, pero la salida es muy larga para leértela.", nil
			default:
				return out, nil
			}
		}}
}

// clip recorta a lo sumo n runas (sin partir un carácter UTF-8), para no leer paredes de texto.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
```

En `Interpret`, agregá la rama **después** del bloque `if choice.Action == "open"` y **antes** de `if t, ok := li.argActions[...]`:

```go
	if choice.Action == "ejecutar" {
		if !li.execEnabled || choice.Arg == "" {
			return li.fallback.Interpret(text) // ejecutor apagado o sin comando → como si no existiera
		}
		if isCatastrophic(choice.Arg) {
			return sayAction("Eso no lo hago, es demasiado peligroso."), nil // rechazo directo, sin pendiente
		}
		// Propuesta: guardamos el comando y pedimos confirmar. NO se corre ni se registra en historial.
		li.pendingCmd = choice.Arg
		return sayAction("Voy a correr: " + choice.Arg + ". ¿Lo confirmo?"), nil
	}
```

- [ ] **Step 4: Correr los tests para verlos pasar**

Run: `cd ~/projects/astro && go test -run 'TestEjecutar' .`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
cd ~/projects/astro
git add llm.go llm_test.go
git commit -m "feat(astro): ruteo de ejecutar (propuesta/rechazo/gate) + execAction (Fase L Task 2)"
```

---

### Task 3: Resolución de la confirmación al tope de `Interpret` + expiración por inactividad

**Files:**
- Modify: `llm.go` (bloque de confirmación al inicio de `Interpret`; limpiar `pendingCmd` en el reset por inactividad)
- Test: `llm_test.go`

**Interfaces:**
- Consumes: `confirmationVerdict`, `execAction`, `sayAction`, `pendingCmd`, `newLLMExec` (Task 1/2); `fakeClock`, `newLLMClock` (existentes).
- Produces: comportamiento de confirmación (turno 2); helper de test `newLLMExecClock(chat, clock)`.

- [ ] **Step 1: Escribir los tests del flujo de confirmación**

En `llm_test.go`, primero el helper con reloj (cerca de `newLLMClock`):

```go
func newLLMExecClock(chat chatFunc, clock *fakeClock) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		ArgActions: buildArgActions(), ExecEnabled: true, Now: clock.now, IdleWindow: 5 * time.Minute})
}
```

Luego los tests:

```go
func TestConfirmarCorreElComando(t *testing.T) {
	// El chat debe fallar si se lo llama en el turno de confirmación (la confirmación va antes del LLM).
	chat := func(system string, history []Exchange, user string) (string, error) {
		if user == "sí" {
			return "", fmt.Errorf("el LLM NO debe llamarse en la confirmación")
		}
		return `{"action":"ejecutar","arg":"echo hola","say":"ok"}`, nil
	}
	li := newLLMExec(chat)
	if _, err := li.Interpret("decí hola por consola"); err != nil { // propuesta
		t.Fatalf("propuesta falló: %v", err)
	}
	a, err := li.Interpret("sí") // confirmación
	if err != nil {
		t.Fatalf("confirmación falló: %v", err)
	}
	if li.pendingCmd != "" {
		t.Errorf("tras confirmar, pendingCmd debe quedar vacío, es %q", li.pendingCmd)
	}
	fake := &fakeRunner{output: "hola"}
	say, err := a.Run(fake)
	if err != nil {
		t.Fatalf("correr falló: %v", err)
	}
	want := []string{"timeout", "60", "sh", "-c", "echo hola"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Errorf("comando corrido = %v, quiero %v", got, want)
	}
	if say != "hola" {
		t.Errorf("say = %q, quiero la salida 'hola'", say)
	}
}

func TestNegativoCancela(t *testing.T) {
	li := newLLMExec(fakeChat(`{"action":"ejecutar","arg":"echo hola","say":"ok"}`, nil))
	li.Interpret("decí hola") // propuesta
	a, err := li.Interpret("no")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if li.pendingCmd != "" {
		t.Errorf("tras cancelar, pendingCmd debe quedar vacío, es %q", li.pendingCmd)
	}
	fake := &fakeRunner{}
	say, _ := a.Run(fake)
	if len(fake.calls) != 0 {
		t.Fatalf("cancelar NO debe correr nada, corrió: %v", fake.calls)
	}
	if !strings.Contains(strings.ToLower(say), "cancel") {
		t.Errorf("say = %q, quiero un aviso de cancelado", say)
	}
}

func TestAmbiguoCancelaYReprocesa(t *testing.T) {
	// CLAVE de seguridad: un pedido nuevo que arranca con muletilla afirmativa ("ok, contame…") NO
	// debe contar como confirmación. Cancela el pendiente y se reprocesa como pedido nuevo (no corre).
	var llamado bool
	chat := func(system string, history []Exchange, user string) (string, error) {
		if user == "ok, contame un chiste" {
			llamado = true
			return `{"action":"none","say":"un chiste corto"}`, nil
		}
		return `{"action":"ejecutar","arg":"echo hola","say":"ok"}`, nil
	}
	li := newLLMExec(chat)
	li.Interpret("decí hola") // propuesta
	a, err := li.Interpret("ok, contame un chiste")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if li.pendingCmd != "" {
		t.Errorf("un pedido nuevo debe cancelar el pendiente, pendingCmd = %q", li.pendingCmd)
	}
	if !llamado {
		t.Error("el pedido nuevo debe reprocesarse por el LLM")
	}
	fake := &fakeRunner{}
	say, _ := a.Run(fake)
	if len(fake.calls) != 0 {
		t.Fatalf("NO debe correr el comando pendiente, corrió: %v", fake.calls)
	}
	if say != "un chiste corto" {
		t.Errorf("say = %q, quiero la respuesta del pedido nuevo", say)
	}
}

func TestComandoFallidoHablaError(t *testing.T) {
	// Un comando confirmado que falla debe HABLAR el error (err=nil): main.go a los errores de Run
	// los imprime en pantalla, no los dice por voz → propagar el error dejaría a Astro mudo.
	li := newLLMExec(fakeChat(`{"action":"ejecutar","arg":"false","say":"ok"}`, nil))
	li.Interpret("hacé fallar algo") // propuesta
	a, _ := li.Interpret("sí")       // confirmación
	fake := &fakeRunner{output: "boom", err: fmt.Errorf("exit status 1")}
	say, err := a.Run(fake)
	if err != nil {
		t.Fatalf("un comando fallido debe hablar el error con err=nil, no propagarlo: %v", err)
	}
	if !strings.Contains(strings.ToLower(say), "fall") {
		t.Errorf("say = %q, quiero que avise que falló", say)
	}
}

func TestPendienteExpiraPorInactividad(t *testing.T) {
	chat := func(system string, history []Exchange, user string) (string, error) {
		if user == "sí" {
			return `{"action":"none","say":"ok"}`, nil // se reprocesa como charla, no como confirmación
		}
		return `{"action":"ejecutar","arg":"echo hola","say":"ok"}`, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMExecClock(chat, clock)
	li.Interpret("decí hola")          // propuesta, pendiente armado
	clock.t = clock.t.Add(6 * time.Minute) // pasa la ventana de inactividad
	a, err := li.Interpret("sí")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if li.pendingCmd != "" {
		t.Errorf("el pendiente debe expirar por inactividad, pendingCmd = %q", li.pendingCmd)
	}
	fake := &fakeRunner{}
	if _, _ = a.Run(fake); len(fake.calls) != 0 {
		t.Fatalf("un 'sí' tardío NO debe correr el comando viejo, corrió: %v", fake.calls)
	}
}
```

Asegurate de que `llm_test.go` importe `"reflect"` (ya se usa en otros tests del archivo; si no está, agregalo).

- [ ] **Step 2: Correr los tests para verlos fallar**

Run: `cd ~/projects/astro && go test -run 'TestConfirmar|TestNegativo|TestAmbiguo|TestComandoFallido|TestPendiente' .`
Expected: FAIL — sin el bloque de confirmación, `Interpret("sí")` llama al LLM (el `chat` de `TestConfirmarCorreElComando` devuelve error a propósito) o reprocesa; `pendingCmd` no se limpia.

- [ ] **Step 3: Implementar el bloque de confirmación y la expiración**

En `llm.go`, dentro de `Interpret`, en el reset por inactividad, sumá la limpieza del pendiente:

```go
	// Reset por inactividad: si pasó demasiado desde la última frase, es charla nueva.
	if !li.lastTurn.IsZero() && now.Sub(li.lastTurn) > li.idleWindow {
		li.history = nil
		li.pendingCmd = "" // un comando propuesto hace rato NO se confirma "a ciegas"
	}
	li.lastTurn = now
```

Justo **después** de `li.lastTurn = now` (y antes del recall de memoria), agregá:

```go
	// Confirmación de un `ejecutar` pendiente: se resuelve acá, antes del LLM (es un sí/no, no un
	// pedido nuevo). "other" (pedido nuevo, aunque arranque con muletilla) descarta el pendiente y
	// cae al flujo normal = default seguro: NO corre.
	if li.pendingCmd != "" {
		cmd := li.pendingCmd
		li.pendingCmd = ""
		switch confirmationVerdict(text) {
		case "no":
			return sayAction("Listo, cancelado."), nil
		case "yes":
			return execAction(cmd), nil
		}
	}
```

- [ ] **Step 4: Correr los tests para verlos pasar**

Run: `cd ~/projects/astro && go test -run 'TestConfirmar|TestNegativo|TestAmbiguo|TestComandoFallido|TestPendiente' .`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
cd ~/projects/astro
git add llm.go llm_test.go
git commit -m "feat(astro): confirmación de dos turnos + expiración del pendiente (Fase L Task 3)"
```

---

### Task 4: `ejecutar` en el `systemPrompt`

**Files:**
- Modify: `llm.go` (`systemPrompt`)
- Test: `llm_test.go`

**Interfaces:**
- Consumes: `LLMInterpreter.systemPrompt`, `LLMInterpreter.execEnabled`, `newLLMExec`, `newLLM`.
- Produces: línea de `ejecutar` en el prompt cuando `execEnabled`.

- [ ] **Step 1: Escribir el test del prompt**

En `llm_test.go`:

```go
func TestSystemPromptListaEjecutarSoloConGate(t *testing.T) {
	on := newLLMExec(fakeChat("", nil)).systemPrompt(nil)
	if !strings.Contains(on, "ejecutar") {
		t.Error("con el gate encendido, el prompt debe listar 'ejecutar'")
	}
	off := newLLM(fakeChat("", nil)).systemPrompt(nil)
	if strings.Contains(off, "ejecutar") {
		t.Error("con el gate apagado, el prompt NO debe listar 'ejecutar'")
	}
}
```

- [ ] **Step 2: Correr el test para verlo fallar**

Run: `cd ~/projects/astro && go test -run 'TestSystemPromptListaEjecutar' .`
Expected: FAIL — el prompt todavía no menciona `ejecutar`.

- [ ] **Step 3: Agregar la línea al `systemPrompt`**

En `llm.go`, en `systemPrompt`, después del bloque de `mirar`/monitores y antes del loop de `argActions`, agregá:

```go
	if li.execEnabled {
		b.WriteString("- ejecutar (arg = un comando de shell): SOLO para lo que ningún otro tool del menú cubre; se te confirmará antes de correr.\n")
	}
```

- [ ] **Step 4: Correr el test para verlo pasar**

Run: `cd ~/projects/astro && go test -run 'TestSystemPromptListaEjecutar' .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd ~/projects/astro
git add llm.go llm_test.go
git commit -m "feat(astro): ejecutar en el systemPrompt bajo el gate (Fase L Task 4)"
```

---

### Task 5: Wiring en `main.go` y `run.sh`

**Files:**
- Modify: `main.go` (leer `ASTRO_EXEC` y pasarlo a `LLMConfig.ExecEnabled`)
- Modify: `run.sh` (exportar `ASTRO_EXEC=1`)

**Interfaces:**
- Consumes: `LLMConfig.ExecEnabled` (Task 1); `os.Getenv` (ya importado en `main.go`).
- Produces: `ejecutar` habilitado en tiempo de ejecución vía `run.sh`.

- [ ] **Step 1: Wirear `ExecEnabled` en `main.go`**

En `main.go`, en el literal `LLMConfig{...}` (líneas ~31-36), agregá `ExecEnabled` a la última línea de campos:

```go
			Vision: vision, Monitors: buildMonitorsPrompt(runner), ExecEnabled: os.Getenv("ASTRO_EXEC") == "1",
```

- [ ] **Step 2: Verificar que compila y toda la suite pasa**

Run: `cd ~/projects/astro && go build ./... && go vet ./... && go test ./...`
Expected: build/vet limpios; `ok` en los tests (incluye A–K + los nuevos de L).

- [ ] **Step 3: Habilitar el gate en `run.sh`**

En `run.sh`, después del bloque de `ASTRO_INPUT` (línea ~33) y antes de `CGO_ENABLED`, agregá:

```bash
# Fase L: habilita el action `ejecutar` (comando de shell genérico). SIEMPRE se confirma antes de
# correr y hay un backstop que rechaza patrones catastróficos; aun así, es shell arbitrario tras tu
# OK — apagalo (borrá esta línea o poné 0) si no lo querés.
export ASTRO_EXEC=1
```

- [ ] **Step 4: Verificar que `run.sh` sigue siendo válido**

Run: `cd ~/projects/astro && bash -n run.sh && echo "sintaxis ok"`
Expected: `sintaxis ok`.

- [ ] **Step 5: Commit**

```bash
cd ~/projects/astro
git add main.go run.sh
git commit -m "feat(astro): wirear ASTRO_EXEC (gate de ejecutar) en main y run.sh (Fase L Task 5)"
```

---

## Definition of Done

1. `go build ./... && go vet ./... && go test ./...` verdes: `confirmationVerdict` (yes/no/other), backstop, propuesta-no-corre, confirma-corre (`timeout 60 sh -c`), fallo-habla-error, negativo/pedido-nuevo no corren, expiración, gate off→fallback, prompt condicional; + A–K intactos.
2. Con `ASTRO_EXEC=1`: un pedido no cubierto → Astro **propone** el comando y pide confirmar; "sí" → lo corre y dice el resultado; "no"/ambiguo → cancela; un comando catastrófico → lo rechaza. (E2E queda pendiente del balance de DeepSeek, como Fase K.)
3. `Interpret` sin cambios de firma; memoria/visión/tools previas intactas; nada destructivo sin confirmación.
4. Convenciones: ids inglés, comentarios/mensajes español, solo stdlib.

## Notas de auto-revisión (self-review)

- **Cobertura spec:** `ejecutar` (Task 2/4) · confirmación 2 turnos (Task 3) · backstop catastrófico (Task 1/2) · gate `ASTRO_EXEC` (Task 1/5) · systemPrompt (Task 4) · errores/bordes (tests de Task 2/3). ✔
- **Consistencia de tipos:** `execAction(cmd string) *Action`, `confirmationVerdict(string) string` (`"yes"/"no"/"other"`), `isCatastrophic(string) bool`, `clip(string,int) string`, `LLMConfig.ExecEnabled bool`, campos `execEnabled`/`pendingCmd` — usados con los mismos nombres en todas las tasks. ✔
- **Decisión asentada (vetable):** el backstop es **conservador** — `rm -rf /…` absoluto (raíz/home) se rechaza aunque apunte a un subdirectorio; borrados de rutas **relativas** sí se permiten (con confirmación). Es un belt honesto, no una sandbox.
