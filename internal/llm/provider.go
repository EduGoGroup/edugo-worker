// Package llm define el puerto LLMProvider del worker (plan 039, D-039.4) y sus
// tipos de entrada/salida. Es infraestructura pura: NO contiene flujo de negocio
// (disparadores, estados, persistencia) —eso vive en los planes 040/041—.
//
// Regla de config (D-039.3): las implementaciones reciben su configuración
// (URLs, modelo, api key) COMO PARÁMETRO del constructor; NUNCA leen variables de
// entorno directo. Los env se resuelven en bootstrap/config y se inyectan. Así,
// si mañana se justifica config por-escuela, se agrega otra fuente sin reescribir
// el provider.
package llm

import (
	"context"
	"encoding/json"
)

// MaterialInput es el material de origen a partir del cual generar una
// evaluación. En v1 es texto plano ya extraído (el worker no vuelve a extraer);
// SubjectHint es una pista opcional de la materia para orientar al modelo.
type MaterialInput struct {
	Title       string
	Content     string
	SubjectHint string
}

// GenerationParams parametriza la generación de una evaluación.
type GenerationParams struct {
	// NumQuestions es el número de preguntas deseado (el modelo puede acercarse).
	NumQuestions int
	// Language es el idioma del contenido generado (ej. "es"). Default "es".
	Language string
	// Difficulty opcional: "easy"|"medium"|"hard". Vacío = mezcla libre.
	Difficulty string
	// QuestionTypes opcional: subconjunto de los 5 tipos del contrato. Vacío =
	// el modelo elige tipos variados.
	QuestionTypes []string
}

// Tipos de pregunta que ramifican el prompt de corrección (plan 040 F3/F4). El
// resto de tipos (multiple_choice, etc.) los resuelve learning sin LLM.
const (
	QuestionTypeOpenEnded   = "open_ended"
	QuestionTypeShortAnswer = "short_answer"
)

// ReviewRequest es la petición de corrección de una respuesta (plan 040).
type ReviewRequest struct {
	// QuestionType ramifica el prompt: "open_ended" (rúbrica, correct/partial/
	// incorrect) vs "short_answer" (equivalencia semántica, correct/incorrect).
	// Vacío se trata como open_ended (compatibilidad F3).
	QuestionType   string
	QuestionText   string
	ExpectedAnswer string // respuesta esperada (canónica en short_answer)
	Rubric         string // rúbrica/criterios (si aplica, para open_ended)
	StudentAnswer  string
	Language       string // default "es"
}

// Verdict es el dictamen cualitativo de una corrección.
type Verdict string

const (
	VerdictCorrect   Verdict = "correct"
	VerdictPartial   Verdict = "partial"
	VerdictIncorrect Verdict = "incorrect"
)

// ReviewResult es el resultado de corregir una respuesta. Score es 0..1
// (fracción del puntaje) para que el consumidor (040) lo escale al puntaje real.
type ReviewResult struct {
	Verdict  Verdict `json:"verdict"`
	Score    float64 `json:"score"`
	Feedback string  `json:"feedback"`
}

// LLMProvider es el puerto que abstrae al modelo (local vía Ollama o remoto vía
// API). Dos operaciones, una por carril (D-039.4).
type LLMProvider interface {
	// GenerateAssessment produce un JSON que DEBE cumplir el contrato
	// `edugo.assessment_import` v1 (plan 038). El caller lo valida con el mismo
	// validador del import: un modelo que alucine el formato se rechaza igual que
	// un JSON malo de una herramienta externa (D-039.4).
	GenerateAssessment(ctx context.Context, material MaterialInput, params GenerationParams) (json.RawMessage, error)

	// ReviewAnswer corrige la respuesta de un alumno contra la esperada/rúbrica.
	ReviewAnswer(ctx context.Context, req ReviewRequest) (ReviewResult, error)

	// Name identifica al provider para logs/harness (ej. "ollama", "anthropic").
	Name() string
}
