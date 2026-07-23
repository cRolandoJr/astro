# Spec — Astro · Fase G (visión de pantalla)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase F (conversación natural, en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Que Astro **vea la pantalla cuando se lo pedís** y responda sobre lo que hay ahí: *"mirá el monitor de la
derecha y decime qué error tira"* → captura ese monitor → lo manda a un LLM multimodal → responde por voz.
**Solo cuando se lo pedís**, nunca de fondo. Podés elegir **qué monitor** mirar.

## 2. Alcance

**Incluye:** un action `mirar` (explícito, ruteado por el LLM); captura de un monitor con `grim -o <output>`;
llamada de **visión** a Gemini flash (multimodal, endpoint OpenAI-compat, imagen en base64); selección de
monitor por voz (el LLM traduce "el de la derecha" → nombre de salida, usando la lista de monitores que Astro
incluye en el prompt). Tests con `visionFunc` fake (sin red) + `httptest` para el cliente.

**NO incluye (diferido con gatillo):** visión conversacional multi-turno (la imagen NO entra al historial;
cada `mirar` es one-off); refresco de monitores por hotplug en vivo (se leen al arranque; reconectar =
reiniciar Astro); cara "pensando" DURANTE la llamada (hoy `main` muestra la cara al responder, no antes de
ejecutar una acción lenta — requeriría cambiar el flujo de `main`); recorte de región arbitraria; downscale
de la imagen si el payload es muy grande.

## 3. Privacidad (explícito)

Cuando pedís `mirar`, **el monitor elegido se manda a la nube de Gemini** (un tercero) en ese momento. Es
solo-cuando-lo-pedís (nunca automático) y **solo ese monitor** (no toda la pantalla si elegís uno). Inherente
a usar un modelo de visión en la nube; el usuario lo decidió a conciencia. Sin cambios en los guardrails de
ejecución (la visión solo lee y habla, no ejecuta nada).

## 4. Arquitectura — la interfaz `Interpreter` NO cambia

`mirar` se rutea en `Interpret` como `open`/`recordar`. Su acción hace: captura (`grim`) → llamada de visión
→ devuelve el texto como respuesta hablada. Es una llamada **distinta** a la de intención (esa sigue
devolviendo `{action,arg,say}` JSON; la visión devuelve **texto libre**).

```
Turno con "mirá el de la derecha, ¿qué dice?":
  intención (chat normal, con la lista de monitores en el prompt) → {"action":"mirar","arg":"HDMI-A-1"}
  ruteo: action=="mirar" && vision!=nil → lookAction(vision, output="HDMI-A-1", question=<frase del usuario>)
  lookAction.Run(r):
      grim -o HDMI-A-1 /tmp/astro-screen.png   (arg vacío → grim sin -o = toda la pantalla, fallback)
      vision(question, "/tmp/astro-screen.png") → texto → se habla
```

### Componentes

**`vision.go` (crear)**
- `type visionFunc func(question, imagePath string) (string, error)` — seam (fake en tests).
- `httpVision(baseURL, apiKey, model string) visionFunc`: lee el PNG, lo codifica base64 (`data:image/png;base64,…`),
  POST a `/chat/completions` con `messages` = un mensaje de sistema (breve, español, hablable) + un mensaje
  `user` cuyo `content` es un array `[{type:"text", text: question}, {type:"image_url", image_url:{url: dataURI}}]`.
  Parsea `choices[0].message.content`. Timeout 40s (la visión es más lenta + sube imagen). Error con `%w` si
  HTTP != 2xx o body inesperado.

**`llm.go` (modificar)**
- `LLMConfig`/`LLMInterpreter` ganan `Vision visionFunc` (nil = sin visión) y `Monitors string` (la lista
  formateada para el prompt; vacía si no se pudo leer).
- `systemPrompt`: si `vision != nil`, menciona en el menú `mirar (arg = nombre del monitor a mirar; vacío = el
  enfocado)` e incluye la sección de monitores (`Monitors`), instruyendo: "elegí el `name` del monitor pedido
  (x menor = más a la izquierda); si no aclara, usá el enfocado".
- `Interpret`: rutea `if choice.Action == "mirar" && li.vision != nil { return lookAction(li.vision, choice.Arg, text), nil }`
  (antes de `open`). Si `vision == nil` → cae a fallback. **No** se registra en el historial (la respuesta real
  la produce `Run` después, y la imagen no se guarda — `mirar` es stateless, como los rechazos post-parseo).
- `lookAction(vision visionFunc, output, question string) *Action`: `&Action{Name:"mirar", Face: Neutral, Run: …}`;
  el Run corre `grim` (`-o <output>` si hay output, sin `-o` si vacío) y devuelve `vision(question, path)`.

**`main.go` (modificar)**
- Al arrancar (rama LLM), consulta `hyprctl monitors -j` vía `Runner`, parsea `name/description/x/focused`,
  arma `monitorsPrompt` (ej. `"eDP-1 — AU Optronics, x=0 (enfocado); HDMI-A-1 — …, x=1920"`). Si falla (no
  Hyprland / no hyprctl) → `""` (visión igual anda, sin resolución de nombres → captura toda la pantalla).
- Construye `httpVision(llmURL, llmKey, envOr("ASTRO_VISION_MODEL", <modelo de chat>))` y lo pasa en `LLMConfig.Vision`
  junto con `Monitors: monitorsPrompt`.

## 5. Modelo y envs

- Visión: reusa `ASTRO_LLM_URL`/`ASTRO_LLM_KEY`; modelo `ASTRO_VISION_MODEL` (default = el de chat, `gemini-flash-latest`,
  que es multimodal). Confirmamos en E2E que la key acepta imágenes (mismo patrón de volatilidad de modelos de fases previas).
- `grim` se agrega al `nix shell` de `run.sh` (ya está en el perfil, pero lo declaramos).

## 6. Errores y bordes (anti-verde-falso)

- `grim` falla (no Wayland / output inexistente) → `lookAction.Run` devuelve error → `main` lo muestra ("Ups: …"),
  sin crash ni turno colgado.
- `hyprctl` falla al arranque → `monitorsPrompt=""` → el prompt omite monitores; `mirar` sin arg captura todo.
- Visión falla (HTTP/red) → error → se habla/muestra el error; el resto del daemon sigue.
- `vision == nil` (brain=rules, o sin querer visión) → `mirar` cae a fallback (no promete lo que no puede).

## 7. Decisiones (defaults, vetables)

- Trigger explícito `mirar`; selección de monitor por el LLM desde la lista; default = enfocado.
- Captura por monitor (`grim -o`), no región; PNG sin downscale (si el payload molesta → gatillo).
- `mirar` es stateless (no entra al historial); cara `Neutral` al responder (thinking-face diferido).
- Reusa `ASTRO_LLM_*`; nuevo `ASTRO_VISION_MODEL`.

## 8. Estructura de archivos

```
~/projects/astro/
  vision.go       — (crear) visionFunc, httpVision (imagen base64 → /chat/completions multimodal)
  vision_test.go  — (crear) httpVision con httptest (arma content text+image_url, parsea la respuesta)
  llm.go          — (modificar) Vision+Monitors en config/struct; ruteo de 'mirar'; lookAction; systemPrompt
  llm_test.go     — (modificar) visionFunc fake; test de ruteo de 'mirar' (grim + vision, sin registrar en historial)
  main.go         — (modificar) query hyprctl monitors → monitorsPrompt; httpVision; wiring en LLMConfig
  run.sh          — (modificar) agregar nixpkgs#grim al nix shell
  (input.go, memory.go, embed.go, voice.go, etc. — sin cambios)
```

## 9. Definition of Done (Fase G)

1. `go build/vet/test ./...` verdes (httpVision con httptest, ruteo de `mirar` con `grim`+`visionFunc` fake,
   nil-safe sin visión; + A/B/C/D/E/F intactos).
2. E2E: *"mirá la pantalla y decime qué ves"* → captura + describe; *"mirá el de la derecha"* (multi-monitor) →
   captura ese; sin aclarar → el enfocado.
3. Interfaz `Interpret` / voz / memoria / multi-turno / seguridad de fases previas intactas.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
