package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completedAttemptPayload arma un payload de intento completado valido.
func completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, schoolID uuid.UUID, score, maxScore float64) events.AssessmentAttemptRecordedPayload {
	return events.AssessmentAttemptRecordedPayload{
		AttemptID:           attemptID.String(),
		AssessmentID:        assessmentID.String(),
		StudentMembershipID: studentMembershipID.String(),
		SubjectID:           subjectID.String(),
		SchoolID:            schoolID.String(),
		Score:               score,
		MaxScore:            maxScore,
		Status:              "completed",
		SubmittedAt:         time.Now(),
		TeacherID:           uuid.New().String(),
		Title:               "Examen de Matematicas",
	}
}

func TestAssessmentAttemptProcessor_EventType(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	assert.Equal(t, "assessment.attempt_recorded", proc.EventType())
}

func TestAssessmentAttemptProcessor_Completed_MaterializesGradeItem(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentMembershipID := uuid.New()
	subjectID := uuid.New()
	schoolID := uuid.New()
	periodID := uuid.New()
	offeringID := uuid.New()
	createdBy := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-100",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload:      completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, schoolID, 85.0, 100.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Resolucion de inscripcion cross-schema
	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}).AddRow(periodID, offeringID))

	// Resolucion del autor del examen
	dbMock.ExpectQuery("SELECT created_by_membership_id").
		WithArgs(assessmentID).
		WillReturnRows(sqlmock.NewRows([]string{"created_by_membership_id"}).AddRow(createdBy))

	// UPSERT del grade_item: value = 85.00
	dbMock.ExpectExec("INSERT INTO academic.grade_item").
		WithArgs(
			sqlmock.AnyArg(), // id deterministico (SHA1)
			studentMembershipID,
			subjectID,
			periodID,
			attemptID,    // source_attempt_id
			assessmentID, // source_assessment_id
			85.0,         // value (porcentaje)
			"Examen de Matematicas",
			createdBy,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_NotCompleted_SkipsMaterialization(t *testing.T) {
	// Un intento pending_review no genera nota (gate de status, R7).
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	pl := completedAttemptPayload(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 50.0, 100.0)
	pl.Status = "pending_review"

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-101",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   pl,
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Act — no debe ejecutarse ninguna query
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_NoEnrollment_SkipsBestEffort(t *testing.T) {
	// Sin inscripcion activa: descartar best-effort (R5), return nil sin error.
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	studentMembershipID := uuid.New()
	subjectID := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-102",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   completedAttemptPayload(uuid.New(), uuid.New(), studentMembershipID, subjectID, uuid.New(), 90.0, 100.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Inscripcion no encontrada -> sql.ErrNoRows
	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}))

	// Act — no debe haber UPSERT
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_DuplicateAttempt_Idempotent(t *testing.T) {
	// Reprocesar el mismo intento usa el mismo ID deterministico (ON CONFLICT DO UPDATE).
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentMembershipID := uuid.New()
	subjectID := uuid.New()
	periodID := uuid.New()
	offeringID := uuid.New()
	createdBy := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-103",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, uuid.New(), 70.0, 100.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	expectFlow := func() {
		dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
			WithArgs(studentMembershipID, subjectID).
			WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}).AddRow(periodID, offeringID))
		dbMock.ExpectQuery("SELECT created_by_membership_id").
			WithArgs(assessmentID).
			WillReturnRows(sqlmock.NewRows([]string{"created_by_membership_id"}).AddRow(createdBy))
		dbMock.ExpectExec("INSERT INTO academic.grade_item").
			WithArgs(
				sqlmock.AnyArg(), studentMembershipID, subjectID, periodID,
				attemptID, assessmentID, 70.0, "Examen de Matematicas", createdBy,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	// Primer y segundo procesamiento: el ID deterministico debe ser idempotente.
	id1 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID.String()+"grade_item"))
	id2 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID.String()+"grade_item"))
	assert.Equal(t, id1, id2, "el ID del grade_item debe ser deterministico por attempt_id")

	expectFlow()
	require.NoError(t, proc.Process(context.Background(), payload))

	expectFlow()
	require.NoError(t, proc.Process(context.Background(), payload))

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_ZeroMaxScore_ValueZero(t *testing.T) {
	// Guard de division por cero: max_score <= 0 -> value = 0.
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentMembershipID := uuid.New()
	subjectID := uuid.New()
	periodID := uuid.New()
	offeringID := uuid.New()
	createdBy := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-104",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, uuid.New(), 0.0, 0.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}).AddRow(periodID, offeringID))
	dbMock.ExpectQuery("SELECT created_by_membership_id").
		WithArgs(assessmentID).
		WillReturnRows(sqlmock.NewRows([]string{"created_by_membership_id"}).AddRow(createdBy))
	dbMock.ExpectExec("INSERT INTO academic.grade_item").
		WithArgs(
			sqlmock.AnyArg(), studentMembershipID, subjectID, periodID,
			attemptID, assessmentID, 0.0, "Examen de Matematicas", createdBy,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_ValueRounding(t *testing.T) {
	// value = round(score/max_score*100, 2): 7.5/9 -> 83.33.
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentMembershipID := uuid.New()
	subjectID := uuid.New()
	periodID := uuid.New()
	offeringID := uuid.New()
	createdBy := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-105",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, uuid.New(), 7.5, 9.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}).AddRow(periodID, offeringID))
	dbMock.ExpectQuery("SELECT created_by_membership_id").
		WithArgs(assessmentID).
		WillReturnRows(sqlmock.NewRows([]string{"created_by_membership_id"}).AddRow(createdBy))
	dbMock.ExpectExec("INSERT INTO academic.grade_item").
		WithArgs(
			sqlmock.AnyArg(), studentMembershipID, subjectID, periodID,
			attemptID, assessmentID, 83.33, "Examen de Matematicas", createdBy,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_MissingStudentMembershipID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	pl := completedAttemptPayload(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 50.0, 100.0)
	pl.StudentMembershipID = ""

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-106",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   pl,
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "student_membership_id is required")
}

func TestAssessmentAttemptProcessor_MissingSubjectID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	pl := completedAttemptPayload(uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), 50.0, 100.0)
	pl.SubjectID = ""

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-107",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   pl,
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "subject_id is required")
}

func TestAssessmentAttemptProcessor_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	err = proc.Process(context.Background(), []byte(`{broken json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentAttemptProcessor_EnrollmentQueryError(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	studentMembershipID := uuid.New()
	subjectID := uuid.New()

	event := events.AssessmentAttemptRecordedEvent{
		EventID:   "evt-108",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload:   completedAttemptPayload(uuid.New(), uuid.New(), studentMembershipID, subjectID, uuid.New(), 50.0, 100.0),
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnError(fmt.Errorf("connection refused"))

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolviendo inscripcion del alumno")
	assert.Contains(t, err.Error(), "connection refused")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_WithNotificationSubProcessor(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	notifProc := NewAssessmentAttemptNotifProcessor(disp, logger)
	proc := NewAssessmentAttemptProcessor(db, notifProc, logger)

	teacherID := uuid.New()
	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentMembershipID := uuid.New()
	subjectID := uuid.New()
	periodID := uuid.New()
	offeringID := uuid.New()
	createdBy := uuid.New()

	pl := completedAttemptPayload(attemptID, assessmentID, studentMembershipID, subjectID, uuid.New(), 75.0, 100.0)
	pl.TeacherID = teacherID.String()
	pl.Title = "Quiz con Notificacion"

	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-109",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload:      pl,
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT soe.period_id, soe.offering_id").
		WithArgs(studentMembershipID, subjectID).
		WillReturnRows(sqlmock.NewRows([]string{"period_id", "offering_id"}).AddRow(periodID, offeringID))
	dbMock.ExpectQuery("SELECT created_by_membership_id").
		WithArgs(assessmentID).
		WillReturnRows(sqlmock.NewRows([]string{"created_by_membership_id"}).AddRow(createdBy))
	dbMock.ExpectExec("INSERT INTO academic.grade_item").
		WithArgs(
			sqlmock.AnyArg(), studentMembershipID, subjectID, periodID,
			attemptID, assessmentID, 75.0, "Quiz con Notificacion", createdBy,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())

	// La notificacion al docente se delega al gateway (1 dispatch con 1 recipient).
	require.Len(t, disp.requests, 1)
	req := disp.requests[0]
	require.Len(t, req.Recipients, 1)
	assert.Equal(t, teacherID.String(), req.Recipients[0].UserID)
	assert.Equal(t, "assessment_attempt_recorded", req.Notification.Type)
	assert.Equal(t, "Un estudiante ha enviado: Quiz con Notificacion", req.Notification.Body)
}

func TestAssessmentAttemptProcessor_DeterministicID(t *testing.T) {
	attemptID := uuid.New().String()

	id1 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID+"grade_item"))
	id2 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID+"grade_item"))
	assert.Equal(t, id1, id2, "deterministic IDs should match for the same attempt_id")

	otherAttemptID := uuid.New().String()
	id3 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(otherAttemptID+"grade_item"))
	assert.NotEqual(t, id1, id3, "different attempt_ids should produce different IDs")
}
