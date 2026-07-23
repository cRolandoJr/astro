# Spec — Astro · Fase B (voz: te escucha y te habla)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase A (mergeada en `main`).
> Convenciones: identificadores en **inglés**, comentarios/explicaciones en **español**.
> Todo se orquesta con binarios externos vía el `Runner` (igual que hyprctl/playerctl en Fase A).

## 1. Objetivo

Cerrar el loop de voz: **le hablás a Astro y te responde con voz**, sin dejar de hacer lo de Fase A.
Apretás Enter (push-to-talk) → Astro graba → transcribe (Whisper) → interpreta y actúa (pipeline de
Fase A) → cambia la cara → y **dice la respuesta en voz alta** (Piper).

## 2. Alcance

**Incluye:** captura de audio (push-to-talk por terminal, ventana fija), STT con whisper.cpp local,
salida TTS con Piper local, e integración al loop existente. Los tests cubren la construcción de
comandos y el parseo de la transcripción con `fakeRunner` (sin audio real).

**NO incluye (diferido):** hotkey global de Hyprland (Fase B.5 — es solo cambiar el disparador),
grabar-hasta-silencio / Enter-para-parar (VAD), wake word (Fase 6), LLM (Fase 5), hardware ESP32.

## 3. Arquitectura — cómo encaja en lo de Fase A

Se agregan dos piezas y se cambia solo la **fuente de entrada** y se suma la **salida de voz**. El
`Interpreter`, las `Action` y el `Display` de Fase A **no se tocan**.

| Pieza | Rol | Interfaz / struct | Impl. Fase B |
|---|---|---|---|
| `InputSource` | de dónde viene el texto del comando | **interfaz** `Listen() (text string, err error)` | `VoiceInput` + `StdinInput` |
| `PiperVoice` | decir un texto en voz alta | **struct** (una impl) | `PiperVoice` |
| `Runner` (extendido) | correr binarios; **ahora también con stdin** | +`RunWithInput(stdin, name, args...)` | `ExecRunner` |

- **`InputSource` SÍ es interfaz** porque hay **dos implementaciones reales hoy**: `VoiceInput` (grabar +
  Whisper) y `StdinInput` (leer una línea — útil para debuggear el pipeline y el TTS sin micrófono).
  `main` elige por un env (`ASTRO_INPUT=voice|stdin`). Dos consumidores presentes → la interfaz se gana.
- **La voz es un struct `PiperVoice`, NO interfaz** — una sola implementación y nadie la testea *a través*
  de la interfaz (los tests de `PiperVoice` usan el `fakeRunner`, no un `fakeVoice`). Meter interfaz sería
  la abstracción especulativa que evitamos (misma regla que `Action` en Fase A). Si mañana la voz sale del
  robot, en Go extraemos la interfaz gratis. *(Aplicando la lección del mentor de Fase A, no repitiéndola.)*
- **`Runner` gana un método `RunWithInput`**: Piper lee el texto por **stdin**, y el `Run` actual (con
  `CombinedOutput`) no alimenta stdin. Es una necesidad real de hoy, no especulativa.

## 4. Flujo de datos

```
[Enter]  →  VoiceInput.Listen():
              1. grabar audio     → arecord -d 4 ... /tmp/astro-in.wav   (Runner.Run)
              2. transcribir       → whisper.cpp -f in.wav -l es ...       (Runner.Run) → texto
           ↓ (texto)
        Interpreter.Interpret(texto) → Action        [pipeline de Fase A, sin cambios]
           ↓
        Action.Run(runner)  +  Display.Show(cara)
           ↓ (reply string)
        Voice.Say(reply):
              1. sintetizar        → piper --model voz ... (texto por stdin) → /tmp/astro-out.wav  (RunWithInput)
              2. reproducir        → pw-play /tmp/astro-out.wav              (Runner.Run)
```

Todo shellout pasa por `Runner`/`RunWithInput` → todo testeable con `fakeRunner` sin tocar audio real.

## 5. Herramientas externas y modelos

| Qué | Herramienta | Cómo se obtiene |
|---|---|---|
| Grabar audio | `arecord` (alsa-utils; anda sobre PipeWire) — 4s, 16kHz, mono, S16 | `nixpkgs#alsa-utils` |
| STT | **whisper.cpp** (binario `whisper-cpp`) + modelo `ggml-small.bin` (multilingüe) | `nixpkgs#whisper-cpp` + bajar el modelo una vez |
| TTS | **Piper** (`piper-tts`) + una voz en español (`.onnx` + `.json`) | `nixpkgs#piper-tts` + bajar la voz una vez |
| Reproducir | `pw-play` (PipeWire) | `nixpkgs#pipewire` (ya lo tenés) |

Los modelos (whisper + voz Piper) se bajan una vez y se guardan fuera del repo (son pesados); sus
rutas se pasan por variables de entorno (§6). Se documenta el comando de descarga en el plan.

## 6. Configuración (variables de entorno)

- `ASTRO_WHISPER_BIN` / `ASTRO_WHISPER_MODEL` — binario y ruta del modelo ggml.
- `ASTRO_PIPER_BIN` / `ASTRO_PIPER_VOICE` — binario y ruta del `.onnx`.
- `ASTRO_REC_SECONDS` (opcional, default 4) — duración de la ventana de grabación.
- Reusa `ASTRO_EWW_CONFIG` de Fase A.

Mismo criterio que Fase A: rutas por env, sin hardcodear el home.

## 7. Manejo de errores

- **No se entendió el audio / transcripción vacía** → cara `pensativo` + "no te escuché, repetí".
- **Falta un binario/modelo** (whisper/piper no instalado o ruta mala) → mensaje claro de qué falta y
  cómo instalarlo; el loop no se corta.
- **Falla la reproducción** → se muestra el texto igual (degradación: si no puede hablar, al menos responde por pantalla).
- La transcripción se **normaliza** con lo de Fase A antes de interpretar (ya saca acentos/minúsculas).

## 8. Testing

- `VoiceInput.Listen()`: con `fakeRunner` devolviendo una transcripción canned, verificar (a) que se
  arma bien el comando de `arecord` y de whisper, y (b) que se extrae el texto limpio de la salida de
  whisper (whisper.cpp imprime con timestamps; hay que parsear/limpiar — se testea ese parseo).
- `PiperVoice.Say()`: con `fakeRunner`, verificar que se pasa el texto por stdin a piper y que se
  reproduce el wav. (Usa el `RunWithInput` nuevo; el fake registra el stdin.)
- **Audio real (mic + parlante) = verificación manual E2E** (como fue eww en Fase A): no se puede
  testear headless.

## 9. Decisiones tomadas (defaults — vetables en la revisión)

- **Trigger:** push-to-talk por terminal (Enter), ventana **fija de 4s**. (Enter-para-parar y VAD, diferidos.)
- **STT:** whisper.cpp modelo **`small`** multilingüe (mejor precisión en español que `base`, a costa de
  algo de velocidad en CPU; sigue siendo swappable por env var si querés `base` para más rapidez).
- **TTS:** Piper con una voz **español** (`es_ES`; alternativas `es_MX`/`es_AR` si preferís el acento).
- **Idioma whisper:** forzado a `es`.
- **Grabación:** `arecord -d N` (ventana fija = un solo comando Run-and-wait, sin control de proceso →
  simple y testeable).

## 10. Estructura de archivos (nuevos / tocados)

```
~/projects/astro/
  runner.go          — (modificar) agregar RunWithInput al interface + ExecRunner
  fake_test.go       — (modificar) fakeRunner registra el stdin
  input.go           — (nuevo) InputSource interface + VoiceInput + StdinInput
  input_test.go      — (nuevo) parseo de whisper + comandos, con fakeRunner
  voice.go           — (nuevo) PiperVoice (struct)
  voice_test.go      — (nuevo) con fakeRunner
  main.go            — (modificar) elegir InputSource por env + PiperVoice.Say(reply) al final
```

## 11. Definition of Done (Fase B)

1. `go build ./...` y `go test ./...` verdes (incluye tests nuevos de VoiceInput y PiperVoice con fake).
2. Con whisper y piper instalados + modelos: apretás Enter, decís "hola"/"qué hora es"/"pausá",
   Astro **transcribe**, actúa, cambia la cara y **responde en voz alta**.
3. Audio vacío/no entendido → cara `pensativo` + pedido de repetir, sin cortar el loop.
4. Falta de binario/modelo → mensaje claro, sin crash.
5. Convenciones respetadas (ids inglés, comentarios español, nada destructivo, solo stdlib en Go).
