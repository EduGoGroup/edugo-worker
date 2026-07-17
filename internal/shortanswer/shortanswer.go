// Package shortanswer implementa el carril de corrección TRITURADO de respuestas
// cortas cuando la pregunta trae un prep con content_kind=list (plan 042 F3c,
// D-042.10). En vez de mandar los dos strings completos al LLM y confiar en un juicio
// global, descompone la respuesta del alumno en fragmentos, los casa de forma
// DETERMINISTA contra los ítems del prep y solo escala al LLM —una llamada BINARIA
// por par— los ítems que el match no resolvió. El veredicto se recompone en Go,
// binario como el contrato F4 del plan 040: todos los ítems presentes ⇒ correct/1.0;
// falta alguno ⇒ incorrect/0.0.
//
// La parte pura (Split, matchItems, la recomposición) no toca red ni LLM y es
// unit-testeada aparte; solo los pares residuales consultan al provider.
package shortanswer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// GradeInput es la entrada del carril triturado: la pregunta y respuesta del alumno
// más los ítems del prep (normalizados + verbatim). Items ya viene normalizado por el
// contrato, pero Grade lo re-normaliza defensivamente (no confía en que el LLM que
// produjo el prep cumplió al pie de la letra).
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

// splitPattern parte la respuesta del alumno por los separadores del contrato
// (D-042.10): coma, barra, punto y coma y las conjunciones « y » / « e » (con
// límites de espacio para no cortar palabras como «hoy» o «ley»).
var splitPattern = regexp.MustCompile(`(?i)\s*(?:,|\||;|\s+y\s+|\s+e\s+)\s*`)

// Split trocea y normaliza la respuesta del alumno en fragmentos. Determinista y
// puro: minúsculas, sin tildes, trim, sin fragmentos vacíos. Un fragmento puede
// contener varias palabras (p. ej. «estados unidos» o «ecuador venezuela» cuando el
// alumno separó solo con espacios) — el match por palabra lo resuelve.
func Split(raw string) []string {
	parts := splitPattern.Split(raw, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		n := normalize(p)
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// accentMap mapea las vocales acentuadas y la diéresis del español a su forma sin
// tilde. Se conserva la «ñ» a propósito (es letra propia, no una tilde): «año» no es
// «ano». Coincide con la normalización «minúsculas, sin tildes» del contrato prep.
var accentMap = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a', 'ã': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
}

// normalize baja a minúsculas, quita tildes/diéresis, colapsa espacios internos y
// hace trim. Es la clave de comparación tanto de los fragmentos del alumno como de
// los ítems del prep, para que el match no dependa de mayúsculas ni acentos.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if rep, ok := accentMap[r]; ok {
			return rep
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// matchResult es el resultado del match determinista: qué ítems quedaron cubiertos y
// qué fragmentos del alumno se consumieron al cubrirlos (los NO consumidos quedan
// como candidatos para los pares residuales).
type matchResult struct {
	itemMatched []bool // por índice de ítem
	fragUsed    []bool // por índice de fragmento del alumno
}

// matchItems casa cada ítem del prep (normalizado) contra la respuesta del alumno de
// forma DETERMINISTA: un ítem está presente si aparece como palabra(s) completa(s) en
// el conjunto de fragmentos. Es puro y no llama al LLM. Marca como usados los
// fragmentos que contienen un ítem casado, para no reofrecerlos como candidatos.
func matchItems(items, frags []string) matchResult {
	res := matchResult{
		itemMatched: make([]bool, len(items)),
		fragUsed:    make([]bool, len(frags)),
	}
	// El texto normalizado del alumno es la unión de sus fragmentos: así un ítem de
	// una sola palabra casa aunque el alumno lo haya metido en un fragmento con más
	// palabras («ecuador venezuela» separado solo por espacios cubre ambos ítems).
	joined := strings.Join(frags, " ")
	for i, item := range items {
		item = normalize(item)
		if item == "" || !containsWords(joined, item) {
			continue
		}
		res.itemMatched[i] = true
		for j, f := range frags {
			if !res.fragUsed[j] && containsWords(f, item) {
				res.fragUsed[j] = true
			}
		}
	}
	return res
}

// containsWords reporta si needle aparece en haystack como secuencia de palabras
// completa (no como subcadena a media palabra). Ambos deben venir normalizados. El
// padding con espacios evita que «uba» case dentro de «cuba».
func containsWords(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(" "+haystack+" ", " "+needle+" ")
}

// Grade ejecuta el carril triturado y devuelve un ReviewResult BINARIO. Flujo:
//  1. Split determinista de la respuesta del alumno.
//  2. Match determinista contra los ítems del prep.
//  3. Por cada ítem SIN casar, elige el mejor candidato sobrante del alumno y hace
//     UNA llamada binaria al LLM (par). Sin candidato sobrante ⇒ el ítem falta, sin
//     llamada.
//  4. Recompone: todos los ítems presentes ⇒ correct/1.0; falta alguno ⇒
//     incorrect/0.0 con feedback de qué faltó.
//
// Un error del provider en un par se propaga (transitorio, el caller reintenta).
func Grade(ctx context.Context, provider llm.LLMProvider, in GradeInput) (llm.ReviewResult, error) {
	lang := in.Language
	if lang == "" {
		lang = "es"
	}
	frags := Split(in.StudentAnswer)
	match := matchItems(in.Items, frags)

	var missing []string // verbatim de los ítems ausentes (para el feedback)
	for i := range in.Items {
		if match.itemMatched[i] {
			continue
		}
		best := bestCandidate(normalize(in.Items[i]), frags, match.fragUsed)
		if best < 0 {
			// Sin fragmento sobrante que ofrecer: el ítem falta, sin gastar una llamada.
			missing = append(missing, verbatimAt(in, i))
			continue
		}
		res, err := provider.JudgePairEquivalence(ctx, llm.PairEquivalenceRequest{
			QuestionText: in.QuestionText,
			Expected:     verbatimAt(in, i),
			Candidate:    frags[best],
			Language:     lang,
		})
		if err != nil {
			return llm.ReviewResult{}, fmt.Errorf("par equivalencia (ítem %q vs %q): %w", in.Items[i], frags[best], err)
		}
		if res.Verdict == llm.VerdictCorrect {
			match.itemMatched[i] = true
			match.fragUsed[best] = true
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

// bestCandidate devuelve el índice del fragmento sobrante (no usado) más parecido al
// ítem por distancia de edición, o -1 si no queda ninguno. Solo ELIGE a quién
// preguntar; la decisión de equivalencia la toma el LLM. Empate ⇒ el primero (orden
// estable) para que el carril sea determinista.
func bestCandidate(item string, frags []string, used []bool) int {
	best := -1
	bestDist := -1
	for j, f := range frags {
		if used[j] {
			continue
		}
		d := levenshtein(item, f)
		if best == -1 || d < bestDist {
			best = j
			bestDist = d
		}
	}
	return best
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

// levenshtein calcula la distancia de edición entre dos strings (una fila, O(n·m)
// tiempo, O(min) espacio). Puro; solo se usa para rankear candidatos, no para juzgar.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
