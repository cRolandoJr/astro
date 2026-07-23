package main

import "fmt"

// Display muestra la cara. Interfaz porque tendrá varias implementaciones: hoy EwwFace
// (pantalla), mañana ESP32Display (robot). Mismo contrato. (Ver nota de diseño del plan:
// la mantenemos como interfaz por aprendizaje; en Go se podría extraer gratis después.)
type Display interface {
	Show(expr Expression) error
}

// EwwFace actualiza una variable de eww para que el widget muestre faces/<expr>.png.
// ConfigDir apunta a la config de eww del repo; si está vacío, usa la default de eww.
type EwwFace struct {
	Runner    Runner
	ConfigDir string
}

func (e EwwFace) Show(expr Expression) error {
	args := []string{"update", fmt.Sprintf("astro_face=%s", expr)}
	if e.ConfigDir != "" {
		args = append([]string{"--config", e.ConfigDir}, args...)
	}
	if _, err := e.Runner.Run("eww", args...); err != nil {
		return fmt.Errorf("no pude actualizar la cara: %w", err)
	}
	return nil
}
