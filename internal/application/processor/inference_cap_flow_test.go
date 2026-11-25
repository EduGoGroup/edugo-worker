package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/llm/inferencecap"
	"github.com/EduGoGroup/edugo-worker/internal/llm/ollama"
)

// The flow test of HU-IA.10: the real Ollama provider against a fake server, cap of 1,
// two reviews (real processor, learning double) and loose material-style calls at once.
// Never two requests in flight, none expires, and the calls of one review arrive together.
func TestFlow_ReviewsAndMaterialCallsShareOneTurn(t *testing.T) {
	const (
		answersPerReview = 3
		looseCalls       = 4
	)
	markers := []string{"REV-A", "REV-B", "LOOSE"}

	var open, peak atomic.Int32
	var mu sync.Mutex
	var order []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := open.Add(1)
		defer open.Add(-1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		tag := "?"
		for _, m := range markers {
			if strings.Contains(req.Prompt, m) {
				tag = m
				break
			}
		}
		mu.Lock()
		order = append(order, tag)
		mu.Unlock()
		time.Sleep(15 * time.Millisecond) // lets every other flow reach the queue
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": `{"verdict":"correct","score":1,"feedback":"ok"}`, "done": true})
	}))
	defer srv.Close()

	limiter := inferencecap.New("local", 1, nil)
	provider := ollama.New(ollama.Config{BaseURL: srv.URL, Model: "m", Timeout: 2 * time.Second, Limiter: limiter})

	answersOf := func(marker string) []m2m.PendingAnswer {
		out := make([]m2m.PendingAnswer, 0, answersPerReview)
		for i := 1; i <= answersPerReview; i++ {
			a := pendingAnswer(fmt.Sprintf("%s-%d", marker, i), 10)
			a.StudentAnswer = fmt.Sprintf("%s respuesta %d", marker, i)
			out = append(out, a)
		}
		return out
	}
	learning := &turnLearning{answers: map[string][]m2m.PendingAnswer{
		"attempt-a": answersOf("REV-A"), "attempt-b": answersOf("REV-B")}}
	p := turnProcessor(learning, provider, reviewModeLocal, limiter)

	var wg sync.WaitGroup
	for _, attemptID := range []string{"attempt-a", "attempt-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.Process(context.Background(), reviewEventFor(t, attemptID)); err != nil {
				t.Errorf("revisión %s: %v", attemptID, err)
			}
		}()
	}
	for i := 0; i < looseCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// A per-call turn, like the material and the preparation: no Hold.
			if _, err := provider.PrepareQuestion(context.Background(), llm.PrepRequest{QuestionText: "LOOSE pregunta"}); err != nil {
				t.Errorf("llamada suelta: %v", err)
			}
		}()
	}
	wg.Wait()

	if peak.Load() != 1 {
		t.Fatalf("hubo %d peticiones en vuelo con tope de 1", peak.Load())
	}
	if want := 2*answersPerReview + looseCalls; len(order) != want {
		t.Fatalf("llegaron %d peticiones, se esperaban %d: %v", len(order), want, order)
	}
	if learning.finalizes.Load() != 2 {
		t.Fatalf("finalizaron %d revisiones de 2", learning.finalizes.Load())
	}
	// The calls of one review arrive together: its tag forms a single run.
	for _, marker := range []string{"REV-A", "REV-B"} {
		first, last, count := -1, -1, 0
		for i, tag := range order {
			if tag == marker {
				if first < 0 {
					first = i
				}
				last = i
				count++
			}
		}
		if count != answersPerReview || last-first+1 != count {
			t.Fatalf("las llamadas de %s no llegaron seguidas: %v", marker, order)
		}
	}
}
