# Spec — Astro · Fase D ("dice más": el LLM redacta la voz y conversa)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase C (LLM, en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib.

## 1. Objetivo

Que Astro **hable con respuestas propias** en vez de frases fijas, y **converse** cuando no hay una
acción: *"che poné pausa"* → pausa **y** dice *"Dale, te pausé 🎵"*; *"contame un chiste"* → responde el
chiste con voz. El LLM decide: ¿comando (acción + confirmación redactada) o charla (solo respuesta)?

## 2. Alcance

**Incluye:** el LLM devuelve `{action?, arg?, say}`; `say` es la respuesta hablada (redactada por el LLM);
si hay `action` se ejecuta y se habla el `say`; si no, solo se habla el `say` (conversación). Prompt para
respuestas **cortas y hablables**. Tests con `chat` fake.

**NO incluye (diferido):** **memoria de conversación multi-turno** (cada frase es independiente; va con
Fase E); que la charla dispare acciones nuevas fuera del menú; streaming de la respuesta.

## 3. Arquitectura — la interfaz NO cambia

El LLM ahora responde `{"action":"<opcional>","arg":"<opcional>","say":"<respuesta hablada>"}`. El
`LLMInterpreter` traduce eso a un `*Action` (misma interfaz `Interpret`), metiendo el `say` como la
**respuesta** del action. `main`, la voz, la cara y los guardrails **no se tocan**.

```
choice = parse(LLM JSON)  // {action, arg, say}

si action ∈ registro (o "open" válido):
    → wrap(baseAction, say):  corre el efecto real, pero habla `say` (si vino) en vez de la frase fija
si no hay action (o "none") y hay say:
    → sayAction(say):  no toca nada, solo habla `say`   ← la CONVERSACIÓN
si no hay action ni say:
    → fallback (reglas → acción o ErrNoEntiendo)

open sigue pasando por isSafeAppName (guardrail intacto).
```

- **`wrap(base, say)`**: si `say==""` devuelve `base`; si no, un `*Action` que corre `base.Run` (el efecto)
  y devuelve `say` como reply. Si `base.Run` falla, propaga el error (main lo maneja, sin panic).
- **`sayAction(say)`**: `&Action{Name:"decir", Face: Feliz, Run: func(Runner)(string,error){ return say, nil }}`
  — no ejecuta comandos, solo habla. Es el camino de charla.

## 4. Prompt (system)

Se actualiza `systemPrompt` para pedir el nuevo formato y **respuestas breves/hablables** (van a TTS,
nada de párrafos), en español, tono robotcito:
- Si es un **comando** → elegí `action` (del menú) y un `say` corto de confirmación.
- Si es **charla/pregunta** → `action:"none"` y contestá en `say` (breve, natural).
- `open` para abrir apps (arg = nombre).

## 5. Seguridad (igual)

- Las **acciones** siguen acotadas al menú + `isSafeAppName`. El `say` es **solo voz** — no ejecuta nada.
- El LLM puede *alucinar* al conversar (decir algo impreciso), pero es texto hablado; **cero riesgo de
  ejecución**. Sin cambios en los guardrails.

## 6. Testing (sin red)

- `chat` fake `{"action":"pausar","say":"Dale, te pausé."}` → el action corre `playerctl play-pause`
  (verificado con `fakeRunner`) **y** su reply es `"Dale, te pausé."` (no la frase fija).
- `{"action":"none","say":"Estoy bien, ¿y vos?"}` → `sayAction`: reply es ese texto, **sin** comandos corridos.
- `{"say":"un chiste..."}` (sin action) → `sayAction`.
- Sigue el fallback (chat error / JSON inválido) y el rechazo de `open` con arg peligroso (Fase C).

## 7. Decisiones (defaults, vetables)

- **Sin memoria** de conversación en v1 (stateless por frase). Multi-turno → Fase E.
- Cara durante la charla: `Feliz`.
- Reusa `ASTRO_LLM_*` de Fase C; sin envs nuevos.

## 8. Estructura de archivos

```
~/projects/astro/
  llm.go       — (modificar) parsear `say`; helpers wrap() y sayAction(); systemPrompt nuevo formato
  llm_test.go  — (modificar) tests: action+say, none+say (charla), say puro
  (main.go, interpreter.go, voice.go, etc. — sin cambios)
```

## 9. Definition of Done (Fase D)

1. `go build/vet/test ./...` verdes (tests nuevos de `say`/charla con `chat` fake).
2. E2E: comando → acción **+ confirmación hablada redactada por el LLM**; pregunta sin acción → **respuesta hablada** (charla).
3. Fallback y seguridad de Fase C siguen intactos (tests).
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
