package main

import "testing"

func TestExecRunnerRunsCommand(t *testing.T) {
	out, err := ExecRunner{}.Run("echo", "hola")
	if err != nil {
		t.Fatalf("no debería fallar: %v", err)
	}
	if out != "hola" {
		t.Fatalf("esperaba %q, obtuve %q", "hola", out)
	}
}
