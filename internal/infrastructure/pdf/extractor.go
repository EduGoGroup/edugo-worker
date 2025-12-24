package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
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

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error leyendo PDF: %w", err)
	}

	rs := bytes.NewReader(data)

	pdfCtx, err := api.ReadContext(rs, model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("error leyendo contexto PDF: %w", err)
	}

	pageCount := pdfCtx.PageCount

	var textBuilder strings.Builder
	for i := 1; i <= pageCount; i++ {
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

		textBuilder.Write(content)
		textBuilder.WriteString("\n")
	}

	rawText := textBuilder.String()
	cleanText := e.cleaner.Clean(rawText)
	wordCount := len(strings.Fields(cleanText))

	isScanned := wordCount < 50 && pageCount > 0

	if isScanned {
		e.logger.Warn("PDF parece ser escaneado (sin texto)", "pageCount", pageCount, "wordCount", wordCount)
	}

	e.logger.Info("extracción completada", "pages", pageCount, "words", wordCount)

	return &ExtractionResult{
		Text:      cleanText,
		RawText:   rawText,
		PageCount: pageCount,
		WordCount: wordCount,
		Metadata:  make(map[string]string),
		HasImages: false,
		IsScanned: isScanned,
	}, nil
}

var _ Extractor = (*PDFExtractor)(nil)
