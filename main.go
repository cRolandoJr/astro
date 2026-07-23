package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"
)

func main() {
	runner := ExecRunner{}
	var display Display = EwwFace{Runner: runner, ConfigDir: os.Getenv("ASTRO_EWW_CONFIG")}
	var interpreter Interpreter = NewRuleInterpreter(buildActions(time.Now))

	_ = display.Show(Neutral)

	fmt.Println("Astro está despierto. Escribí un comando (Ctrl+D para salir).")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		text := scanner.Text()
		if text == "" {
			continue
		}

		action, err := interpreter.Interpret(text)
		if errors.Is(err, ErrNoEntiendo) {
			_ = display.Show(Pensativo)
			fmt.Println("No te entendí. Probá: hola · qué hora es · pausá · siguiente · subí/bajá/silencio · abrí firefox · dormí · despertá")
			continue
		}

		reply, err := action.Run(runner)
		if err != nil {
			_ = display.Show(Neutral)
			fmt.Println("Ups:", err)
			continue
		}
		_ = display.Show(action.Face)
		fmt.Println(reply)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error leyendo stdin:", err)
	}
	fmt.Println("\nChau 👋")
}
