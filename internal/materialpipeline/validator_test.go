package materialpipeline

import (
	"errors"
	"testing"
)

// --- ChunkArtifactsV1 ---

func TestValidateChunkArtifacts_OK(t *testing.T) {
	raw := []byte(`{"version":1,
		"main_ideas":["la fotosíntesis ocurre en los cloroplastos"],
		"secondary_ideas":["requiere luz solar"],
		"chunk_topic":"fotosíntesis"}`)
	a, err := ValidateChunkArtifacts(raw)
	if err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
	if len(a.MainIdeas) != 1 || a.ChunkTopic != "fotosíntesis" {
		t.Fatalf("artefactos mal parseados: %+v", a)
	}
}

func TestValidateChunkArtifacts_SecondaryOmitted_OK(t *testing.T) {
	// secondary_ideas puede omitirse por completo.
	raw := []byte(`{"version":1,"main_ideas":["idea"],"chunk_topic":"tema"}`)
	if _, err := ValidateChunkArtifacts(raw); err != nil {
		t.Fatalf("esperaba válido (secondary omitido es legítimo), got: %v", err)
	}
}

func TestValidateChunkArtifacts_WrongVersion(t *testing.T) {
	raw := []byte(`{"version":2,"main_ideas":["idea"],"chunk_topic":"tema"}`)
	if _, err := ValidateChunkArtifacts(raw); err == nil {
		t.Fatal("esperaba error por versión no soportada")
	}
}

func TestValidateChunkArtifacts_NoMainIdeas(t *testing.T) {
	raw := []byte(`{"version":1,"main_ideas":[],"chunk_topic":"tema"}`)
	if _, err := ValidateChunkArtifacts(raw); err == nil {
		t.Fatal("esperaba error: main_ideas requiere ≥1")
	}
}

func TestValidateChunkArtifacts_BlankMainIdea(t *testing.T) {
	raw := []byte(`{"version":1,"main_ideas":["idea","  "],"chunk_topic":"tema"}`)
	if _, err := ValidateChunkArtifacts(raw); err == nil {
		t.Fatal("esperaba error: una main_idea en blanco no puede pasar")
	}
}

func TestValidateChunkArtifacts_BlankSecondaryIdea(t *testing.T) {
	raw := []byte(`{"version":1,"main_ideas":["idea"],"secondary_ideas":[""],"chunk_topic":"tema"}`)
	if _, err := ValidateChunkArtifacts(raw); err == nil {
		t.Fatal("esperaba error: una secondary_idea en blanco no puede pasar")
	}
}

func TestValidateChunkArtifacts_EmptyTopic(t *testing.T) {
	raw := []byte(`{"version":1,"main_ideas":["idea"],"chunk_topic":"   "}`)
	if _, err := ValidateChunkArtifacts(raw); err == nil {
		t.Fatal("esperaba error: chunk_topic obligatorio")
	}
}

func TestValidateChunkArtifacts_NotJSON(t *testing.T) {
	_, err := ValidateChunkArtifacts([]byte(`no soy json`))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba *ValidationError, got: %v", err)
	}
}

// --- CandidatePayloadV1 ---

func TestValidateCandidate_MultipleChoice_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_choice",
		"question_text":"¿Dónde ocurre la fotosíntesis?",
		"options":["cloroplastos","mitocondrias","núcleo"],
		"correct_answer":"cloroplastos",
		"explanation":"los cloroplastos contienen clorofila",
		"source_ideas":["ocurre en cloroplastos"]}`)
	c, err := ValidateCandidatePayload(raw)
	if err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
	if c.QuestionType != "multiple_choice" || len(c.Options) != 3 {
		t.Fatalf("candidata mal parseada: %+v", c)
	}
}

func TestValidateCandidate_MultipleSelect_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_select",
		"question_text":"¿Cuáles son gases nobles?",
		"options":["helio","neón","oxígeno"],
		"correct_answer":["helio","neón"]}`)
	if _, err := ValidateCandidatePayload(raw); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidateCandidate_TrueFalse_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"true_false",
		"question_text":"El agua hierve a 100°C a nivel del mar.",
		"correct_answer":"true"}`)
	if _, err := ValidateCandidatePayload(raw); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidateCandidate_ShortAnswer_OK(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer",
		"question_text":"Capital de Francia.","correct_answer":"París"}`)
	if _, err := ValidateCandidatePayload(raw); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidateCandidate_OpenEnded_OK(t *testing.T) {
	// open_ended no lleva opciones ni correct_answer.
	raw := []byte(`{"version":1,"question_type":"open_ended",
		"question_text":"Explica el ciclo del agua."}`)
	if _, err := ValidateCandidatePayload(raw); err != nil {
		t.Fatalf("esperaba válido, got: %v", err)
	}
}

func TestValidateCandidate_WrongVersion(t *testing.T) {
	raw := []byte(`{"version":2,"question_type":"short_answer",
		"question_text":"x","correct_answer":"y"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error por versión no soportada")
	}
}

func TestValidateCandidate_UnknownType(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"essay","question_text":"x"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: tipo desconocido")
	}
}

func TestValidateCandidate_EmptyText(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer",
		"question_text":"   ","correct_answer":"y"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: question_text vacío")
	}
}

func TestValidateCandidate_MultipleChoiceTooFewOptions(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_choice",
		"question_text":"x","options":["única"],"correct_answer":"única"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: multiple_choice requiere ≥2 opciones")
	}
}

func TestValidateCandidate_BlankOption(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_choice",
		"question_text":"x","options":["a",""],"correct_answer":"a"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: una opción en blanco no puede pasar")
	}
}

func TestValidateCandidate_ShortAnswerWithOptions(t *testing.T) {
	// short_answer no debe llevar opciones.
	raw := []byte(`{"version":1,"question_type":"short_answer",
		"question_text":"x","options":["a","b"],"correct_answer":"y"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: short_answer no debe llevar opciones")
	}
}

func TestValidateCandidate_MissingCorrectAnswer(t *testing.T) {
	// Todos salvo open_ended requieren correct_answer.
	raw := []byte(`{"version":1,"question_type":"short_answer","question_text":"x"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: correct_answer obligatorio")
	}
}

func TestValidateCandidate_MultipleChoiceAnswerNotInOptions(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_choice",
		"question_text":"x","options":["a","b"],"correct_answer":"c"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: la correcta no coincide con ninguna opción")
	}
}

func TestValidateCandidate_MultipleSelectAnswerNotInOptions(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"multiple_select",
		"question_text":"x","options":["a","b"],"correct_answer":["a","z"]}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: una respuesta del array no coincide con ninguna opción")
	}
}

func TestValidateCandidate_MultipleSelectScalarAnswer(t *testing.T) {
	// multiple_select espera array; un escalar debe rechazarse (guard de polimorfismo).
	raw := []byte(`{"version":1,"question_type":"multiple_select",
		"question_text":"x","options":["a","b"],"correct_answer":"a"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: multiple_select espera array, no escalar")
	}
}

func TestValidateCandidate_TrueFalseBadValue(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"true_false",
		"question_text":"x","correct_answer":"maybe"}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: true_false solo acepta true/false")
	}
}

func TestValidateCandidate_BlankSourceIdea(t *testing.T) {
	raw := []byte(`{"version":1,"question_type":"short_answer",
		"question_text":"x","correct_answer":"y","source_ideas":["idea",""]}`)
	if _, err := ValidateCandidatePayload(raw); err == nil {
		t.Fatal("esperaba error: una source_idea en blanco no puede pasar")
	}
}

func TestValidateCandidate_NotJSON(t *testing.T) {
	_, err := ValidateCandidatePayload([]byte(`no soy json`))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba *ValidationError, got: %v", err)
	}
}
