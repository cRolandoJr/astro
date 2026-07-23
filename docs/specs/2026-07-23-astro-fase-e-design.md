# Spec — Astro · Fase E (memoria semántica persistente)

> Fecha: 2026-07-23 · Estado: borrador para revisión · Sigue a Fase D ("dice más", en `main`).
> Convenciones: identificadores en **inglés**, comentarios/mensajes en **español**. Solo stdlib (Go).

## 1. Objetivo

Que Astro **recuerde hechos entre sesiones** y los use al responder: *"acordáte que uso NixOS y odio el
purple"* → los guarda; en otra sesión *"¿qué distro me conviene?"* → responde sabiendo que usás NixOS.
Escritura **explícita** (vos decidís qué recordar), recuperación **semántica** (por similitud, no por
palabra exacta).

## 2. Alcance

**Incluye:** un action `recordar` que guarda un hecho; almacenamiento **persistente en archivo** de
`{texto, vector}`; **embeddings** vía API de Gemini (`text-embedding-004`); recuperación por **similitud
coseno top-K**; inyección de los hechos recuperados al `systemPrompt`. Interfaz `MemoryStore` como seam
para futura migración (Chroma). Tests con `embedFunc` fake (sin red).

**NO incluye (diferido con gatillo):** índice ANN / migración a ChromaDB (gatillo: hechos ≫ decenas de
miles — irreal con escritura manual; la interfaz queda lista igual). Extracción **automática** de hechos
(hoy es explícita). **Memoria de conversación multi-turno** (contexto dentro de la charla en curso) — es
un problema ortogonal, fase aparte. Borrar/editar hechos por voz.

**Por qué RAG desde ahora y no inyección directa:** la memoria de un asistente personal está diseñada para
**crecer**; la inyección directa (mandar todos los hechos cada turno) degrada en silencio al crecer (más
costo de tokens + el LLM se distrae con hechos irrelevantes → peores respuestas). Recuperar top-K acota eso.
Crecimiento = trayectoria esperada + falla silenciosa → el motor de recuperación se gana su lugar hoy.

## 3. Arquitectura — la interfaz `Interpreter` NO cambia

`memory.go` es el corazón. `LLMInterpreter` gana un `MemoryStore` opcional (nil-safe). El action `recordar`
se maneja como `open` (mencionado en el prompt + caso en `Interpret`). La recuperación aumenta el prompt.

```
Turno (frase del usuario):
  si mem != nil:
      facts = mem.Recall(frase, topK)      // embed(frase) → coseno vs todos → top-K textos
      systemPrompt incluye "Esto sé del usuario:\n- ..."   (si hay facts)
  chat(systemPrompt, frase) → {action, arg, say}      (igual que Fase D)

  si action == "recordar":
      → wrap(memoryWriteAction(mem, arg), say)          // embed(arg) → append → persistir; habla say
  resto: igual que Fase D (registro / open / charla / fallback)
```

### Componentes

**`memory.go`**
- `type Fact struct { Text string; Vec []float32 }`
- `type MemoryStore interface { Remember(text string) error; Recall(query string, k int) ([]string, error) }`
  — el **seam**. Futuro Chroma = otra impl, sin tocar `llm.go`/`main.go`.
- `type embedFunc func(text string) ([]float32, error)` — inyectable (fake en tests).
- `FileMemory{ path string; embed embedFunc; facts []Fact }` (impl):
  - `NewFileMemory(path string, embed embedFunc) (*FileMemory, error)`: carga `path` a `facts` (JSON).
    Si el archivo no existe → memoria vacía (no es error). Si está corrupto → log a stderr + memoria
    vacía (no crashea).
  - `Remember(text)`: `vec = embed(text)` → `append(facts, Fact{text, vec})` → **persiste el archivo
    completo** (JSON, texto+vector). Si `embed` o el write fallan → devuelve error (no muta en memoria si
    falló el embed).
  - `Recall(query, k)`: si `facts` vacío → `nil, nil` (sin llamada de red). Si no: `q = embed(query)` →
    `cosine(q, f.Vec)` por hecho → ordena desc → devuelve los `k` textos más altos.
- `cosine(a, b []float32) float32` — puro; `0` si alguna norma es 0 o dimensiones distintas.

**`embed.go`**
- `httpEmbed(baseURL, apiKey, model string) embedFunc`: `POST {baseURL}/embeddings` con
  `{"input": text, "model": model}`, `Authorization: Bearer {apiKey}`; parsea `data[0].embedding`
  (`[]float64` → `[]float32`). Mismo estilo que `httpChat` (net/http, sin SDK). Error con `%w` si HTTP
  != 2xx o body inesperado.

**`llm.go` (modificar)**
- `LLMInterpreter` gana campo `mem MemoryStore`. `NewLLMInterpreter(chat, actions, fallback)` sin cambio
  de firma; se agrega setter/campo — el constructor puede recibir `mem` como 4º parámetro **o** exponerse
  vía campo. Decisión: **4º parámetro** `NewLLMInterpreter(chat, actions, fallback, mem)` (mem puede ser nil).
- `Interpret`: al inicio, si `mem != nil`, `facts, err := mem.Recall(text, li.topK)`; si `err != nil` →
  log a stderr y `facts = nil` (el turno sigue). Pasar `facts` a `systemPrompt`.
- `systemPrompt(facts []string) string`: si `len(facts) > 0`, antepone bloque
  `"Esto es lo que sé del usuario:\n- <f1>\n- <f2>...\n"`. Menciona el action `recordar <hecho>` junto a
  `open <app>` en la lista.
- Ruteo de `recordar` en `Interpret` (antes o junto a `open`): `if choice.Action == "recordar" && li.mem != nil {
  return wrap(memoryWriteAction(li.mem, choice.Arg), choice.Say), nil }`. Si `mem == nil`, cae a fallback.
- `memoryWriteAction(mem MemoryStore, text string) *Action`: `&Action{Name:"recordar", Face: Feliz,
  Run: func(Runner)(string,error){ if err := mem.Remember(text); err != nil { return "", err }; return "", nil }}`.
  (El `say` del LLM lo pone `wrap`; si el LLM no mandó `say`, `wrap(base,"")` deja el reply vacío → main
  igual muestra/omite; ver §6.)
- `topK int` en el struct (default 5, seteado desde main).

**`main.go` (modificar)**
- Arma `embed := httpEmbed(url, key, os.Getenv ASTRO_EMBED_MODEL||"text-embedding-004")` reusando
  `ASTRO_LLM_URL` + `ASTRO_LLM_KEY`.
- `memPath := envOr("ASTRO_MEM_PATH", <default ~/.local/share/astro/memory.json>)`;
  `mem, err := NewFileMemory(memPath, embed)` (si err de carga irrecuperable → log + `mem=nil`, no aborta el daemon).
- Pasa `mem` y `topK` (`ASTRO_MEM_TOPK`, default 5) al `LLMInterpreter`. Con `ASTRO_BRAIN=rules` → `mem=nil`.

## 4. Persistencia

- Archivo: `~/.local/share/astro/memory.json` (dir se crea si falta). Formato: **array JSON** de
  `{"text": "...", "vector": [..768 floats..]}`. Guardar el vector evita re-embeber al boot (cero red).
- `Remember` reescribe el archivo completo (simple y atómico-suficiente para un solo escritor; write a
  temp + rename para no corromper ante corte). Un solo goroutine (el loop) escribe → sin mutex.

## 5. Embeddings

- Modelo `text-embedding-004` (Gemini, 768 dims), vía endpoint OpenAI-compat `/embeddings`. Reusa la key
  y base URL de Fase C. Sin cómputo local (la GPU no corre modelos de embeddings) — coherente con el
  cloud-brain ya decidido.

## 6. Errores y bordes (anti-verde-falso)

- `Recall` con embed caído → log stderr + turno sin hechos (nunca rompe la conversación).
- `Remember` con embed/write caído → error → el action falla → `main` responde el error (guard existente,
  sin panic). El hecho **no** queda a medias en RAM si falló el embed.
- `memory.json` ausente → vacío. Corrupto → log + vacío (no crashea el daemon).
- `recordar` con `mem == nil` (brain=rules o sin key) → fallback (no promete guardar lo que no puede).
- Dimensiones distintas en `cosine` (vector viejo de otro modelo) → `0` (no paniquea; ese hecho no matchea).

## 7. Decisiones (defaults, vetables)

- Escritura **explícita** (`recordar`); extracción automática → diferida.
- **RAG coseno desde ahora** (no inyección directa) — ver §2.
- Top-K = 5; sin umbral mínimo de similitud en v1 (se puede sumar si se inyectan hechos irrelevantes).
- Cara al recordar: `Feliz`.
- Reusa `ASTRO_LLM_URL/KEY`; envs nuevos: `ASTRO_EMBED_MODEL`, `ASTRO_MEM_PATH`, `ASTRO_MEM_TOPK`.

## 8. Estructura de archivos

```
~/projects/astro/
  memory.go       — (crear) Fact, MemoryStore, FileMemory, cosine, embedFunc
  memory_test.go  — (crear) cosine, Remember/Recall/round-trip con embedFunc fake
  embed.go        — (crear) httpEmbed → embedFunc (net/http, OpenAI-compat)
  llm.go          — (modificar) campo mem+topK; Recall→systemPrompt; ruteo recordar; memoryWriteAction
  llm_test.go     — (modificar) ruteo recordar (fake MemoryStore); inyección de hechos al prompt
  main.go         — (modificar) armar embed + FileMemory; inyectar al interpreter
  (interpreter.go, voice.go, input.go, action.go, etc. — sin cambios)
```

## 9. Definition of Done (Fase E)

1. `go build/vet/test ./...` verdes (tests de `cosine`, `FileMemory` Remember/Recall/round-trip, ruteo
   `recordar`, inyección de hechos; + todos los previos A/B/C/D intactos).
2. E2E: `recordar "X"` persiste en `memory.json`; reiniciar y preguntar algo relacionado → Astro responde
   usando el hecho (recuperado por coseno, no re-embebido al boot).
3. Fallback/seguridad de fases previas intactos; nil-safe con `ASTRO_BRAIN=rules`.
4. Convenciones: ids inglés, comentarios español, nada destructivo, solo stdlib.
