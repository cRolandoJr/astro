# Spec — Astro · Activación hands-free (wake-word "Astro" con openWakeWord)

> Fecha: 2026-07-24 · Estado: borrador · Rama `wake-oww` (desde `main`). Sigue a Fase H (wake plumbing) y Fase J (hotkey).
> Método: **liviano/hands-on** (no subagentes-TDD). El Go ya está hecho y testeado; el trabajo es empaquetado + sidecar + validación empírica en vivo.

## 1. Objetivo

Activación **100% hands-free**: le decís **"Astro"** en voz alta (sin tocar ninguna tecla) y se activa, escucha tu frase, responde, y vuelve a escuchar. Reemplaza al hotkey SUPER+SHIFT+A (que queda como fallback).

## 2. Qué ya existe (no se reconstruye)

El lado Go es **agnóstico del motor** y está en `main` (Fase H):
- `wakeword.go` → `WakeWordInput` corre un sidecar y escucha líneas `DETECTED` en su stdout; en cada una hace `capture()` → interpreta → responde → vuelve a escuchar. Loop hands-free completo. `Close()` = Kill+Wait.
- `main.go` → `ASTRO_INPUT=wake` + `ASTRO_WAKE_CMD=<comando del sidecar>`; cierre por defer y en la señal.

**Falta solo el motor** (el sidecar concreto + su empaquetado + el modelo de "Astro").

## 3. Decisiones

- **Motor = openWakeWord** (FOSS, sin cuenta/key, liviano para always-on). Descartados: Porcupine (gratis pero cuenta + no-FOSS), Vosk (más CPU), whisper always-on (pesado).
- **Palabra = "Astro"** (a secas). Riesgo asumido: 2 sílabas → más falsos disparos; se compensa con umbral, y un mis-fire solo abre una ventana de escucha (inofensivo — y Fase L igual pide confirmación). Plan B si es muy sensible: "che Astro".
- **Captura de audio sin `pyaudio`/`portaudio`:** el sidecar lee **PCM crudo por stdin** (16 kHz, mono, s16le) de un grabador ya disponible (`parec`/`sox`), y alimenta los frames a openWakeWord. Evita depender de libs nativas de audio en pip → más simple en NixOS.

## 4. Arquitectura del sidecar

`scripts/wake_oww.py` (nuevo):
- Lee PCM crudo de **stdin** en frames de 1280 muestras (80 ms @ 16 kHz), los pasa a `openwakeword.Model.predict()`.
- Si el score del modelo supera `ASTRO_OWW_THRESHOLD` (default 0.5) → imprime `DETECTED` en **stdout** (con flush). Logs a **stderr** (stdout es SOLO el contrato de eventos).
- Envs: `ASTRO_OWW_MODEL` (ruta al `.onnx`/`.tflite`; vacío = modelo pre-entrenado embebido para el spike), `ASTRO_OWW_THRESHOLD`.
- `ASTRO_WAKE_CMD` (en run.sh) = pipeline: `parec --format=s16le --rate=16000 --channels=1 | <venv>/bin/python scripts/wake_oww.py` (o `sox`/`arecord` equivalentes).

## 5. Empaquetado (NixOS)

- Venv imperativo (patrón Kokoro): `~/.venvs/astro-wake` con `pip install openwakeword`. `nix-ld` + `LD_LIBRARY_PATH=<gcc-lib>/lib` para los wheels (onnxruntime/tflite) como en Kokoro.
- `run.sh`: `ASTRO_INPUT=wake` + `ASTRO_WAKE_CMD=...`; asegurar `parec` (pipewire-pulse) o `sox` en el `nix shell` del comando.
- **Gatillo:** hacerlo declarativo en el flake (buildPythonPackage/uv2nix) = ejercicio nix posterior.

## 6. Plan incremental (de-riesga en este orden)

- **Paso 1 — spike de viabilidad** (con modelo PRE-ENTRENADO, ej. "hey jarvis"): (a) openwakeword instala/corre en NixOS; (b) detecta; (c) **el micro se comparte** — sidecar always-on + `rec`/sox al activarse (riesgo #1, PipeWire multi-cliente); (d) latencia y loop OK end-to-end.
- **Paso 2 — modelo "Astro"**: entrenar custom (Colab, voz sintética, sin grabar) → apuntar `ASTRO_OWW_MODEL` → tunear `ASTRO_OWW_THRESHOLD` para minimizar falsos disparos.

## 7. Riesgos y bordes

- **Contención de micro** (riesgo #1): validar en el Paso 1; mitigación si falla → el sidecar libera el mic durante `capture()` o se usa un loopback de PipeWire.
- **Falsos disparos**: umbral + palabra; mis-fire inofensivo. Si "Astro" es intratable → "che Astro".
- **Batería**: always-on suma CPU constante; toggle o atarlo a la specialisation de batería del nix-config = **diferido-con-gatillo**.
- **Arranque**: openWakeWord es 100% offline (sin key ni validación de red), a diferencia de Porcupine.

## 8. Definition of Done

1. Decís **"Astro"** sin tocar nada → se activa (chirp), escucha tu frase (VAD), responde con voz, y **vuelve a escuchar**. Loop estable.
2. El micro se comparte sin romper la captura.
3. Falsos disparos a un nivel tolerable (umbral tuneado).
4. `ASTRO_INPUT=wake` en run.sh; hotkey queda como fallback. Cero cambio en el Go de Fase H.
