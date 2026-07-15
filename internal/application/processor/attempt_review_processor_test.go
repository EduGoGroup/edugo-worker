package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
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

// mockLearningClient implementa LearningReviewClient y registra las llamadas.
type mockLearningClient struct {
	pending    m2m.PendingAnswersResponse
	pendingErr error

	reviewErr    error
	reviewCalls  []m2m.AnswerReviewRequest
	reviewAnswer []string // answer_ids en orden

	finalizeErr   error
	finalizeCalls int
}

func (m *mockLearningClient) GetPendingAnswers(_ context.Context, _ string) (m2m.PendingAnswersResponse, error) {
	if m.pendingErr != nil {
		return m2m.PendingAnswersResponse{}, m.pendingErr
	}
	return m.pending, nil
}

func (m *mockLearningClient) PostAnswerReview(_ context.Context, _ string, answerID string, review m2m.AnswerReviewRequest) (m2m.AnswerReviewResponse, error) {
	m.reviewAnswer = append(m.reviewAnswer, answerID)
	m.reviewCalls = append(m.reviewCalls, review)
	if m.reviewErr != nil {
		return m2m.AnswerReviewResponse{}, m.reviewErr
	}
	return m2m.AnswerReviewResponse{AnswerID: answerID, ReviewStatus: "reviewed"}, nil
}

func (m *mockLearningClient) FinalizeAttempt(_ context.Context, attemptID string) (m2m.FinalizeResponse, error) {
	m.finalizeCalls++
	if m.finalizeErr != nil {
		return m2m.FinalizeResponse{}, m.finalizeErr
	}
	return m2m.FinalizeResponse{AttemptID: attemptID, Status: "completed"}, nil
}

// mockLLMProvider implementa llm.LLMProvider. Puede fallar en la N-ésima llamada.
type mockLLMProvider struct {
	score    float64
	feedback string
	verdict  llm.Verdict

	calls      int
	failOnCall int // 1-indexed; 0 = nunca falla
	err        error
}

func (m *mockLLMProvider) GenerateAssessment(_ context.Context, _ llm.MaterialInput, _ llm.GenerationParams) (json.RawMessage, error) {
	return nil, errors.New("no usado en estos tests")
}

func (m *mockLLMProvider) ReviewAnswer(_ context.Context, _ llm.ReviewRequest) (llm.ReviewResult, error) {
	m.calls++
	if m.failOnCall != 0 && m.calls == m.failOnCall {
		return llm.ReviewResult{}, m.err
	}
	return llm.ReviewResult{Verdict: m.verdict, Score: m.score, Feedback: m.feedback}, nil
}

func (m *mockLLMProvider) Name() string { return "mock" }

// --- helpers ---

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

func settingsWith(pairs ...string) m2m.SchoolSettings {
	s := m2m.SchoolSettings{SchoolID: "school-1"}
	for i := 0; i+1 < len(pairs); i += 2 {
		s.Settings = append(s.Settings, m2m.ResolvedSetting{Key: pairs[i], Value: pairs[i+1], Source: "school"})
	}
	return s
}

func pendingAnswer(id string, points float64) m2m.PendingAnswer {
	return m2m.PendingAnswer{
		AnswerID:       id,
		QuestionType:   "open_ended",
		QuestionText:   "¿Explica la fotosíntesis?",
		ExpectedAnswer: "conversión de luz en energía química",
		StudentAnswer:  "las plantas usan la luz del sol",
		Points:         points,
	}
}

func newProcessor(settings SchoolSettingsReader, learning LearningReviewClient, provider llm.LLMProvider) *AttemptReviewProcessor {
	providers := map[string]llm.LLMProvider{}
	if provider != nil {
		providers["local"] = provider
		providers["api"] = provider
	}
	return NewAttemptReviewProcessor(settings, learning, providers, newTestLogger())
}

// --- tests de política/validación (mode off, malformado, settings) ---

func TestAttemptReviewProcessor_EventType(t *testing.T) {
	p := newProcessor(&mockSettingsReader{}, &mockLearningClient{}, &mockLLMProvider{})
	if p.EventType() != "attempt.review_requested" {
		t.Fatalf("event_type inesperado: %s", p.EventType())
	}
}

func TestAttemptReviewProcessor_ModeOff_AckSinAccion(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeOff)}
	learning := &mockLearningClient{}
	p := newProcessor(reader, learning, &mockLLMProvider{})

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("mode=off no debe retornar error (ACK): %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("esperaba 1 lectura de settings, hubo %d", reader.calls)
	}
	if learning.finalizeCalls != 0 || len(learning.reviewCalls) != 0 {
		t.Fatalf("mode=off no debe tocar learning")
	}
}

func TestAttemptReviewProcessor_DefaultOff_CuandoNoHaySetting(t *testing.T) {
	reader := &mockSettingsReader{settings: m2m.SchoolSettings{SchoolID: "school-1"}}
	p := newProcessor(reader, &mockLearningClient{}, &mockLLMProvider{})

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("default off no debe retornar error: %v", err)
	}
}

func TestAttemptReviewProcessor_EventoMalformado_ErrorPermanente(t *testing.T) {
	p := newProcessor(&mockSettingsReader{}, &mockLearningClient{}, &mockLLMProvider{})

	err := p.Process(context.Background(), []byte(`{no es json`))
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("esperaba ErrMalformedEvent, hubo: %v", err)
	}
	if classifyError(err) != ErrorTypePermanent {
		t.Fatalf("evento malformado debe clasificar como permanente")
	}
}

func TestAttemptReviewProcessor_SettingsInaccesible_ErrorTransitorio(t *testing.T) {
	reader := &mockSettingsReader{err: errors.New("academic caído")}
	p := newProcessor(reader, &mockLearningClient{}, &mockLLMProvider{})

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error transitorio por settings inaccesible")
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("fallo de settings debe clasificar como transitorio")
	}
}

// --- tests de orquestación (F2) ---

func TestAttemptReviewProcessor_Direct_Feliz_DosReviewsYFinalize(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		AttemptID: "attempt-1",
		Answers:   []m2m.PendingAnswer{pendingAnswer("a1", 10), pendingAnswer("a2", 5)},
	}}
	provider := &mockLLMProvider{score: 0.5, feedback: "parcial", verdict: llm.VerdictPartial}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("flujo direct feliz no debe fallar: %v", err)
	}
	if len(learning.reviewCalls) != 2 {
		t.Fatalf("esperaba 2 reviews, hubo %d", len(learning.reviewCalls))
	}
	// Score 0.5 * 10 = 5.0 ; 0.5 * 5 = 2.5
	if learning.reviewCalls[0].PointsAwarded != 5.0 {
		t.Fatalf("points_awarded a1 esperado 5.0, hubo %v", learning.reviewCalls[0].PointsAwarded)
	}
	if learning.reviewCalls[1].PointsAwarded != 2.5 {
		t.Fatalf("points_awarded a2 esperado 2.5, hubo %v", learning.reviewCalls[1].PointsAwarded)
	}
	if learning.finalizeCalls != 1 {
		t.Fatalf("flow direct debe finalizar 1 vez, hubo %d", learning.finalizeCalls)
	}
}

func TestAttemptReviewProcessor_Teacher_ReviewsSinFinalize(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeAPI, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)},
	}}
	provider := &mockLLMProvider{score: 1.0, feedback: "correcto", verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("flow teacher no debe fallar: %v", err)
	}
	if len(learning.reviewCalls) != 1 {
		t.Fatalf("esperaba 1 review, hubo %d", len(learning.reviewCalls))
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("flow teacher NO debe finalizar, hubo %d", learning.finalizeCalls)
	}
}

func TestAttemptReviewProcessor_Direct_SinPendientes_Finaliza(t *testing.T) {
	// Redelivery: GET vacío + flow direct → finalize idempotente, sin reviews.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{Answers: nil}}
	provider := &mockLLMProvider{}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("GET vacío + direct no debe fallar: %v", err)
	}
	if len(learning.reviewCalls) != 0 {
		t.Fatalf("sin pendientes no debe haber reviews, hubo %d", len(learning.reviewCalls))
	}
	if learning.finalizeCalls != 1 {
		t.Fatalf("sin pendientes + direct debe finalizar 1 vez, hubo %d", learning.finalizeCalls)
	}
	if provider.calls != 0 {
		t.Fatalf("sin pendientes no debe invocar al LLM, hubo %d", provider.calls)
	}
}

func TestAttemptReviewProcessor_Teacher_SinPendientes_AckSinFinalize(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{Answers: nil}}
	p := newProcessor(reader, learning, &mockLLMProvider{})

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("GET vacío + teacher no debe fallar: %v", err)
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("teacher sin pendientes no debe finalizar, hubo %d", learning.finalizeCalls)
	}
}

func TestAttemptReviewProcessor_FalloLLMaMitad_ErrorSinFinalize(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10), pendingAnswer("a2", 10)},
	}}
	// Falla en la segunda corrección.
	provider := &mockLLMProvider{score: 0.8, failOnCall: 2, err: errors.New("ollama timeout")}
	p := newProcessor(reader, learning, provider)

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error por fallo del LLM a mitad")
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("fallo del LLM debe clasificar transitorio (retry seguro)")
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("no debe finalizar si falló una corrección, hubo %d", learning.finalizeCalls)
	}
	// La primera review sí se escribió (upsert idempotente); el redelivery la re-lee.
	if len(learning.reviewCalls) != 1 {
		t.Fatalf("esperaba 1 review escrita antes del fallo, hubo %d", len(learning.reviewCalls))
	}
}

func TestAttemptReviewProcessor_LearningPermanente_ErrorPermanente(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{
		pendingErr: fmt.Errorf("%w: 404 answer inexistente", m2m.ErrLearningPermanent),
	}
	p := newProcessor(reader, learning, &mockLLMProvider{})

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error por 4xx de learning")
	}
	if classifyError(err) != ErrorTypePermanent {
		t.Fatalf("4xx de learning debe clasificar permanente")
	}
}

func TestAttemptReviewProcessor_ModeSinProvider_ErrorPermanente(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeAPI)}
	// providers vacío: mode=api no tiene provider disponible.
	p := NewAttemptReviewProcessor(reader, &mockLearningClient{}, map[string]llm.LLMProvider{}, newTestLogger())

	err := p.Process(context.Background(), validEventPayload(t))
	if !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("mode sin provider debe ser permanente (ErrMalformedEvent), hubo: %v", err)
	}
}
