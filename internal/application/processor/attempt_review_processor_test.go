package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
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

	// pareo binario (carril triturado short_answer, F3c).
	pairVerdict llm.Verdict // veredicto que devuelve JudgePairEquivalence
	pairCalls   int

	// comprobación por criterio (carril open_ended, F4b). criterionMet indexa por
	// texto de criterio el veredicto; ausente ⇒ incorrect.
	criterionMet   map[string]bool
	criterionCalls int
	// lastReviewReq captura la última ReviewRequest (para verificar el enriquecimiento F4a).
	lastReviewReq llm.ReviewRequest

	// extracción de ideas (carril open_ended, F4/D-045.9). Por defecto vacía ⇒ Grade cae
	// al juicio contra la respuesta cruda (comportamiento previo a F4).
	extractIdeas []string
	extractErr   error
	extractCalls int
}

func (m *mockLLMProvider) GenerateAssessment(_ context.Context, _ llm.MaterialInput, _ llm.GenerationParams) (json.RawMessage, error) {
	return nil, errors.New("no usado en estos tests")
}

func (m *mockLLMProvider) ReviewAnswer(_ context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	m.calls++
	m.lastReviewReq = req
	if m.failOnCall != 0 && m.calls == m.failOnCall {
		return llm.ReviewResult{}, m.err
	}
	return llm.ReviewResult{Verdict: m.verdict, Score: m.score, Feedback: m.feedback}, nil
}

func (m *mockLLMProvider) PrepareQuestion(_ context.Context, _ llm.PrepRequest) (json.RawMessage, error) {
	return nil, errors.New("no usado en estos tests")
}

func (m *mockLLMProvider) JudgePairEquivalence(_ context.Context, _ llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	m.pairCalls++
	v := m.pairVerdict
	if v == "" {
		v = llm.VerdictIncorrect
	}
	score := 0.0
	if v == llm.VerdictCorrect {
		score = 1.0
	}
	return llm.ReviewResult{Verdict: v, Score: score, Feedback: "par: " + string(v)}, nil
}

func (m *mockLLMProvider) CheckCriterion(_ context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	m.criterionCalls++
	v := llm.VerdictIncorrect
	if m.criterionMet[req.Criterion] {
		v = llm.VerdictCorrect
	}
	score := 0.0
	if v == llm.VerdictCorrect {
		score = 1.0
	}
	return llm.ReviewResult{Verdict: v, Score: score, Feedback: "criterio: " + string(v)}, nil
}

func (m *mockLLMProvider) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	m.extractCalls++
	if m.extractErr != nil {
		return nil, m.extractErr
	}
	return m.extractIdeas, nil
}

func (m *mockLLMProvider) DigestChunk(_ context.Context, _ llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	return nil, nil
}

func (m *mockLLMProvider) ProposeCandidates(_ context.Context, _ llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	return nil, nil
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

// listPrepRaw es un prep válido content_kind=list (Gran Colombia) para inyectar en
// la pending answer y disparar el carril triturado (F3c).
const listPrepRaw = `{"version":1,"question_type":"short_answer","content_kind":"list",` +
	`"items":["ecuador","venezuela","colombia"],"items_verbatim":["Ecuador","Venezuela","Colombia"]}`

func shortAnswerListPending(id, student string) m2m.PendingAnswer {
	return m2m.PendingAnswer{
		AnswerID:      id,
		QuestionType:  "short_answer",
		QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
		StudentAnswer: student,
		Points:        3,
		LLMPrep:       json.RawMessage(listPrepRaw),
	}
}

func TestAttemptReviewProcessor_ShortAnswer_PrepList_Determinista(t *testing.T) {
	// Prep list + respuesta que cubre los 3 ítems por match determinista: NO se llama
	// ReviewAnswer NI JudgePairEquivalence; la review posteada es correct (puntaje pleno).
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerListPending("a1", "Ecuador Venezuela y Colombia")},
	}}
	provider := &mockLLMProvider{}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), shortAnswerEventPayload(t)); err != nil {
		t.Fatalf("flujo triturado no debe fallar: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("el carril triturado NO debe llamar ReviewAnswer, hubo %d", provider.calls)
	}
	if provider.pairCalls != 0 {
		t.Fatalf("todo determinista: 0 llamadas de par, hubo %d", provider.pairCalls)
	}
	if len(learning.reviewCalls) != 1 || learning.reviewCalls[0].PointsAwarded != 3 {
		t.Fatalf("esperaba 1 review con 3 puntos (correct), hubo %+v", learning.reviewCalls)
	}
}

func TestAttemptReviewProcessor_ShortAnswer_PrepList_ParRescata(t *testing.T) {
	// Prep list + un ítem representado por un token LEJANO que el fuzzy no cubre
	// ("caracas"↔"venezuela", metonimia) → un ítem residual que escala a UNA llamada de
	// par que lo rescata semánticamente. (Un typo cercano como "benezuela" ya lo
	// resolvería el fuzzy sin LLM; ver el unit test TestGrade_TypoRescatadoPorFuzzy.)
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerListPending("a1", "ecuador, caracas y colombia")},
	}}
	provider := &mockLLMProvider{pairVerdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), shortAnswerEventPayload(t)); err != nil {
		t.Fatalf("flujo triturado no debe fallar: %v", err)
	}
	if provider.pairCalls != 1 {
		t.Fatalf("esperaba exactamente 1 llamada de par, hubo %d", provider.pairCalls)
	}
	if len(learning.reviewCalls) != 1 || learning.reviewCalls[0].PointsAwarded != 3 {
		t.Fatalf("esperaba review correct con 3 puntos tras rescate, hubo %+v", learning.reviewCalls)
	}
}

func TestAttemptReviewProcessor_ShortAnswer_PrepList_Falta(t *testing.T) {
	// Prep list + falta un ítem sin candidato sobrante: incorrect/0 puntos, sin par.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerListPending("a1", "ecuador y colombia")},
	}}
	provider := &mockLLMProvider{}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), shortAnswerEventPayload(t)); err != nil {
		t.Fatalf("flujo triturado no debe fallar: %v", err)
	}
	if provider.pairCalls != 0 {
		t.Fatalf("sin candidato sobrante: 0 llamadas de par, hubo %d", provider.pairCalls)
	}
	if len(learning.reviewCalls) != 1 || learning.reviewCalls[0].PointsAwarded != 0 {
		t.Fatalf("esperaba review incorrect con 0 puntos, hubo %+v", learning.reviewCalls)
	}
}

func TestAttemptReviewProcessor_ShortAnswer_SinPrep_FlujoGlobal(t *testing.T) {
	// Sin prep: se conserva el flujo global (ReviewAnswer), degradación suave (D-042.10 §4).
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{shortAnswerPending("a1", 5)},
	}}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), shortAnswerEventPayload(t)); err != nil {
		t.Fatalf("flujo global short_answer no debe fallar: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("sin prep debe usar ReviewAnswer 1 vez, hubo %d", provider.calls)
	}
	if provider.pairCalls != 0 {
		t.Fatalf("sin prep no hay pares, hubo %d", provider.pairCalls)
	}
}

// criteriaPrepRaw es un prep válido open_ended con 3 criterios (F4b).
const criteriaPrepRaw = `{"version":1,"question_type":"open_ended",` +
	`"question_intent":"medir si explica la fotosíntesis",` +
	`"main_ideas":["convierte luz en energía"],` +
	`"criteria":["menciona los cloroplastos","menciona la luz","menciona el oxígeno"]}`

// enrichPrepRaw es un prep válido open_ended SIN criterios (solo enriquecimiento F4a).
const enrichPrepRaw = `{"version":1,"question_type":"open_ended",` +
	`"question_intent":"medir si identifica el nivel de agua",` +
	`"main_ideas":["el vaso tiene la mitad de agua"],` +
	`"valid_variants":["el vaso está medio lleno","el vaso está medio vacío"]}`

func openEndedPending(id string, points float64, prep string) m2m.PendingAnswer {
	return m2m.PendingAnswer{
		AnswerID:       id,
		QuestionType:   "open_ended",
		QuestionText:   "Explica la fotosíntesis.",
		ExpectedAnswer: "las plantas convierten luz en energía",
		StudentAnswer:  "las plantas usan la luz para producir energía y sueltan oxígeno",
		Points:         points,
		LLMPrep:        json.RawMessage(prep),
	}
}

func openEndedEventPayload(t *testing.T) []byte {
	t.Helper()
	evt := events.AttemptReviewRequestedEvent{
		EventID: "evt-oe", EventType: EventTypeAttemptReviewRequested, EventVersion: "1.0",
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID: "attempt-1", AssessmentID: "assessment-1", SchoolID: "school-1",
			Answers: []events.AttemptReviewAnswerRef{{AnswerID: "a1", QuestionType: "open_ended"}},
		},
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal evento open_ended: %v", err)
	}
	return b
}

func TestAttemptReviewProcessor_OpenEnded_Criterios_Partial(t *testing.T) {
	// Prep con 3 criterios, el alumno cumple 2 → una llamada por criterio (3), NO se
	// usa ReviewAnswer, y la review posteada es partial con puntaje proporcional.
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{openEndedPending("a1", 9, criteriaPrepRaw)},
	}}
	provider := &mockLLMProvider{criterionMet: map[string]bool{
		"menciona la luz":     true,
		"menciona el oxígeno": true,
	}}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), openEndedEventPayload(t)); err != nil {
		t.Fatalf("carril por criterios no debe fallar: %v", err)
	}
	if provider.criterionCalls != 3 {
		t.Fatalf("esperaba 3 llamadas de criterio, hubo %d", provider.criterionCalls)
	}
	if provider.calls != 0 {
		t.Fatalf("el carril por criterios NO debe llamar ReviewAnswer, hubo %d", provider.calls)
	}
	if len(learning.reviewCalls) != 1 {
		t.Fatalf("esperaba 1 review, hubo %d", len(learning.reviewCalls))
	}
	// 2/3 → score 0.3+0.4*(2/3)=0.5667 → puntos 9*0.5667=5.10 (redondeo 2 dec).
	got := learning.reviewCalls[0].PointsAwarded
	if got < 5.09 || got > 5.11 {
		t.Fatalf("esperaba ~5.10 puntos (partial 2/3 de 9), hubo %.2f", got)
	}
}

func TestAttemptReviewProcessor_OpenEnded_SinCriterios_EnriquecePrompt(t *testing.T) {
	// Prep open_ended sin criterios: se usa ReviewAnswer (flujo global) PERO con el
	// prompt enriquecido (req.Prep != nil con intención/ideas/variantes).
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{openEndedPending("a1", 10, enrichPrepRaw)},
	}}
	provider := &mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), openEndedEventPayload(t)); err != nil {
		t.Fatalf("flujo global enriquecido no debe fallar: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("esperaba 1 ReviewAnswer, hubo %d", provider.calls)
	}
	if provider.criterionCalls != 0 {
		t.Fatalf("sin criterios no hay llamadas de criterio, hubo %d", provider.criterionCalls)
	}
	if provider.lastReviewReq.Prep == nil {
		t.Fatal("el prompt de open_ended debe recibir el prep (req.Prep != nil)")
	}
	if len(provider.lastReviewReq.Prep.ValidVariants) != 2 {
		t.Fatalf("esperaba 2 variantes válidas en el prep, hubo %d", len(provider.lastReviewReq.Prep.ValidVariants))
	}
}

func TestAttemptReviewProcessor_OpenEnded_CriteriosEnBlanco_NoCeroSilencioso(t *testing.T) {
	// Prep estructuralmente parecido pero con criteria=["","  "]: el validador lo rechaza
	// (prep se ignora) y el carril degrada al flujo global, NUNCA a un incorrect/0.0 sin
	// consultar al LLM. Verificamos que se consulta ReviewAnswer y no hay 0 silencioso.
	blankCriteriaPrep := `{"version":1,"question_type":"open_ended",` +
		`"question_intent":"medir si explica la fotosíntesis",` +
		`"main_ideas":["convierte luz en energía"],` +
		`"criteria":["","  "]}`
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{openEndedPending("a1", 10, blankCriteriaPrep)},
	}}
	provider := &mockLLMProvider{score: 0.8, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), openEndedEventPayload(t)); err != nil {
		t.Fatalf("criterios en blanco no debe fallar el proceso: %v", err)
	}
	if provider.criterionCalls != 0 {
		t.Fatalf("no debe entrar al carril por criterios (0 llamadas), hubo %d", provider.criterionCalls)
	}
	if provider.calls != 1 {
		t.Fatalf("debe caer al flujo global (1 ReviewAnswer), hubo %d", provider.calls)
	}
	if len(learning.reviewCalls) != 1 {
		t.Fatalf("esperaba 1 review posteada, hubo %d", len(learning.reviewCalls))
	}
	// El puntaje NO es 0 silencioso: viene del juicio global. Desde la HU-INF.5 (P8)
	// el número sale del veredicto, no del score del modelo: «correct» con 0.8 → 10 de 10.
	if got := learning.reviewCalls[0].PointsAwarded; got < 9.99 || got > 10.01 {
		t.Fatalf("esperaba 10 puntos del flujo global (veredicto correct), hubo %.2f (¿0 silencioso?)", got)
	}
}

func TestNonBlankCriteria(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want int
	}{
		{"todos válidos", []string{"c1", "c2"}, 2},
		{"mezcla con vacío", []string{"c1", "  ", "c2"}, 2},
		{"todos en blanco", []string{"", "  ", "\t"}, 0},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(nonBlankCriteria(tc.in)); got != tc.want {
				t.Fatalf("nonBlankCriteria(%v) = %d elementos, quería %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestAttemptReviewProcessor_OpenEnded_SinPrep_FlujoGlobal(t *testing.T) {
	// Sin prep: flujo global intacto, req.Prep nil (fallback F4a).
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)},
	}}
	provider := &mockLLMProvider{score: 0.9, verdict: llm.VerdictCorrect}
	p := newProcessor(reader, learning, provider)

	if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
		t.Fatalf("flujo global sin prep no debe fallar: %v", err)
	}
	if provider.calls != 1 || provider.criterionCalls != 0 {
		t.Fatalf("esperaba 1 ReviewAnswer y 0 criterios, hubo calls=%d crit=%d", provider.calls, provider.criterionCalls)
	}
	if provider.lastReviewReq.Prep != nil {
		t.Fatal("sin prep, req.Prep debe ser nil")
	}
}

// --- HU-INF.1 (bug 0138): el reintento no muere en su propio candado ---

// statefulLearning es un doble de learning CON estado: el candado no es reentrante
// (un segundo Claim con el candado tomado devuelve el 409 de learning, como
// AI_REVIEW_CLAIM_ACTIVE) y el GET de pendientes filtra las respuestas ya revisadas,
// igual que el usecase real. Con él, dos llamadas a Process simulan la reentrega.
type statefulLearning struct {
	answers  []m2m.PendingAnswer
	reviewed map[string]m2m.AnswerReviewRequest
	claimed  bool

	claimCalls      int
	releaseCalls    int
	releaseCtxErrs  []error
	releaseDeadline []time.Duration
	finalizeCalls   int
}

func newStatefulLearning(answers ...m2m.PendingAnswer) *statefulLearning {
	return &statefulLearning{answers: answers, reviewed: map[string]m2m.AnswerReviewRequest{}}
}

func (s *statefulLearning) Claim(_ context.Context, _ string) error {
	s.claimCalls++
	if s.claimed {
		return fmt.Errorf(`%w: {"code":"AI_REVIEW_CLAIM_ACTIVE"}`, m2m.ErrClaimConflict)
	}
	s.claimed = true
	return nil
}

func (s *statefulLearning) ReleaseClaim(ctx context.Context, _ string) error {
	s.releaseCalls++
	s.releaseCtxErrs = append(s.releaseCtxErrs, ctx.Err())
	if dl, ok := ctx.Deadline(); ok {
		s.releaseDeadline = append(s.releaseDeadline, time.Until(dl))
	} else {
		s.releaseDeadline = append(s.releaseDeadline, -1)
	}
	if ctx.Err() != nil {
		// Un contexto cancelado no llega a learning: el candado sigue tomado.
		return ctx.Err()
	}
	s.claimed = false
	return nil
}

func (s *statefulLearning) GetPendingAnswers(_ context.Context, attemptID string) (m2m.PendingAnswersResponse, error) {
	out := m2m.PendingAnswersResponse{AttemptID: attemptID}
	for _, a := range s.answers {
		if _, done := s.reviewed[a.AnswerID]; !done {
			out.Answers = append(out.Answers, a)
		}
	}
	return out, nil
}

func (s *statefulLearning) PostAnswerReview(_ context.Context, _ string, answerID string, review m2m.AnswerReviewRequest) (m2m.AnswerReviewResponse, error) {
	s.reviewed[answerID] = review
	return m2m.AnswerReviewResponse{AnswerID: answerID, ReviewStatus: "reviewed"}, nil
}

func (s *statefulLearning) FinalizeAttempt(_ context.Context, attemptID string) (m2m.FinalizeResponse, error) {
	s.finalizeCalls++
	s.claimed = false
	return m2m.FinalizeResponse{AttemptID: attemptID, Status: "completed"}, nil
}

// failingByAnswerProvider falla según la respuesta, no según el número de llamada.
// ReviewRequest no lleva el answer_id (y reviewOne no se toca en esta HU), así que
// cada respuesta del test lleva un StudentAnswer único y se indexa por él.
// failTimes[x] = n hace fallar n veces la respuesta x; -1, siempre.
type failingByAnswerProvider struct {
	mockLLMProvider
	failTimes map[string]int
	callsBy   map[string]int
}

func newFailingByAnswerProvider(failTimes map[string]int) *failingByAnswerProvider {
	return &failingByAnswerProvider{
		mockLLMProvider: mockLLMProvider{score: 1.0, verdict: llm.VerdictCorrect, feedback: "ok"},
		failTimes:       failTimes,
		callsBy:         map[string]int{},
	}
}

func (f *failingByAnswerProvider) ReviewAnswer(ctx context.Context, req llm.ReviewRequest) (llm.ReviewResult, error) {
	f.callsBy[req.StudentAnswer]++
	n := f.failTimes[req.StudentAnswer]
	if n < 0 || f.callsBy[req.StudentAnswer] <= n {
		return llm.ReviewResult{}, errors.New("ollama timeout en " + req.StudentAnswer)
	}
	return f.mockLLMProvider.ReviewAnswer(ctx, req)
}

// openEndedWithText es una respuesta abierta cuyo StudentAnswer es su propio id, para
// que failingByAnswerProvider la reconozca.
func openEndedWithText(id string, points float64) m2m.PendingAnswer {
	a := pendingAnswer(id, points)
	a.StudentAnswer = id
	return a
}

func directSettings() *mockSettingsReader {
	return &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowDirect)}
}

// CA-INF.1.local: el modelo falla una vez en la segunda respuesta y el mensaje se
// reentrega; el reintento reclama y corrige solo la que faltaba.
func TestAttemptReview_RedeliveryAfterMidFailure_ReviewsMissingAnswer(t *testing.T) {
	learning := newStatefulLearning(openEndedWithText("a1", 10), openEndedWithText("a2", 10))
	provider := newFailingByAnswerProvider(map[string]int{"a2": 1})
	p := newProcessor(directSettings(), learning, provider)
	payload := validEventPayload(t)

	if err := p.Process(context.Background(), payload); err == nil {
		t.Fatal("la primera entrega debe devolver error (falló a2) para que el consumer reintente")
	}
	if learning.claimed {
		t.Fatal("tras la salida con error el candado debe quedar libre")
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("con una respuesta sin propuesta no se finaliza, hubo %d", learning.finalizeCalls)
	}

	// Reentrega del mismo mensaje.
	if err := p.Process(context.Background(), payload); err != nil {
		t.Fatalf("la reentrega debe completar la revisión, hubo error: %v", err)
	}
	if learning.claimCalls != 2 {
		t.Fatalf("la reentrega debe reclamar de nuevo: esperaba 2 claims, hubo %d", learning.claimCalls)
	}
	if _, ok := learning.reviewed["a2"]; !ok {
		t.Fatal("la reentrega debe dejar a2 con propuesta")
	}
	if provider.callsBy["a1"] != 1 || provider.callsBy["a2"] != 2 {
		t.Fatalf("la reentrega solo debe volver sobre a2: llamadas a1=%d a2=%d", provider.callsBy["a1"], provider.callsBy["a2"])
	}
	if learning.finalizeCalls != 1 {
		t.Fatalf("flow direct debe finalizar al completar, hubo %d", learning.finalizeCalls)
	}
}

// Las cinco salidas con error con el candado tomado devuelven error y liberan una vez.
func TestAttemptReview_ErrorExits_ReleaseClaim(t *testing.T) {
	cases := []struct {
		name     string
		learning *mockLearningClient
		provider *mockLLMProvider
	}{
		{
			name:     "falla la lectura de pendientes",
			learning: &mockLearningClient{pendingErr: errors.New("learning 503")},
			provider: &mockLLMProvider{score: 1, verdict: llm.VerdictCorrect},
		},
		{
			name: "falla el modelo",
			learning: &mockLearningClient{pending: m2m.PendingAnswersResponse{
				Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)}}},
			provider: &mockLLMProvider{failOnCall: 1, err: errors.New("ollama timeout")},
		},
		{
			name: "veredicto inválido",
			learning: &mockLearningClient{pending: m2m.PendingAnswersResponse{
				Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)}}},
			provider: &mockLLMProvider{verdict: ""},
		},
		{
			name: "falla la escritura de la review",
			learning: &mockLearningClient{
				pending:   m2m.PendingAnswersResponse{Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)}},
				reviewErr: errors.New("learning 503"),
			},
			provider: &mockLLMProvider{score: 1, verdict: llm.VerdictCorrect},
		},
		{
			name: "falla finalize al terminar",
			learning: &mockLearningClient{
				pending:     m2m.PendingAnswersResponse{Answers: []m2m.PendingAnswer{pendingAnswer("a1", 10)}},
				finalizeErr: errors.New("learning 503"),
			},
			provider: &mockLLMProvider{score: 1, verdict: llm.VerdictCorrect},
		},
		{
			name: "falla finalize sin pendientes",
			learning: &mockLearningClient{
				pending:     m2m.PendingAnswersResponse{Answers: nil},
				finalizeErr: errors.New("learning 503"),
			},
			provider: &mockLLMProvider{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProcessor(directSettings(), tc.learning, tc.provider)
			if err := p.Process(context.Background(), validEventPayload(t)); err == nil {
				t.Fatal("la salida debe devolver error para que el consumer reintente")
			}
			if tc.learning.claimCalls != 1 {
				t.Fatalf("esperaba 1 claim, hubo %d", tc.learning.claimCalls)
			}
			if tc.learning.releaseCall != 1 {
				t.Fatalf("la salida con error debe liberar el candado 1 vez, hubo %d", tc.learning.releaseCall)
			}
		})
	}
}

// Una respuesta que falla siempre no deja sin propuesta a las de detrás.
func TestAttemptReview_FailingAnswer_DoesNotBlockOthers(t *testing.T) {
	learning := newStatefulLearning(
		openEndedWithText("a1", 10), openEndedWithText("a2", 10), openEndedWithText("a3", 10))
	provider := newFailingByAnswerProvider(map[string]int{"a1": -1})
	p := newProcessor(directSettings(), learning, provider)
	payload := validEventPayload(t)

	err := p.Process(context.Background(), payload)
	if err == nil {
		t.Fatal("con a1 sin propuesta Process debe devolver error")
	}
	if classifyError(err) != ErrorTypeTransient {
		t.Fatalf("el fallo del modelo debe seguir siendo transitorio, hubo %v", err)
	}
	for _, id := range []string{"a2", "a3"} {
		if _, ok := learning.reviewed[id]; !ok {
			t.Fatalf("%s debe quedar con propuesta aunque a1 falle", id)
		}
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("con una respuesta sin propuesta no se finaliza, hubo %d", learning.finalizeCalls)
	}

	// Reentrega: solo vuelve sobre a1.
	if err := p.Process(context.Background(), payload); err == nil {
		t.Fatal("a1 sigue fallando: la reentrega debe devolver error")
	}
	if provider.callsBy["a1"] != 2 || provider.callsBy["a2"] != 1 || provider.callsBy["a3"] != 1 {
		t.Fatalf("la reentrega solo debe volver sobre a1: a1=%d a2=%d a3=%d",
			provider.callsBy["a1"], provider.callsBy["a2"], provider.callsBy["a3"])
	}
	if learning.releaseCalls != 2 || learning.claimed {
		t.Fatalf("cada entrega con error debe liberar: releases=%d, tomado=%v", learning.releaseCalls, learning.claimed)
	}
}

// Con el contexto del mensaje cancelado (apagado del worker), la liberación llega a
// learning con un contexto vivo y con tope corto.
func TestAttemptReview_ReleaseSurvivesCancelledContext(t *testing.T) {
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := newStatefulLearning() // sin pendientes: camino que libera hoy
	p := newProcessor(reader, learning, &mockLLMProvider{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = p.Process(ctx, validEventPayload(t))

	if learning.releaseCalls != 1 {
		t.Fatalf("esperaba 1 liberación, hubo %d", learning.releaseCalls)
	}
	if learning.releaseCtxErrs[0] != nil {
		t.Fatalf("la liberación debe llegar con un contexto vivo, llegó con: %v", learning.releaseCtxErrs[0])
	}
	if d := learning.releaseDeadline[0]; d <= 0 || d > releaseClaimTimeout {
		t.Fatalf("la liberación debe tener un tope de %s, tuvo %s", releaseClaimTimeout, d)
	}
	if learning.claimed {
		t.Fatal("el candado debe quedar libre")
	}
}

// El log de abstención dice cuál de las dos causas de learning trajo el 409.
func TestClaimConflictCause(t *testing.T) {
	cases := map[string]string{
		`{"code":"AI_REVIEW_CLAIM_ACTIVE"}`:     "candado activo (otra entrega o el propio candado sin liberar)",
		`{"code":"ATTEMPT_NOT_PENDING_REVIEW"}`: "el intento ya no está pendiente de revisión",
		"candado ajeno vigente":                 "desconocida",
	}
	for body, want := range cases {
		err := fmt.Errorf("%w: %s", m2m.ErrClaimConflict, body)
		if got := claimConflictCause(err); got != want {
			t.Fatalf("causa de %q: esperaba %q, hubo %q", body, want, got)
		}
	}
}

// cancellingProvider cancela el contexto del mensaje en la primera respuesta, como un
// apagado del worker a mitad de la revisión, y devuelve el error del contexto.
type cancellingProvider struct {
	mockLLMProvider
	cancel context.CancelFunc
}

func (c *cancellingProvider) ReviewAnswer(ctx context.Context, _ llm.ReviewRequest) (llm.ReviewResult, error) {
	c.calls++
	c.cancel()
	return llm.ReviewResult{}, ctx.Err()
}

// Con el mensaje cancelado a mitad, el bucle no sigue llamando al modelo por las que
// quedan: corta, devuelve error y libera con un contexto vivo.
func TestAttemptReview_CancelledMidReview_StopsAndReleases(t *testing.T) {
	learning := newStatefulLearning(
		openEndedWithText("a1", 10), openEndedWithText("a2", 10), openEndedWithText("a3", 10))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &cancellingProvider{cancel: cancel}
	p := newProcessor(directSettings(), learning, provider)

	err := p.Process(ctx, validEventPayload(t))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperaba un error de contexto cancelado, hubo: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("tras la cancelación no debe llamar más al modelo, hubo %d llamadas", provider.calls)
	}
	if learning.releaseCalls != 1 || learning.releaseCtxErrs[0] != nil || learning.claimed {
		t.Fatalf("debe liberar 1 vez con contexto vivo: releases=%d err=%v tomado=%v",
			learning.releaseCalls, learning.releaseCtxErrs, learning.claimed)
	}
	if learning.finalizeCalls != 0 {
		t.Fatalf("no debe finalizar, hubo %d", learning.finalizeCalls)
	}
}

// TestAttemptReview_ReviewOffReportsNotApplicable (HU-IA.6): con la revisión apagada el
// worker avisa «no aplica» del intento y sigue confirmando, también si el aviso falla.
func TestAttemptReview_ReviewOffReportsNotApplicable(t *testing.T) {
	for _, reportErr := range []error{nil, errors.New("learning caído")} {
		reader := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeOff)}
		learning := &mockLearningClient{}
		rep := &recordingReporter{err: reportErr}
		p := newProcessor(reader, learning, &mockLLMProvider{}).WithFailureReporter(rep)

		if err := p.Process(context.Background(), validEventPayload(t)); err != nil {
			t.Fatalf("mode=off confirma aunque el aviso falle (%v): %v", reportErr, err)
		}
		want := m2m.FailureReportRequest{TargetType: m2m.RescueTargetAttempt, TargetID: "attempt-1",
			Kind: m2m.RescueKindNotApplicable}
		if len(rep.reports) != 1 || rep.reports[0] != want {
			t.Fatalf("esperaba un aviso «no aplica» del intento, got %+v", rep.reports)
		}
		if learning.finalizeCalls != 0 || len(learning.reviewCalls) != 0 {
			t.Fatalf("mode=off no toca la revisión")
		}
	}

	// Con la revisión encendida, el corto-circuito no aplica y no hay aviso.
	reader := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	rep := &recordingReporter{}
	p := newProcessor(reader, &mockLearningClient{}, &mockLLMProvider{}).WithFailureReporter(rep)
	_ = p.Process(context.Background(), validEventPayload(t))
	if len(rep.reports) != 0 {
		t.Fatalf("con la revisión encendida no se avisa «no aplica»: %+v", rep.reports)
	}
}
