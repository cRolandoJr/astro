package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// buildActions arma el registro. 'now' inyectado para testear la hora.
func buildActions(now func() time.Time) map[string]*Action {
	actions := map[string]*Action{}
	add := func(a *Action) { actions[a.Name] = a }

	add(&Action{Name: "saludar", Desc: "saludar o responder un saludo", Face: Feliz,
		Run: func(r Runner) (string, error) { return "¡Hola! Soy Astro.", nil }})
	add(&Action{Name: "hora", Desc: "decir la hora actual", Face: Neutral,
		Run: func(r Runner) (string, error) { return "Son las " + now().Format("15:04") + ".", nil }})
	add(&Action{Name: "dormir", Desc: "poner a Astro a dormir", Face: Dormido,
		Run: func(r Runner) (string, error) { return "Me duermo… 💤", nil }})
	add(&Action{Name: "despertar", Desc: "despertar a Astro", Face: Neutral,
		Run: func(r Runner) (string, error) { return "¡Ya estoy despierto!", nil }})

	addDesktopActions(add) // definidas en Task 4 (mismo archivo)
	addInfoActions(add, now)
	return actions
}

// addDesktopActions registra las acciones que tocan el escritorio.
func addDesktopActions(add func(*Action)) {
	add(&Action{Name: "pausar", Desc: "pausar o reanudar la música o el video", Face: Feliz, Run: playerctl("play-pause", "Listo.")})
	add(&Action{Name: "siguiente", Desc: "pasar a la siguiente pista", Face: Feliz, Run: playerctl("next", "Siguiente.")})
	add(&Action{Name: "subir_volumen", Desc: "subir el volumen", Face: Neutral, Run: volume("5%+", "Subí el volumen.")})
	add(&Action{Name: "bajar_volumen", Desc: "bajar el volumen", Face: Neutral, Run: volume("5%-", "Bajé el volumen.")})
	add(&Action{Name: "mute", Desc: "silenciar o quitar el silencio", Face: Neutral, Run: func(r Runner) (string, error) {
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

// addInfoActions registra tools de media/sistema/info (no destructivas).
func addInfoActions(add func(*Action), now func() time.Time) {
	add(&Action{Name: "anterior", Desc: "volver a la pista anterior", Face: Feliz, Run: playerctl("previous", "Anterior.")})

	add(&Action{Name: "que_suena", Desc: "decir qué está sonando", Face: Curioso, Run: func(r Runner) (string, error) {
		out, err := r.Run("playerctl", "metadata", "--format", "{{artist}} - {{title}}")
		out = strings.TrimSpace(out)
		if err != nil || out == "" || out == "-" {
			return "No hay nada sonando.", nil
		}
		return "Está sonando: " + out + ".", nil
	}})

	add(&Action{Name: "silenciar_micro", Desc: "silenciar o activar el micrófono", Face: Neutral, Run: func(r Runner) (string, error) {
		if _, err := r.Run("wpctl", "set-mute", "@DEFAULT_AUDIO_SOURCE@", "toggle"); err != nil {
			return "", fmt.Errorf("no pude tocar el micrófono: %w", err)
		}
		return "Micrófono cambiado.", nil
	}})

	add(&Action{Name: "bateria", Desc: "decir el porcentaje de batería", Face: Neutral, Run: func(r Runner) (string, error) {
		devs, err := r.Run("upower", "-e")
		if err != nil {
			return "", fmt.Errorf("no pude leer la batería: %w", err)
		}
		dev := ""
		for _, l := range strings.Split(devs, "\n") {
			if strings.Contains(l, "battery") {
				dev = strings.TrimSpace(l)
				break
			}
		}
		if dev == "" {
			return "No encontré la batería.", nil
		}
		info, err := r.Run("upower", "-i", dev)
		if err != nil {
			return "", fmt.Errorf("no pude leer la batería: %w", err)
		}
		for _, l := range strings.Split(info, "\n") {
			if strings.Contains(l, "percentage:") {
				return "Batería al " + strings.TrimSpace(strings.SplitN(l, ":", 2)[1]) + ".", nil
			}
		}
		return "No pude leer el porcentaje.", nil
	}})

	add(&Action{Name: "fecha", Desc: "decir el día y la fecha de hoy", Face: Neutral, Run: func(r Runner) (string, error) {
		return "Hoy es " + spanishDate(now()) + ".", nil
	}})

	add(&Action{Name: "clima", Desc: "decir el clima actual", Face: Curioso, Run: func(r Runner) (string, error) {
		// timeouts: clima es la única tool de red; sin -m/--connect-timeout un curl estancado
		// colgaría el único goroutine del daemon (no procesaría más voz).
		out, err := r.Run("curl", "-s", "-m", "10", "--connect-timeout", "5", "wttr.in/?format=%C+%t")
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			return "No pude ver el clima.", nil
		}
		return "El clima: " + out + ".", nil
	}})

	add(&Action{Name: "agenda", Desc: "decir el próximo evento de la agenda", Face: Curioso, Run: func(r Runner) (string, error) {
		// --day-format "" suprime el encabezado de día de khal; tomamos la 1ª línea NO vacía.
		out, err := r.Run("khal", "list", "now", "24h", "--day-format", "", "--format", "{start-time} {title}")
		if err != nil {
			return "No tenés nada en la agenda por ahora.", nil
		}
		for _, l := range strings.Split(out, "\n") {
			if s := strings.TrimSpace(l); s != "" {
				return "Próximo: " + s + ".", nil
			}
		}
		return "No tenés nada en la agenda por ahora.", nil
	}})

	add(&Action{Name: "bloquear", Desc: "bloquear la pantalla", Face: Dormido, Run: func(r Runner) (string, error) {
		if _, err := r.Run("hyprlock"); err != nil {
			return "", fmt.Errorf("no pude bloquear: %w", err)
		}
		return "Bloqueando.", nil
	}})
}

// spanishDate formatea la fecha en español (time.Format no localiza).
func spanishDate(t time.Time) string {
	dias := []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	meses := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("%s %d de %s", dias[int(t.Weekday())], t.Day(), meses[int(t.Month())-1])
}

// ArgTool es una tool que necesita un argumento (el LLM lo pasa en `arg`). Build liga el arg y
// devuelve la acción. Van en un mapa aparte porque el registro normal no lleva arg.
type ArgTool struct {
	Desc  string
	Build func(arg string) *Action
}

// buildArgActions arma las tools con arg (todas por Runner/argv → sin shell, sin inyección).
func buildArgActions() map[string]*ArgTool {
	return map[string]*ArgTool{
		"volumen_a": {Desc: "poner el volumen en un valor 0-150 (arg = número)", Build: func(arg string) *Action {
			return &Action{Name: "volumen_a", Face: Neutral, Run: func(r Runner) (string, error) {
				n, err := strconv.Atoi(strings.TrimSpace(arg))
				if err != nil || n < 0 || n > 150 {
					return "", fmt.Errorf("volumen inválido: %q", arg)
				}
				if _, err := r.Run("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", strconv.Itoa(n)+"%"); err != nil {
					return "", fmt.Errorf("no pude cambiar el volumen: %w", err)
				}
				return "Volumen al " + strconv.Itoa(n) + " por ciento.", nil
			}}
		}},
		"brillo": {Desc: "ajustar el brillo (arg = subir, bajar, o un número 0-100)", Build: func(arg string) *Action {
			return &Action{Name: "brillo", Face: Neutral, Run: func(r Runner) (string, error) {
				var val string
				switch strings.TrimSpace(arg) {
				case "subir":
					val = "+10%"
				case "bajar":
					val = "10%-"
				default:
					n, err := strconv.Atoi(strings.TrimSpace(arg))
					if err != nil || n < 0 || n > 100 {
						return "", fmt.Errorf("brillo inválido: %q", arg)
					}
					val = strconv.Itoa(n) + "%"
				}
				if _, err := r.Run("brightnessctl", "set", val); err != nil {
					return "", fmt.Errorf("no pude cambiar el brillo: %w", err)
				}
				return "Brillo ajustado.", nil
			}}
		}},
		"abrir_url": {Desc: "abrir una URL en el navegador (arg = la url)", Build: func(arg string) *Action {
			return &Action{Name: "abrir_url", Face: Curioso, Run: func(r Runner) (string, error) {
				u := strings.TrimSpace(arg)
				if u == "" {
					return "", fmt.Errorf("no dijiste qué URL")
				}
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
					u = "https://" + u
				}
				if _, err := r.Run("xdg-open", u); err != nil {
					return "", fmt.Errorf("no pude abrir la URL: %w", err)
				}
				return "Abriendo la página.", nil
			}}
		}},
		"buscar": {Desc: "buscar algo en el navegador (arg = qué buscar)", Build: func(arg string) *Action {
			return &Action{Name: "buscar", Face: Curioso, Run: func(r Runner) (string, error) {
				q := strings.TrimSpace(arg)
				if q == "" {
					return "", fmt.Errorf("no dijiste qué buscar")
				}
				if _, err := r.Run("xdg-open", "https://duckduckgo.com/?q="+url.QueryEscape(q)); err != nil {
					return "", fmt.Errorf("no pude buscar: %w", err)
				}
				return "Buscando " + q + ".", nil
			}}
		}},
		"ir_a_workspace": {Desc: "cambiar de escritorio/workspace (arg = número 1-10)", Build: func(arg string) *Action {
			return &Action{Name: "ir_a_workspace", Face: Neutral, Run: func(r Runner) (string, error) {
				n, err := strconv.Atoi(strings.TrimSpace(arg))
				if err != nil || n < 1 || n > 10 {
					return "", fmt.Errorf("workspace inválido: %q", arg)
				}
				if _, err := r.Run("hyprctl", "dispatch", "workspace", strconv.Itoa(n)); err != nil {
					return "", fmt.Errorf("no pude cambiar de workspace: %w", err)
				}
				return "Workspace " + strconv.Itoa(n) + ".", nil
			}}
		}},
	}
}

// openAppAction arma una acción al vuelo que abre 'app'. Usa `hyprctl dispatch exec` porque
// no bloquea y respeta tus reglas de Hyprland (igual que tus keybinds).
func openAppAction(app string) *Action {
	return &Action{Name: "abrir_" + app, Desc: "abrir " + app, Face: Curioso, Run: func(r Runner) (string, error) {
		if _, err := r.Run("hyprctl", "dispatch", "exec", app); err != nil {
			return "", fmt.Errorf("no pude abrir %s: %w", app, err)
		}
		return "Abriendo " + app + ".", nil
	}}
}
