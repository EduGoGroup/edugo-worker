package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/ledongthuc/pdf"
)

const (
	// Límites de validación
	maxPDFSize       = 100 * 1024 * 1024 // 100MB
	minWordsPerPage  = 10                // Mínimo de palabras por página para considerar que tiene texto
	scannedThreshold = 50                // Menos de 50 palabras en todo el PDF = probablemente escaneado
)

var (
	// ErrPDFTooLarge indica que el PDF excede el tamaño máximo
	ErrPDFTooLarge = errors.New("PDF demasiado grande")
	// ErrPDFEmpty indica que el PDF está vacío
	ErrPDFEmpty = errors.New("PDF vacío o corrupto")
	// ErrPDFScanned indica que el PDF parece estar escaneado (sin texto extraíble)
	ErrPDFScanned = errors.New("PDF escaneado sin texto extraíble - requiere OCR")
	// ErrPDFCorrupt indica que el PDF está corrupto
	ErrPDFCorrupt = errors.New("PDF corrupto o inválido")
)

// metadataKeys son las entradas del diccionario Info que se copian a Metadata.
// Se emiten en minúscula (title, author, …) como equivalente disponible en
// ledongthuc/pdf del antiguo mapa vacío.
var metadataKeys = []string{"Title", "Author", "Subject", "Keywords", "Creator", "Producer"}

type PDFExtractor struct {
	logger  logger.Logger
	cleaner Cleaner
}

func NewExtractor(log logger.Logger) Extractor {
	return &PDFExtractor{
		logger:  log,
		cleaner: NewCleaner(),
	}
}

func (e *PDFExtractor) Extract(ctx context.Context, reader io.Reader) (string, error) {
	result, err := e.ExtractWithMetadata(ctx, reader)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (e *PDFExtractor) ExtractWithMetadata(ctx context.Context, reader io.Reader) (*ExtractionResult, error) {
	e.logger.Debug("iniciando extracción de PDF")

	// Validar entrada
	if reader == nil {
		return nil, ErrPDFEmpty
	}

	// Leer datos con límite de tamaño
	data, err := io.ReadAll(io.LimitReader(reader, maxPDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("error leyendo PDF: %w", err)
	}

	// Validar tamaño
	if len(data) == 0 {
		return nil, ErrPDFEmpty
	}

	if len(data) > maxPDFSize {
		sizeMB := float64(len(data)) / (1024 * 1024)
		e.logger.Warn("PDF excede tamaño máximo", "size_mb", sizeMB, "max_mb", maxPDFSize/(1024*1024))
		return nil, ErrPDFTooLarge
	}

	e.logger.Debug("PDF leído", "size_bytes", len(data))

	// Extraer texto y metadatos con ledongthuc/pdf (soporta Type0/CID Identity-H).
	extracted, err := e.extract(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	if extracted.pageCount == 0 {
		e.logger.Warn("PDF sin páginas")
		return nil, ErrPDFEmpty
	}

	rawText := extracted.rawText
	cleanText := e.cleaner.Clean(rawText)
	totalWords := len(strings.Fields(cleanText))

	// Detección mejorada de PDFs escaneados
	avgWordsPerPage := float64(totalWords) / float64(extracted.pageCount)
	isScanned := e.detectScannedPDF(totalWords, extracted.pageCount, extracted.pagesWithText, avgWordsPerPage)

	if isScanned {
		e.logger.Warn("PDF escaneado detectado",
			"total_words", totalWords,
			"pages", extracted.pageCount,
			"pages_with_text", extracted.pagesWithText,
			"avg_words_per_page", avgWordsPerPage,
		)
		return nil, ErrPDFScanned
	}

	// Validar que haya contenido mínimo útil
	if totalWords < minWordsPerPage {
		e.logger.Warn("PDF con muy poco texto", "words", totalWords, "pages", extracted.pageCount)
		return nil, ErrPDFScanned
	}

	e.logger.Info("extracción completada",
		"pages", extracted.pageCount,
		"words", totalWords,
		"pages_with_text", extracted.pagesWithText,
		"avg_words_per_page", avgWordsPerPage,
	)

	return &ExtractionResult{
		Text:      cleanText,
		RawText:   rawText,
		PageCount: extracted.pageCount,
		WordCount: totalWords,
		Metadata:  extracted.metadata,
		// HasImages: ledongthuc/pdf no expone la presencia de XObjects imagen sin
		// recorrer los recursos de cada página; se conserva el valor neutro previo.
		HasImages: false,
		IsScanned: false,
	}, nil
}

// extraction agrupa lo que produce el recorrido del PDF antes de limpiar/validar.
type extraction struct {
	rawText       string
	pageCount     int
	pagesWithText int
	metadata      map[string]string
}

// extract recorre el PDF con ledongthuc/pdf y devuelve el texto crudo, el conteo
// de páginas y los metadatos. El linaje rsc.io/pdf entra en panic ante xrefs o
// estructuras rotas, por eso todo el recorrido va envuelto en un recover()
// defensivo que se traduce a ErrPDFCorrupt.
func (e *PDFExtractor) extract(ctx context.Context, rs io.ReaderAt, size int64) (result extraction, err error) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("panic extrayendo PDF (estructura/xref corrupta)", "panic", r)
			result = extraction{}
			err = fmt.Errorf("%w: %v", ErrPDFCorrupt, r)
		}
	}()

	r, rerr := pdf.NewReader(rs, size)
	if rerr != nil {
		e.logger.Error("error abriendo PDF", "error", rerr)
		return extraction{}, fmt.Errorf("%w: %v", ErrPDFCorrupt, rerr)
	}

	pageCount := r.NumPage()
	if pageCount <= 0 {
		// Sin páginas: el llamador lo mapea a ErrPDFEmpty.
		return extraction{pageCount: 0, metadata: map[string]string{}}, nil
	}

	e.logger.Debug("extrayendo texto de páginas", "total_pages", pageCount)

	var textBuilder strings.Builder
	pagesWithText := 0
	fonts := make(map[string]*pdf.Font)

	for i := 1; i <= pageCount; i++ {
		// Verificar contexto de cancelación
		select {
		case <-ctx.Done():
			return extraction{}, ctx.Err()
		default:
		}

		p := r.Page(i)
		// Cachear las fuentes por nombre para no re-parsear el charmap en cada texto.
		for _, name := range p.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := p.Font(name)
				fonts[name] = &f
			}
		}

		pageText, perr := p.GetPlainText(fonts)
		if perr != nil {
			e.logger.Warn("error extrayendo texto de página", "page", i, "error", perr.Error())
			continue
		}

		if len(strings.Fields(pageText)) >= minWordsPerPage {
			pagesWithText++
		}

		textBuilder.WriteString(pageText)
		textBuilder.WriteString("\n")
	}

	return extraction{
		rawText:       textBuilder.String(),
		pageCount:     pageCount,
		pagesWithText: pagesWithText,
		metadata:      extractMetadata(r),
	}, nil
}

// extractMetadata copia el diccionario Info del PDF (si existe) al mapa de
// metadatos. Claves en minúscula; los valores usan Value.Text() para decodificar
// correctamente los strings PDF (incluido UTF-16).
func extractMetadata(r *pdf.Reader) map[string]string {
	m := make(map[string]string)
	info := r.Trailer().Key("Info")
	if info.IsNull() {
		return m
	}
	for _, key := range metadataKeys {
		v := info.Key(key)
		if v.IsNull() {
			continue
		}
		if s := strings.TrimSpace(v.Text()); s != "" {
			m[strings.ToLower(key)] = s
		}
	}
	return m
}

// detectScannedPDF detecta si un PDF probablemente está escaneado usando múltiples heurísticas
func (e *PDFExtractor) detectScannedPDF(totalWords, pageCount, pagesWithText int, avgWordsPerPage float64) bool {
	// Heurística 1: Muy pocas palabras en total
	if totalWords < scannedThreshold {
		return true
	}

	// Heurística 2: Menos del 25% de las páginas tienen texto significativo
	textPageRatio := float64(pagesWithText) / float64(pageCount)
	if textPageRatio < 0.25 {
		e.logger.Debug("bajo ratio de páginas con texto", "ratio", textPageRatio)
		return true
	}

	// Heurística 3: Promedio muy bajo de palabras por página
	if avgWordsPerPage < float64(minWordsPerPage) {
		e.logger.Debug("promedio bajo de palabras por página", "avg", avgWordsPerPage)
		return true
	}

	return false
}

var _ Extractor = (*PDFExtractor)(nil)
