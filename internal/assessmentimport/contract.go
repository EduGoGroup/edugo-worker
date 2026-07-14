// Package assessmentimport replica —del lado del worker— la ESTRUCTURA y las
// reglas del contrato `edugo.assessment_import` v1 (plan 038, design §4/§5).
//
// Por qué una copia y no un import: el validador canónico vive dentro de
// edugo-api-learning (paquete interno `internal/app/api/dto`), inaccesible desde
// aquí. Este paquete existe para que el HARNESS del plan 039 (D-039.8) pueda
// rechazar, sin salir del worker, un JSON que un LLM local alucine con el formato
// equivocado —exactamente igual que learning rechazaría un JSON malo de
// NotebookLM—. La VERDAD servidor-side sigue siendo el endpoint de import de
// learning; esta réplica es solo para smoke/regresión de prompts, no un segundo
// punto de entrada de negocio.
package assessmentimport

import "encoding/json"

// FormatID es el identificador del formato esperado en el campo `format`.
const FormatID = "edugo.assessment_import"

// SupportedVersion es la única versión del contrato soportada por esta réplica.
const SupportedVersion = 1

// Tipos de pregunta válidos (design 038 §4: única verdad = domain/scoring.go de
// learning). Se replican como constantes locales para el harness.
const (
	QuestionTypeMultipleChoice = "multiple_choice"
	QuestionTypeMultipleSelect = "multiple_select"
	QuestionTypeTrueFalse      = "true_false"
	QuestionTypeShortAnswer    = "short_answer"
	QuestionTypeOpenEnded      = "open_ended"
)

// validQuestionTypes indexa los 5 tipos para lookup O(1).
var validQuestionTypes = map[string]struct{}{
	QuestionTypeMultipleChoice: {},
	QuestionTypeMultipleSelect: {},
	QuestionTypeTrueFalse:      {},
	QuestionTypeShortAnswer:    {},
	QuestionTypeOpenEnded:      {},
}

// validDifficulties son los valores normalizados aceptados para `difficulty`
// (null también se permite; ver §5).
var validDifficulties = map[string]struct{}{
	"easy":   {},
	"medium": {},
	"hard":   {},
}

// Contract es el sobre del import (design 038 §4).
type Contract struct {
	Format     string     `json:"format"`
	Version    int        `json:"version"`
	Source     *Source    `json:"source,omitempty"`
	Assessment Assessment `json:"assessment"`
	Questions  []Question `json:"questions"`
}

// Source describe el origen del JSON (opcional, informativo).
type Source struct {
	Kind         string `json:"kind,omitempty"`
	Model        string `json:"model,omitempty"`
	MaterialHint string `json:"material_hint,omitempty"`
}

// Assessment son los metadatos de la evaluación a crear.
type Assessment struct {
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	Purpose          string `json:"purpose,omitempty"`
	PassingScore     *int   `json:"passing_score,omitempty"`
	TimeLimitMinutes *int   `json:"time_limit_minutes,omitempty"`
}

// Question es una pregunta del contrato. `correct_answer` es polimórfico (string
// para la mayoría, array de strings para multiple_select), por eso se conserva
// crudo y se interpreta en la validación.
type Question struct {
	QuestionText  string          `json:"question_text"`
	QuestionType  string          `json:"question_type"`
	Options       []Option        `json:"options,omitempty"`
	CorrectAnswer json.RawMessage `json:"correct_answer,omitempty"`
	Explanation   string          `json:"explanation,omitempty"`
	Points        *float64        `json:"points,omitempty"`
	Difficulty    *string         `json:"difficulty,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
}

// Option es una opción de respuesta. Las opciones NO llevan `is_correct`: la
// correcta se referencia por texto desde `correct_answer` (design 038 §4).
type Option struct {
	OptionText string `json:"option_text"`
	SortOrder  int    `json:"sort_order,omitempty"`
}
