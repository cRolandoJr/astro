package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fakeChat(reply string, err error) chatFunc {
	return func(system string, history []Exchange, user string) (string, error) { return reply, err }
}

func newLLM(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts)})
}

// fakeMemory: MemoryStore de test. Registra lo guardado y devuelve un Recall fijo.
type fakeMemory struct {
	remembered []string
	recall     []string
	recallErr  error
}

func (f *fakeMemory) Remember(text string) error {
	f.remembered = append(f.remembered, text)
	return nil
}
func (f *fakeMemory) Recall(query string, k int) ([]string, error) {
	return f.recall, f.recallErr
}

func newLLMWithMem(chat chatFunc, mem MemoryStore) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts), Mem: mem})
}

func newLLMArg(chat chatFunc) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		ArgActions: buildArgActions()})
}

func TestLLMArgToolVolumen(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLMArg(fakeChat(`{"action":"volumen_a","arg":"30","say":"Dale, al 30."}`, nil)).Interpret("poné el volumen en 30")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Dale, al 30." {
		t.Fatalf("esperaba el say, fue %q", reply)
	}
	want := []string{"wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "30%"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

func TestLLMArgToolVolumenInvalido(t *testing.T) {
	a, err := newLLMArg(fakeChat(`{"action":"volumen_a","arg":"altísimo","say":"ok"}`, nil)).Interpret("subilo a full")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	if _, err := a.Run(&fakeRunner{}); err == nil {
		t.Fatal("un arg no numérico debía dar error al ejecutar")
	}
}

func TestLLMRecordarGuardaElHecho(t *testing.T) {
	mem := &fakeMemory{}
	a, err := newLLMWithMem(fakeChat(`{"action":"recordar","arg":"uso NixOS","say":"Dale, anotado."}`, nil), mem).
		Interpret("acordate que uso NixOS")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(&fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Dale, anotado." {
		t.Fatalf("esperaba el say, fue %q", reply)
	}
	if len(mem.remembered) != 1 || mem.remembered[0] != "uso NixOS" {
		t.Fatalf("esperaba guardar 'uso NixOS', fue %v", mem.remembered)
	}
}

func TestLLMRecordarSinMemVaAFallback(t *testing.T) {
	// mem nil → 'recordar' no disponible → fallback (reglas → ErrNoEntiendo).
	a, err := newLLM(fakeChat(`{"action":"recordar","arg":"algo","say":"ok"}`, nil)).Interpret("xyzzy")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("sin memoria, recordar debe caer al fallback; a=%v err=%v", a, err)
	}
}

func TestLLMInyectaHechosAlPrompt(t *testing.T) {
	mem := &fakeMemory{recall: []string{"el usuario usa NixOS"}}
	var seenSystem string
	chat := func(system string, history []Exchange, user string) (string, error) {
		seenSystem = system
		return `{"action":"none","say":"ok"}`, nil
	}
	if _, err := newLLMWithMem(chat, mem).Interpret("qué distro uso"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenSystem, "el usuario usa NixOS") {
		t.Fatalf("el prompt debía incluir el hecho recuperado; prompt=%q", seenSystem)
	}
}

func TestLLMRecallErrorNoRompeElTurno(t *testing.T) {
	mem := &fakeMemory{recallErr: fmt.Errorf("embed caído")}
	a, err := newLLMWithMem(fakeChat(`{"action":"none","say":"hola"}`, nil), mem).Interpret("hola")
	if err != nil || a == nil {
		t.Fatalf("un fallo de Recall no debe romper el turno; a=%v err=%v", a, err)
	}
}

// #3 (mentor): recordar con arg vacío no debe guardar un hecho vacío (no matchea → fallback).
func TestLLMRecordarArgVacioNoGuarda(t *testing.T) {
	mem := &fakeMemory{}
	_, _ = newLLMWithMem(fakeChat(`{"action":"recordar","arg":"","say":"ok"}`, nil), mem).Interpret("acordate")
	if len(mem.remembered) != 0 {
		t.Fatalf("no debe guardar un hecho vacío; guardó %v", mem.remembered)
	}
}

func TestLLMMapeaAccionDelRegistro(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"pausar"}`, nil)).Interpret("che poné pausa")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba pausar; a=%v err=%v", a, err)
	}
}

func TestLLMOpenAbreLaApp(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox"}`, nil)).Interpret("abrí el navegador")
	if err != nil || a == nil {
		t.Fatalf("esperaba acción; a=%v err=%v", a, err)
	}
	if _, err := a.Run(fake); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := []string{"hyprctl", "dispatch", "exec", "firefox"}
	if got := fake.lastCall(); !reflect.DeepEqual(got, want) {
		t.Fatalf("esperaba %v, fue %v", want, got)
	}
}

// Seguridad: aunque el LLM devuelva un arg peligroso, isSafeAppName lo frena → fallback → ErrNoEntiendo.
func TestLLMOpenInyeccionRechazada(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"open","arg":"firefox; rm -rf ~"}`, nil)).Interpret("abrí firefox; rm -rf ~")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("el arg peligroso debía rechazarse; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiChatFalla(t *testing.T) {
	a, err := newLLM(fakeChat("", fmt.Errorf("sin red"))).Interpret("pausá")
	if err != nil || a == nil || a.Name != "pausar" {
		t.Fatalf("esperaba fallback a reglas → pausar; a=%v err=%v", a, err)
	}
}

func TestLLMFallbackSiJSONInvalido(t *testing.T) {
	a, err := newLLM(fakeChat("no soy json", nil)).Interpret("hola")
	if err != nil || a == nil || a.Name != "saludar" {
		t.Fatalf("esperaba fallback → saludar; a=%v err=%v", a, err)
	}
}

func TestLLMActionConSayHablaElSay(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"pausar","say":"Dale, te pausé."}`, nil)).Interpret("poné pausa")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Dale, te pausé." {
		t.Fatalf("esperaba el say del LLM, fue %q", reply)
	}
	if got := fake.lastCall(); len(got) < 1 || got[0] != "playerctl" {
		t.Fatalf("esperaba que corriera playerctl, fue %v", got)
	}
}

func TestWrapInfoHablaSuDatoNoElSay(t *testing.T) {
	// Acción de info (Speaks): debe hablar SU salida (dato real), ignorando el say inventado del LLM.
	info := &Action{Name: "fecha", Speaks: true, Run: func(r Runner) (string, error) { return "Hoy es lunes.", nil }}
	if got, _ := wrap(info, "Hoy es viernes 28 de marzo de 2025.").Run(&fakeRunner{}); got != "Hoy es lunes." {
		t.Errorf("acción Speaks debe hablar su dato real, no el say del LLM; fue %q", got)
	}
	// Acción de efecto (no Speaks): habla el say del LLM (más natural).
	fx := &Action{Name: "pausar", Run: func(r Runner) (string, error) { return "Listo.", nil }}
	if got, _ := wrap(fx, "Dale, te pausé.").Run(&fakeRunner{}); got != "Dale, te pausé." {
		t.Errorf("acción de efecto debe hablar el say; fue %q", got)
	}
}

func TestWithMoodPoneLaCara(t *testing.T) {
	if a := withMood(sayAction("hola"), "triste"); a.Face != Triste {
		t.Errorf("mood 'triste' → Face=%v, quiero Triste", a.Face)
	}
	base := &Action{Face: Neutral}
	if withMood(base, "").Face != Neutral || withMood(base, "desconocido").Face != Neutral {
		t.Error("mood vacío/desconocido no debe cambiar la cara")
	}
}

func TestLLMCharlaUsaElMood(t *testing.T) {
	a, err := newLLM(fakeChat(`{"action":"none","say":"perdón","mood":"triste"}`, nil)).Interpret("sos un desastre")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	if a.Face != Triste {
		t.Errorf("la charla debe reflejar el mood del LLM: Face=%v, quiero Triste", a.Face)
	}
}

func TestLLMCharlaSoloHabla(t *testing.T) {
	fake := &fakeRunner{}
	a, err := newLLM(fakeChat(`{"action":"none","say":"Estoy bien, ¿y vos?"}`, nil)).Interpret("cómo estás")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if reply != "Estoy bien, ¿y vos?" {
		t.Fatalf("esperaba la charla, fue %q", reply)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("la charla no debe ejecutar comandos, hubo %v", fake.calls)
	}
}

func TestLLMSayPuroSinAction(t *testing.T) {
	a, err := newLLM(fakeChat(`{"say":"un chiste corto"}`, nil)).Interpret("contame un chiste")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, _ := a.Run(&fakeRunner{})
	if reply != "un chiste corto" {
		t.Fatalf("fue %q", reply)
	}
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newLLMClock(chat chatFunc, clock *fakeClock) *LLMInterpreter {
	acts := buildActions(time.Now)
	return NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		Now: clock.now, IdleWindow: 5 * time.Minute})
}

func TestLLMPasaHistorialAlSegundoTurno(t *testing.T) {
	var seen []Exchange
	chat := func(system string, history []Exchange, user string) (string, error) {
		seen = history
		return `{"action":"none","say":"ok"}`, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(chat, clock)
	li.Interpret("subí el volumen") // turno 1: historial vacío
	clock.t = clock.t.Add(10 * time.Second)
	li.Interpret("un poco más") // turno 2: debe ver el turno 1
	if len(seen) != 1 || seen[0].User != "subí el volumen" || seen[0].Assistant != "ok" {
		t.Fatalf("el 2do turno debía ver el 1ro; fue %v", seen)
	}
}

func TestLLMResetPorInactividad(t *testing.T) {
	var seen []Exchange
	chat := func(system string, history []Exchange, user string) (string, error) {
		seen = history
		return `{"action":"none","say":"ok"}`, nil
	}
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(chat, clock)
	li.Interpret("primero")
	clock.t = clock.t.Add(6 * time.Minute) // pasó el umbral de 5 min
	li.Interpret("segundo")                // → reset, historial vacío
	if len(seen) != 0 {
		t.Fatalf("tras >5min el historial se resetea; fue %v", seen)
	}
}

func TestLLMHistorialRecortaAVentana(t *testing.T) {
	chat := func(system string, history []Exchange, user string) (string, error) {
		return `{"action":"none","say":"ok"}`, nil
	}
	acts := buildActions(time.Now)
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := NewLLMInterpreter(LLMConfig{Chat: chat, Actions: acts, Fallback: NewRuleInterpreter(acts),
		Now: clock.now, HistoryTurns: 2})
	for i := 0; i < 4; i++ {
		li.Interpret("frase")
	}
	if len(li.history) != 2 {
		t.Fatalf("la ventana debía recortar a 2 turnos, fue %d", len(li.history))
	}
}

func TestLLMNoRegistraTurnoRechazado(t *testing.T) {
	// open con arg peligroso → cae al fallback → NO debe quedar en el historial (spec §4).
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(fakeChat(`{"action":"open","arg":"firefox; rm -rf ~","say":"abriendo"}`, nil), clock)
	li.Interpret("abrí firefox; rm -rf ~")
	if len(li.history) != 0 {
		t.Fatalf("un turno rechazado por seguridad no debe registrarse; historial=%v", li.history)
	}
}

func TestLLMNoRegistraComandoSinSay(t *testing.T) {
	// Comando sin say: Astro habla la frase enlatada, pero no hay respuesta que registrar →
	// el historial no debe guardar un turno con asistente vacío.
	clock := &fakeClock{t: time.Unix(1000, 0)}
	li := newLLMClock(fakeChat(`{"action":"pausar"}`, nil), clock)
	li.Interpret("pausá")
	if len(li.history) != 0 {
		t.Fatalf("un comando sin say no debe registrar turno vacío; historial=%v", li.history)
	}
}

func TestLLMMirarCapturaYPregunta(t *testing.T) {
	fake := &fakeRunner{}
	var gotQuestion, gotPath string
	vision := func(question, imagePath string) (string, error) {
		gotQuestion, gotPath = question, imagePath
		return "Veo una terminal.", nil
	}
	acts := buildActions(time.Now)
	li := NewLLMInterpreter(LLMConfig{Chat: fakeChat(`{"action":"mirar","arg":"HDMI-A-1"}`, nil),
		Actions: acts, Fallback: NewRuleInterpreter(acts), Vision: vision})
	a, err := li.Interpret("mirá el de la derecha, ¿qué ves?")
	if err != nil || a == nil {
		t.Fatalf("a=%v err=%v", a, err)
	}
	reply, err := a.Run(fake)
	if err != nil {
		t.Fatal(err)
	}
	if reply != "Veo una terminal." {
		t.Fatalf("esperaba la respuesta de visión, fue %q", reply)
	}
	wantGrim := []string{"grim", "-o", "HDMI-A-1", "/tmp/astro-screen.png"}
	if !reflect.DeepEqual(fake.calls[0], wantGrim) {
		t.Fatalf("grim mal armado:\n esperaba %v\n fue      %v", wantGrim, fake.calls[0])
	}
	if gotQuestion != "mirá el de la derecha, ¿qué ves?" {
		t.Fatalf("la pregunta a visión debía ser la frase del usuario, fue %q", gotQuestion)
	}
	if gotPath != "/tmp/astro-screen.png" {
		t.Fatalf("path a visión: %q", gotPath)
	}
	if len(li.history) != 0 {
		t.Fatalf("mirar es stateless, no debe registrar historial; fue %v", li.history)
	}
}

func TestLLMMirarSinOutputCapturaTodo(t *testing.T) {
	fake := &fakeRunner{}
	vision := func(question, imagePath string) (string, error) { return "ok", nil }
	acts := buildActions(time.Now)
	li := NewLLMInterpreter(LLMConfig{Chat: fakeChat(`{"action":"mirar","arg":""}`, nil),
		Actions: acts, Fallback: NewRuleInterpreter(acts), Vision: vision})
	a, _ := li.Interpret("mirá la pantalla")
	if _, err := a.Run(fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"grim", "/tmp/astro-screen.png"} // sin -o
	if !reflect.DeepEqual(fake.calls[0], want) {
		t.Fatalf("sin monitor → grim sin -o; esperaba %v, fue %v", want, fake.calls[0])
	}
}

func TestLLMMirarSinVisionVaAFallback(t *testing.T) {
	// sin Vision configurado → 'mirar' cae al fallback (no promete lo que no puede)
	a, err := newLLM(fakeChat(`{"action":"mirar","arg":"","say":"ok"}`, nil)).Interpret("mirá la pantalla")
	if a != nil || !errors.Is(err, ErrNoEntiendo) {
		t.Fatalf("sin visión, mirar debe caer al fallback; a=%v err=%v", a, err)
	}
}
