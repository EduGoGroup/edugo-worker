package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/llm/inferencecap"
)

func fakeAnthropic(t *testing.T, text string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "secret-key" {
			t.Errorf("x-api-key ausente/incorrecta: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("falta anthropic-version")
		}
		resp := anthropicResponse{Content: []anthropicContentBlock{{Type: "text", Text: text}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestAnthropic_GenerateAssessment_OK(t *testing.T) {
	srv := fakeAnthropic(t, `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},"questions":[]}`)
	defer srv.Close()

	p, err := New(Config{Provider: ProviderAnthropic, APIKey: "secret-key", Model: "claude-x", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New falló: %v", err)
	}
	raw, err := p.GenerateAssessment(context.Background(), llm.MaterialInput{Content: "c"}, llm.GenerationParams{NumQuestions: 1})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !strings.Contains(string(raw), "edugo.assessment_import") {
		t.Fatalf("JSON inesperado: %s", raw)
	}
}

func TestAnthropic_ReviewAnswer_OK(t *testing.T) {
	srv := fakeAnthropic(t, "```json\n{\"verdict\":\"correct\",\"score\":1.0,\"feedback\":\"bien\"}\n```")
	defer srv.Close()

	p, _ := New(Config{Provider: ProviderAnthropic, APIKey: "secret-key", Model: "m", BaseURL: srv.URL})
	res, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.Verdict != llm.VerdictCorrect || res.Score != 1.0 {
		t.Fatalf("resultado inesperado: %+v", res)
	}
}

func TestAnthropic_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(anthropicResponse{Error: &anthropicError{Type: "authentication_error", Message: "invalid key"}})
	}))
	defer srv.Close()

	p, _ := New(Config{Provider: ProviderAnthropic, APIKey: "bad", Model: "m", BaseURL: srv.URL})
	_, err := p.GenerateAssessment(context.Background(), llm.MaterialInput{}, llm.GenerationParams{})
	if err == nil || !strings.Contains(err.Error(), "authentication_error") {
		t.Fatalf("esperaba error de auth: %v", err)
	}
}

func TestGemini_Stub(t *testing.T) {
	p, err := New(Config{Provider: ProviderGemini, APIKey: "k", Model: "gemini"})
	if err != nil {
		t.Fatalf("New (gemini) no debe fallar en construcción: %v", err)
	}
	_, err = p.GenerateAssessment(context.Background(), llm.MaterialInput{}, llm.GenerationParams{})
	if err == nil || !strings.Contains(err.Error(), "gemini") {
		t.Fatalf("esperaba error de stub gemini: %v", err)
	}
}

func TestNew_UnsupportedProvider(t *testing.T) {
	if _, err := New(Config{Provider: "openai"}); err == nil {
		t.Fatal("esperaba error por proveedor no soportado")
	}
}

// El de pago no manda esquema (la versión de API del worker no consta como
// compatible con la salida estructurada, decisión 1 de la HU-INF.5) ni temperatura;
// el resultado lleva el crudo para que quien llama lo revalide.
func TestAnthropic_ReviewCalls_NoSchemaAndRaw(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		resp := anthropicResponse{Content: []anthropicContentBlock{{Type: "text", Text: `{"verdict":"correct","score":1,"feedback":"ok"}`}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	p, err := New(Config{Provider: ProviderAnthropic, APIKey: "k", Model: "claude-x", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	retry := llm.InvalidOutputRetryTemperature
	res, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{Temperature: &retry})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Raw) == 0 {
		t.Fatal("el resultado debe llevar el crudo")
	}
	if _, err := p.JudgePairEquivalence(context.Background(), llm.PairEquivalenceRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CheckCriterion(context.Background(), llm.CriterionCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	for i, body := range bodies {
		for _, key := range []string{"output_config", "output_format", "temperature"} {
			if _, ok := body[key]; ok {
				t.Errorf("petición %d: no debe llevar %s", i, key)
			}
		}
	}
}

// The paid provider has its own cap: never more than N requests in flight. nil is no cap.
func TestComplete_RespectsItsOwnCap(t *testing.T) {
	const calls = 6
	run := func(t *testing.T, limiter *inferencecap.Limiter, handler func(open int32)) int32 {
		t.Helper()
		var open, peak atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			cur := open.Add(1)
			defer open.Add(-1)
			for {
				old := peak.Load()
				if cur <= old || peak.CompareAndSwap(old, cur) {
					break
				}
			}
			handler(cur)
			_ = json.NewEncoder(w).Encode(anthropicResponse{Content: []anthropicContentBlock{
				{Type: "text", Text: `{"verdict":"correct","score":1,"feedback":"ok"}`}}})
		}))
		defer srv.Close()
		p, err := New(Config{Provider: ProviderAnthropic, APIKey: "k", Model: "m", BaseURL: srv.URL, Limiter: limiter})
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < calls; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"}); err != nil {
					t.Errorf("ReviewAnswer: %v", err)
				}
			}()
		}
		wg.Wait()
		return peak.Load()
	}

	t.Run("con tope de 2", func(t *testing.T) {
		peak := run(t, inferencecap.New("api", 2, nil), func(int32) { time.Sleep(20 * time.Millisecond) })
		if peak > 2 {
			t.Fatalf("hubo %d peticiones en vuelo con tope de 2", peak)
		}
	})

	t.Run("nil no limita", func(t *testing.T) {
		// Every request waits until all of them are open: it only ends without a cap.
		all := make(chan struct{})
		var once sync.Once
		peak := run(t, nil, func(open int32) {
			if open == calls {
				once.Do(func() { close(all) })
			}
			<-all
		})
		if peak != calls {
			t.Fatalf("sin tope se esperaban %d a la vez, hubo %d", calls, peak)
		}
	})
}
