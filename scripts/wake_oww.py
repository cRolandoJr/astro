#!/usr/bin/env python3
"""Sidecar de wake-word para Astro (motor: openWakeWord, FOSS, offline).

Lee PCM CRUDO por STDIN (s16le, 16 kHz, mono) — se lo alimenta un grabador
(parec/sox), así evitamos depender de pyaudio/portaudio. Por cada frame corre
openWakeWord y, si el score supera el umbral, emite 'DETECTED' por STDOUT
(el contrato que lee el daemon Go). Los logs van a STDERR.

Env:
  ASTRO_OWW_MODEL      nombre pre-entrenado ("hey_jarvis"...) o ruta a un .onnx custom.
                       Vacío = "hey_jarvis" (para el spike).
  ASTRO_OWW_THRESHOLD  umbral de detección 0..1 (default 0.5).

Uso (pipeline):
  parec --format=s16le --rate=16000 --channels=1 | python scripts/wake_oww.py
"""
import os
import sys
import time

import numpy as np
from openwakeword.model import Model

FRAME_SAMPLES = 1280          # 80 ms @ 16 kHz (tamaño que espera openWakeWord)
FRAME_BYTES = FRAME_SAMPLES * 2  # int16 = 2 bytes/muestra
COOLDOWN_SEC = 2.0            # tras un DETECTED, no re-disparar por este tiempo


def log(*a):
    print("wake:", *a, file=sys.stderr, flush=True)


def main():
    model_spec = os.environ.get("ASTRO_OWW_MODEL", "").strip()
    threshold = float(os.environ.get("ASTRO_OWW_THRESHOLD", "0.5"))

    # Ruta a un .onnx custom (ej. "Astro") → cargamos ese solo y miramos el max score.
    # Nombre pre-entrenado ("hey_jarvis") o vacío → cargamos los bundled y filtramos por nombre.
    is_path = model_spec and ("/" in model_spec or model_spec.endswith((".onnx", ".tflite")))
    if is_path:
        model = Model(wakeword_model_paths=[model_spec])
        target = None
    else:
        model = Model()  # todos los pre-entrenados
        target = model_spec or "hey_jarvis"
    log(f"escuchando (modelo={model_spec or 'hey_jarvis'}, umbral={threshold})…")

    debug = os.environ.get("ASTRO_OWW_DEBUG", "") != ""
    last_fire = 0.0
    win_max = 0.0
    win_n = 0
    stdin = sys.stdin.buffer
    while True:
        chunk = stdin.read(FRAME_BYTES)
        if len(chunk) < FRAME_BYTES:  # EOF: el grabador se cerró
            log("stdin cerrado, salgo")
            break
        frame = np.frombuffer(chunk, dtype=np.int16)
        scores = model.predict(frame)  # {nombre_modelo: score}
        score = scores.get(target, 0.0) if target else (max(scores.values()) if scores else 0.0)
        if debug:  # heartbeat ~1s: muestra el pico de score → ¿llega audio? ¿cuánto puntúa?
            win_max = max(win_max, score)
            win_n += 1
            if win_n >= 12:
                log(f"score_max≈{win_max:.2f} (rms={int(np.sqrt(np.mean(frame.astype(np.float32) ** 2)))})")
                win_max, win_n = 0.0, 0
        if score >= threshold and (time.time() - last_fire) > COOLDOWN_SEC:
            last_fire = time.time()
            print("DETECTED", flush=True)  # STDOUT: el evento que lee Astro
            log(f"detectado (score={score:.2f})")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        pass
