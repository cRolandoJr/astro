package main

import "fmt"

// Display muestra la cara. Interfaz porque tendrá varias implementaciones: hoy EwwFace
// (pantalla), mañana ESP32Display (robot). Mismo contrato. (Ver nota de diseño del plan:
// la mantenemos como interfaz por aprendizaje; en Go se podría extraer gratis después.)
type Display interface {
	Open() error // muestra la ventana de la cara (idle = cerrada)
	Show(expr Expression) error
	Close() error // oculta la ventana (al terminar el turno / al salir)
}

// EwwFace actualiza una variable de eww para que el widget muestre faces/<expr>.png.
// ConfigDir apunta a la config de eww del repo; si está vacío, usa la default de eww.
type EwwFace struct {
	Runner    Runner
	ConfigDir string
}

func (e EwwFace) Show(expr Expression) error {
	return e.eww("update", fmt.Sprintf("astro_face=%s", expr))
}

// Open/Close manejan la ventana del widget. Astro la abre al arrancar y la cierra al
// salir, así la cara no queda colgada (eww corre en su propio daemon, aparte del proceso).
func (e EwwFace) Open() error  { return e.eww("open", "astro") }
func (e EwwFace) Close() error { return e.eww("close", "astro") }

// eww corre un subcomando de eww, anteponiendo --config <dir> si está seteado.
// Factoriza el --config que comparten Show/Open/Close.
func (e EwwFace) eww(args ...string) error {
	if e.ConfigDir != "" {
		args = append([]string{"--config", e.ConfigDir}, args...)
	}
	if _, err := e.Runner.Run("eww", args...); err != nil {
		return fmt.Errorf("eww falló: %w", err)
	}
	return nil
}
