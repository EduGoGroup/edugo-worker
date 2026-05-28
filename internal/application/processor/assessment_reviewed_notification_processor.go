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

// AssessmentReviewedNotifProcessor handles "assessment.reviewed" events
// and creates in-app notifications for the student.
type AssessmentReviewedNotifProcessor struct {
	nc     *NotificationCreator
	logger logger.Logger
}

// NewAssessmentReviewedNotifProcessor creates a new processor.
func NewAssessmentReviewedNotifProcessor(nc *NotificationCreator, logger logger.Logger) *AssessmentReviewedNotifProcessor {
	return &AssessmentReviewedNotifProcessor{nc: nc, logger: logger}
}

func (p *AssessmentReviewedNotifProcessor) EventType() string {
	return "assessment.reviewed"
}

func (p *AssessmentReviewedNotifProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.AssessmentReviewedNotifEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentReviewedNotifProcessor) processEvent(ctx context.Context, event dto.AssessmentReviewedNotifEvent) error {
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

	title := "Evaluacion calificada"
	body := fmt.Sprintf("Tu evaluacion ha sido calificada: %s", pl.Title)

	if err := p.nc.Create(ctx, studentID, "assessment_reviewed", title, body, "assessment_attempt", attemptID); err != nil {
		return fmt.Errorf("failed to create assessment reviewed notification: %w", err)
	}

	p.logger.Info("assessment.reviewed notification processed successfully",
		"student_id", studentID.String(),
		"attempt_id", attemptID.String(),
	)
	return nil
}
