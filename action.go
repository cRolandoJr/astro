package main

// Action es una capacidad de Astro. Struct con campo func (NO interfaz): todas tienen la
// misma forma y una sola implementación → interfaz sería abstracción sin pagar (YAGNI).
// La interfaz se gana donde hay varias implementaciones (Display, Interpreter, Runner).
type Action struct {
	Name string
	Desc string // qué hace, en una línea — para el menú que ve el LLM
	Face Expression
	Run  func(r Runner) (reply string, err error)
}
