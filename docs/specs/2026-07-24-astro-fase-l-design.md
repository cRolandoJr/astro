# Spec — Astro · Fase L (ejecutor genérico con confirmación)

> Fecha: 2026-07-24 · Estado: borrador para revisión · Sigue a Fase K (en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Que Astro pueda hacer "literalmente todo" lo que un comando de shell haría, **para lo que ningún tool
específico cubre** — pero **sin footgun**: todo comando genérico se **confirma** antes de correr, y un
puñado de patrones catastróficos se **rechaza de plano**. Es la opción **B** (poder + freno) que acordamos;
la **C** (shell sin freno) queda descartada por la regla del proyecto ("nada destructivo, sin shell arbitrario").

## 2. Alcance

**Incluye:** un action `ejecutar` (el LLM compone un comando de shell en `arg`); un flujo de **confirmación
de dos turnos** (proponer → confirmar → correr) con estado en el intérprete; un **backstop** de patrones
catastróficos (rechazo directo); gate por env `ASTRO_EXEC` (off por default; on en `run.sh`). Tests del flujo
completo con `fakeChat`/`fakeRunner` (sin correr comandos reales).

**NO incluye (diferido):** saltear la confirmación para comandos "seguros" (allowlist → gatillo si la
confirmación siempre cansa); sandbox/contenedor real; multi-turno de la conversación acoplado al exec.

## 3. Arquitectura — confirmación de dos turnos

`ejecutar` **no corre nada al toque**. El intérprete guarda un `pendingCmd` y confirma en el turno siguiente.

```
Turno 1 (pedido no cubierto): LLM → {action:"ejecutar", arg:"<cmd>", say:"..."}
    si !execEnabled → fallback (ejecutor apagado)
    si <cmd> matchea patrón catastrófico → acción que dice "Eso no lo hago, es peligroso." (NO guarda pending)
    si no → pendingCmd = <cmd>; acción que DICE "Voy a correr: <cmd>. ¿Lo confirmo?" (NO ejecuta)

Turno 2 (al TOPE de Interpret, ANTES del chat):
    si pendingCmd != "":
        tomar y limpiar pendingCmd
        clasificar el texto (confirmationVerdict → yes/no/other):
          NEGATIVO (no/cancelá/dejá/olvidalo) → "Listo, cancelado."
          AFIRMATIVO (sí/dale/confirmo/hacelo/ok…) → correr `timeout 60 sh -c <cmd>` → decir el resultado
          OTHER (un pedido nuevo, aunque arranque con muletilla) → cae al flujo normal = default seguro: NO corre
```

- **La confirmación se resuelve ANTES del LLM** (es un sí/no, no un pedido nuevo). Regla de seguridad:
  cuenta como sí/no SOLO si la **frase entera** es una confirmación — cada palabra (sobre el texto
  normalizado con `normalize`) es afirmativa, negativa o muletilla. **Cualquier palabra ajena** (ej.
  `"ok mostrame la hora"`) → es un pedido nuevo (`other`) → se reprocesa y NO corre. El negativo gana
  sobre el afirmativo (ante `"no, dale"` → no corre). Esto evita que una muletilla suelta dispare el comando.
- **Expiración:** el reset por inactividad (5 min, Fase F) también **limpia `pendingCmd`** — un comando
  pendiente viejo no se confirma "a ciegas" media hora después.
- **Correr:** `Runner.Run("timeout", "60", "sh", "-c", <cmd>)` — **acá sí hay shell arbitrario** (necesario
  para pipes/redirects/globs = "todo"), pero **solo después de que vos confirmes**, y acotado con `timeout`
  (el daemon es un solo goroutine; un comando colgado lo congelaría, igual que un curl sin `-m`). Reply
  hablado: salida corta (≤ ~200 chars) se dice; larga → "Listo, pero la salida es muy larga para leértela";
  falla → "Falló: <primeros chars de la salida>" (con `err=nil`, porque main.go a los errores de Run los
  imprime en pantalla, no los habla).
- **`ejecutar` es stateless re historial** (no entra al multi-turno, como `mirar`).

## 4. Seguridad (el punto)

- **Guarda primaria = confirmación.** Vos ves/oís el comando exacto y lo aprobás. Es lo único robusto para
  comandos arbitrarios (una denylist es gato-y-ratón: obfuscación, `$(...)`, encadenados).
- **Backstop catastrófico** (rechazo directo, aunque confirmes): patrones que borran todo o rompen el sistema
  — `rm -rf /`, `rm -rf ~`, `rm -rf /home`, `dd if=`/`dd of=`, `mkfs`, `sudo`, `:(){` (fork bomb),
  `> /dev/sd`/`> /dev/nvme` y `of=/dev/sd`/`of=/dev/nvme` (este host es NVMe), `shutdown`, `reboot`,
  `chmod -R /`, `chown -R /`. Se chequea sobre el comando en minúsculas con espacios colapsados. Los patrones
  de `dd` llevan `if=`/`of=` a propósito: `dd ` a secas daría falso positivo con `git add .` (contiene `dd `).
  **Honestidad: NO es airtight** (backstop contra alucinaciones obvias, no una sandbox); la confirmación
  sigue siendo la guarda real.
- **Gate `ASTRO_EXEC`:** apagado por default (la capacidad más peligrosa no se prende sola); `run.sh` la
  prende (`ASTRO_EXEC=1`). Con ella apagada, `ejecutar` cae a fallback (como si no existiera).
- **Caveat honesto asentado:** confirmar por voz un comando largo que no parseaste bien es un riesgo residual
  real. El backstop tapa lo catastrófico; el resto lo cubre tu atención al confirmar. Es tu máquina.

## 5. Prompt

`systemPrompt`, solo si `execEnabled`: `"- ejecutar (arg = un comando de shell): SOLO para lo que ningún
otro tool del menú cubre; se te confirmará antes de correr."` → el LLM prioriza los tools específicos y usa
`ejecutar` como último recurso.

## 6. Errores y bordes (anti-verde-falso)

- `execEnabled == false` → `ejecutar` → fallback (no promete lo que no puede).
- `arg` vacío → fallback.
- comando catastrófico → rechazo hablado, sin `pendingCmd`.
- confirmación ambigua / negativa → no corre (default seguro).
- `sh -c` falla → "Falló: <error>" (no crash).
- pending viejo → expira con el reset por inactividad.

## 7. Decisiones (defaults, vetables)

- Confirmar SIEMPRE (v1); saltear-lo-seguro diferido.
- Backstop catastrófico sí (belt, no airtight).
- `ASTRO_EXEC` off por default, on en `run.sh`.
- `ejecutar` stateless re historial; confirmación resuelta antes del LLM.

## 8. Estructura de archivos

```
~/projects/astro/
  llm.go       — (modificar) LLMConfig.ExecEnabled; campos execEnabled/pendingCmd; confirmación al tope de
                 Interpret; ruteo de "ejecutar"; systemPrompt; helpers confirmationVerdict/isCatastrophic/execAction/clip
  llm_test.go  — (modificar) tests: propone-no-corre, confirma-corre (sh -c), negativo-cancela, catastrófico-rechazado,
                 exec-off→fallback, expira-por-inactividad
  main.go      — (modificar) ExecEnabled desde os.Getenv("ASTRO_EXEC")
  run.sh       — (modificar) export ASTRO_EXEC=1
  (interpreter.go: `normalize` se reusa)
```

## 9. Definition of Done (Fase L)

1. `go build/vet/test ./...` verdes (flujo propose→confirm→run, negativo, catastrófico, off→fallback, expiración; + A–K intactos).
2. E2E: pedís algo no cubierto (ej. "cuántos archivos hay en Descargas") → Astro propone el comando y pide confirmar → "sí" → lo corre y dice el resultado; "no" → cancela; un comando catastrófico → lo rechaza.
3. Interfaz `Interpret` sin cambios de firma; seguridad de fases previas intacta; `ejecutar` off si `ASTRO_EXEC` no está.
4. Convenciones: ids inglés, comentarios español, solo stdlib.
