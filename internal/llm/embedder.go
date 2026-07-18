package llm

import "context"

// Embedder es el puerto que abstrae al modelo de embeddings (local vía Ollama hoy;
// otro backend mañana sin tocar los callers). Es una pieza SEPARADA de LLMProvider
// (plan 044 D-044.1): generar texto y vectorizarlo son operaciones distintas, con
// modelos, tamaños y endpoints distintos, así que no se engorda el puerto LLM.
//
// El reduce del pipeline material→evaluación (plan 044) usa los vectores para medir
// significado (coseno en memoria, por job) antes de gastar una llamada LLM: letras →
// significado → LLM (ADR 0035/0036). Regla de config (D-039.3): la implementación
// recibe su configuración por constructor; NUNCA lee env directo.
type Embedder interface {
	// Embed vectoriza un lote de textos y devuelve un vector por texto, en el MISMO
	// orden que la entrada. Un lote vacío devuelve un slice vacío sin llamar al
	// backend. Es responsabilidad de la implementación garantizar que
	// len(result) == len(texts); si el backend devuelve otra cantidad, es error.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
