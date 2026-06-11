package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/google/uuid"
)

// AssessmentReviewedNotifProcessor handles "assessment.reviewed" events and
// delegates the student notification to the Notification Gateway (platform).
type AssessmentReviewedNotifProcessor struct {
	dispatcher NotificationDispatcher
	logger     logger.Logger
}

// NewAssessmentReviewedNotifProcessor creates a new processor.
func NewAssessmentReviewedNotifProcessor(dispatcher NotificationDispatcher, logger logger.Logger) *AssessmentReviewedNotifProcessor {
	return &AssessmentReviewedNotifProcessor{dispatcher: dispatcher, logger: logger}
}

func (p *AssessmentReviewedNotifProcessor) EventType() string {
	return "assessment.reviewed"
}

func (p *AssessmentReviewedNotifProcessor) Process(ctx context.Context, payload []byte) error {
	var event events.AssessmentReviewedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentReviewedNotifProcessor) processEvent(ctx context.Context, event events.AssessmentReviewedEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.reviewed notification",
		"attempt_id", pl.AttemptID,
		"assessment_id", pl.AssessmentID,
		"reviewer_id", pl.ReviewerID,
	)

	if pl.StudentID == "" {
		p.logger.Warn("skipping assessment.reviewed notification: missing student_id",
			"attempt_id", pl.AttemptID,
			"assessment_id", pl.AssessmentID,
		)
		return nil
	}

	studentID, err := uuid.Parse(pl.StudentID)
	if err != nil {
		return fmt.Errorf("invalid student_id: %w", err)
	}

	attemptID, err := uuid.Parse(pl.AttemptID)
	if err != nil {
		return fmt.Errorf("invalid attempt_id: %w", err)
	}

	req := client.DispatchRequest{
		IdempotencyKey: "assessment.reviewed:" + attemptID.String(),
		Recipients:     []client.DispatchRecipient{{UserID: studentID.String()}},
		Notification: client.DispatchNotification{
			Type:         "assessment_reviewed",
			Title:        "Evaluacion calificada",
			Body:         fmt.Sprintf("Tu evaluacion ha sido calificada: %s", pl.Title),
			ResourceType: "assessment_attempt",
			ResourceID:   attemptID.String(),
			// Tenant (F4.6): school_id y academic_unit_id vienen del evento, que los
			// toma del contexto activo del emisor. El productor manda el tenant; el
			// worker no infiere ni consulta BD. Así el push lleva unit_id y el deep-link
			// cambia al contexto exacto (escuela+unidad).
			SchoolID: pl.SchoolID,
			UnitID:   pl.AcademicUnitID,
		},
		Channels: &client.DispatchChannels{InApp: true, Push: true},
		PushData: map[string]string{"event_type": p.EventType()},
		Source:   &client.DispatchSource{Caller: dispatchCaller, CorrelationID: event.EventID},
	}

	p.logger.Info("dispatch_requested",
		"event_type", p.EventType(),
		"attempt_id", attemptID.String(),
		"student_id", studentID.String(),
		"recipients", 1,
	)

	if err := p.dispatcher.Dispatch(ctx, req); err != nil {
		return fmt.Errorf("dispatching assessment.reviewed notification: %w", err)
	}

	return nil
}
