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

// AssessmentAttemptNotifProcessor handles "assessment.attempt_recorded" events
// and delegates the teacher notification to the Notification Gateway (platform).
type AssessmentAttemptNotifProcessor struct {
	dispatcher NotificationDispatcher
	logger     logger.Logger
}

// NewAssessmentAttemptNotifProcessor creates a new processor.
func NewAssessmentAttemptNotifProcessor(dispatcher NotificationDispatcher, logger logger.Logger) *AssessmentAttemptNotifProcessor {
	return &AssessmentAttemptNotifProcessor{dispatcher: dispatcher, logger: logger}
}

func (p *AssessmentAttemptNotifProcessor) EventType() string {
	return "assessment.attempt_recorded"
}

func (p *AssessmentAttemptNotifProcessor) Process(ctx context.Context, payload []byte) error {
	var event events.AssessmentAttemptRecordedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentAttemptNotifProcessor) processEvent(ctx context.Context, event events.AssessmentAttemptRecordedEvent) error {
	pl := event.Payload

	p.logger.Info("procesando notificacion assessment.attempt_recorded",
		"attempt_id", pl.AttemptID,
		"assessment_id", pl.AssessmentID,
		"student_membership_id", pl.StudentMembershipID,
	)

	if pl.TeacherID == "" {
		p.logger.Warn("skipping assessment.attempt_recorded notification: missing teacher_id",
			"attempt_id", pl.AttemptID,
			"assessment_id", pl.AssessmentID,
		)
		return nil
	}

	teacherID, err := uuid.Parse(pl.TeacherID)
	if err != nil {
		return fmt.Errorf("invalid teacher_id: %w", err)
	}

	attemptID, err := uuid.Parse(pl.AttemptID)
	if err != nil {
		return fmt.Errorf("invalid attempt_id: %w", err)
	}

	req := client.DispatchRequest{
		IdempotencyKey: "assessment.attempt_recorded:" + attemptID.String(),
		Recipients:     []client.DispatchRecipient{{UserID: teacherID.String()}},
		Notification: client.DispatchNotification{
			Type:         "assessment_attempt_recorded",
			Title:        "Evaluacion enviada",
			Body:         fmt.Sprintf("Un estudiante ha enviado: %s", pl.Title),
			ResourceType: "assessment_attempt",
			ResourceID:   attemptID.String(),
		},
		Channels: &client.DispatchChannels{InApp: true, Push: true},
		PushData: map[string]string{"event_type": p.EventType()},
		Source:   &client.DispatchSource{Caller: dispatchCaller, CorrelationID: event.EventID},
	}

	p.logger.Info("dispatch_requested",
		"event_type", p.EventType(),
		"attempt_id", attemptID.String(),
		"teacher_id", teacherID.String(),
		"recipients", 1,
	)

	if err := p.dispatcher.Dispatch(ctx, req); err != nil {
		return fmt.Errorf("dispatching assessment.attempt_recorded notification: %w", err)
	}

	return nil
}
