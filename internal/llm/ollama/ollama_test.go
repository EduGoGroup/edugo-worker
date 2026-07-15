package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

func TestGenerateAssessment_OK(t *testing.T) {
	assessmentJSON := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},"questions":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		var req generateRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("modelo inesperado: %s", req.Model)
		}
		if !strings.Contains(req.Prompt, "MATERIAL") {
			t.Errorf("el prompt no incluye el material")
		}
		_ = json.NewEncoder(w).Encode(generateResponse{Response: assessmentJSON, Done: true})
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, Model: "test-model"})
	raw, err := p.GenerateAssessment(context.Background(),
		llm.MaterialInput{Content: "algo"}, llm.GenerationParams{NumQuestions: 2})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !strings.Contains(string(raw), "edugo.assessment_import") {
		t.Fatalf("JSON inesperado: %s", raw)
	}
}

func TestReviewAnswer_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(generateResponse{
			Response: `{"verdict":"partial","score":0.5,"feedback":"casi"}`,
			Done:     true,
		})
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, Model: "m"})
	res, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.Verdict != llm.VerdictPartial || res.Score != 0.5 {
		t.Fatalf("resultado inesperado: %+v", res)
	}
}

func TestGenerateAssessment_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, Model: "m"})
	if _, err := p.GenerateAssessment(context.Background(), llm.MaterialInput{}, llm.GenerationParams{}); err == nil {
		t.Fatal("esperaba error por status 500")
	}
}

func TestName(t *testing.T) {
	if got := New(Config{Model: "abc"}).Name(); got != "ollama:abc" {
		t.Fatalf("Name inesperado: %s", got)
	}
}
