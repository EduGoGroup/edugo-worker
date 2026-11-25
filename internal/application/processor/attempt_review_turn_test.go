package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/llm/inferencecap"
)

const turnTestWait = 5 * time.Second

// fixedSettings is a stateless settings double, safe for concurrent messages.
type fixedSettings struct{ settings m2m.SchoolSettings }

func (f fixedSettings) GetSettings(context.Context, string) (m2m.SchoolSettings, error) {
	return f.settings, nil
}

// turnLearning is a concurrency-safe learning double: answers by attempt, and one
// configurable failure per step.
type turnLearning struct {
	mu          sync.Mutex
	answers     map[string][]m2m.PendingAnswer
	claimErr    error
	pendingErr  error
	finalizeErr error

	claims    atomic.Int32
	finalizes atomic.Int32
}

func (l *turnLearning) Claim(context.Context, string) error {
	l.claims.Add(1)
	return l.claimErr
}

func (l *turnLearning) ReleaseClaim(context.Context, string) error { return nil }

func (l *turnLearning) GetPendingAnswers(_ context.Context, attemptID string) (m2m.PendingAnswersResponse, error) {
	if l.pendingErr != nil {
		return m2m.PendingAnswersResponse{}, l.pendingErr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return m2m.PendingAnswersResponse{AttemptID: attemptID, Answers: l.answers[attemptID]}, nil
}

func (l *turnLearning) PostAnswerReview(_ context.Context, _ string, answerID string, _ m2m.AnswerReviewRequest) (m2m.AnswerReviewResponse, error) {
	return m2m.AnswerReviewResponse{AnswerID: answerID, ReviewStatus: "reviewed"}, nil
}

func (l *turnLearning) FinalizeAttempt(_ context.Context, attemptID string) (m2m.FinalizeResponse, error) {
	l.finalizes.Add(1)
	if l.finalizeErr != nil {
		return m2m.FinalizeResponse{}, l.finalizeErr
	}
	return m2m.FinalizeResponse{AttemptID: attemptID, Status: "completed"}, nil
}

// reviewEventFor builds the open_ended review event of one attempt.
func reviewEventFor(t *testing.T, attemptID string) []byte {
	t.Helper()
	b, err := json.Marshal(events.AttemptReviewRequestedEvent{
		EventID:      "evt-" + attemptID,
		EventType:    EventTypeAttemptReviewRequested,
		EventVersion: "1.0",
		Payload: events.AttemptReviewRequestedPayload{
			AttemptID:    attemptID,
			AssessmentID: "assessment-1",
			SchoolID:     "school-1",
			Answers:      []events.AttemptReviewAnswerRef{{AnswerID: "a1", QuestionType: "open_ended"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal evento: %v", err)
	}
	return b
}

// turnProcessor builds the real processor in mode local, flow direct, with one limiter.
func turnProcessor(learning LearningReviewClient, provider llm.LLMProvider, mode string, limiter *inferencecap.Limiter) *AttemptReviewProcessor {
	settings := fixedSettings{settingsWith(settingKeyReviewMode, mode, settingKeyReviewFlow, reviewFlowDirect)}
	return NewAttemptReviewProcessor(settings, learning, map[string]llm.LLMProvider{"local": provider}, newTestLogger()).
		WithInferenceTurns(map[string]*inferencecap.Limiter{"local": limiter})
}

// waitTurnQueue blocks until n callers wait for the limiter.
func waitTurnQueue(t *testing.T, limiter *inferencecap.Limiter, n int) {
	t.Helper()
	deadline := time.Now().Add(turnTestWait)
	for limiter.Waiting() != n {
		if time.Now().After(deadline) {
			t.Fatalf("la fila del turno no llegó a %d (tiene %d)", n, limiter.Waiting())
		}
		time.Sleep(time.Millisecond)
	}
}

// requireFreeTurn fails unless the limiter grants a turn at once.
func requireFreeTurn(t *testing.T, limiter *inferencecap.Limiter, when string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), turnTestWait)
	defer cancel()
	release, err := limiter.Acquire(ctx)
	if err != nil {
		t.Fatalf("%s: el turno no quedó libre: %v", when, err)
	}
	release()
}

// CA-IA.14, fourth sentence: a review does not take the lock until it has its turn.
func TestReview_NoClaimUntilTurn(t *testing.T) {
	limiter := inferencecap.New("local", 1, nil)
	learning := &turnLearning{answers: map[string][]m2m.PendingAnswer{"attempt-1": {pendingAnswer("a1", 10)}}}
	provider := &mockLLMProvider{score: 1, verdict: llm.VerdictCorrect, feedback: "ok"}
	p := turnProcessor(learning, provider, reviewModeLocal, limiter)

	busy, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.Process(context.Background(), reviewEventFor(t, "attempt-1")) }()

	waitTurnQueue(t, limiter, 1)
	if got := learning.claims.Load(); got != 0 {
		t.Fatalf("Claim se llamó %d veces esperando turno", got)
	}

	busy()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
	case <-time.After(turnTestWait):
		t.Fatal("Process no terminó al soltar el turno")
	}
	if learning.claims.Load() != 1 || learning.finalizes.Load() != 1 || provider.calls != 1 {
		t.Fatalf("con turno: claims=%d finalizes=%d llamadas=%d",
			learning.claims.Load(), learning.finalizes.Load(), provider.calls)
	}
	requireFreeTurn(t, limiter, "tras el finalize")
}

func TestReview_TurnReleasedOnEveryExit(t *testing.T) {
	answers := map[string][]m2m.PendingAnswer{"attempt-1": {pendingAnswer("a1", 10)}}
	okProvider := func() *mockLLMProvider {
		return &mockLLMProvider{score: 1, verdict: llm.VerdictCorrect, feedback: "ok"}
	}
	cases := []struct {
		name     string
		learning *turnLearning
		provider *mockLLMProvider
		wantErr  bool
	}{
		{"409 del candado", &turnLearning{answers: answers,
			claimErr: fmt.Errorf(`%w: {"code":"AI_REVIEW_CLAIM_ACTIVE"}`, m2m.ErrClaimConflict)}, okProvider(), false},
		{"error del modelo", &turnLearning{answers: answers},
			&mockLLMProvider{failOnCall: 1, err: errors.New("modelo caído")}, true},
		{"fallo de learning", &turnLearning{answers: answers, pendingErr: errors.New("learning caído")}, okProvider(), true},
		{"fallo de finalize", &turnLearning{answers: answers, finalizeErr: errors.New("finalize caído")}, okProvider(), true},
		{"finalize correcto", &turnLearning{answers: answers}, okProvider(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := inferencecap.New("local", 1, nil)
			p := turnProcessor(tc.learning, tc.provider, reviewModeLocal, limiter)
			err := p.Process(context.Background(), reviewEventFor(t, "attempt-1"))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v, se esperaba error=%v", err, tc.wantErr)
			}
			requireFreeTurn(t, limiter, tc.name)
		})
	}
}

func TestReview_CancelledWhileWaiting(t *testing.T) {
	limiter := inferencecap.New("local", 1, nil)
	learning := &turnLearning{answers: map[string][]m2m.PendingAnswer{"attempt-1": {pendingAnswer("a1", 10)}}}
	p := turnProcessor(learning, &mockLLMProvider{verdict: llm.VerdictCorrect}, reviewModeLocal, limiter)

	busy, _ := limiter.Acquire(context.Background())
	defer busy()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Process(ctx, reviewEventFor(t, "attempt-1")) }()
	waitTurnQueue(t, limiter, 1)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("se esperaba el error del contexto, llegó %v", err)
	}
	if IsPermanent(err) {
		t.Fatalf("la espera cancelada debe ser transitoria: %v", err)
	}
	if learning.claims.Load() != 0 {
		t.Fatalf("Claim se llamó %d veces sin turno", learning.claims.Load())
	}
	waitTurnQueue(t, limiter, 0)
}

func TestReview_OffSchoolNeverWaits(t *testing.T) {
	limiter := inferencecap.New("local", 1, nil)
	learning := &turnLearning{}
	p := turnProcessor(learning, &mockLLMProvider{}, reviewModeOff, limiter)

	busy, _ := limiter.Acquire(context.Background())
	defer busy()
	ctx, cancel := context.WithTimeout(context.Background(), turnTestWait)
	defer cancel()
	if err := p.Process(ctx, reviewEventFor(t, "attempt-1")); err != nil {
		t.Fatalf("con la revisión apagada se confirma sin pedir turno: %v", err)
	}
	if limiter.Waiting() != 0 || learning.claims.Load() != 0 {
		t.Fatalf("revisión apagada: en fila=%d claims=%d", limiter.Waiting(), learning.claims.Load())
	}
}
