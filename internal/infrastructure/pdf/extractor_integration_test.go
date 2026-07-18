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

// Fixtures reales en testdata/: PDFs de una página generados con pdfcpu
// (api.Create) desde los .txt hermanos, copiados de cmd/llm-harness/testdata/material.
// Ejercitan el camino completo ReadContext -> EnsurePageCount -> extracción de texto.
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

	for _, name := range []string{"ciclo_del_agua", "fotosintesis", "sistema_solar"} {
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

func uniqueWords(s string) map[string]bool {
	out := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:¿?¡!()\"'—–-")
		if len(w) >= 4 { // ignorar palabras muy cortas (artículos, etc.)
			out[w] = true
		}
	}
	return out
}
