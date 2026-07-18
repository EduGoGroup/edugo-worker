package materialpipeline

import "strings"

// NormalizeChunkArtifacts limpia los artefactos del chunk ANTES de validarlos
// (resiliencia fase A del pipeline, plan 043): filtra de main_ideas y secondary_ideas
// los ítems vacíos o de solo espacios que algunos modelos locales chicos emiten de
// forma intermitente (una lista con un "" degenerado en medio de ideas legítimas).
//
// NO afloja el contrato: es un saneamiento de forma, no de contenido. Si tras filtrar
// main_ideas queda vacía, ValidateChunkArtifacts seguirá fallando (main_ideas exige
// ≥1): normalizar rescata una lista que solo estaba sucia, nunca inventa una idea.
//
// Devuelve una COPIA normalizada; no muta el argumento (las listas filtradas son slices
// nuevos, así que el llamador puede seguir usando las originales sin sorpresas).
func NormalizeChunkArtifacts(a ChunkArtifactsV1) ChunkArtifactsV1 {
	a.MainIdeas = filterNonBlank(a.MainIdeas)
	a.SecondaryIdeas = filterNonBlank(a.SecondaryIdeas)
	return a
}

// filterNonBlank devuelve una copia de items sin los elementos vacíos o de solo
// espacios. Conserva el orden y el texto original de los que sí tienen contenido (no
// recorta ni altera lo legítimo). Una lista sin elementos a filtrar se devuelve tal cual.
func filterNonBlank(items []string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
