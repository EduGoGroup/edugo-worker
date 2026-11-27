package processor

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	rabbit "github.com/EduGoGroup/edugo-shared/messaging/rabbit"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

// Líneas de log del aviso a learning (HU-IA.6). Son estables: los guiones las buscan.
const (
	FailureReportedLogMessage           = "aviso de fallo enviado a learning"
	FailureReportFailedLogMessage       = "no se pudo avisar el fallo a learning"
	NotApplicableReportedLogMessage     = "aviso «no aplica» enviado a learning"
	NotApplicableReportFailedLogMessage = "no se pudo avisar «no aplica» a learning"
	failureReportTimeout                = 10 * time.Second
	maxReportErrorBytes                 = 1024
)

// FailureReporter es la porción del cliente de rescate que usan el aviso y el «no
// aplica». *m2m.LearningRescueClient la satisface.
type FailureReporter interface {
	Report(ctx context.Context, req m2m.FailureReportRequest) (m2m.FailureReportResponse, error)
}

// MessageProcessor es lo que el handler del consumer llama con cada cuerpo: el
// registry, o el decorador que lo envuelve.
type MessageProcessor interface {
	Process(ctx context.Context, payload []byte) error
}

// FailureReportingProcessor envuelve el registry y avisa a learning cuando el mensaje va
// a morir (decisión 4 de la HU-IA.6): un error permanente es de dato; uno transitorio en
// el último reintento, de servicio; otro fallo no avisa. El aviso es de mejor esfuerzo y
// nunca cambia el error devuelto. El material no avisa (P11) y un mensaje irrutable o sin
// id tampoco: queda en la cola muerta.
type FailureReportingProcessor struct {
	next          MessageProcessor
	reporter      FailureReporter
	logger        logger.Logger
	isLastAttempt func(context.Context) bool
}

// NewFailureReportingProcessor construye el decorador. Sin reporter solo devuelve el error.
func NewFailureReportingProcessor(next MessageProcessor, reporter FailureReporter, log logger.Logger) *FailureReportingProcessor {
	return &FailureReportingProcessor{next: next, reporter: reporter, logger: log, isLastAttempt: lastAttemptFromDelivery}
}

// WithLastAttempt cambia cómo se sabe si la ejecución es la última (para los tests: el
// consumer es el único que deja la entrega en el contexto).
func (p *FailureReportingProcessor) WithLastAttempt(fn func(context.Context) bool) *FailureReportingProcessor {
	p.isLastAttempt = fn
	return p
}

func lastAttemptFromDelivery(ctx context.Context) bool {
	info, ok := rabbit.DeliveryFromContext(ctx)
	return ok && info.IsLastAttempt()
}

// Process satisface MessageProcessor.
func (p *FailureReportingProcessor) Process(ctx context.Context, payload []byte) error {
	err := p.next.Process(ctx, payload)
	if err == nil || p.reporter == nil || errors.Is(err, ErrUnroutableMessage) {
		return err
	}
	cause := ""
	switch {
	case IsPermanent(err):
		cause = m2m.RescueCauseData
	case p.isLastAttempt(ctx):
		cause = m2m.RescueCauseService
	default:
		return err
	}
	targetType, targetID := failureTarget(payload)
	if targetID == "" {
		return err
	}
	req := m2m.FailureReportRequest{
		TargetType: targetType, TargetID: targetID, Kind: m2m.RescueKindFailure,
		CauseClass: cause, Error: capUTF8(err.Error(), maxReportErrorBytes),
	}
	out, reportErr := sendReport(ctx, p.reporter, req)
	if reportErr != nil {
		p.logger.Warn(FailureReportFailedLogMessage, "target_type", targetType, "target_id", targetID,
			"cause_class", cause, "error", reportErr.Error())
		return err
	}
	p.logger.Info(FailureReportedLogMessage, "target_type", targetType, "target_id", targetID,
		"cause_class", cause, "status", out.Status)
	return err
}

// sendReport avisa con un contexto propio: la cancelación del mensaje (apagado del
// worker) no debe cortar el aviso.
func sendReport(ctx context.Context, r FailureReporter, req m2m.FailureReportRequest) (m2m.FailureReportResponse, error) {
	reportCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureReportTimeout)
	defer cancel()
	return r.Report(reportCtx, req)
}

// failureTarget saca el destino del cuerpo: el intento de una revisión o la pregunta de
// una preparación. Cualquier otro evento (el material) no tiene destino.
func failureTarget(payload []byte) (targetType, targetID string) {
	var body struct {
		EventType string `json:"event_type"`
		Payload   struct {
			AttemptID  string `json:"attempt_id"`
			QuestionID string `json:"question_id"`
		} `json:"payload"`
	}
	if json.Unmarshal(payload, &body) != nil {
		return "", ""
	}
	switch body.EventType {
	case EventTypeAttemptReviewRequested:
		return m2m.RescueTargetAttempt, body.Payload.AttemptID
	case events.EventTypeQuestionPrepRequested:
		return m2m.RescueTargetQuestion, body.Payload.QuestionID
	}
	return "", ""
}

// reportNotApplicable avisa «no aplica» (la IA apagada en la escuela, decisión 6). Es de
// mejor esfuerzo: el llamador confirma el mensaje aunque el aviso falle.
func reportNotApplicable(ctx context.Context, r FailureReporter, log logger.Logger, targetType, targetID string) {
	if r == nil {
		return
	}
	out, err := sendReport(ctx, r, m2m.FailureReportRequest{
		TargetType: targetType, TargetID: targetID, Kind: m2m.RescueKindNotApplicable,
	})
	if err != nil {
		log.Warn(NotApplicableReportFailedLogMessage, "target_type", targetType, "target_id", targetID, "error", err.Error())
		return
	}
	log.Info(NotApplicableReportedLogMessage, "target_type", targetType, "target_id", targetID, "status", out.Status)
}

// WithFailureReporter fija a quién se avisa el «no aplica» de la revisión apagada.
func (p *AttemptReviewProcessor) WithFailureReporter(r FailureReporter) *AttemptReviewProcessor {
	p.failures = r
	return p
}

// WithFailureReporter fija a quién se avisa el «no aplica» de la preparación apagada.
func (p *QuestionPrepProcessor) WithFailureReporter(r FailureReporter) *QuestionPrepProcessor {
	p.failures = r
	return p
}

// capUTF8 acota s a limit bytes sin partir una runa.
func capUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
