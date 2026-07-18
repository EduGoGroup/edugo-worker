package processor

import (
	"context"
	"encoding/json"
	"errors"
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

// mockPrepProvider produce un JSON crudo fijo (o error) para PrepareQuestion.
type mockPrepProvider struct {
	raw json.RawMessage
	err error
}

func (m *mockPrepProvider) GenerateAssessment(_ context.Context, _ llm.MaterialInput, _ llm.GenerationParams) (json.RawMessage, error) {
	return nil, errors.New("no usado")
}
func (m *mockPrepProvider) ReviewAnswer(_ context.Context, _ llm.ReviewRequest) (llm.ReviewResult, error) {
	return llm.ReviewResult{}, errors.New("no usado")
}
func (m *mockPrepProvider) PrepareQuestion(_ context.Context, _ llm.PrepRequest) (json.RawMessage, error) {
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

func TestQuestionPrep_ModeOff_Acks(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeOff)}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(validListPrep)})

	if err := p.Process(context.Background(), prepEvent("q1", "a1", "created")); err != nil {
		t.Fatalf("esperaba ACK sin error, got: %v", err)
	}
	if learning.saveCalls != 0 {
		t.Fatalf("mode=off no debe persistir; saveCalls=%d", learning.saveCalls)
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

func TestQuestionPrep_InvalidPrep_TransientNoPut(t *testing.T) {
	settings := &mockSettingsReader{settings: settingsWith(settingKeyReviewMode, reviewModeLocal)}
	learning := &mockPrepLearning{source: prepSource(llm.QuestionTypeShortAnswer, "h1")}
	// content_kind=list sin items → inválido contra el contrato.
	bad := `{"version":1,"question_type":"short_answer","content_kind":"list","items":[],"items_verbatim":[]}`
	p := newPrepProcessor(settings, learning, &mockPrepProvider{raw: json.RawMessage(bad)})

	err := p.Process(context.Background(), prepEvent("q1", "a1", "created"))
	if err == nil {
		t.Fatal("un prep inválido debe fallar (transitorio)")
	}
	if !errors.Is(err, ErrInvalidPrep) {
		t.Fatalf("esperaba ErrInvalidPrep, got: %v", err)
	}
	if errors.Is(err, ErrMalformedEvent) {
		t.Fatal("prep inválido NO debe ser permanente (reintentar puede dar un prep bueno)")
	}
	if learning.saveCalls != 0 {
		t.Fatalf("prep inválido NUNCA se persiste; saveCalls=%d", learning.saveCalls)
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
