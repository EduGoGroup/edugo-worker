package processor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EduGoGroup/edugo-shared/auth"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cierre F2.4: roundtrip cruzado worker→gateway a través del NotificationDispatchClient
// REAL (con service JWT firmado) contra un httptest que simula el endpoint de
// platform. Verifica el contrato DispatchRequest y el header Authorization, sin
// reimplementar la lógica de F2.1–F2.3.

const (
	itSvcSecret   = "worker-f24-service-jwt-secret-min-32-chars!"
	itSvcIssuer   = "edugo-identity-test"
	itSvcAudience = "edugo-api-platform"
)

// newDispatchHarness levanta un gateway falso que captura el último DispatchRequest
// y el header Authorization, y devuelve un client real apuntando a él.
func newDispatchHarness(t *testing.T) (*client.NotificationDispatchClient, *capturedDispatch, func()) {
	t.Helper()
	captured := &capturedDispatch{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path = r.URL.Path
		captured.authHeader = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&captured.req)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))

	provider, err := client.NewServiceTokenProvider(client.ServiceTokenConfig{
		Secret:   itSvcSecret,
		Issuer:   itSvcIssuer,
		Audience: itSvcAudience,
		ClientID: "edugo-worker",
		Scopes:   []string{"notifications.dispatch"},
		TTL:      15 * time.Minute,
	})
	require.NoError(t, err)

	dispatchClient := client.NewNotificationDispatchClient(client.NotificationDispatchClientConfig{
		BaseURL:       srv.URL,
		Timeout:       5 * time.Second,
		TokenProvider: provider,
	})
	return dispatchClient, captured, srv.Close
}

type capturedDispatch struct {
	path       string
	authHeader string
	req        client.DispatchRequest
}

// assertValidServiceJWT comprueba que el header lleva un service JWT que el
// gateway aceptaría (firma + iss + aud + scope).
func assertValidServiceJWT(t *testing.T, authHeader string) {
	t.Helper()
	const prefix = "Bearer "
	require.True(t, len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix, "falta Bearer: %q", authHeader)
	mgr := auth.NewServiceJWTManager(itSvcSecret, itSvcIssuer, itSvcAudience)
	claims, err := mgr.ValidateServiceToken(authHeader[len(prefix):])
	require.NoError(t, err)
	assert.Equal(t, "edugo-worker", claims.ClientID)
	assert.True(t, claims.HasScope("notifications.dispatch"))
}

func TestCrossContract_AssessmentReviewed(t *testing.T) {
	dispatchClient, captured, closeFn := newDispatchHarness(t)
	defer closeFn()

	proc := NewAssessmentReviewedNotifProcessor(dispatchClient, newTestLogger())

	studentID := uuid.New()
	attemptID := uuid.New()
	event := events.AssessmentReviewedEvent{
		EventID:      "evt-it-020",
		EventType:    "assessment.reviewed",
		EventVersion: "1.0.0",
		Timestamp:    time.Now(),
		Payload: events.AssessmentReviewedPayload{
			AttemptID:    attemptID.String(),
			AssessmentID: uuid.New().String(),
			ReviewerID:   uuid.New().String(),
			SchoolID:     uuid.New().String(),
			Status:       "reviewed",
			StudentID:    studentID.String(),
			Title:        "Examen Final",
		},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	require.NoError(t, proc.Process(context.Background(), payload))

	// Header de auth: service JWT válido.
	assertValidServiceJWT(t, captured.authHeader)
	assert.Equal(t, "/api/v1/internal/notifications/dispatch", captured.path)

	// Contrato DispatchRequest.
	req := captured.req
	require.Len(t, req.Recipients, 1)
	assert.Equal(t, studentID.String(), req.Recipients[0].UserID)
	assert.Equal(t, "assessment_reviewed", req.Notification.Type)
	assert.Equal(t, attemptID.String(), req.Notification.ResourceID)
	assert.Equal(t, "assessment_attempt", req.Notification.ResourceType)
	assert.Equal(t, "assessment.reviewed:"+attemptID.String(), req.IdempotencyKey)
	require.NotNil(t, req.Channels)
	assert.True(t, req.Channels.InApp)
	assert.True(t, req.Channels.Push)
	assert.Equal(t, "assessment.reviewed", req.PushData["event_type"])
	require.NotNil(t, req.Source)
	assert.Equal(t, "edugo-worker", req.Source.Caller)
	assert.Equal(t, "evt-it-020", req.Source.CorrelationID)
}

func TestCrossContract_AssessmentAssigned_FanOut(t *testing.T) {
	dispatchClient, captured, closeFn := newDispatchHarness(t)
	defer closeFn()

	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	proc := NewAssessmentAssignedNotifProcessor(db, dispatchClient, newTestLogger())

	offeringID := uuid.New()
	assessmentID := uuid.New()
	assignmentID := uuid.New()
	s1 := uuid.New()
	s2 := uuid.New()

	event := events.AssessmentAssignedEvent{
		EventID:      "evt-it-001",
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

	dbMock.ExpectQuery("SELECT DISTINCT m.user_id").
		WithArgs(offeringID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(s1).AddRow(s2))

	require.NoError(t, proc.Process(context.Background(), payload))
	require.NoError(t, dbMock.ExpectationsWereMet())

	assertValidServiceJWT(t, captured.authHeader)

	// 1 dispatch con N=2 recipients (fan-out resuelto en el worker).
	req := captured.req
	require.Len(t, req.Recipients, 2)
	got := map[string]bool{req.Recipients[0].UserID: true, req.Recipients[1].UserID: true}
	assert.True(t, got[s1.String()] && got[s2.String()])
	assert.Equal(t, "assessment_assigned", req.Notification.Type)
	assert.Equal(t, assessmentID.String(), req.Notification.ResourceID)
	assert.Equal(t, "assessment.assigned:"+assignmentID.String(), req.IdempotencyKey)
}
