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

func TestAssessmentAttemptProcessor_EventType(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	assert.Equal(t, "assessment.attempt_recorded", proc.EventType())
}

func TestAssessmentAttemptProcessor_ValidEvent_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentID := uuid.New()
	schoolID := uuid.New()

	event := dto.AssessmentAttemptEvent{
		EventID:      "evt-100",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:       attemptID.String(),
			AssessmentID:    assessmentID.String(),
			StudentID:       studentID.String(),
			SchoolID:        schoolID.String(),
			Score:           85.0,
			TotalPoints:     100.0,
			DurationSeconds: 1200,
			TeacherID:       uuid.New().String(),
			Title:           "Examen de Matematicas",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect analytics INSERT
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), // id (SHA1)
			attemptID,
			assessmentID,
			studentID,
			schoolID,
			85.0,             // score
			100.0,            // total_points
			85.0,             // percentage (calculated)
			1200,             // duration_seconds
			sqlmock.AnyArg(), // submitted_at
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect stats UPSERT
	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(
			sqlmock.AnyArg(), // id (SHA1)
			assessmentID,
			85.0, // score
			85.0, // percentage
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_DuplicateEvent_Idempotent(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentID := uuid.New()
	schoolID := uuid.New()

	event := dto.AssessmentAttemptEvent{
		EventID:      "evt-101",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: assessmentID.String(),
			StudentID:    studentID.String(),
			SchoolID:     schoolID.String(),
			Score:        70.0,
			TotalPoints:  100.0,
			TeacherID:    uuid.New().String(),
			Title:        "Quiz de Ciencias",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// First call: analytics INSERT succeeds (1 row affected)
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), attemptID, assessmentID, studentID, schoolID,
			70.0, 100.0, 70.0, nil, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(sqlmock.AnyArg(), assessmentID, 70.0, 70.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)

	// Second call: analytics INSERT does nothing (ON CONFLICT DO NOTHING = 0 rows)
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), attemptID, assessmentID, studentID, schoolID,
			70.0, 100.0, 70.0, nil, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows = idempotent

	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(sqlmock.AnyArg(), assessmentID, 70.0, 70.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)

	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_MissingAttemptID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-102",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    "", // missing
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        50.0,
			TotalPoints:  100.0,
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attempt_id is required")
}

func TestAssessmentAttemptProcessor_MissingAssessmentID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-103",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: "", // missing
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        50.0,
			TotalPoints:  100.0,
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "assessment_id is required")
}

func TestAssessmentAttemptProcessor_MissingStudentID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-104",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    "", // missing
			SchoolID:     uuid.New().String(),
			Score:        50.0,
			TotalPoints:  100.0,
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "student_id is required")
}

func TestAssessmentAttemptProcessor_MissingSchoolID_ValidationError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-105",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     "", // missing
			Score:        50.0,
			TotalPoints:  100.0,
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "school_id is required")
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

func TestAssessmentAttemptProcessor_AnalyticsDBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-106",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        50.0,
			TotalPoints:  100.0,
			TeacherID:    uuid.New().String(),
			Title:        "Examen Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Analytics INSERT fails
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			50.0, 100.0, 50.0, nil, sqlmock.AnyArg(),
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inserting analytics")
	assert.Contains(t, err.Error(), "connection refused")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_StatsError_NonFatal(t *testing.T) {
	// Stats failure should NOT cause the processor to fail
	// because analytics were already recorded.

	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-107",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        90.0,
			TotalPoints:  100.0,
			TeacherID:    uuid.New().String(),
			Title:        "Examen Stats Fallido",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Analytics INSERT succeeds
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			90.0, 100.0, 90.0, nil, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Stats UPSERT fails
	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), 90.0, 90.0).
		WillReturnError(fmt.Errorf("stats table unavailable"))

	// Act
	err = proc.Process(context.Background(), payload)

	// Assert - no error because stats failure is non-fatal
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_WithNotificationSubProcessor(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)
	notifProc := NewAssessmentAttemptNotifProcessor(nc, logger)
	proc := NewAssessmentAttemptProcessor(db, notifProc, logger)

	teacherID := uuid.New()
	attemptID := uuid.New()
	assessmentID := uuid.New()
	studentID := uuid.New()
	schoolID := uuid.New()

	event := dto.AssessmentAttemptEvent{
		EventID:      "evt-108",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: assessmentID.String(),
			StudentID:    studentID.String(),
			SchoolID:     schoolID.String(),
			Score:        75.0,
			TotalPoints:  100.0,
			TeacherID:    teacherID.String(),
			Title:        "Quiz con Notificacion",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect analytics INSERT
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), attemptID, assessmentID, studentID, schoolID,
			75.0, 100.0, 75.0, nil, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect stats UPSERT
	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(sqlmock.AnyArg(), assessmentID, 75.0, 75.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect notification INSERT (from sub-processor delegation)
	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			teacherID,
			"assessment_attempt_recorded",
			"Evaluacion enviada",
			"Un estudiante ha enviado: Quiz con Notificacion",
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

func TestAssessmentAttemptProcessor_PercentageCalculation(t *testing.T) {
	// When percentage is not provided, it should be calculated from score/total_points

	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAttemptProcessor(db, nil, logger)

	event := dto.AssessmentAttemptEvent{
		EventID:   "evt-109",
		EventType: "assessment.attempt_recorded",
		Timestamp: time.Now(),
		Payload: dto.AssessmentAttemptPayload{
			AttemptID:    uuid.New().String(),
			AssessmentID: uuid.New().String(),
			StudentID:    uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Score:        7.5,
			TotalPoints:  10.0,
			Percentage:   0, // not provided
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Expect analytics INSERT with calculated percentage = 75.0
	dbMock.ExpectExec("INSERT INTO assessment.attempt_analytics").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			7.5, 10.0, 75.0, // percentage calculated
			nil, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	dbMock.ExpectExec("INSERT INTO assessment.assessment_stats").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), 7.5, 75.0).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = proc.Process(context.Background(), payload)
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAttemptProcessor_DeterministicID(t *testing.T) {
	// Verify that the same attempt_id always produces the same analytics UUID
	attemptID := uuid.New().String()

	id1 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID+"analytics"))
	id2 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(attemptID+"analytics"))

	assert.Equal(t, id1, id2, "deterministic IDs should match for the same attempt_id")

	// Different attempt_id should produce different IDs
	otherAttemptID := uuid.New().String()
	id3 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(otherAttemptID+"analytics"))
	assert.NotEqual(t, id1, id3, "different attempt_ids should produce different IDs")
}
