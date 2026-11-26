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

// ErrInvalidPrep marca un artefacto del LLM que no cumple el contrato v1 tras el único
// reintento a llm.InvalidOutputRetryTemperature (HU-IA.4, P9.2): fallo de dato,
// permanente (cola muerta sin reintentos) y la pregunta queda failed. NUNCA se hace
// PUT de un prep malformado (envenenaría la corrección, D-042.2).
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
	// SetPrepOutcome cierra la preparación en failed o none sin artefacto (HU-IA.4).
	SetPrepOutcome(ctx context.Context, questionID string, req m2m.SetPrepStatusRequest) (m2m.SetPrepStatusResponse, error)
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
	// aquí es transitorio salvo 404 (pregunta borrada → ErrLearningPermanent). Sin
	// fuente no hay hash: no se escribe ningún estado.
	src, err := p.learning.GetPrepSource(ctx, questionID)
	if err != nil {
		return fmt.Errorf("leyendo prep-source de la pregunta %s: %w", questionID, err)
	}

	// Solo se salta lo ready (HU-IA.4): ready equivale a «el artefacto guardado es el
	// de este hash», porque todo camino que pide trabajo pone pending antes de
	// publicar. Una pending o una failed se ejecutan aunque el hash coincida.
	if src.LLMPrepStatus == m2m.PrepStatusReady {
		p.logger.Info("preparación ya lista, no se repite",
			"question_id", questionID, "reason", evt.Payload.Reason)
		return nil
	}

	// Política de la escuela (misma llave que el carril de corrección, D-042.8).
	settings, err := p.settings.GetSettings(ctx, src.SchoolID)
	if err != nil {
		return fmt.Errorf("leyendo settings de escuela %s: %w", src.SchoolID, err)
	}
	mode := settingValueOr(settings, settingKeyPrepMode, reviewModeOff)

	// Corto-circuito: corrección IA apagada ⇒ preparar es trabajo inútil. La
	// pregunta queda none (HU-IA.4); si esa escritura falla, el mensaje se reintenta.
	if mode == reviewModeOff {
		if _, err := p.learning.SetPrepOutcome(ctx, questionID, m2m.SetPrepStatusRequest{
			Status: m2m.PrepStatusNone, SourceHash: src.SourceHash,
		}); err != nil {
			return fmt.Errorf("marcando none la pregunta %s: %w", questionID, err)
		}
		p.logger.Info("preparación apagada para la escuela (llm.review.mode=off), se ignora (ACK)",
			"question_id", questionID, "school_id", src.SchoolID, "reason", evt.Payload.Reason)
		return nil
	}

	err = p.orchestrate(ctx, mode, evt.Payload.Reason, src)
	if IsPermanent(err) {
		// Fallo de dato: failed ANTES de devolver el error, para que el aviso de la
		// HU-IA.6 llegue después. Un fallo de servicio deja pending (su dueño es la
		// fila de lo fallado). Si la escritura falla, sale el error original.
		p.markFailed(ctx, src, err)
	}
	return err
}

// markFailed escribe failed con el hash leído. Un fallo al escribir solo se loguea:
// la pregunta queda pending para el barrido de la HU-IA.6.
func (p *QuestionPrepProcessor) markFailed(ctx context.Context, src m2m.PrepSourceResponse, cause error) {
	out, err := p.learning.SetPrepOutcome(ctx, src.QuestionID, m2m.SetPrepStatusRequest{
		Status: m2m.PrepStatusFailed, SourceHash: src.SourceHash,
	})
	if err != nil {
		p.logger.Warn("no se pudo marcar failed la preparación (queda pending)",
			"question_id", src.QuestionID, "error", err.Error(), "motivo", cause.Error())
		return
	}
	p.logger.Info("preparación marcada failed",
		"question_id", src.QuestionID, "applied", out.Applied, "conflict", out.Conflict, "motivo", cause.Error())
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

	rawPrep, err := p.prepareValid(ctx, provider, req, src)
	if err != nil {
		return err
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

// prepareValid pide el artefacto y lo valida contra el contrato v1. Una salida
// inválida o un fallo de calidad (llm.ErrLLMQuality, p. ej. el bucle de repetición)
// se reintenta UNA vez a llm.InvalidOutputRetryTemperature; si persiste, ErrInvalidPrep
// (fallo de dato). Un fallo de transporte sale tal cual (transitorio).
func (p *QuestionPrepProcessor) prepareValid(ctx context.Context, provider llm.LLMProvider, req llm.PrepRequest, src m2m.PrepSourceResponse) (json.RawMessage, error) {
	var cause error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			t := llm.InvalidOutputRetryTemperature
			req.Temperature = &t
		}
		rawPrep, err := provider.PrepareQuestion(ctx, req)
		if err != nil {
			if !errors.Is(err, llm.ErrLLMQuality) {
				return nil, fmt.Errorf("LLM preparando pregunta %s: %w", src.QuestionID, err)
			}
			cause = err
		} else if _, verr := questionprep.Validate(rawPrep, src.QuestionType); verr != nil {
			cause = verr
		} else {
			return rawPrep, nil
		}
		p.logger.Warn("prep del LLM inválido, se descarta (no se persiste)",
			"question_id", src.QuestionID,
			"question_type", src.QuestionType,
			"provider", provider.Name(),
			"attempt", attempt+1,
			"motivo", cause.Error(),
		)
	}
	return nil, fmt.Errorf("preparando pregunta %s: %w: %v", src.QuestionID, ErrInvalidPrep, cause)
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
