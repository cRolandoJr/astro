# Spec — Astro · Fase H (activación por voz / wake-word)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase G (visión, en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Activar a Astro diciendo **"Astro"**, sin tocar el teclado. Un motor de wake-word dedicado escucha
continuo a bajo costo y, al oír la palabra, Astro captura el comando (con la VAD de Fase F), lo transcribe
y lo procesa como siempre. Reemplaza el push-to-talk (Enter) por activación por voz, **opt-in** (modo nuevo,
el Enter sigue disponible).

## 2. Alcance

**Incluye:** un `InputSource` nuevo `WakeWordInput` que espera el evento de wake y dispara la captura;
un **sidecar** externo (proceso aparte) que corre el motor de wake-word y emite un evento por stdout al oír
"Astro"; el Go se acopla **solo al contrato del sidecar** (línea `DETECTED`), no al motor; selección del modo
por env (`ASTRO_INPUT=wake`); manejo del ciclo de vida del sidecar (lanzar/cerrar). Tests del lado Go con un
stream de eventos fake (sin micrófono ni motor).

**NO incluye (diferido con gatillo):** entrenar/elegir el modelo (lo provee el usuario — ver §5); wake-word
**on-device en el ESP32** (track hardware, sin comprar); barge-in (interrumpir a Astro mientras habla);
descartar eventos de wake que lleguen DURANTE una captura (re-trigger espurio — ver §6); múltiples wake-words
o comandos por voz sin wake.

## 3. Arquitectura — reusa `InputSource` y `capture()`

```
sidecar (proceso Python, motor de wake) ──stdout "DETECTED\n"──▶ WakeWordInput.Listen()
                                                                      │ al recibir el evento:
                                                                      ▼
                                                          voice.capture()  (VAD + whisper, Fase F)
                                                                      │
                                                                      ▼  texto del comando → main → interpreter…
```

- **`WakeWordInput` implementa `InputSource`** (misma interfaz que `VoiceInput`/`StdinInput`): `Listen()`
  bloquea leyendo el stream de eventos del sidecar hasta una línea de wake; ahí llama a `capture()` (reusa
  la grabación-por-silencio + whisper que ya existen) y devuelve el texto. **La interfaz no cambia**; `main`,
  el interpreter, la voz y la cara siguen igual — solo se elige otra `InputSource`.
- **Seam para tests:** `WakeWordInput` lee los eventos de un `*bufio.Scanner` (sobre cualquier `io.Reader`)
  **inyectado**, y captura vía un `*VoiceInput` (que ya es testeable con `fakeRunner`). En tests se le pasa un
  `strings.NewReader("DETECTED\n")` + un `VoiceInput{Runner: fakeRunner}` → `Listen()` verificable sin motor
  ni micrófono. El cableado real (lanzar el sidecar, tomar su `StdoutPipe`) vive en el constructor que usa `main`.
- **Ciclo de vida:** el sidecar es **long-lived** (escucha continuo) — no encaja en `Runner.Run` (que corre
  hasta terminar). `WakeWordInput` lo administra con `os/exec` directo (`StdoutPipe` + `Scanner`) y expone
  `Close()` para matarlo. `main` hace `defer wake.Close()` y lo cierra también en el handler de señal.
- Si el sidecar muere, `Scan()` devuelve false → `Listen()` devuelve error/EOF → `main` corta el loop y
  reporta (falla **ruidosa**, no silenciosa).

## 4. Contrato del sidecar (el motor queda intercambiable)

El sidecar es un proceso que:
- Abre el micrófono y corre el modelo de wake-word en loop.
- Al detectar "Astro", imprime **`DETECTED`** + newline por **stdout** y hace **flush** (línea a línea).
- No imprime nada más por stdout (los logs van a stderr).

Astro solo depende de eso. El motor concreto es un detalle del sidecar:
- **Recomendado — Porcupine (Picovoice):** genera un modelo custom "Astro" (`.ppn`) en su consola web (gratis,
  minutos) + una access key free. `pvporcupine` (Python) corre el modelo. Camino más rápido a la palabra custom.
- **Alternativa — openWakeWord:** open-source, sin cuenta/servicio, pero la palabra custom "Astro" hay que
  **entrenarla** (pipeline de muestras sintéticas) — más laborioso. Más alineado con self-hosted/open si el
  usuario lo prefiere. Mismo contrato de stdout → swap sin tocar el Go.

El plan incluye un sidecar de referencia (Porcupine); el usuario provee el modelo + la key (§5).

## 5. Configuración y lo que provee el usuario

- `ASTRO_INPUT=wake` → activa el modo wake-word (default sigue siendo Enter/VoiceInput; `stdin` para debug).
- `ASTRO_WAKE_CMD` → comando para lanzar el sidecar (ej. `python <ruta>/wake.py`). Si falta en modo wake → error claro al arranque.
- El **modelo** de wake-word y su **key** (si el motor la pide, ej. Porcupine) los provee el usuario, vía el
  sidecar / env (ej. `PICOVOICE_ACCESS_KEY` en `secrets.env`, nunca versionado). El repo versiona el **script**
  del sidecar, no el modelo ni la key.

## 6. Errores y bordes (anti-verde-falso)

- Sidecar no configurado en modo wake (`ASTRO_WAKE_CMD` vacío) → error al construir, mensaje claro, no arranca a medias.
- Sidecar muere / no se puede lanzar → `Listen()` devuelve error → `main` corta y reporta (ruidoso).
- Evento de wake DURANTE una captura → queda buffereado en el pipe y podría disparar una captura espuria en el
  próximo `Listen()`. **Diferido** (§2): aceptable en MVP (el sidecar no se dispara con tu comando salvo que
  digas "Astro" de nuevo); si molesta, drenar eventos previos al entrar a `Listen`.
- Contención de micrófono: el sidecar y `rec` (captura) leen el mismo mic. PipeWire permite múltiples clientes
  de captura → conviven. **Se confirma en E2E** (si `rec` no engancha con el sidecar activo, es el gatillo para
  pausar el sidecar durante la captura).

## 7. Decisiones (defaults, vetables)

- Wake-word dedicado (Camino 2), no whisper-siempre (más eficiente/preciso/privado — ver brainstorm).
- Motor recomendado Porcupine (palabra custom rápida); openWakeWord como alternativa open (mismo contrato).
- Opt-in por `ASTRO_INPUT=wake`; el Enter (push-to-talk) sigue como default.
- Contrato mínimo del sidecar = línea `DETECTED` por stdout.
- Privacidad: siempre-escuchando pero **solo detecta la palabra** (no transcribe), 100% local hasta que
  disparás "Astro"; nada a la nube antes.

## 8. Estructura de archivos

```
~/projects/astro/
  wakeword.go       — (crear) WakeWordInput (InputSource): Listen() espera evento → capture(); Close() mata el sidecar
  wakeword_test.go  — (crear) Listen() con Scanner sobre strings.Reader + VoiceInput{fakeRunner}
  scripts/wake.py   — (crear) sidecar de referencia (Porcupine): mic → modelo "Astro" → "DETECTED" por stdout
  main.go           — (modificar) ASTRO_INPUT=wake → WakeWordInput; defer Close() + cierre en señal
  secrets.env.example — (modificar) documentar PICOVOICE_ACCESS_KEY (si Porcupine)
  run.sh            — (modificar, opcional) variante/nota para modo wake (python + el sidecar)
  (input.go — VoiceInput.capture() se reusa sin cambios)
```

## 9. Definition of Done (Fase H)

1. `go build/vet/test ./...` verdes (WakeWordInput.Listen() con evento fake dispara `capture` y devuelve el
   texto; sidecar muerto → error; + A/B/C/D/E/F/G intactos).
2. E2E: con el sidecar corriendo, decir **"Astro"** activa la escucha y luego el comando se ejecuta; sin
   hablar, no pasa nada; matar el sidecar corta el loop con mensaje.
3. Interfaz `Interpret`/`InputSource`/voz/memoria/visión de fases previas intactas; el Enter sigue funcionando.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib (el sidecar es proceso aparte).
