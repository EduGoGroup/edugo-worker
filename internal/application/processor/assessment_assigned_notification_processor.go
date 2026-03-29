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

// AssessmentAssignedNotifProcessor handles "assessment.assigned" events
// and creates in-app notifications for the assigned student.
type AssessmentAssignedNotifProcessor struct {
	nc     *NotificationCreator
	logger logger.Logger
}

// NewAssessmentAssignedNotifProcessor creates a new processor.
func NewAssessmentAssignedNotifProcessor(nc *NotificationCreator, logger logger.Logger) *AssessmentAssignedNotifProcessor {
	return &AssessmentAssignedNotifProcessor{nc: nc, logger: logger}
}

func (p *AssessmentAssignedNotifProcessor) EventType() string {
	return "assessment.assigned"
}

func (p *AssessmentAssignedNotifProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.AssessmentAssignedNotifEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentAssignedNotifProcessor) processEvent(ctx context.Context, event dto.AssessmentAssignedNotifEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.assigned notification",
		"assessment_id", pl.AssessmentID,
		"target_type", pl.TargetType,
		"target_id", pl.TargetID,
	)

	// Only handle direct student assignments for now.
	// Unit-level assignments require resolving enrolled students, which is planned for a future phase.
	if pl.TargetType != "student" {
		p.logger.Warn("skipping assessment.assigned notification: unsupported target_type",
			"target_type", pl.TargetType,
			"assessment_id", pl.AssessmentID,
		)
		return nil
	}

	studentID, err := uuid.Parse(pl.TargetID)
	if err != nil {
		return fmt.Errorf("invalid target_id (student): %w", err)
	}

	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}

	title := "Nueva evaluacion asignada"
	body := fmt.Sprintf("Te han asignado: %s", pl.Title)

	if err := p.nc.Create(ctx, studentID, "assessment_assigned", title, body, "assessment", assessmentID); err != nil {
		return fmt.Errorf("failed to create assessment assigned notification: %w", err)
	}

	p.logger.Info("assessment.assigned notification processed successfully",
		"student_id", studentID.String(),
		"assessment_id", assessmentID.String(),
	)
	return nil
}
