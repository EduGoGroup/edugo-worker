# testdata del harness modo material (plan 043 F3b)

Entradas para medir las llamadas A (digest) y B (candidatas) del pipeline
material→evaluación.

## Contenido

- `*.txt` — contenido educativo real en español, escrito a mano para esta prueba
  (ciclo del agua, sistema solar, fotosíntesis). No provienen de terceros. Son la
  **entrada por defecto** del modo material (`-material-inputs` vacío toma todos los
  `.txt` de esta carpeta). `fotosintesis.txt` está dimensionado para partir en 2 trozos
  con `chunking.DefaultConfig`, y así ejercita el **encadenado de summaries** (A del
  trozo N alimenta el `PrevSummary` del trozo N+1).
- `*.pdf` — generados desde los `.txt` hermanos con `gen_pdf.go` (pdfcpu `api.Create`,
  la misma librería vendorizada que usa el extractor del worker). Su única función es
  ejercitar el camino `pdf.Extractor` del harness.
- `gen_pdf.go` (`//go:build ignore`) — regenera los `.pdf`. Uso desde `edugo-worker/`:
  `go run ./cmd/llm-harness/testdata/material/gen_pdf.go`.

## Hallazgo abierto: el extractor de PDF rechaza estos PDFs

El extractor (`internal/infrastructure/pdf`) hoy **no puede leer** estos PDFs de una
sola página: `ExtractWithMetadata` lee `pdfCtx.PageCount` justo después de
`api.ReadContext`, pero `ReadContext` deja `PageCount` en 0 hasta que se invoca
`EnsurePageCount()` (o `ReadValidateAndOptimize`). Resultado: todo PDF recién creado se
rechaza como `ErrPDFEmpty` ("PDF vacío o corrupto"). Además, cuando el conteo funciona,
`PageContent` devuelve el **content-stream crudo** (operadores `BT/Tf/Tj/ET` con el
texto legible entre paréntesis), no texto limpio.

Ese paquete está **fuera del alcance de escritura de F3a/F3b** (solo `internal/llm/**` y
`cmd/llm-harness/**`); el arreglo corresponde a su dueño. Por eso la medición usa los
`.txt` como entrada, y el harness enruta `.pdf` → `pdf.Extractor` y `.txt` → texto plano.
