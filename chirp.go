package main

import "strings"

// Chirps hace los "beeps" tipo Wall-E/R2 con `sox synth` (sin archivos de audio).
// Cada kind es una receta de sox synth; se puede tunear por env sin recompilar.
type Chirps struct {
	Runner  Runner
	Enabled bool
	WavPath string
	Specs   map[string]string // kind -> args de `sox synth ...`
}

// defaultChirpSpecs: recetas validadas (sweep de frecuencia + tremolo = timbre robotcito).
func defaultChirpSpecs() map[string]string {
	return map[string]string{
		"wake":     "synth 0.18 sine 500-1400 tremolo 30 40 vol 0.5", // saludo, sube = "¡hola!"
		"confused": "synth 0.28 sine 1200-400 tremolo 25 50 vol 0.5", // baja = "¿eh?"
		"ok":       "synth 0.12 sine 800-1200 vol 0.5",               // blip corto de "listo"
	}
}

// Play emite el chirp de 'kind'. Es decorativo: si sox/pw-play fallan, no corta el flujo.
func (c Chirps) Play(kind string) {
	if !c.Enabled {
		return
	}
	spec, ok := c.Specs[kind]
	if !ok {
		return
	}
	args := append([]string{"-n", c.WavPath}, strings.Fields(spec)...)
	if _, err := c.Runner.Run("sox", args...); err != nil {
		return
	}
	_, _ = c.Runner.Run("pw-play", c.WavPath)
}
