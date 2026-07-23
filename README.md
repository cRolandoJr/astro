# Astro 🤖

Robot compañero de escritorio: una **cara expresiva** que entiende lo que le decís,
te **responde con voz** y **actúa en la laptop** (abrir apps, media, ventanas…).
El cerebro vive en la PC (daemon en Go); el cuerpo es un ESP32 con pantalla, brazos y parlante.

> Proyecto personal / de aprendizaje (Go, embebido, NixOS/Hyprland). En diseño — arranca por software.

## Estado

- ✅ Diseño y arquitectura definidos
- ✅ Cara y 13 expresiones (dibujadas por primitivas)
- 🔜 **Fase A** (en PC, sin hardware): cara en pantalla + escribís un comando → hace la acción + responde
- ⏳ Voz (Whisper + Piper) · LLM · hardware ESP32

## Estructura

```
arquitectura.md        — arquitectura del asistente + roadmap por fases
guia-armado.md         — cómo armar el robot físico paso a paso
compra-final.md        — lista de compra única (componentes + precios)
mockup.html            — mockup visual (cara, expresiones, carcasas)
docs/specs/            — specs por fase (empezando por Fase A)
tutorial-original/     — tutorial base del que partió el proyecto + código nuestro (caras, firmware)
```

## Cómo funciona (resumen)

```
tu voz → Whisper (texto) → intención (reglas / LLM) → acción en la laptop
                                                     + voz de respuesta (Piper)
                                                     + cara/gesto (pantalla hoy, ESP32 después)
```

La IA **decide**; el código **ejecuta**. Las capacidades están acotadas a acciones curadas —
nada destructivo, sin shell arbitrario.
