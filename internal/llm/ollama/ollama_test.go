package ollama

import (
	"context"
	"encoding/json"
	"errors"
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

func TestGenerate_RequestIncluyeThinkFalse(t *testing.T) {
	// El body del request a Ollama DEBE incluir "think":false (evita el `{}` de los
	// modelos con thinking como qwen3 bajo format:json). Se verifica que la clave
	// exista y sea false, no solo que el campo por defecto esté ausente.
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		_ = json.NewEncoder(w).Encode(generateResponse{
			Response: `{"verdict":"correct","score":1,"feedback":"ok"}`,
			Done:     true,
		})
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, Model: "qwen3:1.7b"})
	if _, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	think, ok := captured["think"]
	if !ok {
		t.Fatal("el body del request no incluye la clave \"think\"")
	}
	if think != false {
		t.Fatalf("think esperado=false, hubo %v", think)
	}
}

func TestGenerate_RequestIncluyeTemperature(t *testing.T) {
	// La corrección pide JSON estructurado, no prosa: el body DEBE llevar
	// options.temperature con el valor configurado (default 0 = greedy determinista)
	// para que el veredicto sea reproducible entre corridas (045).
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		_ = json.NewEncoder(w).Encode(generateResponse{
			Response: `{"verdict":"correct","score":1,"feedback":"ok"}`,
			Done:     true,
		})
	}))
	defer srv.Close()

	p := New(Config{BaseURL: srv.URL, Model: "gemma3:4b", Temperature: 0})
	if _, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"}); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	opts, ok := captured["options"].(map[string]any)
	if !ok {
		t.Fatal("el body del request no incluye el objeto \"options\"")
	}
	temp, ok := opts["temperature"]
	if !ok {
		t.Fatal("options no incluye la clave \"temperature\"")
	}
	if temp != float64(0) {
		t.Fatalf("temperature esperada=0, hubo %v", temp)
	}
}

func TestName(t *testing.T) {
	if got := New(Config{Model: "abc"}).Name(); got != "ollama:abc" {
		t.Fatalf("Name inesperado: %s", got)
	}
}

// capturingServer responde siempre lo mismo y guarda el body de cada petición.
func capturingServer(t *testing.T, response string, bodies *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		*bodies = append(*bodies, body)
		_ = json.NewEncoder(w).Encode(generateResponse{Response: response, Done: true})
	}))
}

// enumOf devuelve la lista cerrada del veredicto del esquema mandado en format.
func enumOf(t *testing.T, format any) []any {
	t.Helper()
	schema, ok := format.(map[string]any)
	if !ok {
		t.Fatalf("format no es un esquema: %v", format)
	}
	verdict := schema["properties"].(map[string]any)["verdict"].(map[string]any)
	return verdict["enum"].([]any)
}

// Las llamadas del carril de revisión mandan el esquema de su llamada en format; el
// resto sigue con "json" (HU-INF.5, decisión 1).
func TestReviewCalls_SendTheirSchemaInFormat(t *testing.T) {
	var bodies []map[string]any
	srv := capturingServer(t, `{"verdict":"correct","score":1,"feedback":"ok","ideas":["a"]}`, &bodies)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, Model: "m"})
	ctx := context.Background()

	if _, err := p.ReviewAnswer(ctx, llm.ReviewRequest{QuestionType: llm.QuestionTypeShortAnswer}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReviewAnswer(ctx, llm.ReviewRequest{QuestionType: llm.QuestionTypeOpenEnded}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.JudgePairEquivalence(ctx, llm.PairEquivalenceRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckCriterion(ctx, llm.CriterionCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ExtractIdeas(ctx, llm.ExtractIdeasRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.GenerateAssessment(ctx, llm.MaterialInput{}, llm.GenerationParams{}); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 6 {
		t.Fatalf("se esperaban 6 peticiones, hubo %d", len(bodies))
	}
	if got := enumOf(t, bodies[0]["format"]); len(got) != 2 {
		t.Errorf("la corta manda correct|incorrect, salió %v", got)
	}
	if got := enumOf(t, bodies[1]["format"]); len(got) != 3 {
		t.Errorf("la larga manda también partial, salió %v", got)
	}
	for i, name := range []string{"par", "criterio"} {
		if got := enumOf(t, bodies[2+i]["format"]); len(got) != 2 {
			t.Errorf("el %s es binario, salió %v", name, got)
		}
	}
	ideas := bodies[4]["format"].(map[string]any)
	if ideas["properties"].(map[string]any)["ideas"] == nil {
		t.Errorf("la extracción manda el esquema de ideas, salió %v", ideas)
	}
	if bodies[5]["format"] != "json" {
		t.Errorf("la generación sigue con format json, salió %v", bodies[5]["format"])
	}
}

// El reintento de una salida inválida pide su propia temperatura; sin ella, la del
// proveedor. El resultado lleva el crudo para revalidarlo.
func TestReviewCalls_TemperatureOverrideAndRaw(t *testing.T) {
	var bodies []map[string]any
	srv := capturingServer(t, `{"verdict":"Correct","score":0.3,"feedback":"ok"}`, &bodies)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, Model: "m"})
	retry := llm.InvalidOutputRetryTemperature

	res, err := p.CheckCriterion(context.Background(), llm.CriterionCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Raw), `"Correct"`) || res.Verdict != "Correct" {
		t.Fatalf("el proveedor no valida ni normaliza, solo adjunta el crudo: %q %s", res.Verdict, res.Raw)
	}
	if _, err := p.CheckCriterion(context.Background(), llm.CriterionCheckRequest{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.JudgePairEquivalence(context.Background(), llm.PairEquivalenceRequest{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{0, retry, retry, retry} {
		got := bodies[i]["options"].(map[string]any)["temperature"].(float64)
		if got != want {
			t.Errorf("petición %d: temperatura %v, se esperaba %v", i, got, want)
		}
	}
}

// Una salida que no parsea es un error de calidad que conserva el texto «no parseable».
func TestReviewAnswer_UnparseableIsQualityError(t *testing.T) {
	var bodies []map[string]any
	srv := capturingServer(t, `no es json`, &bodies)
	defer srv.Close()
	_, err := New(Config{BaseURL: srv.URL, Model: "m"}).ReviewAnswer(context.Background(), llm.ReviewRequest{})
	if !errors.Is(err, llm.ErrLLMQuality) || !strings.Contains(err.Error(), "no parseable") {
		t.Fatalf("se esperaba un error de calidad «no parseable», salió %v", err)
	}
}
