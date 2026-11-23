package main

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/openended"
)

// runOpenEndedPrep corre la batería open_ended con prep (F4c). Dos tipos de caso:
//   - enrich: mide el prompt GLOBAL con vs sin prep (F4a). El caso es el que motivó
//     esto: el alumno SÍ incluye Venezuela y el prep lo lista en main_ideas; se corre
//     también sin prep para contrastar.
//   - criteria: mide el carril por criterios (F4b) contra el provider real, con el
//     conteo de llamadas (una por criterio) y el veredicto/score agregados en Go.
//
// Una corrida por caso; se mide, no se juzga (ADR 0048).
func runOpenEndedPrep(base llm.LLMProvider, timeout time.Duration) []reviewPrepRun {
	fmt.Printf("== llm-harness (review-prep · carril open_ended con prep F4a/F4b/F4c) ==\n")
	fmt.Printf("provider : %s\n\n", base.Name())

	runs := runOpenEndedEnrich(base, timeout)
	fmt.Printf("\n")
	return append(runs, runOpenEndedCriteria(base, timeout)...)
}

// --- F4a: enriquecimiento del prompt global (con vs sin prep) ---

// venezuelaPrep lista Venezuela entre las ideas esperadas y como variante válida: es
// justo la información que evita el falso «omite Venezuela».
var venezuelaPrep = &llm.ReviewPrep{
	QuestionIntent: "medir si el alumno identifica los países que formaron la Gran Colombia",
	MainIdeas:      []string{"menciona Ecuador", "menciona Venezuela", "menciona Colombia"},
	ValidVariants:  []string{"la Gran Colombia la formaron Ecuador, Venezuela y Colombia"},
}

func runOpenEndedEnrich(base llm.LLMProvider, timeout time.Duration) []reviewPrepRun {
	fmt.Printf("-- F4a: prompt global open_ended con vs sin prep (caso Venezuela) --\n")

	req := llm.ReviewRequest{
		QuestionType:   llm.QuestionTypeOpenEnded,
		QuestionText:   "¿Qué países formaron la Gran Colombia? Explica brevemente.",
		ExpectedAnswer: "Ecuador, Venezuela y Colombia",
		StudentAnswer:  "La Gran Colombia estuvo formada por Ecuador, Venezuela y Colombia.",
		Language:       "es",
	}

	runs := make([]reviewPrepRun, 0, 2)
	for _, variant := range []struct {
		name string
		prep *llm.ReviewPrep
	}{{"sin-prep-venezuela", nil}, {"con-prep-venezuela", venezuelaPrep}} {
		req.Prep = variant.prep
		start := time.Now()
		res, err := runOneReview(base, req, timeout)
		r := reviewPrepRun{Battery: "open_ended_enrich", Name: variant.name, Verdict: string(res.Verdict),
			Score: res.Score, Calls: 1, WantCalls: 1, ElapsedMS: time.Since(start).Milliseconds()}
		// El alumno nombra los tres países: lo esperado es correct, con o sin prep.
		measurePrepCase(&r, llm.VerdictCorrect, err)
		runs = append(runs, r)
		fmt.Printf("  %-8s %-20s verdict=%s score=%.2f  feedback: %s\n",
			r.Outcome, variant.name, res.Verdict, res.Score, truncate(res.Feedback, 100))
	}
	return runs
}

// runOneReview corre un ReviewAnswer aislado (para el contraste con/sin prep).
func runOneReview(p llm.LLMProvider, req llm.ReviewRequest, timeout time.Duration) (llm.ReviewResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.ReviewAnswer(ctx, req)
}

// --- F4b: carril por criterios (conteo duro de llamadas) ---

type oeCriteriaCase struct {
	name        string
	in          openended.GradeInput
	wantVerdict llm.Verdict
	wantCalls   int
	scoreMin    float64
	scoreMax    float64
	note        string
}

var oeCriteriaCases = []oeCriteriaCase{
	{
		name: "fotosintesis-cumple-2-de-3",
		in: openended.GradeInput{
			QuestionText:   "Explica el proceso de la fotosíntesis.",
			ExpectedAnswer: "En los cloroplastos, la planta usa luz, agua y CO2 para producir glucosa y liberar oxígeno.",
			// El alumno menciona la luz y el oxígeno, pero NO los cloroplastos → 2 de 3.
			StudentAnswer: "Las plantas usan la luz del sol para producir su alimento y liberan oxígeno al aire.",
			Criteria: []string{
				"menciona que ocurre en los cloroplastos",
				"menciona que usa la luz",
				"menciona que libera oxígeno",
			},
			Language: "es",
		},
		wantVerdict: llm.VerdictPartial,
		wantCalls:   3,
		scoreMin:    0.3,
		scoreMax:    0.7,
		note:        "alumno cumple 2 de 3; se esperan 3 llamadas (determinista) y verdict partial",
	},
}

func runOpenEndedCriteria(base llm.LLMProvider, timeout time.Duration) []reviewPrepRun {
	fmt.Printf("-- F4b: carril por criterios (una llamada binaria por criterio) --\n")

	runs := make([]reviewPrepRun, 0, len(oeCriteriaCases))
	for _, tc := range oeCriteriaCases {
		prov := &countingProvider{LLMProvider: base}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		res, err := openended.Grade(ctx, prov, tc.in)
		elapsed := time.Since(start)
		cancel()

		r := reviewPrepRun{Battery: "open_ended_criteria", Name: tc.name, Verdict: string(res.Verdict), Score: res.Score,
			Calls: prov.criterionCalls, WantCalls: tc.wantCalls, ElapsedMS: elapsed.Milliseconds()}
		measurePrepCase(&r, tc.wantVerdict, err)
		if r.Outcome == "matches" && (res.Score < tc.scoreMin || res.Score > tc.scoreMax) {
			r.Outcome, r.Reason = "differs", fmt.Sprintf("score %.2f fuera de [%.2f,%.2f]", res.Score, tc.scoreMin, tc.scoreMax)
		}
		runs = append(runs, r)

		fmt.Printf("  %-8s %-28s (%s)\n", r.Outcome, tc.name, elapsed.Round(time.Millisecond))
		fmt.Printf("        verdict=%s score=%.2f llamadas=%d  esperado: verdict=%s llamadas=%d score∈[%.2f,%.2f]\n",
			res.Verdict, res.Score, prov.criterionCalls, tc.wantVerdict, tc.wantCalls, tc.scoreMin, tc.scoreMax)
		if r.Reason != "" {
			fmt.Printf("        motivo  : %s\n", r.Reason)
		}
		fmt.Printf("        nota    : %s\n", tc.note)
	}
	return runs
}
