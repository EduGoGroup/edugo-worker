package materialpipeline

import "strings"

// deictic.go — detector determinista de referencias deícticas al contexto del prompt
// (deuda 043-candidatas-enunciados-no-autocontenidos). Un enunciado que dice «según las
// ideas proporcionadas» o «según el texto» referencia contexto interno del pipeline
// (main_ideas/chunk) que el alumno jamás ve: no es autocontenido. Ciego al modelo y
// gratis; lo consumen QualityPass (guard post-generación) y el harness (medición).

// deicticPhrases son las frases prohibidas, ya normalizadas (minúsculas, sin acentos,
// espacios simples). Lista CONSERVADORA: cada frase ancla al sustantivo del contexto del
// prompt (ideas/texto/material/fragmento/párrafo/visto/mencionado), nunca a «según» solo
// — «según la ley» o «según el reglamento» son legítimos en un manual de conducir.
var deicticPhrases = []string{
	// «las ideas» = las main_ideas del prompt B; el alumno no ve ideas de ningún tipo.
	"segun las ideas",
	"las ideas proporcionadas",
	"las ideas dadas",
	"las ideas mencionadas",
	"las ideas presentadas",
	"las ideas anteriores",
	// «el texto» del chunk (no se incluye «en el texto» pelado: un enunciado legítimo
	// podría hablar del texto de una señal).
	"segun el texto",
	"de acuerdo con el texto",
	"de acuerdo al texto",
	"conforme al texto",
	"el texto anterior",
	"el texto proporcionado",
	// «el material» que viaja por el pipeline.
	"segun el material",
	"en el material",
	"el material proporcionado",
	// unidades internas del porcionado.
	"segun el fragmento",
	"en el fragmento",
	"segun el parrafo",
	"en el parrafo anterior",
	// deixis temporal a un discurso previo que el alumno no compartió.
	"segun lo visto",
	"visto anteriormente",
	"vista anteriormente",
	"mencionado anteriormente",
	"mencionada anteriormente",
	"mencionados anteriormente",
	"mencionadas anteriormente",
	"se menciono anteriormente",
	"segun lo anterior",
}

// deicticNormalizer aplana el texto para el match: minúsculas se aplican antes; aquí
// solo se quitan los acentos del español (la ñ se conserva: distingue palabras).
var deicticNormalizer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
)

// normalizeDeictic normaliza un texto para compararlo contra deicticPhrases:
// minúsculas, sin acentos y con todo espacio en blanco colapsado a uno.
func normalizeDeictic(text string) string {
	s := deicticNormalizer.Replace(strings.ToLower(text))
	return strings.Join(strings.Fields(s), " ")
}

// DetectDeicticReference busca en el texto de un enunciado referencias deícticas al
// contexto del prompt. Devuelve la primera frase prohibida encontrada (normalizada,
// para logs/motivos) o "" si el enunciado es autocontenido en este aspecto.
func DetectDeicticReference(text string) string {
	norm := normalizeDeictic(text)
	if norm == "" {
		return ""
	}
	for _, phrase := range deicticPhrases {
		if strings.Contains(norm, phrase) {
			return phrase
		}
	}
	return ""
}
