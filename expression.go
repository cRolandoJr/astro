package main

// Expression es el gesto de Astro. String para mapear directo al PNG y al comando de eww.
type Expression string

const (
	Neutral     Expression = "neutral"
	Feliz       Expression = "feliz"
	Curioso     Expression = "curioso"
	Pensativo   Expression = "pensativo"
	Sorprendido Expression = "sorprendido"
	Dormido     Expression = "dormido"
	Triste      Expression = "triste"
	Mareado     Expression = "mareado"
)

// (parpadeo, bostezo, guino, amor, enojado también existen como PNG — se usan en fases
//  con voz/estados; por eso hay 13 PNGs y 8 constantes en Fase A.)
