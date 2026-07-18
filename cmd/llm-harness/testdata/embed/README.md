# Batería de calibración de embeddings (dedupe) — F1b / D-044.7

Pares etiquetados para (a) elegir modelo de embeddings (nomic-embed-text vs embeddinggemma)
y (b) calibrar los umbrales `dup_high` / `dup_low` del coseno sobre `question_text`
(con `correct_answer` como desempate en el harness). Etiqueta confiable **por construcción**.

Insumo: `_backups/pipeline-043-conaset-2026-07-18/candidatas.json` (404 candidatas reales del PDF
CONASET de seguridad vial, español; 5 tipos de pregunta). Cada par: `{id,a,b,label,difficulty,source,note}`.

Construcción:
- `dup` / `conaset-paraphrase` (30): pregunta real + paráfrasis genuina escrita a mano (sinónimos, reordenamiento, activa↔pasiva).
- `dup` / `synthetic` (10): pares de matemática escolar e historia, para chequear que el resultado no es específico de CONASET.
- `no_dup` / `easy` (30): dos preguntas reales de temas claramente distintos del mismo PDF.
- `no_dup` / `hard` (29): mismo tema/idea, pregunta distinta de fondo — 19 pares reales del CONASET
  (incl. redacción casi idéntica con respuesta distinta, p.ej. 264/265, 370/371) + 10 escritos a mano.

Conteos (99 pares): label → dup 40, no_dup 59 · label/dif → dup/easy 32, dup/hard 8,
no_dup/easy 30, no_dup/hard 29 · source → conaset-real 49, conaset-paraphrase 40, synthetic 10.
Los textos autorados llevan tildes correctas para no meter señal ortográfica artificial frente al corpus real.
