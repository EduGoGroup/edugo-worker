// Package openended implementa el carril de corrección POR CRITERIOS de las preguntas
// abiertas cuando la pregunta trae un prep con criteria (plan 042 F4b, D-042.10). En
// vez de un juicio global del LLM, comprueba CADA criterio con una llamada binaria
// («¿la respuesta cumple X? correct|incorrect») y AGREGA el veredicto+score de forma
// DETERMINISTA en Go, anclada a las mismas escalas del prompt open_ended actual.
//
// La agregación (aggregate) es pura y unit-testeada aparte; solo las comprobaciones
// de criterio consultan al provider.
package openended

import (
	"context"
	"fmt"
	"strings"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// GradeInput es la entrada del carril por criterios: la pregunta y respuesta del
// alumno más los criterios verificables del prep (≥1).
type GradeInput struct {
	QuestionText   string
	ExpectedAnswer string
	StudentAnswer  string
	// Criteria son los criterios verificables del prep (open_ended). Una llamada
	// binaria por criterio.
	Criteria []string
	// Language del feedback (default "es").
	Language string
}

// Grade comprueba cada criterio con una llamada binaria y agrega el resultado de
// forma determinista. Hace EXACTAMENTE len(Criteria) llamadas al provider (una por
// criterio no vacío). Un error del provider en cualquier criterio se propaga
// (transitorio; el caller reintenta el intento completo, que es idempotente).
func Grade(ctx context.Context, provider llm.LLMProvider, in GradeInput) (llm.ReviewResult, error) {
	lang := in.Language
	if lang == "" {
		lang = "es"
	}

	var met, total int
	var unmet []string
	for _, c := range in.Criteria {
		crit := strings.TrimSpace(c)
		if crit == "" {
			continue
		}
		total++
		res, err := provider.CheckCriterion(ctx, llm.CriterionCheckRequest{
			QuestionText:   in.QuestionText,
			ExpectedAnswer: in.ExpectedAnswer,
			Criterion:      crit,
			StudentAnswer:  in.StudentAnswer,
			Language:       lang,
		})
		if err != nil {
			return llm.ReviewResult{}, fmt.Errorf("comprobando criterio %q: %w", crit, err)
		}
		if res.Verdict == llm.VerdictCorrect {
			met++
		} else {
			unmet = append(unmet, crit)
		}
	}

	return aggregate(met, total, unmet), nil
}

// aggregate recompone el veredicto+score global a partir de cuántos criterios se
// cumplieron (plan 042 F4b). Decisión de agregación (documentada): la proporción de
// criterios cumplidos se ancla a las escalas del prompt open_ended actual
// (incorrect 0.0–0.2 / partial 0.3–0.7 / correct 0.8–1.0):
//   - 0 cumplidos            → incorrect, score 0.0
//   - todos cumplidos        → correct,   score 1.0
//   - parcial (0<p<1)        → partial,   score 0.3 + 0.4·p (queda dentro de 0.3–0.7)
//
// Sin criterios (total==0) el carril no aplica; se devuelve incorrect/0 defensivo (el
// caller solo entra aquí con ≥1 criterio, pero no asumimos).
func aggregate(met, total int, unmet []string) llm.ReviewResult {
	if total == 0 {
		return llm.ReviewResult{
			Verdict:  llm.VerdictIncorrect,
			Score:    0.0,
			Feedback: "No hay criterios evaluables para esta pregunta.",
		}
	}

	switch met {
	case 0:
		return llm.ReviewResult{
			Verdict:  llm.VerdictIncorrect,
			Score:    0.0,
			Feedback: fmt.Sprintf("No se cumplió ninguno de los %d criterios esperados.", total),
		}
	case total:
		return llm.ReviewResult{
			Verdict:  llm.VerdictCorrect,
			Score:    1.0,
			Feedback: fmt.Sprintf("Respuesta correcta: cumple los %d criterios esperados.", total),
		}
	default:
		p := float64(met) / float64(total)
		score := 0.3 + 0.4*p
		return llm.ReviewResult{
			Verdict:  llm.VerdictPartial,
			Score:    score,
			Feedback: fmt.Sprintf("Respuesta parcial: cumple %d de %d criterios. Faltó: %s.", met, total, strings.Join(unmet, "; ")),
		}
	}
}
