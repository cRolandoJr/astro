# Astro — Arquitectura y roadmap

> Documento de diseño del asistente. Consolida las decisiones charladas.
> Relacionados: `~/projects/astro/guia-armado.md` (armado físico),
> `~/projects/astro/tutorial-original/lista-materiales.md` (compras),
> `~/projects/astro/tutorial-original/Parte 3/cara_expresiones.ino` (caras),
> `.../astro_firmware.ino` (firmware base v2).

## 1. Visión

Compañero de escritorio con **cara expresiva + brazos + voz propia**, que **entiende lo
que le decís** y **actúa en tu laptop** (abrir apps, mover ventanas entre monitores,
media, etc.). El **cerebro vive en la laptop**; el robot es la **cara/voz/manos**.

## 2. Principios (las reglas que mandan)

1. **La IA decide, tu código ejecuta.** El LLM solo elige *qué* acción; el daemon corre
   el comando real. El modelo nunca toca la máquina directo.
2. **Techo = lo que definís. Nada destructivo.** Solo existen las herramientas que
   programás; sin shell arbitrario. Las sensibles piden confirmación. (Tu regla.)
3. **Model-agnostic.** El LLM va detrás de una interfaz (`interpretar(texto)→acción`)
   contra una API OpenAI-compatible → cambiás de proveedor con un string.
4. **Audio local (liviano), intención en la nube (barata).** No hoggea la laptop.
5. **Declarativo/reproducible.** Todo en el flake NixOS (paquetes + systemd) — tu carril.
6. **Sin batería.** Corriente de enchufe; comunicación por WiFi.

## 3. El sistema de punta a punta

```
  vos hablás
     │  mic de la laptop
     ▼
 ┌──────────────────── LAPTOP (daemon Go, systemd) ────────────────────┐
 │  ① Whisper (whisper.cpp)   audio → texto        [local, liviano]     │
 │  ② interpretar(texto)      texto → acción       [LLM nube, barato]   │
 │       └─ tool-calling: el LLM elige UNA herramienta del menú          │
 │  ③ ejecutar acción         corre el comando real [local]             │
 │       hyprctl / playerctl / wtype / wpctl / xdg-open ...             │
 │  ④ Piper                   respuesta → voz .wav  [local, liviano]    │
 └───────────────┬───────────────────────────────┬──────────────────────┘
                 │ WiFi: comando (FACE/ARMS)      │ WiFi: audio (la voz)
                 ▼                                ▼
        ┌──────────────── Astro (ESP32) ─────────────────┐
        │  cara (OLED) + brazos (servos) reaccionan            │
        │  parlante (I2S) reproduce la voz → lip-sync          │
        │  botón = push-to-talk (avisa "escuchame")            │
        └──────────────────────────────────────────────────────┘
                 ▲ corriente: enchufe → adaptador 5V/2A
```

## 4. Componentes por capa

| Capa | Herramienta | Dónde corre | Costo |
|---|---|---|---|
| Captura de voz | micrófono de la laptop | laptop | $0 |
| STT (audio→texto) | **whisper.cpp** (modelo `small`) | laptop, local | $0 |
| Orquestador | **daemon en Go** (systemd) | laptop, local | $0 |
| Intención (texto→acción) | **LLM** (DeepSeek / Gemini Flash) vía API OpenAI-compat | nube | centavos/mes |
| Acciones | CLIs de escritorio (ver §5) | laptop, local | $0 |
| TTS (texto→voz) | **Piper** | laptop, local | $0 |
| Transporte al robot | **WiFi** (audio + comandos) | — | $0 |
| Cuerpo | ESP32 + MAX98357A + parlante + 2 servos + OLED | robot | ver lista-materiales §4 |

## 5. Las herramientas (lo que "hace posible que actúe")

Cada acción es una función del daemon que corre un comando que ya existe en tu
Hyprland/Wayland. El LLM elige de este menú; nunca inventa comandos.

| Herramienta | Comando real | Tipo |
|---|---|---|
| `abrir(app/url)` | lanzar binario · `xdg-open` · `mpv <url>` | cambia estado |
| `mover_ventana(monitor/ws)` | `hyprctl dispatch movetoworkspace / focusmonitor` | cambia estado |
| `fullscreen()` | `hyprctl dispatch fullscreen` | cambia estado |
| `media(play/pause/next)` | `playerctl` | cambia estado |
| `volumen(+/-/mute)` | `wpctl set-volume` (WirePlumber) | cambia estado |
| `brillo(+/-)` | `brightnessctl` | cambia estado |
| `escribir(texto)` / `tecla(combo)` | `wtype` · `ydotool` | cambia estado |
| `avisar(texto)` | `notify-send` (mako) | seguro |
| `captura_pantalla()` | `grim` + `slurp` | seguro |
| `que_ventanas()` / `que_suena()` | `hyprctl clients` · `playerctl metadata` | seguro (lectura) |
| `buscar_web(consulta)` | HTTP en Go | seguro |
| `expresion(cara)` / `gesto(brazos)` | comando WiFi al robot | seguro |

> **Ampliar = sumar una función acá.** Eso es "casi de todo": incremental, no mágico.
> **NO existe** una tool de borrar/matar procesos/shell crudo. Si algún día se suma algo
> sensible, entra como "requiere confirmación" (ver §6), nunca automático.

## 6. Seguridad / guardrails (cero destructivo)

- **Allowlist estricta:** solo las tools de §5. El LLM no puede ejecutar texto libre.
- **Clasificación:** `seguras` (se corren solas) vs `sensibles` (piden confirmación por
  voz/botón antes de ejecutar). Hoy no hay destructivas; si se agregan, van acá.
- **Sin shell arbitrario** por default. La "escotilla shell" queda fuera del diseño.
- **Log** de cada acción ejecutada (qué pidió, qué corrió) — auditable.
- **Privacidad:** a la nube viaja solo el *texto* del comando (no el audio). No le dictes
  secretos; si te molesta el proveedor, el modelo es swappable (§2).

## 7. Protocolo laptop ↔ robot

Mensajes cortos por WiFi (el daemon los manda; el ESP32 los interpreta):

- `FACE <expr>` — neutral, feliz, curioso, pensativo, mareado, ... (las de `drawFace`)
- `ARMS <gesto>` — reposo, saludo, baile
- `SPEAK` — el daemon manda/streamea el .wav; el ESP32 lo reproduce y anima la boca
- Del robot a la laptop: `BUTTON` — el botón = push-to-talk ("empezá a escucharme")

Las **expresiones automáticas por contexto** (reposo→parpadeo/pensativo/dormido, etc.)
las decide el firmware con `updateFace()`; el daemon solo pisa con un gesto cuando hay
un evento (te oye → CURIOSO, responde → FELIZ).

## 8. Roadmap por fases (cada una anda sola)

| Fase | Qué | Depende de |
|---|---|---|
| **0** | Robot base: cara + gestos + música, por USB (tutorial v1→v2) | nada |
| **1** | Firmware entiende comandos (`FACE/ARMS`) por USB y reacciona | 0 |
| **2** | Daemon Go v1: **push-to-talk** (botón) → Whisper → **set de tools fijas** → actúa en la laptop → Piper responde por parlantes de la laptop | 1 |
| **3** | **Voz del robot**: parlante I2S en el ESP32 + lip-sync | 2 |
| **4** | **Cortar el cable de datos**: pasar el enlace a WiFi | 3 |
| **5** | **Intención con LLM** (lenguaje natural) detrás de la interfaz + más tools | 2 |
| **6** | Extras: wake-word (sacar el botón), micro en el robot, más contexto | 5 |

Orden pragmático: **0→1→2** te da un asistente funcional atado por USB con comandos
fijos. Recién ahí decidís si priorizás **voz propia (3-4)** o **lenguaje natural (5)**.

## 9. Costos

- **Hardware:** robot base (~$43-52k) + add-on asistente (~$22-32k) — ver lista §1 y §4.
- **Operativo:** LLM ~centavos/mes (prompts cortos); Whisper + Piper = $0; sin nube para
  el audio. No hay costo recurrente relevante.

## 10. Qué NO hace / diferido (anti-scope-creep)

- **No batería** — vive enchufado.
- **No funciona sin la laptop** prendida y en la red (es un compañero de escritorio).
- **Micro en el robot** — diferido; el de la laptop alcanza (no desbloquea capacidades).
- **Sin acciones destructivas ni shell arbitrario** — por diseño.
- **Wake-word / "siempre escuchando"** — Fase 6; se arranca con push-to-talk.

---

### Nota de entorno (NixOS/Wayland)

- Wayland no permite `xdotool`; para teclado/mouse va **`ydotool`** (usa uinput, corre como
  `ydotoold` — servicio systemd + permisos, declarable en el flake).
- Todas las CLIs (`hyprctl`, `playerctl`, `wtype`, `wpctl`, `brightnessctl`, `grim`, `slurp`,
  `whisper.cpp`, `piper`) son paquetes del flake → reproducible.
- El daemon Go = otra unit systemd, mismo patrón que `scraper-pedco`.

---

## Backlog de ideas (parkeadas — YAGNI, se suman por fase)

Cosas que quiero pero se difieren a propósito. La arquitectura ya las hace fáciles de sumar
(cada una = una `Action` nueva, o el LLM de la Fase 5). Diferir no es perder.

- **Leer un archivo** ("leeme X archivo"): `Action` de lectura. Mostrar contenido crudo = simple.
  **"Decime algo de él" / resumir = necesita el LLM (Fase 5)** — ahí está la línea leer vs entender.
- **Abrir apps "como en mi bind de Hyprland"**: `Action` que corre el mismo comando del keybind.
  Idea: los binds de `hyprland.conf` son una lista lista-para-usar de "cómo me gusta abrir cada cosa";
  Astro los puede envolver directo.
- (ir agregando acá lo que surja, para no meterlo antes de tiempo)
