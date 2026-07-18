package chunking

import (
	"strings"
	"unicode"
)

// isTitleLine decide si una línea parece un encabezado de sección. Reimplementa
// (sin importar) la heurística de internal/infrastructure/nlp/fallback para
// mantener este paquete puro. Reconoce: líneas TODO EN MAYÚSCULAS, numeración
// (1., 1.2, romanos I., II.…), palabras clave de capítulo/sección/tema/unidad/
// parte (es-ES e inglés) y líneas cortas sin punto final.
func isTitleLine(line string) bool {
	line = strings.TrimSpace(line)
	if len(line) == 0 || len(line) >= 100 {
		return false
	}

	// Las viñetas de lista no son encabezados aunque sean cortas.
	if isBulletLine(line) {
		return false
	}

	// TODO EN MAYÚSCULAS con al menos tres letras.
	if isAllUpper(line) && countLetters(line) >= 3 {
		return true
	}

	// Prefijos numerados: "1.", "2.", …, y romanos "I.", "II.", …
	numberedPrefixes := []string{
		"1.", "2.", "3.", "4.", "5.", "6.", "7.", "8.", "9.",
		"I.", "II.", "III.", "IV.", "V.", "VI.", "VII.", "VIII.", "IX.", "X.",
	}
	for _, prefix := range numberedPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	// Palabras clave de encabezado (insensible a mayúsculas), es-ES e inglés.
	lower := strings.ToLower(line)
	keywordPrefixes := []string{
		"chapter ", "capítulo ", "capitulo ",
		"sección ", "seccion ", "section ",
		"tema ", "unidad ", "parte ", "part ",
	}
	for _, kw := range keywordPrefixes {
		if strings.HasPrefix(lower, kw) {
			return true
		}
	}

	// Línea corta (< 80 caracteres) que no termina en punto: heurística de
	// título breve. Se exige entre 1 y 10 palabras para no confundir con frases.
	if len(line) < 80 && !strings.HasSuffix(line, ".") {
		words := strings.Fields(line)
		if len(words) >= 1 && len(words) <= 10 {
			return true
		}
	}

	return false
}

// isBulletLine detecta líneas que arrancan con un marcador de viñeta de lista.
func isBulletLine(line string) bool {
	for _, marker := range []string{"- ", "* ", "• ", "– ", "— ", "· "} {
		if strings.HasPrefix(line, marker) {
			return true
		}
	}
	return false
}

// isAllUpper indica si todas las letras de la cadena son mayúsculas (soporta
// acentos y eñe vía unicode). Requiere al menos una letra.
func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

// countLetters cuenta caracteres alfabéticos (unicode) en la cadena.
func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}
