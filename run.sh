#!/usr/bin/env bash
# Lanza Astro con toda su config. El secreto (la API key) vive en secrets.env, que NO se
# versiona. Copiá secrets.env.example a secrets.env y pegá tu key ahí.
set -euo pipefail
cd "$(dirname "$0")"

if [[ ! -f ./secrets.env ]]; then
  echo "Falta secrets.env — copiá secrets.env.example a secrets.env y pegá tu ASTRO_LLM_KEY." >&2
  exit 1
fi
source ./secrets.env   # define ASTRO_LLM_KEY

# LLM en la nube (Gemini free, endpoint OpenAI-compat)
export ASTRO_LLM_URL=https://generativelanguage.googleapis.com/v1beta/openai
export ASTRO_LLM_MODEL=gemini-flash-latest
export ASTRO_EMBED_MODEL=gemini-embedding-001

# Cara (eww), STT (whisper), voz (piper + efecto robot sox)
export ASTRO_EWW_CONFIG="$PWD/eww"
export ASTRO_WHISPER_MODEL="$HOME/modelos/ggml-small.bin"
export ASTRO_PIPER_VOICE="$HOME/modelos/es_MX-ald-medium.onnx"
export ASTRO_VOICE_FX="pitch 550 tempo 1.15 highpass 350 lowpass 3000 tremolo 6 25"

exec nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#sox nixpkgs#go \
  --command go run .
