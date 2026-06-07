package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/google/uuid"
)

// AssessmentAssignedNotifProcessor handles "assessment.assigned" events
// and creates in-app notifications for the students enrolled in the
// assigned subject offering (session). In N4 an assessment is always
// assigned to a subject offering; the recipients are the active students
// enrolled in that offering.
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
	var event events.AssessmentAssignedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

// processEvent resuelve los alumnos inscritos en la oferta y notifica a cada uno.
// Los fallos parciales se agregan y se devuelven como un unico error; los alumnos
// que ya recibieron notificacion no se revierten.
func (p *AssessmentAssignedNotifProcessor) processEvent(ctx context.Context, event events.AssessmentAssignedEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.assigned notification",
		"assessment_id", pl.AssessmentID,
		"subject_offering_id", pl.SubjectOfferingID,
	)

	// Timeout para la operacion masiva de notificaciones
	// (200 alumnos * ~50ms cada uno = ~10s max).
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	offeringID, err := uuid.Parse(pl.SubjectOfferingID)
	if err != nil {
		return fmt.Errorf("invalid subject_offering_id: %w", err)
	}

	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}

	studentIDs, err := p.resolveEnrolledStudentIDs(ctx, offeringID)
	if err != nil {
		return fmt.Errorf("resolving offering students: %w", err)
	}

	if len(studentIDs) == 0 {
		p.logger.Info("subject offering has no enrolled students",
			"subject_offering_id", offeringID.String(),
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
			p.logger.Error("failed to notify enrolled student",
				"student_id", studentID.String(),
				"subject_offering_id", offeringID.String(),
				"error", err.Error(),
			)
		}
	}

	p.logger.Info("assessment.assigned notifications processed",
		"subject_offering_id", offeringID.String(),
		"assessment_id", assessmentID.String(),
		"total_students", len(studentIDs),
		"errors", len(errs),
	)

	if len(errs) > 0 {
		summary := fmt.Errorf("failed to notify %d/%d students in offering %s", len(errs), len(studentIDs), offeringID)
		return stderrors.Join(append([]error{summary}, errs...)...)
	}
	return nil
}

// resolveEnrolledStudentIDs devuelve los user IDs de los alumnos activos
// inscritos en la oferta dada, uniendo la tabla de inscripciones con la de
// membresias. La tabla subject_offering_enrollments no tiene soft-delete
// (una baja es un DELETE real con CASCADE), por lo que el filtro de actividad
// se aplica sobre la membresia.
func (p *AssessmentAssignedNotifProcessor) resolveEnrolledStudentIDs(ctx context.Context, offeringID uuid.UUID) ([]uuid.UUID, error) {
	const query = `SELECT DISTINCT m.user_id
	               FROM academic.subject_offering_enrollments e
	               JOIN academic.memberships m ON m.id = e.student_membership_id
	               WHERE e.offering_id = $1
	                 AND m.is_active = true`

	rows, err := p.db.QueryContext(ctx, query, offeringID)
	if err != nil {
		return nil, fmt.Errorf("querying enrolled students: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
