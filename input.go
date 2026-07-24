package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
	MaxSeconds   int                          // tope duro de grabación (seg); el silence corta antes en uso normal
	SilencePct   string                       // umbral de silencio (ej "3%"); vacío → default
	TrailSec     string                       // silencio de cola antes de cortar (ej "1.5"); vacío → default
	WavPath      string                       // ej. /tmp/astro-in.wav
	ReadFile     func(string) ([]byte, error) // default os.ReadFile; inyectable en tests
	Face         Display                      // opcional: muestra "escuchando" al grabar (nil = sin cara)
	trigger      *bufio.Scanner
}

func NewVoiceInput(r Runner, whisperBin, whisperModel string, maxSeconds int) *VoiceInput {
	return &VoiceInput{
		Runner: r, WhisperBin: whisperBin, WhisperModel: whisperModel,
		MaxSeconds: maxSeconds, WavPath: "/tmp/astro-in.wav",
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

// capture graba y transcribe con el tope normal (v.MaxSeconds). Es la parte testeable.
func (v *VoiceInput) capture() (string, error) { return v.captureMax(v.MaxSeconds) }

// captureMax es capture con un tope de grabación explícito (seg; <=0 → 30). Lo usa la ventana de
// conversación con un tope corto para volver a dormir rápido si no hablás.
func (v *VoiceInput) captureMax(maxSecs int) (string, error) {
	if maxSecs <= 0 {
		maxSecs = 30
	}
	thresh := v.SilencePct
	if thresh == "" {
		thresh = "3%"
	}
	trail := v.TrailSec
	if trail == "" {
		trail = "1.5"
	}
	if v.Face != nil {
		_ = v.Face.Open()        // idle = cara oculta; aparece al empezar a escuchar
		_ = v.Face.Show(Curioso) // cara de "te escucho" mientras grabás
	}
	fmt.Println("🎙️  hablá… (corta sola al callarte)")
	timing := os.Getenv("ASTRO_TIMING") == "1"
	// rec (sox) graba hasta el silencio de cola; 'timeout' es el tope duro si el umbral nunca
	// detecta silencio. rec sale 0 al cortar por silencio.
	tRec := time.Now()
	_, recErr := v.Runner.Run("timeout", strconv.Itoa(maxSecs),
		"rec", "-q", "-c", "1", "-r", "16000", v.WavPath,
		"silence", "1", "0.1", thresh, "1", trail, thresh)
	if timing {
		fmt.Fprintf(os.Stderr, "⏱ rec(grabación+VAD): %v\n", time.Since(tRec).Round(time.Millisecond))
	}
	if recErr != nil {
		// 'timeout' devuelve 124 al cortar por el tope: en la ventana de conversación = no hablaste,
		// o ruido constante que nunca dispara el silencio. NO es fatal → captura vacía (el llamador
		// decide dormir / "no te escuché"). Cualquier otro error de rec sí es real.
		if isTimeout(recErr) {
			return "", nil
		}
		return "", fmt.Errorf("no pude grabar (¿sox/rec + timeout/coreutils + PipeWire?): %w", recErr)
	}
	// whisper escribe la transcripción a <of>.txt (-nt: sin timestamps).
	ofPrefix := strings.TrimSuffix(v.WavPath, ".wav")
	tW := time.Now()
	if _, err := v.Runner.Run(v.WhisperBin, "-m", v.WhisperModel, "-f", v.WavPath,
		"-l", "es", "-nt", "-otxt", "-of", ofPrefix); err != nil {
		return "", fmt.Errorf("no pude transcribir (¿whisper/modelo?): %w", err)
	}
	if timing {
		fmt.Fprintf(os.Stderr, "⏱ whisper: %v\n", time.Since(tW).Round(time.Millisecond))
	}
	readFile := v.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	raw, err := readFile(ofPrefix + ".txt")
	if err != nil {
		return "", fmt.Errorf("no pude leer la transcripción: %w", err)
	}
	text := cleanTranscript(string(raw))
	fmt.Printf("🗣️  entendí: %q\n", text) // feedback: qué transcribió Whisper (debug + UX)
	return text, nil
}

// isTimeout indica si el error viene de `timeout` cortando por el tope (exit 124) — esperado (fin de
// ventana sin voz / ruido constante), NO una falla real de grabación.
func isTimeout(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == 124
}

// isRealUtterance decide si una transcripción es habla real y no vacío/ruido/alucinación. Whisper
// sobre silencio devuelve "" o inventa muletillas ("Gracias", "Adiós", "Subtítulos…"); esto evita
// mandar basura al LLM y mantener abierta la conversación por ruido. Reusa normalize (interpreter.go).
func isRealUtterance(text string) bool {
	t := normalize(text) // minúsculas, sin acentos, sin puntuación de cola
	if t == "" {
		return false
	}
	if strings.Contains(t, "blank_audio") || strings.Contains(t, "silenc") {
		return false // marcadores de no-habla de whisper (ej "[BLANK_AUDIO]", "[silencio]")
	}
	junk := map[string]bool{
		"gracias": true, "adios": true, "muchas gracias": true, "subtitulos": true,
		"gracias por ver": true, "gracias por ver el video": true,
	}
	return !junk[t]
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
