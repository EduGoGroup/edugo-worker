package questionprep

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Issue es un problema puntual de validación del contrato.
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
	return fmt.Sprintf("contrato llm_prep inválido (%d problemas): %s", len(e.Issues), strings.Join(parts, "; "))
}

// Validate parsea raw y lo valida contra el contrato llm_prep v1 (D-042.2) para el
// question_type esperado (el de la fila real, guard de coherencia). Devuelve el Prep
// parseado y, si hay problemas, un *ValidationError. El primer valor es útil aun ante
// error salvo que el JSON no parsee. NO juzga el contenido: solo forma/cardinalidades.
func Validate(raw []byte, expectedType string) (*Prep, error) {
	var p Prep
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &ValidationError{Issues: []Issue{{
			Field:  "<json>",
			Reason: "no es JSON válido del contrato: " + err.Error(),
		}}}
	}

	var issues []Issue

	if p.Version != SupportedVersion {
		issues = append(issues, Issue{"version", fmt.Sprintf("versión %d no soportada (soportada: %d)", p.Version, SupportedVersion)})
	}
	// Coherencia de tipo: el prep espeja el question_type de la fila (D-042.2 §validación 2).
	if p.QuestionType != expectedType {
		issues = append(issues, Issue{"question_type", fmt.Sprintf("%q no coincide con el tipo de la pregunta %q", p.QuestionType, expectedType)})
	}

	switch expectedType {
	case QuestionTypeShortAnswer:
		issues = append(issues, validateShortAnswer(p)...)
	case QuestionTypeOpenEnded:
		issues = append(issues, validateOpenEnded(p)...)
	default:
		issues = append(issues, Issue{"question_type", fmt.Sprintf("tipo %q no tiene prep (solo short_answer/open_ended)", expectedType)})
	}

	if len(issues) > 0 {
		return &p, &ValidationError{Issues: issues}
	}
	return &p, nil
}

// validateShortAnswer aplica las reglas de content_kind + items/items_verbatim
// (D-042.2 short_answer).
func validateShortAnswer(p Prep) []Issue {
	var issues []Issue

	if _, ok := validContentKinds[p.ContentKind]; !ok {
		issues = append(issues, Issue{"content_kind", fmt.Sprintf("valor %q inválido (list|number|date|term|free)", p.ContentKind)})
	}

	// Cardinalidad de items por content_kind: list ≥1; el resto exactamente 1.
	switch p.ContentKind {
	case ContentKindList:
		if len(p.Items) < 1 {
			issues = append(issues, Issue{"items", "content_kind=list requiere al menos 1 ítem"})
		}
	case ContentKindNumber, ContentKindDate, ContentKindTerm, ContentKindFree:
		if len(p.Items) != 1 {
			issues = append(issues, Issue{"items", fmt.Sprintf("content_kind=%s requiere exactamente 1 ítem (llegaron %d)", p.ContentKind, len(p.Items))})
		}
	}

	// items y items_verbatim: mismo largo (mismo orden implícito, D-042.2).
	if len(p.Items) != len(p.ItemsVerbatim) {
		issues = append(issues, Issue{"items_verbatim", fmt.Sprintf("debe tener el mismo largo que items (%d vs %d)", len(p.ItemsVerbatim), len(p.Items))})
	}
	for i, it := range p.Items {
		if strings.TrimSpace(it) == "" {
			issues = append(issues, Issue{fmt.Sprintf("items[%d]", i), "no puede estar vacío"})
		}
	}
	for i, it := range p.ItemsVerbatim {
		if strings.TrimSpace(it) == "" {
			issues = append(issues, Issue{fmt.Sprintf("items_verbatim[%d]", i), "no puede estar vacío"})
		}
	}

	return issues
}

// validateOpenEnded aplica las reglas de intención + ideas (D-042.2 open_ended).
// secondary_ideas/valid_variants/criteria pueden venir vacíos (lista de 0 elementos),
// pero si criteria trae elementos NINGUNO puede estar en blanco: un criterio vacío haría
// que el carril por criterios cuente 0 reales y devuelva incorrect/0.0 sin consultar al
// LLM (simetría con items no vacíos de short_answer).
func validateOpenEnded(p Prep) []Issue {
	var issues []Issue

	if strings.TrimSpace(p.QuestionIntent) == "" {
		issues = append(issues, Issue{"question_intent", "obligatorio (1 frase con qué mide la pregunta)"})
	}
	if len(p.MainIdeas) < 1 {
		issues = append(issues, Issue{"main_ideas", "requiere al menos 1 idea principal"})
	}
	for i, idea := range p.MainIdeas {
		if strings.TrimSpace(idea) == "" {
			issues = append(issues, Issue{fmt.Sprintf("main_ideas[%d]", i), "no puede estar vacío"})
		}
	}
	for i, c := range p.Criteria {
		if strings.TrimSpace(c) == "" {
			issues = append(issues, Issue{fmt.Sprintf("criteria[%d]", i), "no puede estar vacío"})
		}
	}

	return issues
}
