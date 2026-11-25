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

## La medida del borrador (AR-IA.3)

Los modos `material` y `material-pairs` del arnés leen de aquí sus datos, **congelados antes de la
primera corrida** (su huella sha256 va en la cabecera de cada corrida):

- `independencia_hispanoamerica.html`, `.pdf` y `.txt` — **el PDF largo**: «La independencia de
  Hispanoamérica», unas 2.600 palabras en 8 secciones numeradas, escrito para esta medida (sin texto
  de terceros). El `.pdf` sale del `.html` con Chrome sin cabecera (receta de arriba) y es
  CID/Identity-H; el `.txt` es el mismo texto en plano, para leerlo. Con el troceo de producción da
  9 trozos: supera el cupo de 20 preguntas. **No se regenera**: Chrome no da el mismo PDF byte a byte
  y cambiaría la huella de la línea base.
- `cases.json` — los casos del modo `material`: el PDF largo, con el título literal de cada sección,
  y `fotosintesis.pdf` y `ciclo_del_agua.pdf` (los M2 y M1 de la corrida de punta a punta del
  2026-11-24). Un título se busca en el texto **extraído**, con los espacios colapsados y las
  ligaduras `ﬁ`/`ﬂ` deshechas; si no aparece, el arnés no corre.
- `pairs.json` — 40 parejas de preguntas sobre el PDF largo para el modo `material-pairs`, cada
  miembro con la forma completa de una candidata. Cuatro categorías de 10: `paraphrase` (la misma
  pregunta dicha de otra forma, `dup`), `other_type` (la misma pregunta en otro tipo, `dup`),
  `same_idea_other_question` (misma idea y pregunta distinta, o casi el mismo enunciado con otra
  respuesta, `no_dup`) y `different_section` (`no_dup`). **Etiquetadas por construcción** por quien
  las escribió; nadie las revisó (R-065). En `other_type` la etiqueta `dup` es criterio de quien las
  escribió: una afirmación de verdadero o falso y la pregunta que la pide se cuentan como la misma.
