package main

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/shortanswer"
)

// countingProvider decora un provider para contar las llamadas de par (equivalencia
// binaria) y de criterio. El conteo es comportamiento DETERMINISTA del worker (no
// depende del modelo): el arnés lo reporta junto al esperado.
type countingProvider struct {
	llm.LLMProvider
	pairCalls      int
	criterionCalls int
}

func (c *countingProvider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	c.pairCalls++
	return c.LLMProvider.JudgePairEquivalence(ctx, req)
}

func (c *countingProvider) CheckCriterion(ctx context.Context, req llm.CriterionCheckRequest) (llm.ReviewResult, error) {
	c.criterionCalls++
	return c.LLMProvider.CheckCriterion(ctx, req)
}

// saPrepCase es un caso del modo review-prep: una entrada del carril triturado más lo
// esperado. wantPairCalls es el número de llamadas de par que el worker debería hacer
// (determinista); wantVerdict, el veredicto recompuesto. Se mide, no se juzga.
type saPrepCase struct {
	name          string
	in            shortanswer.GradeInput
	wantVerdict   llm.Verdict
	wantPairCalls int
	note          string
}

func grandColombiaInput(student string) shortanswer.GradeInput {
	return shortanswer.GradeInput{
		QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
		StudentAnswer: student,
		Items:         []string{"ecuador", "venezuela", "colombia"},
		ItemsVerbatim: []string{"Ecuador", "Venezuela", "Colombia"},
	}
}

// saPrepCases cubre los cuatro escenarios del carril triturado tras la migración a
// textmatch (plan 045 F3): todo determinista (sin par), typo cercano rescatado por el
// fuzzy determinista (sin par), typo fuerte que escala a un par LLM, y falta un ítem
// sin candidato (sin par, sin adivinar).
var saPrepCases = []saPrepCase{
	{
		name:          "todo-determinista-sin-comas",
		in:            grandColombiaInput("Ecuador Venezuela y Colombia"),
		wantVerdict:   llm.VerdictCorrect,
		wantPairCalls: 0,
		note:          "los 3 ítems casan por palabra; ni una llamada al modelo",
	},
	{
		name:          "typo-rescatado-por-fuzzy",
		in:            grandColombiaInput("ecuador, benezuela y colombia"),
		wantVerdict:   llm.VerdictCorrect,
		wantPairCalls: 0,
		note:          "benezuela dista 1 de venezuela (sim 0.888 ≥ 0.85): lo rescata el fuzzy determinista, SIN llamada al modelo (plan 045)",
	},
	{
		name:          "typo-fuerte-escala-a-par",
		in:            grandColombiaInput("ecuador, benezuola y colombia"),
		wantVerdict:   llm.VerdictCorrect,
		wantPairCalls: 1,
		note:          "benezuola dista 2 de venezuela (sim 0.778 < 0.85): el fuzzy no alcanza, escala a 1 par binario; correct SOLO si el modelo lo rescata",
	},
	{
		name:          "falta-item-sin-candidato",
		in:            grandColombiaInput("ecuador y colombia"),
		wantVerdict:   llm.VerdictIncorrect,
		wantPairCalls: 0,
		note:          "venezuela falta y no hay fragmento sobrante: incorrect sin adivinar",
	},
}

// reviewPrepRun es lo medido de un caso del modo review-prep.
type reviewPrepRun struct {
	Battery   string  `json:"battery"`
	Name      string  `json:"name"`
	Verdict   string  `json:"verdict,omitempty"`
	Score     float64 `json:"score"`
	Calls     int     `json:"calls"`
	WantCalls int     `json:"want_calls"`
	Outcome   string  `json:"outcome"` // matches | differs | error
	Reason    string  `json:"reason,omitempty"`
	ElapsedMS int64   `json:"elapsed_ms"`
}

// runReviewPrep corre las dos baterías del carril de corrección con prep: el
// triturado de short_answer (F3d) y el de open_ended (F4c). Una corrida por caso;
// no termina con código != 0 por lo que diga el modelo (ADR 0048).
func runReviewPrep(base llm.LLMProvider, info runInfo, runsDir string, timeout time.Duration) {
	runs := runShortAnswerPrep(base, timeout)
	fmt.Printf("\n")
	runs = append(runs, runOpenEndedPrep(base, timeout)...)

	counts := map[string]int{}
	for _, r := range runs {
		counts[r.Outcome]++
	}
	fmt.Printf("\nRESULTADO : %d casos · coinciden %d · difieren %d · errores %d (una corrida por caso)\n",
		len(runs), counts["matches"], counts["differs"], counts["error"])
	saveRun(runsDir, info, runs)
}

// measurePrepCase decide el desenlace de un caso del carril con prep: coincide si
// el veredicto y el número de llamadas son los esperados.
func measurePrepCase(r *reviewPrepRun, wantVerdict llm.Verdict, err error) {
	switch {
	case err != nil:
		r.Outcome, r.Reason = "error", err.Error()
	case r.Calls != r.WantCalls:
		r.Outcome, r.Reason = "differs", fmt.Sprintf("llamadas %d, se esperaban %d", r.Calls, r.WantCalls)
	case llm.Verdict(r.Verdict) != wantVerdict:
		r.Outcome, r.Reason = "differs", fmt.Sprintf("verdict %s, se esperaba %s", r.Verdict, wantVerdict)
	default:
		r.Outcome = "matches"
	}
}

// runShortAnswerPrep corre la batería del carril triturado (F3d) contra el provider
// real y mide, por caso, el número de llamadas de par y el veredicto recompuesto.
func runShortAnswerPrep(base llm.LLMProvider, timeout time.Duration) []reviewPrepRun {
	fmt.Printf("== llm-harness (review-prep · carril triturado short_answer F3c/F3d) ==\n")
	fmt.Printf("provider : %s\n", base.Name())
	fmt.Printf("casos    : %d\n\n", len(saPrepCases))

	runs := make([]reviewPrepRun, 0, len(saPrepCases))
	for _, tc := range saPrepCases {
		prov := &countingProvider{LLMProvider: base}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		res, err := shortanswer.Grade(ctx, prov, tc.in)
		elapsed := time.Since(start)
		cancel()

		r := reviewPrepRun{Battery: "short_answer", Name: tc.name, Verdict: string(res.Verdict), Score: res.Score,
			Calls: prov.pairCalls, WantCalls: tc.wantPairCalls, ElapsedMS: elapsed.Milliseconds()}
		measurePrepCase(&r, tc.wantVerdict, err)
		runs = append(runs, r)

		fmt.Printf("%-8s %-28s (%s)\n", r.Outcome, tc.name, elapsed.Round(time.Millisecond))
		fmt.Printf("        verdict=%s score=%.2f pares=%d  esperado: verdict=%s pares=%d\n",
			res.Verdict, res.Score, prov.pairCalls, tc.wantVerdict, tc.wantPairCalls)
		if r.Reason != "" {
			fmt.Printf("        motivo  : %s\n", r.Reason)
		}
		fmt.Printf("        nota    : %s\n", tc.note)
	}
	return runs
}
