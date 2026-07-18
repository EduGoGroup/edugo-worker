package chunking

import "strings"

// Chunk es un trozo del texto porcionado. Seq es su posición 0-based dentro del
// documento; la concatenación de los Text en orden reconstruye el texto original
// salvo normalización de espacios (no hay solape ni pérdida).
type Chunk struct {
	Seq  int
	Text string
}

// block es la unidad atómica interna del porcionado: un párrafo (o un fragmento
// de un párrafo gigante partido por oraciones). isHeader marca si su primera
// línea parece un encabezado.
type block struct {
	text     string
	words    int
	isHeader bool
}

// chunkAcc acumula bloques mientras se arma un trozo.
type chunkAcc struct {
	blocks []block
	words  int
}

// text materializa el texto del trozo uniendo sus bloques con doble salto.
func (c chunkAcc) text() string {
	parts := make([]string, len(c.blocks))
	for i, b := range c.blocks {
		parts[i] = b.text
	}
	return strings.Join(parts, "\n\n")
}

// Split porciona text en trozos según cfg. Es pura y determinista: mismo texto
// y misma cfg producen siempre el mismo resultado. Reglas:
//   - Texto vacío o solo espacios devuelve un slice vacío.
//   - Texto con menos de MinWords palabras devuelve un único trozo.
//   - Los cortes caen en encabezados y líneas en blanco; un encabezado abre
//     trozo nuevo solo si el actual ya alcanzó MinWords.
//   - Se acumula hasta ~TargetWords sin pasar MaxWords; un párrafo mayor que
//     MaxWords se parte por oraciones (nunca por palabras).
//   - Sin solape entre trozos; los restos por debajo de MergeThresholdWords se
//     fusionan con el vecino (el anterior por defecto).
func Split(text string, cfg Config) []Chunk {
	cfg = cfg.normalized()

	text = normalizeNewlines(text)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	paragraphs := splitParagraphs(text)
	blocks := buildBlocks(paragraphs, cfg)
	if len(blocks) == 0 {
		return nil
	}

	accs := packBlocks(blocks, cfg)
	accs = mergeSmall(accs, cfg)

	chunks := make([]Chunk, len(accs))
	for i, a := range accs {
		chunks[i] = Chunk{Seq: i, Text: a.text()}
	}
	return chunks
}

// normalizeNewlines unifica los saltos de línea a "\n".
func normalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// splitParagraphs parte el texto en párrafos por líneas en blanco (doble salto),
// descartando los vacíos y recortando espacios. Varias líneas en blanco seguidas
// cuentan como una sola frontera.
func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}

// firstLineOf devuelve la primera línea no vacía de un párrafo.
func firstLineOf(paragraph string) string {
	line := paragraph
	if idx := strings.IndexByte(paragraph, '\n'); idx >= 0 {
		line = paragraph[:idx]
	}
	return strings.TrimSpace(line)
}

// buildBlocks convierte párrafos en bloques atómicos. Un párrafo normal es un
// bloque; uno que supera MaxWords se parte en varios preservando el orden y sin
// romper palabras (prefiere fronteras de oración).
func buildBlocks(paragraphs []string, cfg Config) []block {
	var blocks []block
	for _, p := range paragraphs {
		header := isTitleLine(firstLineOf(p))
		words := strings.Fields(p)
		if len(words) <= cfg.MaxWords {
			blocks = append(blocks, block{text: p, words: len(words), isHeader: header})
			continue
		}
		// Párrafo gigante: se reparte por oraciones/palabras.
		for gi, group := range splitWordsIntoBlocks(words, cfg) {
			blocks = append(blocks, block{
				text:     strings.Join(group, " "),
				words:    len(group),
				isHeader: header && gi == 0,
			})
		}
	}
	return blocks
}

// splitWordsIntoBlocks reparte una secuencia de palabras en grupos apuntando a
// TargetWords, cerrando el grupo en la primera frontera de oración a partir del
// objetivo. Si se alcanza MaxWords sin frontera, corta igual (nunca parte una
// palabra: cada corte cae entre palabras).
func splitWordsIntoBlocks(words []string, cfg Config) [][]string {
	var groups [][]string
	var cur []string
	for _, w := range words {
		cur = append(cur, w)
		n := len(cur)
		switch {
		case n >= cfg.MaxWords:
			groups = append(groups, cur)
			cur = nil
		case n >= cfg.TargetWords && endsSentence(w):
			groups = append(groups, cur)
			cur = nil
		}
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}

// endsSentence indica si la palabra cierra una oración (ignora comillas y
// cierres de paréntesis/corchetes al final).
func endsSentence(w string) bool {
	w = strings.TrimRight(w, "\"')]}»›")
	return strings.HasSuffix(w, ".") ||
		strings.HasSuffix(w, "!") ||
		strings.HasSuffix(w, "?") ||
		strings.HasSuffix(w, "…")
}

// packBlocks empaqueta bloques en trozos. Cierra el trozo actual y abre uno
// nuevo cuando: (a) el bloque es un encabezado y el actual ya alcanzó MinWords,
// (b) agregar el bloque superaría MaxWords, o (c) el actual ya alcanzó
// TargetWords. Nunca hay solape: cada bloque va a un solo trozo.
func packBlocks(blocks []block, cfg Config) []chunkAcc {
	var accs []chunkAcc
	var cur chunkAcc
	for _, b := range blocks {
		if len(cur.blocks) > 0 {
			startNew := false
			switch {
			case b.isHeader && cur.words >= cfg.MinWords:
				startNew = true
			case cur.words+b.words > cfg.MaxWords:
				startNew = true
			case cur.words >= cfg.TargetWords:
				startNew = true
			}
			if startNew {
				accs = append(accs, cur)
				cur = chunkAcc{}
			}
		}
		cur.blocks = append(cur.blocks, b)
		cur.words += b.words
	}
	if len(cur.blocks) > 0 {
		accs = append(accs, cur)
	}
	return accs
}

// mergeSmall fusiona los trozos por debajo de MergeThresholdWords con su vecino:
// el anterior por defecto y, si no hay anterior, el siguiente. Preserva el orden
// del texto. Un único trozo se devuelve tal cual (caso texto corto).
func mergeSmall(accs []chunkAcc, cfg Config) []chunkAcc {
	for len(accs) > 1 {
		idx := -1
		for i, a := range accs {
			if a.words < cfg.MergeThresholdWords {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		if idx > 0 {
			accs[idx-1] = merge(accs[idx-1], accs[idx])
			accs = append(accs[:idx], accs[idx+1:]...)
		} else {
			accs[idx+1] = merge(accs[idx], accs[idx+1])
			accs = append(accs[:idx], accs[idx+1:]...)
		}
	}
	return accs
}

// merge concatena dos acumuladores conservando el orden (a antes que b).
func merge(a, b chunkAcc) chunkAcc {
	return chunkAcc{
		blocks: append(append([]block{}, a.blocks...), b.blocks...),
		words:  a.words + b.words,
	}
}
