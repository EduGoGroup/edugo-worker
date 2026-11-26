package m2m

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// staticToken (definido en m2m_test.go) provee un token fijo; su campo es `v`.

func TestLearningPrepClient_GetPrepSource_OK(t *testing.T) {
	correct := "Ecuador, Venezuela y Colombia"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/v1/internal/questions/q1/prep-source") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("falta el bearer: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(PrepSourceResponse{
			QuestionID:    "q1",
			AssessmentID:  "a1",
			SchoolID:      "s1",
			QuestionType:  "short_answer",
			QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
			CorrectAnswer: &correct,
			SourceHash:    "hash-1",
		})
	}))
	defer srv.Close()

	c := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.GetPrepSource(context.Background(), "q1")
	if err != nil {
		t.Fatalf("GetPrepSource falló: %v", err)
	}
	if got.SchoolID != "s1" || got.SourceHash != "hash-1" || got.CorrectAnswer == nil {
		t.Fatalf("respuesta mal mapeada: %+v", got)
	}
}

func TestLearningPrepClient_GetPrepSource_404Permanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	c := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	_, err := c.GetPrepSource(context.Background(), "q1")
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("esperaba ErrLearningPermanent, got: %v", err)
	}
}

func TestLearningPrepClient_SavePrep_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("método inesperado: %s", r.Method)
		}
		var body SavePrepRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.SourceHash != "hash-1" || !body.ConsumedFeedback {
			t.Errorf("body inesperado: %+v", body)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}))
	defer srv.Close()

	c := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SavePrep(context.Background(), "q1", SavePrepRequest{
		LLMPrep:          json.RawMessage(`{"version":1}`),
		SourceHash:       "hash-1",
		ConsumedFeedback: true,
	})
	if err != nil {
		t.Fatalf("SavePrep falló: %v", err)
	}
}

func TestLearningPrepClient_SavePrep_409Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"PREP_HASH_MISMATCH"}`))
	}))
	defer srv.Close()

	c := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SavePrep(context.Background(), "q1", SavePrepRequest{SourceHash: "stale"})
	if !errors.Is(err, ErrPrepHashConflict) {
		t.Fatalf("esperaba ErrPrepHashConflict, got: %v", err)
	}
}

func TestLearningPrepClient_SavePrep_500Transient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SavePrep(context.Background(), "q1", SavePrepRequest{SourceHash: "h"})
	if err == nil || errors.Is(err, ErrLearningPermanent) || errors.Is(err, ErrPrepHashConflict) {
		t.Fatalf("esperaba error transitorio (sin sentinel), got: %v", err)
	}
}

// TestLearningPrepClient_SetPrepOutcome_Statuses (HU-IA.4): 200 con y sin applied; el
// 409 no es fallo; 4xx permanente; 5xx transitorio.
func TestLearningPrepClient_SetPrepOutcome_Statuses(t *testing.T) {
	cases := []struct {
		name      string
		code      int
		body      string
		applied   bool
		conflict  bool
		wantErr   bool
		permanent bool
	}{
		{name: "200_applied", code: 200, body: `{"question_id":"q1","status":"failed","applied":true}`, applied: true},
		{name: "200_not_applied", code: 200, body: `{"question_id":"q1","status":"ready","applied":false}`},
		{name: "409_conflict", code: 409, body: `{"code":"PREP_HASH_MISMATCH"}`, conflict: true},
		{name: "400_permanent", code: 400, body: `{}`, wantErr: true, permanent: true},
		{name: "503_transient", code: 503, body: `{}`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/api/v1/internal/questions/q1/prep-status" {
					t.Errorf("petición inesperada: %s %s", r.Method, r.URL.Path)
				}
				var req SetPrepStatusRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Status != PrepStatusFailed || req.SourceHash != "h1" {
					t.Errorf("cuerpo inesperado: %+v", req)
				}
				w.WriteHeader(c.code)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()

			cl := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
			out, err := cl.SetPrepOutcome(context.Background(), "q1", SetPrepStatusRequest{Status: PrepStatusFailed, SourceHash: "h1"})
			if (err != nil) != c.wantErr || errors.Is(err, ErrLearningPermanent) != c.permanent {
				t.Fatalf("err=%v", err)
			}
			if out.Applied != c.applied || out.Conflict != c.conflict {
				t.Fatalf("out=%+v", out)
			}
		})
	}
}

// TestLearningPrepClient_GetPrepSource_Status: el estado viaja en la fuente.
func TestLearningPrepClient_GetPrepSource_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"question_id":"q1","source_hash":"h","llm_prep_status":"ready"}`))
	}))
	defer srv.Close()
	cl := NewLearningPrepClient(LearningPrepClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := cl.GetPrepSource(context.Background(), "q1")
	if err != nil || got.LLMPrepStatus != PrepStatusReady {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
