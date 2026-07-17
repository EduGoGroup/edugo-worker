package main

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/openended"
)

// runOpenEndedPrep corre la batería open_ended con prep (F4c). Dos tipos de caso:
//   - enrich: mide el prompt GLOBAL con vs sin prep (F4a). El caso obligatorio es el
//     que motivó esto: el alumno SÍ incluye Venezuela y el prep lo lista en main_ideas
//     — se comprueba si el prep evita el falso «omite Venezuela» que qwen 1.7b alucinó
//     en 040 F4-UI (se corre también sin prep para contrastar).
//   - criteria: mide el carril por criterios (F4b) contra el provider real, con conteo
//     DURO de llamadas (una por criterio) y el veredicto/score agregados en Go.
//
// Devuelve true si algún caso no-flaky falló.
func runOpenEndedPrep(base llm.LLMProvider, timeout time.Duration) bool {
	fmt.Printf("== llm-harness (review-prep · carril open_ended con prep F4a/F4b/F4c) ==\n")
	fmt.Printf("provider : %s\n\n", base.Name())

	hardFail := false
	hardFail = runOpenEndedEnrich(base, timeout) || hardFail
	fmt.Printf("\n")
	hardFail = runOpenEndedCriteria(base, timeout) || hardFail
	return hardFail
}

// --- F4a: enriquecimiento del prompt global (con vs sin prep) ---

// venezuelaPrep lista Venezuela entre las ideas esperadas y como variante válida: es
// justo la información que evita el falso «omite Venezuela».
var venezuelaPrep = &llm.ReviewPrep{
	QuestionIntent: "medir si el alumno identifica los países que formaron la Gran Colombia",
	MainIdeas:      []string{"menciona Ecuador", "menciona Venezuela", "menciona Colombia"},
	ValidVariants:  []string{"la Gran Colombia la formaron Ecuador, Venezuela y Colombia"},
}

func runOpenEndedEnrich(base llm.LLMProvider, timeout time.Duration) bool {
	fmt.Printf("-- F4a: prompt global open_ended con vs sin prep (caso Venezuela) --\n")

	req := llm.ReviewRequest{
		QuestionType:   llm.QuestionTypeOpenEnded,
		QuestionText:   "¿Qué países formaron la Gran Colombia? Explica brevemente.",
		ExpectedAnswer: "Ecuador, Venezuela y Colombia",
		StudentAnswer:  "La Gran Colombia estuvo formada por Ecuador, Venezuela y Colombia.",
		Language:       "es",
	}

	// Sin prep (contraste; su veredicto es informativo, no cuenta contra la meta).
	sinPrep := runOneReview(base, req, timeout)
	fmt.Printf("  sin prep : verdict=%s score=%.2f  feedback: %s\n",
		sinPrep.Verdict, sinPrep.Score, truncate(sinPrep.Feedback, 100))

	// Con prep: el alumno SÍ nombra Venezuela y el prep la lista → debe ser correct.
	req.Prep = venezuelaPrep
	conPrep := runOneReview(base, req, timeout)
	fmt.Printf("  con prep : verdict=%s score=%.2f  feedback: %s\n",
		conPrep.Verdict, conPrep.Score, truncate(conPrep.Feedback, 100))

	// El veredicto de un 1.7b es flaky: el caso mide el prompt, no garantiza el modelo.
	if conPrep.Verdict == llm.VerdictCorrect {
		fmt.Printf("  PASS  con-prep-venezuela [verdict known-flaky]: el prep sostuvo correct\n")
	} else {
		fmt.Printf("  FLAKY con-prep-venezuela [verdict known-flaky]: el modelo no dio correct (verdict %s); "+
			"el prompt lleva la info de Venezuela, la corrección real usa un modelo mejor (D-042.8)\n", conPrep.Verdict)
	}
	// Nunca es fallo duro: es una medición comparativa del prompt con modelo chico.
	return false
}

// runOneReview corre un ReviewAnswer aislado (para el contraste con/sin prep).
func runOneReview(p llm.LLMProvider, req llm.ReviewRequest, timeout time.Duration) llm.ReviewResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	res, err := p.ReviewAnswer(ctx, req)
	if err != nil {
		return llm.ReviewResult{Verdict: "error", Feedback: err.Error()}
	}
	return res
}

// --- F4b: carril por criterios (conteo duro de llamadas) ---

type oeCriteriaCase struct {
	name         string
	in           openended.GradeInput
	wantVerdict  llm.Verdict
	wantCalls    int
	scoreMin     float64
	scoreMax     float64
	verdictFlaky bool
	note         string
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
		wantVerdict:  llm.VerdictPartial,
		wantCalls:    3,
		scoreMin:     0.3,
		scoreMax:     0.7,
		verdictFlaky: true,
		note:         "alumno cumple 2 de 3; conteo de llamadas duro (=3), verdict partial depende del modelo",
	},
}

func runOpenEndedCriteria(base llm.LLMProvider, timeout time.Duration) bool {
	fmt.Printf("-- F4b: carril por criterios (una llamada binaria por criterio) --\n")
	hardFail := false

	for _, tc := range oeCriteriaCases {
		prov := &countingProvider{LLMProvider: base}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		res, err := openended.Grade(ctx, prov, tc.in)
		elapsed := time.Since(start)
		cancel()

		flakyTag := ""
		if tc.verdictFlaky {
			flakyTag = " [verdict known-flaky]"
		}
		if err != nil {
			fmt.Printf("  FAIL  %-28s (%s)%s\n        error: %v\n", tc.name, elapsed.Round(time.Millisecond), flakyTag, err)
			hardFail = true
			continue
		}

		callsOK := prov.criterionCalls == tc.wantCalls
		verdictOK := res.Verdict == tc.wantVerdict
		scoreOK := res.Score >= tc.scoreMin && res.Score <= tc.scoreMax

		status := "PASS"
		if !callsOK || (!verdictOK && !tc.verdictFlaky) {
			status = "FAIL"
		}
		fmt.Printf("  %-5s %-28s (%s)%s\n", status, tc.name, elapsed.Round(time.Millisecond), flakyTag)
		fmt.Printf("        verdict=%s score=%.2f llamadas=%d  esperado: verdict=%s llamadas=%d score∈[%.2f,%.2f]\n",
			res.Verdict, res.Score, prov.criterionCalls, tc.wantVerdict, tc.wantCalls, tc.scoreMin, tc.scoreMax)
		fmt.Printf("        feedback: %s\n        nota    : %s\n", truncate(res.Feedback, 120), tc.note)

		if !callsOK {
			fmt.Printf("        motivo FAIL: llamadas %d != esperado %d (determinista, duro)\n", prov.criterionCalls, tc.wantCalls)
			hardFail = true
			continue
		}
		if !verdictOK {
			if tc.verdictFlaky {
				fmt.Printf("        nota flaky : el modelo evaluó distinto los criterios esta corrida (verdict %s)\n", res.Verdict)
			} else {
				fmt.Printf("        motivo FAIL: verdict %s != esperado %s\n", res.Verdict, tc.wantVerdict)
				hardFail = true
				continue
			}
		}
		// El score sí lo verificamos como coherencia interna del anclaje cuando el
		// veredicto salió como se esperaba (agregación determinista en Go).
		if verdictOK && !scoreOK {
			fmt.Printf("        motivo FAIL: score %.2f fuera de [%.2f,%.2f] (anclaje determinista)\n", res.Score, tc.scoreMin, tc.scoreMax)
			hardFail = true
		}
	}
	return hardFail
}
