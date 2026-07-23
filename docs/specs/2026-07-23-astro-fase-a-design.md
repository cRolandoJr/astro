# Spec — Astro · Fase A ("existe en la PC", text-first)

> Fecha: 2026-07-23 · Estado: aprobado (diseño) · Parte del roadmap en `../../arquitectura.md`
> Convenciones: identificadores en **inglés**, comentarios/explicaciones en **español**.
> Modo de trabajo: lectura guiada (explicar patrón + herramienta antes de escribir, incrementos chicos).

## 1. Objetivo

Que Astro **exista en la laptop sin ningún hardware**: corrés `astro` en una terminal, escribís un
comando en lenguaje simple, y Astro (a) **hace la acción real** en el escritorio, (b) **cambia de cara**
en un widget de eww, y (c) **te responde por texto**.

## 2. Alcance

**Incluye:** entrada por stdin (texto), interpretación por reglas, un set curado de acciones de
escritorio, cara en eww, respuesta por texto, tests con Runner falso.

**NO incluye (diferido a fases siguientes):** voz de entrada (Whisper, Fase B), voz de salida
(Piper, Fase B), interpretación por LLM (Fase 5), leer/resumir archivos (backlog), systemd/flake,
hardware ESP32. Corre con `go run`.

## 3. Arquitectura

Cinco unidades chicas, cada una con un propósito y comunicadas por **interfaces** (para poder
cambiar implementaciones sin tocar el resto):

| Unidad | Rol | Interfaz | Impl. Fase A | Impl. futura |
|---|---|---|---|---|
| `Interpreter` | texto → acción | `Interpret(text) (Action, error)` | `RuleInterpreter` (reglas) | LLM (Fase 5) |
| `Action` | una capacidad de Astro | `Name()`, `Run(Runner) (reply, error)`, `Face()` | acciones curadas | +acciones |
| `Display` | mostrar la cara | `Show(Expression) error` | `EwwFace` | `ESP32Display` (hardware) |
| `Runner` | ejecutar comandos del SO | `Run(name, args...) (out, error)` | `ExecRunner` | — |
| `main` | wiring + loop | — | inyección de dependencias | — |

Patrones que se aprenden: **dependency inversion** (Interpreter/Display swappables), **command
pattern/registry** (Action), **dependency injection para testeo** (Runner), **una interfaz con dos
implementaciones** (Display: eww hoy, ESP32 mañana — el mismo contrato que usará el robot físico).

## 4. Flujo de datos

```
texto (stdin)
  → Interpreter.Interpret(text) → Action        (o error "no entendí")
      → Action.Run(runner)                        efecto: corre hyprctl/playerctl/... vía Runner
      → Display.Show(action.Face())               la cara reacciona (eww)
      → imprime reply                             te contesta por texto
```

El cerebro no sabe de dónde viene el texto (mañana: tu voz) ni cómo se pinta la cara
(mañana: ESP32). Ese desacople es el objetivo de aprendizaje.

## 5. Acciones iniciales (curadas, nada destructivo)

| Comando (ejemplos que lo disparan) | Efecto | Cara |
|---|---|---|
| `hola` | saluda por texto | feliz |
| `qué hora es` / `hora` | responde la hora | neutral |
| `abrí <app>` (firefox, kitty, ...) | lanza la app | curioso |
| `pausá` / `seguí` / `siguiente` | control de media (`playerctl`) | feliz |
| `subí` / `bajá` / `mute` volumen | `wpctl set-volume` | neutral |
| `dormí` / `despertá` | cambia la cara (dormido / neutral) | dormido/neutral |

**Regla dura:** solo existen estas acciones. No hay borrar/matar procesos ni ejecución de shell
arbitrario. Ampliar = agregar una `Action` (no toca el resto).

## 6. Manejo de errores

- **No se entiende el comando** → cara `pensativo` + reply "no te entendí; probá con: …".
- **La acción falla** (app inexistente, comando devuelve error) → cara `neutral` (o `triste`) +
  reply con el motivo real (`stderr`/error), sin romper el loop.
- El loop nunca corta por un comando fallido; sigue esperando el próximo.

## 7. Testing

- `RuleInterpreter`: entrada de texto → se verifica la `Action` resultante (incluye el caso "no entendí").
- `Action.Run`: se le inyecta un **Runner falso** que registra qué comando *se habría corrido*, y se
  verifica sin ejecutar nada real (no se abre firefox en el test). Patrón idiomático de Go: interfaz +
  fake para aislar efectos.
- `EwwFace`: se testea contra un Runner falso (verifica el `eww update` correcto).

## 8. Estructura de archivos

```
~/projects/astro/astro/
  main.go            — wiring de dependencias + loop de lectura
  interpreter.go     — Interpreter + RuleInterpreter
  action.go          — tipo Action + registro
  actions.go         — acciones concretas (greet, time, open, media, volume, sleep)
  display.go         — Display + EwwFace
  runner.go          — Runner + ExecRunner (+ fake en _test.go)
  expression.go      — tipo Expression (las 13 caras)
  faces/             — PNGs de las caras (generados con el script existente)
  eww/               — widget de Astro (ventana que muestra faces/<expr>.png)
```

## 9. Decisiones tomadas

- **Lenguaje:** Go (carril portfolio; audio futuro se orquesta con binarios whisper.cpp/piper por `exec`).
- **Entrada Fase A:** stdin en terminal (swappable; mañana lo reemplaza la voz).
- **Interpretación Fase A:** por reglas (sin API/LLM, cero costo y cero dependencia externa).
- **Cara:** widget de eww (stack actual del user); el daemon le dice qué imagen mostrar.
- **Nombres:** descriptivos, largo escala con alcance (cortos solo en scope mínimo).

## 10. Criterios de éxito (Definition of Done de la Fase A)

1. `go run` levanta Astro; aparece el widget con la cara `neutral`.
2. Escribir cada comando de §5 produce el efecto real + el cambio de cara + el reply correcto.
3. Un comando desconocido da cara `pensativo` + mensaje de ayuda, sin cortar el loop.
4. `go test ./...` pasa (interpreter + al menos una acción con Runner falso).
5. Todo el código respeta las convenciones de §9 y tiene comentarios del "por qué" en español.
