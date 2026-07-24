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

# Cerebro (chat) vía OpenRouter (OpenAI-compat): trae DeepSeek v4-flash y acepta tarjeta
# internacional (el pago directo a DeepSeek/China no cursaba). Bonus: OpenRouter TAMBIÉN expone
# /embeddings (gemini-embedding-001, 3072 dims) → `recordar` anda por la misma URL/key, sin
# proveedor aparte. `mirar` (visión) reusa la misma URL; depende de que el modelo acepte imágenes.
export ASTRO_LLM_URL=https://openrouter.ai/api/v1
export ASTRO_LLM_MODEL=deepseek/deepseek-v4-flash
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
