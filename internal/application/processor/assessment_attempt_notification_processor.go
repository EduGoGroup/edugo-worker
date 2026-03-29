package processor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/google/uuid"
)

// AssessmentAttemptNotifProcessor handles "assessment.attempt_recorded" events
// and creates in-app notifications for the teacher.
type AssessmentAttemptNotifProcessor struct {
	nc     *NotificationCreator
	logger logger.Logger
}

// NewAssessmentAttemptNotifProcessor creates a new processor.
func NewAssessmentAttemptNotifProcessor(nc *NotificationCreator, logger logger.Logger) *AssessmentAttemptNotifProcessor {
	return &AssessmentAttemptNotifProcessor{nc: nc, logger: logger}
}

func (p *AssessmentAttemptNotifProcessor) EventType() string {
	return "assessment.attempt_recorded"
}

func (p *AssessmentAttemptNotifProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.AssessmentAttemptNotifEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentAttemptNotifProcessor) processEvent(ctx context.Context, event dto.AssessmentAttemptNotifEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.attempt_recorded notification",
		"attempt_id", pl.AttemptID,
		"assessment_id", pl.AssessmentID,
		"student_id", pl.StudentID,
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

	title := "Evaluacion enviada"
	body := fmt.Sprintf("Un estudiante ha enviado: %s", pl.Title)

	if err := p.nc.Create(ctx, teacherID, "assessment_attempt_recorded", title, body, "assessment_attempt", attemptID); err != nil {
		return fmt.Errorf("failed to create assessment attempt notification: %w", err)
	}

	p.logger.Info("assessment.attempt_recorded notification processed successfully",
		"teacher_id", teacherID.String(),
		"attempt_id", attemptID.String(),
	)
	return nil
}
