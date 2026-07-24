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
# Front-end de audio: capturar de la fuente PROCESADA de PipeWire (echo-cancel WebRTC: AEC +
# supresión de ruido + pasa-altos), declarada en nix-config/modules/audio.nix. Limpia el audio para
# whisper y el wake. El NIVEL se da subiendo el micro real (~2.5, wpctl) para que la voz sobreviva al NS.
export PULSE_SOURCE=astro_echo_cancel_source
# VAD de captura (corta sola al callarte). Con el micro boosteado el ambiente sube, así que:
#  - SILENCE_PCT (umbral): 6% → el ruido de fondo cuenta como "silencio" y corta (default 3% se quedaba grabando).
#  - TRAIL: 0.8s de silencio antes de cortar → responde ágil. Si te corta al hacer una pausa, subí TRAIL;
#    si sigue grabando de más por ruido, subí SILENCE_PCT.
export ASTRO_REC_SILENCE_PCT=6%
export ASTRO_REC_TRAIL_SEC=0.8

# Voz: Kokoro (neural, español) vía wrapper compat-piper. El FX +600 le da el timbre agudito.
# LD_LIBRARY_PATH = libstdc++ (gcc) + zlib: los wheels de pip de Kokoro y de openWakeWord
# (onnxruntime, wake-word) las necesitan en NixOS. Se calcula una vez acá (no por frase).
export LD_LIBRARY_PATH="$(nix build --no-link --print-out-paths nixpkgs#stdenv.cc.cc.lib)/lib:$(nix build --no-link --print-out-paths nixpkgs#zlib)/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export ASTRO_PIPER_BIN="$PWD/scripts/kokoro-say"
export ASTRO_PIPER_VOICE="ef_dora"
export ASTRO_VOICE_FX="gain -3 pitch 600"

# Activación 100% hands-free por wake-word (openWakeWord). El sidecar escucha el micro y avisa a
# Astro por stdout (DETECTED). Sidecar = grabador (rec → PCM crudo s16le) | python con openWakeWord.
# ASTRO_OWW_MODEL vacío = "hey_jarvis" (pre-entrenado, para validar); luego apuntará al modelo "Astro".
# Para volver al modo tecla: ASTRO_INPUT=hotkey (SUPER+SHIFT+A) sigue disponible.
export ASTRO_INPUT=wake
# Sin gain en el pipeline: el nivel lo da el micro real (wpctl ~2.5) y el audio ya viene limpio por
# la fuente echo-cancel (PULSE_SOURCE arriba). El wake y whisper toman la misma fuente procesada.
export ASTRO_WAKE_CMD="rec -q -c 1 -r 16000 -b 16 -e signed-integer -t raw - 2>/dev/null | $HOME/.venvs/astro-wake-oww/bin/python $PWD/scripts/wake_oww.py"
# ASTRO_OWW_MODEL: vacío = "hey_jarvis". Para el test del loop usá alexa (export antes de ./run.sh);
# luego apuntará al modelo "Astro" entrenado. ASTRO_OWW_THRESHOLD sube/baja la sensibilidad.
# export ASTRO_OWW_THRESHOLD=0.5
# Ventana de conversación: cuánto escucha tras cada respuesta SIN re-decir "alexa" antes de dormir.
# 60s = charla relajada (podés pensar entre pedidos). Si no hablás, se duerme al llegar al tope.
export ASTRO_CONVERSATION_SECS=60

# Astro no tiene deps de C → compilar en Go puro (resolver netgo). Evita necesitar gcc, que no está
# en el PATH mínimo del servicio systemd (interactivo andaba porque tu shell de login sí lo tiene).
export CGO_ENABLED=0

exec nix shell nixpkgs#whisper-cpp nixpkgs#piper-tts nixpkgs#alsa-utils nixpkgs#sox nixpkgs#grim nixpkgs#go \
  --command go run .
