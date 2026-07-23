# Astro · Fase D — Plan de Implementación ("dice más": el LLM redacta la voz y conversa)

> **Para quien ejecuta:** SUB-SKILL: superpowers:subagent-driven-development, tarea por tarea. Checkbox `- [ ]`.

**Goal:** El LLM devuelve `{action?, arg?, say}`. Con acción → la ejecuta y **habla el `say`**; sin acción →
**solo habla el `say`** (conversación). La interfaz `Interpreter` no cambia (el `say` va como la respuesta
de un `*Action`).

**Architecture:** Todo en `llm.go`. Dos helpers: `wrap(base, say)` (corre el efecto, habla el `say`) y
`sayAction(say)` (charla pura). `main`/voz/cara/guardrails intactos. Backwards-compat: sin `say` → frase fija.

**Tech Stack:** Go, stdlib. LLM: Gemini free (Fase C). Spec: `docs/specs/2026-07-23-astro-fase-d-design.md`.

## Global Constraints

- Identificadores en **inglés**; comentarios/mensajes en **español**. Nada destructivo; solo stdlib.
- TDD: rojo → mínimo → verde → commit. Go vía `nix shell nixpkgs#go --command <cmd>`.
- Seguridad intacta: acciones acotadas al menú + `isSafeAppName`; el `say` es solo voz.

---

### Task 1: `say` en `Interpret` + `wrap` + `sayAction`

**Files:** Modify `llm.go`, `llm_test.go`
**Produces:** parseo de `say`; `wrap(base *Action, say string) *Action`; `sayAction(say string) *Action`.

- [ ] **Step 1: Tests que fallan (agregar a `llm_test.go`)**

```go
func TestLLMActionConSayHablaElSay(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"pausar","say":"Dale, te pausé."}`, nil)).Interpret("poné pausa")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Dale, te pausé." {
		t.Fatalf("esperaba el say del LLM, fue %q", reply)
	}
	if got := fake.lastCall(); len(got) < 1 || got[0] != "playerctl" {
		t.Fatalf("esperaba que corriera playerctl, fue %v", got)
	}
}

func TestLLMCharlaSoloHabla(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"none","say":"Estoy bien, ¿y vos?"}`, nil)).Interpret("cómo estás")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Estoy bien, ¿y vos?" {
		t.Fatalf("esperaba la charla, fue %q", reply)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("la charla no debe ejecutar comandos, hubo %v", fake.calls)
	}
}

func TestLLMSayPuroSinAction(t *testing.T) {
	a, err := newLLM(fakeChat(`{"say":"un chiste corto"}`, nil)).Interpret("contame un chiste")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, _ := a.Run(&fakeRunner{})
	if reply != "un chiste corto" {
		t.Fatalf("fue %q", reply)
	}
}
```

- [ ] **Step 2: Correr — falla**

Run: `nix shell nixpkgs#go --command go test ./... -run 'TestLLMActionConSay|TestLLMCharla|TestLLMSayPuro'` → FALLA.

- [ ] **Step 3: Modificar `Interpret` + agregar helpers en `llm.go`**

En el struct de parseo, agregá `Say`:
```go
	var choice struct {
		Action string `json:"action"`
		Arg    string `json:"arg"`
		Say    string `json:"say"`
	}
```
Reemplazá el bloque de mapeo (desde `if choice.Action == "open"` hasta el `return li.fallback...` final) por:
```go
	// Charla pura: sin acción (o "none") pero con say → solo hablar.
	if (choice.Action == "" || choice.Action == "none") && choice.Say != "" {
		return sayAction(choice.Say), nil
	}
	if choice.Action == "open" {
		if !isSafeAppName(choice.Arg) {
			return li.fallback.Interpret(text) // arg peligroso → no por acá
		}
		return wrap(openAppAction(choice.Arg), choice.Say), nil
	}
	if a, ok := li.actions[choice.Action]; ok {
		return wrap(a, choice.Say), nil
	}
	return li.fallback.Interpret(text) // ni acción ni say → reglas
```
Y agregá los helpers al final de `llm.go`:
```go
// wrap devuelve una acción que corre el efecto de 'base' pero habla 'say' (lo que
// redactó el LLM) en vez de la frase fija. Si say=="", devuelve base tal cual (compat).
func wrap(base *Action, say string) *Action {
	if say == "" {
		return base
	}
	return &Action{Name: base.Name, Desc: base.Desc, Face: base.Face,
		Run: func(r Runner) (string, error) {
			if _, err := base.Run(r); err != nil {
				return "", err
			}
			return say, nil
		}}
}

// sayAction devuelve una acción que SOLO habla 'say' (charla; no toca la máquina).
func sayAction(say string) *Action {
	return &Action{Name: "decir", Face: Feliz,
		Run: func(r Runner) (string, error) { return say, nil }}
}
```

- [ ] **Step 4: Correr — pasa**

Run: `nix shell nixpkgs#go --command sh -c 'go vet ./... && go test ./...'` → PASS (nuevos + todos los
previos; las pruebas de Fase C sin `say` siguen verdes porque `wrap(base,"")` devuelve `base`).

- [ ] **Step 5: Commit**

```bash
git add llm.go llm_test.go
git commit -m "feat: dice más — say del LLM (wrap acción+say / sayAction charla); interfaz intacta"
```

---

### Task 2: `systemPrompt` con el nuevo formato + E2E

**Files:** Modify `llm.go` (solo el texto de `systemPrompt`)

- [ ] **Step 1: Actualizar `systemPrompt`**

Cambiá el cuerpo de `systemPrompt` para pedir el nuevo formato y respuestas hablables. Dejá el armado del
menú (el `for` sobre las acciones + la línea de `open`) igual; cambiá solo el texto de instrucción:
```go
	b.WriteString("Sos Astro, un asistente de escritorio con voz. El usuario te habla en español. ")
	b.WriteString("Respondé SOLO un JSON: {\"action\":\"<opcional>\",\"arg\":\"<opcional>\",\"say\":\"<respuesta hablada>\"}. ")
	b.WriteString("Si es un COMANDO, elegí un `action` del menú y un `say` corto de confirmación. ")
	b.WriteString("Si es CHARLA o una pregunta, usá action:\"none\" y contestá en `say`. ")
	b.WriteString("El `say` se lee en voz alta: que sea BREVE (1-2 frases), natural y en español. Menú:\n")
```
(El resto de `systemPrompt` —el `for names` y la línea `open`— queda igual.)

- [ ] **Step 2: Compila + tests verdes**

Run: `nix shell nixpkgs#go --command sh -c 'go build ./... && go vet ./... && go test ./...'` → OK + PASS.

- [ ] **Step 3: E2E con Gemini (manual — necesita la key)**

```bash
export ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai
export ASTRO_LLM_KEY='TU_KEY'
export ASTRO_LLM_MODEL=gemini-flash-latest
cd ~/projects/astro
printf 'che poné pausa\ncontame un chiste corto\n¿cómo estás?\n' | ASTRO_INPUT=stdin \
  nix shell nixpkgs#go --command go run .
```
Expected: "che poné pausa" → pausa + confirmación redactada por el LLM; "contame un chiste" y "¿cómo estás?"
→ **respuestas conversacionales** (charla). (Y por voz: el flujo completo, ahora conversando.)

- [ ] **Step 4: Commit**

```bash
git add llm.go
git commit -m "feat: systemPrompt para dice más (comando→action+say / charla→say breve hablable)"
```

---

## Definition of Done (Fase D)

1. `go build/vet/test ./...` verdes (tests de action+say, charla, say puro; + los de Fase C intactos).
2. E2E: comando → acción + **confirmación hablada del LLM**; pregunta sin acción → **respuesta conversacional**.
3. Fallback y seguridad de Fase C intactos.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.

## Auto-revisión (hecha)

- **Backwards-compat:** `wrap(base, "")` devuelve `base` → los tests de Fase C (JSON sin `say`) siguen verdes.
- **Seguridad:** `open` sigue por `isSafeAppName` antes de `wrap`; el `say` no ejecuta nada (`sayAction` no toca Runner).
- **Sin fugas de error:** todo camino sigue devolviendo `(*Action, nil)` o va al fallback (`nil`/`ErrNoEntiendo`) → el guard de `main` sigue sin poder deref-ear nil.
- **Interfaz intacta:** `Interpret` sigue devolviendo `(*Action, error)`; `main`/voz/cara no se tocan.
