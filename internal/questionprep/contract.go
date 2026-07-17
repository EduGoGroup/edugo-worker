// Package questionprep define el contrato del artefacto llm_prep v1 (plan 042
// D-042.2) y su validador, con el mismo nivel de formalidad que assessmentimport
// (plan 038): todo prep que el worker recibe del LLM se valida contra este contrato
// ANTES del PUT a learning; un prep que no valida jamás se persiste (retry/DLQ).
//
// El destinatario de diseño del artefacto es OTRO LLM (el revisor): strings
// normalizados donde se compara, verbatim donde se muestra. El validador no juzga el
// CONTENIDO (no sabe si "Panamá" sobra en la Gran Colombia) —eso lo ve el profesor en
// la UI—: solo verifica FORMA, versión, coherencia de tipo y cardinalidades.
package questionprep

import "encoding/json"

// SupportedVersion es la única versión del contrato que el worker sabe producir y
// validar hoy (D-042.2). Un bump de forma futura sube este número.
const SupportedVersion = 1

// Tipos de pregunta que tienen prep (los mismos que ramifican corrección en llm).
const (
	QuestionTypeShortAnswer = "short_answer"
	QuestionTypeOpenEnded   = "open_ended"
)

// content_kind válidos para short_answer (D-042.2).
const (
	ContentKindList   = "list"
	ContentKindNumber = "number"
	ContentKindDate   = "date"
	ContentKindTerm   = "term"
	ContentKindFree   = "free"
)

// validContentKinds indexa los content_kind aceptados.
var validContentKinds = map[string]struct{}{
	ContentKindList:   {},
	ContentKindNumber: {},
	ContentKindDate:   {},
	ContentKindTerm:   {},
	ContentKindFree:   {},
}

// Prep es la forma unificada del artefacto llm_prep v1. Los campos por tipo conviven
// en un solo struct (como el JSONB): el validador exige los del question_type real y
// que no falten los required. omitempty para que el JSON persistido sea mínimo.
type Prep struct {
	Version      int    `json:"version"`
	QuestionType string `json:"question_type"`

	// short_answer
	ContentKind   string   `json:"content_kind,omitempty"`
	Items         []string `json:"items,omitempty"`
	ItemsVerbatim []string `json:"items_verbatim,omitempty"`
	Unit          *string  `json:"unit,omitempty"`

	// open_ended
	QuestionIntent string   `json:"question_intent,omitempty"`
	MainIdeas      []string `json:"main_ideas,omitempty"`
	SecondaryIdeas []string `json:"secondary_ideas,omitempty"`
	ValidVariants  []string `json:"valid_variants,omitempty"`
	Criteria       []string `json:"criteria,omitempty"`
}

// Marshal serializa el prep validado a JSON crudo para el PUT a learning. Se usa en
// tests y donde convenga recomponer el artefacto; el worker persiste el RawMessage
// que devolvió el modelo (ya validado), no una re-serialización.
func (p Prep) Marshal() (json.RawMessage, error) {
	return json.Marshal(p)
}
