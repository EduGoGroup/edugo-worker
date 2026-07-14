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
