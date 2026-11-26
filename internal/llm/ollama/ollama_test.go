package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

// repeatLimitResponse es el cuerpo literal del 500 de Ollama 0.34.4 ante un bucle.
const repeatLimitResponse = `{"error":"prediction aborted, token repeat limit reached"}`

// statusServer responde siempre el mismo estado y cuerpo.
func statusServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// El 500 por bucle de repetición sale del proveedor como fallo de calidad, con su texto.
func TestGenerate_RepeatLimit500IsQuality(t *testing.T) {
	srv := statusServer(http.StatusInternalServerError, repeatLimitResponse)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, Model: "m"})
	ctx := context.Background()

	calls := map[string]func() error{
		"ProposeCandidates": func() error {
			_, err := p.ProposeCandidates(ctx, llm.ProposeCandidatesInput{})
			return err
		},
		"DigestChunk": func() error {
			_, err := p.DigestChunk(ctx, llm.DigestChunkInput{})
			return err
		},
		"ReviewAnswer": func() error {
			_, err := p.ReviewAnswer(ctx, llm.ReviewRequest{})
			return err
		},
		"PrepareQuestion": func() error {
			_, err := p.PrepareQuestion(ctx, llm.PrepRequest{})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		if !errors.Is(err, ErrRepeatLimit) || !errors.Is(err, llm.ErrLLMQuality) {
			t.Errorf("%s: se esperaba ErrRepeatLimit (calidad), salió %v", name, err)
			continue
		}
		if !strings.Contains(err.Error(), "token repeat limit reached") || !strings.Contains(err.Error(), "500") {
			t.Errorf("%s: el error no conserva el estado y el texto de Ollama: %v", name, err)
		}
	}
}

// Cualquier otro estado no 2xx sigue siendo infraestructura: sube sin el centinela de
// calidad (el 500 con otro texto, y el texto del bucle con otro estado).
func TestGenerateAssessment_HTTPError(t *testing.T) {
	cases := []struct {
		status int
		body   string
	}{
		{http.StatusInternalServerError, "boom"},
		{http.StatusServiceUnavailable, repeatLimitResponse},
		{http.StatusInternalServerError, `{"error":"model runner has unexpectedly stopped"}`},
	}
	for _, c := range cases {
		srv := statusServer(c.status, c.body)
		_, err := New(Config{BaseURL: srv.URL, Model: "m"}).GenerateAssessment(context.Background(), llm.MaterialInput{}, llm.GenerationParams{})
		srv.Close()
		if err == nil || errors.Is(err, llm.ErrLLMQuality) {
			t.Errorf("%d %q: se esperaba un error de infraestructura, salió %v", c.status, c.body, err)
		}
	}
}

// La semilla viaja solo en los reintentos de «leer» y «preguntar»; el resto de las
// peticiones lleva en options lo mismo que antes: solo la temperatura.
func TestGenerate_SeedOnlyOnMaterialRetry(t *testing.T) {
	var bodies []map[string]any
	response := `{"version":1,"chunk_topic":"t","summary":"s","main_ideas":["i"],"candidates":[],"category":"central",` +
		`"verdict":"correct","score":1,"feedback":"ok","ideas":["a"]}`
	srv := capturingServer(t, response, &bodies)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, Model: "m"})
	ctx := context.Background()
	retry := llm.InvalidOutputRetryTemperature

	options := func(from int) []map[string]any {
		out := make([]map[string]any, 0, len(bodies)-from)
		for _, b := range bodies[from:] {
			out = append(out, b["options"].(map[string]any))
		}
		return out
	}

	// Con temperatura de reintento: las dos mitades de «leer» y «preguntar» mandan seed.
	if _, err := p.DigestChunk(ctx, llm.DigestChunkInput{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ProposeCandidates(ctx, llm.ProposeCandidatesInput{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	seeded := options(0)
	if len(seeded) != 3 {
		t.Fatalf("se esperaban 3 peticiones con reintento, hubo %d", len(seeded))
	}
	for i, o := range seeded {
		seed, ok := o["seed"].(float64)
		if !ok || seed == 0 || o["temperature"] != retry {
			t.Errorf("petición %d del reintento de material: options %v (se espera seed distinta de cero y temperatura %v)", i, o, retry)
		}
	}

	// Sin temperatura de reintento, y en toda llamada que no es del material: sin seed.
	mark := len(bodies)
	unseeded := []func() error{
		func() error { _, err := p.DigestChunk(ctx, llm.DigestChunkInput{}); return err },
		func() error { _, err := p.ProposeCandidates(ctx, llm.ProposeCandidatesInput{}); return err },
		func() error { _, err := p.ReviewAnswer(ctx, llm.ReviewRequest{Temperature: &retry}); return err },
		func() error {
			_, err := p.JudgePairEquivalence(ctx, llm.PairEquivalenceRequest{Temperature: &retry})
			return err
		},
		func() error {
			_, err := p.CheckCriterion(ctx, llm.CriterionCheckRequest{Temperature: &retry})
			return err
		},
		func() error { _, err := p.ExtractIdeas(ctx, llm.ExtractIdeasRequest{}); return err },
		func() error { _, err := p.ScoreRelevance(ctx, llm.RelevanceRequest{}); return err },
		func() error { _, err := p.PrepareQuestion(ctx, llm.PrepRequest{}); return err },
	}
	for i, call := range unseeded {
		if err := call(); err != nil {
			t.Fatalf("llamada %d sin semilla: %v", i, err)
		}
	}
	for i, o := range options(mark) {
		if len(o) != 1 || o["temperature"] == nil {
			t.Errorf("petición %d: options %v (se espera solo temperature, como antes)", i, o)
		}
	}
	// El cuerpo entero lleva las seis claves de siempre y ninguna más.
	for i, b := range bodies[mark:] {
		for _, key := range []string{"model", "prompt", "stream", "format", "think", "options"} {
			if _, ok := b[key]; !ok {
				t.Errorf("petición %d: falta la clave %q", i, key)
			}
		}
		if len(b) != 6 {
			t.Errorf("petición %d: el cuerpo trae %d claves, se esperan 6: %v", i, len(b), b)
		}
	}
}

// testGuard only bounds a test that hangs; no assertion depends on it elapsing.
const testGuard = 10 * time.Second

// gatedOllama is a fake Ollama that, like the real one with OLLAMA_NUM_PARALLEL=1, serves
// one request at a time and queues the rest. It tells the test when a request is being
// served (entered) and holds it until the test says so (release), or until the client
// gives up. peak is the most requests it ever had open at once (queued included) and
// hits how many arrived.
type gatedOllama struct {
	srv        *httptest.Server
	entered    chan struct{}
	release    chan struct{}
	peak, hits atomic.Int32
}

func newGatedOllama(t *testing.T) *gatedOllama {
	t.Helper()
	g := &gatedOllama{entered: make(chan struct{}, 64), release: make(chan struct{}, 64)}
	var open atomic.Int32
	serial := make(chan struct{}, 1)
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The body is read first: only then does the server notice a client that gave up.
		_, _ = io.Copy(io.Discard, r.Body)
		g.hits.Add(1)
		cur := open.Add(1)
		defer open.Add(-1)
		for {
			old := g.peak.Load()
			if cur <= old || g.peak.CompareAndSwap(old, cur) {
				break
			}
		}
		select {
		case serial <- struct{}{}:
			defer func() { <-serial }()
		case <-r.Context().Done():
			return
		}
		g.entered <- struct{}{}
		select {
		case <-g.release:
		case <-r.Context().Done():
			return
		}
		if r.URL.Path == "/api/embed" {
			_ = json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1}}})
			return
		}
		_ = json.NewEncoder(w).Encode(generateResponse{
			Response: `{"verdict":"correct","score":1,"feedback":"ok"}`, Done: true})
	}))
	t.Cleanup(g.srv.Close)
	return g
}

// serve lets n requests through, one by one, as each one is being served.
func (g *gatedOllama) serve(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-g.entered:
			g.release <- struct{}{}
		case <-time.After(testGuard):
			t.Errorf("la petición %d de %d no llegó al servidor", i+1, n)
			return
		}
	}
}

// watchedLimiter builds a limiter whose queue length changes arrive on a channel.
func watchedLimiter(capacity int) (*inferencecap.Limiter, <-chan int) {
	l := inferencecap.New("local", capacity, nil)
	changes := make(chan int, 1024)
	l.OnQueueChange(func(waiting int) { changes <- waiting })
	return l, changes
}

// waitQueued blocks until the limiter reports n callers queued.
func waitQueued(t *testing.T, changes <-chan int, n int) {
	t.Helper()
	guard := time.After(testGuard)
	for {
		select {
		case got := <-changes:
			if got == n {
				return
			}
		case <-guard:
			t.Fatalf("la fila del turno no llegó a %d", n)
		}
	}
}

// reviewAtOnce fires n ReviewAnswer calls at the same time; failed receives how many
// failed once all of them returned.
func reviewAtOnce(p *Provider, n int) (failed <-chan int32) {
	out := make(chan int32, 1)
	var wg sync.WaitGroup
	var errs atomic.Int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.ReviewAnswer(context.Background(), llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"}); err != nil {
				errs.Add(1)
			}
		}()
	}
	go func() {
		wg.Wait()
		out <- errs.Load()
	}()
	return out
}

// CA-IA.14: with more calls asked than the cap, the server never sees two at once and
// the wait for the turn is off the call clock. Without the cap (nil), the same load
// expires: the calls queue inside the server with their clock running.
func TestGenerate_WaitDoesNotCountOnClock(t *testing.T) {
	const (
		calls   = 5
		timeout = 250 * time.Millisecond
	)

	t.Run("con tope de 1 ninguna vence", func(t *testing.T) {
		g := newGatedOllama(t)
		limiter, changes := watchedLimiter(1)
		p := New(Config{BaseURL: g.srv.URL, Model: "m", Timeout: timeout, Limiter: limiter})

		// The test holds the only turn: the five calls queue outside the server.
		busy, err := limiter.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		failed := reviewAtOnce(p, calls)
		waitQueued(t, changes, calls)
		// The one real duration of this test: the calls wait for their turn longer than
		// their whole Timeout. A clock that counted the wait would expire all of them.
		<-time.After(2 * timeout)
		if g.hits.Load() != 0 {
			t.Fatalf("llegaron %d peticiones al servidor sin turno", g.hits.Load())
		}
		busy()
		g.serve(t, calls)
		if got := <-failed; got != 0 {
			t.Fatalf("vencieron %d llamadas con el tope puesto", got)
		}
		if g.peak.Load() != 1 || g.hits.Load() != calls {
			t.Fatalf("el servidor vio %d a la vez (%d peticiones esperadas, llegaron %d)", g.peak.Load(), calls, g.hits.Load())
		}
	})

	t.Run("sin tope vencen en la fila del servidor", func(t *testing.T) {
		// Nobody releases the server: every call sits inside it, the first being served
		// and the rest queued, until its own Timeout expires.
		g := newGatedOllama(t)
		p := New(Config{BaseURL: g.srv.URL, Model: "m", Timeout: timeout})
		if got := <-reviewAtOnce(p, calls); got == 0 {
			t.Fatal("sin tope se esperaba al menos una llamada vencida")
		}
		if g.peak.Load() < 2 {
			t.Fatalf("sin tope el servidor debía ver varias a la vez, vio %d", g.peak.Load())
		}
	})
}

func TestGenerate_CancelledWhileWaitingNeverCalls(t *testing.T) {
	g := newGatedOllama(t)
	limiter, changes := watchedLimiter(1)
	busy, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer busy()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	p := New(Config{BaseURL: g.srv.URL, Model: "m", Limiter: limiter})
	go func() {
		_, err := p.ReviewAnswer(ctx, llm.ReviewRequest{QuestionText: "q", StudentAnswer: "a"})
		errCh <- err
	}()
	waitQueued(t, changes, 1)
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("se esperaba context.Canceled, llegó %v", err)
	}
	if g.hits.Load() != 0 {
		t.Fatalf("el servidor recibió %d peticiones de una llamada cancelada en la fila", g.hits.Load())
	}
}

// TestPrepareQuestion_TemperatureOverride (HU-IA.4): sin temperatura, el cuerpo de
// hoy (la del proveedor, sin seed); con ella, options.temperature 0,3.
func TestPrepareQuestion_TemperatureOverride(t *testing.T) {
	var bodies []map[string]any
	srv := capturingServer(t, `{"version":1}`, &bodies)
	defer srv.Close()
	p := New(Config{BaseURL: srv.URL, Model: "m"})
	retry := llm.InvalidOutputRetryTemperature

	if _, err := p.PrepareQuestion(context.Background(), llm.PrepRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.PrepareQuestion(context.Background(), llm.PrepRequest{Temperature: &retry}); err != nil {
		t.Fatal(err)
	}
	base, over := bodies[0]["options"].(map[string]any), bodies[1]["options"].(map[string]any)
	if base["temperature"] != p.temperature || base["seed"] != nil {
		t.Errorf("sin override: options %v", base)
	}
	if over["temperature"] != retry || over["seed"] != nil {
		t.Errorf("con override: options %v", over)
	}
}
