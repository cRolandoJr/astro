#!/usr/bin/env python3
"""Wrapper TTS de Kokoro para Astro, compatible con la CLI de piper.

Astro llama a la voz como a piper: texto por stdin + '--model <voz>' + '--output_file <wav>'.
Acá '--model' es el NOMBRE de voz de Kokoro (ej. ef_dora), no un archivo. Los archivos del modelo
(.onnx / .bin) vienen por env (ASTRO_KOKORO_MODEL / ASTRO_KOKORO_VOICES), default en ~/modelos/.
Sintetiza en español y escribe el wav; los efectos (pitch, etc.) los aplica Astro con sox después.
"""
import argparse
import os
import sys

import soundfile as sf
from kokoro_onnx import Kokoro


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", default="ef_dora")  # nombre de voz Kokoro (compat con piper --model)
    ap.add_argument("--output_file", required=True)
    args = ap.parse_args()

    text = sys.stdin.read().strip()
    if not text:
        return  # nada que decir

    model = os.environ.get("ASTRO_KOKORO_MODEL", os.path.expanduser("~/modelos/kokoro-v1.0.onnx"))
    voices = os.environ.get("ASTRO_KOKORO_VOICES", os.path.expanduser("~/modelos/voices-v1.0.bin"))
    kokoro = Kokoro(model, voices)
    samples, sample_rate = kokoro.create(text, voice=args.model, lang="es")
    sf.write(args.output_file, samples, sample_rate)


if __name__ == "__main__":
    main()
