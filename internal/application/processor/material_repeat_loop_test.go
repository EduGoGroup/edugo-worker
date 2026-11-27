package processor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm/ollama"
)

// HU-IA.9: the real processor with the real Ollama provider against a fake Ollama
// server. No model is called.

const (
	repeatLimitBody = `{"error":"prediction aborted, token repeat limit reached"}`

	fakeSummaryOutput = `{"version":1,"chunk_topic":"fotosíntesis","summary":"La fotosíntesis convierte luz en energía química."}`
	fakeIdeasOutput   = `{"version":1,"main_ideas":["La fotosíntesis convierte luz en energía química."],"secondary_ideas":[]}`
	fakeProposeOutput = `{"candidates":[{"version":1,"question_type":"true_false","question_text":"¿La fotosíntesis usa luz solar?","correct_answer":"true"}]}`
)

// Kinds of material call, told apart by the opening line of each prompt.
const (
	callSummary = "summary"
	callIdeas   = "ideas"
	callPropose = "propose"
)

// fakeOllamaCall is one request the fake server received.
type fakeOllamaCall struct {
	kind    string
	options map[string]any
}

// fakeOllama answers /api/generate. loops says, per kind, which calls (1-based, counted
// per kind) get Ollama's repeat-limit 500; every other call gets a valid output.
type fakeOllama struct {
	mu    sync.Mutex
	loops map[string]map[int]bool
	seen  map[string]int
	calls []fakeOllamaCall
}

func (f *fakeOllama) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Prompt  string         `json:"prompt"`
			Options map[string]any `json:"options"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body no parseable: %v", err)
		}
		kind, output := callIdeas, fakeIdeasOutput
		switch {
		case strings.HasPrefix(body.Prompt, "Eres un generador de preguntas"):
			kind, output = callPropose, fakeProposeOutput
		case strings.Contains(body.Prompt, "produces SOLO dos cosas"):
			kind, output = callSummary, fakeSummaryOutput
		}

		f.mu.Lock()
		f.seen[kind]++
		n := f.seen[kind]
		f.calls = append(f.calls, fakeOllamaCall{kind: kind, options: body.Options})
		loop := f.loops[kind][n]
		f.mu.Unlock()

		if loop {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(repeatLimitBody))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"response": output, "done": true})
	}
}

// ofKind returns the calls of one kind, in order.
func (f *fakeOllama) ofKind(kind string) []fakeOllamaCall {
	var out []fakeOllamaCall
	for _, c := range f.calls {
		if c.kind == kind {
			out = append(out, c)
		}
	}
	return out
}

// runRepeatLoopCase processes a two-chunk job with the given loops and returns the fake
// server's record and the pipeline mock.
func runRepeatLoopCase(t *testing.T, loops map[string]map[int]bool) (*fakeOllama, *mockMaterialPipeline) {
	t.Helper()
	fake := &fakeOllama{loops: loops, seen: map[string]int{}}
	srv := httptest.NewServer(fake.handler(t))
	defer srv.Close()

	second := pendingChunk("c2")
	second.Seq = 1
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1"), second}}
	prov := ollama.New(ollama.Config{BaseURL: srv.URL, Model: "fake"})

	if err := newMaterialProcessor(onSettings(), pipe, prov).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("un bucle de repetición del modelo no debe tumbar el evento: %v", err)
	}
	// HU-IA.5: la fase 1 deja el job en processing/1; el done lo escribe la entrega.
	if last := pipe.updateCalls[len(pipe.updateCalls)-1]; last.status != jobStatusProcessing || last.phase != 1 {
		t.Fatalf("la fase 1 debe terminar en processing/1, quedó %+v", last)
	}
	return fake, pipe
}

// assertRetryOptions checks the first call went out as always and the retry carries the
// retry temperature and a seed.
func assertRetryOptions(t *testing.T, calls []fakeOllamaCall) {
	t.Helper()
	if len(calls) < 2 {
		t.Fatalf("se esperaban al menos dos llamadas, hubo %d", len(calls))
	}
	if _, seeded := calls[0].options["seed"]; seeded || calls[0].options["temperature"] != float64(0) {
		t.Errorf("la primera llamada va a temperatura 0 y sin semilla: %v", calls[0].options)
	}
	seed, ok := calls[1].options["seed"].(float64)
	if !ok || seed == 0 || calls[1].options["temperature"] != llmRetryTemperature {
		t.Errorf("el reintento va a %v y con semilla: %v", llmRetryTemperature, calls[1].options)
	}
}

func TestMaterialProcess_ProposeRepeatLoop_RetriesThenEmptyAndContinues(t *testing.T) {
	// «Leer» bien; «preguntar» da el 500 del bucle en la llamada y en su reintento.
	fake, pipe := runRepeatLoopCase(t, map[string]map[int]bool{callPropose: {1: true, 2: true}})

	propose := fake.ofKind(callPropose)
	if len(propose) != 3 {
		t.Fatalf("se esperaban 3 llamadas a «preguntar» (2 del primer trozo y 1 del segundo), hubo %d", len(propose))
	}
	assertRetryOptions(t, propose)
	if pipe.saveCalls != 2 || len(pipe.savedCandidates[0]) != 0 || len(pipe.savedCandidates[1]) != 1 {
		t.Fatalf("el primer trozo se guarda sin candidatas y el segundo con la suya: %v", pipe.savedCandidates)
	}
	if len(pipe.markFailedChunks) != 0 {
		t.Fatalf("un bucle en «preguntar» no aísla el trozo: %v", pipe.markFailedChunks)
	}
}

func TestMaterialProcess_ProposeRepeatLoop_RetrySucceeds(t *testing.T) {
	fake, pipe := runRepeatLoopCase(t, map[string]map[int]bool{callPropose: {1: true}})

	propose := fake.ofKind(callPropose)
	if len(propose) != 3 {
		t.Fatalf("se esperaban 3 llamadas a «preguntar», hubo %d", len(propose))
	}
	assertRetryOptions(t, propose)
	if pipe.saveCalls != 2 || len(pipe.savedCandidates[0]) != 1 || len(pipe.savedCandidates[1]) != 1 {
		t.Fatalf("el reintento trae la candidata y se guarda: %v", pipe.savedCandidates)
	}
}

func TestMaterialProcess_DigestRepeatLoop_IsolatesChunk(t *testing.T) {
	// «Leer» da el 500 del bucle en la primera mitad, en la llamada y en su reintento.
	fake, pipe := runRepeatLoopCase(t, map[string]map[int]bool{callSummary: {1: true, 2: true}})

	assertRetryOptions(t, fake.ofKind(callSummary))
	if len(pipe.markFailedChunks) != 1 || pipe.markFailedChunks[0] != "c1" {
		t.Fatalf("el trozo del bucle queda aislado: %v", pipe.markFailedChunks)
	}
	if pipe.saveCalls != 1 || len(pipe.savedCandidates[0]) != 1 || len(fake.ofKind(callPropose)) != 1 {
		t.Fatalf("el segundo trozo se procesa entero: guardados %d, candidatas %v", pipe.saveCalls, pipe.savedCandidates)
	}
}
