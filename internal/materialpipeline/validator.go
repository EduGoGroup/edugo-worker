package materialpipeline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EduGoGroup/edugo-worker/internal/assessmentimport"
)

// minOptionsChoice es la cardinalidad mínima de opciones para los tipos con opciones
// (multiple_choice/multiple_select), igual que en el contrato import v1 (038 §5).
const minOptionsChoice = 2

// Issue es un problema puntual de validación de un contrato del pipeline.
type Issue struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (i Issue) String() string { return fmt.Sprintf("[%s] %s", i.Field, i.Reason) }

// ValidationError agrupa todos los Issue detectados. Su Error() los serializa para
// diagnóstico en logs/harness.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, iss := range e.Issues {
		parts[i] = iss.String()
	}
	return fmt.Sprintf("contrato del pipeline inválido (%d problemas): %s", len(e.Issues), strings.Join(parts, "; "))
}

// ValidateChunkArtifacts parsea raw y lo valida contra ChunkArtifactsV1 (D-043.4).
// Devuelve los artefactos parseados y, si hay problemas, un *ValidationError. El primer
// valor es útil aun ante error salvo que el JSON no parsee. NO juzga el contenido: solo
// forma/cardinalidades.
func ValidateChunkArtifacts(raw []byte) (*ChunkArtifactsV1, error) {
	var a ChunkArtifactsV1
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, &ValidationError{Issues: []Issue{{
			Field:  "<json>",
			Reason: "no es JSON válido del contrato: " + err.Error(),
		}}}
	}

	var issues []Issue

	if a.Version != SupportedVersion {
		issues = append(issues, Issue{"version", fmt.Sprintf("versión %d no soportada (soportada: %d)", a.Version, SupportedVersion)})
	}

	if len(a.MainIdeas) < 1 {
		issues = append(issues, Issue{"main_ideas", "requiere al menos 1 idea principal"})
	}
	for i, idea := range a.MainIdeas {
		if strings.TrimSpace(idea) == "" {
			issues = append(issues, Issue{fmt.Sprintf("main_ideas[%d]", i), "no puede estar vacío"})
		}
	}
	// secondary_ideas puede venir vacío u omitido; si trae elementos, ninguno en blanco.
	for i, idea := range a.SecondaryIdeas {
		if strings.TrimSpace(idea) == "" {
			issues = append(issues, Issue{fmt.Sprintf("secondary_ideas[%d]", i), "no puede estar vacío"})
		}
	}

	if strings.TrimSpace(a.ChunkTopic) == "" {
		issues = append(issues, Issue{"chunk_topic", "obligatorio (no puede estar vacío)"})
	}

	if len(issues) > 0 {
		return &a, &ValidationError{Issues: issues}
	}
	return &a, nil
}

// ValidateCandidatePayload parsea raw y lo valida contra CandidatePayloadV1 (D-043.5),
// alineado al contrato import v1 (038): tipo de pregunta válido, cardinalidad de opciones
// por tipo y correct_answer polimórfico (escalar/array). Devuelve la candidata parseada y,
// si hay problemas, un *ValidationError. NO juzga el contenido: solo forma/cardinalidades.
func ValidateCandidatePayload(raw []byte) (*CandidatePayloadV1, error) {
	var c CandidatePayloadV1
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, &ValidationError{Issues: []Issue{{
			Field:  "<json>",
			Reason: "no es JSON válido del contrato: " + err.Error(),
		}}}
	}

	var issues []Issue

	if c.Version != SupportedVersion {
		issues = append(issues, Issue{"version", fmt.Sprintf("versión %d no soportada (soportada: %d)", c.Version, SupportedVersion)})
	}

	if strings.TrimSpace(c.QuestionText) == "" {
		issues = append(issues, Issue{"question_text", "no puede estar vacío"})
	}

	if _, ok := validCandidateTypes[c.QuestionType]; !ok {
		issues = append(issues, Issue{"question_type", fmt.Sprintf("tipo desconocido %q", c.QuestionType)})
		// Sin tipo válido no tiene sentido validar opciones/correct_answer.
		if len(issues) > 0 {
			return &c, &ValidationError{Issues: issues}
		}
		return &c, nil
	}

	issues = append(issues, validateCandidateOptions(c)...)
	issues = append(issues, validateCandidateCorrectAnswer(c)...)

	// source_ideas es opcional; si trae elementos, ninguno en blanco.
	for i, idea := range c.SourceIdeas {
		if strings.TrimSpace(idea) == "" {
			issues = append(issues, Issue{fmt.Sprintf("source_ideas[%d]", i), "no puede estar vacío"})
		}
	}

	if len(issues) > 0 {
		return &c, &ValidationError{Issues: issues}
	}
	return &c, nil
}

// validateCandidateOptions aplica la cardinalidad de opciones por tipo (038 §5): los
// tipos de opciones requieren ≥2 y sin strings vacíos; el resto no debe llevar opciones.
func validateCandidateOptions(c CandidatePayloadV1) []Issue {
	var issues []Issue

	switch c.QuestionType {
	case assessmentimport.QuestionTypeMultipleChoice, assessmentimport.QuestionTypeMultipleSelect:
		if len(c.Options) < minOptionsChoice {
			issues = append(issues, Issue{"options", fmt.Sprintf("%s requiere al menos %d opciones", c.QuestionType, minOptionsChoice)})
		}
	case assessmentimport.QuestionTypeTrueFalse, assessmentimport.QuestionTypeShortAnswer, assessmentimport.QuestionTypeOpenEnded:
		if len(c.Options) > 0 {
			issues = append(issues, Issue{"options", fmt.Sprintf("%s no debe llevar opciones", c.QuestionType)})
		}
	}

	for i, opt := range c.Options {
		if strings.TrimSpace(opt) == "" {
			issues = append(issues, Issue{fmt.Sprintf("options[%d]", i), "no puede estar vacío"})
		}
	}

	return issues
}

// validateCandidateCorrectAnswer aplica el polimorfismo escalar/array de correct_answer
// alineado a assessmentimport: array de strings para multiple_select, string para el
// resto; obligatorio para todos salvo open_ended. Donde hay opciones, la correcta debe
// coincidir (match exacto) con alguna opción.
func validateCandidateCorrectAnswer(c CandidatePayloadV1) []Issue {
	if c.QuestionType == assessmentimport.QuestionTypeOpenEnded {
		return nil // open_ended no lleva correcta
	}

	if len(c.CorrectAnswer) == 0 {
		return []Issue{{"correct_answer", "obligatorio para este tipo de pregunta"}}
	}

	switch c.QuestionType {
	case assessmentimport.QuestionTypeMultipleSelect:
		var answers []string
		if err := json.Unmarshal(c.CorrectAnswer, &answers); err != nil {
			return []Issue{{"correct_answer", "multiple_select espera un array JSON de textos"}}
		}
		if len(answers) == 0 {
			return []Issue{{"correct_answer", "el array de respuestas no puede estar vacío"}}
		}
		opts := optionSet(c.Options)
		var issues []Issue
		for _, ans := range answers {
			if _, ok := opts[ans]; !ok {
				issues = append(issues, Issue{"correct_answer", fmt.Sprintf("%q no coincide con ninguna opción", ans)})
			}
		}
		return issues

	case assessmentimport.QuestionTypeTrueFalse:
		var answer string
		if err := json.Unmarshal(c.CorrectAnswer, &answer); err != nil {
			return []Issue{{"correct_answer", "true_false espera 'true' o 'false' como string"}}
		}
		a := strings.ToLower(strings.TrimSpace(answer))
		if a != "true" && a != "false" {
			return []Issue{{"correct_answer", "true_false espera 'true' o 'false'"}}
		}
		return nil

	case assessmentimport.QuestionTypeMultipleChoice:
		var answer string
		if err := json.Unmarshal(c.CorrectAnswer, &answer); err != nil {
			return []Issue{{"correct_answer", "multiple_choice espera un string con el texto correcto"}}
		}
		if _, ok := optionSet(c.Options)[answer]; !ok {
			return []Issue{{"correct_answer", fmt.Sprintf("%q no coincide (match exacto) con ninguna opción", answer)}}
		}
		return nil

	default: // short_answer: string libre, solo obligatorio (ya verificado)
		var answer string
		if err := json.Unmarshal(c.CorrectAnswer, &answer); err != nil {
			return []Issue{{"correct_answer", "short_answer espera un string"}}
		}
		if strings.TrimSpace(answer) == "" {
			return []Issue{{"correct_answer", "no puede estar vacío"}}
		}
		return nil
	}
}

func optionSet(opts []string) map[string]struct{} {
	set := make(map[string]struct{}, len(opts))
	for _, o := range opts {
		set[o] = struct{}{}
	}
	return set
}
