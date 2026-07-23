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

## 1b. Visión ampliada — hacia un asistente personal tipo "Janus"

Inspiración: el ecosistema de IA de **Nate Gentile** (Orion = dashboard/"SO" operativo; Janus =
agente con acceso por API a todas sus herramientas, sobre LLM en server privado; Fama = follow-up de
equipo; todo sobre un server local RTX 6000 / 96GB + base vectorial de su contenido). **Astro es el
germen de eso, a escala personal:** un agente de voz que maneja tu laptop, responde sobre *tus* datos,
con tu dashboard. Misma **forma** que Janus, pero **cloud-brain y de un solo usuario**.

**Mapeo honesto (qué copiamos / sustituimos / salteamos):**

| Nate | Qué es | Versión de Astro | Nota |
|---|---|---|---|
| **Janus** | agente LLM + wrappers de API a sus tools | Astro con LLM + tool-calling (Fase C) | el camino ya trazado |
| **Base semántica** | vector DB de su historial (ChromaDB) | RAG local: indexar tus notas/docs (Fase D) | factible en tu laptop |
| **Orion** | dashboard central (tareas/calendario/stats) | evolucionar tu **hub de eww** + khal | base ya existe |
| **Middleware Python** | "capa intermedia" API wrappers | tu **capa de `Action`** en Go | en marcha |
| **Fama** | follow-up de equipo | — | salteado (no hay equipo) |
| **Server RTX 6000 / LLM local** | modelos pesados on-prem | **API en la nube barata** | tu hardware no da, y no hace falta |

**Reality-checks (límites reales, resueltos):**
1. **LLM local NO** — la RX 6500M (~4GB) no corre modelos pesados. Nate usa local por privacidad +
   costo a escala de empresa; para uso personal, **API cloud (DeepSeek/Gemini) es mejor** (centavos/mes).
2. **Claude corporativo no se usa** — el daemon pega a una **API key personal** (sin dependencia del trabajo).
3. **Escala de una persona** — nada de Fama ni infra de equipo; "algo así" = agente de voz personal por fases.

**Ángulo de carrera:** Nate llama a esto **"implementador de sistemas"** (integrar IAs en empresas chicas;
requiere redes + Python + automatización de APIs). Es **tu carril DevOps casi textual** → construir
Astro-como-Janus es **práctica real** de ese perfil emergente y un portfolio concreto, no una distracción.

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

## 8. Roadmap por fases (estado real)

**Track software (en la laptop) — lo que se está construyendo:**

| Fase | Qué | Estado |
|---|---|---|
| **A** | Astro en la PC: cara en widget **eww** + comandos por **texto** → acción en la laptop | ✅ hecho |
| **B** | **Voz**: le hablás (Whisper STT, push-to-talk) y te **responde hablando** (Piper TTS); cara auto | ✅ hecho |
| **C** | **Cerebro:** LLM (Gemini free) interpreta **lenguaje natural** → elige acción; fallback a reglas | ✅ hecho |
| **D** | **"Dice más":** el LLM **redacta las respuestas habladas** (conversa, contesta) en vez de las frases fijas | 🔜 **próximo** |
| **E** | **Memoria (RAG):** ChromaDB + embeddings sobre tus notas/`~/Documentos` → responde sobre *lo tuyo* | ⏳ |
| **Orion** | **Dashboard:** evolucionar el hub de eww (tareas/calendario/estado) | 🟡 base existe |
| **+tools** | Más acciones (buscar web, leer archivos, ventanas finas) — **continuo**, cuando se necesiten | ➕ ongoing |

**Orden acordado del track software:** **D (dice más) → E (memoria) → Orion**, con *+tools* espolvoreadas
cuando pinten. Criterio: D es chica y es la que más hace *sentir* al asistente (reusa el LLM de C); E es
la más grande (setup de RAG); Orion es track visual aparte.

**Track hardware (el robot físico) — paralelo, cuando compres las piezas:**

| Fase | Qué | Estado |
|---|---|---|
| **H1** | Robot base: cara (OLED) + gestos (servos), firmware por USB | ⏳ sin comprar |
| **H2** | Voz del robot (parlante I2S) + lip-sync | ⏳ |
| **H3** | Cortar el cable de datos → **WiFi**; wake-word; micro en el robot | ⏳ |

La lógica (A→B→C→D) **corre sin el robot**; el hardware es "otra pantalla/voz" para el mismo cerebro
(mismo protocolo `FACE/ARMS/SPEAK`, §7). Orden pragmático: seguir el track software; el robot cuando
haya presupuesto y ganas.

**Próximo:** **Fase D — "dice más"**: hoy Astro entiende (C) pero responde frases fijas; en D el LLM
**redacta la respuesta** que después Piper habla → conversa y contesta. Es el complemento natural de C.

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
