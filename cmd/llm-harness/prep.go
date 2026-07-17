package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/questionprep"
)

// maxPrepAttempts es el tope de reintentos por caso ante un artefacto DEGENERADO
// (JSON vacío/no válido contra el contrato v1). Mismo motivo que el modo review:
// qwen3:1.7b con format:"json" devuelve de forma intermitente un `{}` que no valida.
// El reintento aísla esa lotería del modelo chico; NO reintenta un prep VÁLIDO que
// discrepe de lo esperado (eso sí mide el prompt).
const maxPrepAttempts = 6

// prepCase es un caso de la batería del modo prep: una petición de preparación más la
// expectativa contra la que se juzga PASS/FAIL. Todo prep debe validar contra el
// contrato v1 (D-042.2); encima se comprueban aserciones semánticas por caso.
type prepCase struct {
	name string
	req  llm.PrepRequest

	// check evalúa el prep YA validado contra el contrato. Devuelve (ok, motivo).
	check func(p questionprep.Prep) (bool, string)

	// flaky marca casos duros para un modelo chico: su FAIL no cuenta contra la meta.
	flaky     bool
	flakyNote string
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
		flaky:     true,
		flakyNote: "clasificar number + aislar la unidad exige comprensión; duro para 1.7B",
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
		flaky:     true,
		flakyNote: "derivar criterios verificables de una rúbrica es la tarea más exigente; duro para 1.7B",
	},
}

// runPrep corre la batería del modo prep contra el provider y reporta N/M. Los casos
// known-flaky que fallan no cuentan contra el total efectivo. Sale con código != 0 si
// algún caso NO-flaky falla.
func runPrep(p llm.LLMProvider, timeout time.Duration) {
	fmt.Printf("== llm-harness (prep) ==\n")
	fmt.Printf("provider : %s\n", p.Name())
	fmt.Printf("casos    : %d\n\n", len(prepCases))

	pass, effectiveTotal := 0, 0
	hardFail := false

	for _, tc := range prepCases {
		start := time.Now()
		var prep *questionprep.Prep
		var err error
		var valErr error
		attempts := 0
		for attempts < maxPrepAttempts {
			attempts++
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			raw, perr := p.PrepareQuestion(ctx, tc.req)
			cancel()
			if perr != nil {
				err = perr
				continue
			}
			err = nil
			prep, valErr = questionprep.Validate(raw, tc.req.QuestionType)
			// Reintenta solo ante artefacto degenerado (no valida); un prep válido que
			// discrepe de la aserción semántica NO se reintenta (mide el prompt).
			if valErr == nil {
				break
			}
		}
		elapsed := time.Since(start)

		retryTag := ""
		if attempts > 1 {
			retryTag = fmt.Sprintf(" [%d intentos]", attempts)
		}
		flakyTag := ""
		if tc.flaky {
			flakyTag = " [known-flaky]"
		} else {
			effectiveTotal++
		}

		if err != nil {
			fmt.Printf("FAIL  %-34s (%s)%s%s\n", tc.name, elapsed.Round(time.Millisecond), flakyTag, retryTag)
			fmt.Printf("        error: %v\n", err)
			if !tc.flaky {
				hardFail = true
			}
			continue
		}
		if valErr != nil {
			fmt.Printf("FAIL  %-34s (%s)%s%s\n", tc.name, elapsed.Round(time.Millisecond), flakyTag, retryTag)
			fmt.Printf("        contrato inválido: %v\n", valErr)
			if !tc.flaky {
				hardFail = true
			}
			continue
		}

		ok, reason := tc.check(*prep)
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("%-5s %-34s (%s)%s%s\n", status, tc.name, elapsed.Round(time.Millisecond), flakyTag, retryTag)
		fmt.Printf("        %s\n", summarizePrep(*prep))
		if !ok {
			fmt.Printf("        motivo FAIL: %s\n", reason)
			if tc.flaky {
				fmt.Printf("        nota flaky : %s\n", tc.flakyNote)
			} else {
				hardFail = true
			}
		}
		if ok && !tc.flaky {
			pass++
		} else if ok && tc.flaky {
			fmt.Printf("        nota       : caso known-flaky PASÓ esta corrida\n")
		}
	}

	fmt.Printf("\nRESULTADO : %d/%d casos no-flaky en PASS (los known-flaky no cuentan contra la meta)\n", pass, effectiveTotal)
	if hardFail {
		os.Exit(1)
	}
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
