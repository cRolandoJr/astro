# Lista de materiales — Astro (Argentina / MercadoLibre)

> Precios relevados el **2026-07-22** en MercadoLibre AR y tiendas locales
> (HobbyTronica, PatagoniaTec, Nubbeo). Son **orientativos**: en pesos varían
> semana a semana, así que tomalos como referencia y confirmá al comprar.
> Los links son búsquedas, no un vendedor puntual — elegí por reputación (MercadoLíder)
> y envío, no solo por precio.

## ⚠️ Las 2 trampas que NO podés equivocar

1. **Pantalla OLED con driver `SH1106`** — NO `SSD1306`. El código es para SH1106.
   Las de 0.96" baratas casi siempre son SSD1306 y **no andan** sin reescribir código.
   Pedí explícitamente "OLED **1.3** SH1106 I2C".
2. **Buzzer `PASIVO`** — NO activo. El activo suena un solo tono fijo; solo el
   pasivo reproduce la melodía.

---

## 1. Esenciales — para que el tutorial funcione (v1 + botón)

| # | Cant. | Componente | Detalle crítico | Precio aprox. c/u (ARS) | Buscar |
|---|---|---|---|---|---|
| 1 | 1 | Arduino Uno R3 (clon CH340) | El clon anda igual; instala driver CH340 (en Linux ya viene) | ~$9.650 | [ML](https://listado.mercadolibre.com.ar/arduino-uno-r3-ch340-atmega328p) |
| 2 | 2 | Servo SG90 9g | El azulito estándar (no hace falta "360/continuo") | ~$3.500–5.000 | [ML](https://listado.mercadolibre.com.ar/servo-sg90-9g) |
| 3 | 1 | Display OLED **1.3" 128x64 I2C SH1106** | ⚠️ SH1106, no SSD1306 | ~$12.000–15.000 | [ML](https://listado.mercadolibre.com.ar/oled-sh1106) |
| 4 | 1 | Buzzer **pasivo** 5V | ⚠️ pasivo | ~$700–1.200 | [ML](https://listado.mercadolibre.com.ar/buzzer-pasivo-5v) |
| 5 | 1 | Protoboard 830 puntos (MB-102) | La "grande"; sobra para esto | ~$8.600 | [ML](https://listado.mercadolibre.com.ar/protoboard-830-puntos) |
| 6 | 1 | Kit jumpers Dupont (M-M y M-H, ~40 c/u) | Necesitás macho-macho **y** macho-hembra | ~$4.000 | [ML](https://listado.mercadolibre.com.ar/jumpers-dupont) |
| 7 | 1 | Pack pulsadores (push button 4 patas) | Para el botón de la v2 (pin 2 → GND) | ~$1.000 | [ML](https://listado.mercadolibre.com.ar/pulsador-push-button-arduino) |

**Subtotal esenciales: ~$43.000–52.000 ARS** (los dos ítems que más pesan y más
varían son la pantalla y los servos).

> 💡 **Cable USB**: el Uno usa USB tipo B (el "de impresora"). Suele venir con la
> placa — confirmalo en la publicación; si no, sumá uno (~$2.000).

---

## 2. Recomendado para que no se resetee solo (barato, sumalo ya)

| # | Cant. | Componente | Para qué | Precio aprox. (ARS) | Buscar |
|---|---|---|---|---|---|
| 8 | 1 | Capacitor electrolítico 470–1000 µF / 16V | Absorbe el pico de corriente de los servos (evita resets) | ~$300–600 | [ML](https://listado.mercadolibre.com.ar/capacitor-electrolitico-1000uf-16v) |

---

## 3. Versión robusta "mil caídas" — comprar DESPUÉS (cuando llegues a la v2 física)

No lo compres ahora; primero armá todo en protoboard y confirmá que funciona.

| Componente | Para qué | Precio aprox. (ARS) |
|---|---|---|
| Arduino **Nano** (clon CH340) | Reemplaza al Uno en la versión final soldada (mismo código, chiquito) | ~$7.000–9.000 |
| Placa perforada (perfboard) + tira de pines hembra | Soldar en vez de protoboard (la falla #1 en caídas son jumpers que se salen) | ~$2.000–4.000 |
| Soldador 30–40W + estaño | Para soldar la perfboard | ~$8.000–15.000 |
| Caja de paso PVC (ferretería, ~10×10cm) | Carcasa casi indestructible (ver guía §6) | ~$2.000–4.000 |
| Sensor de vibración **SW-420** | El toque jocoso: grita y pone cara ☠️ al caerse | ~$1.500 |
| (Opcional) Servos **MG90S** (engranaje metálico) | Upgrade si un SG90 muere por un brazo pesado | ~$5.000 c/u |

---

## 4. Camino asistente de voz (v3) — que entienda, controle la laptop y hable

El cerebro vive en **tu laptop** (un daemon en Go: Whisper para entender + reglas/Claude
para decidir + acciones sobre la laptop + Piper para la voz). El robot pasa a **ESP32**
para tener **WiFi** (libera el puerto USB) y **parlante propio** (la voz sale de él).
Corriente por **enchufe**, sin batería.

| Cant. | Componente | Para qué | Precio aprox. (ARS) |
|---|---|---|---|
| 1 | **ESP32** devkit (WiFi/BT) | Reemplaza al Uno/Nano: WiFi + audio I2S + servos | ~$12.000 |
| 1 | Amplificador **I2S MAX98357A** (3W) | Convierte el audio digital del ESP32 en sonido | ~$7.700 |
| 1 | Parlante chico 4–8Ω, 2–3W | Por acá sale la voz | ~$2.000–4.000 |
| 1 | Adaptador de pared **5V / 2A** (USB) | Corriente por enchufe (o reciclá un cargador de celu) | ~$0–8.000 |
| opc. | Micrófono **I2S INMP441** | Solo si querés que escuche *desde el robot* (ver nota ↓) | ~$4.000–6.000 |

**Add-on asistente (sobre el robot base): ~$22.000–32.000 ARS** (~$26–38k con micro propio).

> ⚠️ **Energía:** los servos + WiFi + audio piden un adaptador **5V/2A** decente (no el
> USB de la laptop). Sumá el capacitor del punto 2. El ESP32 maneja lógica de 3.3V; los
> SG90 andan igual con esa señal.
>
> 🎙️ **¿Y si le integro el micro al robot?** Tu pregunta. Respuesta honesta: **funciona
> igual — el micro en el robot NO cambia lo que puede hacer, solo desde dónde escucha.**
> El cerebro (entender y controlar la laptop) vive en la laptop pase lo que pase.
> - **Micro de la laptop** (recomendado para empezar): gratis, mejor calidad, cero código
>   extra. Le hablás de frente a la laptop.
> - **Micro en el robot** (INMP441): más inmersivo (le hablás *a él*), pero el ESP32 tiene
>   que capturar ese audio y mandarlo por WiFi a la laptop = más laburo (audio full-duplex:
>   entra el micro y sale el parlante a la vez) y un poco más de plata. **No desbloquea
>   nada nuevo.** Sumalo después si querés el feeling de hablarle al robot.

---

## 5. NO comprar (aunque aparezcan en el esquemático original)

El archivo `Circuito y placa/imagen circuito.png` incluye piezas que **el código
del tutorial no usa**. Ahorrátelas:

- ❌ Receptor infrarrojo (TSOP38 / TSSP58038)
- ❌ Módulo Bluetooth + resistencias del divisor (10k / 4.7k)
- ❌ LED + resistencia 220Ω

---

## 6. Atajo posible: "Kit iniciación Arduino"

Hay kits que traen **Uno + protoboard + jumpers + buzzer + LEDs + resistencias +
a veces servos** por menos que comprándolos sueltos (~$30.000–45.000 según el kit).
**Pero** casi ninguno incluye la OLED 1.3" SH1106 (suelen traer SSD1306 0.96" o
ninguna), así que la pantalla la comprás aparte igual. Si te sirve tener repuestos
para futuros proyectos, el kit conviene; si querés lo mínimo, la lista de arriba.
Buscar: [kit inicio Arduino](https://listado.mercadolibre.com.ar/kit-inicio-arduino-uno).

---

*Fuentes de precio consultadas (2026-07-22): MercadoLibre Argentina, HobbyTronica
(servo $8.374 modelo premium, protoboard $8.634), PatagoniaTec (SG92R $5.148,
MG90S $3.696), Nubbeo (OLED 1.3 SH1106 $14.299; MAX98357A $7.699), ESP32 devkit
$12.090. Verificá siempre en el momento.*
