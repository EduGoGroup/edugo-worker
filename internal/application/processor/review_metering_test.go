package processor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// HU-IA.2: el log de una revisión dice carril, llamadas al modelo y duración. Todo
// con un doble del modelo (ADR 0048) y un logger que captura.

const (
	msgReviewVerdict = "answer revisada por LLM"
	msgReviewFailure = "fallo del LLM o veredicto inválido en una respuesta, se sigue con las demás"
	msgGlobalLane    = "revisión por juicio global"
	msgLLMCall       = "llamada al modelo en revisión"
)

// logEntry es una línea capturada: nivel, mensaje y campos (los de With incluidos).
type logEntry struct {
	level  string
	msg    string
	fields map[string]any
}

// logSink guarda las líneas de un captureLogger y de sus hijos de With.
type logSink struct {
	mu      sync.Mutex
	entries []logEntry
}

// captureLogger es un logger.Logger que guarda cada línea en su sink.
type captureLogger struct {
	sink *logSink
	base []any
}

func newCaptureLogger() *captureLogger { return &captureLogger{sink: &logSink{}} }

func (l *captureLogger) record(level, msg string, kv []any) {
	fields := map[string]any{}
	all := append(append([]any{}, l.base...), kv...)
	for i := 0; i+1 < len(all); i += 2 {
		fields[fmt.Sprint(all[i])] = all[i+1]
	}
	l.sink.mu.Lock()
	defer l.sink.mu.Unlock()
	l.sink.entries = append(l.sink.entries, logEntry{level: level, msg: msg, fields: fields})
}

func (l *captureLogger) Debug(msg string, kv ...interface{}) { l.record("debug", msg, kv) }
func (l *captureLogger) Info(msg string, kv ...interface{})  { l.record("info", msg, kv) }
func (l *captureLogger) Warn(msg string, kv ...interface{})  { l.record("warn", msg, kv) }
func (l *captureLogger) Error(msg string, kv ...interface{}) { l.record("error", msg, kv) }
func (l *captureLogger) Fatal(msg string, kv ...interface{}) { l.record("fatal", msg, kv) }
func (l *captureLogger) Sync() error                         { return nil }
func (l *captureLogger) With(kv ...interface{}) logger.Logger {
	return &captureLogger{sink: l.sink, base: append(append([]any{}, l.base...), kv...)}
}

// byMsg devuelve las líneas con ese mensaje, en orden.
func (l *captureLogger) byMsg(msg string) []logEntry {
	l.sink.mu.Lock()
	defer l.sink.mu.Unlock()
	var out []logEntry
	for _, e := range l.sink.entries {
		if e.msg == msg {
			out = append(out, e)
		}
	}
	return out
}

// failingIdeas es un doble cuya extracción de ideas falla: el carril de criterios
// sigue con la respuesta cruda, y esa llamada cuenta y sale con ok=false.
type failingIdeas struct{ *laneFakeProvider }

func (f failingIdeas) ExtractIdeas(_ context.Context, _ llm.ExtractIdeasRequest) ([]string, error) {
	f.extractCalls++
	return nil, errors.New("boom ideas")
}

// runReviewLog pasa UNA respuesta por Process y devuelve el logger con lo capturado.
func runReviewLog(t *testing.T, provider llm.LLMProvider, ans m2m.PendingAnswer) (*captureLogger, error) {
	t.Helper()
	reader := &mockSettingsReader{settings: settingsWith(
		settingKeyReviewMode, reviewModeLocal, settingKeyReviewFlow, reviewFlowTeacher)}
	learning := &mockLearningClient{pending: m2m.PendingAnswersResponse{
		AttemptID: "attempt-1", Answers: []m2m.PendingAnswer{ans},
	}}
	log := newCaptureLogger()
	p := NewAttemptReviewProcessor(reader, learning, map[string]llm.LLMProvider{reviewModeLocal: provider}, log)
	payload := validEventPayload(t)
	if ans.QuestionType == llm.QuestionTypeShortAnswer {
		payload = shortAnswerEventPayload(t)
	}
	return log, p.Process(context.Background(), payload)
}

// onlyLine exige exactamente una línea con ese mensaje y la devuelve.
func onlyLine(t *testing.T, log *captureLogger, msg string) logEntry {
	t.Helper()
	lines := log.byMsg(msg)
	if len(lines) != 1 {
		t.Fatalf("se esperaba 1 línea %q, hay %d", msg, len(lines))
	}
	return lines[0]
}

// assertLaneFields afirma lane, llm_calls y duration_ms (entero ≥ 0, sin valor) y
// que provider esté solo si hubo llamada.
func assertLaneFields(t *testing.T, e logEntry, lane ReviewLane, calls int) {
	t.Helper()
	if got := e.fields["lane"]; got != string(lane) {
		t.Errorf("lane = %v, se esperaba %q", got, lane)
	}
	if got := e.fields["llm_calls"]; got != calls {
		t.Errorf("llm_calls = %v, se esperaba %d", got, calls)
	}
	d, ok := e.fields["duration_ms"].(int64)
	if !ok || d < 0 {
		t.Errorf("duration_ms = %v (%T), se esperaba un entero >= 0", e.fields["duration_ms"], e.fields["duration_ms"])
	}
	prov, has := e.fields["provider"]
	switch {
	case calls > 0 && prov != "lane-fake":
		t.Errorf("provider = %v, se esperaba lane-fake (hubo %d llamadas)", prov, calls)
	case calls == 0 && has:
		t.Errorf("sin llamada no debe haber provider, hay %v", prov)
	}
}

func TestReviewLog_VerdictLine_LaneCallsDuration(t *testing.T) {
	cases := []struct {
		name  string
		ans   m2m.PendingAnswer
		lane  ReviewLane
		calls int
	}{
		{"lista con un par escalado", laneAnswer(llm.QuestionTypeShortAnswer, "ecuador caracas", laneListPrep), ReviewLaneList, 1},
		{"criterios con dos criterios", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep), ReviewLaneCriteria, 3},
		{"juicio global", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""), ReviewLaneGlobal, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect, reviewScore: 1, criterionVerdict: llm.VerdictCorrect}
			log, err := runReviewLog(t, f, c.ans)
			if err != nil {
				t.Fatalf("Process no debe fallar: %v", err)
			}
			line := onlyLine(t, log, msgReviewVerdict)
			assertLaneFields(t, line, c.lane, c.calls)
			if got := f.reviewCalls + f.pairCalls + f.criterionCalls + f.extractCalls; got != c.calls {
				t.Errorf("el doble recibió %d llamadas, se esperaban %d", got, c.calls)
			}
			t.Logf("línea del veredicto: %v", line.fields)
		})
	}
}

func TestReviewLog_NoCall_NoProvider(t *testing.T) {
	cases := []struct {
		name string
		ans  m2m.PendingAnswer
		lane ReviewLane
	}{
		{"blanco por regla", laneAnswer(llm.QuestionTypeOpenEnded, "...", ""), ReviewLaneBlankRule},
		{"lista resuelta por el match determinista", laneAnswer(llm.QuestionTypeShortAnswer, "ecuador venezuela", laneListPrep), ReviewLaneList},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &laneFakeProvider{}
			log, err := runReviewLog(t, f, c.ans)
			if err != nil {
				t.Fatalf("Process no debe fallar: %v", err)
			}
			line := onlyLine(t, log, msgReviewVerdict)
			assertLaneFields(t, line, c.lane, 0)
			if got := f.reviewCalls + f.pairCalls + f.criterionCalls + f.extractCalls; got != 0 {
				t.Errorf("el doble recibió %d llamadas, se esperaban 0", got)
			}
			t.Logf("línea sin llamada: %v", line.fields)
		})
	}
}

func TestReviewLog_GlobalRetry_CountsBothCalls(t *testing.T) {
	f := &seqReviewProvider{outs: []llm.ReviewResult{{Verdict: ""}, {Verdict: llm.VerdictCorrect, Feedback: "ok"}}}
	log, err := runReviewLog(t, f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""))
	if err != nil {
		t.Fatalf("Process no debe fallar: %v", err)
	}
	assertLaneFields(t, onlyLine(t, log, msgReviewVerdict), ReviewLaneGlobal, 2)
}

func TestReviewLog_FailureLine_CarriesLaneAndCalls(t *testing.T) {
	t.Run("el modelo falla", func(t *testing.T) {
		f := &laneFakeProvider{reviewErr: errors.New("boom")}
		log, err := runReviewLog(t, f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""))
		if err == nil {
			t.Fatal("Process debe devolver el fallo de la respuesta")
		}
		line := onlyLine(t, log, msgReviewFailure)
		assertLaneFields(t, line, ReviewLaneGlobal, 1)
		if line.fields["veredicto_invalido"] != false {
			t.Errorf("veredicto_invalido = %v, se esperaba false", line.fields["veredicto_invalido"])
		}
		if n := len(log.byMsg(msgReviewVerdict)); n != 0 {
			t.Errorf("no debe haber línea del veredicto, hay %d", n)
		}
	})
	t.Run("salida inválida dos veces", func(t *testing.T) {
		f := &seqReviewProvider{outs: []llm.ReviewResult{{Verdict: ""}}}
		log, err := runReviewLog(t, f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""))
		if !errors.Is(err, ErrInvalidVerdict) {
			t.Fatalf("se esperaba ErrInvalidVerdict, hay %v", err)
		}
		line := onlyLine(t, log, msgReviewFailure)
		assertLaneFields(t, line, ReviewLaneGlobal, 2)
		if line.fields["veredicto_invalido"] != true {
			t.Errorf("veredicto_invalido = %v, se esperaba true", line.fields["veredicto_invalido"])
		}
	})
}

func TestReviewLog_GlobalLaneLine(t *testing.T) {
	cases := []struct {
		name string
		ans  m2m.PendingAnswer
		want int
	}{
		{"juicio global", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""), 1},
		{"lista", laneAnswer(llm.QuestionTypeShortAnswer, "ecuador caracas", laneListPrep), 0},
		{"criterios", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect, reviewScore: 1, criterionVerdict: llm.VerdictCorrect}
			log, err := runReviewLog(t, f, c.ans)
			if err != nil {
				t.Fatalf("Process no debe fallar: %v", err)
			}
			lines := log.byMsg(msgGlobalLane)
			if len(lines) != c.want {
				t.Fatalf("líneas %q = %d, se esperaban %d", msgGlobalLane, len(lines), c.want)
			}
			for _, l := range lines {
				if l.level != "info" || l.fields["answer_id"] != "ans-1" {
					t.Errorf("línea del juicio global = %s %v, se esperaba info con answer_id=ans-1", l.level, l.fields)
				}
			}
		})
	}
}

func TestReviewLog_PerCallLine(t *testing.T) {
	assertCalls := func(t *testing.T, log *captureLogger, wantOps []string, wantOK []bool) {
		t.Helper()
		lines := log.byMsg(msgLLMCall)
		if len(lines) != len(wantOps) {
			t.Fatalf("líneas por llamada = %d, se esperaban %d", len(lines), len(wantOps))
		}
		for i, l := range lines {
			if l.level != "debug" {
				t.Errorf("línea %d: nivel %s, se esperaba debug", i, l.level)
			}
			if l.fields["op"] != wantOps[i] || l.fields["ok"] != wantOK[i] || l.fields["answer_id"] != "ans-1" {
				t.Errorf("línea %d = %v, se esperaba op=%s ok=%v answer_id=ans-1", i, l.fields, wantOps[i], wantOK[i])
			}
			if d, ok := l.fields["duration_ms"].(int64); !ok || d < 0 {
				t.Errorf("línea %d: duration_ms = %v, se esperaba un entero >= 0", i, l.fields["duration_ms"])
			}
		}
	}
	ops := []string{"extract_ideas", "check_criterion", "check_criterion"}
	ans := laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep)

	t.Run("todas bien", func(t *testing.T) {
		log, err := runReviewLog(t, &laneFakeProvider{criterionVerdict: llm.VerdictCorrect}, ans)
		if err != nil {
			t.Fatalf("Process no debe fallar: %v", err)
		}
		assertCalls(t, log, ops, []bool{true, true, true})
	})
	t.Run("una llamada falla", func(t *testing.T) {
		f := failingIdeas{&laneFakeProvider{criterionVerdict: llm.VerdictCorrect}}
		log, err := runReviewLog(t, f, ans)
		if err != nil {
			t.Fatalf("Process no debe fallar (la extracción degrada): %v", err)
		}
		assertCalls(t, log, ops, []bool{false, true, true})
		assertLaneFields(t, onlyLine(t, log, msgReviewVerdict), ReviewLaneCriteria, 3)
	})
}

func TestReviewAnswerLane_FillsLaneAndCalls(t *testing.T) {
	cases := []struct {
		name  string
		ans   m2m.PendingAnswer
		lane  ReviewLane
		calls int
	}{
		{"lista", laneAnswer(llm.QuestionTypeShortAnswer, "ecuador caracas", laneListPrep), ReviewLaneList, 1},
		{"criterios", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", laneCriteriaPrep), ReviewLaneCriteria, 3},
		{"global", laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""), ReviewLaneGlobal, 1},
		{"blanco por regla", laneAnswer(llm.QuestionTypeOpenEnded, " . ", ""), ReviewLaneBlankRule, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &laneFakeProvider{reviewVerdict: llm.VerdictCorrect, reviewScore: 1, criterionVerdict: llm.VerdictCorrect}
			got, err := ReviewAnswerLane(context.Background(), f, c.ans, newTestLogger())
			if err != nil {
				t.Fatalf("no debe fallar: %v", err)
			}
			if got.Lane != c.lane || got.LLMCalls != c.calls || got.Duration < 0 {
				t.Errorf("Lane=%q LLMCalls=%d Duration=%v, se esperaba %q, %d y >= 0", got.Lane, got.LLMCalls, got.Duration, c.lane, c.calls)
			}
		})
	}

	t.Run("con error también se rellenan", func(t *testing.T) {
		f := &laneFakeProvider{reviewErr: errors.New("boom")}
		got, err := ReviewAnswerLane(context.Background(), f, laneAnswer(llm.QuestionTypeOpenEnded, "respuesta larga", ""), newTestLogger())
		if err == nil {
			t.Fatal("se esperaba el error del proveedor")
		}
		if got.Lane != ReviewLaneGlobal || got.LLMCalls != 1 {
			t.Errorf("Lane=%q LLMCalls=%d, se esperaba global y 1", got.Lane, got.LLMCalls)
		}
	})
}
