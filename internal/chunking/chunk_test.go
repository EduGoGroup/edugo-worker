package chunking

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// --- Helpers de construcción de textos representativos ---------------------

// sentence devuelve una oración en español de ~n palabras terminada en punto.
func sentence(n int) string {
	base := []string{
		"la", "célula", "es", "la", "unidad", "estructural", "y", "funcional",
		"de", "todos", "los", "seres", "vivos", "que", "conocemos", "en",
		"nuestro", "planeta", "según", "la", "teoría", "celular", "moderna",
	}
	words := make([]string, 0, n)
	for i := 0; i < n; i++ {
		words = append(words, base[i%len(base)])
	}
	return strings.Join(words, " ") + "."
}

// paragraph arma un párrafo de aproximadamente n palabras encadenando oraciones.
func paragraph(n int) string {
	var b strings.Builder
	count := 0
	for count < n {
		s := sentence(12)
		if count > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s)
		count += 12
	}
	return b.String()
}

// wordCount cuenta palabras al estilo de la implementación.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// --- Utilidades de aserción -------------------------------------------------

// assertCoverage verifica que la concatenación de trozos conserve exactamente
// las palabras del original (mismo orden, sin pérdida ni duplicado): esto prueba
// cobertura total y ausencia de solape a la vez.
func assertCoverage(t *testing.T, original string, chunks []Chunk) {
	t.Helper()
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}
	gotWords := strings.Fields(strings.Join(texts, " "))
	wantWords := strings.Fields(normalizeNewlines(original))
	if !reflect.DeepEqual(gotWords, wantWords) {
		t.Fatalf("cobertura rota: %d palabras en trozos vs %d en original", len(gotWords), len(wantWords))
	}
}

// assertSeq verifica que los Seq sean consecutivos desde 0.
func assertSeq(t *testing.T, chunks []Chunk) {
	t.Helper()
	for i, c := range chunks {
		if c.Seq != i {
			t.Fatalf("Seq[%d] = %d, quería %d", i, c.Seq, i)
		}
	}
}

// assertSizes verifica los rangos de tamaño: todos los trozos dentro de
// [MergeThreshold, MaxWords+MergeThreshold], salvo el caso de un único trozo.
func assertSizes(t *testing.T, chunks []Chunk, cfg Config) {
	t.Helper()
	if len(chunks) <= 1 {
		return
	}
	upper := cfg.MaxWords + cfg.MergeThresholdWords
	for _, c := range chunks {
		n := wordCount(c.Text)
		if n < cfg.MergeThresholdWords {
			t.Errorf("trozo %d con %d palabras < umbral de fusión %d", c.Seq, n, cfg.MergeThresholdWords)
		}
		if n > upper {
			t.Errorf("trozo %d con %d palabras > tope %d", c.Seq, n, upper)
		}
	}
}

// --- Tests -----------------------------------------------------------------

func TestSplit_Determinism(t *testing.T) {
	cfg := DefaultConfig()
	var sections []string
	for i := 1; i <= 6; i++ {
		sections = append(sections, fmt.Sprintf("%d. Sección número %d", i, i))
		sections = append(sections, paragraph(400))
	}
	text := strings.Join(sections, "\n\n")

	a := Split(text, cfg)
	b := Split(text, cfg)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Split no es determinista: dos llamadas dieron resultados distintos")
	}
}

func TestSplit_Empty(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\n\n", "  \r\n  \t \n "} {
		if got := Split(in, DefaultConfig()); len(got) != 0 {
			t.Errorf("Split(%q) devolvió %d trozos, quería 0", in, len(got))
		}
	}
}

func TestSplit_ShortTextSingleChunk(t *testing.T) {
	cfg := DefaultConfig()
	text := paragraph(120) // muy por debajo de MinWords (500)
	chunks := Split(text, cfg)
	if len(chunks) != 1 {
		t.Fatalf("texto corto debería dar 1 trozo, dio %d", len(chunks))
	}
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
}

func TestSplit_ShortTextMultiParagraphSingleChunk(t *testing.T) {
	cfg := DefaultConfig()
	// Tres párrafos chicos que suman < MinWords: deben quedar en un solo trozo.
	text := strings.Join([]string{paragraph(60), paragraph(60), paragraph(60)}, "\n\n")
	chunks := Split(text, cfg)
	if len(chunks) != 1 {
		t.Fatalf("texto corto multipárrafo debería dar 1 trozo, dio %d", len(chunks))
	}
	assertCoverage(t, text, chunks)
}

func TestSplit_PlainNoHeaders(t *testing.T) {
	cfg := DefaultConfig()
	// ~3000 palabras en párrafos de ~150 sin encabezados.
	var paras []string
	for i := 0; i < 20; i++ {
		paras = append(paras, paragraph(150))
	}
	text := strings.Join(paras, "\n\n")

	chunks := Split(text, cfg)
	if len(chunks) < 3 {
		t.Fatalf("texto plano de ~3000 palabras debería dar varios trozos, dio %d", len(chunks))
	}
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)
}

func TestSplit_HeadersOpenChunks(t *testing.T) {
	cfg := DefaultConfig()
	// Documento con encabezados numerados; cada sección tiene ~600 palabras,
	// suficiente para que el siguiente encabezado abra un trozo nuevo.
	headers := []string{
		"1. Introducción a la biología celular",
		"2. La membrana plasmática",
		"3. El núcleo y el material genético",
		"4. Mitocondrias y respiración",
	}
	var parts []string
	for _, h := range headers {
		parts = append(parts, h)
		parts = append(parts, paragraph(600))
	}
	text := strings.Join(parts, "\n\n")

	chunks := Split(text, cfg)
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)

	// Cada trozo (salvo un eventual preámbulo) debería comenzar en un encabezado.
	headerStarts := 0
	for _, c := range chunks {
		if isTitleLine(firstLineOf(c.Text)) {
			headerStarts++
		}
	}
	if headerStarts < len(headers) {
		t.Errorf("se esperaban al menos %d trozos iniciando en encabezado, hubo %d", len(headers), headerStarts)
	}
}

func TestSplit_HeaderKeepsWithContentIfSmall(t *testing.T) {
	cfg := DefaultConfig()
	// Encabezado seguido de una sección chica (< MinWords): el encabezado NO debe
	// forzar corte contra un trozo previo aún incompleto.
	text := strings.Join([]string{
		paragraph(200),
		"2. Una sección breve",
		paragraph(120),
	}, "\n\n")

	chunks := Split(text, cfg)
	if len(chunks) != 1 {
		t.Fatalf("con secciones chicas debería consolidarse en 1 trozo, dio %d", len(chunks))
	}
	assertCoverage(t, text, chunks)
}

func TestSplit_GiantParagraph(t *testing.T) {
	cfg := DefaultConfig()
	// Un único párrafo gigante (~2000 palabras) sin líneas en blanco.
	text := paragraph(2000)

	chunks := Split(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("un párrafo gigante debería partirse en varios trozos, dio %d", len(chunks))
	}
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)
}

func TestSplit_GiantParagraphNeverSplitsWords(t *testing.T) {
	cfg := DefaultConfig()
	text := paragraph(2000)
	original := strings.Fields(text)

	chunks := Split(text, cfg)
	// Recolectar todas las palabras de los trozos y comparar 1:1 con el original.
	var got []string
	for _, c := range chunks {
		got = append(got, strings.Fields(c.Text)...)
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatal("el porcionado de un párrafo gigante alteró o partió palabras")
	}
}

func TestSplit_AllCapsHeaders(t *testing.T) {
	cfg := DefaultConfig()
	text := strings.Join([]string{
		"INTRODUCCIÓN",
		paragraph(600),
		"DESARROLLO DEL TEMA",
		paragraph(600),
		"CONCLUSIONES FINALES",
		paragraph(600),
	}, "\n\n")

	chunks := Split(text, cfg)
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)
	if len(chunks) < 3 {
		t.Fatalf("con tres secciones grandes en MAYÚSCULAS deberían salir >= 3 trozos, dio %d", len(chunks))
	}
}

func TestSplit_WithLists(t *testing.T) {
	cfg := DefaultConfig()
	list := strings.Join([]string{
		"- primer elemento de la lista de repaso",
		"- segundo elemento de la lista de repaso",
		"- tercer elemento de la lista de repaso",
	}, "\n")
	text := strings.Join([]string{
		"1. Guía de estudio",
		paragraph(550),
		list,
		paragraph(550),
	}, "\n\n")

	chunks := Split(text, cfg)
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)
}

func TestSplit_MergesSmallRemainder(t *testing.T) {
	cfg := DefaultConfig()
	// Dos secciones grandes y una cola diminuta que debe fusionarse con el vecino.
	text := strings.Join([]string{
		"1. Primera sección",
		paragraph(700),
		"2. Segunda sección",
		paragraph(700),
		"3. Coda",
		paragraph(40), // resto muy chico
	}, "\n\n")

	chunks := Split(text, cfg)
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg) // ningún trozo por debajo del umbral de fusión
}

func TestSplit_ConfigNormalization(t *testing.T) {
	// Config completamente inválida: debe caer a defaults y no paniquear.
	bad := Config{TargetWords: 0, MaxWords: -5, MinWords: 0, MergeThresholdWords: -1}
	text := paragraph(1500)
	chunks := Split(text, bad)
	if len(chunks) == 0 {
		t.Fatal("Split con Config inválida no debería devolver vacío para texto no trivial")
	}
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, DefaultConfig())
}

func TestSplit_ConfigOrderEnforced(t *testing.T) {
	// Min > Target > Max desordenados: normalized debe reordenar sin panic.
	weird := Config{TargetWords: 300, MaxWords: 200, MinWords: 900, MergeThresholdWords: 400}
	got := weird.normalized()
	if !(got.MinWords <= got.TargetWords && got.TargetWords <= got.MaxWords) {
		t.Fatalf("normalized no aseguró Min<=Target<=Max: %+v", got)
	}
	if got.MergeThresholdWords > got.MinWords {
		t.Fatalf("normalized dejó MergeThreshold > Min: %+v", got)
	}
}

func TestConfig_Validate(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Errorf("DefaultConfig debería ser válida, error: %v", err)
	}
	invalid := []Config{
		{TargetWords: 0, MaxWords: 800, MinWords: 500, MergeThresholdWords: 150},
		{TargetWords: 650, MaxWords: 400, MinWords: 500, MergeThresholdWords: 150}, // Target > Max
		{TargetWords: 650, MaxWords: 800, MinWords: 500, MergeThresholdWords: 600}, // Threshold > Min
	}
	for i, c := range invalid {
		if err := c.Validate(); err == nil {
			t.Errorf("caso inválido %d debería fallar Validate", i)
		}
	}
}

func TestSplit_CustomConfig(t *testing.T) {
	cfg := Config{TargetWords: 120, MaxWords: 160, MinWords: 90, MergeThresholdWords: 30}
	var paras []string
	for i := 0; i < 12; i++ {
		paras = append(paras, paragraph(80))
	}
	text := strings.Join(paras, "\n\n")

	chunks := Split(text, cfg)
	if len(chunks) < 3 {
		t.Fatalf("con Config chica deberían salir varios trozos, dio %d", len(chunks))
	}
	assertSeq(t, chunks)
	assertCoverage(t, text, chunks)
	assertSizes(t, chunks, cfg)
}
