package processor

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// Qué es una respuesta en blanco (decisión 3 de la HU-INF.5), con sus bordes.
func TestIsBlankAnswer(t *testing.T) {
	cases := []struct {
		student, reference string
		want               bool
	}{
		{"", "París", true},
		{"   ", "París", true},
		{"\t\n", "París", true},
		{".", "París", true},
		{"...", "París", true},
		{"-", "París", true},
		{"?", "París", true},
		{"¿?", "París", true},
		{"no sé", "París", false},
		{"nose", "París", false},
		{"0", "París", false},
		{"ñ", "París", false},
		{"   x  ", "París", false},
		// La esperada es un símbolo: la regla no se aplica.
		{"?", "?", false},
		{"", "—", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := isBlankAnswer(c.student, c.reference); got != c.want {
			t.Errorf("isBlankAnswer(%q, %q) = %v, se esperaba %v", c.student, c.reference, got, c.want)
		}
	}
}

func TestReferenceAnswer_RubricWhenNoExpected(t *testing.T) {
	ans := laneAnswer(llm.QuestionTypeOpenEnded, "", "")
	ans.ExpectedAnswer = "  "
	ans.Rubric = "Nombra las tres fases."
	if got := referenceAnswer(ans); got != ans.Rubric {
		t.Fatalf("sin esperada, la referencia es la rúbrica; salió %q", got)
	}
}

// El comentario se corta en su tope, contado en caracteres, no en bytes.
func TestCapFeedback(t *testing.T) {
	short := "Bien."
	if capFeedback(short) != short {
		t.Fatal("un comentario corto no se toca")
	}
	long := strings.Repeat("ñ", maxFeedbackRunes+50)
	got := capFeedback(long)
	if utf8.RuneCountInString(got) != maxFeedbackRunes || !utf8.ValidString(got) {
		t.Fatalf("se esperaba un corte en %d caracteres válidos, salieron %d", maxFeedbackRunes, utf8.RuneCountInString(got))
	}
}

func TestVerdictFraction(t *testing.T) {
	for v, want := range map[llm.Verdict]float64{
		llm.VerdictCorrect: 1, llm.VerdictPartial: 0.5, llm.VerdictIncorrect: 0, "": 0,
	} {
		if got := verdictFraction(v); got != want {
			t.Errorf("verdictFraction(%q) = %v, se esperaba %v", v, got, want)
		}
	}
}
