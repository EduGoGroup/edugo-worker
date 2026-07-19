# Batería de relevancia (F2a / D-044.7)

Calibra el umbral `RelevanceMin` (score 0..1) de la pasada 2: dado un set de
`ideas` (main_ideas de un chunk) y una `question`, decidir si la pregunta es
`central` (se responde con esas ideas y ataca una idea principal), `peripheral`
(on-topic pero ataca un detalle secundario no cubierto por las ideas mostradas)
o `unanswerable` (las ideas no responden la pregunta).

## Cómo se construyó

Cruce de `_backups/pipeline-043-conaset-2026-07-18/{chunks,candidatas}.json` por
`chunk_id`. Las `source_ideas` de cada candidata coinciden literalmente con las
main_ideas o secondary_ideas de su chunk, lo que da la etiqueta base:

- `central`: pregunta real emparejada con las main_ideas de SU chunk; su fuente es una main_idea.
- `peripheral`: pregunta real + main_ideas de SU chunk, pero su fuente es una secondary_idea
  con bajo solapamiento de tokens contra las mains (detalle tangencial, no central).
- `unanswerable`: pregunta real contra main_ideas de OTRO chunk topicamente lejano
  (seq distante, overlap de tokens ~0), más 5 sintéticas de otro dominio (matemática/historia).

Cada caso: `{id, ideas[], question, label, source, note}`.

## Conteos

- Total: 60 — central 20, peripheral 20, unanswerable 20.
- source: conaset 55, synthetic 5.

## Variante agregada (`cases-aggregate.json`) — condición de producción

En producción la pasada 2 no recibe las ~3 ideas de UN chunk sino el AGREGADO de ideas del job.
Regla del agregado (fija, reproducible): pool base = main_ideas de una muestra de 25 seqs
distribuidas uniformemente por sequence (`round(i*162/24)`, i=0..24) sobre los 163 chunks; solo
aportan los 'done' (los 'failed' no tienen artifacts) → 19 chunks = 58 ideas. Cada caso
central/peripheral suma además las main_ideas de SU chunk de origen (siempre presente en el pool),
ordenado por seq → 58–64 ideas/caso. Los 20 unanswerable (15 de otros dominios + 5 sintéticas de v1)
reciben solo el pool base. Mismas 40 preguntas central/peripheral de v1 (ids con sufijo `-a`).
