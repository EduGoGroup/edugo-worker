package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

// EventTypeAttemptReviewRequested es el event_type que enruta el registry hacia
// este processor. Debe coincidir con la routing key del binding en setupRabbitMQ
// y con el event_type que publica learning.
const EventTypeAttemptReviewRequested = "attempt.review_requested"

// Claves de política por escuela leídas vía SettingsClient (design 039/040 §rieles).
const (
	settingKeyReviewMode = "llm.review.mode" // local | api | off
	settingKeyReviewFlow = "llm.review.flow" // direct | teacher
)

// Defaults de plataforma cuando la escuela no fija la clave (design 040): revisión
// apagada y, si se enciende, con validación docente.
const (
	reviewModeOff     = "off"
	reviewFlowTeacher = "teacher"
)

// ErrMalformedEvent marca un evento que no se puede decodificar/validar. El
// clasificador de retry lo trata como permanente: reintentar no lo arregla, va al
// DLQ. Ver classifyError en retry.go.
var ErrMalformedEvent = errors.New("evento attempt.review_requested malformado")

// SchoolSettingsReader es la porción del SettingsClient M2M que usa el processor.
// Se define como interfaz para poder mockearla en tests. *m2m.SettingsClient la
// satisface.
type SchoolSettingsReader interface {
	GetSettings(ctx context.Context, schoolID string) (m2m.SchoolSettings, error)
}

// AttemptReviewProcessor consume attempt.review_requested y arranca la revisión
// asistida por LLM de un intento entregado.
//
// Estado F1 (esqueleto): decodifica/valida el evento, lee la política de la escuela
// y corta-circuito si la revisión está apagada. La orquestación real (llamada a
// LearningClient + LLM) llega en F2; aquí solo se loguea la continuación.
type AttemptReviewProcessor struct {
	settings SchoolSettingsReader
	logger   logger.Logger
}

// NewAttemptReviewProcessor construye el processor.
func NewAttemptReviewProcessor(settings SchoolSettingsReader, log logger.Logger) *AttemptReviewProcessor {
	return &AttemptReviewProcessor{settings: settings, logger: log}
}

// EventType satisface processor.Processor.
func (p *AttemptReviewProcessor) EventType() string { return EventTypeAttemptReviewRequested }

// Process decodifica el evento, aplica la política de la escuela y (F1) se detiene
// antes de orquestar. Errores:
//   - evento malformado (decode/validación) → ErrMalformedEvent (permanente → DLQ).
//   - settings inaccesible → error transitorio (el consumer reintenta con backoff).
func (p *AttemptReviewProcessor) Process(ctx context.Context, payload []byte) error {
	var evt events.AttemptReviewRequestedEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrMalformedEvent, err)
	}

	if err := validateReviewEvent(evt); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedEvent, err)
	}

	// Leer política de la escuela. Un fallo aquí es transitorio (academic caído,
	// timeout): se devuelve tal cual para que el consumer reintente.
	settings, err := p.settings.GetSettings(ctx, evt.Payload.SchoolID)
	if err != nil {
		return fmt.Errorf("leyendo settings de escuela %s: %w", evt.Payload.SchoolID, err)
	}

	mode := settingValueOr(settings, settingKeyReviewMode, reviewModeOff)
	flow := settingValueOr(settings, settingKeyReviewFlow, reviewFlowTeacher)

	// Corto-circuito: revisión apagada para esta escuela.
	if mode == reviewModeOff {
		p.logger.Info("review apagado para la escuela, se ignora (ACK)",
			"attempt_id", evt.Payload.AttemptID,
			"school_id", evt.Payload.SchoolID,
			"mode", mode,
		)
		return nil
	}

	// F1: modo activo (local|api). La orquestación (LearningClient + LLM) llega en F2.
	p.logger.Info("review solicitado, orquestación llega en F2",
		"attempt_id", evt.Payload.AttemptID,
		"assessment_id", evt.Payload.AssessmentID,
		"school_id", evt.Payload.SchoolID,
		"answers", len(evt.Payload.Answers),
		"mode", mode,
		"flow", flow,
	)
	return nil
}

// validateReviewEvent comprueba los campos mínimos que necesita el carril. No
// reusa el constructor del shared porque ese regenera el Timestamp.
func validateReviewEvent(evt events.AttemptReviewRequestedEvent) error {
	if evt.EventType != EventTypeAttemptReviewRequested {
		return fmt.Errorf("event_type inesperado: %q", evt.EventType)
	}
	if evt.Payload.AttemptID == "" {
		return errors.New("attempt_id vacío")
	}
	if evt.Payload.AssessmentID == "" {
		return errors.New("assessment_id vacío")
	}
	if evt.Payload.SchoolID == "" {
		return errors.New("school_id vacío")
	}
	if len(evt.Payload.Answers) == 0 {
		return errors.New("answers vacío")
	}
	return nil
}

// settingValueOr devuelve el valor resuelto de una clave o el default de plataforma.
func settingValueOr(s m2m.SchoolSettings, key, def string) string {
	if v, ok := s.Get(key); ok && v != "" {
		return v
	}
	return def
}
