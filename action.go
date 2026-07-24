package main

// Action es una capacidad de Astro. Struct con campo func (NO interfaz): todas tienen la
// misma forma y una sola implementación → interfaz sería abstracción sin pagar (YAGNI).
// La interfaz se gana donde hay varias implementaciones (Display, Interpreter, Runner).
type Action struct {
	Name string
	Desc string // qué hace, en una línea — para el menú que ve el LLM
	Face Expression
	// Speaks: la acción produce su PROPIA respuesta hablada (un DATO real: la hora, la fecha, el clima…)
	// → wrap habla la salida de Run, NO el `say` del LLM (que inventaría el dato). Default false =
	// acción de EFECTO (pausar, subir volumen…): se habla el `say` que redactó el LLM (más natural).
	Speaks bool
	Run    func(r Runner) (reply string, err error)
}
