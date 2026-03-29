package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AssessmentAssignedNotifProcessor tests ---

func TestAssessmentAssignedNotifProcessor_EventType(t *testing.T) {
	logger := newTestLogger()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	assert.Equal(t, "assessment.assigned", proc.EventType())
}

func TestAssessmentAssignedNotifProcessor_StudentTarget_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	studentID := uuid.New()
	assessmentID := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-001",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: assessmentID.String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "student",
			TargetID:     studentID.String(),
			Title:        "Examen de Matematicas",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(), // notification id
			studentID,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen de Matematicas",
			"assessment",
			assessmentID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_UnitTarget_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	unitID := uuid.New()
	assessmentID := uuid.New()
	student1 := uuid.New()
	student2 := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-002",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: assessmentID.String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "unit",
			TargetID:     unitID.String(),
			Title:        "Examen de Historia",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect membership query to return 2 students
	rows := sqlmock.NewRows([]string{"user_id"}).
		AddRow(student1).
		AddRow(student2)
	dbMock.ExpectQuery("SELECT user_id FROM academic.memberships").
		WithArgs(unitID).
		WillReturnRows(rows)

	// Expect one INSERT per student
	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			student1,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen de Historia",
			"assessment",
			assessmentID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			student2,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen de Historia",
			"assessment",
			assessmentID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_UnitTarget_NoStudents(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	unitID := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-002b",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: uuid.New().String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "unit",
			TargetID:     unitID.String(),
			Title:        "Examen de Historia",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Return empty result set
	rows := sqlmock.NewRows([]string{"user_id"})
	dbMock.ExpectQuery("SELECT user_id FROM academic.memberships").
		WithArgs(unitID).
		WillReturnRows(rows)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — no INSERT should have been called
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_UnitTarget_PartialDBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	unitID := uuid.New()
	assessmentID := uuid.New()
	student1 := uuid.New()
	student2 := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-002c",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: assessmentID.String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "unit",
			TargetID:     unitID.String(),
			Title:        "Examen Parcial Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Return 2 students
	rows := sqlmock.NewRows([]string{"user_id"}).
		AddRow(student1).
		AddRow(student2)
	dbMock.ExpectQuery("SELECT user_id FROM academic.memberships").
		WithArgs(unitID).
		WillReturnRows(rows)

	// First student succeeds
	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			student1,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen Parcial Fallido",
			"assessment",
			assessmentID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Second student fails
	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			student2,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen Parcial Fallido",
			"assessment",
			assessmentID,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — partial failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to notify 1/2 students")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_UnitTarget_QueryError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	unitID := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-002d",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: uuid.New().String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "unit",
			TargetID:     unitID.String(),
			Title:        "Examen Query Fallida",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Membership query fails
	dbMock.ExpectQuery("SELECT user_id FROM academic.memberships").
		WithArgs(unitID).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolving unit students")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_UnsupportedTargetType(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-002e",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: uuid.New().String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "school",
			TargetID:     uuid.New().String(),
			Title:        "Examen Global",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — unsupported target_type is silently skipped
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	err = proc.Process(context.Background(), []byte(`{"invalid json}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentAssignedNotifProcessor_DBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	studentID := uuid.New()
	assessmentID := uuid.New()

	event := dto.AssessmentAssignedNotifEvent{
		EventID:      "evt-003",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAssignedNotifPayload{
			AssessmentID: assessmentID.String(),
			AssignmentID: uuid.New().String(),
			SchoolID:     uuid.New().String(),
			AssignedByID: uuid.New().String(),
			TargetType:   "student",
			TargetID:     studentID.String(),
			Title:        "Examen Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			studentID,
			"assessment_assigned",
			"Nueva evaluacion asignada",
			"Te han asignado: Examen Fallido",
			"assessment",
			assessmentID,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create assessment assigned notification")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// --- AssessmentAttemptNotifProcessor tests ---

func TestAssessmentAttemptNotifProcessor_EventType(t *testing.T) {
	logger := newTestLogger()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAttemptNotifProcessor(nc, logger)

	assert.Equal(t, "assessment.attempt_recorded", proc.EventType())
}

func TestAssessmentAttemptNotifProcessor_WithTeacherID_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAttemptNotifProcessor(nc, logger)

	teacherID := uuid.New()
	attemptID := uuid.New()

	event := dto.AssessmentAttemptNotifEvent{
		EventID:      "evt-010",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptNotifPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        85.0,
			TotalPoints:  100.0,
			TeacherID:    teacherID.String(),
			Title:        "Quiz de Ciencias",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			teacherID,
			"assessment_attempt_recorded",
			"Evaluacion enviada",
			"Un estudiante ha enviado: Quiz de Ciencias",
			"assessment_attempt",
			attemptID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptNotifProcessor_MissingTeacherID_Skips(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAttemptNotifProcessor(nc, logger)

	event := dto.AssessmentAttemptNotifEvent{
		EventID:      "evt-011",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptNotifPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        70.0,
			TotalPoints:  100.0,
			TeacherID:    "", // empty
			Title:        "Quiz sin Teacher",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — no INSERT should have been called
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptNotifProcessor_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAttemptNotifProcessor(nc, logger)

	err = proc.Process(context.Background(), []byte(`not json`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentAttemptNotifProcessor_DBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAttemptNotifProcessor(nc, logger)

	teacherID := uuid.New()
	attemptID := uuid.New()

	event := dto.AssessmentAttemptNotifEvent{
		EventID:      "evt-012",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptNotifPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        85.0,
			TotalPoints:  100.0,
			TeacherID:    teacherID.String(),
			Title:        "Quiz Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			teacherID,
			"assessment_attempt_recorded",
			"Evaluacion enviada",
			"Un estudiante ha enviado: Quiz Fallido",
			"assessment_attempt",
			attemptID,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create assessment attempt notification")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// --- AssessmentReviewedNotifProcessor tests ---

func TestAssessmentReviewedNotifProcessor_EventType(t *testing.T) {
	logger := newTestLogger()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentReviewedNotifProcessor(nc, logger)

	assert.Equal(t, "assessment.reviewed", proc.EventType())
}

func TestAssessmentReviewedNotifProcessor_WithStudentID_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentReviewedNotifProcessor(nc, logger)

	studentID := uuid.New()
	attemptID := uuid.New()

	event := dto.AssessmentReviewedNotifEvent{
		EventID:      "evt-020",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentReviewedNotifPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: uuid.New().String(),
			ReviewerID:   uuid.New().String(),
			SchoolID:     uuid.New().String(),
			FinalScore:   92.5,
			TotalPoints:  100.0,
			Status:       "reviewed",
			StudentID:    studentID.String(),
			Title:        "Examen Final de Lengua",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			studentID,
			"assessment_reviewed",
			"Evaluacion calificada",
			"Tu evaluacion ha sido calificada: Examen Final de Lengua",
			"assessment_attempt",
			attemptID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentReviewedNotifProcessor_MissingStudentID_Skips(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentReviewedNotifProcessor(nc, logger)

	event := dto.AssessmentReviewedNotifEvent{
		EventID:      "evt-021",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentReviewedNotifPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			ReviewerID:   uuid.New().String(),
			SchoolID:     uuid.New().String(),
			FinalScore:   60.0,
			TotalPoints:  100.0,
			Status:       "reviewed",
			StudentID:    "", // empty
			Title:        "Examen sin Student",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — no INSERT should have been called
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentReviewedNotifProcessor_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentReviewedNotifProcessor(nc, logger)

	err = proc.Process(context.Background(), []byte(`{broken`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentReviewedNotifProcessor_DBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentReviewedNotifProcessor(nc, logger)

	studentID := uuid.New()
	attemptID := uuid.New()

	event := dto.AssessmentReviewedNotifEvent{
		EventID:      "evt-022",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentReviewedNotifPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: uuid.New().String(),
			ReviewerID:   uuid.New().String(),
			SchoolID:     uuid.New().String(),
			FinalScore:   92.5,
			TotalPoints:  100.0,
			Status:       "reviewed",
			StudentID:    studentID.String(),
			Title:        "Examen Fallido de Lengua",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			studentID,
			"assessment_reviewed",
			"Evaluacion calificada",
			"Tu evaluacion ha sido calificada: Examen Fallido de Lengua",
			"assessment_attempt",
			attemptID,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create assessment reviewed notification")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}
