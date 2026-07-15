package assessmentimport

import (
	"strings"
	"testing"
)

const validJSON = `{
  "format": "edugo.assessment_import",
  "version": 1,
  "assessment": {"title": "Fotosíntesis", "passing_score": 60},
  "questions": [
    {
      "question_text": "¿Pigmento principal?",
      "question_type": "multiple_choice",
      "options": [
        {"option_text": "Clorofila", "sort_order": 1},
        {"option_text": "Melanina", "sort_order": 2}
      ],
      "correct_answer": "Clorofila",
      "points": 1,
      "difficulty": "easy"
    },
    {
      "question_text": "El agua se divide en la fase luminosa",
      "question_type": "true_false",
      "options": [],
      "correct_answer": "true"
    },
    {
      "question_text": "Selecciona los productos",
      "question_type": "multiple_select",
      "options": [
        {"option_text": "Glucosa"},
        {"option_text": "Oxígeno"},
        {"option_text": "Hierro"}
      ],
      "correct_answer": ["Glucosa", "Oxígeno"]
    },
    {
      "question_text": "Explica por qué las hojas son verdes",
      "question_type": "open_ended"
    }
  ]
}`

func TestValidate_OK(t *testing.T) {
	c, err := Validate([]byte(validJSON), DefaultLimits())
	if err != nil {
		t.Fatalf("esperaba válido, obtuve: %v", err)
	}
	if c.Assessment.Title != "Fotosíntesis" {
		t.Fatalf("título inesperado: %q", c.Assessment.Title)
	}
	if len(c.Questions) != 4 {
		t.Fatalf("esperaba 4 preguntas, hay %d", len(c.Questions))
	}
}

func TestValidate_BadFormatAndVersion(t *testing.T) {
	j := `{"format":"otro","version":2,"assessment":{"title":"x"},"questions":[{"question_text":"q","question_type":"open_ended"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error por format/version")
	}
	ve := err.(*ValidationError)
	if !hasField(ve, "format") || !hasField(ve, "version") {
		t.Fatalf("esperaba issues de format y version, obtuve: %v", ve.Issues)
	}
}

func TestValidate_UnknownQuestionType(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},"questions":[{"question_text":"q","question_type":"essay"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error por tipo desconocido")
	}
	if !hasField(err.(*ValidationError), "question_type") {
		t.Fatalf("esperaba issue de question_type: %v", err)
	}
}

func TestValidate_MultipleChoiceCorrectAnswerMustMatchOption(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},
	"questions":[{"question_text":"q","question_type":"multiple_choice",
	"options":[{"option_text":"A"},{"option_text":"B"}],"correct_answer":"Z"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error: correct_answer no coincide")
	}
	if !hasField(err.(*ValidationError), "correct_answer") {
		t.Fatalf("esperaba issue de correct_answer: %v", err)
	}
}

func TestValidate_MultipleChoiceTooFewOptions(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},
	"questions":[{"question_text":"q","question_type":"multiple_choice",
	"options":[{"option_text":"A"}],"correct_answer":"A"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error por menos de 2 opciones")
	}
}

func TestValidate_TrueFalseWithOptionsRejected(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},
	"questions":[{"question_text":"q","question_type":"true_false",
	"options":[{"option_text":"A"}],"correct_answer":"true"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error: true_false no debe llevar opciones")
	}
}

func TestValidate_OpenEndedNeedsNoCorrectAnswer(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x"},
	"questions":[{"question_text":"q","question_type":"open_ended"}]}`
	if _, err := Validate([]byte(j), DefaultLimits()); err != nil {
		t.Fatalf("open_ended sin correct_answer debe ser válido: %v", err)
	}
}

func TestValidate_EmptyTitle(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"   "},
	"questions":[{"question_text":"q","question_type":"open_ended"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil || !hasField(err.(*ValidationError), "assessment.title") {
		t.Fatalf("esperaba issue de título vacío: %v", err)
	}
}

func TestValidate_PassingScoreOutOfRange(t *testing.T) {
	j := `{"format":"edugo.assessment_import","version":1,"assessment":{"title":"x","passing_score":150},
	"questions":[{"question_text":"q","question_type":"open_ended"}]}`
	_, err := Validate([]byte(j), DefaultLimits())
	if err == nil || !hasField(err.(*ValidationError), "assessment.passing_score") {
		t.Fatalf("esperaba issue de passing_score: %v", err)
	}
}

func TestValidate_TooManyQuestions(t *testing.T) {
	limits := Limits{MaxQuestions: 1, MaxJSONBytes: DefaultMaxJSONBytes, MaxOptionsPerQ: DefaultMaxOptionsPerQ}
	_, err := Validate([]byte(validJSON), limits)
	if err == nil {
		t.Fatal("esperaba error por exceso de preguntas")
	}
}

func TestValidate_JSONTooBig(t *testing.T) {
	limits := Limits{MaxQuestions: DefaultMaxQuestions, MaxJSONBytes: 10, MaxOptionsPerQ: DefaultMaxOptionsPerQ}
	_, err := Validate([]byte(validJSON), limits)
	if err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("esperaba error por tamaño: %v", err)
	}
}

func TestValidate_NotJSON(t *testing.T) {
	_, err := Validate([]byte("esto no es json"), DefaultLimits())
	if err == nil {
		t.Fatal("esperaba error por JSON inválido")
	}
}

func hasField(ve *ValidationError, field string) bool {
	for _, iss := range ve.Issues {
		if iss.Field == field {
			return true
		}
	}
	return false
}
