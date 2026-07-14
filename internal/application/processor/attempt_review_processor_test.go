package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

// mockSettingsReader implementa SchoolSettingsReader para los tests.
type mockSettingsReader struct {
	settings m2m.SchoolSettings
	err      error
	calls    int
}

func (m *mockSettingsReader) GetSettings(_ context.Context, _ string) (m2m.SchoolSettings, error) {
	m.calls++
	if m.err != nil {
		return m2m.SchoolSettings{}, m.err
	}
	return m.settings, nil
}

// validEventPayload construye un evento válido serializado.
func validEventPayload(t *testing.T) []byte {
	t.Helper()
	evt := events.AttemptReviewRequestedEvent{
		EventID:      "evt-1",
		EventType:    EventTypeAttemptReviewRequested,
		EventVersion: "1.0",
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID:    "attempt-1",
			AssessmentID: "assessment-1",
			SchoolID:     "school-1",
			Answers: []events.AttemptReviewAnswerRef{
				{AnswerID: "a1", QuestionType: "open_ended"},
			},
		},
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal evento: %v", err)
	}
	return b
}

func settingsWithMode(mode string) m2m.SchoolSettings {
	return m2m.SchoolSettings{
		SchoolID: "school-1",
		Settings: []m2m.ResolvedSetting{
			{Key: settingKeyReviewMode, Value: mode, Source: "school"},
		},
	}
}

func TestAttemptReviewProcessor_EventType(t *testing.T) {
	p := NewAttemptReviewProcessor(&mockSettingsReader{}, newTestLogger())
	if p.EventType() != "attempt.review_requested" {
		t.Fatalf("event_type inesperado: %s", p.EventType())
	}
}

func TestAttemptReviewProcessor_ModeOff_AckSinAccion(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWithMode(reviewModeOff)}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("mode=off no debe retornar error (ACK): %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("esperaba 1 lectura de settings, hubo %d", reader.calls)
	}
}

func TestAttemptReviewProcessor_DefaultOff_CuandoNoHaySetting(t *testing.T) {
	// Sin la clave llm.review.mode → default de plataforma = off → ACK sin acción.
	reader := &mockSettingsReader{settings: m2m.SchoolSettings{SchoolID: "school-1"}}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("default off no debe retornar error: %v", err)
	}
}

func TestAttemptReviewProcessor_ModeAPI_ContinuaSinError(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWithMode("api")}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("mode=api debe loguear continuación sin error en F1: %v", err)
	}
}

func TestAttemptReviewProcessor_EventoMalformado_ErrorPermanente(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWithMode("api")}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	// JSON inválido.
	err := p.Process(context.Background(), []byte(`{no es json`))
	if err == nil {
		t.Fatal("esperaba error por evento malformado")
	}
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("esperaba ErrMalformedEvent, hubo: %v", err)
	}
	if classifyError(err) != ErrorTypePermanent {
		t.Fatalf("evento malformado debe clasificar como permanente")
	}
	if reader.calls != 0 {
		t.Fatalf("no debe leer settings si el evento es malformado, hubo %d", reader.calls)
	}
}

func TestAttemptReviewProcessor_CamposFaltantes_ErrorPermanente(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWithMode("api")}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	// Evento sintácticamente válido pero sin answers.
	evt := events.AttemptReviewRequestedEvent{
		EventType: EventTypeAttemptReviewRequested,
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID:    "attempt-1",
			AssessmentID: "assessment-1",
			SchoolID:     "school-1",
		},
	}
	b, _ := json.Marshal(evt)

	err := p.Process(context.Background(), b)
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("esperaba ErrMalformedEvent por answers vacío, hubo: %v", err)
	}
}

func TestAttemptReviewProcessor_SettingsInaccesible_ErrorTransitorio(t *testing.T) {
	reader := &mockSettingsReader{err: errors.New("academic caído")}
	p := NewAttemptReviewProcessor(reader, newTestLogger())

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error transitorio por settings inaccesible")
	}
	if errors.Is(err, ErrMalformedEvent) {
		t.Fatal("un fallo de settings no debe ser permanente")
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("fallo de settings debe clasificar como transitorio")
	}
}
