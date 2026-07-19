package reduce

import (
	"strconv"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// words genera "w1 w2 … wn" (n palabras únicas separadas por espacio).
func words(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "palabra" + strconv.Itoa(i)
	}
	return strings.Join(parts, " ")
}

// Frontera exacta del candado (D-044.4): «más de 25 palabras contiguas». 25 palabras
// verbatim NO disparan; 26 sí. El chunk contiene una tirada de 30 palabras.
func TestIsLocalOnly_Frontera25vs26(t *testing.T) {
	chunk := words(30)

	if IsLocalOnly(words(25), chunk, 25) {
		t.Fatalf("25 palabras contiguas NO deben disparar el candado (frontera)")
	}
	if !IsLocalOnly(words(26), chunk, 25) {
		t.Fatalf("26 palabras contiguas SÍ deben disparar el candado (>25)")
	}
}

// Umbral <= 0 cae al default firmado (25): misma frontera.
func TestIsLocalOnly_ThresholdDefault(t *testing.T) {
	chunk := words(30)
	if IsLocalOnly(words(25), chunk, 0) {
		t.Fatalf("con umbral 0 (default 25) 25 palabras no deben disparar")
	}
	if !IsLocalOnly(words(26), chunk, 0) {
		t.Fatalf("con umbral 0 (default 25) 26 palabras sí deben disparar")
	}
}

// Candidata más corta que la ventana (threshold+1) nunca dispara.
func TestIsLocalOnly_TooShort(t *testing.T) {
	if IsLocalOnly(words(10), words(100), 25) {
		t.Fatalf("una candidata de 10 palabras no puede citar >25 contiguas")
	}
}

// Sin cita literal (paráfrasis / no contigua) no dispara aunque compartan vocabulario.
func TestIsLocalOnly_NoVerbatim(t *testing.T) {
	chunk := words(40)
	// Toma palabras salteadas: comparten léxico pero no una tirada contigua de 26.
	var picked []string
	for i := 0; i < 40; i += 2 {
		picked = append(picked, "palabra"+strconv.Itoa(i))
	}
	cand := strings.Join(picked, " ")
	if IsLocalOnly(cand, chunk, 25) {
		t.Fatalf("palabras salteadas no son una cita contigua: no debe disparar")
	}
}

// La comparación es por tokens normalizados: mayúsculas, tildes y puntuación no impiden
// detectar la cita literal (misma tirada, distinta ortografía superficial).
func TestIsLocalOnly_NormalizacionCasoYTildes(t *testing.T) {
	chunk := words(30)
	// La misma tirada de 26 pero en MAYÚSCULAS y con signos de puntuación intercalados.
	loud := strings.ToUpper(words(26))
	loud = strings.ReplaceAll(loud, " ", ", ") + "!"
	if !IsLocalOnly(loud, chunk, 25) {
		t.Fatalf("la cita literal debe detectarse pese a mayúsculas/puntuación (normalización)")
	}
}

// candidateVerbatimText concatena question_text + options + explanation: una cita repartida
// entre pregunta y explicación se detecta igual.
func TestCandidateVerbatimText_ConcatenaCampos(t *testing.T) {
	chunk := words(30)
	c := materialpipeline.CandidatePayloadV1{
		Version:      1,
		QuestionType: "short_answer",
		QuestionText: words(13),                            // primeras 13
		Explanation:  strings.Join(splitFrom(13, 26), " "), // 13..25 → junto, 26 contiguas
	}
	if !IsLocalOnly(candidateVerbatimText(c), chunk, 25) {
		t.Fatalf("la cita repartida entre pregunta y explicación debe detectarse")
	}
}

// splitFrom devuelve las palabras [from, to) de la secuencia words().
func splitFrom(from, to int) []string {
	out := make([]string, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, "palabra"+strconv.Itoa(i))
	}
	return out
}
