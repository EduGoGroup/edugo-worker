package llm

import (
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

func TestBuildDigestChunkPrompt_ContainsRulesAndChunk(t *testing.T) {
	prev := "El tema es la fotosíntesis; ya se definió clorofila."
	p := BuildDigestChunkPrompt(DigestChunkInput{
		ChunkText:   "CUERPO DEL TROZO",
		PrevSummary: &prev,
		Language:    "es",
	})
	for _, want := range []string{
		"CUERPO DEL TROZO",       // el trozo va delimitado
		"main_ideas",             // forma de salida
		"chunk_topic",            // forma de salida
		"summary",                // forma de salida
		"120",                    // tope del summary
		"OTRO MODELO",            // el summary es para otro LLM
		"RESUMEN DE LO ANTERIOR", // se inyecta el prev summary
		prev,                     // el contenido del prev summary
		"SEGURIDAD",              // anti-injection heredado
	} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt A no contiene %q", want)
		}
	}
}

func TestBuildDigestChunkPrompt_SinPrevSummary(t *testing.T) {
	p := BuildDigestChunkPrompt(DigestChunkInput{ChunkText: "trozo inicial"})
	if strings.Contains(p, "RESUMEN DE LO ANTERIOR") {
		t.Error("el primer trozo (PrevSummary nil) no debe incluir la sección de resumen anterior")
	}
}

func TestBuildProposeCandidatesPrompt_ContainsIdeasAndRules(t *testing.T) {
	p := BuildProposeCandidatesPrompt(ProposeCandidatesInput{
		Artifacts: materialpipeline.ChunkArtifactsV1{
			Version:        1,
			MainIdeas:      []string{"La clorofila absorbe la luz"},
			SecondaryIdeas: []string{"Las hojas se ven verdes"},
			ChunkTopic:     "La clorofila",
		},
		Language: "es",
	})
	for _, want := range []string{
		"candidates",                  // forma de salida envuelta
		"La clorofila absorbe la luz", // idea principal presente
		"Las hojas se ven verdes",     // idea secundaria presente
		"La clorofila",                // tema presente
		"multiple_choice",             // tipos permitidos
		"open_ended",                  // tipos permitidos
		"COPIA EXACTAMENTE",           // regla anti-letra
		"source_ideas",                // trazabilidad de ideas
		"entre 2 y 4",                 // sobregeneración acotada
	} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt B no contiene %q", want)
		}
	}
}

func TestParseDigestResult_SeparaSummaryYValida(t *testing.T) {
	raw := []byte(`{
		"version": 1,
		"main_ideas": ["El agua se evapora por el calor del Sol"],
		"secondary_ideas": ["La evaporación ocurre en océanos y ríos"],
		"chunk_topic": "Evaporación",
		"summary": "Cubrió evaporación: el Sol calienta el agua y la vuelve vapor."
	}`)

	art, summary, err := ParseDigestResult(raw)
	if err != nil {
		t.Fatalf("ParseDigestResult devolvió error: %v", err)
	}
	if art.Version != 1 || art.ChunkTopic != "Evaporación" || len(art.MainIdeas) != 1 {
		t.Fatalf("artefactos mal parseados: %+v", art)
	}
	if summary == "" || strings.Contains(summary, "evapora") == false {
		t.Fatalf("summary mal extraído: %q", summary)
	}
	// El summary NO debe quedar dentro de los artefactos (columna aparte, D-043.7).
	rawArt, _ := art.Marshal()
	if strings.Contains(string(rawArt), "summary") {
		t.Errorf("el summary se filtró dentro de ChunkArtifactsV1: %s", rawArt)
	}
	// Los artefactos parseados validan contra el contrato.
	if _, verr := materialpipeline.ValidateChunkArtifacts(rawArt); verr != nil {
		t.Errorf("artefactos válidos rechazados por el validador: %v", verr)
	}
}

func TestParseDigestResult_FaithfulVersion(t *testing.T) {
	// El parser es FIEL: si el modelo omite version, la validación posterior debe fallar
	// (no se enmascara forzando version=1).
	raw := []byte(`{"main_ideas":["idea"],"chunk_topic":"tema","summary":"s"}`)
	art, _, err := ParseDigestResult(raw)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if art.Version != 0 {
		t.Fatalf("esperaba version 0 (omitida por el modelo), got %d", art.Version)
	}
	rawArt, _ := art.Marshal()
	if _, verr := materialpipeline.ValidateChunkArtifacts(rawArt); verr == nil {
		t.Error("un artefacto sin version debería ser rechazado por el validador")
	}
}

func TestParseCandidates_MixDeTipos(t *testing.T) {
	raw := []byte(`{"candidates":[
		{"version":1,"question_type":"multiple_choice","question_text":"¿Qué pigmento capta la luz?","options":["Clorofila","Hemoglobina"],"correct_answer":"Clorofila","source_ideas":["La clorofila absorbe la luz"]},
		{"version":1,"question_type":"multiple_select","question_text":"¿Cuáles son gases nobles?","options":["Helio","Neón","Oxígeno"],"correct_answer":["Helio","Neón"]},
		{"version":1,"question_type":"open_ended","question_text":"Explica la evaporación."}
	]}`)

	cands, err := ParseCandidates(raw)
	if err != nil {
		t.Fatalf("ParseCandidates devolvió error: %v", err)
	}
	if len(cands) != 3 {
		t.Fatalf("esperaba 3 candidatas, got %d", len(cands))
	}
	// correct_answer polimórfico se conserva crudo: array para multiple_select.
	if !strings.HasPrefix(strings.TrimSpace(string(cands[1].CorrectAnswer)), "[") {
		t.Errorf("multiple_select debería conservar correct_answer como array: %s", cands[1].CorrectAnswer)
	}
	// Cada candidata válida pasa el contrato.
	for i, c := range cands {
		rawCand, _ := c.Marshal()
		if _, verr := materialpipeline.ValidateCandidatePayload(rawCand); verr != nil {
			t.Errorf("candidata %d válida rechazada: %v", i, verr)
		}
	}
}

func TestParseCandidates_ViaExtractJSON_Envuelto(t *testing.T) {
	// Salida envuelta en una clave espuria + vallas markdown: ExtractJSON debe
	// desenvolver y ParseCandidates leer los candidates.
	out := "```json\n{\"result\":{\"candidates\":[{\"version\":1,\"question_type\":\"true_false\",\"question_text\":\"El agua hierve a 100°C al nivel del mar.\",\"correct_answer\":\"true\"}]}}\n```"
	rawJSON, err := ExtractJSON(out)
	if err != nil {
		t.Fatalf("ExtractJSON falló: %v", err)
	}
	cands, err := ParseCandidates(rawJSON)
	if err != nil {
		t.Fatalf("ParseCandidates falló: %v", err)
	}
	if len(cands) != 1 || cands[0].QuestionType != "true_false" {
		t.Fatalf("candidatas mal parseadas tras desenvolver: %+v", cands)
	}
}
