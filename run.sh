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

# Cerebro (chat) en DeepSeek pago (OpenAI-compat) — sin límite diario como el free de Gemini.
# NOTA: embeddings (recordar) y visión (mirar) reusan ASTRO_LLM_URL → con DeepSeek se degradan
# (DeepSeek no tiene esos endpoints); se restauran separando proveedores (Gemini) más adelante.
export ASTRO_LLM_URL=https://api.deepseek.com
export ASTRO_LLM_MODEL=deepseek-v4-flash
export ASTRO_EMBED_MODEL=gemini-embedding-001

# Cara (eww), STT (whisper)
export ASTRO_EWW_CONFIG="$PWD/eww"
export ASTRO_WHISPER_MODEL="$HOME/modelos/ggml-small.bin"

# Voz: Kokoro (neural, español) vía wrapper compat-piper. El FX +600 le da el timbre agudito.
# LD_LIBRARY_PATH = la libstdc++ de gcc (los wheels de pip de Kokoro la necesitan en NixOS);
# se calcula una vez acá (no por frase).
export LD_LIBRARY_PATH="$(nix build --no-link --print-out-paths nixpkgs#stdenv.cc.cc.lib)/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export ASTRO_PIPER_BIN="$PWD/scripts/kokoro-say"
export ASTRO_PIPER_VOICE="ef_dora"
export ASTRO_VOICE_FX="gain -3 pitch 600"

# Activación por voz sin Enter: apretás SUPER+SHIFT+A (bind de Hyprland) y Astro escucha una vez.
export ASTRO_INPUT=hotkey

# Astro no tiene deps de C → compilar en Go puro (resolver netgo). Evita necesitar gcc, que no está
# en el PATH mínimo del servicio systemd (interactivo andaba porque tu shell de login sí lo tiene).
export CGO_ENABLED=0

exec nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#sox nixpkgs#grim nixpkgs#go \
  --command go run .
