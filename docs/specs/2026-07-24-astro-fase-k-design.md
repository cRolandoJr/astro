# Spec — Astro · Fase K (batería de tools)

> Fecha: 2026-07-24 · Estado: borrador para revisión · Sigue a Fase J + voz DeepSeek (en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Ampliar mucho lo que Astro puede hacer, con tools **no destructivas**: media, audio, pantalla, sistema,
info y web. Es la opción **A** (allowlist amplia y segura); el ejecutor genérico guardado es la Fase L (B).

## 2. Alcance

**Incluye** las tools de §4, el mecanismo para tools **con arg** que escale sin ensuciar `Interpret`, y su
mención en el `systemPrompt`. Tests con `fakeRunner` (verifican el comando armado, sin efectos reales).

**NO incluye:** ejecutor genérico / shell arbitrario (Fase L, con guardas/confirmación); tools destructivas
(cerrar/matar ventanas, borrar); `timer <N>` (necesita "avisar más tarde" = proceso en background →
diferido con gatillo, su propio mini-diseño).

## 3. Arquitectura — registro (sin arg) + factories (con arg)

Hoy `Interpret` rutea `open`/`recordar`/`mirar` como **casos especiales sueltos** (cada uno cierra sobre
una capability: shell-safe / mem / vision). Para la batería:

- **Tools sin arg** → van al registro actual `actions map[string]*Action` (vía `buildActions`). Ya las
  rutea la rama `if a, ok := li.actions[choice.Action]; ok` — **cero cambio en `Interpret`**, solo agregar
  entradas. Su `Run(r Runner)` corre el comando (y, si corresponde, **lee la salida y la dice**).
- **Tools con arg** → un mapa nuevo `argActions map[string]*ArgTool`, donde:
  ```go
  type ArgTool struct {
      Desc  string
      Build func(arg string) *Action   // devuelve la acción ya ligada al arg
  }
  ```
  Enrutadas por **una sola** rama nueva en `Interpret` (antes del registro):
  ```go
  if t, ok := li.argActions[choice.Action]; ok {
      li.remember(text, choice.Say)
      return wrap(t.Build(choice.Arg), choice.Say), nil
  }
  ```
  Agregar una tool con arg futura = una entrada en el mapa. No se toca `Interpret`. (`open`/`recordar`/
  `mirar` quedan como están — dependen de capabilities, no encajan en `Build(arg)` puro.)

- **`systemPrompt`** lista el registro (como hoy, por `Desc`) **y** las `argActions` (`"- <name> (arg = …): <Desc>"`).
- **Seguridad:** las tools nuevas corren por `Runner` con **argv** (no shell) → sin inyección. Validación =
  **correctitud** del arg (numérico en `volumen_a`/`ir_a_workspace`; esquema http(s) en `abrir_url`); un arg
  inválido → la acción devuelve un error claro (main lo maneja), no ejecuta basura.

`buildActions` pasa a devolver también el `argActions` (o un segundo builder `buildArgActions`); `LLMConfig`/
`LLMInterpreter` ganan el campo `argActions`.

## 4. Tools

**Sin arg (registro):**
| name | qué hace | comando |
|---|---|---|
| `anterior` | track previo | `playerctl previous` |
| `que_suena` | dice qué está sonando | `playerctl metadata --format '{{artist}} - {{title}}'` → lo dice (vacío → "No hay nada sonando") |
| `silenciar_micro` | mutea/desmutea el micrófono | `wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle` |
| `bateria` | dice el % de batería | `upower -e` → device battery → `upower -i <dev>` → parsea `percentage:` |
| `fecha` | dice el día y la fecha | usa `now func` (como `hora`), formato español |
| `clima` | dice el clima | `curl -s 'wttr.in/?format=%C+%t'` → lo dice |
| `agenda` | próximo evento | `khal list now 24h --format '{start-time} {title}'` → dice el primero (vacío → "Nada en la agenda") |
| `bloquear` | bloquea la pantalla | `hyprlock` |

**Con arg (argActions):**
| name | arg | comando |
|---|---|---|
| `volumen_a` | N (0-150) | valida numérico → `wpctl set-volume @DEFAULT_AUDIO_SINK@ N%` |
| `brillo` | `subir`/`bajar`/N | `brightnessctl set +10%` / `10%-` / `N%` |
| `abrir_url` | url | valida http(s):// → `xdg-open <url>` |
| `buscar` | query | `xdg-open https://duckduckgo.com/?q=<url-encoded>` (via `net/url` QueryEscape) |
| `ir_a_workspace` | N | valida numérico → `hyprctl dispatch workspace N` |

Cara: la que corresponda (`Feliz`/`Neutral`/`Curioso`); las que "dicen" info hablan el resultado como reply.

## 5. Errores y bordes (anti-verde-falso)

- Comando que falla (ej. `playerctl` sin reproductor, `curl` sin red, `brightnessctl` sin permiso) → la
  acción devuelve error → main muestra "Ups: …", sin crash. Las que "leen para decir" (`que_suena`,
  `bateria`, `clima`, `agenda`) devuelven un texto amable si la salida está vacía.
- Arg inválido (`volumen_a` no numérico, `abrir_url` sin esquema) → error claro, no ejecuta.
- Verificado: playerctl/wpctl/brightnessctl/hyprctl/hyprlock/khal/upower presentes; wttr.in alcanzable;
  batería NO en sysfs → vía `upower`.

## 6. Decisiones (defaults, vetables)

- Solo tools no destructivas; las destructivas (cerrar ventana) quedan afuera; `timer` diferido.
- `argActions` (factories con `Desc`) para tools con arg; sin arg al registro. Una sola rama en `Interpret`.
- Sin shell (todo argv) → sin inyección; validación = correctitud del arg.

## 7. Estructura de archivos

```
~/projects/astro/
  actions.go     — (modificar) agregar las tools sin arg al registro; `ArgTool` + `buildArgActions()` con las tools con arg
  actions_test.go— (modificar/crear) tests de comandos (fakeRunner): un par representativas sin-arg + con-arg
  llm.go         — (modificar) campo `argActions`; rama de ruteo en `Interpret`; `systemPrompt` lista argActions
  llm_test.go    — (modificar) test del ruteo de una arg-tool (fake) + que aparece en el prompt
  main.go        — (modificar) wirear `buildArgActions()` en `LLMConfig`
  (interpreter.go/RuleInterpreter — opcional: no hace falta; el fallback rules queda como está)
```

## 8. Definition of Done (Fase K)

1. `go build/vet/test ./...` verdes (tools sin-arg + ruteo argActions + validación de arg; + A–J intactos).
2. E2E: "poné el volumen en 30" → 30%; "qué está sonando" → lo dice; "cuánto de batería" → el %; "qué clima
   hace" → lo dice; "abrí youtube punto com" → abre; "andá al workspace 3" → cambia. Todo por voz/DeepSeek.
3. Interfaz `Interpret` sin cambios de firma; seguridad/memoria/visión previas intactas; nada destructivo.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
