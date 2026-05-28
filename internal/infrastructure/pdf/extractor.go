package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
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

	rs := bytes.NewReader(data)

	// Intentar leer contexto PDF
	pdfCtx, err := api.ReadContext(rs, model.NewDefaultConfiguration())
	if err != nil {
		e.logger.Error("error leyendo contexto PDF", "error", err)
		// Distinguir entre PDF corrupto y otros errores
		if strings.Contains(err.Error(), "not a PDF file") ||
			strings.Contains(err.Error(), "malformed") ||
			strings.Contains(err.Error(), "invalid") {
			return nil, fmt.Errorf("%w: %v", ErrPDFCorrupt, err)
		}
		return nil, fmt.Errorf("error procesando PDF: %w", err)
	}

	pageCount := pdfCtx.PageCount

	if pageCount == 0 {
		e.logger.Warn("PDF sin páginas")
		return nil, ErrPDFEmpty
	}

	e.logger.Debug("extrayendo texto de páginas", "total_pages", pageCount)

	var textBuilder strings.Builder
	pageWordCounts := make([]int, pageCount)
	pagesWithText := 0

	for i := 1; i <= pageCount; i++ {
		// Verificar contexto de cancelación
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Extraer contenido de cada página
		contentReader, err := pdfcpu.ExtractPageContent(pdfCtx, i)
		if err != nil {
			e.logger.Warn("error extrayendo página", "page", i, "error", err.Error())
			continue
		}

		// Leer el contenido
		content, err := io.ReadAll(contentReader)
		if err != nil {
			e.logger.Warn("error leyendo contenido de página", "page", i, "error", err.Error())
			continue
		}

		pageText := string(content)
		pageWords := len(strings.Fields(pageText))
		pageWordCounts[i-1] = pageWords

		if pageWords >= minWordsPerPage {
			pagesWithText++
		}

		textBuilder.Write(content)
		textBuilder.WriteString("\n")
	}

	rawText := textBuilder.String()
	cleanText := e.cleaner.Clean(rawText)
	totalWords := len(strings.Fields(cleanText))

	// Detección mejorada de PDFs escaneados
	avgWordsPerPage := float64(totalWords) / float64(pageCount)
	isScanned := e.detectScannedPDF(totalWords, pageCount, pagesWithText, avgWordsPerPage)

	if isScanned {
		e.logger.Warn("PDF escaneado detectado",
			"total_words", totalWords,
			"pages", pageCount,
			"pages_with_text", pagesWithText,
			"avg_words_per_page", avgWordsPerPage,
		)
		return nil, ErrPDFScanned
	}

	// Validar que haya contenido mínimo útil
	if totalWords < minWordsPerPage {
		e.logger.Warn("PDF con muy poco texto", "words", totalWords, "pages", pageCount)
		return nil, ErrPDFScanned
	}

	e.logger.Info("extracción completada",
		"pages", pageCount,
		"words", totalWords,
		"pages_with_text", pagesWithText,
		"avg_words_per_page", avgWordsPerPage,
	)

	return &ExtractionResult{
		Text:      cleanText,
		RawText:   rawText,
		PageCount: pageCount,
		WordCount: totalWords,
		Metadata:  make(map[string]string),
		HasImages: false,
		IsScanned: false,
	}, nil
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
