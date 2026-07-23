package main

// fakeRunner registra las llamadas en vez de ejecutarlas: verifica QUÉ comando se
// habría corrido, sin efectos reales.
type fakeRunner struct {
	calls  [][]string
	inputs []string // stdin de cada RunWithInput
	output string
	err    error
}

func (f *fakeRunner) Run(name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.output, f.err
}

func (f *fakeRunner) lastCall() []string {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeRunner) RunWithInput(stdin string, name string, args ...string) (string, error) {
	f.inputs = append(f.inputs, stdin)
	return f.Run(name, args...)
}

func (f *fakeRunner) lastInput() string {
	if len(f.inputs) == 0 {
		return ""
	}
	return f.inputs[len(f.inputs)-1]
}
