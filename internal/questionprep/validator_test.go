package questionprep

import (
	"errors"
	"testing"
)

func TestValidate_ShortAnswerList_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"list",
		"items":["ecuador","venezuela","colombia"],
		"items_verbatim":["Ecuador","Venezuela","Colombia"],"unit":null}`)
	p, err := Validate(raw, QuestionTypeShortAnswer)
	if err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
	if p.ContentKind != ContentKindList || len(p.Items) != 3 {
		t.Fatalf("prep mal parseado: %+v", p)
	}
}

func TestValidate_NumberWithUnit_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"number",
		"items":["150000000"],"items_verbatim":["150 millones"],"unit":"km"}`)
	if _, err := Validate(raw, QuestionTypeShortAnswer); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidate_OpenEnded_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"open_ended",
		"question_intent":"medir si identifica el proceso",
		"main_ideas":["ocurre en cloroplastos"],"secondary_ideas":[],
		"valid_variants":[],"criteria":["menciona oxígeno"]}`)
	if _, err := Validate(raw, QuestionTypeOpenEnded); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidate_WrongVersion(t *testing.T) {
	raw := []byte(`{"version":2,"question_type":"short_answer","content_kind":"term","items":["x"],"items_verbatim":["X"]}`)
	if _, err := Validate(raw, QuestionTypeShortAnswer); err == nil {
		t.Fatal("esperaba error por versión no soportada")
	}
}

func TestValidate_TypeMismatch(t *testing.T) {
	// El prep dice short_answer pero la fila real es open_ended: incoherencia.
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"term","items":["x"],"items_verbatim":["X"]}`)
	_, err := Validate(raw, QuestionTypeOpenEnded)
	if err == nil {
		t.Fatal("esperaba error por question_type incoherente")
	}
}

func TestValidate_ListEmptyItems(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"list","items":[],"items_verbatim":[]}`)
	if _, err := Validate(raw, QuestionTypeShortAnswer); err == nil {
		t.Fatal("esperaba error: list requiere ≥1 ítem")
	}
}

func TestValidate_ItemsVerbatimLengthMismatch(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"list",
		"items":["a","b"],"items_verbatim":["A"]}`)
	if _, err := Validate(raw, QuestionTypeShortAnswer); err == nil {
		t.Fatal("esperaba error: items e items_verbatim de distinto largo")
	}
}

func TestValidate_NumberRequiresExactlyOneItem(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer","content_kind":"number",
		"items":["1","2"],"items_verbatim":["1","2"]}`)
	if _, err := Validate(raw, QuestionTypeShortAnswer); err == nil {
		t.Fatal("esperaba error: number requiere exactamente 1 ítem")
	}
}

func TestValidate_OpenEndedMissingIntent(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"open_ended","question_intent":"  ","main_ideas":["x"]}`)
	if _, err := Validate(raw, QuestionTypeOpenEnded); err == nil {
		t.Fatal("esperaba error: question_intent obligatorio")
	}
}

func TestValidate_OpenEndedNoMainIdeas(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"open_ended","question_intent":"medir X","main_ideas":[]}`)
	if _, err := Validate(raw, QuestionTypeOpenEnded); err == nil {
		t.Fatal("esperaba error: main_ideas requiere ≥1")
	}
}

func TestValidate_OpenEndedBlankCriteria(t *testing.T) {
	// criteria con elementos pero todos en blanco: debe rechazarse. Si pasara, el carril
	// por criterios contaría 0 reales y devolvería incorrect/0.0 sin consultar al LLM.
	raw := []byte(`{"version":1,"question_type":"open_ended","question_intent":"medir X",
		"main_ideas":["idea"],"criteria":["","  "]}`)
	if _, err := Validate(raw, QuestionTypeOpenEnded); err == nil {
		t.Fatal("esperaba error: criteria con elementos en blanco no puede pasar")
	}
}

func TestValidate_OpenEndedEmptyCriteria_OK(t *testing.T) {
	// criteria de 0 elementos SÍ es válido (carril F4a: enriquecimiento del prompt global).
	raw := []byte(`{"version":1,"question_type":"open_ended","question_intent":"medir X",
		"main_ideas":["idea"],"criteria":[]}`)
	if _, err := Validate(raw, QuestionTypeOpenEnded); err != nil {
		t.Fatalf("esperaba válido (criteria vacío es legítimo), got: %v", err)
	}
}

func TestValidate_NotJSON(t *testing.T) {
	_, err := Validate([]byte(`no soy json`), QuestionTypeShortAnswer)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba *ValidationError, got: %v", err)
	}
}

func TestValidate_UnsupportedType(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_choice"}`)
	if _, err := Validate(raw, "multiple_choice"); err == nil {
		t.Fatal("esperaba error: multiple_choice no tiene prep")
	}
}
