package reduce

import (
	"strings"
	"unicode"

	"github.com/EduGoGroup/edugo-shared/textmatch"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// defaultVerbatimMaxWords es el umbral FIRMADO por el dueño (D-044.4, 2026-07-17): una
// candidata que cite literalmente MÁS de 25 palabras contiguas del chunk se marca
// local_only y sus pasadas LLM nunca salen por API. Vive en config (ajustable sin cambio
// de arquitectura); este es el fallback cuando el umbral llega en cero.
const defaultVerbatimMaxWords = 25

// IsLocalOnly decide, de forma DETERMINISTA y gratis (n-gramas, sin LLM), si una candidata
// cita literalmente más de `thresholdWords` palabras CONTIGUAS del texto del chunk del que
// salió (candado de derechos de autor, D-044.4). Devuelve true si existe una tirada de
// (thresholdWords+1) palabras del texto de la candidata que aparece —en el mismo orden y
// contigua— dentro del texto del chunk. La comparación es por tokens normalizados
// (textmatch.Normalize: minúsculas, sin tildes, preserva la «ñ»; frontera = todo carácter
// no alfanumérico). Se computa AL VUELO (no se persiste: es recomputable gratis).
//
// A diferencia de textmatch.SplitTokens, aquí NO se descartan las conectoras «y»/«e»:
// quitarlas alteraría la contigüidad y volvería «verbatim» a un texto que no lo es. El
// candidateText lo concatena el caller (question_text + options + explanation).
//
// `thresholdWords <= 0` cae al default firmado (25). «más de 25 palabras contiguas» ⇒ la
// frontera está en 26: una cita de exactamente 25 palabras NO dispara; 26 sí.
func IsLocalOnly(candidateText, chunkText string, thresholdWords int) bool {
	if thresholdWords <= 0 {
		thresholdWords = defaultVerbatimMaxWords
	}
	window := thresholdWords + 1 // > threshold contiguas ⇒ una ventana de threshold+1

	cand := verbatimTokens(candidateText)
	chunk := verbatimTokens(chunkText)
	if len(cand) < window || len(chunk) < window {
		return false
	}

	// Conjunto de n-gramas (longitud=window) del chunk; una ventana igual en la candidata
	// prueba la cita literal. \x00 separa tokens para que el join sea inyectivo.
	grams := make(map[string]struct{}, len(chunk))
	for i := 0; i+window <= len(chunk); i++ {
		grams[strings.Join(chunk[i:i+window], "\x00")] = struct{}{}
	}
	for i := 0; i+window <= len(cand); i++ {
		if _, ok := grams[strings.Join(cand[i:i+window], "\x00")]; ok {
			return true
		}
	}
	return false
}

// verbatimTokens normaliza el texto (textmatch.Normalize) y lo parte en palabras usando
// como frontera todo carácter no alfanumérico unicode. Conserva TODAS las palabras
// (incluidas «y»/«e») para no alterar la contigüidad que mide el candado verbatim.
func verbatimTokens(s string) []string {
	norm := textmatch.Normalize(s)
	return strings.FieldsFunc(norm, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// candidateVerbatimText concatena los campos textuales de una candidata que el candado
// verbatim inspecciona (D-044.4): question_text + options + explanation, separados por
// espacio para preservar las fronteras de palabra.
func candidateVerbatimText(c materialpipeline.CandidatePayloadV1) string {
	parts := make([]string, 0, len(c.Options)+2)
	parts = append(parts, c.QuestionText)
	parts = append(parts, c.Options...)
	if c.Explanation != "" {
		parts = append(parts, c.Explanation)
	}
	return strings.Join(parts, " ")
}
