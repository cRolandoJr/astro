# Astro — Lista de compra ÚNICA (definitiva)

> **Un solo gasto, un solo armado.** Esta lista reemplaza a `tutorial-original/lista-materiales.md`
> (esa tenía el enfoque "v1 barata → v2 robusta"; acá vamos directo al robot final: **ESP32**
> como cerebro-cuerpo, voz propia, y ensamblado soldado en caja).
> Precios ARS orientativos (relevados 22-jul-2026 en MercadoLibre AR); confirmá al comprar.

## ⚠️ La trampa que NO podés equivocar

**Pantalla OLED con driver `SH1106`** — NO `SSD1306`. Pedí "OLED **1.3** SH1106 I2C".
Las de 0.96" baratas suelen ser SSD1306 y no andan con este código.

---

## La lista

### Cerebro + cuerpo
- [ ] **ESP32 WROOM-32 DevKit** (v1/DevKitC, 30-38 pines, USB CH340/CP2102) · ~$12.000 · [ML](https://listado.mercadolibre.com.ar/esp32-devkit)
      ⚠️ Que diga **WROOM-32** (no C3/CAM). Es el board definitivo — hace WiFi + audio + servos + pantalla.
- [ ] **2× Servo SG90 9g** (el azulito estándar) · ~$4.000 c/u · [ML](https://listado.mercadolibre.com.ar/servo-sg90-9g)
- [ ] **1× Display OLED 1.3" 128x64 I2C SH1106** ⚠️ · ~$13.000 · [ML](https://listado.mercadolibre.com.ar/oled-sh1106)
- [ ] **1× Buzzer pasivo 5V** (⚠️ **pasivo**, no activo) — bips/sonidos simples para probar desde el día 1, antes de cablear el audio. Barato · ~$1.000 · [ML](https://listado.mercadolibre.com.ar/buzzer-pasivo-5v)
- [ ] **1× Pack pulsadores** (push button, para el botón push-to-talk) · ~$1.000 · [ML](https://listado.mercadolibre.com.ar/pulsador-push-button-arduino)
- [ ] **1× Capacitor electrolítico 470–1000 µF / 16V** (aguanta el pico de los servos) · ~$500 · [ML](https://listado.mercadolibre.com.ar/capacitor-electrolitico-1000uf-16v)

### Voz del robot (solo la VOZ; la música va por la laptop)
> El parlante del robot es **para que te responda con voz** (el asistente). La **música suena en
> los parlantes de la laptop**, no en el robot. (El buzzer de arriba es solo para bips de prueba,
> no para música.)
- [ ] **1× Amplificador I2S MAX98357A** (3W) · ~$7.700 · [ML](https://listado.mercadolibre.com.ar/max98357a)
- [ ] **1× Parlante chico 4–8Ω, 2–3W** · ~$3.000 · [ML](https://listado.mercadolibre.com.ar/parlante-4-ohm-3w)

### Energía (enchufe, sin batería)
- [ ] **1× Fuente/adaptador de pared 5V / 2A** (USB) — o reciclá un cargador de celu · ~$0–8.000 · [ML](https://listado.mercadolibre.com.ar/fuente-5v-2a)

### Prototipo + ensamblado final
- [ ] **1× Protoboard 830 puntos** (para PROBAR antes de soldar — ver nota) · ~$8.600 · [ML](https://listado.mercadolibre.com.ar/protoboard-830-puntos)
- [ ] **1× Kit jumpers Dupont** (M-M y M-H) · ~$4.000 · [ML](https://listado.mercadolibre.com.ar/jumpers-dupont)
- [ ] **1× Placa perforada (perfboard) + tira de pines hembra** (el armado final soldado) · ~$3.000 · [ML](https://listado.mercadolibre.com.ar/placa-perforada-perfboard)
- [ ] **1× Soldador 30–40W + estaño** (o pedí uno prestado) · ~$11.000 · [ML](https://listado.mercadolibre.com.ar/soldador-estano-kit)

### Carcasa + anti-caída (PVC **o** impresión 3D — sin madera)
Elegí UNA. Las dos las **diseñamos** antes (la ventana de la pantalla, huecos de servos/cable):
- [ ] **Opción A — Caja de paso PVC** (ferretería, ~10×10cm) · ~$3.000
      La agujereás/limás según un plano que armamos. Barata e indestructible.
- [ ] **Opción B — Impresión 3D en PETG** (no PLA, frágil) · filamento ~$0 si ya tenés / servicio local ~$5.000–15.000 según tamaño
      Diseño el modelo (STL) a medida: ventana de pantalla, monta-servos, bumpers de TPU. La más "mascota".
- [ ] **1× Sensor de vibración SW-420** (para la cara de "golpe") · ~$1.500 · [ML](https://listado.mercadolibre.com.ar/sensor-vibracion-sw-420)

### Cuerpo físico + montaje (esto es lo que me había faltado)
- [ ] **Material para los brazos** — goma eva, plástico fino, palitos o impresión 3D/TPU. Livianos y flexibles (un brazo pesado arranca el engranaje del servo en una caída) · ~$0–3.000 (mercería/lo que tengas)
- [ ] **Pistola de silicona caliente + barritas** — para fijar el parlante, los brazos al horn del servo, y la electrónica adentro · ~$5.000–8.000 · [ML](https://listado.mercadolibre.com.ar/pistola-silicona-caliente)
- [ ] **Tornillos chicos autorroscantes + separadores/standoffs M3** — atornillar servos y perfboard a la caja (nada pegado que se suelte) · ~$2.000–3.000 · [ML](https://listado.mercadolibre.com.ar/separadores-standoff-m3)
      (Los SG90 vienen con sus horns y tornillitos; estos son para fijar a la caja.)

### Programación + cableado
- [ ] **Cable USB para el ESP32** — casi siempre **micro-USB** (algunos devkit son USB-C). Programa y alimenta durante el desarrollo. Probablemente ya tengas uno · ~$0–2.000
- [ ] **Alambre de conexión (hook-up wire)** para soldar la perfboard — o sacrificás jumpers cortándolos · ~$0–2.000 · [ML](https://listado.mercadolibre.com.ar/cable-hook-up-wire-arduino)

### Opcional — solo si querés hablarle AL robot (no hace falta)
- [ ] **1× Micrófono I2S INMP441** — el micro de la laptop alcanza; esto no desbloquea nada, solo el feeling · ~$5.000 · [ML](https://listado.mercadolibre.com.ar/microfono-inmp441)

---

### Herramientas que probablemente YA tengas (no hace falta comprar)
- [ ] **Taladro + mecha** — para los agujeros de la caja (pantalla, servos, cable, parlante)
- [ ] **Cutter / lima / (o Dremel)** — para agrandar la ventana rectangular de la pantalla
- [ ] (opcional) **Tester/multímetro** — debug de electrónica; muy útil en el primer proyecto
- [ ] (opcional) **"Tercera mano"** — sujeta las piezas mientras soldás

---

## Total aproximado

- **Todo el robot final, sin opcionales: ~$85.000–105.000 ARS.**
- Varias líneas son **cosas que quizá ya tengas** (cable USB, taladro, tester, material de brazos) → puede bajar bastante.
- Baja ~$11k si conseguís soldador prestado, y ~$6k si reciclás un cargador para los 5V.
- Con el micro del robot (opcional): +~$5.000.

## Notas importantes

1. **"Probar antes de soldar" NO es un segundo gasto ni un segundo robot.** Con las MISMAS piezas
   lo armás primero en la protoboard, confirmás que anda, y recién ahí lo pasás a la perfboard soldada
   y la caja. Es el mismo dinero y evita soldar un error. (Por eso van protoboard **y** perfboard.)
2. **El código del tutorial es para Arduino (AVR); hay que portarlo al ESP32** — cambios chicos
   (servos con `ESP32Servo`, buzzer con `ledc`; la pantalla y las caras andan igual). Lo hacemos
   cuando llegue el momento.
3. **Energía / cómo llega el 5V:** los servos + WiFi + audio piden la fuente **5V/2A** (no el USB de
   la laptop). El adaptador alimenta al ESP32 (por su USB o pin 5V) y de ahí salen los 5V para servos
   y amplificador; el capacitor va entre 5V y GND cerca de los servos. El ESP32 es lógica de 3.3V y los
   SG90 andan bien así; si **tiemblan** (raro), se arregla con un **level shifter** (~$1.500) — no lo
   compres hasta ver si hace falta.
