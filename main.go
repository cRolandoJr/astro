package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

func main() {
	runner := ExecRunner{}
	var display Display = EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	// Salida de voz (si no hay piper configurado, degradamos a solo-texto).
	voice := PiperVoice{
		Runner: runner, PiperBin: envOr("ASTRO_PIPER_BIN", "piper"),
		Voice: os.Getenv("ASTRO_PIPER_VOICE"), WavPath: "/tmp/astro-out.wav",
	}

	// Entrada: voz por default, stdin si ASTRO_INPUT=stdin (para debug).
	var input InputSource
	if os.Getenv("ASTRO_INPUT") == "stdin" {
		input = NewStdinInput()
	} else {
		secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS"))
		input = NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cpp"),
			os.Getenv("ASTRO_WHISPER_MODEL"), secs)
	}

	_ = display.Show(Neutral)
	fmt.Println("Astro está despierto.")

	for {
		text, err := input.Listen()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "entrada:", err)
			continue
		}
		if text == "" {
			respond(display, voice, Pensativo, "No te escuché, repetí.")
			continue
		}

		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			respond(display, voice, Pensativo, "No te entendí.")
			continue
		}

		reply, err := action.Run(runner)
		if err != nil {
			_ = display.Show(Neutral)
			fmt.Println("Ups:", err) // error de ejecución: dev-facing, solo pantalla
			continue
		}
		respond(display, voice, action.Face, reply)
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
