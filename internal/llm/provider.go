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

	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
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
	// ExtractedIdeas, si no vacío, son las ideas atómicas del alumno ya extraídas por
	// ExtractIdeas (plan 045 F4, D-045.9). ENRIQUECEN el prompt como AYUDA (ideas ya
	// separadas de la prosa) sin quitar la respuesta cruda. nil/vacío = comportamiento
	// anterior (el modelo juzga solo la respuesta cruda).
	ExtractedIdeas []string
	// Language del feedback (default "es").
	Language string
}

// ExtractIdeasRequest es la petición de EXTRACCIÓN DE IDEAS de la respuesta del alumno
// (plan 045 F4, D-045.9, carril open_ended): UNA llamada que descompone la prosa del
// alumno en una lista corta de ideas atómicas, para que la comprobación por criterio
// (CheckCriterion) juzgue ideas ya estructuradas en vez de prosa cruda (Go descompone
// → el LLM decide chiquito). El caller valida el JSON; una extracción que no parsea es
// fallo transitorio.
type ExtractIdeasRequest struct {
	// QuestionText da contexto (opcional) para orientar qué ideas son relevantes.
	QuestionText string
	// StudentAnswer es la respuesta del alumno a descomponer.
	StudentAnswer string
	// Language del contenido (default "es").
	Language string
}

// DigestChunkInput es la entrada de la llamada A ("leer") del pipeline material→
// evaluación (plan 043 F3, D-043.7): UN trozo del material más —encadenado— el
// resumen del/los trozo(s) anterior(es), para que el modelo mantenga continuidad
// sin volver a ver los trozos ya leídos. La salida (DigestChunkResult) alimenta a
// ProposeCandidates: A "lee", B "pregunta".
type DigestChunkInput struct {
	// ChunkText es el trozo de material a leer (texto ya extraído y porcionado por
	// internal/chunking; el worker no vuelve a extraer aquí).
	ChunkText string
	// PrevSummary es el resumen del trozo anterior (nil en el PRIMER trozo). Se inyecta
	// como contexto de continuidad; el modelo NO lo re-extrae ni lo repregunta como
	// ideas de este trozo.
	PrevSummary *string
	// Language del contenido (default "es").
	Language string
}

// DigestChunkResult es la salida de la llamada A: los artefactos del trozo (ideas +
// tema, validables contra materialpipeline.ChunkArtifactsV1) y un resumen NUEVO para
// encadenar al trozo siguiente. El summary va APARTE de los artefactos (columna
// propia, ≤120 palabras, escrito para otro LLM) y por eso NO forma parte de
// ChunkArtifactsV1 (D-043.7).
type DigestChunkResult struct {
	Artifacts materialpipeline.ChunkArtifactsV1
	Summary   string
}

// ProposeCandidatesInput es la entrada de la llamada B ("preguntar"): SOLO las ideas
// y el tema del trozo (jamás el texto crudo, D-043.7). A partir de ellas el modelo
// sobregenera preguntas candidatas; el filtrado fino es del plan 044, así que aquí
// conviene sobregenerar variedad.
type ProposeCandidatesInput struct {
	// Artifacts son los artefactos que produjo la llamada A (main/secondary ideas +
	// chunk_topic). B trabaja SOLO con esto, nunca con el ChunkText original.
	Artifacts materialpipeline.ChunkArtifactsV1
	// Language del contenido (default "es").
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

	// ExtractIdeas descompone la respuesta del alumno en una lista corta de ideas
	// atómicas (plan 045 F4, D-045.9). Una sola llamada; el caller valida el JSON. Una
	// extracción que no parsea es fallo transitorio. Es una AYUDA para CheckCriterion:
	// su falla NO debe romper la corrección (el caller cae a la respuesta cruda).
	ExtractIdeas(ctx context.Context, req ExtractIdeasRequest) ([]string, error)

	// DigestChunk ejecuta la llamada A ("leer") del pipeline material→evaluación (plan
	// 043 F3, D-043.7): lee un trozo —con el resumen anterior como contexto de
	// continuidad— y devuelve sus artefactos (ideas + tema) más un resumen nuevo para
	// encadenar al trozo siguiente. El caller valida los artefactos contra
	// materialpipeline.ChunkArtifactsV1 antes de persistirlos; un artefacto que no
	// valida es fallo transitorio (nunca se persiste).
	DigestChunk(ctx context.Context, in DigestChunkInput) (*DigestChunkResult, error)

	// ProposeCandidates ejecuta la llamada B ("preguntar") del pipeline (plan 043 F3,
	// D-043.7): a partir SOLO de las ideas y el tema del trozo (nunca el texto crudo)
	// propone 2–4 preguntas candidatas. El caller valida cada candidata contra
	// materialpipeline.CandidatePayloadV1; sobregenerar está bien (el filtrado fino es
	// del plan 044).
	ProposeCandidates(ctx context.Context, in ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error)

	// Name identifica al provider para logs/harness (ej. "ollama", "anthropic").
	Name() string
}
