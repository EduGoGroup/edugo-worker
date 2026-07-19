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

func TestLearningPipelineClient_GetJob_OK(t *testing.T) {
	assessmentID := "asmt-1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/internal/pipeline/jobs/job-1" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("falta el bearer: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(PipelineJob{
			JobID:         "job-1",
			MaterialID:    "mat-1",
			MaterialTitle: "Fracciones",
			Status:        "processing",
			Phase:         1,
			AssessmentID:  &assessmentID,
			ChunkCounts:   map[string]int{"pending": 2, "done": 1},
			CreatedAt:     "2026-07-17T00:00:00Z",
			UpdatedAt:     "2026-07-17T00:01:00Z",
		})
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob falló: %v", err)
	}
	if got.MaterialTitle != "Fracciones" || got.Phase != 1 || got.ChunkCounts["pending"] != 2 {
		t.Fatalf("respuesta mal mapeada: %+v", got)
	}
	if got.AssessmentID == nil || *got.AssessmentID != "asmt-1" {
		t.Fatalf("assessment_id mal mapeado: %+v", got.AssessmentID)
	}
}

func TestLearningPipelineClient_GetJob_404Permanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	_, err := c.GetJob(context.Background(), "job-1")
	if !errors.Is(err, ErrLearningPermanent) {
		t.Fatalf("esperaba ErrLearningPermanent, got: %v", err)
	}
	if errors.Is(err, ErrPipelineConflict) {
		t.Fatal("404 no debe ser ErrPipelineConflict")
	}
}

func TestLearningPipelineClient_GetFileURL_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/pipeline/jobs/job-1/file-url") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(PresignedFile{
			URL:       "https://r2.example/signed?sig=abc",
			ExpiresAt: "2026-07-17T01:00:00Z",
		})
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.GetFileURL(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetFileURL falló: %v", err)
	}
	if got.URL == "" || got.ExpiresAt == "" {
		t.Fatalf("presigned mal mapeado: %+v", got)
	}
}

func TestLearningPipelineClient_SaveChunks_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pipeline/jobs/job-1/chunks") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body saveChunksRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Chunks) != 2 || body.Chunks[0].Seq != 0 || body.Chunks[1].ChunkText != "b" {
			t.Errorf("body inesperado: %+v", body)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"count": 2})
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SaveChunks(context.Background(), "job-1", []ChunkInput{
		{Seq: 0, ChunkText: "a"},
		{Seq: 1, ChunkText: "b"},
	})
	if err != nil {
		t.Fatalf("SaveChunks falló: %v", err)
	}
}

func TestLearningPipelineClient_SaveChunks_409Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"CHUNKS_CLOSED"}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SaveChunks(context.Background(), "job-1", nil)
	if !errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("esperaba ErrPipelineConflict, got: %v", err)
	}
	// El 409 NO debe clasificarse como permanente: es un guard de estado, el caller decide.
	if errors.Is(err, ErrLearningPermanent) {
		t.Fatal("409 de pipeline no debe ser ErrLearningPermanent")
	}
}

func TestLearningPipelineClient_SaveChunks_500Transient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SaveChunks(context.Background(), "job-1", nil)
	if err == nil || errors.Is(err, ErrLearningPermanent) || errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("esperaba error transitorio (sin sentinel), got: %v", err)
	}
}

func TestLearningPipelineClient_GetNextPendingChunk_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/pipeline/jobs/job-1/chunks/pending") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"chunk":{"chunk_id":"ch-1","job_id":"job-1","seq":3,"chunk_text":"texto","status":"pending"},"prev_summary":"resumen previo"}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.GetNextPendingChunk(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetNextPendingChunk falló: %v", err)
	}
	if got == nil || got.ChunkID != "ch-1" || got.Seq != 3 {
		t.Fatalf("chunk mal mapeado: %+v", got)
	}
	if got.PrevSummary == nil || *got.PrevSummary != "resumen previo" {
		t.Fatalf("prev_summary no adjuntado: %+v", got.PrevSummary)
	}
}

func TestLearningPipelineClient_GetNextPendingChunk_Null(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"chunk":null}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.GetNextPendingChunk(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("chunk:null no debe fallar: %v", err)
	}
	if got != nil {
		t.Fatalf("esperaba nil cuando chunk es null, got: %+v", got)
	}
}

func TestLearningPipelineClient_SaveChunkArtifacts_OK(t *testing.T) {
	summary := "resumen del chunk"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pipeline/chunks/ch-1/artifacts") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body saveChunkArtifactsRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Summary == nil || *body.Summary != summary {
			t.Errorf("summary inesperado: %+v", body.Summary)
		}
		if len(body.Candidates) != 1 {
			t.Errorf("candidates inesperados: %+v", body.Candidates)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SaveChunkArtifacts(context.Background(), "ch-1", &summary,
		json.RawMessage(`{"key":"val"}`),
		[]CandidatePayload{{Payload: json.RawMessage(`{"question":"¿?"}`)}},
	)
	if err != nil {
		t.Fatalf("SaveChunkArtifacts falló: %v", err)
	}
}

func TestLearningPipelineClient_SaveChunkArtifacts_409Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.SaveChunkArtifacts(context.Background(), "ch-1", nil, nil, nil)
	if !errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("esperaba ErrPipelineConflict, got: %v", err)
	}
}

func TestLearningPipelineClient_UpdateJobStatus_OK(t *testing.T) {
	lastErr := "boom"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pipeline/jobs/job-1/status") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body updateJobStatusRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Status != "processing" || body.Phase != 2 {
			t.Errorf("body inesperado: %+v", body)
		}
		if body.LastError == nil || *body.LastError != "boom" {
			t.Errorf("last_error inesperado: %+v", body.LastError)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.UpdateJobStatus(context.Background(), "job-1", "processing", 2, &lastErr)
	if err != nil {
		t.Fatalf("UpdateJobStatus falló: %v", err)
	}
}

func TestLearningPipelineClient_UpdateJobStatus_409Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	err := c.UpdateJobStatus(context.Background(), "job-1", "done", 3, nil)
	if !errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("esperaba ErrPipelineConflict, got: %v", err)
	}
}

func TestLearningPipelineClient_ListCandidates_OK(t *testing.T) {
	grp := "grp-1"
	score := 0.8
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/internal/pipeline/jobs/job-1/candidates" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("falta el bearer: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"candidates":[
			{"id":"c1","chunk_id":"ch1","chunk_sequence":0,"payload":{"version":1},"status":"candidate","dedupe_group":null,"score":null,"embedding":null},
			{"id":"c2","chunk_id":"ch2","chunk_sequence":1,"payload":{"version":1},"status":"dropped_dup","dedupe_group":"grp-1","score":0.8,"embedding":[0.1,0.2]}
		]}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	got, err := c.ListCandidates(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListCandidates falló: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 candidatas, got %d", len(got))
	}
	if got[0].ID != "c1" || got[0].ChunkSequence != 0 || got[0].DedupeGroup != nil || got[0].Score != nil {
		t.Fatalf("c1 mal mapeada: %+v", got[0])
	}
	if got[1].DedupeGroup == nil || *got[1].DedupeGroup != grp || got[1].Score == nil || *got[1].Score != score {
		t.Fatalf("c2 mal mapeada: %+v", got[1])
	}
	if string(got[1].Embedding) != "[0.1,0.2]" {
		t.Fatalf("embedding crudo mal mapeado: %s", string(got[1].Embedding))
	}
}

func TestLearningPipelineClient_ListCandidates_EmptyJobID(t *testing.T) {
	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: "http://x", TokenProvider: staticToken{"tok"}})
	if _, err := c.ListCandidates(context.Background(), ""); err == nil {
		t.Fatal("esperaba error con job_id vacío")
	}
}

func TestLearningPipelineClient_UpdateCandidates_OK(t *testing.T) {
	dropped := "dropped_dup"
	grp := "grp-9"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("método inesperado: %s", r.Method)
		}
		if r.URL.Path != "/api/v1/internal/pipeline/candidates" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var body updateCandidatesRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("body ilegible: %v", err)
		}
		if len(body.Updates) != 2 {
			t.Errorf("esperaba 2 updates, got %d", len(body.Updates))
		}
		// El update parcial de embedding no debe llevar status/dedupe_group (omitempty).
		raw, _ := json.Marshal(body.Updates[0])
		if strings.Contains(string(raw), "status") || strings.Contains(string(raw), "dedupe_group") {
			t.Errorf("update parcial no debía incluir campos nil: %s", raw)
		}
		_, _ = w.Write([]byte(`{"updated":2}`))
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	n, err := c.UpdateCandidates(context.Background(), []CandidateUpdate{
		{ID: "c1", Embedding: json.RawMessage(`[0.1,0.2]`)},
		{ID: "c2", Status: &dropped, DedupeGroup: &grp},
	})
	if err != nil {
		t.Fatalf("UpdateCandidates falló: %v", err)
	}
	if n != 2 {
		t.Fatalf("esperaba updated=2, got %d", n)
	}
}

func TestLearningPipelineClient_UpdateCandidates_EmptyNoCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	n, err := c.UpdateCandidates(context.Background(), nil)
	if err != nil || n != 0 {
		t.Fatalf("lote vacío debía ser (0, nil): (%d, %v)", n, err)
	}
	if called {
		t.Fatal("un lote vacío no debía tocar la red")
	}
}

func TestLearningPipelineClient_UpdateCandidates_409Conflict(t *testing.T) {
	dropped := "dropped_dup"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := NewLearningPipelineClient(LearningPipelineClientConfig{BaseURL: srv.URL, TokenProvider: staticToken{"tok"}})
	_, err := c.UpdateCandidates(context.Background(), []CandidateUpdate{{ID: "c1", Status: &dropped}})
	if !errors.Is(err, ErrPipelineConflict) {
		t.Fatalf("esperaba ErrPipelineConflict, got: %v", err)
	}
}
