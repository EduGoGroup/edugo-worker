package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/google/uuid"
)

// AssessmentAssignedNotifProcessor handles "assessment.assigned" events
// and creates in-app notifications for the assigned student(s).
// When target_type is "student", a single notification is created.
// When target_type is "unit", enrolled students are resolved from the
// academic.memberships table and each one receives a notification.
type AssessmentAssignedNotifProcessor struct {
	db     *sql.DB
	nc     *NotificationCreator
	logger logger.Logger
}

// NewAssessmentAssignedNotifProcessor creates a new processor.
func NewAssessmentAssignedNotifProcessor(db *sql.DB, nc *NotificationCreator, logger logger.Logger) *AssessmentAssignedNotifProcessor {
	return &AssessmentAssignedNotifProcessor{db: db, nc: nc, logger: logger}
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

	switch pl.TargetType {
	case "student":
		return p.notifyStudent(ctx, pl)
	case "unit":
		return p.notifyUnit(ctx, pl)
	default:
		p.logger.Warn("skipping assessment.assigned notification: unsupported target_type",
			"target_type", pl.TargetType,
			"assessment_id", pl.AssessmentID,
		)
		return nil
	}
}

// notifyStudent creates a single notification for a direct student assignment.
func (p *AssessmentAssignedNotifProcessor) notifyStudent(ctx context.Context, pl dto.AssessmentAssignedNotifPayload) error {
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

// notifyUnit resolves all students enrolled in the given academic unit and
// creates a notification for each one. Partial failures are aggregated and
// returned as a single error; students that succeed are not rolled back.
func (p *AssessmentAssignedNotifProcessor) notifyUnit(ctx context.Context, pl dto.AssessmentAssignedNotifPayload) error {
	unitID, err := uuid.Parse(pl.TargetID)
	if err != nil {
		return fmt.Errorf("invalid target_id (unit): %w", err)
	}

	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}

	studentIDs, err := p.resolveStudentIDs(ctx, unitID)
	if err != nil {
		return fmt.Errorf("resolving unit students: %w", err)
	}

	if len(studentIDs) == 0 {
		p.logger.Warn("unit has no enrolled students",
			"unit_id", unitID.String(),
			"assessment_id", assessmentID.String(),
		)
		return nil
	}

	title := "Nueva evaluacion asignada"
	body := fmt.Sprintf("Te han asignado: %s", pl.Title)

	var errs []error
	for _, studentID := range studentIDs {
		if err := p.nc.Create(ctx, studentID, "assessment_assigned", title, body, "assessment", assessmentID); err != nil {
			errs = append(errs, err)
			p.logger.Error("failed to notify student in unit",
				"student_id", studentID.String(),
				"unit_id", unitID.String(),
				"error", err.Error(),
			)
		}
	}

	p.logger.Info("unit assessment.assigned notifications processed",
		"unit_id", unitID.String(),
		"assessment_id", assessmentID.String(),
		"total_students", len(studentIDs),
		"errors", len(errs),
	)

	if len(errs) > 0 {
		return fmt.Errorf("failed to notify %d/%d students in unit %s", len(errs), len(studentIDs), unitID)
	}
	return nil
}

// resolveStudentIDs returns the user IDs of all active students enrolled in
// the given academic unit by querying academic.memberships.
func (p *AssessmentAssignedNotifProcessor) resolveStudentIDs(ctx context.Context, unitID uuid.UUID) ([]uuid.UUID, error) {
	const query = `SELECT user_id FROM academic.memberships
	               WHERE academic_unit_id = $1 AND role = 'student' AND is_active = true`

	rows, err := p.db.QueryContext(ctx, query, unitID)
	if err != nil {
		return nil, fmt.Errorf("querying enrolled students: %w", err)
	}
	defer rows.Close()

	var studentIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning student id: %w", err)
		}
		studentIDs = append(studentIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating student rows: %w", err)
	}

	return studentIDs, nil
}
