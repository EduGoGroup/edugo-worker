package m2m

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// rescueServer responde con status y body fijos y guarda la última petición.
func rescueServer(t *testing.T, status int, body string, seen *map[string]any, path *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("falta el bearer: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type inesperado: %q", r.Header.Get("Content-Type"))
		}
		raw, _ := io.ReadAll(r.Body)
		if seen != nil {
			_ = json.Unmarshal(raw, seen)
		}
		if path != nil {
			*path = r.URL.Path
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func newRescueClient(url string) *LearningRescueClient {
	return NewLearningRescueClient(LearningRescueClientConfig{BaseURL: url, TokenProvider: staticToken{"tok"}})
}

func TestLearningRescueClient_ReportFailure(t *testing.T) {
	var seen map[string]any
	var path string
	srv := rescueServer(t, http.StatusOK, `{"status":"retrying"}`, &seen, &path)
	defer srv.Close()

	out, err := newRescueClient(srv.URL).Report(context.Background(), FailureReportRequest{
		TargetType: RescueTargetAttempt, TargetID: "att-1", Kind: RescueKindFailure,
		CauseClass: RescueCauseService, Error: "timeout",
	})
	if err != nil {
		t.Fatalf("Report falló: %v", err)
	}
	if out.Status != "retrying" {
		t.Fatalf("status mal mapeado: %+v", out)
	}
	if path != "/api/v1/internal/ai-failures/report" {
		t.Fatalf("path inesperado: %s", path)
	}
	want := map[string]any{"target_type": "attempt", "target_id": "att-1", "kind": "failure",
		"cause_class": "service", "error": "timeout"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("cuerpo inesperado: got %v, want %v", seen, want)
	}
}

// En not_applicable la clase y el error se omiten del cuerpo.
func TestLearningRescueClient_ReportNotApplicable(t *testing.T) {
	var seen map[string]any
	srv := rescueServer(t, http.StatusOK, `{"status":"resolved"}`, &seen, nil)
	defer srv.Close()

	out, err := newRescueClient(srv.URL).Report(context.Background(), FailureReportRequest{
		TargetType: RescueTargetQuestion, TargetID: "q-1", Kind: RescueKindNotApplicable,
	})
	if err != nil || out.Status != "resolved" {
		t.Fatalf("got %+v, %v", out, err)
	}
	want := map[string]any{"target_type": "question", "target_id": "q-1", "kind": "not_applicable"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("cuerpo inesperado: got %v, want %v", seen, want)
	}
}

func TestLearningRescueClient_Sweep(t *testing.T) {
	var seen map[string]any
	var path string
	srv := rescueServer(t, http.StatusOK,
		`{"resolved":1,"exhausted":2,"republished":3,"orphans":4,"jobs_failed":5}`, &seen, &path)
	defer srv.Close()

	out, err := newRescueClient(srv.URL).Sweep(context.Background(), SweepRequest{
		MaxRetries: 5, RetryBackoffSeconds: []int64{120, 600, 1800, 7200, 21600},
		MinAgeSeconds: 900, JobPendingStaleSeconds: 21600,
	})
	if err != nil {
		t.Fatalf("Sweep falló: %v", err)
	}
	if out != (SweepResponse{Resolved: 1, Exhausted: 2, Republished: 3, Orphans: 4, JobsFailed: 5}) {
		t.Fatalf("conteos mal mapeados: %+v", out)
	}
	if path != "/api/v1/internal/ai-failures/sweep" {
		t.Fatalf("path inesperado: %s", path)
	}
	want := map[string]any{"max_retries": float64(5),
		"retry_backoff_seconds": []any{float64(120), float64(600), float64(1800), float64(7200), float64(21600)},
		"min_age_seconds":       float64(900), "job_pending_stale_seconds": float64(21600)}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("cuerpo inesperado: got %v, want %v", seen, want)
	}
}

// 404 y otros 4xx son permanentes; 5xx, 408 y 429, transitorios.
func TestLearningRescueClient_StatusClassification(t *testing.T) {
	cases := []struct {
		status    int
		permanent bool
		notFound  bool
	}{
		{http.StatusNotFound, true, true},
		{http.StatusBadRequest, true, false},
		{http.StatusForbidden, true, false},
		{http.StatusRequestTimeout, false, false},
		{http.StatusTooManyRequests, false, false},
		{http.StatusInternalServerError, false, false},
		{http.StatusServiceUnavailable, false, false},
	}
	for _, tc := range cases {
		srv := rescueServer(t, tc.status, `{"error":"x"}`, nil, nil)
		c := newRescueClient(srv.URL)
		_, reportErr := c.Report(context.Background(), FailureReportRequest{
			TargetType: RescueTargetAttempt, TargetID: "att-1", Kind: RescueKindFailure, CauseClass: RescueCauseData,
		})
		_, sweepErr := c.Sweep(context.Background(), SweepRequest{MinAgeSeconds: 900})
		srv.Close()
		for name, err := range map[string]error{"report": reportErr, "sweep": sweepErr} {
			if err == nil {
				t.Fatalf("%s %d: esperaba error", name, tc.status)
			}
			if got := errors.Is(err, ErrLearningPermanent); got != tc.permanent {
				t.Fatalf("%s %d: permanente=%v, want %v (%v)", name, tc.status, got, tc.permanent, err)
			}
			if got := errors.Is(err, ErrRescueTargetNotFound); got != tc.notFound {
				t.Fatalf("%s %d: not found=%v, want %v", name, tc.status, got, tc.notFound)
			}
		}
	}
}

func TestLearningRescueClient_NetworkErrorIsTransient(t *testing.T) {
	srv := rescueServer(t, http.StatusOK, `{}`, nil, nil)
	url := srv.URL
	srv.Close()
	_, err := newRescueClient(url).Sweep(context.Background(), SweepRequest{})
	if err == nil || errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("un fallo de red es transitorio: %v", err)
	}
}
