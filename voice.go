package main

import "fmt"

// PiperVoice dice un texto en voz alta: piper (texto por stdin) → wav → pw-play.
// Struct (no interfaz): una sola implementación hoy. Si mañana la voz sale del robot,
// en Go extraemos la interfaz sin tocar esto.
type PiperVoice struct {
	Runner   Runner
	PiperBin string
	Voice    string   // ruta al .onnx
	WavPath  string   // ej. /tmp/astro-out.wav (salida cruda de piper)
	FxArgs   []string // efectos sox opcionales (voz "robot"); vacío = sin efecto
	FxPath   string   // wav con el efecto aplicado, ej. /tmp/astro-fx.wav
}

func (p PiperVoice) Say(text string) error {
	if text == "" {
		return nil
	}
	if _, err := p.Runner.RunWithInput(text, p.PiperBin, "--model", p.Voice, "--output_file", p.WavPath); err != nil {
		return fmt.Errorf("no pude sintetizar la voz (¿piper/voz?): %w", err)
	}
	toPlay := p.WavPath
	// Efecto opcional: sox <cruda> <efectuada> <efectos...> para el timbre robot.
	if len(p.FxArgs) > 0 {
		soxArgs := append([]string{p.WavPath, p.FxPath}, p.FxArgs...)
		if _, err := p.Runner.Run("sox", soxArgs...); err != nil {
			return fmt.Errorf("no pude aplicar el efecto de voz (¿sox?): %w", err)
		}
		toPlay = p.FxPath
	}
	if _, err := p.Runner.Run("pw-play", toPlay); err != nil {
		return fmt.Errorf("no pude reproducir el audio: %w", err)
	}
	return nil
}
