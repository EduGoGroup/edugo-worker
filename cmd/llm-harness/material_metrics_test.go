package main

import (
	"encoding/json"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

func TestAnswerInChunk_ByType(t *testing.T) {
	chunk := "El 7 de agosto de 1819 Bolívar venció en la batalla de Boyacá y entró en Bogotá."
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	cases := []struct {
		name string
		c    materialpipeline.CandidatePayloadV1
		want []string
	}{
		{"corta, toda (sin tildes ni mayúsculas)", materialpipeline.CandidatePayloadV1{QuestionType: "short_answer", CorrectAnswer: raw("la batalla de BOYACA")}, []string{supportFull}},
		{"corta, a medias", materialpipeline.CandidatePayloadV1{QuestionType: "short_answer", CorrectAnswer: raw("Boyacá y Carabobo")}, []string{supportPartial}},
		{"opción múltiple, no", materialpipeline.CandidatePayloadV1{QuestionType: "multiple_choice", CorrectAnswer: raw("San Martín cruzó los Andes")}, []string{supportNone}},
		{"selección múltiple, opción por opción", materialpipeline.CandidatePayloadV1{QuestionType: "multiple_select", CorrectAnswer: raw([]string{"Bogotá", "Lima"})}, []string{supportFull, supportNone}},
		{"verdadero o falso no aplica", materialpipeline.CandidatePayloadV1{QuestionType: "true_false", CorrectAnswer: raw("true")}, []string{supportNotApplicable}},
		{"abierta no aplica", materialpipeline.CandidatePayloadV1{QuestionType: "open_ended"}, []string{supportNotApplicable}},
		{"respuesta solo de palabras vacías", materialpipeline.CandidatePayloadV1{QuestionType: "short_answer", CorrectAnswer: raw("de la")}, []string{supportNotApplicable}},
	}
	for _, tc := range cases {
		got := answerInChunk(tc.c, chunk)
		if len(got) != len(tc.want) {
			t.Errorf("%s: %v, se espera %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: %v, se espera %v", tc.name, got, tc.want)
			}
		}
	}
}

func TestSectionCoverage_ChunkBelongsWhereItStarts(t *testing.T) {
	// El texto extraído trae ligaduras y saltos que el título declarado no tiene.
	text := "Documento de prueba 1. El ﬁnal  del\nsiglo uno dos tres 2. Las causas cuatro cinco seis siete 3. El cierre ocho nueve"
	titles := []string{"1. El final del siglo", "2. Las causas", "3. El cierre"}
	sections, err := sectionOffsets(text, titles)
	if err != nil {
		t.Fatalf("sectionOffsets: %v", err)
	}
	chunks := []string{
		"Documento de prueba 1. El ﬁnal del siglo uno", // empieza antes del primer título: cuenta en la sección 0
		"dos tres 2. Las causas cuatro",                // empieza en la sección 0 aunque contenga el título de la 1
		"cinco seis siete 3. El cierre",                // empieza en la sección 1
		"ocho nueve",                                   // empieza en la sección 2
	}
	offsets := chunkOffsets(text, chunks)
	want := []int{0, 0, 1, 2}
	for i, off := range offsets {
		if got := sectionOfOffset(off, sections); got != want[i] {
			t.Errorf("trozo %d: sección %d, se espera %d", i, got, want[i])
		}
	}
	if got := sectionOfOffset(10, nil); got != -1 {
		t.Errorf("sin secciones declaradas: %d, se espera -1", got)
	}
	if _, err := sectionOffsets(text, []string{"1. El final del siglo", "9. No existe"}); err == nil {
		t.Error("un título que no aparece debe dar error")
	}
	if _, err := sectionOffsets(text, []string{"2. Las causas", "1. El final del siglo"}); err == nil {
		t.Error("los títulos fuera de orden deben dar error")
	}
}

func TestCandidatesPerChunkDistribution(t *testing.T) {
	d := newDistribution([]int{4, 2, 4, 3})
	if d.Min != 2 || d.Max != 4 || d.Median != 3.5 {
		t.Fatalf("mín/mediana/máx: %d/%v/%d, se espera 2/3.5/4", d.Min, d.Median, d.Max)
	}
	if d.Histogram[4] != 2 || d.Histogram[2] != 1 || d.Histogram[3] != 1 || len(d.Histogram) != 3 {
		t.Fatalf("reparto: %v", d.Histogram)
	}
	if got := histogramString(d.Histogram); got != "2×1 3×1 4×2" {
		t.Fatalf("reparto escrito: %q", got)
	}
	if odd := newDistribution([]int{5, 1, 3}); odd.Median != 3 {
		t.Fatalf("mediana impar: %v", odd.Median)
	}
	if empty := newDistribution(nil); empty.Min != 0 || empty.Max != 0 || len(empty.Histogram) != 0 {
		t.Fatalf("reparto vacío: %+v", empty)
	}
}

func TestSourceIdeasSubset(t *testing.T) {
	given := []string{"La batalla de Boyacá fue en 1819.", "Bolívar cruzó los Andes"}
	total, ok := sourceIdeasGiven([]string{"la batalla de boyaca fue en 1819", "Una idea que el modelo inventó"}, given)
	if total != 2 || ok != 1 {
		t.Fatalf("dadas %d de %d, se espera 1 de 2", ok, total)
	}
	if total, ok := sourceIdeasGiven(nil, given); total != 0 || ok != 0 {
		t.Fatalf("sin source_ideas: %d de %d", ok, total)
	}
}
