package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	runner := ExecRunner{}
	face := EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	acts := buildActions(time.Now)
	rules := NewRuleInterpreter(acts)
	var interpreter Interpreter = rules
	if os.Getenv("ASTRO_BRAIN") != "rules" { // default: cerebro LLM (con fallback a reglas)
		llmURL := os.Getenv("ASTRO_LLM_URL")
		llmKey := os.Getenv("ASTRO_LLM_KEY")
		embed := httpEmbed(llmURL, llmKey, envOr("ASTRO_EMBED_MODEL", "text-embedding-004"))
		topK, _ := strconv.Atoi(os.Getenv("ASTRO_MEM_TOPK"))
		idleMin, _ := strconv.Atoi(os.Getenv("ASTRO_HISTORY_IDLE_MIN"))
		historyTurns, _ := strconv.Atoi(os.Getenv("ASTRO_HISTORY_TURNS"))
		vision := httpVision(llmURL, llmKey, envOr("ASTRO_VISION_MODEL", envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")))
		interpreter = NewLLMInterpreter(LLMConfig{
			Chat:    httpChat(llmURL, llmKey, envOr("ASTRO_LLM_MODEL", "gemini-flash-latest")),
			Actions: acts, ArgActions: buildArgActions(), Fallback: rules, Mem: buildMemory(embed), TopK: topK, Now: time.Now,
			HistoryTurns: historyTurns, IdleWindow: time.Duration(idleMin) * time.Minute,
			Vision: vision, Monitors: buildMonitorsPrompt(runner),
		})
	}

	// Salida de voz (si no hay piper configurado, degradamos a solo-texto).
	// ASTRO_VOICE_FX (opcional) = cadena de efectos sox para el timbre "robot".
	voice := PiperVoice{
		Runner: runner, PiperBin: envOr("ASTRO_PIPER_BIN", "piper"),
		Voice: os.Getenv("ASTRO_PIPER_VOICE"), WavPath: "/tmp/astro-out.wav",
		FxArgs: strings.Fields(os.Getenv("ASTRO_VOICE_FX")), FxPath: "/tmp/astro-fx.wav",
	}

	// Entrada: voz por default, stdin si ASTRO_INPUT=stdin (para debug), wake-word si ASTRO_INPUT=wake.
	var input InputSource
	var wake *WakeWordInput
	var hotkey *HotkeyInput
	switch os.Getenv("ASTRO_INPUT") {
	case "stdin":
		input = NewStdinInput()
	case "wake":
		w, err := NewWakeWordInput(os.Getenv("ASTRO_WAKE_CMD"), newVoice(runner, face))
		if err != nil {
			fmt.Fprintln(os.Stderr, "modo wake:", err)
			return
		}
		wake = w
		defer wake.Close()
		input = wake
	case "hotkey":
		h, err := NewHotkeyInput(envOr("ASTRO_TRIGGER_FIFO", "/tmp/astro-trigger.fifo"), newVoice(runner, face))
		if err != nil {
			fmt.Fprintln(os.Stderr, "modo hotkey:", err)
			return
		}
		hotkey = h
		defer hotkey.Close()
		input = hotkey
	default:
		input = newVoice(runner, face)
	}

	// Chirps tipo Wall-E (sox synth). Se apagan con ASTRO_CHIRPS=0; cada sonido se puede
	// tunear por env (ASTRO_CHIRP_WAKE / _CONFUSED / _OK) sin recompilar.
	chirpSpecs := defaultChirpSpecs()
	for kind := range chirpSpecs {
		if v := os.Getenv("ASTRO_CHIRP_" + strings.ToUpper(kind)); v != "" {
			chirpSpecs[kind] = v
		}
	}
	chirps := Chirps{
		Runner: runner, Enabled: os.Getenv("ASTRO_CHIRPS") != "0",
		WavPath: "/tmp/astro-chirp.wav", Specs: chirpSpecs,
	}

	// La cara aparece SOLO durante un turno (capture() la abre al escuchar; el loop la cierra
	// al terminar) → idle sin cara, no estorba. El defer/handler de señal la cierran al salir
	// por si un turno la dejó abierta.
	defer face.Close()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = face.Close()
		if wake != nil {
			_ = wake.Close()
		}
		if hotkey != nil {
			_ = hotkey.Close()
		}
		os.Exit(0)
	}()

	fmt.Println("Astro está despierto.")
	chirps.Play("wake")

	for {
		text, err := input.Listen()
		if err != nil {
			// EOF real = salida normal (Ctrl+D); cualquier otro error de lectura
			// también corta el loop, pero se reporta (no es una salida esperada).
			if !errors.Is(err, io.EOF) {
				fmt.Fprintln(os.Stderr, "entrada:", err)
			}
			break
		}
		// capture() abrió la cara al escuchar; la cerramos al terminar el turno (idle sin cara).
		func() {
			defer face.Close()
			if text == "" {
				chirps.Play("confused")
				respond(face, voice, Pensativo, "No te escuché, repetí.")
				return
			}
			action, err := interpreter.Interpret(text)
			if err != nil { // ErrNoEntiendo o cualquier error del cerebro: reacciona y sigue, sin crash
				chirps.Play("confused")
				respond(face, voice, Pensativo, "No te entendí.")
				return
			}
			reply, err := action.Run(runner)
			if err != nil {
				_ = face.Show(Neutral)
				fmt.Println("Ups:", err) // error de ejecución: dev-facing, solo pantalla
				return
			}
			respond(face, voice, action.Face, reply)
		}()
	}
	fmt.Println("\nChau 👋")
}

// respond muestra la cara, IMPRIME el texto y lo dice en voz alta. Imprimir siempre
// (aunque el TTS esté off) es la degradación que promete la spec §7: si no puede
// hablar, al menos responde por pantalla. Se usa en las 3 ramas de usuario.
func respond(d Display, v PiperVoice, face Expression, text string) {
	_ = d.Show(face)
	fmt.Println(text)
	say(v, text)
}

// say habla, pero si el TTS falla no corta el flujo (ya se mostró el texto).
func say(v PiperVoice, text string) {
	if err := v.Say(text); err != nil {
		fmt.Fprintln(os.Stderr, "(voz off:", err, ")")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newVoice arma la entrada por voz (grabación por silencio + whisper) desde el entorno.
func newVoice(runner Runner, face Display) *VoiceInput {
	secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS"))
	vi := NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cli"),
		os.Getenv("ASTRO_WHISPER_MODEL"), secs)
	vi.SilencePct = os.Getenv("ASTRO_REC_SILENCE_PCT")
	vi.TrailSec = os.Getenv("ASTRO_REC_TRAIL_SEC")
	vi.Face = face // muestra "escuchando" al grabar
	return vi
}

// buildMemory arma la memoria persistente. Si el archivo no se puede cargar, loguea y devuelve
// nil (Astro sigue funcionando, sin memoria) — nunca aborta el daemon.
func buildMemory(embed embedFunc) MemoryStore {
	path := os.Getenv("ASTRO_MEM_PATH")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "(sin HOME, memoria off:", err, ")")
			return nil
		}
		path = filepath.Join(home, ".local", "share", "astro", "memory.json")
	}
	mem, err := NewFileMemory(path, embed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "(no pude cargar la memoria, sigo sin ella:", err, ")")
		return nil
	}
	return mem
}

// buildMonitorsPrompt lee los monitores de Hyprland para que el LLM resuelva "el de la derecha" →
// nombre de salida. Si falla (no Hyprland / no hyprctl), devuelve "" (mirar captura toda la pantalla).
func buildMonitorsPrompt(r Runner) string {
	out, err := r.Run("hyprctl", "monitors", "-j")
	if err != nil {
		return ""
	}
	var mons []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		X           int    `json:"x"`
		Focused     bool   `json:"focused"`
	}
	if err := json.Unmarshal([]byte(out), &mons); err != nil {
		return ""
	}
	var parts []string
	for _, m := range mons {
		p := fmt.Sprintf("%s — %s, x=%d", m.Name, m.Description, m.X)
		if m.Focused {
			p += " (enfocado)"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}
