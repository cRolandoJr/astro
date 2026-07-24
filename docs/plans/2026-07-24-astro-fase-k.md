# Astro · Fase K — Plan de Implementación (batería de tools)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** Muchas tools no destructivas (media/audio/pantalla/sistema/info/web). Las sin-arg al registro; las con-arg por un mapa de factories `ArgTool` ruteado por una sola rama en `Interpret`.

**Architecture:** `actions.go` agrega las tools; `ArgTool{Desc, Build}` + `buildArgActions()`; `llm.go` gana `argActions` (campo, ruteo, systemPrompt); `main.go` lo wirea. Todo por `Runner`/argv (sin shell → sin inyección); validación = correctitud del arg. Spec: `docs/specs/2026-07-24-astro-fase-k-design.md`.

**Tech Stack:** Go, stdlib (`net/url`, `strconv`). Comandos externos vía `Runner`: playerctl, wpctl, brightnessctl, hyprctl, hyprlock, khal, upower, curl (wttr.in).

## Global Constraints

- Identificadores **inglés**; comentarios/mensajes **español**. Nada destructivo; **solo stdlib**.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Interfaz `Interpret(text) (*Action, error)` sin cambios; seguridad/memoria/visión previas intactas.
- Cada tarea termina **compilando y con todos los tests verdes** (A–J incluidos).

---

### Task 1: Tools sin arg (registro)

**Files:** Modify `actions.go`, `actions_test.go` (crear si no existe)
**Produces:** tools `anterior`, `que_suena`, `silenciar_micro`, `bateria`, `fecha`, `clima`, `agenda`, `bloquear` en `buildActions`; helper `fechaES`.

- [ ] **Step 1: Tests que fallan (`actions_test.go`)**

```go
package main

import (
	"testing"
	"time"
)

func TestQueSuenaDiceLaCancion(t *testing.T) {
	acts := buildActions(time.Now)
	fake := &fakeRunner{output: "Nate Gentile - Mi PC Linux"}
	reply, err := acts["que_suena"].Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Está sonando: Nate Gentile - Mi PC Linux." {
		t.Fatalf("fue %q", reply)
	}
	if got := fake.lastCall(); got[0] != "playerctl" || got[1] != "metadata" {
		t.Fatalf("esperaba playerctl metadata, fue %v", got)
	}
}

func TestBateriaParseaPorcentaje(t *testing.T) {
	acts := buildActions(time.Now)
	// upower -e → device; upower -i <dev> → info con "percentage:"
	fake := &fakeRunner{output: "/org/freedesktop/UPower/devices/battery_BAT0\n    percentage:          87%\n"}
	reply, err := acts["bateria"].Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Batería al 87%." {
		t.Fatalf("fue %q", reply)
	}
}

func TestFechaEnEspanol(t *testing.T) {
	fija := func() time.Time { return time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC) } // viernes 24 de julio
	acts := buildActions(fija)
	reply, _ := acts["fecha"].Run(&fakeRunner{})
	if reply != "Hoy es viernes 24 de julio." {
		t.Fatalf("fue %q", reply)
	}
}

func TestClimaVacioAmable(t *testing.T) {
	acts := buildActions(time.Now)
	reply, err := acts["clima"].Run(&fakeRunner{output: ""})
	if err != nil || reply != "No pude ver el clima." {
		t.Fatalf("reply=%q err=%v", reply, err)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestQueSuena|TestBateria|TestFecha|TestClima'` → FALLA.

- [ ] **Step 3: Implementar en `actions.go`**

Agregá `"net/url"`? no acá; agregá `"strings"` a los imports de `actions.go` (ya tiene `fmt`, `time`).
En `buildActions`, después de `addDesktopActions(add)`:
```go
	addInfoActions(add, now)
```
Agregá las funciones:
```go
// addInfoActions registra tools de media/sistema/info (no destructivas).
func addInfoActions(add func(*Action), now func() time.Time) {
	add(&Action{Name: "anterior", Desc: "volver a la pista anterior", Face: Feliz, Run: playerctl("previous", "Anterior.")})

	add(&Action{Name: "que_suena", Desc: "decir qué está sonando", Face: Curioso, Run: func(r Runner) (string, error) {
		out, err := r.Run("playerctl", "metadata", "--format", "{{artist}} - {{title}}")
		out = strings.TrimSpace(out)
		if err != nil || out == "" || out == "-" {
			return "No hay nada sonando.", nil
		}
		return "Está sonando: " + out + ".", nil
	}})

	add(&Action{Name: "silenciar_micro", Desc: "silenciar o activar el micrófono", Face: Neutral, Run: func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-mute", "@DEFAULT_AUDIO_SOURCE@", "toggle"); err != nil {
			return "", fmt.Errorf("no pude tocar el micrófono: %w", err)
		}
		return "Micrófono cambiado.", nil
	}})

	add(&Action{Name: "bateria", Desc: "decir el porcentaje de batería", Face: Neutral, Run: func(r Runner) (string, error) {
		devs, err := r.Run("upower", "-e")
		if err != nil {
			return "", fmt.Errorf("no pude leer la batería: %w", err)
		}
		dev := ""
		for _, l := range strings.Split(devs, "\n") {
			if strings.Contains(l, "battery") {
				dev = strings.TrimSpace(l)
				break
			}
		}
		if dev == "" {
			return "No encontré la batería.", nil
		}
		info, err := r.Run("upower", "-i", dev)
		if err != nil {
			return "", fmt.Errorf("no pude leer la batería: %w", err)
		}
		for _, l := range strings.Split(info, "\n") {
			if strings.Contains(l, "percentage:") {
				return "Batería al " + strings.TrimSpace(strings.SplitN(l, ":", 2)[1]) + ".", nil
			}
		}
		return "No pude leer el porcentaje.", nil
	}})

	add(&Action{Name: "fecha", Desc: "decir el día y la fecha de hoy", Face: Neutral, Run: func(r Runner) (string, error) {
		return "Hoy es " + fechaES(now()) + ".", nil
	}})

	add(&Action{Name: "clima", Desc: "decir el clima actual", Face: Curioso, Run: func(r Runner) (string, error) {
		out, err := r.Run("curl", "-s", "wttr.in/?format=%C+%t")
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			return "No pude ver el clima.", nil
		}
		return "El clima: " + out + ".", nil
	}})

	add(&Action{Name: "agenda", Desc: "decir el próximo evento de la agenda", Face: Curioso, Run: func(r Runner) (string, error) {
		out, err := r.Run("khal", "list", "now", "24h", "--format", "{start-time} {title}")
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			return "No tenés nada en la agenda por ahora.", nil
		}
		return "Próximo: " + strings.SplitN(out, "\n", 2)[0] + ".", nil
	}})

	add(&Action{Name: "bloquear", Desc: "bloquear la pantalla", Face: Dormido, Run: func(r Runner) (string, error) {
		if _, err := r.Run("hyprlock"); err != nil {
			return "", fmt.Errorf("no pude bloquear: %w", err)
		}
		return "Bloqueando.", nil
	}})
}

// fechaES formatea la fecha en español (time.Format no localiza).
func fechaES(t time.Time) string {
	dias := []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	meses := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("%s %d de %s", dias[int(t.Weekday())], t.Day(), meses[int(t.Month())-1])
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS (nuevos + A–J intactos).

- [ ] **Step 5: Commit**

```bash
git add actions.go actions_test.go
git commit -m "feat: tools sin arg — anterior/que_suena/silenciar_micro/bateria/fecha/clima/agenda/bloquear"
```

---

### Task 2: Mecanismo `ArgTool` + `volumen_a` + wiring

**Files:** Modify `actions.go`, `llm.go`, `main.go`, `llm_test.go`
**Interfaces:**
- Produces: `type ArgTool struct{ Desc string; Build func(arg string) *Action }`; `buildArgActions() map[string]*ArgTool`; `LLMConfig.ArgActions`; ruteo en `Interpret`; `systemPrompt` lista argActions.

- [ ] **Step 1: Tests que fallan (agregar a `llm_test.go`)**

```go
func newLLMArg(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		ArgActions: buildArgActions()})
}

func TestLLMArgToolVolumen(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLMArg(fakeChat(`{"action":"volumen_a","arg":"30","say":"Dale, al 30."}`, nil)).Interpret("poné el volumen en 30")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Dale, al 30." {
		t.Fatalf("esperaba el say, fue %q", reply)
	}
	want := []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "30%"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestLLMArgToolVolumenInvalido(t *testing.T) {
	a, err := newLLMArg(fakeChat(`{"action":"volumen_a","arg":"altísimo","say":"ok"}`, nil)).Interpret("subilo a full")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	if _, err := a.Run(&fakeRunner{}); err == nil {
		t.Fatal("un arg no numérico debía dar error al ejecutar")
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run TestLLMArgTool` → FALLA (`ArgActions`/`buildArgActions` no existen).

- [ ] **Step 3: `actions.go` — `ArgTool` + `buildArgActions`**

Agregá `"net/url"` y `"strconv"` a los imports de `actions.go`. Agregá:
```go
// ArgTool es una tool que necesita un argumento (el LLM lo pasa en `arg`). Build liga el arg y
// devuelve la acción. Van en un mapa aparte porque el registro normal no lleva arg.
type ArgTool struct {
	Desc  string
	Build func(arg string) *Action
}

// buildArgActions arma las tools con arg (todas por Runner/argv → sin shell, sin inyección).
func buildArgActions() map[string]*ArgTool {
	return map[string]*ArgTool{
		"volumen_a": {Desc: "poner el volumen en un valor 0-150 (arg = número)", Build: func(arg string) *Action {
			return &Action{Name: "volumen_a", Face: Neutral, Run: func(r Runner) (string, error) {
				n, err := strconv.Atoi(strings.TrimSpace(arg))
				if err != nil || n < 0 || n > 150 {
					return "", fmt.Errorf("volumen inválido: %q", arg)
				}
				if _, err := r.Run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", strconv.Itoa(n)+"%"); err != nil {
					return "", fmt.Errorf("no pude cambiar el volumen: %w", err)
				}
				return "Volumen al " + strconv.Itoa(n) + " por ciento.", nil
			}}
		}},
	}
}
```

- [ ] **Step 4: `llm.go` — campo + ruteo + prompt**

En `LLMConfig` agregá:
```go
	ArgActions map[string]*ArgTool // tools con arg (nil = ninguna)
```
En el struct `LLMInterpreter` agregá `argActions map[string]*ArgTool`; en el `return &LLMInterpreter{…}` del constructor agregá `argActions: cfg.ArgActions,`.
En `Interpret`, **antes** de `if a, ok := li.actions[choice.Action]; ok {`:
```go
	if t, ok := li.argActions[choice.Action]; ok {
		li.remember(text, choice.Say)
		return wrap(t.Build(choice.Arg), choice.Say), nil
	}
```
En `systemPrompt`, después del bloque de `vision` (antes del `return`):
```go
	argNames := make([]string, 0, len(li.argActions))
	for n := range li.argActions {
		argNames = append(argNames, n)
	}
	sort.Strings(argNames)
	for _, n := range argNames {
		fmt.Fprintf(&b, "- %s: %s\n", n, li.argActions[n].Desc)
	}
```

- [ ] **Step 5: `main.go` — wiring**

En el `LLMConfig{…}` de `main.go` agregá:
```go
			ArgActions: buildArgActions(),
```

- [ ] **Step 6: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 7: Commit**

```bash
git add actions.go llm.go main.go llm_test.go
git commit -m "feat: mecanismo ArgTool (factories con arg) + volumen_a; ruteo en Interpret + prompt"
```

---

### Task 3: Resto de tools con arg

**Files:** Modify `actions.go`, `actions_test.go`
**Produces:** `brillo`, `abrir_url`, `buscar`, `ir_a_workspace` en `buildArgActions`.

- [ ] **Step 1: Tests que fallan (agregar a `actions_test.go`)**

```go
func TestArgBrilloSubir(t *testing.T) {
	fake := &fakeRunner{}
	if _, err := buildArgActions()["brillo"].Build("subir").Run(fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"brightnessctl", "set", "+10%"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestArgAbrirUrlAgregaHttps(t *testing.T) {
	fake := &fakeRunner{}
	if _, err := buildArgActions()["abrir_url"].Build("youtube.com").Run(fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"xdg-open", "https://youtube.com"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestArgBuscarUrlEncode(t *testing.T) {
	fake := &fakeRunner{}
	if _, err := buildArgActions()["buscar"].Build("gatos monos").Run(fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"xdg-open", "https://duckduckgo.com/?q=gatos+monos"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestArgWorkspaceInvalido(t *testing.T) {
	if _, err := buildArgActions()["ir_a_workspace"].Build("catorce").Run(&fakeRunner{}); err == nil {
		t.Fatal("workspace no numérico debía dar error")
	}
}
```
(agregá `"reflect"` a los imports de `actions_test.go` si falta.)

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestArgBrillo|TestArgAbrir|TestArgBuscar|TestArgWorkspace'` → FALLA.

- [ ] **Step 3: Agregar al mapa de `buildArgActions` en `actions.go`**

Dentro del `map[string]*ArgTool{…}`, agregá:
```go
		"brillo": {Desc: "ajustar el brillo (arg = subir, bajar, o un número 0-100)", Build: func(arg string) *Action {
			return &Action{Name: "brillo", Face: Neutral, Run: func(r Runner) (string, error) {
				var val string
				switch strings.TrimSpace(arg) {
				case "subir":
					val = "+10%"
				case "bajar":
					val = "10%-"
				default:
					n, err := strconv.Atoi(strings.TrimSpace(arg))
					if err != nil || n < 0 || n > 100 {
						return "", fmt.Errorf("brillo inválido: %q", arg)
					}
					val = strconv.Itoa(n) + "%"
				}
				if _, err := r.Run("brightnessctl", "set", val); err != nil {
					return "", fmt.Errorf("no pude cambiar el brillo: %w", err)
				}
				return "Brillo ajustado.", nil
			}}
		}},
		"abrir_url": {Desc: "abrir una URL en el navegador (arg = la url)", Build: func(arg string) *Action {
			return &Action{Name: "abrir_url", Face: Curioso, Run: func(r Runner) (string, error) {
				u := strings.TrimSpace(arg)
				if u == "" {
					return "", fmt.Errorf("no dijiste qué URL")
				}
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
					u = "https://" + u
				}
				if _, err := r.Run("xdg-open", u); err != nil {
					return "", fmt.Errorf("no pude abrir la URL: %w", err)
				}
				return "Abriendo la página.", nil
			}}
		}},
		"buscar": {Desc: "buscar algo en el navegador (arg = qué buscar)", Build: func(arg string) *Action {
			return &Action{Name: "buscar", Face: Curioso, Run: func(r Runner) (string, error) {
				q := strings.TrimSpace(arg)
				if q == "" {
					return "", fmt.Errorf("no dijiste qué buscar")
				}
				if _, err := r.Run("xdg-open", "https://duckduckgo.com/?q="+url.QueryEscape(q)); err != nil {
					return "", fmt.Errorf("no pude buscar: %w", err)
				}
				return "Buscando " + q + ".", nil
			}}
		}},
		"ir_a_workspace": {Desc: "cambiar de escritorio/workspace (arg = número 1-10)", Build: func(arg string) *Action {
			return &Action{Name: "ir_a_workspace", Face: Neutral, Run: func(r Runner) (string, error) {
				n, err := strconv.Atoi(strings.TrimSpace(arg))
				if err != nil || n < 1 || n > 10 {
					return "", fmt.Errorf("workspace inválido: %q", arg)
				}
				if _, err := r.Run("hyprctl", "dispatch", "workspace", strconv.Itoa(n)); err != nil {
					return "", fmt.Errorf("no pude cambiar de workspace: %w", err)
				}
				return "Workspace " + strconv.Itoa(n) + ".", nil
			}}
		}},
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 5: E2E (manual — necesita DeepSeek con saldo + micrófono; lo corre el usuario)**

Con Astro corriendo (`SUPER+SHIFT+A`): "poné el volumen en 30", "qué está sonando", "cuánta batería queda", "qué clima hace", "abrí youtube punto com", "buscá pizza cerca", "andá al workspace 3", "bloqueá la pantalla".

- [ ] **Step 6: Commit**

```bash
git add actions.go actions_test.go
git commit -m "feat: tools con arg — brillo/abrir_url/buscar/ir_a_workspace"
```

---

## Definition of Done (Fase K)

1. `go build/vet/test ./...` verdes (tools sin-arg, mecanismo ArgTool, tools con-arg + validación; + A–J intactos).
2. E2E: los ejemplos del Step 5 andan por voz/DeepSeek.
3. Interfaz `Interpret` sin cambios de firma; seguridad/memoria/visión intactas; nada destructivo.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Cobertura del spec:** tools sin-arg → Task 1; mecanismo ArgTool + ruteo/prompt/wiring → Task 2; resto arg-tools → Task 3.
- **Compila en cada tarea:** Task 1 solo agrega al registro (Interpret ya lo rutea). Task 2 agrega el campo `ArgActions` (opcional, nil-safe) + una rama + wiring. Task 3 solo agrega entradas al mapa. Nada roto entre tareas.
- **Sin inyección:** todas por `Runner`/argv (no shell). Validación de correctitud: numérico (`volumen_a`/`ir_a_workspace`), esquema (`abrir_url`), no-vacío (`buscar`) → arg inválido = error al ejecutar, no comando basura.
- **nil-safe:** `argActions` nil (tests viejos que no lo setean) → la rama `if t, ok := li.argActions[...]` no matchea → sigue al registro/fallback. Sin panic.
- **Anti-verde-falso:** tests afirman el comando exacto (`reflect.DeepEqual` sobre `lastCall`), el reply, y el error en args inválidos — no "no crashea". Las que leen-para-decir se testean con `fakeRunner.output`.
- **No destructivo:** ninguna borra/cierra; `bloquear` es benigno; destructivas y `timer` quedan afuera (spec §2).
