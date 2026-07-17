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

	// Prep, si != nil, aporta las pistas del artefacto llm_prep (plan 042 F4a) para
	// ENRIQUECER el prompt global de open_ended: intención, ideas esperadas y
	// variantes válidas. Sin él, el prompt es el actual (fallback). No aplica al
	// carril de criterios (F4b lo reemplaza por completo).
	Prep *ReviewPrep
}

// ReviewPrep son las pistas del artefacto llm_prep (open_ended) que enriquecen el
// prompt global de corrección (plan 042 F4a). El worker las mapea desde el prep
// validado; el puerto llm se mantiene libre del contrato questionprep.
type ReviewPrep struct {
	// QuestionIntent describe qué mide la pregunta (1 frase).
	QuestionIntent string
	// MainIdeas son las ideas que una respuesta correcta DEBE contener.
	MainIdeas []string
	// SecondaryIdeas son ideas deseables pero no imprescindibles.
	SecondaryIdeas []string
	// ValidVariants son reformulaciones equivalentes de la respuesta esperada: una
	// respuesta que exprese cualquiera de ellas es CORRECTA aunque use otras palabras.
	ValidVariants []string
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

// PrepRequest es la petición de PREPARACIÓN de una pregunta para el LLM (plan 042
// D-042.4): a partir de la canónica del profesor se produce el artefacto llm_prep
// (descomposición para short_answer, enriquecimiento para open_ended). El worker
// valida la salida contra el contrato v1 antes de persistirla; el modelo NUNCA
// corrige ni inventa contenido (los ítems salen textuales del texto del profesor).
type PrepRequest struct {
	// QuestionType ramifica el prompt: "short_answer" (descomponer/clasificar) vs
	// "open_ended" (intención/ideas/variantes/criterios).
	QuestionType string
	QuestionText string
	// CorrectAnswer es la respuesta canónica (short_answer) o esperada (open_ended);
	// puede venir vacía en open_ended.
	CorrectAnswer string
	// Explanation es la explicación/rúbrica de la pregunta (si existe). En open_ended
	// alimenta los criterios cuando es una rúbrica.
	Explanation string
	// Feedback es el comentario del profesor sobre una prep previa (reason=feedback,
	// D-042.7). Vacío en el caso normal; si viene, el prompt lo prioriza.
	Feedback string
	// Language del contenido (default "es").
	Language string
}

// PairEquivalenceRequest es la petición BINARIA de equivalencia de UN par (plan 042
// F3c, carril short_answer triturado): el modelo decide si el fragmento de la
// respuesta del alumno nombra el MISMO elemento que el ítem esperado del prep del
// profesor. Se hace una llamada por ítem residual (los que el match determinista no
// resolvió); el veredicto global se recompone en Go. Salida binaria correct|incorrect.
type PairEquivalenceRequest struct {
	// QuestionText da contexto (opcional) para desambiguar la equivalencia.
	QuestionText string
	// Expected es el ítem esperado del prep (verbatim, legible para el modelo).
	Expected string
	// Candidate es el fragmento sobrante de la respuesta del alumno a comparar.
	Candidate string
	// Language del feedback (default "es").
	Language string
}

// CriterionCheckRequest es la petición BINARIA de cumplimiento de UN criterio (plan
// 042 F4b, carril open_ended por criterios): el modelo decide si la respuesta del
// alumno CUMPLE el criterio dado. Se hace una llamada por criterio del prep; el
// veredicto+score globales se agregan de forma DETERMINISTA en Go. Salida binaria
// correct|incorrect.
type CriterionCheckRequest struct {
	QuestionText string
	// ExpectedAnswer da contexto (opcional) de la respuesta esperada/rúbrica.
	ExpectedAnswer string
	// Criterion es el criterio verificable a comprobar (p. ej. "menciona los cloroplastos").
	Criterion string
	// StudentAnswer es la respuesta del alumno a evaluar contra el criterio.
	StudentAnswer string
	// Language del feedback (default "es").
	Language string
}

// LLMProvider es el puerto que abstrae al modelo (local vía Ollama o remoto vía
// API). Una operación por carril (D-039.4): generación (038), corrección (040) y
// preparación (042).
type LLMProvider interface {
	// GenerateAssessment produce un JSON que DEBE cumplir el contrato
	// `edugo.assessment_import` v1 (plan 038). El caller lo valida con el mismo
	// validador del import: un modelo que alucine el formato se rechaza igual que
	// un JSON malo de una herramienta externa (D-039.4).
	GenerateAssessment(ctx context.Context, material MaterialInput, params GenerationParams) (json.RawMessage, error)

	// ReviewAnswer corrige la respuesta de un alumno contra la esperada/rúbrica.
	ReviewAnswer(ctx context.Context, req ReviewRequest) (ReviewResult, error)

	// PrepareQuestion produce el artefacto de preparación (JSON crudo del contrato
	// llm_prep v1, plan 042). El caller lo valida contra el contrato antes de
	// persistirlo; un prep que no valida es fallo transitorio (nunca se guarda).
	PrepareQuestion(ctx context.Context, req PrepRequest) (json.RawMessage, error)

	// JudgePairEquivalence decide la equivalencia binaria de UN par (ítem esperado vs
	// fragmento del alumno) del carril short_answer triturado (plan 042 F3c). Devuelve
	// un ReviewResult binario: verdict correct|incorrect, score 1.0|0.0.
	JudgePairEquivalence(ctx context.Context, req PairEquivalenceRequest) (ReviewResult, error)

	// CheckCriterion decide binariamente si la respuesta del alumno CUMPLE UN criterio
	// del carril open_ended por criterios (plan 042 F4b). Devuelve un ReviewResult
	// binario: verdict correct|incorrect, score 1.0|0.0. La agregación a verdict+score
	// globales la hace el caller de forma determinista.
	CheckCriterion(ctx context.Context, req CriterionCheckRequest) (ReviewResult, error)

	// Name identifica al provider para logs/harness (ej. "ollama", "anthropic").
	Name() string
}
