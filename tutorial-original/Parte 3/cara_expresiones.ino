// ============================================================================
//  Astro · cara con expresiones (dibujada con primitivas, NO bitmap)
// ----------------------------------------------------------------------------
//  Por qué primitivas y no un bitmap por cara:
//   - Un bitmap 128x64 ocupa 1024 bytes de flash. Con 9 caras = 9 KB por una
//     carita. Dibujando con lineas/circulos, TODAS las caras juntas son ~1-2 KB
//     de codigo y comparten el marco: agregar o animar un gesto es trivial.
//   - Reemplaza a: epd_bitmap_1[], BITMAP_ANCHO/ALTO y dibujarCara() del v2.
//
//  Pega este bloque en astro_firmware.ino (arriba de setup()). Requiere que
//  'display' ya este declarado (Adafruit_SH1106 display(OLED_RESET);).
// ============================================================================

enum Cara { NEUTRAL, PARPADEO, PENSATIVO, CURIOSO, FELIZ, SORPRENDIDO,
            BOSTEZO, DORMIDO, MAREADO, GUINO, AMOR, TRISTE, ENOJADO };

// Linea "gruesa" (~3 px): GFX dibuja lineas de 1 px, asi que apilamos 3.
static void tLine(int x0, int y0, int x1, int y1) {
  display.drawLine(x0, y0 - 1, x1, y1 - 1, WHITE);
  display.drawLine(x0, y0,     x1, y1,     WHITE);
  display.drawLine(x0, y0 + 1, x1, y1 + 1, WHITE);
}
// Linea fina (~2 px), para cejas.
static void tLine2(int x0, int y0, int x1, int y1) {
  display.drawLine(x0, y0,     x1, y1,     WHITE);
  display.drawLine(x0, y0 + 1, x1, y1 + 1, WHITE);
}
// Franja horizontal gruesa (para ojos cerrados / boca recta).
static void barra(int x, int y, int w, int alto) {
  for (int i = 0; i < alto; i++) display.drawFastHLine(x, y + i, w, WHITE);
}
// Curva de boca: parabola de x0..x1. En el centro vale 'ymid'; en los extremos
// 'yend'. yend<ymid => sonrisa; yend>ymid => boca hacia abajo.
static void curvaBoca(int x0, int x1, int ymid, int yend) {
  float cx = (x0 + x1) / 2.0, half = (x1 - x0) / 2.0;
  int px = x0, py = ymid;
  for (int x = x0; x <= x1; x += 2) {
    float n = (x - cx) / half;
    int y = (int)(ymid + (yend - ymid) * n * n + 0.5);
    tLine(px, py, x, y);
    px = x; py = y;
  }
}

// Llamar SOLO al cambiar de gesto (display() usa el bus I2C, ~30-40 ms; no lo
// pongas en cada vuelta del loop).
void drawFace(Cara c) {
  display.clearDisplay();

  // Marco del visor (~2 px), comun a todas las caras
  display.drawRoundRect(8, 6, 112, 52, 10, WHITE);
  display.drawRoundRect(9, 7, 110, 50, 9,  WHITE);

  switch (c) {
    case NEUTRAL:                                    // ojos barra + boca recta
      display.fillRoundRect(32, 20, 21, 13, 4, WHITE);
      display.fillRoundRect(76, 20, 21, 13, 4, WHITE);
      barra(50, 45, 29, 3);
      break;

    case PARPADEO:                                   // ojos cerrados (un instante) + boca recta
      barra(32, 26, 21, 2);
      barra(76, 26, 21, 2);
      barra(50, 45, 29, 3);
      break;

    case PENSATIVO:                                  // ojos arriba + ceja + "..."
      display.fillCircle(42, 23, 4, WHITE);
      display.fillCircle(86, 23, 4, WHITE);
      display.drawLine(78, 17, 94, 19, WHITE);
      barra(48, 46, 17, 2);
      display.fillCircle(98, 44, 1, WHITE);
      display.fillCircle(104, 44, 1, WHITE);
      display.fillCircle(110, 44, 1, WHITE);
      break;

    case CURIOSO:                                    // ojos grandes + una ceja levantada + boca 'o'
      display.fillCircle(42, 26, 7, WHITE);
      display.fillCircle(86, 26, 7, WHITE);
      display.drawLine(76, 15, 96, 18, WHITE);
      display.fillCircle(64, 46, 3, WHITE);
      break;

    case FELIZ:                                      // ojos ^ ^ + sonrisa
      tLine(32, 30, 42, 20); tLine(42, 20, 52, 30);
      tLine(76, 30, 86, 20); tLine(86, 20, 96, 30);
      curvaBoca(46, 82, 50, 42);
      break;

    case BOSTEZO:                                    // ojos entornados + boca abierta 'O'
      barra(34, 24, 17, 2);
      barra(78, 24, 17, 2);
      display.drawCircle(64, 46, 8, WHITE);
      display.drawCircle(64, 46, 7, WHITE);
      break;

    case GUINO:                                      // un ojo abierto + un ^ + sonrisa
      display.fillRoundRect(32, 20, 21, 13, 4, WHITE);
      tLine(76, 30, 86, 20); tLine(86, 20, 96, 30);
      curvaBoca(46, 82, 50, 42);
      break;

    case AMOR:                                       // ojos corazon + sonrisa
      for (int i = 0; i < 2; i++) {
        int e = (i == 0) ? 42 : 86;
        display.fillCircle(e - 4, 24, 4, WHITE);
        display.fillCircle(e + 4, 24, 4, WHITE);
        display.fillTriangle(e - 8, 26, e + 8, 26, e, 34, WHITE);
      }
      curvaBoca(48, 80, 49, 43);
      break;

    case SORPRENDIDO:                                // ojos redondos + boca 'o'
      display.fillCircle(42, 26, 9, WHITE);
      display.fillCircle(86, 26, 9, WHITE);
      display.fillCircle(64, 46, 5, WHITE);
      break;

    case TRISTE:                                     // cejas hacia adentro + lagrima + boca abajo
      display.fillRoundRect(34, 27, 17, 8, 3, WHITE);
      display.fillRoundRect(78, 27, 17, 8, 3, WHITE);
      tLine2(34, 26, 50, 20);
      tLine2(78, 20, 94, 26);
      display.fillCircle(38, 41, 3, WHITE);
      curvaBoca(48, 80, 45, 51);
      break;

    case ENOJADO:                                    // cejas \ / + ojos + boca abajo
      tLine(34, 20, 52, 28); tLine(94, 20, 76, 28);
      display.fillRoundRect(36, 30, 15, 7, 3, WHITE);
      display.fillRoundRect(78, 30, 15, 7, 3, WHITE);
      curvaBoca(50, 78, 43, 48);
      break;

    case DORMIDO:                                    // ojos cerrados + boca chica + Z
      barra(32, 27, 21, 3);
      barra(76, 27, 21, 3);
      barra(58, 45, 13, 2);
      display.drawLine(100, 14, 112, 14, WHITE);
      display.drawLine(112, 14, 100, 22, WHITE);
      display.drawLine(100, 22, 112, 22, WHITE);
      break;

    case MAREADO:                                    // ojos X X + boca ondulada
      tLine(33, 19, 51, 33); tLine(51, 19, 33, 33);
      tLine(77, 19, 95, 33); tLine(95, 19, 77, 33);
      { int px = 46, py = 46;
        for (int x = 46; x <= 82; x += 2) {
          int y = (int)(46 + 2.4 * sin((x - 46) / 3.0) + 0.5);
          tLine(px, py, x, y); px = x; py = y;
        } }
      break;
  }
  display.display();
}

// ============================================================================
//  MOTOR DE CONTEXTO — las caras se disparan solas
// ----------------------------------------------------------------------------
//  La idea: NO llamar drawFace() a mano por todos lados, sino que una funcion
//  updateFace() decida la cara segun el contexto en cada vuelta del loop, y
//  solo redibuje cuando cambia (redibujar cuesta ~30-40 ms de I2C).
//
//  El "contexto" sale de entradas. Las que el robot YA tiene:
//    - estado interno (showSonando del v2: ¿esta tocando?)
//    - boton (PIN_BOTON)
//    - tiempo: cuanto hace que no pasa nada (inactividad)
//  Las que piden un sensor barato (declaralos si los sumás):
//    - sonido/microfono  -> te oye  -> CURIOSO / despierta
//    - golpe (SW-420)     -> MAREADO
//    - luz (LDR)          -> se duerme a oscuras, etc.
//
//  Escala de reposo por inactividad (los umbrales, a gusto):
//    0-8 s   : NEUTRAL con PARPADEO cada ~4 s
//    8-20 s  : de vez en cuando PENSATIVO
//    20-35 s : BOSTEZO
//    >35 s   : DORMIDO   (y despierta CURIOSO si te oye o toca el boton)

Cara caraActual = NEUTRAL;
unsigned long ultimaActividad = 0;   // millis() del ultimo "algo pasó"
unsigned long ultimoParpadeo  = 0;

// Llamalo apenas haya interaccion (boton, sonido, arranque) para reiniciar el
// reloj de inactividad.
void marcarActividad() { ultimaActividad = millis(); }

// Pide una cara; solo redibuja si cambió respecto a la que ya está en pantalla.
void setFace(Cara c) { if (c != caraActual) { caraActual = c; drawFace(c); } }

// Decide la cara segun el contexto. Llamalo en cada vuelta de loop().
// 'sonando' = tu showSonando del v2. 'oye' = lectura del sensor de sonido
// (o false si todavia no tenés micrófono). 'golpe' = sensor de caída.
void updateFace(bool sonando, bool oye, bool golpe) {
  unsigned long ahora = millis();

  if (golpe)   { marcarActividad(); setFace(MAREADO);     return; }
  if (sonando) { marcarActividad(); setFace(FELIZ);       return; }
  if (oye)     { marcarActividad(); setFace(CURIOSO);     return; }

  unsigned long inactivo = ahora - ultimaActividad;
  if (inactivo > 35000UL) { setFace(DORMIDO); return; }
  if (inactivo > 20000UL) { setFace(BOSTEZO); return; }
  if (inactivo > 8000UL)  { setFace(PENSATIVO); return; }

  // reposo tranquilo: parpadeo natural cada ~4 s
  if (ahora - ultimoParpadeo > 4000UL) {
    ultimoParpadeo = ahora;
    drawFace(PARPADEO); caraActual = PARPADEO;
    // el parpadeo dura un toque; volvemos a NEUTRAL en la proxima pasada
  } else {
    setFace(NEUTRAL);
  }
}

// ----------------------------------------------------------------------------
//  Integracion en astro_firmware.ino
//  1) BORRA del v2: epd_bitmap_1[], #define BITMAP_ANCHO/ALTO y dibujarCara().
//  2) En setup(): drawFace(NEUTRAL); marcarActividad();
//  3) En loop(), en vez de actualizarCaraIdle(), llamá:
//         updateFace(showSonando, /*oye=*/false, /*golpe=*/false);
//     Cuando sumes sensores, reemplazás esos false por digitalRead(PIN_...).
//  4) En botonRecienApretado()==true: marcarActividad(); setFace(SORPRENDIDO);
//  5) GUINO/AMOR/TRISTE/ENOJADO no tienen contexto natural: quedan para llamar
//     a mano (combo de botones, o comando serie desde la laptop / daemon Go).
//
//  Para AGREGAR una cara: nombre al enum + un 'case' en drawFace() + (si querés
//  que salga sola) una regla en updateFace(). Sin bitmaps ni bytes que generar.
// ============================================================================
