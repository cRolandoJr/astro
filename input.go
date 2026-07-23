package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// InputSource entrega el próximo comando como texto. Interfaz porque hay dos impls
// reales: VoiceInput (voz) y StdinInput (teclado, para debug). Swappable por env en main.
type InputSource interface {
	Listen() (text string, err error)
}

// ---- StdinInput: leer una línea (útil para debuggear el pipeline sin micrófono) ----
type StdinInput struct{ scanner *bufio.Scanner }

func NewStdinInput() *StdinInput { return &StdinInput{scanner: bufio.NewScanner(os.Stdin)} }

func (s *StdinInput) Listen() (string, error) {
	fmt.Print("> ")
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return s.scanner.Text(), nil
}

// ---- VoiceInput: Enter para hablar → grabar → transcribir ----
type VoiceInput struct {
	Runner       Runner
	WhisperBin   string
	WhisperModel string
	RecSeconds   int
	WavPath      string                       // ej. /tmp/astro-in.wav
	ReadFile     func(string) ([]byte, error) // default os.ReadFile; inyectable en tests
	trigger      *bufio.Scanner
}

func NewVoiceInput(r Runner, whisperBin, whisperModel string, recSeconds int) *VoiceInput {
	return &VoiceInput{
		Runner: r, WhisperBin: whisperBin, WhisperModel: whisperModel,
		RecSeconds: recSeconds, WavPath: "/tmp/astro-in.wav",
		trigger: bufio.NewScanner(os.Stdin),
	}
}

func (v *VoiceInput) Listen() (string, error) {
	fmt.Print("[Enter] para hablar (Ctrl+D para salir) ")
	if !v.trigger.Scan() {
		if err := v.trigger.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return v.capture()
}

// capture graba y transcribe. Es la parte testeable (sin el Enter interactivo).
func (v *VoiceInput) capture() (string, error) {
	secs := v.RecSeconds
	if secs <= 0 {
		secs = 4
	}
	fmt.Printf("🎙️  grabando %ds… hablá ahora\n", secs)
	if _, err := v.Runner.Run("arecord", "-q", "-d", strconv.Itoa(secs),
		"-f", "S16_LE", "-r", "16000", "-c", "1", v.WavPath); err != nil {
		return "", fmt.Errorf("no pude grabar (¿arecord instalado?): %w", err)
	}
	// whisper escribe la transcripción a <of>.txt (-nt: sin timestamps).
	ofPrefix := strings.TrimSuffix(v.WavPath, ".wav")
	if _, err := v.Runner.Run(v.WhisperBin, "-m", v.WhisperModel, "-f", v.WavPath,
		"-l", "es", "-nt", "-otxt", "-of", ofPrefix); err != nil {
		return "", fmt.Errorf("no pude transcribir (¿whisper/modelo?): %w", err)
	}
	readFile := v.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	raw, err := readFile(ofPrefix + ".txt")
	if err != nil {
		return "", fmt.Errorf("no pude leer la transcripción: %w", err)
	}
	return cleanTranscript(string(raw)), nil
}

// cleanTranscript limpia la salida de whisper: saca marcadores entre corchetes
// ([BLANK_AUDIO], [música], etc.), espacios y saltos de línea sobrantes.
func cleanTranscript(raw string) string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = stripBrackets(line)
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

// stripBrackets remueve tramos "[...]" (marcadores no-verbales de whisper).
func stripBrackets(s string) string {
	for {
		i := strings.IndexByte(s, '[')
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], ']')
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+j+1:]
	}
}
