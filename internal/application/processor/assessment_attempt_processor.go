package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"math"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/google/uuid"
)

// AssessmentAttemptProcessor handles "assessment.attempt_recorded" events.
// Materializa un componente de nota auto_scored en academic.grade_item a partir
// de un intento de evaluacion autocalificado, resolviendo cross-schema el periodo
// y la oferta de la sesion, y delega la notificacion al sub-procesador.
type AssessmentAttemptProcessor struct {
	db        *sql.DB
	notifProc *AssessmentAttemptNotifProcessor
	logger    logger.Logger
}

// NewAssessmentAttemptProcessor creates a new processor with DB access for the
// grade_item materialization and an optional notification sub-processor for the
// teacher notification.
func NewAssessmentAttemptProcessor(db *sql.DB, notifProc *AssessmentAttemptNotifProcessor, logger logger.Logger) *AssessmentAttemptProcessor {
	return &AssessmentAttemptProcessor{db: db, notifProc: notifProc, logger: logger}
}

func (p *AssessmentAttemptProcessor) EventType() string {
	return "assessment.attempt_recorded"
}

func (p *AssessmentAttemptProcessor) Process(ctx context.Context, payload []byte) error {
	var event events.AssessmentAttemptRecordedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}

	if err := p.validatePayload(event.Payload); err != nil {
		return err
	}

	return p.processEvent(ctx, event)
}

// validatePayload checks that required fields are present.
func (p *AssessmentAttemptProcessor) validatePayload(pl events.AssessmentAttemptRecordedPayload) error {
	if pl.AttemptID == "" {
		return errors.NewValidationError("attempt_id is required")
	}
	if pl.AssessmentID == "" {
		return errors.NewValidationError("assessment_id is required")
	}
	if pl.StudentMembershipID == "" {
		return errors.NewValidationError("student_membership_id is required")
	}
	if pl.SubjectID == "" {
		return errors.NewValidationError("subject_id is required")
	}
	if pl.SchoolID == "" {
		return errors.NewValidationError("school_id is required")
	}
	return nil
}

func (p *AssessmentAttemptProcessor) processEvent(ctx context.Context, event events.AssessmentAttemptRecordedEvent) error {
	pl := event.Payload

	p.logger.Info("procesando assessment.attempt_recorded",
		"attempt_id", pl.AttemptID,
		"assessment_id", pl.AssessmentID,
		"student_membership_id", pl.StudentMembershipID,
		"subject_id", pl.SubjectID,
		"status", pl.Status,
		"score", pl.Score,
		"max_score", pl.MaxScore,
	)

	// Paso 1 — Gate de status: solo un intento completado materializa nota.
	// pending_review no genera nota hasta que finalize lo resuelva (R7).
	if pl.Status != "completed" {
		p.logger.Info("intento no completado, no se materializa nota",
			"attempt_id", pl.AttemptID,
			"status", pl.Status,
		)
		return nil
	}

	// Paso 3 — Resolver period_id + offering_id cross-schema desde academic.
	// Si el alumno no esta inscrito en la materia, se descarta best-effort (R5).
	studentMembershipID, err := uuid.Parse(pl.StudentMembershipID)
	if err != nil {
		return fmt.Errorf("invalid student_membership_id: %w", err)
	}
	subjectID, err := uuid.Parse(pl.SubjectID)
	if err != nil {
		return fmt.Errorf("invalid subject_id: %w", err)
	}

	periodID, offeringID, found, err := p.resolveEnrollment(ctx, studentMembershipID, subjectID)
	if err != nil {
		return fmt.Errorf("resolviendo inscripcion del alumno: %w", err)
	}
	if !found {
		p.logger.Warn("intento sin inscripcion activa, no se materializa nota (best-effort)",
			"attempt_id", pl.AttemptID,
			"student_membership_id", pl.StudentMembershipID,
			"subject_id", pl.SubjectID,
		)
		return nil
	}

	// Paso 4 — Resolver el autor del examen como created_by_membership_id (R2).
	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}
	createdBy, err := p.resolveAssessmentAuthor(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("resolviendo autor de la evaluacion: %w", err)
	}

	// Paso 5 — Calcular value como porcentaje 0-100 (guard de division por cero).
	value := 0.0
	if pl.MaxScore > 0 {
		value = math.Round(pl.Score/pl.MaxScore*100*100) / 100
	}

	// Paso 6 — UPSERT idempotente en academic.grade_item con ID deterministico.
	attemptID, err := uuid.Parse(pl.AttemptID)
	if err != nil {
		return fmt.Errorf("invalid attempt_id: %w", err)
	}
	title := pl.Title
	if title == "" {
		title = "Evaluacion"
	}

	if err := p.upsertGradeItem(ctx, gradeItemUpsert{
		attemptID:           attemptID,
		studentMembershipID: studentMembershipID,
		subjectID:           subjectID,
		periodID:            periodID,
		offeringID:          offeringID,
		assessmentID:        assessmentID,
		value:               value,
		title:               title,
		createdBy:           createdBy,
	}); err != nil {
		return fmt.Errorf("materializando grade_item auto_scored: %w", err)
	}

	// Paso 7 — Delegar la notificacion al docente al sub-procesador (sin cambio de cadena).
	if p.notifProc != nil {
		if err := p.notifProc.processEvent(ctx, event); err != nil {
			p.logger.Error("fallo al crear notificacion del intento (no fatal)",
				"attempt_id", pl.AttemptID,
				"error", err.Error(),
			)
			// No fatal: el grade_item ya quedo materializado.
		}
	}

	p.logger.Info("assessment.attempt_recorded procesado: grade_item materializado",
		"attempt_id", pl.AttemptID,
		"value", value,
	)
	return nil
}

// resolveEnrollment resuelve el period_id y el offering_id de la sesion en la que
// el alumno esta inscrito para la materia dada, filtrando por periodo activo.
// La tabla subject_offering_enrollments no tiene soft-delete (una baja es DELETE
// con CASCADE), por lo que basta el filtro de periodo activo. Si hay mas de un
// periodo activo se toma el primero (deuda de desambiguacion documentada). found
// es false cuando no hay inscripcion (descartar best-effort, sin error).
func (p *AssessmentAttemptProcessor) resolveEnrollment(ctx context.Context, studentMembershipID, subjectID uuid.UUID) (periodID, offeringID uuid.UUID, found bool, err error) {
	const query = `SELECT soe.period_id, soe.offering_id
	               FROM academic.subject_offering_enrollments soe
	               JOIN academic.academic_periods p ON p.id = soe.period_id
	               WHERE soe.student_membership_id = $1
	                 AND soe.subject_id = $2
	                 AND p.is_active = true
	               LIMIT 1`

	err = p.db.QueryRowContext(ctx, query, studentMembershipID, subjectID).Scan(&periodID, &offeringID)
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, uuid.Nil, false, nil
		}
		return uuid.Nil, uuid.Nil, false, fmt.Errorf("consultando inscripcion: %w", err)
	}
	return periodID, offeringID, true, nil
}

// resolveAssessmentAuthor devuelve el created_by_membership_id del examen, que
// se usa como autor (created_by_membership_id) del grade_item generado.
func (p *AssessmentAttemptProcessor) resolveAssessmentAuthor(ctx context.Context, assessmentID uuid.UUID) (uuid.UUID, error) {
	const query = `SELECT created_by_membership_id
	               FROM assessment.assessment
	               WHERE id = $1`

	var createdBy uuid.UUID
	if err := p.db.QueryRowContext(ctx, query, assessmentID).Scan(&createdBy); err != nil {
		return uuid.Nil, fmt.Errorf("consultando autor de la evaluacion: %w", err)
	}
	return createdBy, nil
}

// gradeItemUpsert agrupa los datos resueltos para el UPSERT del grade_item.
type gradeItemUpsert struct {
	attemptID           uuid.UUID
	studentMembershipID uuid.UUID
	subjectID           uuid.UUID
	periodID            uuid.UUID
	offeringID          uuid.UUID
	assessmentID        uuid.UUID
	value               float64
	title               string
	createdBy           uuid.UUID
}

// upsertGradeItem materializa (o actualiza) el componente de nota auto_scored.
// El ID es deterministico sobre el attempt_id: reprocesar el mismo intento no
// duplica el grade_item (R6). El UNIQUE parcial uq_grade_item_attempt actua como
// red de seguridad adicional en BD. grade_letter queda NULL en auto_scored (R4).
func (p *AssessmentAttemptProcessor) upsertGradeItem(ctx context.Context, in gradeItemUpsert) error {
	gradeItemID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(in.attemptID.String()+"grade_item"))

	const query = `INSERT INTO academic.grade_item
		(id, membership_id, subject_id, period_id, source,
		 source_attempt_id, source_assessment_id,
		 value, title, created_by_membership_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'auto_scored', $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			value      = EXCLUDED.value,
			title      = EXCLUDED.title,
			updated_at = NOW()`

	_, err := p.db.ExecContext(ctx, query,
		gradeItemID,
		in.studentMembershipID,
		in.subjectID,
		in.periodID,
		in.attemptID,
		in.assessmentID,
		in.value,
		in.title,
		in.createdBy,
	)
	if err != nil {
		return fmt.Errorf("upsert grade_item: %w", err)
	}

	p.logger.Info("grade_item auto_scored materializado",
		"grade_item_id", gradeItemID.String(),
		"attempt_id", in.attemptID.String(),
		"value", in.value,
	)
	return nil
}
