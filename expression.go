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
	Enojado     Expression = "enojado" // reacción a maltrato/insulto (mood del LLM)
	Amor        Expression = "amor"    // reacción a algo cariñoso (mood del LLM)
)

// (parpadeo, bostezo, guino también existen como PNG — para animación de idle/estados; por eso hay
//  13 PNGs. Enojado/Amor se sumaron como constantes al hacer la cara reactiva por 'mood'.)
