package main

import (
	"os/exec"
	"strings"
)

// Runner ejecuta comandos del sistema. Interfaz para inyectar un fake en tests y NO
// abrir programas de verdad al testear.
type Runner interface {
	Run(name string, args ...string) (string, error)
	RunWithInput(stdin string, name string, args ...string) (string, error)
}

// ExecRunner corre comandos reales. CombinedOutput junta stdout+stderr y espera el fin.
type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunWithInput corre el comando alimentando 'stdin' por la entrada estándar (para
// binarios como piper que leen el texto por stdin).
func (ExecRunner) RunWithInput(stdin string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
