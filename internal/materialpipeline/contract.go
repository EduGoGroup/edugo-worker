// Package materialpipeline define los contratos JSONB v1 del pipeline
// material→evaluación (plan 043) y sus validadores, con el mismo nivel de
// formalidad que questionprep (plan 042) y assessmentimport (plan 038): todo
// artefacto que el worker recibe de un LLM se valida contra su contrato ANTES de
// persistirse o encadenarse al siguiente paso; un artefacto que no valida jamás se
// persiste (el caller lo trata como fallo transitorio: retry/DLQ).
//
// Dos contratos conviven aquí, uno por eslabón del pipeline:
//   - ChunkArtifactsV1  (D-043.4): lo que el LLM extrae de un trozo de material.
//   - CandidatePayloadV1 (D-043.5): una pregunta candidata, alineada al contrato
//     `edugo.assessment_import` v1 del 038 para que la conversión a import sea trivial.
//
// El destinatario de diseño es OTRO LLM (el siguiente paso del pipeline). El
// validador no juzga el CONTENIDO (no sabe si una idea principal es correcta) —eso
// lo ve el profesor en la UI—: solo verifica FORMA, versión, coherencia de tipo y
// cardinalidades.
package materialpipeline

import (
	"encoding/json"

	"github.com/EduGoGroup/edugo-worker/internal/assessmentimport"
)

// SupportedVersion es la única versión de los contratos del pipeline que el worker
// sabe producir y validar hoy (D-043.4/D-043.5). Un bump de forma futura sube este
// número. Ambos contratos comparten el mismo eje de versión v1.
const SupportedVersion = 1

// validCandidateTypes indexa los tipos de pregunta aceptados por CandidatePayloadV1.
// Se construye a partir de las constantes EXPORTADAS de assessmentimport (los 5 tipos
// del contrato import v1) para que la candidata no pueda derivar del import: el mapa
// `validQuestionTypes` de assessmentimport es privado, pero sus constantes de tipo sí
// están exportadas, así que se reusan como única fuente de los valores de string.
var validCandidateTypes = map[string]struct{}{
	assessmentimport.QuestionTypeMultipleChoice: {},
	assessmentimport.QuestionTypeMultipleSelect: {},
	assessmentimport.QuestionTypeTrueFalse:      {},
	assessmentimport.QuestionTypeShortAnswer:    {},
	assessmentimport.QuestionTypeOpenEnded:      {},
}

// ChunkArtifactsV1 son los artefactos que el LLM extrae de un trozo (chunk) de
// material (D-043.4): las ideas que alimentan la generación posterior de preguntas.
type ChunkArtifactsV1 struct {
	Version        int      `json:"version"`
	MainIdeas      []string `json:"main_ideas"`
	SecondaryIdeas []string `json:"secondary_ideas,omitempty"`
	ChunkTopic     string   `json:"chunk_topic"`
}

// Marshal serializa los artefactos validados a JSON crudo. Se usa en tests y donde
// convenga recomponer el artefacto; el worker persiste el RawMessage que devolvió el
// modelo (ya validado), no una re-serialización.
func (a ChunkArtifactsV1) Marshal() (json.RawMessage, error) {
	return json.Marshal(a)
}

// CandidatePayloadV1 es una pregunta candidata generada a partir de los artefactos de
// un chunk (D-043.5). Espeja la forma de una Question del contrato import v1 (038) para
// que la conversión candidata→import sea un mapeo directo. `correct_answer` es
// polimórfico igual que en import (string para la mayoría, array de strings para
// multiple_select), por eso se conserva crudo y se interpreta en la validación.
//
// Nota de forma: a diferencia de la Question del import (donde `options` es un array de
// objetos {option_text, sort_order}), aquí `options` es un array plano de strings: la
// candidata aún no tiene orden persistido; el sort_order se asigna al convertir a import.
type CandidatePayloadV1 struct {
	Version       int             `json:"version"`
	QuestionType  string          `json:"question_type"`
	QuestionText  string          `json:"question_text"`
	Options       []string        `json:"options,omitempty"`
	CorrectAnswer json.RawMessage `json:"correct_answer,omitempty"`
	Explanation   string          `json:"explanation,omitempty"`
	SourceIdeas   []string        `json:"source_ideas,omitempty"`
}

// Marshal serializa la candidata validada a JSON crudo (ver nota en ChunkArtifactsV1.Marshal).
func (c CandidatePayloadV1) Marshal() (json.RawMessage, error) {
	return json.Marshal(c)
}
