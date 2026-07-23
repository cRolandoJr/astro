# Guía Astro — de cero electrónica a robot de escritorio robusto

> Basada en el tutorial de `~/projects/astro/tutorial-original/` (3 partes + esquemático
> KiCad). Escrita para alguien que programa bien (Go/Linux) pero nunca tocó electrónica.

## 0. Qué es y qué hace Astro (resumen)

Un robot de escritorio mínimo: una carita en pantalla OLED, dos bracitos con
servomotores, y un buzzer que toca música. Al encenderlo:

1. Muestra la carita (un bitmap) en la pantalla.
2. Toca el riff de **Megalovania** (Undertale) en el buzzer.
3. Agita los dos brazos al ritmo de cada nota.

Eso es todo — es un juguete/mascota de escritorio. El tutorial va en 3 partes
incrementales: **Parte 1** = solo buzzer (suena el ringtone de Nokia como prueba),
**Parte 2** = buzzer + servos bailando, **Parte 3** = todo + pantalla con carita.

**Ojo con el esquemático**: `Circuito y placa/imagen circuito.png` incluye cosas que
el código del tutorial **no usa**: un receptor infrarrojo (TSOP38), un módulo
Bluetooth (divisor 10k/4.7k en RXD) y un LED con resistencia de 220Ω. Son extras del
build original del autor. **No los compres** para este tutorial — código pasivo no
comprado es plata ahorrada.

### ¿Podés hacerlo?

Sí, sin duda. La parte "difícil" de este proyecto es el software, y eso ya lo tenés
de sobra. La electrónica es nivel entrada absoluta: no hay que soldar nada para la
v1 (todo va pinchado en una protoboard), no hay tensiones peligrosas (todo es 5V por
USB), y no podés romper nada caro (el componente más caro sale lo que un delivery).
Lo único genuinamente nuevo para vos: qué es GND/VCC, PWM e I2C — va abajo.

---

## 1. Lista de compras (MercadoLibre / electrónica local)

| Componente | Detalle crítico | Para qué |
|---|---|---|
| Arduino Uno (clon con CH340 sirve) | — | El cerebro |
| 2× servo **SG90** (9g) | Los chiquitos azules | Los brazos |
| Pantalla OLED 1.3" **I2C con driver SH1106** | ⚠️ El código es para SH1106. Las de 0.96" suelen ser SSD1306 y NO andan con este código sin cambios. Pedí explícitamente "OLED 1.3 pulgadas SH1106 I2C" | La cara |
| Buzzer **pasivo** | ⚠️ Pasivo, no activo. El activo suena a un solo tono fijo; el pasivo reproduce las notas | La música |
| Protoboard (media, 400 puntos) | — | Conectar sin soldar |
| Jumpers macho-macho y macho-hembra | ~20 de cada | Cables |
| Cable USB-B (el "de impresora") | Suele venir con el Uno | Programar y alimentar |

**Para la v2 robusta (podés comprarlo después):** placa perforada (perfboard),
tira de pines hembra, estaño + soldador básico de 30-40W, capacitor electrolítico
de 470–1000 µF / 16V, y una cajita (ver §6).

**Trampa del autor sobre la pantalla** (está en su `IMPORTANTE.docx`): además del
driver, fijate el **orden de los pines** serigrafiado en la placa de la pantalla.
Él usa una que tiene `VCC` antes que `GND`. No importa cuál te toque —
**conectás según lo que dice la serigrafía de TU pantalla**, no según fotos del
tutorial. Ese es el único cuidado real.

---

## 2. Conceptos de electrónica — lo mínimo que necesitás

- **VCC y GND**: el + y el − de toda la vida. VCC acá es 5V (sale del USB). **Regla
  de oro: todos los GND del circuito van conectados entre sí** ("GND común"). El 90%
  de los "no funciona" de principiante es un GND sin conectar.
- **Pin digital**: un pin que el Arduino pone en 0V o 5V por software. El buzzer va
  en uno de estos (pin 3).
- **PWM (lo que mueve los servos)**: el Arduino no puede sacar "2.5V", pero puede
  prender y apagar un pin muy rápido. Un servo lee el *ancho* de ese pulso y lo
  traduce a un ángulo (0–180°). `servo.write(90)` = "ponete a 90°". Es conceptualmente
  igual a un contrato de API: vos mandás el ángulo, el servo resuelve el cómo.
- **I2C (lo que habla con la pantalla)**: un bus serie de 2 cables — SDA (datos) y
  SCL (clock) — donde cada dispositivo tiene una dirección (la pantalla es la
  `0x3C` que ves en el código). En el Uno, SDA = pin A4 y SCL = pin A5, fijo.
- **Por qué el buzzer pasivo**: es un parlantito tonto; el Arduino le manda una onda
  a la frecuencia de cada nota con `tone()`. Uno "activo" trae su propio oscilador
  interno y suena siempre igual — no sirve para melodías.
- **Protoboard**: las filas de 5 agujeros están unidas por dentro; los rieles largos
  de los costados (+ y −) recorren todo el borde. Sirve para prototipar sin soldar.

---

## 3. Preparar el entorno en NixOS

Nada de bajar instaladores: va declarativo, como todo en tu máquina.

**a) Arduino IDE** — en tu flake (`~/projects/nix-config/`), al módulo donde tengas
los paquetes de usuario:

```nix
home.packages = with pkgs; [ arduino-ide ];
```

**b) Permiso del puerto serie** — el Uno aparece como `/dev/ttyUSB0` (clon CH340) o
`/dev/ttyACM0` (original). Ese device pertenece al grupo `dialout`; tu usuario tiene
que estar en él:

```nix
users.users.rolando.extraGroups = [ "dialout" ];  # junto a los grupos que ya tengas
```

*Por qué:* udev crea el device con grupo `dialout` por regla estándar; en NixOS no
hay que instalar drivers (el CH340 viene en el kernel) ni tocar reglas udev — solo
darte membresía al grupo. Requiere `nixos-rebuild switch` + **relogin** (los grupos
se leen al iniciar sesión).

**c) Verificar**: enchufá el Uno y corré `dmesg | tail` — tenés que ver el device
nuevo. En el IDE: *Tools → Board → Arduino Uno*, *Tools → Port → /dev/ttyUSB0*.

**d) Librerías (solo para partes 2 y 3)** — en el IDE, *Library Manager*:
- `Servo` (viene incluida con el core del Uno, no hay que instalar nada)
- `Adafruit GFX Library` (del Library Manager)
- ⚠️ `Adafruit_SH1106` **no está en el Library Manager** — es un fork comunitario.
  Se instala por ZIP: bajá https://github.com/wonho-maker/Adafruit_SH1106 y en el
  IDE *Sketch → Include Library → Add .ZIP Library*. (A esto se refiere el
  `Importante.docx` con "si te sale error de librería".) En §5 te muestro la
  alternativa mantenida.

---

## 4. Construcción paso a paso

### Parte 1 — el buzzer solo (30 min)

Objetivo: validar toolchain completo (IDE → compilar → subir → hardware responde)
con el mínimo de piezas. Es tu "hello world" de hardware.

1. Buzzer en la protoboard. Pata **+** (la más larga, o la marcada +) → jumper →
   **pin 3** del Arduino. Pata − → riel − de la protoboard → jumper → **GND** del Arduino.
2. Abrí `Parte 1/diometutorial1/diometutorial1.ino` en el IDE.
3. Botón **Upload** (flecha →). Compila, sube por USB, y al terminar el Arduino se
   resetea solo y suena el ringtone de Nokia. 🎉
4. Si querés repetirlo: botón físico **RESET** del Uno (todo el show vive en
   `setup()`, que corre una vez por encendido/reset).

**Cómo funciona el código** (vas a reconocer todo): `melody[]` es un array plano de
pares `(nota, duración)`. La nota es una frecuencia en Hz (`NOTE_A4 = 440`). La
duración es notación musical: `4` = negra, `8` = corchea, `16` = semicorchea,
negativo = con puntillo (×1.5). El loop calcula los milisegundos de cada nota a
partir del `tempo` y llama `tone(pin, freq, ms)`.

### Parte 2 — se suman los brazos (30 min)

1. **Rieles de la protoboard**: jumper del pin **5V** del Arduino al riel **+**, y
   de **GND** al riel **−**. Ahora tenés "alimentación" distribuida.
2. Cada SG90 tiene 3 cables: **marrón = GND** (riel −), **rojo = 5V** (riel +),
   **naranja = señal**. Señal del servo 1 → **pin 9**; señal del servo 2 → **pin 10**.
3. Buzzer queda como estaba (pin 3).
4. Subí `Parte 2/diometutorial2.ino`: suena Megalovania y los brazos se agitan
   alternando entre dos posiciones con cada nota.
5. **Antes de montar los brazos definitivos**: con los servos a 50°
   (`servoMotor.write(50)` del arranque), colocá el bracito en el eje en la posición
   "reposo" que quieras. El código los mueve entre 0–30° y 100–130°.

**Si el Arduino se resetea solo o los servos tiemblan**, es caída de tensión: los
dos servos arrancando a la vez chupan picos que el USB (500 mA) a veces no banca.
Fix barato: capacitor de 470–1000 µF entre los rieles + y − (pata larga al +,
**la franja gris del capacitor va al −**, no lo pongas al revés). El capacitor actúa
de "buffer de picos". En el build original con brazos de papel anda sin esto; con
brazos más pesados (tu caso robusto) lo vas a querer sí o sí — ver §5.

### Parte 3 — la cara (30 min)

1. Pantalla OLED a la protoboard (o directo con jumpers hembra):
   **VCC → riel +**, **GND → riel −**, **SCL → A5**, **SDA → A4**.
   ⚠️ Leé la serigrafía de TU pantalla para VCC/GND — hay dos variantes de layout.
2. Subí `Parte 3/diometutorial3.ino`: aparece la carita y arranca el show completo.
3. **Pantalla en negro y el resto anda** → 99% es dirección I2C o driver. Subí un
   sketch "I2C scanner" (buscalo así, es estándar) y fijate qué dirección reporta:
   si no es `0x3C`, cambiala en `display.begin(SH1106_SWITCHCAPVCC, 0x3C)`. Si el
   scanner la ve pero se ve basura/corrida, tu pantalla es SSD1306 → §5.
4. **Cara personalizada**: cualquier imagen B/N de 128×64 → https://javl.github.io/image2cpp/
   → genera el array C → reemplazás el contenido de `epd_bitmap_1[]`. (Podés dibujarla
   con la paleta Deep Ocean en mente y exportar a 1-bit, quedará como pixel art.)

---

## 5. Mejoras — qué, cómo y por qué

Cada una responde a un problema real del código del tutorial, no a "hardening por
si acaso". En orden de valor:

### 5.1 Alimentación de servos decente (problema físico real)
**Problema:** los servos comparten los 5V del regulador del Arduino, que a su vez
vive del USB. Picos de arranque → brownout → resets aleatorios, "funciona a veces".
**Fix mínimo:** el capacitor de §4/Parte 2.
**Fix correcto para la versión final:** fuente 5V externa (un cargador de celular
viejo + cable sacrificado, o un powerbank) alimentando los servos directo, con
**GND común** con el Arduino (regla de oro de §2). El Arduino sigue por USB o por
la misma fuente al pin 5V.

### 5.2 Botón para disparar el show (bug de diseño del original)
**Problema:** todo vive en `setup()` → suena UNA vez al enchufar y muere; para
repetir hay que resetear. Un robot de escritorio que solo actúa al enchufarlo es
medio inútil.
**Fix:** un pulsador entre el pin 2 y GND, `pinMode(2, INPUT_PULLUP)`, y mover el
show a una función que `loop()` llama cuando `digitalRead(2) == LOW`. El
`INPUT_PULLUP` activa una resistencia interna que mantiene el pin en 5V hasta que
el botón lo baja a GND — así no necesitás resistencia externa.
*(Costo: un pulsador de $nada y 6 líneas. Es la mejora con mejor ratio valor/esfuerzo.)*

### 5.3 De bloqueante a no-bloqueante (la mejora "de programador")
**Problema:** el código usa `delay(noteDuration)` — durante cada nota el micro está
congelado. Por eso los servos solo pueden saltar entre 2 poses discretas y la cara
es estática: no hay ciclos libres para nada más.
**Fix:** patrón `millis()` (el "event loop de pobre" del mundo embebido): en vez de
dormir, cada pasada de `loop()` compara el reloj contra "cuándo toca la próxima
nota / el próximo paso de servo / el próximo frame de cara" y avanza lo que
corresponda. Resultado: brazos con movimiento suave (interpolando ángulo de a 1-2°),
cara que parpadea/anima MIENTRAS suena la música. Es la misma idea que un scheduler
cooperativo, te va a resultar familiar. Esta mejora es la diferencia entre "suena y
tiembla" y "está vivo".

### 5.4 Limpiezas puntuales del código
- El par `14, 99` al inicio de cada frase de `melody[]` es un hack: 14 Hz es
  inaudible y lo usan como separador de tiempo (y de paso alterna los servos).
  Reemplazalo por `REST, 32` que es lo que expresa la intención.
- `#define bitmap_height 128` / `bitmap_width 64` están **al revés** (el bitmap es
  128 de ancho × 64 de alto). Funciona de casualidad porque también los pasan
  invertidos en `drawBitmap()`. Dales la vuelta a ambos: hoy es una mina para
  cualquiera que toque ese código.
- `ledcontrol` controla servos, no LEDs; `counter` no se usa. Renombrar/borrar.

### 5.5 Librería de pantalla mantenida (opcional, recomendado si tu pantalla difiere)
**Problema:** `Adafruit_SH1106` es un fork abandonado que se instala por ZIP, y te
ata a un driver exacto de pantalla (la trampa del `IMPORTANTE.docx`).
**Fix:** migrar a **U8g2** (Library Manager, activamente mantenida), que soporta
SH1106 **y** SSD1306 con cambiar UNA línea (el constructor). Elimina la clase entera
de problema "compré la pantalla equivocada". Costo: adaptar ~10 líneas de la parte 3
(`drawBitmap`/`display` tienen equivalentes directos: `drawXBM`/`sendBuffer`).
Para v1 con la pantalla correcta, no hace falta; hacelo si te toca SSD1306 o cuando
pases a la v2.

---

## 6. Versión robusta ("mil caídas") — la parte que el original no tiene

El del rollo de papel es un chiste que funciona porque pesa nada. La robustez real
sale de cuatro decisiones, en este orden de impacto:

### 6.1 Soldar (la #1 por lejos)
La causa número uno de muerte por caída no es rotura: es **jumpers que se salen de
la protoboard**. Para la versión final, soldá los componentes a una **placa
perforada** (perfboard) con tiras de pines hembra donde enchufás el Arduino Nano*
y la pantalla. Nunca soldaste — es una habilidad de una tarde con tutoriales de
YouTube, y practicás sobre la perfboard vacía antes.

*Nota: para el build final conviene un **Arduino Nano** (mismo chip que el Uno,
mismo código, formato chiquito que se solda a la perfboard). El Uno queda como tu
placa de desarrollo para siempre.

Alternativa sin soldar (menos robusta pero digna): jumpers **con una gota de
silicona caliente sobre cada conector** en ambos extremos. Feo pero efectivo.

Sobre la placa KiCad incluida (`placa.kicad_pcb`): podrías mandarla a fabricar
(JLCPCB, ~5 placas por pocos USD + envío), pero trae los componentes extra (IR, BLE)
que no usás. Para este proyecto, perfboard gana: más barato, inmediato, y aprendés más.

### 6.2 Caja
Opciones reales, de más accesible a más linda:

- **Caja de paso/estanca de PVC de ferretería** (las de instalación eléctrica,
  ~10×10 cm): virtualmente indestructible, barata, se agujerea con mecha común para
  pantalla/servos/USB. Mi recomendación para tu objetivo "mil caídas".
- **Madera** (una cajita de encastre o hecha a mano): resistente y queda cálida.
- **Impresión 3D** (PETG, no PLA — el PLA es frágil a impactos): si no tenés
  impresora, en Viedma buscá servicio de impresión local o el fablab de la
  universidad. Permite el diseño más "mascota", y podés imprimir esquinas/bumpers
  en TPU (goma) que absorben todo.

### 6.3 Diseño anti-caída (independiente del material)
- **La pantalla es lo único genuinamente frágil** (es vidrio). Va **rebajada**: el
  frente de la caja sobresale unos mm por delante del vidrio, así el impacto lo
  recibe la caja, nunca la pantalla. Es lo mismo que el borde de un celular.
- **Servos atornillados** a la caja (traen orejas con agujeros), no pegados.
- **Brazos livianos y flexibles**: goma eva, plástico fino o TPU. Un brazo rígido
  y pesado es una palanca que arranca el engranaje del servo en la caída — los SG90
  tienen engranajes de nylon. (Existen SG90 con engranajes metálicos, "MG90S", por
  poco más: upgrade directo si un brazo muere.)
- **Todo lo interno fijado**: perfboard atornillada con separadores o asentada en
  silicona caliente; nada suelto que se convierta en proyectil interno.
- **Strain relief del USB**: el cable entra por un agujero justo y se fija por
  dentro con un precinto, así el tirón no llega a la soldadura.
- **Masa baja**: menos masa = menos energía en el impacto. No pongas powerbank
  ADENTRO si podés alimentarlo por cable.

### 6.4 El toque jocoso-funcional
Un **sensor de vibración SW-420** (o tilt switch, centavos) en un pin digital: si
el robot detecta el golpe de la caída, grita (frecuencias agudas en el buzzer) y
pone cara de ☠️ en la OLED. Convierte tu requisito "que aguante caídas" en la
feature más divertida del robot. Costo: un componente y 15 líneas.

---

## 7. ¿Y Go? — veredicto honesto

**En el Arduino Uno: contra, no lo hagas.** El Uno tiene 2 KB de RAM; solo el
framebuffer de la pantalla (128×64÷8) come 1 KB. TinyGo soporta AVR pero es su
target más limitado. Pelearías contra la herramienta sin ganancia.

**Camino Go que sí es un pro — firmware TinyGo en Raspberry Pi Pico (v2):**
la Pico (RP2040, ~264 KB RAM, suele costar menos que el Uno) es target de primera
clase de TinyGo, y verifiqué hoy que `tinygo-org/drivers` trae exactamente los tres
drivers que este proyecto necesita: **`sh1106`**, **`servo`** y **`buzzer`**.
O sea: podés portar Astro a Go real (`machine.I2C`, goroutines para la
animación no-bloqueante de §5.3 — que en Go sale natural con goroutines en vez del
patrón millis). Único costo: abandonás el ecosistema de tutoriales Arduino/C++.
Por eso la secuencia correcta es **v1 en C++ tal cual el tutorial** (menor fricción
mientras aprendés la electrónica, que es la variable nueva) → **v2 port a
TinyGo/Pico** cuando el hardware ya te sea conocido.

**El pro más limpio de todos — Go del lado del host (v3):** el robot conectado por
USB expone un protocolo serie trivial (`SAD\n`, `PLAY megalovania\n`...) y un
**daemon en Go en tu laptop** — mismo patrón que pedco-bot: systemd unit declarada
en el flake — le manda comandos según eventos reales: una notificación de mako, un
build que falla, una alerta del futuro lab Wazuh. Ahí Go juega de local (serial +
daemon + systemd es exactamente tu stack), y Astro pasa de juguete a periférico
de tu escritorio. Esta capa ni siquiera requiere cambiar de microcontrolador: anda
igual con el firmware C++ de la v1 leyendo `Serial`.

---

## 8. Hoja de ruta sugerida

| Fase | Qué | Tiempo estimado |
|---|---|---|
| v0 | Comprar lista de §1 · instalar IDE + dialout en el flake | mientras llega el envío |
| v1 | Partes 1-2-3 del tutorial tal cual, en protoboard | una tarde |
| v1.5 | Botón (§5.2) + capacitor (§5.1) + cara propia | una tarde |
| v2 | Soldado en perfboard + caja PVC + sensor de caída (§6) | un finde |
| v3 | Port TinyGo/Pico **o** daemon Go host-side (§7) — el que más te pique | un finde |

Cortá donde quieras: cada fase termina en algo que funciona.
