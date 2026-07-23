package main

import "fmt"

// PiperVoice dice un texto en voz alta: piper (texto por stdin) → wav → pw-play.
// Struct (no interfaz): una sola implementación hoy. Si mañana la voz sale del robot,
// en Go extraemos la interfaz sin tocar esto.
type PiperVoice struct {
	Runner   Runner
	PiperBin string
	Voice    string // ruta al .onnx
	WavPath  string // ej. /tmp/astro-out.wav
}

func (p PiperVoice) Say(text string) error {
	if text == "" {
		return nil
	}
	if _, err := p.Runner.RunWithInput(text, p.PiperBin, "--model", p.Voice, "--output_file", p.WavPath); err != nil {
		return fmt.Errorf("no pude sintetizar la voz (¿piper/voz?): %w", err)
	}
	if _, err := p.Runner.Run("pw-play", p.WavPath); err != nil {
		return fmt.Errorf("no pude reproducir el audio: %w", err)
	}
	return nil
}
