# Spec — Astro · Fase F (conversación natural)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase E (memoria persistente, en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Que hablarle a Astro se sienta natural, en dos frentes independientes:

1. **Grabación por silencio (VAD):** hoy graba **fijo 4s** y te corta a mitad de frase. Debe grabar
   **mientras hablás y cortar sola al callarte** — sea una palabra o una parrafada.
2. **Contexto multi-turno:** hoy cada frase es independiente. Debe mantener el **hilo de la charla en
   curso** para entender seguimientos: *"subí el volumen"* → *"un poco más"*; *"abrí firefox"* → *"cerralo"*.

Son ortogonales (uno es *input*, otro es *cerebro*) pero apuntan al mismo objetivo; van juntos en la fase.

## 2. Alcance

**Incluye:**
- Reemplazar `arecord -d N` por `sox rec` con detección de silencio (arranca con voz, corta tras
  silencio de cola), con **tope duro** por si el umbral no engancha.
- Un **historial de conversación efímero** (RAM) de los últimos N turnos, inyectado al LLM como mensajes;
  **reset por inactividad** (charla nueva si pasó el umbral de silencio entre frases).

**NO incluye (diferido):** persistir la conversación (es efímera por diseño; reiniciar = charla nueva);
detección de fin-de-frase por modelo (VAD neuronal); barge-in (interrumpir a Astro mientras habla);
visión de pantalla (**próxima fase**, ya acordada).

## 3. Parte A — Grabación por silencio (VAD)

`sox rec` ya engancha con el PipeWire del user (verificado: abre `'default' (pulseaudio)`, graba y corta).
Reemplaza `arecord` en `VoiceInput.capture()`:

```
rec -q -c 1 -r 16000 <wav> silence 1 0.1 <thresh>% 1 <trail>s <thresh>%
```
- `silence 1 0.1 <thresh>%` → **arranca** a grabar cuando detecta 0.1s de sonido sobre el umbral.
- `1 <trail>s <thresh>%` → **corta** tras `<trail>` segundos de silencio bajo el umbral.
- **Tope duro** de `<max>` segundos anteponiendo `timeout <max>` (coreutils, ya en el PATH del nix shell)
  al comando: `Runner.Run("timeout", "<max>", "rec", …)`. Si el umbral nunca detecta silencio (ruido
  constante), `timeout` corta igual y no graba infinito. Es el "anticiparse a fallas", sin tocar `Runner`.

**Defaults (vetables por env):** `thresh=3%` (`ASTRO_REC_SILENCE_PCT`), `trail=1.5s`
(`ASTRO_REC_TRAIL_SEC`), `max=30s` (repurposa `ASTRO_REC_SECONDS` → ahora es el tope, no la duración fija).

El resto de `capture()` (whisper → limpiar transcripción → `entendí: %q`) queda igual. `arecord` sale;
`sox` ya está en el stack (efecto de voz y chirps).

## 4. Parte B — Contexto multi-turno

El LLM hoy recibe `{system, user}` de a una frase. Pasa a recibir `{system, historial…, user}`.

- **Historial efímero** en `LLMInterpreter`: ventana rodante de los últimos **N turnos** (default 6,
  `ASTRO_HISTORY_TURNS`). Cada turno = `{user, assistant}` = una ida y vuelta (lo que dijiste + el `say`
  que Astro habló). En RAM; **no se persiste**. Un turno sin `say` (comando mudo) no se registra.
- **Reset por inactividad:** si entre una frase y la siguiente pasaron más de **5 min**
  (`ASTRO_HISTORY_IDLE_MIN`), se vacía el historial antes de procesar → charla nueva. Requiere un reloj
  inyectado `now func() time.Time` (patrón ya usado en `buildActions`).
- **Registro del turno:** dentro de `Interpret`, tras parsear la respuesta del LLM, se agrega
  `{user: text, assistant: choice.Say}` al historial y se recorta a N. Solo se registra en el camino LLM
  exitoso (si cae a fallback por error/JSON inválido, no hay turno útil que guardar).

### Cambio de seam: `chatFunc`

```go
type Exchange struct{ User, Assistant string }
type chatFunc func(system string, history []Exchange, user string) (string, error)
```
`httpChat` arma el array de mensajes: `[{system}, {user:h1.User},{assistant:h1.Assistant}, …, {user}]`.
`Interpret` llama `li.chat(li.systemPrompt(facts), li.history, text)`.

**Lo que NO cambia:** la interfaz `Interpreter.Interpret(text) (*Action, error)`; la voz, la cara, la
seguridad de `open`, la memoria persistente (Fase E — los hechos siguen yendo al `systemPrompt`, aparte
del historial). El fallback a reglas y el logueo anti-verde-falso siguen igual.

## 5. Interacción con Fase E (memoria persistente)

Dos contextos distintos, sin colisión: los **hechos** (Fase E) van al `systemPrompt` (recuperados por
coseno); el **historial** (Fase F) va como mensajes previos. La query de `Recall` sigue siendo solo la
frase actual (`text`), no el historial.

## 6. Errores y bordes (anti-verde-falso)

- `rec` falla al abrir el device → mismo manejo que hoy (`capture` devuelve error → main reacciona, sin crash).
- Tope duro de grabación → siempre corta (no cuelga esperando silencio que no llega).
- Historial vacío (primer turno / post-reset) → el prompt es igual al de hoy (sin sección de historial).
- Reset por inactividad → se prueba con un `now` fake que salta el umbral (sin esperar en tests).
- Cambio de firma de `chatFunc` → actualiza `httpChat`, `fakeChat` y los tests existentes; los tests de
  Fase C/D/E siguen verdes (historial vacío = comportamiento previo).

## 7. Decisiones (defaults, vetables)

- VAD por `sox` (verificado con PipeWire); tope duro 30s; trail 1.5s; umbral 3%.
- Historial N=6, efímero, reset por inactividad 5 min.
- Se registra el `say` como turno del asistente (lo que Astro habló).
- Reusa envs; nuevos: `ASTRO_REC_SILENCE_PCT`, `ASTRO_REC_TRAIL_SEC`, `ASTRO_HISTORY_TURNS`,
  `ASTRO_HISTORY_IDLE_MIN` (+ `ASTRO_REC_SECONDS` repurposado a tope).

## 8. Estructura de archivos

```
~/projects/astro/
  input.go       — (modificar) capture(): sox rec + silence en vez de arecord; tope duro
  input_test.go  — (modificar/crear) capture() arma el comando rec correcto (fakeRunner)
  llm.go         — (modificar) Exchange, chatFunc con historial, LLMInterpreter.history + now,
                   Interpret threading + registro de turno + reset por inactividad; httpChat arma mensajes
  llm_test.go    — (modificar) nueva firma de fakeChat; tests de multi-turno + reset
  main.go        — (modificar) inyectar now (time.Now), envs de historial y de grabación
  (memory.go, embed.go, voice.go, action.go, etc. — sin cambios)
```

## 9. Definition of Done (Fase F)

1. `go build/vet/test ./...` verdes (tests de `capture` con rec, multi-turno, reset por inactividad;
   + A/B/C/D/E intactos).
2. E2E: (a) le hablás una frase corta y una larga → graba completo y corta sola al callarte (no a los 4s);
   (b) *"subí el volumen"* → *"un poco más"* se entiende como seguimiento; tras 5 min de silencio, arranca charla nueva.
3. Fallback/seguridad/memoria de fases previas intactos; interfaz `Interpret` sin cambios.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
