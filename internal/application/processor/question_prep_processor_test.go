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
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// --- mocks del carril de preparación ---

type mockPrepLearning struct {
	source    m2m.PrepSourceResponse
	sourceErr error
	savedReq  *m2m.SavePrepRequest
	saveErr   error
	getCalls  int
	saveCalls int
	savedQIDs []string
	// outcome* registran SetPrepOutcome (HU-IA.4).
	outcomes   []m2m.SetPrepStatusRequest
	outcomeErr error
}

func (m *mockPrepLearning) SetPrepOutcome(_ context.Context, _ string, req m2m.SetPrepStatusRequest) (m2m.SetPrepStatusResponse, error) {
	m.outcomes = append(m.outcomes, req)
	if m.outcomeErr != nil {
		return m2m.SetPrepStatusResponse{}, m.outcomeErr
	}
	return m2m.SetPrepStatusResponse{Status: req.Status, Applied: true}, nil
}

func (m *mockPrepLearning) GetPrepSource(_ context.Context, _ string) (m2m.PrepSourceResponse, error) {
	m.getCalls++
	if m.sourceErr != nil {
		return m2m.PrepSourceResponse{}, m.sourceErr
	}
	return m.source, nil
}

func (m *mockPrepLearning) SavePrep(_ context.Context, questionID string, req m2m.SavePrepRequest) error {
	m.saveCalls++
	m.savedQIDs = append(m.savedQIDs, questionID)
	if m.saveErr != nil {
		return m.saveErr
	}
	r := req
	m.savedReq = &r
	return nil
}

// mockPrepProvider produce un JSON crudo fijo (o error) para PrepareQuestion. Con
// seq, cada llamada toma su salida en orden (la última se repite). Cuenta las
// llamadas y guarda cada petición (para la temperatura del reintento).
type mockPrepProvider struct {
	raw  json.RawMessage
	err  error
	seq  []prepOut
	reqs []llm.PrepRequest
}

type prepOut struct {
	raw string
	err error
}

func (m *mockPrepProvider) GenerateAssessment(_ context.Context, _ llm.MaterialInput, _ llm.GenerationParams) (json.RawMessage, error) {
	return nil, errors.New("no usado")
}
func (m *mockPrepProvider) ReviewAnswer(_ context.Context, _ llm.ReviewRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, errors.New("no usado")
}
func (m *mockPrepProvider) PrepareQuestion(_ context.Context, req llm.PrepRequest) (json.RawMessage, error) {
	m.reqs = append(m.reqs, req)
	if len(m.seq) > 0 {
		o := m.seq[min(len(m.reqs), len(m.seq))-1]
		return json.RawMessage(o.raw), o.err
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.raw, nil
}
func (m *mockPrepProvider) JudgePairEquivalence(_ context.Context, _ llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, errors.New("no usado")
}
func (m *mockPrepProvider) CheckCriterion(_ context.Context, _ llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, errors.New("no usado")
}
func (m *mockPrepProvider) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	return nil, errors.New("no usado")
}
func (m *mockPrepProvider) DigestChunk(_ context.Context, _ llm.DigestChunkInput) (*llm.DigestChunkResult, error) {
	return nil, nil
}

func (m *mockPrepProvider) ProposeCandidates(_ context.Context, _ llm.ProposeCandidatesInput) ([]materialpipeline.CandidatePayloadV1, error) {
	return nil, nil
}

func (m *mockPrepProvider) Name() string { return "mock-prep" }

// --- helpers ---

func prepEvent(qid, aid, reason string) []byte {
	evt := events.QuestionPrepRequestedEvent{
		EventType: events.EventTypeQuestionPrepRequested,
		Payload: events.QuestionPrepRequestedPayload{
			QuestionID:   qid,
			AssessmentID: aid,
			Reason:       reason,
		},
	}
	b, _ := json.Marshal(evt)
	return b
}

func prepSource(qtype, hash string) m2m.PrepSourceResponse {
	correct := "Ecuador, Venezuela y Colombia"
	return m2m.PrepSourceResponse{
		QuestionID:    "q1",
		AssessmentID:  "a1",
		SchoolID:      "s1",
		QuestionType:  qtype,
		QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
		CorrectAnswer: &correct,
		SourceHash:    hash,
	}
}

const validListPrep = `{"version":1,"question_type":"short_answer","content_kind":"list",` +
	`"items":["ecuador","venezuela","colombia"],"items_verbatim":["Ecuador","Venezuela","Colombia"],"unit":null}`

func newPrepProcessor(settings SchoolSettingsReader, learning LearningPrepClient, provider llm.LLMProvider) *QuestionPrepProcessor {
	providers := map[string]llm.LLMProvider{}
	if provider != nil {
		providers["local"] = provider
	}
	return NewQuestionPrepProcessor(settings, learning, providers, newTestLogger())
}

// --- tests ---

// TestQuestionPrep_ModeOff_WritesNone (invierte _ModeOff_Acks, HU-IA.4): con la IA
// apagada confirma, no llama al modelo y escribe none; si esa escritura falla, el
// error sale (se reintenta).
func TestQuestionPrep_ModeOff_WritesNone(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeOff)}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	prov := &mockPrepProvider{raw: json.RawMessage(validListPrep)}
	p := newPrepProcessor(settings, learning, prov)

	if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err != nil {
		t.Fatalf("esperaba ACK sin error, got: %v", err)
	}
	if learning.saveCalls != 0 || len(prov.reqs) != 0 {
		t.Fatalf("mode=off no llama ni persiste; save=%d llamadas=%d", learning.saveCalls, len(prov.reqs))
	}
	if len(learning.outcomes) != 1 || learning.outcomes[0] != (m2m.SetPrepStatusRequest{Status: m2m.PrepStatusNone, SourceHash: "h1"}) {
		t.Fatalf("esperaba none con h1, got %+v", learning.outcomes)
	}

	learning.outcomeErr = errors.New("learning caído")
	if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err == nil || IsPermanent(err) {
		t.Fatalf("un fallo al escribir none debe salir transitorio, got %v", err)
	}
}

func TestQuestionPrep_HappyPath_PersistsWithHash(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err != nil {
		t.Fatalf("esperaba éxito, got: %v", err)
	}
	if learning.savedReq == nil {
		t.Fatal("esperaba un PUT llm-prep")
	}
	if learning.savedReq.SourceHash != "h1" {
		t.Fatalf("source_hash del PUT %q != h1 (debe ser el que trabajó)", learning.savedReq.SourceHash)
	}
	if learning.savedReq.ConsumedFeedback {
		t.Fatal("sin feedback pendiente, consumed_feedback debe ser false")
	}
}

func TestQuestionPrep_FeedbackConsumed(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	src := prepSource(llm.QuestionTypeShortAnswer, "h1")
	fb := "Faltó Panamá"
	src.LLMPrepFeedback = &fb
	learning := &mockPrepLearning{source: src}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	if err := p.Process(context.Background(), prepEvent("q1", "a1", "feedback")); err != nil {
		t.Fatalf("esperaba éxito, got: %v", err)
	}
	if learning.savedReq == nil || !learning.savedReq.ConsumedFeedback {
		t.Fatal("con feedback pendiente, consumed_feedback debe ser true")
	}
}

func TestQuestionPrep_HashConflict_Acks(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	learning := &mockPrepLearning{
		source:  prepSource(llm.QuestionTypeShortAnswer, "h1"),
		saveErr: m2m.ErrPrepHashConflict,
	}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	if err := p.Process(context.Background(), prepEvent("q1", "a1", "updated")); err != nil {
		t.Fatalf("un 409 debe ACKear sin error, got: %v", err)
	}
}

const badListPrep = `{"version":1,"question_type":"short_answer","content_kind":"list","items":[],"items_verbatim":[]}`

func localPrep(learning *mockPrepLearning, prov *mockPrepProvider) *QuestionPrepProcessor {
	return newPrepProcessor(&mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}, learning, prov)
}

// TestQuestionPrep_InvalidPrep_PermanentMarksFailed (invierte _TransientNoPut): dos
// llamadas, la segunda a 0,3; ningún PUT del artefacto, failed con el hash leído, y
// el error es ErrInvalidPrep y permanente.
func TestQuestionPrep_InvalidPrep_PermanentMarksFailed(t *testing.T) {
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	prov := &mockPrepProvider{raw: json.RawMessage(badListPrep)}

	err := localPrep(learning, prov).Process(context.Background(), prepEvent("q1", "a1", "created"))
	if !errors.Is(err, ErrInvalidPrep) || !IsPermanent(err) {
		t.Fatalf("esperaba ErrInvalidPrep permanente, got: %v", err)
	}
	assertRetriedAtLowTemp(t, prov)
	if learning.saveCalls != 0 {
		t.Fatalf("prep inválido NUNCA se persiste; saveCalls=%d", learning.saveCalls)
	}
	if len(learning.outcomes) != 1 || learning.outcomes[0] != (m2m.SetPrepStatusRequest{Status: m2m.PrepStatusFailed, SourceHash: "h1"}) {
		t.Fatalf("esperaba failed con h1, got %+v", learning.outcomes)
	}
}

func assertRetriedAtLowTemp(t *testing.T, prov *mockPrepProvider) {
	t.Helper()
	if len(prov.reqs) != 2 {
		t.Fatalf("esperaba 2 llamadas al modelo, got %d", len(prov.reqs))
	}
	if prov.reqs[0].Temperature != nil {
		t.Fatalf("la primera llamada va sin temperatura, got %v", *prov.reqs[0].Temperature)
	}
	if prov.reqs[1].Temperature == nil || *prov.reqs[1].Temperature != llm.InvalidOutputRetryTemperature {
		t.Fatalf("el reintento va a %v", llm.InvalidOutputRetryTemperature)
	}
}

// TestQuestionPrep_InvalidThenValid_Persists: inválido y luego válido: dos llamadas,
// ready, sin failed.
func TestQuestionPrep_InvalidThenValid_Persists(t *testing.T) {
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	prov := &mockPrepProvider{seq: []prepOut{{raw: badListPrep}, {raw: validListPrep}}}

	if err := localPrep(learning, prov).Process(context.Background(), prepEvent("q1", "a1", "created")); err != nil {
		t.Fatalf("esperaba éxito, got: %v", err)
	}
	assertRetriedAtLowTemp(t, prov)
	if learning.saveCalls != 1 || len(learning.outcomes) != 0 {
		t.Fatalf("save=%d outcomes=%+v", learning.saveCalls, learning.outcomes)
	}
}

// TestQuestionPrep_QualityError_CountsAsInvalid: un error que envuelve
// llm.ErrLLMQuality (el bucle de repetición) sigue el camino del inválido.
func TestQuestionPrep_QualityError_CountsAsInvalid(t *testing.T) {
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	prov := &mockPrepProvider{err: fmt.Errorf("ollama: bucle de repetición: %w", llm.ErrLLMQuality)}

	err := localPrep(learning, prov).Process(context.Background(), prepEvent("q1", "a1", "created"))
	if !errors.Is(err, ErrInvalidPrep) || !IsPermanent(err) {
		t.Fatalf("esperaba ErrInvalidPrep permanente, got: %v", err)
	}
	assertRetriedAtLowTemp(t, prov)
	if len(learning.outcomes) != 1 || learning.outcomes[0].Status != m2m.PrepStatusFailed {
		t.Fatalf("esperaba failed, got %+v", learning.outcomes)
	}
}

// TestQuestionPrep_AlreadyReady_SkipsModel: cero llamadas repetidas. Con ready, ni
// llamada ni escritura, se confirma y queda la línea del salto; también apagada.
func TestQuestionPrep_AlreadyReady_SkipsModel(t *testing.T) {
	for _, mode := range []string{reviewModeLocal, reviewModeOff} {
		src := prepSource(llm.QuestionTypeShortAnswer, "h1")
		src.LLMPrepStatus = m2m.PrepStatusReady
		learning := &mockPrepLearning{source: src}
		prov := &mockPrepProvider{raw: json.RawMessage(validListPrep)}
		log := newCaptureLogger()
		p := NewQuestionPrepProcessor(&mockSettingsReader{settings: settingsWith(settingKeyReviewMode, mode)},
			learning, map[string]llm.LLMProvider{"local": prov}, log)

		if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err != nil {
			t.Fatalf("[%s] esperaba ACK, got %v", mode, err)
		}
		if len(prov.reqs) != 0 || learning.saveCalls != 0 || len(learning.outcomes) != 0 {
			t.Fatalf("[%s] llamadas=%d save=%d outcomes=%d", mode, len(prov.reqs), learning.saveCalls, len(learning.outcomes))
		}
		if len(log.byMsg("preparación ya lista, no se repite")) != 1 {
			t.Fatalf("[%s] falta la línea del salto", mode)
		}
	}
}

// TestQuestionPrep_PendingOrFailed_Runs: una pending y una failed con el mismo hash
// se ejecutan (gancho 2 de la HU-IA.6).
func TestQuestionPrep_PendingOrFailed_Runs(t *testing.T) {
	for _, st := range []string{"pending", m2m.PrepStatusFailed} {
		src := prepSource(llm.QuestionTypeShortAnswer, "h1")
		src.LLMPrepStatus = st
		learning := &mockPrepLearning{source: src}
		prov := &mockPrepProvider{raw: json.RawMessage(validListPrep)}
		if err := localPrep(learning, prov).Process(context.Background(), prepEvent("q1", "a1", "republished")); err != nil {
			t.Fatalf("[%s] %v", st, err)
		}
		if len(prov.reqs) != 1 || learning.saveCalls != 1 {
			t.Fatalf("[%s] llamadas=%d save=%d", st, len(prov.reqs), learning.saveCalls)
		}
	}
}

// TestQuestionPrep_ServiceFailure_LeavesPending: modelo caído, tiempo agotado o 5xx
// de learning: una llamada, ninguna escritura de estado, error transitorio.
func TestQuestionPrep_ServiceFailure_LeavesPending(t *testing.T) {
	cases := map[string]struct {
		prov    *mockPrepProvider
		saveErr error
	}{
		"modelo_caido":   {prov: &mockPrepProvider{err: errors.New("connection refused")}},
		"tiempo_agotado": {prov: &mockPrepProvider{err: context.DeadlineExceeded}},
		"learning_5xx":   {prov: &mockPrepProvider{raw: json.RawMessage(validListPrep)}, saveErr: errors.New("save-prep returned status 503")},
	}
	for name, c := range cases {
		learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1"), saveErr: c.saveErr}
		err := localPrep(learning, c.prov).Process(context.Background(), prepEvent("q1", "a1", "created"))
		if err == nil || IsPermanent(err) {
			t.Fatalf("[%s] esperaba transitorio, got %v", name, err)
		}
		if len(c.prov.reqs) != 1 || len(learning.outcomes) != 0 {
			t.Fatalf("[%s] llamadas=%d outcomes=%+v", name, len(c.prov.reqs), learning.outcomes)
		}
	}
}

// TestQuestionPrep_MarkFailedError_KeepsOriginalError: si escribir failed falla, sale
// el ErrInvalidPrep original (la pregunta queda pending para el barrido).
func TestQuestionPrep_MarkFailedError_KeepsOriginalError(t *testing.T) {
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1"), outcomeErr: errors.New("learning caído")}
	err := localPrep(learning, &mockPrepProvider{raw: json.RawMessage(badListPrep)}).
		Process(context.Background(), prepEvent("q1", "a1", "created"))
	if !errors.Is(err, ErrInvalidPrep) || !IsPermanent(err) || len(learning.outcomes) != 1 {
		t.Fatalf("err=%v outcomes=%d", err, len(learning.outcomes))
	}
}

// TestQuestionPrep_NoMarkWithoutSource: evento malformado o prep-source caído: sin
// hash, no se escribe.
func TestQuestionPrep_NoMarkWithoutSource(t *testing.T) {
	for _, srcErr := range []error{fmt.Errorf("%w: 404", m2m.ErrLearningPermanent), errors.New("learning returned status 503")} {
		learning := &mockPrepLearning{sourceErr: srcErr}
		p := localPrep(learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})
		if err := p.Process(context.Background(), []byte(`{"event_type":"question.prep_requested","payload":{}}`)); err == nil {
			t.Fatal("evento malformado debe fallar")
		}
		if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err == nil {
			t.Fatalf("prep-source caído (%v) debe fallar", srcErr)
		}
		if len(learning.outcomes) != 0 {
			t.Fatalf("sin fuente no se escribe, got %+v", learning.outcomes)
		}
	}
}

func TestQuestionPrep_MalformedEvent_Permanent(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	err := p.Process(context.Background(), []byte(`{"event_type":"question.prep_requested","payload":{}}`))
	if err == nil || !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("evento sin question_id debe ser permanente (ErrMalformedEvent), got: %v", err)
	}
	if learning.getCalls != 0 {
		t.Fatal("evento malformado no debe llegar a M2M")
	}
}

func TestQuestionPrep_ModeWithoutProvider_Permanent(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, "api")}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	// provider solo tiene "local"; mode=api no tiene provider.
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	err := p.Process(context.Background(), prepEvent("q1", "a1", "created"))
	if err == nil || !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("mode sin provider debe ser permanente, got: %v", err)
	}
	// Proveedor sin configurar es fallo de dato: queda failed (HU-IA.4).
	if len(learning.outcomes) != 1 || learning.outcomes[0].Status != m2m.PrepStatusFailed {
		t.Fatalf("esperaba failed, got %+v", learning.outcomes)
	}
}

func TestQuestionPrep_PrepSourceError_Transient(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	learning := &mockPrepLearning{sourceErr: errors.New("academic caído")}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	err := p.Process(context.Background(), prepEvent("q1", "a1", "created"))
	if err == nil {
		t.Fatal("un fallo de prep-source debe propagarse (transitorio)")
	}
	if errors.Is(err, ErrMalformedEvent) {
		t.Fatal("un fallo de red no debe ser permanente")
	}
}
