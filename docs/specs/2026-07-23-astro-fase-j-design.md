# Spec — Astro · Fase J (activación por hotkey)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a la voz Kokoro (en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Activar a Astro con un **atajo de teclado global** (`SUPER+SHIFT+A`), sin apretar Enter en la terminal y
sin wake-word pago. El atajo dispara **una** escucha (la VAD que ya existe). El micro solo se abre al
apretar el bind — nada de escuchar de fondo.

## 2. Alcance

**Incluye:** un `InputSource` nuevo `HotkeyInput` que espera un disparo por un **FIFO** y ahí captura el
comando; el bind de Hyprland que escribe al FIFO; selección por `ASTRO_INPUT=hotkey`; ciclo de vida
(crear/abrir/cerrar/borrar el FIFO). Tests del lado Go con un stream de disparos fake (sin FIFO ni teclado).

**NO incluye (diferido):** generalizar `HotkeyInput`/`WakeWordInput` en un tipo común (DRY — hoy la
duplicación es ~6 líneas de guard-clause y `WakeWordInput` quedó sin uso al descartar Porcupine); múltiples
atajos; IPC por socket (el FIFO alcanza); hacer el bind declarativo vía home-manager (se agrega al
`binds.conf` de los dotfiles, que ya es la fuente declarativa symlinkeada).

## 3. Arquitectura — reusa `InputSource` y `capture()`

```
bind SUPER+SHIFT+A ──(echo go)──▶ /tmp/astro-trigger.fifo ──▶ HotkeyInput.Listen()
                                                                   │ al leer una línea:
                                                                   ▼
                                                        voice.capture()  (VAD + whisper) → texto → main → interpreter…
```

- **`HotkeyInput` implementa `InputSource`.** `Listen()` bloquea leyendo una línea del FIFO; cuando llega,
  imprime "👂 ¡te escucho!" y llama a `capture()` (reusa grabación-por-silencio + whisper). La interfaz
  `Interpret`/voz/cara/memoria/visión **no cambian** — solo se elige otra `InputSource`.
- **FIFO abierto `O_RDWR`:** el truco clave. Mantener un extremo de escritura abierto (nosotros) hace que
  el `bufio.Scanner` **nunca reciba EOF** entre disparos — así un `echo` puntual del bind aparece como una
  línea y el `Listen()` sigue vivo para el próximo, sin reabrir nada.
- **Seam para tests:** `HotkeyInput` lee de un `*bufio.Scanner` **inyectable**; en tests se le pasa un
  `strings.NewReader("go\n")` + un `VoiceInput{Runner: fakeRunner}` → `Listen()` verificable sin FIFO ni
  micrófono (mismo patrón que `WakeWordInput` de Fase H, ya auditado). El `mkfifo`+`open` reales viven en el
  constructor que usa `main`.
- **Ciclo de vida:** el constructor crea el FIFO si no existe (`syscall.Mkfifo`, ignora "ya existe") y lo
  abre `O_RDWR`; `Close()` cierra el fd y **borra** el FIFO. `main` hace `defer Close()` + cierre en el
  handler de señal (como `wake`).

## 4. El bind de Hyprland (dotfiles)

En `~/projects/dotfiles/hypr/.config/hypr/configs/binds.conf` (fuente declarativa symlinkeada; se aplica con
`hyprctl reload`):
```
bind = SUPER SHIFT, A, exec, timeout 0.3 bash -c 'echo go > /tmp/astro-trigger.fifo' || true
```
- `timeout 0.3 … || true` es **defensivo y necesario**: si Astro NO está corriendo, no hay lector del FIFO
  y el `echo` (que abre el FIFO para escritura) **bloquearía** — el `timeout` lo corta en 0.3s y no deja un
  proceso de Hyprland colgado. Con Astro corriendo (lector `O_RDWR` presente), escribe al instante.

`SUPER+SHIFT+A` verificado **libre** en los binds actuales (mnemónico "A de Astro").

## 5. Configuración

- `ASTRO_INPUT=hotkey` → activa el modo (default en `run.sh`, para que la UX sea hands-free). `stdin` (debug)
  y el default Enter (VoiceInput) siguen disponibles.
- `ASTRO_TRIGGER_FIFO` → ruta del FIFO (default `/tmp/astro-trigger.fifo`).

## 6. Errores y bordes (anti-verde-falso)

- Sin `ASTRO_TRIGGER_FIFO` (vacío) → error al construir, mensaje claro, no arranca a medias.
- No se puede crear/abrir el FIFO → error al construir → `main` reporta y no arranca el modo.
- Astro apagado + bind apretado → el `echo` no cuelga Hyprland (timeout-guard, §4).
- `Close()` nil-safe (fd/path nil en tests).
- FIFO ya existente (de una corrida previa) → `Mkfifo` "ya existe" se ignora, se reutiliza.

## 7. Decisiones (defaults, vetables)

- Hotkey por FIFO (no socket, no señal) — encaja en el seam disparo→captura, mínimo código.
- `HotkeyInput` tipo propio (no generalizar con `WakeWordInput`) — YAGNI, sin refactor de lo mergeado.
- Bind `SUPER+SHIFT+A`; `ASTRO_INPUT=hotkey` default en `run.sh`.
- Porcupine/wake-word por servicio: **descartado** (pago). `WakeWordInput` queda como código muerto opt-in.

## 8. Estructura de archivos

```
~/projects/astro/
  hotkey.go        — (crear) HotkeyInput (InputSource): FIFO O_RDWR → Listen() espera disparo → capture(); Close borra el FIFO
  hotkey_test.go   — (crear) Listen() con Scanner sobre strings.Reader + VoiceInput{fakeRunner}; sin-fifo error; Close nil-safe
  main.go          — (modificar) case "hotkey" en el switch; defer Close + cierre en señal
  run.sh           — (modificar) ASTRO_INPUT=hotkey por default
  (input.go / VoiceInput.capture() — se reusa sin cambios)
~/projects/dotfiles/hypr/.config/hypr/configs/binds.conf — (modificar, aparte) el bind SUPER+SHIFT+A
```

## 9. Definition of Done (Fase J)

1. `go build/vet/test ./...` verdes (Listen() dispara `capture` tras el evento; sin FIFO → error; Close
   nil-safe; + todo lo previo intacto).
2. E2E: con Astro corriendo (`ASTRO_INPUT=hotkey`), apretar `SUPER+SHIFT+A` activa la escucha y el comando
   se ejecuta; sin apretar, nada; con Astro apagado, el bind no cuelga Hyprland.
3. Interfaz `InputSource`/`Interpret`/voz/memoria/visión intactas; el Enter (default) sigue disponible.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib (Go).
