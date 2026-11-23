package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// El modo bank (AR-INF.1, decisiones 3 y 4): pasa el banco etiquetado de learning
// por el carril de corrección real del worker (processor.ReviewAnswerLane), una
// corrida por caso, y saca una tabla por categoría. Mide, no juzga (ADR 0048): no
// hay PASS/FAIL por caso, no se repite nada y no termina con código != 0 por lo
// que diga el modelo; sí, si no pudo correr (sin modelo, sin banco, informe de otro
// banco).
//
// Solo pasan por el modelo los casos que la parte determinista de learning dejó
// pendientes (el informe de -deterministic): son los que en producción le llegan.

// bankCase es un caso del banco (internal/core/domain/testdata/answer_bank.json de
// edugo-api-learning). answer_spec no se lee: hoy ningún carril la usa.
type bankCase struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Category string          `json:"category"`
	Group    string          `json:"group"`
	Question string          `json:"question"`
	Expected string          `json:"expected"`
	Student  string          `json:"student"`
	Label    string          `json:"label"`
	Rubric   string          `json:"rubric,omitempty"`
	LLMPrep  json.RawMessage `json:"llm_prep,omitempty"`
}

// deterministicReport es el informe que escribe la prueba de learning con
// ANSWER_BANK_REPORT.
type deterministicReport struct {
	BankSHA256 string `json:"bank_sha256"`
	Cases      []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
	} `json:"cases"`
}

const deterministicToModel = "to_model"

// Desenlace de un caso por el carril del modelo.
const (
	bankHit           = "hit"            // el veredicto coincide con la etiqueta
	bankFalseHit      = "false_hit"      // el veredicto es más favorable que la etiqueta
	bankFalseMiss     = "false_miss"     // el veredicto es menos favorable que la etiqueta
	bankInvalid       = "invalid"        // veredicto inválido o salida que no parsea
	bankProviderError = "provider_error" // red, timeout, servidor: no es del modelo
)

// verdictRank ordena los veredictos (y las etiquetas) de menos a más favorable.
var verdictRank = map[string]int{"incorrect": 0, "partial": 1, "correct": 2}

// bankCaseRun es lo medido de un caso.
type bankCaseRun struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Category    string  `json:"category"`
	Group       string  `json:"group"`
	Label       string  `json:"label"`
	Verdict     string  `json:"verdict,omitempty"`
	Score       float64 `json:"score"`
	Points      float64 `json:"points"`
	Outcome     string  `json:"outcome"`
	Error       string  `json:"error,omitempty"`
	ElapsedMS   int64   `json:"elapsed_ms"`
	SubCalls    int     `json:"sub_calls"`
	SubInvalid  int     `json:"sub_invalid"`
	IdeasFailed bool    `json:"ideas_failed,omitempty"`
}

// bankRunData es lo que se guarda en el historial.
type bankRunData struct {
	BankFile            string        `json:"bank_file"`
	BankSHA256          string        `json:"bank_sha256"`
	DeterministicSHA256 string        `json:"deterministic_sha256"`
	BankCases           int           `json:"bank_cases"`
	ToModel             int           `json:"to_model"`
	Cases               []bankCaseRun `json:"cases"`
	Table               []bankRunRow  `json:"table"`
	Markdown            string        `json:"markdown"`
}

func runBank(p llm.LLMProvider, info runInfo, bankPath, detPath, runsDir string, timeout time.Duration) {
	if strings.TrimSpace(bankPath) == "" || strings.TrimSpace(detPath) == "" {
		fatalf("el modo bank pide -bank (el banco de learning) y -deterministic (su informe)")
	}
	bankRaw, err := os.ReadFile(bankPath)
	if err != nil {
		fatalf("leyendo el banco %q: %v", bankPath, err)
	}
	var bank struct {
		Cases []bankCase `json:"cases"`
	}
	if err := json.Unmarshal(bankRaw, &bank); err != nil || len(bank.Cases) == 0 {
		fatalf("el banco %q no parsea o está vacío: %v", bankPath, err)
	}
	detRaw, err := os.ReadFile(detPath)
	if err != nil {
		fatalf("leyendo el informe determinista %q: %v", detPath, err)
	}
	var det deterministicReport
	if err := json.Unmarshal(detRaw, &det); err != nil {
		fatalf("el informe determinista %q no parsea: %v", detPath, err)
	}
	bankSum := fileSHA256(bankRaw)
	if det.BankSHA256 != bankSum {
		fatalf("el informe determinista es de otro banco (sha256 %s, el banco es %s)", det.BankSHA256, bankSum)
	}
	outcome := make(map[string]string, len(det.Cases))
	for _, c := range det.Cases {
		outcome[c.ID] = c.Outcome
	}
	toModel := make([]bankCase, 0, len(bank.Cases))
	for _, c := range bank.Cases {
		o, ok := outcome[c.ID]
		if !ok {
			fatalf("el caso %s no está en el informe determinista", c.ID)
		}
		if o == deterministicToModel {
			toModel = append(toModel, c)
		}
	}

	fmt.Printf("== llm-harness (bank) ==\n")
	fmt.Printf("provider : %s · modelo %s %s · temperatura %.0f\n", p.Name(), info.Model, info.ModelDigest, info.Temperature)
	fmt.Printf("banco    : sha256 %s · %d casos · %d llegan al modelo\n", bankSum, len(bank.Cases), len(toModel))
	fmt.Printf("worker   : %s (árbol sucio: %v)\n\n", info.WorkerCommit, info.WorkerDirty)

	log := logger.NewSlogAdapter(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	runs := make([]bankCaseRun, 0, len(toModel))
	for i, c := range toModel {
		r := runBankCase(p, c, log, timeout)
		runs = append(runs, r)
		fmt.Printf("[%3d/%3d] %-26s %-15s etiqueta=%-9s veredicto=%-9s %-14s %6dms\n",
			i+1, len(toModel), c.ID, c.Category, c.Label, orDash(r.Verdict), r.Outcome, r.ElapsedMS)
	}

	table := bankTable(runs)
	md := bankRunMarkdown(info, bankSum, len(bank.Cases), table)
	fmt.Printf("\n%s", md)

	saveRun(runsDir, info, bankRunData{
		BankFile:            "edugo-api-learning/internal/core/domain/testdata/answer_bank.json",
		BankSHA256:          bankSum,
		DeterministicSHA256: fileSHA256(detRaw),
		BankCases:           len(bank.Cases),
		ToModel:             len(toModel),
		Cases:               runs,
		Table:               table,
		Markdown:            md,
	})
}

// runBankCase pasa UN caso por el carril real, una sola vez.
func runBankCase(base llm.LLMProvider, c bankCase, log logger.Logger, timeout time.Duration) bankCaseRun {
	questionType := llm.QuestionTypeShortAnswer
	if c.Kind == "long" {
		questionType = llm.QuestionTypeOpenEnded
	}
	ans := m2m.PendingAnswer{
		AnswerID:       c.ID,
		QuestionType:   questionType,
		QuestionText:   c.Question,
		Rubric:         c.Rubric,
		ExpectedAnswer: c.Expected,
		StudentAnswer:  c.Student,
		Points:         1,
		LLMPrep:        c.LLMPrep,
	}
	audit := &auditProvider{LLMProvider: base}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	start := time.Now()
	res, err := processor.ReviewAnswerLane(ctx, audit, ans, log)
	elapsed := time.Since(start)
	cancel()

	r := bankCaseRun{
		ID: c.ID, Kind: c.Kind, Category: c.Category, Group: c.Group, Label: c.Label,
		Verdict: string(res.Result.Verdict), Score: res.Result.Score, Points: res.Points,
		ElapsedMS: elapsed.Milliseconds(), SubCalls: audit.subCalls, SubInvalid: audit.subInvalid,
		IdeasFailed: audit.ideasFailed,
	}
	if err != nil {
		r.Error = err.Error()
		r.Outcome = classifyLaneError(err)
		return r
	}
	switch got, want := verdictRank[r.Verdict], verdictRank[c.Label]; {
	case got == want:
		r.Outcome = bankHit
	case got > want:
		r.Outcome = bankFalseHit
	default:
		r.Outcome = bankFalseMiss
	}
	return r
}

// classifyLaneError separa lo que es del modelo (un veredicto inválido, una salida
// que no parsea) de lo que no lo es (red, timeout, servidor). El proveedor local no
// tipa sus errores de parseo (internal/llm, fuera de esta ficha): se reconocen por
// su texto, «no parseable», y por llm.ErrLLMQuality.
func classifyLaneError(err error) string {
	if errors.Is(err, processor.ErrInvalidVerdict) || errors.Is(err, llm.ErrLLMQuality) ||
		strings.Contains(err.Error(), "no parseable") {
		return bankInvalid
	}
	return bankProviderError
}

// auditProvider cuenta las sub-llamadas de los carriles de lista (pares) y de
// criterios. Un sub-veredicto fuera de correct|incorrect hoy cuenta como «no» sin
// dejar rastro (shortanswer y openended solo miran si es correct): aquí se cuenta
// aparte, que es lo que la HU-INF.5 necesita ver.
type auditProvider struct {
	llm.LLMProvider
	subCalls    int
	subInvalid  int
	ideasFailed bool
}

func (a *auditProvider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	res, err := a.LLMProvider.JudgePairEquivalence(ctx, req)
	a.countSub(res, err)
	return res, err
}

func (a *auditProvider) CheckCriterion(ctx context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	res, err := a.LLMProvider.CheckCriterion(ctx, req)
	a.countSub(res, err)
	return res, err
}

func (a *auditProvider) ExtractIdeas(ctx context.Context, req llm.ExtractIdeasRequest) ([]string, error) {
	ideas, err := a.LLMProvider.ExtractIdeas(ctx, req)
	if err != nil || len(ideas) == 0 {
		a.ideasFailed = true
	}
	return ideas, err
}

func (a *auditProvider) countSub(res llm.ReviewResult, err error) {
	a.subCalls++
	if err == nil && res.Verdict != llm.VerdictCorrect && res.Verdict != llm.VerdictIncorrect {
		a.subInvalid++
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// bankRunRow es una fila de la tabla del modelo, por categoría.
type bankRunRow struct {
	Kind          string `json:"kind"`
	Category      string `json:"category"`
	Arrive        int    `json:"arrive"`
	Hits          int    `json:"hits"`
	FalseHits     int    `json:"false_hits"`
	FalseMisses   int    `json:"false_misses"`
	Invalid       int    `json:"invalid"`
	ProviderError int    `json:"provider_errors"`
	SubCalls      int    `json:"sub_calls"`
	SubInvalid    int    `json:"sub_invalid"`
}

// bankTable agrupa los casos por categoría, en el orden en que aparecen.
func bankTable(runs []bankCaseRun) []bankRunRow {
	var order []string
	rows := map[string]*bankRunRow{}
	for _, r := range runs {
		row, ok := rows[r.Category]
		if !ok {
			row = &bankRunRow{Kind: r.Kind, Category: r.Category}
			rows[r.Category] = row
			order = append(order, r.Category)
		}
		row.Arrive++
		row.SubCalls += r.SubCalls
		row.SubInvalid += r.SubInvalid
		switch r.Outcome {
		case bankHit:
			row.Hits++
		case bankFalseHit:
			row.FalseHits++
		case bankFalseMiss:
			row.FalseMisses++
		case bankInvalid:
			row.Invalid++
		case bankProviderError:
			row.ProviderError++
		}
	}
	out := make([]bankRunRow, 0, len(order))
	for _, cat := range order {
		out = append(out, *rows[cat])
	}
	return out
}

func bankRunMarkdown(info runInfo, bankSum string, bankCases int, table []bankRunRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Modelo: %s (digest %s, Ollama %s), temperatura %.0f. Worker %s (árbol sucio: %v).\n",
		info.Model, orDash(info.ModelDigest), orDash(info.OllamaVersion), info.Temperature, info.WorkerCommit, info.WorkerDirty)
	fmt.Fprintf(&b, "Banco: sha256 %s, %d casos.\n\n", bankSum, bankCases)
	b.WriteString("| Tipo | Categoría | Llegan | Aciertos | Falsos aciertos | Falsos rechazos | Inválidas | Errores del proveedor | Sub-llamadas | Sub-veredictos inválidos |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	var t bankRunRow
	for _, r := range table {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d | %d | %d |\n",
			r.Kind, r.Category, r.Arrive, r.Hits, r.FalseHits, r.FalseMisses, r.Invalid, r.ProviderError, r.SubCalls, r.SubInvalid)
		t.Arrive += r.Arrive
		t.Hits += r.Hits
		t.FalseHits += r.FalseHits
		t.FalseMisses += r.FalseMisses
		t.Invalid += r.Invalid
		t.ProviderError += r.ProviderError
		t.SubCalls += r.SubCalls
		t.SubInvalid += r.SubInvalid
	}
	fmt.Fprintf(&b, "| **total** | | %d | %d | %d | %d | %d | %d | %d | %d |\n",
		t.Arrive, t.Hits, t.FalseHits, t.FalseMisses, t.Invalid, t.ProviderError, t.SubCalls, t.SubInvalid)
	return b.String()
}
