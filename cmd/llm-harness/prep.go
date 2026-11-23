package main

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/questionprep"
)

// prepCase es un caso de la batería del modo prep: una petición de preparación más lo
// que se espera de ella; se mide si coincide, no se juzga (ADR 0048). Todo prep debe validar contra el
// contrato v1 (D-042.2); encima se comprueban aserciones semánticas por caso.
type prepCase struct {
	name string
	req  llm.PrepRequest

	// check evalúa el prep YA validado contra el contrato. Devuelve (ok, motivo).
	check func(p questionprep.Prep) (bool, string)
}

// prepCases cubre los tipos exigidos por F2d: la Gran Colombia (list, 3 ítems, sin
// comas en la canónica), un number con unidad, un term simple y un open_ended con
// explanation-rúbrica (criterios).
var prepCases = []prepCase{
	{
		name: "short-answer-lista-gran-colombia",
		req: llm.PrepRequest{
			QuestionType:  questionprep.QuestionTypeShortAnswer,
			QuestionText:  "¿Cuáles países formaron la Gran Colombia?",
			CorrectAnswer: "Ecuador, Venezuela y Colombia",
		},
		check: func(p questionprep.Prep) (bool, string) {
			if p.ContentKind != questionprep.ContentKindList {
				return false, fmt.Sprintf("content_kind=%q, esperado list", p.ContentKind)
			}
			if len(p.Items) != 3 {
				return false, fmt.Sprintf("items=%d, esperado 3 (Ecuador/Venezuela/Colombia)", len(p.Items))
			}
			return true, ""
		},
	},
	{
		name: "short-answer-number-unidad",
		req: llm.PrepRequest{
			QuestionType:  questionprep.QuestionTypeShortAnswer,
			QuestionText:  "¿Cuál es la distancia media de la Tierra al Sol?",
			CorrectAnswer: "150 millones de km",
		},
		check: func(p questionprep.Prep) (bool, string) {
			if p.ContentKind != questionprep.ContentKindNumber {
				return false, fmt.Sprintf("content_kind=%q, esperado number", p.ContentKind)
			}
			if p.Unit == nil || *p.Unit == "" {
				return false, "unit vacío, esperado la unidad (km)"
			}
			return true, ""
		},
	},
	{
		name: "short-answer-term",
		req: llm.PrepRequest{
			QuestionType:  questionprep.QuestionTypeShortAnswer,
			QuestionText:  "¿Qué pigmento da el color verde a las plantas?",
			CorrectAnswer: "Clorofila",
		},
		check: func(p questionprep.Prep) (bool, string) {
			if p.ContentKind != questionprep.ContentKindTerm && p.ContentKind != questionprep.ContentKindFree {
				return false, fmt.Sprintf("content_kind=%q, esperado term (o free)", p.ContentKind)
			}
			if len(p.Items) != 1 {
				return false, fmt.Sprintf("items=%d, esperado 1", len(p.Items))
			}
			return true, ""
		},
	},
	{
		name: "open-ended-rubrica-criterios",
		req: llm.PrepRequest{
			QuestionType: questionprep.QuestionTypeOpenEnded,
			QuestionText: "Explica el proceso de la fotosíntesis.",
			Explanation:  "Rúbrica: (1) menciona que ocurre en los cloroplastos; (2) explica que se convierte luz, agua y CO2; (3) indica que se libera oxígeno.",
		},
		check: func(p questionprep.Prep) (bool, string) {
			if len(p.MainIdeas) < 1 {
				return false, "main_ideas vacío"
			}
			if len(p.Criteria) < 1 {
				return false, "criteria vacío, esperado derivar de la rúbrica"
			}
			return true, ""
		},
	},
}

// prepRun es lo medido de un caso del modo prep.
type prepRun struct {
	Name      string `json:"name"`
	Outcome   string `json:"outcome"` // matches | differs | invalid | error
	Reason    string `json:"reason,omitempty"`
	Summary   string `json:"summary,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// runPrep corre la batería del modo prep contra el provider, una llamada por caso.
// Un artefacto que no valida contra el contrato v1 es una salida inválida (un
// dato), no un motivo para repetir ni para salir con código != 0 (ADR 0048).
func runPrep(p llm.LLMProvider, info runInfo, runsDir string, timeout time.Duration) {
	fmt.Printf("== llm-harness (prep) ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("casos    : %d\n\n", len(prepCases))

	counts := map[string]int{}
	runs := make([]prepRun, 0, len(prepCases))
	for _, tc := range prepCases {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		raw, err := p.PrepareQuestion(ctx, tc.req)
		elapsed := time.Since(start)
		cancel()

		r := prepRun{Name: tc.name, ElapsedMS: elapsed.Milliseconds()}
		if err != nil {
			r.Outcome, r.Reason = "error", err.Error()
		} else if prep, valErr := questionprep.Validate(raw, tc.req.QuestionType); valErr != nil {
			r.Outcome, r.Reason = "invalid", "contrato inválido: "+valErr.Error()
		} else {
			r.Summary = summarizePrep(*prep)
			if ok, reason := tc.check(*prep); ok {
				r.Outcome = "matches"
			} else {
				r.Outcome, r.Reason = "differs", reason
			}
		}
		counts[r.Outcome]++
		runs = append(runs, r)

		fmt.Printf("%-8s %-34s (%s)\n", r.Outcome, tc.name, elapsed.Round(time.Millisecond))
		if r.Summary != "" {
			fmt.Printf("        %s\n", r.Summary)
		}
		if r.Reason != "" {
			fmt.Printf("        motivo: %s\n", r.Reason)
		}
	}

	fmt.Printf("\nRESULTADO : %d casos · coinciden %d · difieren %d · inválidas %d · errores %d (una corrida por caso)\n",
		len(prepCases), counts["matches"], counts["differs"], counts["invalid"], counts["error"])
	saveRun(runsDir, info, runs)
}

// summarizePrep imprime una línea compacta del artefacto para inspección.
func summarizePrep(p questionprep.Prep) string {
	if p.QuestionType == questionprep.QuestionTypeShortAnswer {
		unit := "null"
		if p.Unit != nil {
			unit = *p.Unit
		}
		return fmt.Sprintf("content_kind=%s items=%v unit=%s", p.ContentKind, p.Items, unit)
	}
	return fmt.Sprintf("intent=%q main_ideas=%d secondary=%d variants=%d criteria=%d",
		truncate(p.QuestionIntent, 60), len(p.MainIdeas), len(p.SecondaryIdeas), len(p.ValidVariants), len(p.Criteria))
}
