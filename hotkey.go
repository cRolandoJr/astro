package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
)

// HotkeyInput espera un disparo por un FIFO (lo escribe un bind de Hyprland) y ahí captura el
// comando reusando la grabación por voz. Implementa InputSource.
type HotkeyInput struct {
	events *bufio.Scanner // líneas del FIFO: una por activación del atajo
	voice  *VoiceInput    // captura el comando tras el disparo (VAD + whisper)
	fifo   *os.File       // el FIFO abierto O_RDWR (nil si se inyectó el scanner en tests)
	path   string         // ruta del FIFO, para borrarlo en Close
}

// NewHotkeyInput crea el FIFO si falta y lo abre O_RDWR. Abrir con lectura+escritura mantiene un
// escritor propio abierto → el Scanner nunca recibe EOF entre disparos (el FIFO no se "termina").
func NewHotkeyInput(fifoPath string, voice *VoiceInput) (*HotkeyInput, error) {
	if fifoPath == "" {
		return nil, fmt.Errorf("modo hotkey: falta ASTRO_TRIGGER_FIFO (ruta del FIFO)")
	}
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("no pude crear el FIFO %q: %w", fifoPath, err)
	}
	f, err := os.OpenFile(fifoPath, os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("no pude abrir el FIFO %q: %w", fifoPath, err)
	}
	return &HotkeyInput{events: bufio.NewScanner(f), voice: voice, fifo: f, path: fifoPath}, nil
}

func (h *HotkeyInput) Listen() (string, error) {
	fmt.Println("⌨️  esperando el atajo (SUPER+SHIFT+A)…")
	if !h.events.Scan() {
		if err := h.events.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("el FIFO de disparo se cerró")
	}
	fmt.Println("👂 ¡te escucho!")
	return h.voice.capture()
}

// Close cierra el FIFO y lo borra. main lo llama con defer y en el handler de señal. Nil-safe.
func (h *HotkeyInput) Close() error {
	if h.fifo != nil {
		_ = h.fifo.Close()
	}
	if h.path != "" {
		_ = os.Remove(h.path)
	}
	return nil
}
