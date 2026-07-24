package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// WakeWordInput espera a que un sidecar (motor de wake-word) emita "DETECTED" por stdout y ahí
// captura el comando reusando la grabación por voz. Implementa InputSource.
type WakeWordInput struct {
	events *bufio.Scanner // stdout del sidecar: una línea por activación
	voice  *VoiceInput    // captura el comando tras el wake (VAD + whisper)
	cmd    *exec.Cmd      // el sidecar, para cerrarlo (nil si se inyectó el scanner en tests)
}

// NewWakeWordInput lanza el sidecar (wakeCmd, ej. "python .../wake.py") vía sh -c (permite flags)
// y toma su stdout como stream de eventos. El sidecar es long-lived → os/exec directo, no Runner.
func NewWakeWordInput(wakeCmd string, voice *VoiceInput) (*WakeWordInput, error) {
	if wakeCmd == "" {
		return nil, fmt.Errorf("modo wake: falta ASTRO_WAKE_CMD (comando del sidecar)")
	}
	cmd := exec.Command("sh", "-c", wakeCmd)
	// Grupo de procesos propio: el sidecar suele ser un pipeline (grabador | python), y así
	// Close() puede matar TODO el grupo de una (ver Close). Sin esto, matar el sh dejaría
	// huérfanos a los hijos reteniendo el micrófono.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = os.Stderr // los logs del sidecar (stderr) quedan visibles
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("no pude tomar el stdout del sidecar: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no pude lanzar el sidecar de wake (%q): %w", wakeCmd, err)
	}
	return &WakeWordInput{events: bufio.NewScanner(out), voice: voice, cmd: cmd}, nil
}

func (w *WakeWordInput) Listen() (string, error) {
	fmt.Println("💤 esperando \"Astro\"…")
	if !w.events.Scan() {
		if err := w.events.Err(); err != nil {
			return "", err
		}
		// stdout cerrado = el sidecar murió. NO es un quit limpio (a diferencia de Ctrl+D en stdin):
		// es una falla → error descriptivo para que main lo reporte, no io.EOF silencioso.
		return "", fmt.Errorf("el sidecar de wake murió (cerró su stdout)")
	}
	fmt.Println("👂 ¡te escucho!")
	return w.voice.capture()
}

// Close mata el sidecar y lo cosecha. main lo llama con defer y en el handler de señal.
func (w *WakeWordInput) Close() error {
	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}
	// Matamos el GRUPO entero (-pid), no solo al sh: así también mueren el grabador y el python
	// del pipeline. El sh es líder del grupo por el Setpgid de NewWakeWordInput.
	_ = syscall.Kill(-w.cmd.Process.Pid, syscall.SIGKILL)
	return w.cmd.Wait() // cosecha el zombie + cierra el pipe (completa el contrato de Start/StdoutPipe)
}
