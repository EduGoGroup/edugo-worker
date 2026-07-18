# testdata del harness modo material (plan 043 F3b · deuda 038)

Entradas para medir las llamadas A (digest) y B (candidatas) del pipeline
material→evaluación.

## Contenido

- `*.txt` — contenido educativo real en español, escrito a mano para esta prueba
  (ciclo del agua, sistema solar, fotosíntesis, la célula, los ecosistemas, la energía).
  No provienen de terceros. Son la **entrada por defecto** del modo material
  (`-material-inputs` vacío toma todos los `.txt` de esta carpeta). `fotosintesis.txt`
  está dimensionado para partir en 2 trozos con `chunking.DefaultConfig`, y así ejercita
  el **encadenado de summaries** (A del trozo N alimenta el `PrevSummary` del trozo N+1).
- `*.pdf` — dos familias de fuente, para ejercitar el camino `pdf.Extractor` del harness
  con los dos casos que importan:
  - **WinAnsi** (`ciclo_del_agua`, `fotosintesis`, `sistema_solar`): un byte por glifo.
    Generados originalmente con pdfcpu; **committeados, no se regeneran** (pdfcpu ya no es
    dependencia del worker tras la deuda 038).
  - **Type0/CID Identity-H** (`la_celula`, `los_ecosistemas`, `la_energia`): dos bytes por
    glifo vía CMap, el caso típico de exportar a PDF desde navegador/procesador moderno.
    Era el caso que el extractor no sabía leer (deuda 038); ahora sí. Generados con Chrome
    headless a partir de los `.html` hermanos (ver «Regeneración» abajo).
- `*.html` — fuente de los PDF CID. Se conservan para poder regenerar el `.pdf` si hiciera
  falta.

## Regeneración de los PDF CID (con Chrome headless)

Los `.pdf` CID se generan desde su `.html` hermano con Chrome en modo headless, que embebe
las fuentes como **Type0/CID con codificación Identity-H** (verificable con `pdffonts`):

```sh
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
for n in la_celula los_ecosistemas la_energia; do
  "$CHROME" --headless --disable-gpu --no-pdf-header-footer \
    --print-to-pdf="$PWD/$n.pdf" "file://$PWD/$n.html"
  pdffonts "$n.pdf"   # debe mostrar «CID TrueType / Identity-H» en todas las fuentes
done
```

> Los `.pdf` ya committeados **no se regeneran** salvo necesidad: al reexportar, Chrome
> puede reordenar el layout (saltos de línea, subsetting de fuentes) y desalinear los
> asserts de contenido de `internal/infrastructure/pdf/extractor_integration_test.go`.
> Si se regeneran, revisar esos asserts.

> Nota de procedencia: el generador anterior `gen_pdf.go` (pdfcpu `api.Create`, solo
> WinAnsi) se retiró junto con la dependencia pdfcpu en la deuda 038.
