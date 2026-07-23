package main

import (
	"os/exec"
	"strings"
)

// Runner ejecuta comandos del sistema. Interfaz para inyectar un fake en tests y NO
// abrir programas de verdad al testear.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// ExecRunner corre comandos reales. CombinedOutput junta stdout+stderr y espera el fin.
type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
