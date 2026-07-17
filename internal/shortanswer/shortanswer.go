// Package shortanswer implementa el carril de corrección TRITURADO de respuestas
// cortas cuando la pregunta trae un prep con content_kind=list (plan 042 F3c,
// D-042.10). En vez de mandar los dos strings completos al LLM y confiar en un juicio
// global, matchea el CONJUNTO de ítems del prep contra la respuesta del alumno y solo
// escala al LLM —una llamada BINARIA por par— los ítems que el match determinista no
// resolvió. El veredicto se recompone en Go, binario como el contrato F4 del plan 040:
// todos los ítems presentes ⇒ correct/1.0; falta alguno ⇒ incorrect/0.0.
//
// Desde el plan 045 (F3) la comparación vive en el módulo compartido
// `edugo-shared/textmatch`: la fase determinista es un SetMatcher con cascada
// exacto→fuzzy (rescata typos de lo correcto —"whastapp"≈"whatsapp"— SIN gastar LLM),
// y el escalado orquesta las llamadas de par con las primitivas de textmatch para
// conservar el modelo de costo del worker (≤1 llamada por ítem faltante, D-045.5).
package shortanswer

import (
	"context"
	"fmt"
	"strings"

	"github.com/EduGoGroup/edugo-shared/textmatch"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// GradeInput es la entrada del carril triturado: la pregunta y respuesta del alumno
// más los ítems del prep (normalizados + verbatim). Items ya viene normalizado por el
// contrato; textmatch re-normaliza al comparar (no confía en que el LLM que produjo el
// prep cumplió al pie de la letra).
type GradeInput struct {
	QuestionText  string
	StudentAnswer string
	// Items son los elementos esperados normalizados (del prep, content_kind=list).
	Items []string
	// ItemsVerbatim son los mismos elementos tal cual los escribió el profesor; se
	// usan para el prompt del par (legible) y el feedback (qué faltó). Puede venir
	// desalineado/corto: verbatimAt cae al ítem normalizado si falta.
	ItemsVerbatim []string
	// Language del feedback (default "es").
	Language string
}

// llmPairStrategy adapta el LLMProvider del worker a la interfaz textmatch.Strategy
// (D-045.5): así el juicio semántico es "una estrategia más" (inyectada, DIP) sin que
// textmatch dependa de infra/red. NO se mete en la cascada del SetMatcher —eso
// dispararía el LLM por cada candidato de cada ítem—; el worker la invoca a mano en la
// fase de escalado, una vez por ítem faltante.
type llmPairStrategy struct {
	provider     llm.LLMProvider
	questionText string
	language     string
}

// Name identifica la estrategia (procedencia en Result.Strategy).
func (llmPairStrategy) Name() string { return "llm-pair" }

// Compare pide al modelo la equivalencia binaria del par y la mapea a un
// textmatch.Result: VerdictCorrect ⇒ Match/1.0, cualquier otro ⇒ NoMatch/0.0. El
// error del provider (transitorio) se propaga para que el caller reintente el intento.
func (s llmPairStrategy) Compare(ctx context.Context, expected, candidate string) (textmatch.Result, error) {
	res, err := s.provider.JudgePairEquivalence(ctx, llm.PairEquivalenceRequest{
		QuestionText: s.questionText,
		Expected:     expected,
		Candidate:    candidate,
		Language:     s.language,
	})
	if err != nil {
		return textmatch.Result{}, err
	}
	if res.Verdict == llm.VerdictCorrect {
		return textmatch.Result{Outcome: textmatch.OutcomeMatch, Confidence: 1.0, Evidence: res.Feedback, Strategy: "llm-pair"}, nil
	}
	return textmatch.Result{Outcome: textmatch.OutcomeNoMatch, Confidence: 0.0, Evidence: res.Feedback, Strategy: "llm-pair"}, nil
}

// Grade ejecuta el carril triturado y devuelve un ReviewResult BINARIO. Flujo en dos
// fases (D-045.5), preservando el costo del LLM:
//  1. Fase determinista: SetMatcher(Lenient) con cascada exacto→fuzzy cubre los ítems
//     que aparecen tal cual o con typos rescatables, SIN una sola llamada al modelo.
//  2. Fase de escalado: por cada ítem SIN cubrir se arma el pool de candidatos con los
//     tokens sobrantes, se elige el mejor por distancia de edición (solo rankea) y se
//     hace UNA llamada binaria al LLM. Sin candidato sobrante ⇒ el ítem falta, sin
//     llamada (no se adivina).
//  3. Recomposición: todos los ítems presentes ⇒ correct/1.0; falta alguno ⇒
//     incorrect/0.0 con feedback de qué faltó.
//
// Un error del provider en un par se propaga (transitorio, el caller reintenta).
func Grade(ctx context.Context, provider llm.LLMProvider, in GradeInput) (llm.ReviewResult, error) {
	lang := in.Language
	if lang == "" {
		lang = "es"
	}

	// Fase 1 — determinista (exacto + fuzzy), sin LLM. Policy Lenient: los sobrantes
	// del alumno ("el famoso") no penalizan; solo importa cubrir los ítems esperados.
	det := textmatch.NewCascade(textmatch.Exact{}, textmatch.NewFuzzy(0))
	rep, err := textmatch.NewSetMatcher(det, textmatch.PolicyLenient).MatchAnswer(ctx, in.Items, in.StudentAnswer)
	if err != nil {
		// Las estrategias deterministas nunca devuelven error; se propaga por defensa.
		return llm.ReviewResult{}, fmt.Errorf("match determinista de ítems: %w", err)
	}

	// Fase 2 — escalado LLM sobre los tokens sobrantes de la fase 1. rep.Leftover son
	// índices de token base no consumidos; con ellos se arma un pool de candidatos
	// (tokens + n-gramas hasta el ítem más largo) que se ofrece a los ítems faltantes.
	tokens := textmatch.SplitTokens(in.StudentAnswer)
	leftoverToks := make([]string, len(rep.Leftover))
	for k, idx := range rep.Leftover {
		leftoverToks[k] = tokens[idx]
	}
	maxItemLen := 1
	for _, item := range in.Items {
		if n := len(textmatch.SplitTokens(item)); n > maxItemLen {
			maxItemLen = n
		}
	}
	cands := textmatch.GenerateCandidates(leftoverToks, maxItemLen)
	usedCand := make([]bool, len(leftoverToks))
	pair := llmPairStrategy{provider: provider, questionText: in.QuestionText, language: lang}

	var missing []string // verbatim de los ítems ausentes (para el feedback)
	for i := range in.Items {
		if rep.Covered[i] {
			continue
		}
		best, ok := bestCandidate(textmatch.Normalize(in.Items[i]), cands, usedCand)
		if !ok {
			// Sin candidato sobrante que ofrecer: el ítem falta, sin gastar una llamada.
			missing = append(missing, verbatimAt(in, i))
			continue
		}
		r, err := pair.Compare(ctx, verbatimAt(in, i), best.Text)
		if err != nil {
			return llm.ReviewResult{}, fmt.Errorf("par equivalencia (ítem %q vs %q): %w", in.Items[i], best.Text, err)
		}
		if r.Outcome == textmatch.OutcomeMatch {
			// Marca los tokens del candidato usados para no reofrecerlos a otro ítem.
			for k := best.Start; k < best.End; k++ {
				usedCand[k] = true
			}
		} else {
			missing = append(missing, verbatimAt(in, i))
		}
	}

	if len(missing) == 0 {
		return llm.ReviewResult{
			Verdict:  llm.VerdictCorrect,
			Score:    1.0,
			Feedback: "Respuesta correcta: se identificaron todos los elementos esperados.",
		}, nil
	}
	return llm.ReviewResult{
		Verdict:  llm.VerdictIncorrect,
		Score:    0.0,
		Feedback: fmt.Sprintf("Respuesta incompleta: faltó mencionar %s.", strings.Join(missing, ", ")),
	}, nil
}

// bestCandidate devuelve el candidato sobrante (span no usado) más parecido al ítem por
// distancia de edición, o ok=false si no queda ninguno. Solo ELIGE a quién preguntar;
// la decisión de equivalencia la toma el LLM. Empate ⇒ el primero (los candidatos ya
// vienen en orden estable de GenerateCandidates) para que el carril sea determinista.
func bestCandidate(item string, cands []textmatch.Candidate, used []bool) (textmatch.Candidate, bool) {
	best := -1
	bestDist := -1
	for idx, c := range cands {
		if spanUsed(used, c) {
			continue
		}
		d := textmatch.EditDistance(item, c.Text)
		if best == -1 || d < bestDist {
			best = idx
			bestDist = d
		}
	}
	if best == -1 {
		return textmatch.Candidate{}, false
	}
	return cands[best], true
}

// spanUsed indica si algún token del rango del candidato ya fue consumido por otro ítem.
func spanUsed(used []bool, c textmatch.Candidate) bool {
	for k := c.Start; k < c.End; k++ {
		if used[k] {
			return true
		}
	}
	return false
}

// verbatimAt devuelve el verbatim del ítem i; si ItemsVerbatim viene corto o vacío en
// esa posición, cae al ítem normalizado (nunca devuelve vacío para no romper prompt/
// feedback).
func verbatimAt(in GradeInput, i int) string {
	if i < len(in.ItemsVerbatim) {
		if v := strings.TrimSpace(in.ItemsVerbatim[i]); v != "" {
			return v
		}
	}
	if i < len(in.Items) {
		return in.Items[i]
	}
	return ""
}
