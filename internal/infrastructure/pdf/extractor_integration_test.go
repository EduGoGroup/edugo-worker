package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures reales en testdata/, dos familias de fuente que ejercitan el extractor
// ledongthuc/pdf de punta a punta (NewReader -> NumPage -> Page.GetPlainText):
//   - WinAnsi (ciclo_del_agua, fotosintesis, sistema_solar): un byte por glifo.
//   - Type0/CID Identity-H (la_celula, los_ecosistemas, la_energia): dos bytes por
//     glifo vía CMap, generados con Chrome headless (ver testdata/../material/README).
//     Este era el caso que el parser propio anterior no sabía leer (deuda 038).
func TestExtractWithMetadata_RealPDFs(t *testing.T) {
	cases := []struct {
		file     string
		keywords []string // palabras que deben aparecer en el texto extraído
	}{
		{
			file:     "ciclo_del_agua.pdf",
			keywords: []string{"ciclo del agua", "hidrológico", "evaporación", "Tierra"},
		},
		{
			file:     "fotosintesis.pdf",
			keywords: []string{"fotosíntesis", "glucosa", "oxígeno", "clorofila"},
		},
		{
			file:     "sistema_solar.pdf",
			keywords: []string{"sistema solar", "Sol", "planetas", "gravedad"},
		},
		{
			file:     "la_celula.pdf",
			keywords: []string{"célula", "núcleo", "mitocondrias", "membrana"},
		},
		{
			file:     "los_ecosistemas.pdf",
			keywords: []string{"ecosistema", "cadenas alimentarias", "biodiversidad", "hábitat"},
		},
		{
			file:     "la_energia.pdf",
			keywords: []string{"energía", "eléctrica", "hidroeléctricas", "eólica"},
		},
	}

	ex := NewExtractor(newTestLogger())

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.file))
			require.NoError(t, err)

			res, err := ex.ExtractWithMetadata(context.Background(), bytes.NewReader(data))
			require.NoError(t, err, "el PDF de una página no debe rechazarse")
			require.NotNil(t, res)

			assert.Equal(t, 1, res.PageCount, "PDF de una sola página")
			assert.Greater(t, res.WordCount, 50, "debe extraer texto sustancial")
			assert.NotEmpty(t, strings.TrimSpace(res.Text))
			assert.False(t, res.IsScanned)

			lower := strings.ToLower(res.Text)
			for _, kw := range tc.keywords {
				assert.Containsf(t, lower, strings.ToLower(kw),
					"el texto extraído debe contener %q", kw)
			}

			// El texto extraído no debe contener operadores crudos del content-stream.
			assert.NotContains(t, res.Text, " Tj ")
			assert.NotContains(t, res.Text, " BT ")
		})
	}
}

// El texto extraído del PDF debe aproximarse al .txt fuente (mismo contenido,
// solo re-envuelto por el generador). Comprobamos que comparten la mayoría de
// palabras significativas.
func TestExtractWithMetadata_MatchesSourceText(t *testing.T) {
	ex := NewExtractor(newTestLogger())

	for _, name := range []string{
		"ciclo_del_agua", "fotosintesis", "sistema_solar",
		"la_celula", "los_ecosistemas", "la_energia",
	} {
		t.Run(name, func(t *testing.T) {
			pdfData, err := os.ReadFile(filepath.Join("testdata", name+".pdf"))
			require.NoError(t, err)
			txtData, err := os.ReadFile(filepath.Join("testdata", name+".txt"))
			require.NoError(t, err)

			res, err := ex.ExtractWithMetadata(context.Background(), bytes.NewReader(pdfData))
			require.NoError(t, err)

			srcWords := uniqueWords(string(txtData))
			gotWords := uniqueWords(res.Text)

			var hits int
			for w := range srcWords {
				if gotWords[w] {
					hits++
				}
			}
			ratio := float64(hits) / float64(len(srcWords))
			assert.Greaterf(t, ratio, 0.9,
				"el texto extraído debe cubrir >90%% de las palabras del .txt fuente (cobertura=%.2f)", ratio)
		})
	}
}

// TestExtractWithMetadata_CIDContent es la prueba de regresión de la deuda 038: los
// PDFs Type0/CID (Identity-H) generados por navegadores/procesadores modernos deben
// entregar texto español legible. Comprueba frases exactas con acentos, eñes, signos
// de apertura (¿ ¡) y símbolos (€, ½) que el parser WinAnsi propio anterior no leía.
func TestExtractWithMetadata_CIDContent(t *testing.T) {
	cases := []struct {
		file    string
		phrases []string
	}{
		{
			file: "la_celula.pdf",
			phrases: []string{
				"La célula es la unidad más pequeña",
				"¿Qué hay dentro de una célula?",
				"¡Es como una pequeña ciudad amurallada!",
				"un niño en",
				"dañados", // eñe
				"300 €",
				"½ milímetro",
			},
		},
		{
			file: "los_ecosistemas.pdf",
			phrases: []string{
				"Un ecosistema es el conjunto formado",
				"¿Cómo se relacionan los seres vivos?",
				"¡Cada especie cumple",
				"su hábitat",
				"el océano",
				"cientos de €",
				"½ de las especies",
			},
		},
		{
			file: "la_energia.pdf",
			phrases: []string{
				"La energía es la capacidad",
				"¿De qué formas se presenta la energía?",
				"¡Una caída de agua se convierte en electricidad",
				"mañana", // eñe
				"la solar y la eólica",
				"200 €",
				"½ del consumo",
			},
		},
	}

	ex := NewExtractor(newTestLogger())

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tc.file))
			require.NoError(t, err)

			res, err := ex.ExtractWithMetadata(context.Background(), bytes.NewReader(data))
			require.NoError(t, err, "un PDF CID/Identity-H debe extraerse, no rechazarse")
			require.NotNil(t, res)

			for _, p := range tc.phrases {
				assert.Containsf(t, res.Text, p,
					"el texto extraído del PDF CID debe contener %q", p)
			}
		})
	}
}

func uniqueWords(s string) map[string]bool {
	out := make(map[string]bool)
	for w := range strings.FieldsSeq(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:¿?¡!()\"'—–-")
		if len(w) >= 4 { // ignorar palabras muy cortas (artículos, etc.)
			out[w] = true
		}
	}
	return out
}
