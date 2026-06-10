package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDispatcher captura los DispatchRequest enviados y opcionalmente falla,
// para verificar la delegación al gateway sin red (plan 020 F2.3).
type fakeDispatcher struct {
	requests []client.DispatchRequest
	err      error
}

func (f *fakeDispatcher) Dispatch(_ context.Context, req client.DispatchRequest) error {
	f.requests = append(f.requests, req)
	return f.err
}

// --- AssessmentAssignedNotifProcessor tests ---

func TestAssessmentAssignedNotifProcessor_EventType(t *testing.T) {
	logger := newTestLogger()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	proc := NewAssessmentAssignedNotifProcessor(db, &fakeDispatcher{}, logger)

	assert.Equal(t, "assessment.assigned", proc.EventType())
}

func TestAssessmentAssignedNotifProcessor_EnrolledStudents_Success(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

	offeringID := uuid.New()
	assessmentID := uuid.New()
	assignmentID := uuid.New()
	student1 := uuid.New()
	student2 := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-001",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           assessmentID.String(),
			AssignmentID:           assignmentID.String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen de Historia",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"user_id"}).AddRow(student1).AddRow(student2)
	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(rows)

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())

	// 1 dispatch con N=2 recipients (fan-out resuelto por el worker).
	require.Len(t, disp.requests, 1)
	req := disp.requests[0]
	require.Len(t, req.Recipients, 2)
	gotUsers := map[string]bool{req.Recipients[0].UserID: true, req.Recipients[1].UserID: true}
	assert.True(t, gotUsers[student1.String()])
	assert.True(t, gotUsers[student2.String()])
	assert.Equal(t, "assessment_assigned", req.Notification.Type)
	assert.Equal(t, "Nueva evaluacion asignada", req.Notification.Title)
	assert.Equal(t, "Te han asignado: Examen de Historia", req.Notification.Body)
	assert.Equal(t, "assessment", req.Notification.ResourceType)
	assert.Equal(t, assessmentID.String(), req.Notification.ResourceID)
	assert.Equal(t, "assessment.assigned:"+assignmentID.String(), req.IdempotencyKey)
	require.NotNil(t, req.Channels)
	assert.True(t, req.Channels.InApp)
	assert.True(t, req.Channels.Push)
}

func TestAssessmentAssignedNotifProcessor_NoEnrolledStudents(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

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

	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}))

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
	assert.Empty(t, disp.requests, "sin alumnos no debe haber dispatch")
}

func TestAssessmentAssignedNotifProcessor_DispatchError(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{err: fmt.Errorf("gateway 503")}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

	offeringID := uuid.New()
	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002c",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           uuid.New().String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               uuid.New().String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen Parcial",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(uuid.New()))

	// Dispatch falla → Process devuelve error (Rabbit reintenta).
	err = proc.Process(context.Background(), payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispatching assessment.assigned notifications")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_QueryError(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

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

	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnError(fmt.Errorf("connection refused"))

	err = proc.Process(context.Background(), payload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolving offering students")
	assert.Empty(t, disp.requests)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestAssessmentAssignedNotifProcessor_InvalidOfferingID(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

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

	err = proc.Process(context.Background(), payload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subject_offering_id")
	assert.Empty(t, disp.requests)
}

func TestAssessmentAssignedNotifProcessor_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	proc := NewAssessmentAssignedNotifProcessor(db, &fakeDispatcher{}, logger)

	err = proc.Process(context.Background(), []byte(`{"invalid json}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

// --- AssessmentAttemptNotifProcessor tests ---

func TestAssessmentAttemptNotifProcessor_EventType(t *testing.T) {
	proc := NewAssessmentAttemptNotifProcessor(&fakeDispatcher{}, newTestLogger())
	assert.Equal(t, "assessment.attempt_recorded", proc.EventType())
}

func TestAssessmentAttemptNotifProcessor_WithTeacherID_Success(t *testing.T) {
	disp := &fakeDispatcher{}
	proc := NewAssessmentAttemptNotifProcessor(disp, newTestLogger())

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

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	require.Len(t, disp.requests, 1)
	req := disp.requests[0]
	require.Len(t, req.Recipients, 1)
	assert.Equal(t, teacherID.String(), req.Recipients[0].UserID)
	assert.Equal(t, "assessment_attempt_recorded", req.Notification.Type)
	assert.Equal(t, "Evaluacion enviada", req.Notification.Title)
	assert.Equal(t, "Un estudiante ha enviado: Quiz de Ciencias", req.Notification.Body)
	assert.Equal(t, "assessment_attempt", req.Notification.ResourceType)
	assert.Equal(t, attemptID.String(), req.Notification.ResourceID)
}

func TestAssessmentAttemptNotifProcessor_MissingTeacherID_Skips(t *testing.T) {
	disp := &fakeDispatcher{}
	proc := NewAssessmentAttemptNotifProcessor(disp, newTestLogger())

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
			TeacherID:           "",
			Title:               "Quiz sin Teacher",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	assert.Empty(t, disp.requests, "sin teacher_id no debe haber dispatch")
}

func TestAssessmentAttemptNotifProcessor_InvalidJSON(t *testing.T) {
	proc := NewAssessmentAttemptNotifProcessor(&fakeDispatcher{}, newTestLogger())

	err := proc.Process(context.Background(), []byte(`not json`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentAttemptNotifProcessor_DispatchError(t *testing.T) {
	disp := &fakeDispatcher{err: fmt.Errorf("gateway timeout")}
	proc := NewAssessmentAttemptNotifProcessor(disp, newTestLogger())

	teacherID := uuid.New()
	event := events.AssessmentAttemptRecordedEvent{
		EventID:      "evt-012",
		EventType:    "assessment.attempt_recorded",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAttemptRecordedPayload{
			AttemptID:           uuid.New().String(),
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

	err = proc.Process(context.Background(), payload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispatching assessment.attempt_recorded notification")
}

// --- AssessmentReviewedNotifProcessor tests ---

func TestAssessmentReviewedNotifProcessor_EventType(t *testing.T) {
	proc := NewAssessmentReviewedNotifProcessor(&fakeDispatcher{}, newTestLogger())
	assert.Equal(t, "assessment.reviewed", proc.EventType())
}

func TestAssessmentReviewedNotifProcessor_WithStudentID_Success(t *testing.T) {
	disp := &fakeDispatcher{}
	proc := NewAssessmentReviewedNotifProcessor(disp, newTestLogger())

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

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	require.Len(t, disp.requests, 1)
	req := disp.requests[0]
	require.Len(t, req.Recipients, 1)
	assert.Equal(t, studentID.String(), req.Recipients[0].UserID)
	assert.Equal(t, "assessment_reviewed", req.Notification.Type)
	assert.Equal(t, "Evaluacion calificada", req.Notification.Title)
	assert.Equal(t, "Tu evaluacion ha sido calificada: Examen Final de Lengua", req.Notification.Body)
	assert.Equal(t, "assessment_attempt", req.Notification.ResourceType)
	assert.Equal(t, attemptID.String(), req.Notification.ResourceID)
}

func TestAssessmentReviewedNotifProcessor_MissingStudentID_Skips(t *testing.T) {
	disp := &fakeDispatcher{}
	proc := NewAssessmentReviewedNotifProcessor(disp, newTestLogger())

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
			StudentID:    "",
			Title:        "Examen sin Student",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	assert.Empty(t, disp.requests, "sin student_id no debe haber dispatch")
}

func TestAssessmentReviewedNotifProcessor_InvalidJSON(t *testing.T) {
	proc := NewAssessmentReviewedNotifProcessor(&fakeDispatcher{}, newTestLogger())

	err := proc.Process(context.Background(), []byte(`{broken`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestAssessmentReviewedNotifProcessor_DispatchError(t *testing.T) {
	disp := &fakeDispatcher{err: fmt.Errorf("gateway 500")}
	proc := NewAssessmentReviewedNotifProcessor(disp, newTestLogger())

	studentID := uuid.New()
	event := events.AssessmentReviewedEvent{
		EventID:      "evt-022",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentReviewedPayload{
			AttemptID:    uuid.New().String(),
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

	err = proc.Process(context.Background(), payload)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispatching assessment.reviewed notification")
}
