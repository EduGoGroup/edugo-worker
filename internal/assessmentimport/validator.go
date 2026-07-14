package assessmentimport

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Límites anti-abuso por defecto (design 038 §5 / D-038.12). Replican
// DefaultMaxImportQuestions y DefaultMaxImportJSONBytes del validador de learning.
// En el flujo real de import estos límites son configurables por env/escuela
// (deuda 019); aquí son solo los defaults duros para el harness.
const (
	DefaultMaxQuestions   = 100
	DefaultMaxJSONBytes   = 1 << 20 // 1 MiB
	DefaultMaxOptionsPerQ = 10
	maxTitleLen           = 255
	minOptionsChoice      = 2
)

// Limits agrupa los límites anti-abuso aplicables al validar.
type Limits struct {
	MaxQuestions   int
	MaxJSONBytes   int
	MaxOptionsPerQ int
}

// DefaultLimits devuelve los límites duros por defecto.
func DefaultLimits() Limits {
	return Limits{
		MaxQuestions:   DefaultMaxQuestions,
		MaxJSONBytes:   DefaultMaxJSONBytes,
		MaxOptionsPerQ: DefaultMaxOptionsPerQ,
	}
}

// Issue es un problema de validación puntual. Cuando aplica a una pregunta,
// QuestionIndex es su índice (base 0); vale -1 para problemas del sobre.
type Issue struct {
	QuestionIndex int    `json:"question_index"`
	Field         string `json:"field"`
	Reason        string `json:"reason"`
}

func (i Issue) String() string {
	if i.QuestionIndex < 0 {
		return fmt.Sprintf("[%s] %s", i.Field, i.Reason)
	}
	return fmt.Sprintf("[pregunta %d · %s] %s", i.QuestionIndex, i.Field, i.Reason)
}

// ValidationError agrupa todos los Issue detectados. Su Error() los serializa
// para diagnóstico rápido en el harness.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, iss := range e.Issues {
		parts[i] = iss.String()
	}
	return fmt.Sprintf("contrato inválido (%d problemas): %s", len(e.Issues), strings.Join(parts, "; "))
}

// Validate parsea y valida raw contra las reglas del contrato v1 (design 038 §5).
// Devuelve el Contract parseado y, si hay problemas, un *ValidationError con el
// detalle por pregunta. El primer valor de retorno es útil incluso ante error
// parcial (p.ej. para inspección en el harness) salvo que el JSON no parsee.
func Validate(raw []byte, limits Limits) (*Contract, error) {
	if limits.MaxQuestions == 0 {
		limits = DefaultLimits()
	}

	if len(raw) > limits.MaxJSONBytes {
		return nil, &ValidationError{Issues: []Issue{{
			QuestionIndex: -1,
			Field:         "<json>",
			Reason:        fmt.Sprintf("el JSON pesa %d bytes; máximo %d", len(raw), limits.MaxJSONBytes),
		}}}
	}

	var c Contract
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		// Reintentar tolerando campos extra: un LLM puede añadir claves de más y
		// aun así producir un draft utilizable. El harness reporta ambos casos.
		if err2 := json.Unmarshal(raw, &c); err2 != nil {
			return nil, &ValidationError{Issues: []Issue{{
				QuestionIndex: -1,
				Field:         "<json>",
				Reason:        "no es JSON válido del contrato: " + err2.Error(),
			}}}
		}
	}

	var issues []Issue

	if c.Format != FormatID {
		issues = append(issues, Issue{-1, "format", fmt.Sprintf("se esperaba %q (re-exporta la plantilla); llegó %q", FormatID, c.Format)})
	}
	if c.Version != SupportedVersion {
		issues = append(issues, Issue{-1, "version", fmt.Sprintf("versión %d no soportada; re-exporta la plantilla (soportada: %d)", c.Version, SupportedVersion)})
	}

	title := strings.TrimSpace(c.Assessment.Title)
	if title == "" {
		issues = append(issues, Issue{-1, "assessment.title", "el título no puede estar vacío"})
	} else if len(title) > maxTitleLen {
		issues = append(issues, Issue{-1, "assessment.title", fmt.Sprintf("el título excede %d caracteres", maxTitleLen)})
	}
	if ps := c.Assessment.PassingScore; ps != nil && (*ps < 0 || *ps > 100) {
		issues = append(issues, Issue{-1, "assessment.passing_score", "debe estar entre 0 y 100"})
	}

	if len(c.Questions) == 0 {
		issues = append(issues, Issue{-1, "questions", "el contrato no trae preguntas"})
	}
	if len(c.Questions) > limits.MaxQuestions {
		issues = append(issues, Issue{-1, "questions", fmt.Sprintf("%d preguntas exceden el máximo de %d", len(c.Questions), limits.MaxQuestions)})
	}

	for i, q := range c.Questions {
		issues = append(issues, validateQuestion(i, q, limits)...)
	}

	if len(issues) > 0 {
		return &c, &ValidationError{Issues: issues}
	}
	return &c, nil
}

func validateQuestion(idx int, q Question, limits Limits) []Issue {
	var issues []Issue

	if strings.TrimSpace(q.QuestionText) == "" {
		issues = append(issues, Issue{idx, "question_text", "no puede estar vacío"})
	}

	if _, ok := validQuestionTypes[q.QuestionType]; !ok {
		issues = append(issues, Issue{idx, "question_type", fmt.Sprintf("tipo desconocido %q", q.QuestionType)})
		// Sin tipo válido no tiene sentido validar opciones/correct_answer.
		return issues
	}

	if len(q.Options) > limits.MaxOptionsPerQ {
		issues = append(issues, Issue{idx, "options", fmt.Sprintf("%d opciones exceden el máximo de %d", len(q.Options), limits.MaxOptionsPerQ)})
	}

	// Cardinalidad de opciones por tipo (design 038 §5).
	switch q.QuestionType {
	case QuestionTypeMultipleChoice, QuestionTypeMultipleSelect:
		if len(q.Options) < minOptionsChoice {
			issues = append(issues, Issue{idx, "options", fmt.Sprintf("%s requiere al menos %d opciones", q.QuestionType, minOptionsChoice)})
		}
	case QuestionTypeTrueFalse, QuestionTypeShortAnswer, QuestionTypeOpenEnded:
		if len(q.Options) > 0 {
			issues = append(issues, Issue{idx, "options", fmt.Sprintf("%s no debe llevar opciones", q.QuestionType)})
		}
	}

	for oi, opt := range q.Options {
		if strings.TrimSpace(opt.OptionText) == "" {
			issues = append(issues, Issue{idx, fmt.Sprintf("options[%d].option_text", oi), "no puede estar vacío"})
		}
	}

	// correct_answer: obligatorio salvo open_ended; con match contra opciones.
	issues = append(issues, validateCorrectAnswer(idx, q)...)

	// difficulty: null o easy/medium/hard.
	if q.Difficulty != nil {
		d := strings.ToLower(strings.TrimSpace(*q.Difficulty))
		if d != "" {
			if _, ok := validDifficulties[d]; !ok {
				issues = append(issues, Issue{idx, "difficulty", fmt.Sprintf("valor %q no normalizable a easy/medium/hard", *q.Difficulty)})
			}
		}
	}

	if q.Points != nil && *q.Points < 0 {
		issues = append(issues, Issue{idx, "points", "no puede ser negativo"})
	}

	return issues
}

func validateCorrectAnswer(idx int, q Question) []Issue {
	if q.QuestionType == QuestionTypeOpenEnded {
		return nil // open_ended no lleva correcta
	}

	if len(q.CorrectAnswer) == 0 {
		return []Issue{{idx, "correct_answer", "obligatorio para este tipo de pregunta"}}
	}

	switch q.QuestionType {
	case QuestionTypeMultipleSelect:
		var answers []string
		if err := json.Unmarshal(q.CorrectAnswer, &answers); err != nil {
			return []Issue{{idx, "correct_answer", "multiple_select espera un array JSON de textos"}}
		}
		if len(answers) == 0 {
			return []Issue{{idx, "correct_answer", "el array de respuestas no puede estar vacío"}}
		}
		opts := optionTextSet(q.Options)
		var issues []Issue
		for _, a := range answers {
			if _, ok := opts[a]; !ok {
				issues = append(issues, Issue{idx, "correct_answer", fmt.Sprintf("%q no coincide con ninguna opción", a)})
			}
		}
		return issues

	case QuestionTypeTrueFalse:
		var answer string
		if err := json.Unmarshal(q.CorrectAnswer, &answer); err != nil {
			return []Issue{{idx, "correct_answer", "true_false espera 'true' o 'false' como string"}}
		}
		a := strings.ToLower(strings.TrimSpace(answer))
		if a != "true" && a != "false" {
			return []Issue{{idx, "correct_answer", "true_false espera 'true' o 'false'"}}
		}
		return nil

	case QuestionTypeMultipleChoice:
		var answer string
		if err := json.Unmarshal(q.CorrectAnswer, &answer); err != nil {
			return []Issue{{idx, "correct_answer", "multiple_choice espera un string con el texto correcto"}}
		}
		if _, ok := optionTextSet(q.Options)[answer]; !ok {
			return []Issue{{idx, "correct_answer", fmt.Sprintf("%q no coincide (match exacto) con ninguna opción", answer)}}
		}
		return nil

	default: // short_answer: string libre, solo obligatorio (ya verificado)
		var answer string
		if err := json.Unmarshal(q.CorrectAnswer, &answer); err != nil {
			return []Issue{{idx, "correct_answer", "short_answer espera un string"}}
		}
		if strings.TrimSpace(answer) == "" {
			return []Issue{{idx, "correct_answer", "no puede estar vacío"}}
		}
		return nil
	}
}

func optionTextSet(opts []Option) map[string]struct{} {
	set := make(map[string]struct{}, len(opts))
	for _, o := range opts {
		set[o.OptionText] = struct{}{}
	}
	return set
}
