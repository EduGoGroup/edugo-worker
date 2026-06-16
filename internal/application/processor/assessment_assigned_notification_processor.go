package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/google/uuid"
)

// AssessmentAssignedNotifProcessor handles "assessment.assigned" events and
// delegates notification delivery to the Notification Gateway (platform) for the
// students enrolled in the assigned subject offering. The worker only resolves
// recipients; it no longer writes notifications nor sends push (plan 020 D13).
type AssessmentAssignedNotifProcessor struct {
	db         *sql.DB
	dispatcher NotificationDispatcher
	logger     logger.Logger
}

// NewAssessmentAssignedNotifProcessor creates a new processor.
func NewAssessmentAssignedNotifProcessor(db *sql.DB, dispatcher NotificationDispatcher, logger logger.Logger) *AssessmentAssignedNotifProcessor {
	return &AssessmentAssignedNotifProcessor{db: db, dispatcher: dispatcher, logger: logger}
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

// processEvent resuelve los alumnos inscritos en la oferta y delega UN dispatch
// con N destinatarios al gateway. Si el dispatch falla (5xx/timeout) se devuelve
// el error para que RabbitMQ reintente; la idempotencia del gateway evita duplicados.
func (p *AssessmentAssignedNotifProcessor) processEvent(ctx context.Context, event events.AssessmentAssignedEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.assigned notification",
		"assessment_id", pl.AssessmentID,
		"subject_offering_id", pl.SubjectOfferingID,
	)

	// Timeout para resolver destinatarios + 1 request al gateway.
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

	recipients := make([]client.DispatchRecipient, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		recipients = append(recipients, client.DispatchRecipient{UserID: studentID.String()})
	}

	// Tenant del recurso (F4.6.0): school_id viene del evento; unit_id se resuelve
	// desde la oferta (academic.subject_offerings.academic_unit_id). Si falla la
	// resolución del unit, se deja vacío (se omite en el push) sin abortar el
	// dispatch — el deep-link cae al fallback (lista) en el cliente.
	unitID := p.resolveOfferingUnitID(ctx, offeringID)

	req := client.DispatchRequest{
		IdempotencyKey: "assessment.assigned:" + pl.AssignmentID,
		Recipients:     recipients,
		Notification: client.DispatchNotification{
			Type:         "assessment_assigned",
			Title:        "Nueva evaluacion asignada",
			Body:         fmt.Sprintf("Te han asignado: %s", pl.Title),
			ResourceType: "assessment",
			ResourceID:   assessmentID.String(),
			SchoolID:     pl.SchoolID,
			UnitID:       unitID,
		},
		Channels: &client.DispatchChannels{InApp: true, Push: true},
		PushData: map[string]string{"event_type": p.EventType()},
		Source:   &client.DispatchSource{Caller: dispatchCaller, CorrelationID: event.EventID},
	}

	p.logger.Info("dispatch_requested",
		"event_type", p.EventType(),
		"assessment_id", assessmentID.String(),
		"subject_offering_id", offeringID.String(),
		"recipients", len(recipients),
	)

	if err := p.dispatcher.Dispatch(ctx, req); err != nil {
		return fmt.Errorf("dispatching assessment.assigned notifications: %w", err)
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
	                 AND m.status = 'active'`

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

// resolveOfferingUnitID devuelve el academic_unit_id de la oferta para el
// deep-link multi-tenant (F4.6.0). Es best-effort: ante cualquier fallo
// (oferta inexistente, error de BD) devuelve "" — el unit se omite del push y
// el cliente cae al fallback. Nunca aborta el dispatch.
func (p *AssessmentAssignedNotifProcessor) resolveOfferingUnitID(ctx context.Context, offeringID uuid.UUID) string {
	const query = `SELECT academic_unit_id
	               FROM academic.subject_offerings
	               WHERE id = $1`

	var unitID uuid.UUID
	if err := p.db.QueryRowContext(ctx, query, offeringID).Scan(&unitID); err != nil {
		p.logger.Warn("could not resolve offering academic_unit_id; omitting unit_id in push",
			"subject_offering_id", offeringID.String(),
			"error", err.Error(),
		)
		return ""
	}
	return unitID.String()
}
