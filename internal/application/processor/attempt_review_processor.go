package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
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

// Valores del carril de revisión (design 040 §rieles).
const (
	// mode
	reviewModeOff   = "off"   // revisión apagada (default de plataforma)
	reviewModeLocal = "local" // provider LLM local (Ollama)
	reviewModeAPI   = "api"   // provider LLM por API (Anthropic)
	// flow
	reviewFlowTeacher = "teacher" // deja el intento ai_reviewed para el profesor (default)
	reviewFlowDirect  = "direct"  // finaliza el intento tras revisar (sin docente)
)

// reviewLanguage es el idioma que se pide al LLM para el feedback. El carril es
// español (regla global del ecosistema).
const reviewLanguage = "es"

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

// LearningReviewClient es la porción del LearningClient M2M que usa el processor
// para orquestar la revisión. Se define como interfaz para mockearla en tests.
// *m2m.LearningClient la satisface.
type LearningReviewClient interface {
	GetPendingAnswers(ctx context.Context, attemptID string) (m2m.PendingAnswersResponse, error)
	PostAnswerReview(ctx context.Context, attemptID, answerID string, review m2m.AnswerReviewRequest) (m2m.AnswerReviewResponse, error)
	FinalizeAttempt(ctx context.Context, attemptID string) (m2m.FinalizeResponse, error)
}

// AttemptReviewProcessor consume attempt.review_requested y orquesta la revisión
// asistida por LLM de un intento entregado.
//
// Flujo (F2): decodifica/valida el evento, lee la política de la escuela y, si la
// revisión está activa (mode=local|api), lee las respuestas pendientes de learning
// (M2M), las corrige con el LLMProvider del mode y escribe cada review de vuelta.
// Con flow=direct finaliza el intento; con flow=teacher lo deja ai_reviewed para el
// profesor (F3).
type AttemptReviewProcessor struct {
	settings  SchoolSettingsReader
	learning  LearningReviewClient
	providers map[string]llm.LLMProvider
	logger    logger.Logger
}

// NewAttemptReviewProcessor construye el processor. providers mapea el mode
// ("local"/"api") al LLMProvider correspondiente; un mode sin provider disponible
// se trata como configuración inválida al procesar.
func NewAttemptReviewProcessor(
	settings SchoolSettingsReader,
	learning LearningReviewClient,
	providers map[string]llm.LLMProvider,
	log logger.Logger,
) *AttemptReviewProcessor {
	return &AttemptReviewProcessor{
		settings:  settings,
		learning:  learning,
		providers: providers,
		logger:    log,
	}
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

	return p.orchestrate(ctx, evt.Payload.AttemptID, mode, flow)
}

// orchestrate ejecuta la revisión asistida de un intento con la política resuelta.
//
// Idempotencia y retry (gate de tasks.md): todo el flujo es seguro de reprocesar.
// El GET re-lee solo las respuestas AÚN pendientes y el POST review es upsert del
// lado de learning, así que un redelivery tras un fallo a mitad no duplica ni
// corrompe. Por eso los fallos transitorios (LLM o M2M 5xx/red) se devuelven como
// error para que el consumer reintente. Un 4xx permanente de learning
// (ErrLearningPermanent) o un mode sin provider se envuelven en un error que el
// clasificador marca permanente; aun así el consumer con DLQ reintenta MaxRetries
// antes de mandar el mensaje al DLQ (ConsumeWithDLQ no consulta el clasificador),
// lo cual es inofensivo porque el reproceso es idempotente.
func (p *AttemptReviewProcessor) orchestrate(ctx context.Context, attemptID, mode, flow string) error {
	provider, ok := p.providers[mode]
	if !ok || provider == nil {
		// mode desconocido o provider no disponible: es config errónea, no un fallo
		// transitorio. Permanente para no reintentar en vano.
		return fmt.Errorf("%w: no hay LLMProvider para mode=%q", ErrMalformedEvent, mode)
	}

	pending, err := p.learning.GetPendingAnswers(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("leyendo answers pendientes de attempt %s: %w", attemptID, err)
	}

	// Sin pendientes: nada que corregir. Puede ser un redelivery de un intento ya
	// revisado. En flow=direct intentamos finalize (idempotente) para cerrar el
	// intento si quedó a medias; en flow=teacher solo ACK.
	if len(pending.Answers) == 0 {
		if flow == reviewFlowDirect {
			return p.finalize(ctx, attemptID, "sin pendientes (posible redelivery)")
		}
		p.logger.Info("review sin pendientes, flow teacher: ACK",
			"attempt_id", attemptID, "mode", mode, "flow", flow)
		return nil
	}

	for _, ans := range pending.Answers {
		result, err := provider.ReviewAnswer(ctx, llm.ReviewRequest{
			QuestionText:   ans.QuestionText,
			ExpectedAnswer: ans.ExpectedAnswer,
			Rubric:         ans.Rubric,
			StudentAnswer:  ans.StudentAnswer,
			Language:       reviewLanguage,
		})
		if err != nil {
			// Fallo del LLM: transitorio. Reintentar es seguro (aún no escribimos esta
			// review; las ya escritas no vuelven a aparecer en el GET pending).
			return fmt.Errorf("LLM revisando answer %s (attempt %s): %w", ans.AnswerID, attemptID, err)
		}

		points := scaledPoints(result.Score, ans.Points)
		if _, err := p.learning.PostAnswerReview(ctx, attemptID, ans.AnswerID, m2m.AnswerReviewRequest{
			PointsAwarded: points,
			Feedback:      result.Feedback,
		}); err != nil {
			return fmt.Errorf("escribiendo review de answer %s (attempt %s): %w", ans.AnswerID, attemptID, err)
		}

		p.logger.Info("answer revisada por LLM",
			"attempt_id", attemptID,
			"answer_id", ans.AnswerID,
			"verdict", string(result.Verdict),
			"score", result.Score,
			"points_awarded", points,
			"provider", provider.Name(),
		)
	}

	// Todas las pendientes quedaron revisadas.
	if flow == reviewFlowDirect {
		return p.finalize(ctx, attemptID, "todas las respuestas revisadas")
	}

	// flow=teacher: NO finalize. El intento queda ai_reviewed para el profesor (F3).
	p.logger.Info("review completada, flow teacher: intento queda para el profesor (sin finalize)",
		"attempt_id", attemptID, "answers", len(pending.Answers))
	return nil
}

// finalize cierra la revisión del intento (flow direct). Es idempotente del lado
// de learning (200 no-op si ya estaba completed).
func (p *AttemptReviewProcessor) finalize(ctx context.Context, attemptID, reason string) error {
	resp, err := p.learning.FinalizeAttempt(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("finalizando attempt %s: %w", attemptID, err)
	}
	p.logger.Info("attempt finalizado (flow direct)",
		"attempt_id", attemptID, "status", resp.Status, "motivo", reason)
	return nil
}

// scaledPoints escala el Score del LLM (fracción 0..1) al puntaje real de la
// pregunta y lo redondea a 2 decimales (paso mínimo razonable para puntajes
// escolares; evita colas binarias tipo 4.999999). El Score se acota a [0,1] por
// si el modelo devuelve un valor fuera de rango.
func scaledPoints(score, questionPoints float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*questionPoints*100) / 100
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
