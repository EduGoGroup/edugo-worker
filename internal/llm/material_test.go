package llm

import (
	"strings"
	"testing"
	"unicode/utf8"

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
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	for _, want := range []string{
		"candidates",                  // forma de salida envuelta
		"La clorofila absorbe la luz", // idea principal presente
		"Las hojas se ven verdes",     // idea secundaria presente
		"La clorofila",                // tema presente
		"multiple_choice",             // tipos permitidos
		"open_ended",                  // tipos permitidos
		"COPIA EXACTAMENTE",           // regla anti-letra
		"source_ideas",                // trazabilidad de ideas
		"entre 2 y 4",                 // sobregeneración acotada (HU-IA.8, decisión 6: no se mueve)
		"multiple_select",             // los cinco tipos siguen en la lista
		"true_false",
		"short_answer",
		"ELIGE el tipo",              // la guía de tipos (HU-IA.8)
		"Debe mencionar:",            // la rúbrica de la abierta (HU-IA.8)
		"AUTOCONTENIDO",              // lo que se queda: autocontenido
		"SOLO a partir de las ideas", // lo que se queda: solo ideas
		"SEGURIDAD",                  // lo que se queda: antiinyección
	} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt B no contiene %q", want)
		}
	}
}

// proposePromptFixture es la entrada fija de los tests del prompt B: el largo del
// prompt se mide con ella.
func proposePromptFixture() ProposeCandidatesInput {
	return ProposeCandidatesInput{
		Artifacts: materialpipeline.ChunkArtifactsV1{
			Version:        1,
			MainIdeas:      []string{"La clorofila absorbe la luz"},
			SecondaryIdeas: []string{"Las hojas se ven verdes"},
			ChunkTopic:     "La clorofila",
		},
		Language: "es",
	}
}

func TestBuildProposeCandidatesPrompt_TypeByIdea(t *testing.T) {
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	for _, want := range []string{
		"un dato, un nombre o una fecha: short_answer o multiple_choice",
		"una afirmación que se puede negar: true_false",
		"una lista de elementos: multiple_select",
		"una causa, un proceso, una comparación o una consecuencia: open_ended",
		"NO hace falta usar todos los tipos ni seguir el orden de la lista",
		"dos preguntas del mismo tipo están bien si las ideas lo piden",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the type guide is missing %q", want)
		}
	}
}

func TestBuildProposeCandidatesPrompt_OpenEndedRubric(t *testing.T) {
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	for _, want := range []string{
		"open_ended: como mucho UNA por respuesta", // el tope por trozo
		"\"explanation\" es OBLIGATORIA",           // la rúbrica no es opcional en la abierta
		"rúbrica breve sacada de las ideas",        // de dónde sale
		"«Debe mencionar:»",                        // la forma que la preparación reconoce
		"open_ended: OMITE \"correct_answer\"",     // sigue sin respuesta correcta
		"salvo en open_ended, donde es la rúbrica", // la regla general no la contradice
		proposeExampleOpenEnded,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the open-ended rule is missing %q", want)
		}
	}
}

func TestBuildProposeCandidatesPrompt_MultipleSelectFromIdeas(t *testing.T) {
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	for _, want := range []string{
		"multiple_select SOLO si las ideas enumeran 2 o más elementos correctos",
		"cada opción correcta sale de las ideas",
		"\"correct_answer\" es un ARREGLO que copia esas opciones",
		proposeExampleMultipleSelect,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the multiple-select rule is missing %q", want)
		}
	}
}

// Medido en la HU-IA.8: sin esta línea el modelo numeraba "version" como un contador
// dentro de cada respuesta y el contrato descartaba todas las candidatas menos la primera.
func TestBuildProposeCandidatesPrompt_VersionIsAlwaysOne(t *testing.T) {
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	const rule = "\"version\" es SIEMPRE 1 en TODAS las candidatas: no es un contador ni un número de orden."
	if !strings.Contains(p, rule) {
		t.Errorf("the version rule is missing %q", rule)
	}
	// La forma y los dos ejemplos enseñan la misma versión, la única que admite el contrato.
	if n := strings.Count(p, "\"version\":1"); n != 3 {
		t.Errorf("the prompt shows \"version\":1 %d times, want 3 (the shape and the two examples)", n)
	}
	for _, other := range []string{"\"version\":2", "\"version\":0"} {
		if strings.Contains(p, other) {
			t.Errorf("the prompt shows %s", other)
		}
	}
}

// Un ejemplo roto enseñaría al modelo a romper el contrato: cada ejemplo que el
// prompt muestra pasa el mismo validador que las candidatas del modelo.
func TestBuildProposeCandidatesPrompt_ExamplesPassContract(t *testing.T) {
	examples := map[string]struct {
		raw      string
		wantType string
	}{
		"multiple_select": {proposeExampleMultipleSelect, "multiple_select"},
		"open_ended":      {proposeExampleOpenEnded, "open_ended"},
	}
	for name, ex := range examples {
		c, verr := materialpipeline.ValidateCandidatePayload([]byte(ex.raw))
		if verr != nil {
			t.Errorf("example %s breaks the contract: %v", name, verr)
			continue
		}
		if c.QuestionType != ex.wantType {
			t.Errorf("example %s has type %q, want %q", name, c.QuestionType, ex.wantType)
		}
		if len(c.SourceIdeas) == 0 {
			t.Errorf("example %s shows no source_ideas", name)
		}
	}

	// La abierta de ejemplo enseña lo que la regla pide: rúbrica y sin correcta.
	open, _ := materialpipeline.ValidateCandidatePayload([]byte(proposeExampleOpenEnded))
	if open == nil || !strings.HasPrefix(open.Explanation, "Debe mencionar:") {
		t.Errorf("the open-ended example does not carry a rubric in explanation")
	}
	if open != nil && len(open.CorrectAnswer) != 0 {
		t.Errorf("the open-ended example carries correct_answer: %s", open.CorrectAnswer)
	}
}

// proposePromptMaxRunes es el tope de largo del prompt B con proposePromptFixture
// (ADR 0048, punto 3). El prompt medía 2.924 runas antes de la HU-IA.8, 4.441 con su
// primer texto y 4.532 con la regla de "version"; el tope deja un margen corto para que alargarlo sea una decisión y no un
// descuido: cada línea de más alarga la llamada de «preguntar».
const proposePromptMaxRunes = 4600

func TestBuildProposeCandidatesPrompt_LengthCap(t *testing.T) {
	p := BuildProposeCandidatesPrompt(proposePromptFixture())
	if n := utf8.RuneCountInString(p); n > proposePromptMaxRunes {
		t.Errorf("the propose prompt is %d runes long, cap is %d", n, proposePromptMaxRunes)
	} else {
		t.Logf("propose prompt length: %d runes (cap %d)", n, proposePromptMaxRunes)
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
