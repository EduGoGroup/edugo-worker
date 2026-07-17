package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/questionprep"
)

// settingKeyPrepMode es la clave de política por escuela que gobierna el carril de
// preparación. Elección (D-042.8): el prep REUSA la política del carril de
// corrección (`llm.review.mode`), no una llave nueva. Racional: el prep existe para
// alimentar la corrección —descompone/enriquece la pregunta para que el revisor
// acierte—; si una escuela tiene la corrección IA apagada, preparar sus preguntas es
// trabajo inútil. El catálogo del 039 solo ofrece `llm.generation.mode` (crear
// evaluaciones desde material) y `llm.review.mode` (corregir intentos); prep no es
// generación, es la mitad preparatoria de la corrección, así que review.mode es la
// más coherente. Mismos valores off|local|api que el carril de revisión.
const settingKeyPrepMode = settingKeyReviewMode

// prepLanguage es el idioma que se pide al LLM para la preparación (regla global).
const prepLanguage = "es"

// ErrMalformedPrepEvent marca un evento question.prep_requested indecodificable o
// inválido. Permanente (→ DLQ): reintentar no lo arregla. classifyError lo trata
// como ErrMalformedEvent (mismo carril permanente) porque lo envuelve.
var ErrMalformedPrepEvent = fmt.Errorf("%w: evento question.prep_requested", ErrMalformedEvent)

// ErrInvalidPrep marca un artefacto del LLM que no cumple el contrato v1. Se trata
// como fallo del provider (transitorio): reintentar puede dar un prep válido; NUNCA
// se hace PUT de un prep malformado (envenenaría la corrección, D-042.2).
var ErrInvalidPrep = errors.New("prep del LLM inválido (no cumple el contrato v1)")

// LearningPrepReader/Writer es la porción del LearningPrepClient M2M que usa el
// processor. Se define como interfaz para mockearla en tests; *m2m.LearningPrepClient
// la satisface.
type LearningPrepClient interface {
	// GetPrepSource lee la fuente fresca de la pregunta (texto, tipo, canónica,
	// explicación, feedback pendiente, hash actual, school_id).
	GetPrepSource(ctx context.Context, questionID string) (m2m.PrepSourceResponse, error)
	// SavePrep persiste el prep con concurrencia optimista por hash. Un
	// m2m.ErrPrepHashConflict (409) obliga a abstenerse (no es fallo).
	SavePrep(ctx context.Context, questionID string, req m2m.SavePrepRequest) error
}

// QuestionPrepProcessor consume question.prep_requested y orquesta la preparación
// asistida por LLM de una pregunta (plan 042 D-042.4). Orquestador PURO: cero SQL,
// todo por M2M. Lee la fuente fresca, resuelve la política de la escuela (off = ack),
// pide UNA llamada de preparación al LLM del modo, valida el artefacto contra el
// contrato v1 y lo escribe con el source_hash con el que trabajó.
type QuestionPrepProcessor struct {
	settings  SchoolSettingsReader
	learning  LearningPrepClient
	providers map[string]llm.LLMProvider
	logger    logger.Logger
}

// NewQuestionPrepProcessor construye el processor. providers mapea el mode
// ("local"/"api") al LLMProvider; un mode sin provider disponible se trata como
// configuración inválida al procesar.
func NewQuestionPrepProcessor(
	settings SchoolSettingsReader,
	learning LearningPrepClient,
	providers map[string]llm.LLMProvider,
	log logger.Logger,
) *QuestionPrepProcessor {
	return &QuestionPrepProcessor{
		settings:  settings,
		learning:  learning,
		providers: providers,
		logger:    log,
	}
}

// EventType satisface processor.Processor.
func (p *QuestionPrepProcessor) EventType() string { return events.EventTypeQuestionPrepRequested }

// Process decodifica el evento, lee la fuente fresca, aplica la política de la
// escuela y orquesta la preparación. Errores:
//   - evento malformado → ErrMalformedPrepEvent (permanente → DLQ).
//   - prep-source/settings inaccesible → transitorio (el consumer reintenta).
func (p *QuestionPrepProcessor) Process(ctx context.Context, payload []byte) error {
	var evt events.QuestionPrepRequestedEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrMalformedPrepEvent, err)
	}
	if err := validatePrepEvent(evt); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedPrepEvent, err)
	}

	questionID := evt.Payload.QuestionID

	// Fuente fresca: el evento no trae contenido (D-042.3), se lee por M2M. Un fallo
	// aquí es transitorio salvo 404 (pregunta borrada → ErrLearningPermanent).
	src, err := p.learning.GetPrepSource(ctx, questionID)
	if err != nil {
		return fmt.Errorf("leyendo prep-source de la pregunta %s: %w", questionID, err)
	}

	// Política de la escuela (misma llave que el carril de corrección, D-042.8).
	settings, err := p.settings.GetSettings(ctx, src.SchoolID)
	if err != nil {
		return fmt.Errorf("leyendo settings de escuela %s: %w", src.SchoolID, err)
	}
	mode := settingValueOr(settings, settingKeyPrepMode, reviewModeOff)

	// Corto-circuito: corrección IA apagada ⇒ preparar es trabajo inútil (ACK).
	if mode == reviewModeOff {
		p.logger.Info("preparación apagada para la escuela (llm.review.mode=off), se ignora (ACK)",
			"question_id", questionID, "school_id", src.SchoolID, "reason", evt.Payload.Reason)
		return nil
	}

	return p.orchestrate(ctx, mode, evt.Payload.Reason, src)
}

// orchestrate ejecuta la preparación con la política resuelta. Idempotente por
// naturaleza (D-042.5): preparar dos veces produce el mismo artefacto y el PUT ancla
// por hash, así que reprocesar tras un fallo transitorio es seguro.
func (p *QuestionPrepProcessor) orchestrate(ctx context.Context, mode, reason string, src m2m.PrepSourceResponse) error {
	provider, ok := p.providers[mode]
	if !ok || provider == nil {
		// mode desconocido o provider no disponible: config errónea, permanente.
		return fmt.Errorf("%w: no hay LLMProvider para mode=%q", ErrMalformedPrepEvent, mode)
	}

	// Solo short_answer/open_ended tienen prep. Cualquier otro tipo es un evento que
	// no debió publicarse: permanente (no reintentar).
	if src.QuestionType != questionprep.QuestionTypeShortAnswer && src.QuestionType != questionprep.QuestionTypeOpenEnded {
		return fmt.Errorf("%w: question_type %q no admite preparación", ErrMalformedPrepEvent, src.QuestionType)
	}

	feedback := deref(src.LLMPrepFeedback)
	req := llm.PrepRequest{
		QuestionType:  src.QuestionType,
		QuestionText:  src.QuestionText,
		CorrectAnswer: deref(src.CorrectAnswer),
		Explanation:   deref(src.Explanation),
		Feedback:      feedback,
		Language:      prepLanguage,
	}

	rawPrep, err := provider.PrepareQuestion(ctx, req)
	if err != nil {
		// Fallo del LLM: transitorio. Aún no escribimos nada; reintentar es seguro.
		return fmt.Errorf("LLM preparando pregunta %s: %w", src.QuestionID, err)
	}

	// Validación de contrato ANTES del PUT: un prep inválido jamás se persiste
	// (envenenaría la corrección). Se trata como fallo del provider (transitorio).
	if _, verr := questionprep.Validate(rawPrep, src.QuestionType); verr != nil {
		p.logger.Warn("prep del LLM inválido, se descarta (no se persiste)",
			"question_id", src.QuestionID,
			"question_type", src.QuestionType,
			"provider", provider.Name(),
			"motivo", verr.Error(),
		)
		return fmt.Errorf("preparando pregunta %s: %w: %v", src.QuestionID, ErrInvalidPrep, verr)
	}

	// PUT con el source_hash CON EL QUE TRABAJAMOS (concurrencia optimista, D-042.5) y
	// consumed_feedback=true si el prompt usó el comentario del profesor (D-042.7).
	consumedFeedback := strings.TrimSpace(feedback) != ""
	err = p.learning.SavePrep(ctx, src.QuestionID, m2m.SavePrepRequest{
		LLMPrep:          rawPrep,
		SourceHash:       src.SourceHash,
		ConsumedFeedback: consumedFeedback,
	})
	if err != nil {
		// 409: el profesor editó en medio; el update ya re-encoló con datos frescos.
		// El prep viejo se descarta sin ruido (ACK).
		if errors.Is(err, m2m.ErrPrepHashConflict) {
			p.logger.Info("prep descartado: la pregunta se editó en medio (409), el update re-encoló (ACK)",
				"question_id", src.QuestionID, "reason", reason)
			return nil
		}
		return fmt.Errorf("persistiendo prep de la pregunta %s: %w", src.QuestionID, err)
	}

	p.logger.Info("pregunta preparada por LLM",
		"question_id", src.QuestionID,
		"question_type", src.QuestionType,
		"reason", reason,
		"mode", mode,
		"consumed_feedback", consumedFeedback,
		"provider", provider.Name(),
	)
	return nil
}

// validatePrepEvent comprueba los campos mínimos del evento del carril.
func validatePrepEvent(evt events.QuestionPrepRequestedEvent) error {
	if evt.EventType != events.EventTypeQuestionPrepRequested {
		return fmt.Errorf("event_type inesperado: %q", evt.EventType)
	}
	if evt.Payload.QuestionID == "" {
		return errors.New("question_id vacío")
	}
	if evt.Payload.AssessmentID == "" {
		return errors.New("assessment_id vacío")
	}
	return nil
}

// deref devuelve el string apuntado o "" si el puntero es nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
