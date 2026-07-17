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
	claimErr    error
	claimCalls  int
	releaseErr  error
	releaseCall int

	pending    m2m.PendingAnswersResponse
	pendingErr error

	reviewErr    error
	reviewCalls  []m2m.AnswerReviewRequest
	reviewAnswer []string // answer_ids en orden

	finalizeErr   error
	finalizeCalls int
}

func (m *mockLearningClient) Claim(_ context.Context, _ string) error {
	m.claimCalls++
	return m.claimErr
}

func (m *mockLearningClient) ReleaseClaim(_ context.Context, _ string) error {
	m.releaseCall++
	return m.releaseErr
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

func (m *mockLLMProvider) PrepareQuestion(_ context.Context, _ llm.PrepRequest) (json.RawMessage, error) {
	return nil, errors.New("no usado en estos tests")
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

func shortAnswerPending(id string, points float64) m2m.PendingAnswer {
	return m2m.PendingAnswer{
		AnswerID:       id,
		QuestionType:   "short_answer",
		QuestionText:   "¿Capital de Francia?",
		ExpectedAnswer: "París",
		StudentAnswer:  "paris",
		Points:         points,
	}
}

func shortAnswerEventPayload(t *testing.T) []byte {
	t.Helper()
	evt := events.AttemptReviewRequestedEvent{
		EventID:      "evt-sa",
		EventType:    EventTypeAttemptReviewRequested,
		EventVersion: "1.0",
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID:    "attempt-1",
			AssessmentID: "assessment-1",
			SchoolID:     "school-1",
			Answers: []events.AttemptReviewAnswerRef{
				{AnswerID: "a1", QuestionType: "short_answer"},
			},
		},
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal evento short_answer: %v", err)
	}
	return b
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
	// Falla en la segunda corrección. La primera devuelve un verdict válido para
	// que sí se escriba su review antes del fallo.
	provider := &mockLLMProvider{score: 0.8, verdict: llm.VerdictCorrect, failOnCall: 2, err: errors.New("ollama timeout")}
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

func TestAttemptReviewProcessor_VerdictVacio_NoProponeReview(t *testing.T) {
	// Regresión del bug de qwen3: el LLM devuelve `{}` → verdict="" (inválido). NO
	// se debe postear una review IA de 0 puntos «propuesta»; se cuenta como fallo
	// del LLM (transitorio) y la answer queda sin review para el profesor.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)},
	}}
	// Verdict vacío (default): simula el `{}` de qwen3. Score 0, feedback vacío.
	provider := &mockLLMProvider{verdict: "", score: 0}
	p := newProcessor(reader, learning, provider)

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error por verdict inválido del LLM")
	}
	if !errors.Is(err, ErrInvalidVerdict) {
		t.Fatalf("esperaba ErrInvalidVerdict, hubo: %v", err)
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("verdict inválido debe clasificar transitorio (mismo carril que fallo de provider)")
	}
	if len(learning.reviewCalls) != 0 {
		t.Fatalf("NUNCA se debe postear review con verdict inválido, hubo %d", len(learning.reviewCalls))
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("no debe finalizar si el verdict fue inválido, hubo %d", learning.finalizeCalls)
	}
}

func TestAttemptReviewProcessor_ShortAnswer_VerdictPartial_NoProponeReview(t *testing.T) {
	// "partial" es válido para open_ended pero NO para short_answer (binario). No
	// se debe postear la propuesta.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerPending("a1", 5)},
	}}
	provider := &mockLLMProvider{verdict: llm.VerdictPartial, score: 0.5}
	p := newProcessor(reader, learning, provider)

	err := p.Process(context.Background(), shortAnswerEventPayload(t))
	if !errors.Is(err, ErrInvalidVerdict) {
		t.Fatalf("esperaba ErrInvalidVerdict para 'partial' en short_answer, hubo: %v", err)
	}
	if len(learning.reviewCalls) != 0 {
		t.Fatalf("no debe postear review con verdict inválido para short_answer, hubo %d", len(learning.reviewCalls))
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

// --- tests del candado (plan 040 F4: claim/release) ---

func TestAttemptReviewProcessor_ReclamaAntesDeProcesar(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)},
	}}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("flujo con claim OK no debe fallar: %v", err)
	}
	if learning.claimCalls != 1 {
		t.Fatalf("esperaba 1 claim antes de procesar, hubo %d", learning.claimCalls)
	}
	// open_ended + direct → finalize (no release).
	if learning.finalizeCalls != 1 {
		t.Fatalf("open_ended direct debe finalizar, hubo %d", learning.finalizeCalls)
	}
	if learning.releaseCall != 0 {
		t.Fatalf("no debe liberar candado cuando finaliza, hubo %d", learning.releaseCall)
	}
}

func TestAttemptReviewProcessor_ClaimConflict_SeAbstiene(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{
		claimErr: fmt.Errorf("%w: candado ajeno vigente", m2m.ErrClaimConflict),
		pending:  m2m.PendingAnswersResponse{Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)}},
	}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	// 409 → abstenerse: ACK sin error, sin tocar el resto de learning ni el LLM.
	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("claim 409 debe abstenerse (ACK), hubo error: %v", err)
	}
	if learning.claimCalls != 1 {
		t.Fatalf("esperaba 1 intento de claim, hubo %d", learning.claimCalls)
	}
	if len(learning.reviewCalls) != 0 || learning.finalizeCalls != 0 || learning.releaseCall != 0 {
		t.Fatalf("tras 409 no debe revisar/finalizar/liberar: reviews=%d finalize=%d release=%d",
			len(learning.reviewCalls), learning.finalizeCalls, learning.releaseCall)
	}
	if provider.calls != 0 {
		t.Fatalf("tras 409 no debe invocar al LLM, hubo %d", provider.calls)
	}
}

func TestAttemptReviewProcessor_ClaimTransitorio_Error(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{claimErr: errors.New("learning 503")}
	p := newProcessor(reader, learning, &mockLLMProvider{})

	err := p.Process(context.Background(), validEventPayload(t))
	if err == nil {
		t.Fatal("esperaba error transitorio por fallo de claim (no 409)")
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("fallo de claim (no 409) debe clasificar transitorio")
	}
}

func TestAttemptReviewProcessor_ShortAnswer_ReleaseSinFinalize(t *testing.T) {
	// short_answer con flow=direct: la presencia de short_answer FUERZA teacher →
	// se revisa, se libera el candado y NO se finaliza (queda para el profesor).
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerPending("a1", 5)},
	}}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect, feedback: "equivalente"}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), shortAnswerEventPayload(t)); err != nil {
		t.Fatalf("flujo short_answer no debe fallar: %v", err)
	}
	if learning.claimCalls != 1 {
		t.Fatalf("esperaba 1 claim, hubo %d", learning.claimCalls)
	}
	if len(learning.reviewCalls) != 1 {
		t.Fatalf("esperaba 1 review de short_answer, hubo %d", len(learning.reviewCalls))
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("short_answer NO debe finalizar (aunque flow=direct), hubo %d", learning.finalizeCalls)
	}
	if learning.releaseCall != 1 {
		t.Fatalf("short_answer debe liberar el candado 1 vez, hubo %d", learning.releaseCall)
	}
}

func TestAttemptReviewProcessor_Mixto_NoFinalize(t *testing.T) {
	// Intento MIXTO (open_ended + short_answer) con flow=direct: la presencia de
	// short_answer impide finalizar; se revisan ambas y se libera el candado.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10), shortAnswerPending("a2", 5)},
	}}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	evt := events.AttemptReviewRequestedEvent{
		EventID: "evt-mix", EventType: EventTypeAttemptReviewRequested, EventVersion: "1.0",
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID: "attempt-1", AssessmentID: "assessment-1", SchoolID: "school-1",
			Answers: []events.AttemptReviewAnswerRef{
				{AnswerID: "a1", QuestionType: "open_ended"},
				{AnswerID: "a2", QuestionType: "short_answer"},
			},
		},
	}
	payload, _ := json.Marshal(evt)

	if err := p.Process(context.Background(), payload); err != nil {
		t.Fatalf("flujo mixto no debe fallar: %v", err)
	}
	if len(learning.reviewCalls) != 2 {
		t.Fatalf("esperaba 2 reviews en mixto, hubo %d", len(learning.reviewCalls))
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("mixto con short_answer NO debe finalizar, hubo %d", learning.finalizeCalls)
	}
	if learning.releaseCall != 1 {
		t.Fatalf("mixto debe liberar el candado, hubo %d", learning.releaseCall)
	}
}
