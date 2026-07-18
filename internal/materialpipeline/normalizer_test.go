package materialpipeline

import "testing"

func TestNormalizeChunkArtifacts_FiltraItemsVacios(t *testing.T) {
	in := ChunkArtifactsV1{
		Version:        1,
		MainIdeas:      []string{"idea real", "", "   ", "otra idea"},
		SecondaryIdeas: []string{"  ", "detalle"},
		ChunkTopic:     "tema",
	}

	got := NormalizeChunkArtifacts(in)

	if len(got.MainIdeas) != 2 || got.MainIdeas[0] != "idea real" || got.MainIdeas[1] != "otra idea" {
		t.Fatalf("main_ideas debería quedar solo con las no vacías en orden, got %#v", got.MainIdeas)
	}
	if len(got.SecondaryIdeas) != 1 || got.SecondaryIdeas[0] != "detalle" {
		t.Fatalf("secondary_ideas debería filtrar los vacíos, got %#v", got.SecondaryIdeas)
	}
}

func TestNormalizeChunkArtifacts_RescataListaSuciaParaValidar(t *testing.T) {
	// Una lista con ítems legítimos y un "" degenerado NO valida antes de normalizar
	// (main_ideas[1] no puede estar vacío) pero SÍ valida después: normalizar rescata
	// la salida sin aflojar el contrato.
	dirty := ChunkArtifactsV1{
		Version:    1,
		MainIdeas:  []string{"la fotosíntesis convierte luz en energía", ""},
		ChunkTopic: "fotosíntesis",
	}

	rawDirty, err := dirty.Marshal()
	if err != nil {
		t.Fatalf("marshal sucio: %v", err)
	}
	if _, verr := ValidateChunkArtifacts(rawDirty); verr == nil {
		t.Fatal("los artefactos con un ítem vacío deberían fallar la validación ANTES de normalizar")
	}

	rawClean, err := NormalizeChunkArtifacts(dirty).Marshal()
	if err != nil {
		t.Fatalf("marshal limpio: %v", err)
	}
	if _, verr := ValidateChunkArtifacts(rawClean); verr != nil {
		t.Fatalf("tras normalizar los artefactos deberían validar, got %v", verr)
	}
}

func TestNormalizeChunkArtifacts_MainVaciaSigueInvalida(t *testing.T) {
	// Si TODAS las main_ideas eran basura, normalizar deja la lista vacía y la
	// validación sigue fallando: no se afloja el contrato (main_ideas exige ≥1).
	allBlank := ChunkArtifactsV1{
		Version:    1,
		MainIdeas:  []string{"", "   "},
		ChunkTopic: "tema",
	}

	got := NormalizeChunkArtifacts(allBlank)
	if len(got.MainIdeas) != 0 {
		t.Fatalf("main_ideas de puros vacíos debería quedar vacía, got %#v", got.MainIdeas)
	}

	raw, err := got.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, verr := ValidateChunkArtifacts(raw); verr == nil {
		t.Fatal("main_ideas vacía tras normalizar DEBE seguir fallando la validación")
	}
}

func TestNormalizeChunkArtifacts_NoMutaElOriginal(t *testing.T) {
	original := ChunkArtifactsV1{
		Version:   1,
		MainIdeas: []string{"idea", ""},
	}
	_ = NormalizeChunkArtifacts(original)
	if len(original.MainIdeas) != 2 {
		t.Fatalf("NormalizeChunkArtifacts no debe mutar el argumento, got %#v", original.MainIdeas)
	}
}
