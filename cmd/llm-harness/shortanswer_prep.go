package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/shortanswer"
)

// countingProvider decora un provider para contar las llamadas de par (equivalencia
// binaria). El conteo es comportamiento DETERMINISTA del worker (no depende del
// modelo), así que el harness lo verifica exacto aunque el veredicto sea flaky.
type countingProvider struct {
	llm.LLMProvider
	pairCalls int
}

func (c *countingProvider) JudgePairEquivalence(ctx context.Context, req llm.PairEquivalenceRequest) (llm.ReviewResult, error) {
	c.pairCalls++
	return c.LLMProvider.JudgePairEquivalence(ctx, req)
}

// saPrepCase es un caso del modo review-prep: una entrada del carril triturado más la
// expectativa. wantPairCalls fija el número EXACTO de llamadas de par que el worker
// debe hacer (determinista); wantVerdict es el veredicto recompuesto. verdictFlaky
// marca los casos cuyo veredicto depende de que el modelo rescate un par (un FAIL de
// veredicto no cuenta contra la meta, pero el conteo de pares SÍ es duro).
type saPrepCase struct {
	name          string
	in            shortanswer.GradeInput
	wantVerdict   llm.Verdict
	wantPairCalls int
	verdictFlaky  bool
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

// saPrepCases cubre los tres escenarios de F3d: todo determinista (sin par), rescate
// por un par (typo), y falta un ítem sin candidato (sin par, sin adivinar).
var saPrepCases = []saPrepCase{
	{
		name:          "todo-determinista-sin-comas",
		in:            grandColombiaInput("Ecuador Venezuela y Colombia"),
		wantVerdict:   llm.VerdictCorrect,
		wantPairCalls: 0,
		note:          "los 3 ítems casan por palabra; ni una llamada al modelo",
	},
	{
		name:          "typo-rescatado-por-un-par",
		in:            grandColombiaInput("ecuador, benezuela y colombia"),
		wantVerdict:   llm.VerdictCorrect,
		wantPairCalls: 1,
		verdictFlaky:  true,
		note:          "benezuela≡venezuela: 1 par binario; correct SOLO si el modelo lo rescata",
	},
	{
		name:          "falta-item-sin-candidato",
		in:            grandColombiaInput("ecuador y colombia"),
		wantVerdict:   llm.VerdictIncorrect,
		wantPairCalls: 0,
		note:          "venezuela falta y no hay fragmento sobrante: incorrect sin adivinar",
	},
}

// runShortAnswerPrep corre la batería del carril triturado (F3d) contra el provider
// real. Verifica DOS cosas por caso: el número exacto de llamadas de par (duro,
// determinista) y el veredicto recompuesto (flaky donde depende del rescate del
// modelo). Sale con código != 0 si algún caso NO-flaky falla en cualquiera de las dos.
func runShortAnswerPrep(base llm.LLMProvider, timeout time.Duration) {
	fmt.Printf("== llm-harness (review-prep, carril triturado short_answer F3c/F3d) ==\n")
	fmt.Printf("provider : %s\n", base.Name())
	fmt.Printf("casos    : %d\n\n", len(saPrepCases))

	pass, effectiveTotal := 0, 0
	hardFail := false

	for _, tc := range saPrepCases {
		prov := &countingProvider{LLMProvider: base}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		res, err := shortanswer.Grade(ctx, prov, tc.in)
		elapsed := time.Since(start)
		cancel()

		flakyTag := ""
		if tc.verdictFlaky {
			flakyTag = " [verdict known-flaky]"
		}

		if err != nil {
			fmt.Printf("FAIL  %-28s (%s)%s\n", tc.name, elapsed.Round(time.Millisecond), flakyTag)
			fmt.Printf("        error: %v\n", err)
			hardFail = true // un error de infra es duro aunque el veredicto sea flaky
			continue
		}

		// El conteo de pares es SIEMPRE duro (comportamiento determinista del worker).
		pairsOK := prov.pairCalls == tc.wantPairCalls
		verdictOK := res.Verdict == tc.wantVerdict

		// El caso cuenta contra la meta salvo que su veredicto sea flaky (el conteo de
		// pares nunca es flaky, pero lo agrupamos en el mismo total efectivo del caso).
		if !tc.verdictFlaky {
			effectiveTotal++
		}

		status := "PASS"
		if !pairsOK || (!verdictOK && !tc.verdictFlaky) {
			status = "FAIL"
		}
		fmt.Printf("%-5s %-28s (%s)%s\n", status, tc.name, elapsed.Round(time.Millisecond), flakyTag)
		fmt.Printf("        verdict=%s score=%.2f pares=%d  esperado: verdict=%s pares=%d\n",
			res.Verdict, res.Score, prov.pairCalls, tc.wantVerdict, tc.wantPairCalls)
		fmt.Printf("        feedback: %s\n", truncate(res.Feedback, 120))
		fmt.Printf("        nota    : %s\n", tc.note)

		if !pairsOK {
			fmt.Printf("        motivo FAIL: llamadas de par %d != esperado %d (determinista, duro)\n", prov.pairCalls, tc.wantPairCalls)
			hardFail = true
			continue
		}
		if !verdictOK {
			if tc.verdictFlaky {
				fmt.Printf("        nota flaky : el modelo no rescató el par esta corrida (verdict %s)\n", res.Verdict)
			} else {
				fmt.Printf("        motivo FAIL: verdict %s != esperado %s\n", res.Verdict, tc.wantVerdict)
				hardFail = true
				continue
			}
		}
		if !tc.verdictFlaky {
			pass++
		} else if verdictOK {
			fmt.Printf("        nota       : caso verdict-flaky PASÓ (el modelo rescató el par)\n")
		}
	}

	fmt.Printf("\nRESULTADO : %d/%d casos no-flaky en PASS (el conteo de pares es duro en todos)\n", pass, effectiveTotal)
	if hardFail {
		os.Exit(1)
	}
}
