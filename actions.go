package main

import (
	"fmt"
	"time"
)

// buildActions arma el registro. 'now' inyectado para testear la hora.
func buildActions(now func() time.Time) map[string]*Action {
	actions := map[string]*Action{}
	add := func(a *Action) { actions[a.Name] = a }

	add(&Action{Name: "saludar", Face: Feliz,
		Run: func(r Runner) (string, error) { return "¡Hola! Soy Astro.", nil }})
	add(&Action{Name: "hora", Face: Neutral,
		Run: func(r Runner) (string, error) { return "Son las " + now().Format("15:04") + ".", nil }})
	add(&Action{Name: "dormir", Face: Dormido,
		Run: func(r Runner) (string, error) { return "Me duermo… 💤", nil }})
	add(&Action{Name: "despertar", Face: Neutral,
		Run: func(r Runner) (string, error) { return "¡Ya estoy despierto!", nil }})

	addDesktopActions(add) // definidas en Task 4 (mismo archivo)
	return actions
}

// addDesktopActions registra las acciones que tocan el escritorio.
func addDesktopActions(add func(*Action)) {
	add(&Action{Name: "pausar", Face: Feliz, Run: playerctl("play-pause", "Listo.")})
	add(&Action{Name: "siguiente", Face: Feliz, Run: playerctl("next", "Siguiente.")})
	add(&Action{Name: "subir_volumen", Face: Neutral, Run: volume("5%+", "Subí el volumen.")})
	add(&Action{Name: "bajar_volumen", Face: Neutral, Run: volume("5%-", "Bajé el volumen.")})
	add(&Action{Name: "mute", Face: Neutral, Run: func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle"); err != nil {
			return "", fmt.Errorf("no pude silenciar: %w", err)
		}
		return "Mute.", nil
	}})
}

func playerctl(cmd, ok string) func(Runner) (string, error) {
	return func(r Runner) (string, error) {
		if _, err := r.Run("playerctl", cmd); err != nil {
			return "", fmt.Errorf("no pude controlar la música: %w", err)
		}
		return ok, nil
	}
}

func volume(delta, ok string) func(Runner) (string, error) {
	return func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", delta); err != nil {
			return "", fmt.Errorf("no pude cambiar el volumen: %w", err)
		}
		return ok, nil
	}
}

// openAppAction arma una acción al vuelo que abre 'app'. Usa `hyprctl dispatch exec` porque
// no bloquea y respeta tus reglas de Hyprland (igual que tus keybinds).
func openAppAction(app string) *Action {
	return &Action{Name: "abrir_" + app, Face: Curioso, Run: func(r Runner) (string, error) {
		if _, err := r.Run("hyprctl", "dispatch", "exec", app); err != nil {
			return "", fmt.Errorf("no pude abrir %s: %w", app, err)
		}
		return "Abriendo " + app + ".", nil
	}}
}
