package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDigestSummaryPrompt_SoloSummaryYTopic(t *testing.T) {
	prev := "El tema es la señalización vial; ya se definió zona de escuela."
	p := BuildDigestSummaryPrompt(DigestChunkInput{
		ChunkText:   "CUERPO DEL TROZO",
		PrevSummary: &prev,
		Language:    "es",
	})
	for _, want := range []string{
		"CUERPO DEL TROZO",       // el trozo va delimitado
		"chunk_topic",            // forma de salida
		"summary",                // forma de salida
		"120",                    // tope del summary
		"OTRO MODELO",            // el summary es para otro LLM
		"NUNCA lo dejes vacío",   // el refuerzo anti-degeneración medido
		"RESUMEN DE LO ANTERIOR", // se inyecta el prev summary
		prev,                     // el contenido del prev summary
		"SEGURIDAD",              // anti-injection
	} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt A1 no contiene %q", want)
		}
	}
	// La tarea partida NO pide ideas en A1: pedir de más es lo que degenera al 4B.
	if strings.Contains(p, "main_ideas") || strings.Contains(p, "secondary_ideas") {
		t.Error("el prompt A1 no debe pedir ideas (esa es la tarea de A2)")
	}
}

func TestBuildDigestIdeasPrompt_SoloIdeas(t *testing.T) {
	p := BuildDigestIdeasPrompt(DigestChunkInput{ChunkText: "trozo inicial"})
	for _, want := range []string{
		"trozo inicial",
		"main_ideas",
		"secondary_ideas",
		"SEGURIDAD",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("el prompt A2 no contiene %q", want)
		}
	}
	// A2 no pide summary ni topic (van en A1) y sin PrevSummary no inyecta la sección.
	if strings.Contains(p, "\"summary\"") || strings.Contains(p, "chunk_topic") {
		t.Error("el prompt A2 no debe pedir summary ni chunk_topic (esa es la tarea de A1)")
	}
	if strings.Contains(p, "RESUMEN DE LO ANTERIOR") {
		t.Error("el primer trozo (PrevSummary nil) no debe incluir la sección de resumen anterior")
	}
}

func TestParseDigestParts_Fiel(t *testing.T) {
	s, err := ParseDigestSummaryPart(json.RawMessage(`{"version":1,"chunk_topic":"Señales","summary":"  resumen encadenable  "}`))
	if err != nil {
		t.Fatalf("A1 válida no debería fallar: %v", err)
	}
	if s.ChunkTopic != "Señales" || s.Summary != "resumen encadenable" {
		t.Errorf("A1 mal parseada (el summary debe venir recortado): %+v", s)
	}

	i, err := ParseDigestIdeasPart(json.RawMessage(`{"version":1,"main_ideas":["a","b"],"secondary_ideas":[]}`))
	if err != nil {
		t.Fatalf("A2 válida no debería fallar: %v", err)
	}
	if len(i.MainIdeas) != 2 || len(i.SecondaryIdeas) != 0 {
		t.Errorf("A2 mal parseada: %+v", i)
	}

	if _, err := ParseDigestSummaryPart(json.RawMessage(`no-json`)); err == nil {
		t.Error("A1 no parseable debería fallar")
	}
	if _, err := ParseDigestIdeasPart(json.RawMessage(`[]`)); err == nil {
		t.Error("A2 con forma equivocada debería fallar")
	}
}

func TestCombineDigestParts_EnsamblaElContratoCompleto(t *testing.T) {
	got := CombineDigestParts(
		DigestSummaryPart{Version: 1, ChunkTopic: "Señales", Summary: "resumen"},
		DigestIdeasPart{Version: 1, MainIdeas: []string{"idea"}, SecondaryIdeas: []string{"detalle"}},
	)
	if got.Artifacts.Version != 1 || got.Artifacts.ChunkTopic != "Señales" || got.Summary != "resumen" {
		t.Errorf("ensamblado incompleto: %+v", got)
	}
	if len(got.Artifacts.MainIdeas) != 1 || len(got.Artifacts.SecondaryIdeas) != 1 {
		t.Errorf("las ideas de A2 no llegaron al ensamblado: %+v", got.Artifacts)
	}
}

func TestCombineDigestParts_PropagaVersionInvalida(t *testing.T) {
	// Si cualquiera de las dos mitades devolvió versión != 1, el ensamblado la propaga
	// para que el validador del caller la castigue (medición honesta).
	if got := CombineDigestParts(DigestSummaryPart{Version: 2}, DigestIdeasPart{Version: 1}); got.Artifacts.Version != 2 {
		t.Errorf("la versión inválida de A1 debe propagarse, got %d", got.Artifacts.Version)
	}
	if got := CombineDigestParts(DigestSummaryPart{Version: 1}, DigestIdeasPart{Version: 0}); got.Artifacts.Version != 0 {
		t.Errorf("la versión inválida de A2 debe propagarse, got %d", got.Artifacts.Version)
	}
}
