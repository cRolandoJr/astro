package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	runner := ExecRunner{}
	face := EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	// Salida de voz (si no hay piper configurado, degradamos a solo-texto).
	// ASTRO_VOICE_FX (opcional) = cadena de efectos sox para el timbre "robot".
	voice := PiperVoice{
		Runner: runner, PiperBin: envOr("ASTRO_PIPER_BIN", "piper"),
		Voice: os.Getenv("ASTRO_PIPER_VOICE"), WavPath: "/tmp/astro-out.wav",
		FxArgs: strings.Fields(os.Getenv("ASTRO_VOICE_FX")), FxPath: "/tmp/astro-fx.wav",
	}

	// Entrada: voz por default, stdin si ASTRO_INPUT=stdin (para debug).
	var input InputSource
	if os.Getenv("ASTRO_INPUT") == "stdin" {
		input = NewStdinInput()
	} else {
		secs, _ := strconv.Atoi(os.Getenv("ASTRO_REC_SECONDS"))
		input = NewVoiceInput(runner, envOr("ASTRO_WHISPER_BIN", "whisper-cli"),
			os.Getenv("ASTRO_WHISPER_MODEL"), secs)
	}

	// Astro abre su propia cara y la cierra al salir: con Ctrl+D (defer) y también si
	// matan el proceso con Ctrl+C / kill (handler de señal), así no queda colgada.
	if err := face.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "(no pude abrir la cara:", err, ")")
	}
	defer face.Close()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = face.Close()
		os.Exit(0)
	}()

	_ = face.Show(Neutral)
	fmt.Println("Astro está despierto.")

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
		if text == "" {
			respond(face, voice, Pensativo, "No te escuché, repetí.")
			continue
		}

		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			respond(face, voice, Pensativo, "No te entendí.")
			continue
		}

		reply, err := action.Run(runner)
		if err != nil {
			_ = face.Show(Neutral)
			fmt.Println("Ups:", err) // error de ejecución: dev-facing, solo pantalla
			continue
		}
		respond(face, voice, action.Face, reply)
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
