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
	schoolID := uuid.New()
	unitID := uuid.New()
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
			SchoolID:               schoolID.String(),
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
	// F4.6.0: resolución del academic_unit_id de la oferta para el push.
	dbMock.ExpectQuery("SELECT academic_unit_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"academic_unit_id"}).AddRow(unitID))

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
	// F4.6.0: tenant del recurso en el dispatch (school del evento, unit de la oferta).
	assert.Equal(t, schoolID.String(), req.Notification.SchoolID)
	assert.Equal(t, unitID.String(), req.Notification.UnitID)
	assert.Equal(t, "assessment.assigned:"+assignmentID.String(), req.IdempotencyKey)
	require.NotNil(t, req.Channels)
	assert.True(t, req.Channels.InApp)
	assert.True(t, req.Channels.Push)
}

func TestAssessmentAssignedNotifProcessor_UnitResolutionFails_OmitsUnitID(t *testing.T) {
	// F4.6.0: si no se puede resolver el academic_unit_id de la oferta, el unit_id
	// se omite (queda vacío) pero el dispatch SÍ ocurre (best-effort, no aborta).
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	disp := &fakeDispatcher{}
	proc := NewAssessmentAssignedNotifProcessor(db, disp, logger)

	offeringID := uuid.New()
	schoolID := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-002f",
		EventType:    "assessment.assigned",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentAssignedPayload{
			AssessmentID:           uuid.New().String(),
			AssignmentID:           uuid.New().String(),
			SchoolID:               schoolID.String(),
			AssignedByMembershipID: uuid.New().String(),
			SubjectOfferingID:      offeringID.String(),
			Title:                  "Examen sin unidad",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(uuid.New()))
	dbMock.ExpectQuery("SELECT academic_unit_id").
		WithArgs(offeringID).
		WillReturnError(fmt.Errorf("offering not found"))

	err = proc.Process(context.Background(), payload)

	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
	require.Len(t, disp.requests, 1)
	req := disp.requests[0]
	assert.Equal(t, schoolID.String(), req.Notification.SchoolID)
	assert.Empty(t, req.Notification.UnitID, "unit_id se omite si no se puede resolver")
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
	// F4.6.0: el unit se resuelve antes del dispatch.
	dbMock.ExpectQuery("SELECT academic_unit_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"academic_unit_id"}).AddRow(uuid.New()))

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
