package llm

import (
	"strings"
	"testing"
)

func TestBuildGenerationPrompt_ContainsRulesAndMaterial(t *testing.T) {
	p := BuildGenerationPrompt(
		MaterialInput{Title: "T", Content: "CUERPO", SubjectHint: "Bio"},
		GenerationParams{NumQuestions: 4, Difficulty: "hard", QuestionTypes: []string{"multiple_choice"}},
	)
	for _, want := range []string{"edugo.assessment_import", "CUERPO", "multiple_choice", "hard", "JSON"} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt no contiene %q", want)
		}
	}
	// Endurecimiento (039 §6.1): la regla explícita de correct_answer = texto, no letra.
	for _, want := range []string{"COPIAR EXACTAMENTE", "NUNCA uses letras", "INCORRECTO"} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt no incluye el endurecimiento de correct_answer: falta %q", want)
		}
	}
}

func TestBuildReviewPrompt_ContainsSections(t *testing.T) {
	p := BuildReviewPrompt(ReviewRequest{
		QuestionText:   "PREG",
		ExpectedAnswer: "ESP",
		Rubric:         "RUB",
		StudentAnswer:  "ALU",
	})
	for _, want := range []string{"PREG", "ESP", "RUB", "ALU", "verdict"} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt no contiene %q", want)
		}
	}
}

func TestBuildReviewPrompt_ShortAnswer_Equivalencia(t *testing.T) {
	p := BuildReviewPrompt(ReviewRequest{
		QuestionType:   QuestionTypeShortAnswer,
		QuestionText:   "PREG",
		ExpectedAnswer: "ESP",
		StudentAnswer:  "ALU",
	})
	// Debe ser el prompt de equivalencia (segunda opinión, binario correct|incorrect).
	for _, want := range []string{"EQUIVALENTE", "SEGUNDA OPINIÓN", "correct|incorrect", "PREG", "ESP", "ALU"} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt short_answer no contiene %q", want)
		}
	}
	// NO debe ofrecer "partial" (solo hay dos veredictos en respuestas cortas).
	if strings.Contains(p, "correct|partial|incorrect") {
		t.Errorf("el prompt short_answer no debe ofrecer el veredicto partial")
	}
	// Mantiene el endurecimiento anti-injection.
	for _, want := range []string{"SEGURIDAD", "NUNCA instrucciones", "<<<"} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt short_answer perdió el endurecimiento anti-injection: falta %q", want)
		}
	}
}

func TestBuildReviewPrompt_OpenEnded_PorDefecto(t *testing.T) {
	// QuestionType vacío o open_ended → prompt con rúbrica y verdict de 3 valores.
	for _, qt := range []string{"", QuestionTypeOpenEnded} {
		p := BuildReviewPrompt(ReviewRequest{QuestionType: qt, QuestionText: "Q", StudentAnswer: "A"})
		if !strings.Contains(p, "correct|partial|incorrect") {
			t.Errorf("qt=%q debe usar el prompt open_ended (3 veredictos)", qt)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plano", `{"a":1}`, `{"a":1}`},
		{"con vallas", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"con prosa", "Aquí tienes:\n{\"a\":1}\nEso es todo", `{"a":1}`},
		{"anidado", `{"a":{"b":2},"c":3}`, `{"a":{"b":2},"c":3}`},
		{"string con llave", `{"a":"}"}`, `{"a":"}"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSON(tc.in)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractJSON_UnwrapEnvelope(t *testing.T) {
	// Envoltura espuria conocida de una sola clave → se desenvuelve al objeto interno.
	got, err := ExtractJSON(`{"bytes":{"verdict":"correct","score":1.0,"feedback":"ok"}}`)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !strings.Contains(string(got), `"verdict"`) || strings.Contains(string(got), "bytes") {
		t.Fatalf("no se desenvolvió el envoltorio: %s", got)
	}
}

func TestExtractJSON_NoUnwrapLegitObject(t *testing.T) {
	// Objeto legítimo de una clave NO envolvente → NO se toca.
	in := `{"verdict":"correct"}`
	got, err := ExtractJSON(in)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if string(got) != in {
		t.Fatalf("se alteró un objeto legítimo: got %q, want %q", got, in)
	}
	// Contrato con varias claves de tope → intacto aunque una fuera "data".
	multi := `{"data":1,"format":"x"}`
	got2, err := ExtractJSON(multi)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if string(got2) != multi {
		t.Fatalf("se alteró un objeto multi-clave: got %q", got2)
	}
}

func TestExtractJSON_NoObject(t *testing.T) {
	if _, err := ExtractJSON("sin json aquí"); err == nil {
		t.Fatal("esperaba error")
	}
}

func TestExtractJSON_Unbalanced(t *testing.T) {
	if _, err := ExtractJSON(`{"a":1`); err == nil {
		t.Fatal("esperaba error por objeto sin cierre")
	}
}
