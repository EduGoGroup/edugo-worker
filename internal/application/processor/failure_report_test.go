package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// recordingReporter guarda cada aviso y puede fallar.
type recordingReporter struct {
	mu      sync.Mutex
	reports []m2m.FailureReportRequest
	err     error
	ctxErr  error
}

func (r *recordingReporter) Report(ctx context.Context, req m2m.FailureReportRequest) (m2m.FailureReportResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, req)
	r.ctxErr = ctx.Err()
	if r.err != nil {
		return m2m.FailureReportResponse{}, r.err
	}
	return m2m.FailureReportResponse{Status: "retrying"}, nil
}

// fixedProcessor devuelve siempre el mismo error.
type fixedProcessor struct{ err error }

func (p fixedProcessor) Process(context.Context, []byte) error { return p.err }

func reviewBody(attemptID string) []byte {
	b, _ := json.Marshal(map[string]any{
		"event_type": EventTypeAttemptReviewRequested,
		"payload":    map[string]any{"attempt_id": attemptID},
	})
	return b
}

func prepBody(questionID string) []byte {
	b, _ := json.Marshal(map[string]any{
		"event_type": events.EventTypeQuestionPrepRequested,
		"payload":    map[string]any{"question_id": questionID},
	})
	return b
}

func materialBody() []byte {
	b, _ := json.Marshal(map[string]any{
		"event_type": events.EventTypeMaterialAssessmentRequested,
		"payload":    map[string]any{"material_id": "m-1", "job_id": "j-1"},
	})
	return b
}

// TestFailureReport_Decision: el aviso solo sale cuando el mensaje va a morir, con la
// clase que corresponde, y nunca cambia el error devuelto.
func TestFailureReport_Decision(t *testing.T) {
	transient := errors.New("modelo caído: timeout")
	quality := fmt.Errorf("revisando answer a1: %w", llm.ErrLLMQuality)
	cases := []struct {
		name      string
		body      []byte
		err       error
		last      bool
		want      *m2m.FailureReportRequest
		reportErr error
	}{
		{name: "éxito no avisa", body: reviewBody("att-1"), err: nil, last: true},
		{name: "revisión permanente avisa data", body: reviewBody("att-1"), err: fmt.Errorf("%w: x", ErrMalformedEvent),
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetAttempt, TargetID: "att-1",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseData}},
		{name: "preparación inválida avisa data", body: prepBody("q-1"), err: fmt.Errorf("q-1: %w", ErrInvalidPrep),
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetQuestion, TargetID: "q-1",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseData}},
		{name: "revisión en el último reintento avisa service", body: reviewBody("att-2"), err: transient, last: true,
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetAttempt, TargetID: "att-2",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseService}},
		{name: "bucle de repetición en la revisión que muere es service", body: reviewBody("att-3"), err: quality, last: true,
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetAttempt, TargetID: "att-3",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseService}},
		{name: "preparación en el último reintento avisa service", body: prepBody("q-2"), err: transient, last: true,
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetQuestion, TargetID: "q-2",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseService}},
		{name: "reintento intermedio no avisa", body: reviewBody("att-4"), err: transient, last: false},
		{name: "material permanente no avisa", body: materialBody(), err: fmt.Errorf("%w: x", ErrMalformedEvent)},
		{name: "material en el último reintento no avisa", body: materialBody(), err: transient, last: true},
		{name: "JSON roto no avisa", body: []byte(`{"broken": `), err: fmt.Errorf("%w: decode", ErrUnroutableMessage)},
		{name: "tipo desconocido no avisa", body: []byte(`{"event_type":"x.y","payload":{"attempt_id":"att-9"}}`),
			err: fmt.Errorf("%w: no processor", ErrUnroutableMessage)},
		{name: "sin id no avisa", body: reviewBody(""), err: fmt.Errorf("%w: x", ErrMalformedEvent)},
		{name: "el aviso que falla no cambia el error", body: reviewBody("att-5"), err: transient, last: true,
			reportErr: errors.New("learning caído"),
			want: &m2m.FailureReportRequest{TargetType: m2m.RescueTargetAttempt, TargetID: "att-5",
				Kind: m2m.RescueKindFailure, CauseClass: m2m.RescueCauseService}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &recordingReporter{err: tc.reportErr}
			last := tc.last
			h := NewFailureReportingProcessor(fixedProcessor{err: tc.err}, rep, newTestLogger()).
				WithLastAttempt(func(context.Context) bool { return last })

			got := h.Process(context.Background(), tc.body)
			if got != tc.err { // el mismo valor, no un envoltorio
				t.Fatalf("el error devuelto cambió: got %v, want %v", got, tc.err)
			}
			if tc.want == nil {
				if len(rep.reports) != 0 {
					t.Fatalf("no debía avisar, avisó %+v", rep.reports)
				}
				return
			}
			if len(rep.reports) != 1 {
				t.Fatalf("esperaba un aviso, hubo %d", len(rep.reports))
			}
			r := rep.reports[0]
			if r.TargetType != tc.want.TargetType || r.TargetID != tc.want.TargetID ||
				r.Kind != tc.want.Kind || r.CauseClass != tc.want.CauseClass {
				t.Fatalf("aviso inesperado: got %+v, want %+v", r, *tc.want)
			}
			if r.Error != tc.err.Error() {
				t.Fatalf("el aviso lleva el texto del error: got %q", r.Error)
			}
		})
	}
}

// The report goes on a context that the message cancellation does not reach, and its
// text is capped.
func TestFailureReport_DetachedContextAndCappedText(t *testing.T) {
	rep := &recordingReporter{}
	long := errors.New(strings.Repeat("ñ", 2000))
	h := NewFailureReportingProcessor(fixedProcessor{err: long}, rep, newTestLogger()).
		WithLastAttempt(func(context.Context) bool { return true })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = h.Process(ctx, reviewBody("att-1"))
	if len(rep.reports) != 1 {
		t.Fatalf("esperaba un aviso, hubo %d", len(rep.reports))
	}
	if rep.ctxErr != nil {
		t.Fatalf("el aviso no debe heredar la cancelación del mensaje: %v", rep.ctxErr)
	}
	if n := len(rep.reports[0].Error); n > maxReportErrorBytes {
		t.Fatalf("el texto del error pasa del tope: %d bytes", n)
	}
	if !strings.HasPrefix(long.Error(), rep.reports[0].Error) {
		t.Fatalf("el texto se corta en frontera de runa")
	}
}

// Without reporter the decorator only passes the error through.
func TestFailureReport_NilReporter(t *testing.T) {
	want := errors.New("x")
	h := NewFailureReportingProcessor(fixedProcessor{err: want}, nil, newTestLogger()).
		WithLastAttempt(func(context.Context) bool { return true })
	if got := h.Process(context.Background(), reviewBody("att-1")); got != want {
		t.Fatalf("got %v", got)
	}
}
