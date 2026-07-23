# Spec — Astro · Fase C (el cerebro: LLM entiende lenguaje natural)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase A y B (en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib.

## 1. Objetivo

Que le hables **natural** y Astro haga lo correcto: *"che, ponéme algo de música"* → media play;
*"subime un toque el volumen"* → subir volumen; *"abrime el firefox"* → abrir firefox. El LLM
**reemplaza al intérprete de reglas** eligiendo una de las **acciones que ya existen** (Fase A/B).

## 2. Alcance

**Incluye:** `LLMInterpreter` (implementa la interfaz `Interpreter` existente); prompt con el menú de
acciones + respuesta JSON; llamada a un endpoint **OpenAI-compatible** (chat completions, JSON mode);
**fallback** al `RuleInterpreter` si el LLM falla; campo `Desc` en `Action`; guard de error en `main`.

**NO incluye (diferido):** multi-paso / conversación (repreguntar, encadenar); function-calling nativo;
memoria / RAG (Fase D); nuevas acciones (las capacidades no cambian, solo *quién las elige*).

## 3. Arquitectura

`LLMInterpreter` entra en lugar de `RuleInterpreter` — misma interfaz `Interpret(text) (*Action, error)`,
así el resto (acciones, `Display`, `PiperVoice`, `Chirps`) **no se toca**. `main` elige por env
`ASTRO_BRAIN` (`llm` por default, `rules` para volver al de Fase A).

```go
type chatFunc func(systemPrompt, userText string) (reply string, err error)

type LLMInterpreter struct {
    chat     chatFunc     // llamada al LLM; inyectable (real por HTTP / fake en tests)
    actions  map[string]*Action
    fallback Interpreter  // el RuleInterpreter: red de seguridad si el LLM falla
}
```

**Flujo (por comando):**
```
texto → LLMInterpreter.Interpret:
   1. arma el menú: nombre + Desc de cada acción del registro, + la pseudo-acción "open" (toma un arg)
   2. chat(systemPrompt+menú, texto) → el LLM responde {"action":"open","arg":"firefox"}
   3. parsea el JSON:
        - action == "open"        → openAppAction(arg)  [pasa por isSafeAppName]
        - action ∈ registro       → esa Action
        - action == "none"/otro   → fallback.Interpret(text)   (quizá las reglas lo agarran)
   4. si chat() o el parseo fallan → fallback.Interpret(text)
   → devuelve *Action (lo ejecuta el mismo pipeline de siempre)
```

**El seam `chat`:** el default hace un `POST` (net/http) a `<ASTRO_LLM_URL>/chat/completions` con
`Authorization: Bearer <key>`, body `{model, messages:[system,user], response_format:{type:"json_object"},
temperature:0}`, y devuelve `choices[0].message.content`. En tests se inyecta un `chat` fake → sin red.

## 4. Configuración (env)

- `ASTRO_BRAIN` = `llm` (default) | `rules`.
- `ASTRO_LLM_URL`, `ASTRO_LLM_KEY`, `ASTRO_LLM_MODEL`. **Default para arrancar gratis — Gemini free tier:**
  - `ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai`
  - `ASTRO_LLM_KEY=<key de aistudio.google.com>` (sin tarjeta)
  - `ASTRO_LLM_MODEL=gemini-2.5-flash` (o el Flash más nuevo que liste AI Studio)
- Model-agnostic: cambiás a DeepSeek/OpenRouter/otro tocando esas 3 (todos OpenAI-compat).

## 5. Manejo de errores (sin crash, con degradación)

- **LLM caído / sin key / sin red / JSON inválido / acción desconocida** → `fallback.Interpret` (reglas).
  Si las reglas tampoco → `ErrNoEntiendo`.
- **`main` (guard del mentor, regla gatillada de Fase B):** el loop trata **cualquier** error del
  intérprete (no solo `ErrNoEntiendo`) como "no lo pude procesar" → cara `pensativo` + chirp `confused`
  + sigue. **Nunca** `action.Run` sobre un `*Action` nil → nunca panic.

## 6. Seguridad (igual que antes, no se afloja)

- El LLM **solo puede elegir del menú** (allowlist). Si devuelve algo fuera de la lista → fallback / "no
  entendí". **Nunca** ejecuta texto libre ni comandos crudos.
- `openApp` sigue pasando por **`isSafeAppName`** (el fix C1 de Fase A) — el arg del LLM se valida igual.
- A la nube viaja **solo el texto** de tu comando (no audio). La key es personal (no el Claude del laburo).

## 7. Testing (sin red)

- `LLMInterpreter.Interpret` con un `chat` fake que devuelve JSON canned:
  - `{"action":"pausar"}` → devuelve la acción `pausar`.
  - `{"action":"open","arg":"firefox"}` → acción que corre `hyprctl dispatch exec firefox` (verificado con `fakeRunner`).
  - `{"action":"open","arg":"firefox; rm -rf ~"}` → **rechazado** por `isSafeAppName` → fallback/ErrNoEntiendo (test de seguridad).
  - `chat` devuelve error → cae al `RuleInterpreter` (y un comando de regla como "pausá" resuelve igual).
  - JSON basura → fallback.
- El parseo del JSON y el armado del menú se testean como funciones puras donde se pueda.

## 8. Decisiones (defaults, vetables en revisión)

- Proveedor default para arrancar: **Gemini free tier** (AI Studio, sin tarjeta) vía OpenAI-compat;
  **JSON mode** + `temperature 0` (determinista). Free = familia Flash; ~1.500 req/día alcanzan de sobra.
- `ASTRO_BRAIN=llm` por default; `rules` conserva el comportamiento de Fase A.
- **Fallback a reglas** (reusa lo de Fase A como red de seguridad — nada se desperdicia).
- `Desc` en `Action` para armar el menú; "open" es una pseudo-acción con arg (como ya lo maneja el RuleInterpreter).

## 9. Estructura de archivos

```
~/projects/astro/
  llm.go            — (nuevo) LLMInterpreter + chatFunc + el chat real (HTTP OpenAI-compat)
  llm_test.go       — (nuevo) Interpret con chat fake: mapeo, open+seguridad, fallback
  action.go         — (modificar) agregar campo Desc a Action
  actions.go        — (modificar) completar Desc de cada acción
  interpreter.go    — (sin cambios de fondo; RuleInterpreter sigue siendo el fallback)
  main.go           — (modificar) elegir intérprete por ASTRO_BRAIN + guard de error genérico
```

## 10. Definition of Done (Fase C)

1. `go build/vet/test ./...` verdes (tests nuevos de LLMInterpreter con `chat` fake, incluido el de seguridad).
2. Con `ASTRO_LLM_*` seteados: le hablás natural y Astro elige la acción correcta y responde.
3. Sin key / sin red / `ASTRO_BRAIN=rules`: sigue andando con el intérprete de reglas (fallback).
4. Comando fuera del menú o error del LLM → cara pensativa + chirp, **sin crash**.
5. `isSafeAppName` sigue bloqueando inyección por el arg del LLM (test que lo prueba).
6. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
