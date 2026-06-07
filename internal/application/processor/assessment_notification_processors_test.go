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

func TestAssessmentAssignedNotifProcessor_EnrolledStudents_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	offeringID := uuid.New()
	assessmentID := uuid.New()
	student1 := uuid.New()
	student2 := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-001",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           assessmentID.String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen de Historia",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect enrollment resolution query to return 2 students
	rows := sqlmock.NewRows([]string{"user_id"}).
		AddRow(student1).
		AddRow(student2)
	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
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

func TestAssessmentAssignedNotifProcessor_NoEnrolledStudents(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	offeringID := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002b",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           uuid.New().String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen de Historia",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Return empty result set
	rows := sqlmock.NewRows([]string{"user_id"})
	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(rows)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — no INSERT should have been called
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_PartialDBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	offeringID := uuid.New()
	assessmentID := uuid.New()
	student1 := uuid.New()
	student2 := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002c",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           assessmentID.String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen Parcial Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Return 2 students
	rows := sqlmock.NewRows([]string{"user_id"}).
		AddRow(student1).
		AddRow(student2)
	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
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

func TestAssessmentAssignedNotifProcessor_QueryError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	offeringID := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002d",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           uuid.New().String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen Query Fallida",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Enrollment resolution query fails
	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolving offering students")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_InvalidOfferingID(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	proc := NewAssessmentAssignedNotifProcessor(db, nc, logger)

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002e",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           uuid.New().String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      "not-a-uuid",
			Title:                  "Examen Global",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert — invalid offering id is rejected before any query
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subject_offering_id")
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

	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-010",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAttemptRecordedPayload{
			AttemptID:           attemptID.String(),
			AssessmentID:        uuid.New().String(),
			StudentMembershipID: uuid.New().String(),
			SubjectID:           uuid.New().String(),
			SchoolID:            uuid.New().String(),
			Score:               85.0,
			MaxScore:            100.0,
			Status:              "completed",
			TeacherID:           teacherID.String(),
			Title:               "Quiz de Ciencias",
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

	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-011",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAttemptRecordedPayload{
			AttemptID:           uuid.New().String(),
			AssessmentID:        uuid.New().String(),
			StudentMembershipID: uuid.New().String(),
			SubjectID:           uuid.New().String(),
			SchoolID:            uuid.New().String(),
			Score:               70.0,
			MaxScore:            100.0,
			Status:              "completed",
			TeacherID:           "", // empty
			Title:               "Quiz sin Teacher",
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

	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-012",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAttemptRecordedPayload{
			AttemptID:           attemptID.String(),
			AssessmentID:        uuid.New().String(),
			StudentMembershipID: uuid.New().String(),
			SubjectID:           uuid.New().String(),
			SchoolID:            uuid.New().String(),
			Score:               85.0,
			MaxScore:            100.0,
			Status:              "completed",
			TeacherID:           teacherID.String(),
			Title:               "Quiz Fallido",
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

	event := events.AssessmentReviewedEvent{
		EventID:      "evt-020",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentReviewedPayload{
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

	event := events.AssessmentReviewedEvent{
		EventID:      "evt-021",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentReviewedPayload{
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

	event := events.AssessmentReviewedEvent{
		EventID:      "evt-022",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentReviewedPayload{
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
