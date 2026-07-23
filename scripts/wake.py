#!/usr/bin/env python3
"""Sidecar de wake-word para Astro (motor: Porcupine).
Escucha el micrófono y emite 'DETECTED' por STDOUT al oír la palabra "Astro".
Los logs van a STDERR (stdout es SOLO el contrato de eventos que lee el daemon Go).

Requisitos (los provee el usuario, no el repo):
  - pip install pvporcupine pvrecorder   (o vía nix)
  - PICOVOICE_ACCESS_KEY  (key free de Picovoice)
  - ASTRO_WAKE_PPN=/ruta/astro.ppn  (modelo custom "Astro" generado en la consola de Picovoice)
Uso: PICOVOICE_ACCESS_KEY=... ASTRO_WAKE_PPN=... python scripts/wake.py
"""
import os
import sys

import pvporcupine
from pvrecorder import PvRecorder


def main():
    key = os.environ["PICOVOICE_ACCESS_KEY"]
    ppn = os.environ["ASTRO_WAKE_PPN"]
    porcupine = pvporcupine.create(access_key=key, keyword_paths=[ppn])
    recorder = PvRecorder(frame_length=porcupine.frame_length)
    recorder.start()
    print("wake: escuchando 'Astro'…", file=sys.stderr, flush=True)
    try:
        while True:
            if porcupine.process(recorder.read()) >= 0:
                print("DETECTED", flush=True)  # STDOUT: el evento que lee Astro
    except KeyboardInterrupt:
        pass
    finally:
        recorder.stop()
        recorder.delete()
        porcupine.delete()


if __name__ == "__main__":
    main()
