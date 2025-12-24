package mocks

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/stretchr/testify/mock"
)

// NewSuccessfulMockExtractor crea un mock que siempre extrae texto exitosamente
func NewSuccessfulMockExtractor(t *testing.T) *MockExtractor {
	mockExtractor := NewMockExtractor(t)

	// Configurar ExtractWithMetadata para retornar resultado exitoso
	mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
		Return(&pdf.ExtractionResult{
			Text:      "Este es un texto de prueba extraído del PDF. Contiene suficiente contenido para validación.",
			PageCount: 5,
			WordCount: 100,
		}, nil).
		Maybe()

	return mockExtractor
}

// NewFailingMockExtractor crea un mock que siempre falla con el error especificado
func NewFailingMockExtractor(t *testing.T, err error) *MockExtractor {
	mockExtractor := NewMockExtractor(t)

	mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
		Return(nil, err).
		Maybe()

	return mockExtractor
}

// NewCorruptPDFMockExtractor crea un mock que simula PDF corrupto
func NewCorruptPDFMockExtractor(t *testing.T) *MockExtractor {
	return NewFailingMockExtractor(t, pdf.ErrPDFCorrupt)
}

// NewScannedPDFMockExtractor crea un mock que simula PDF escaneado
func NewScannedPDFMockExtractor(t *testing.T) *MockExtractor {
	return NewFailingMockExtractor(t, pdf.ErrPDFScanned)
}

// NewTooLargePDFMockExtractor crea un mock que simula PDF demasiado grande
func NewTooLargePDFMockExtractor(t *testing.T) *MockExtractor {
	return NewFailingMockExtractor(t, pdf.ErrPDFTooLarge)
}

// NewEmptyPDFMockExtractor crea un mock que simula PDF vacío
func NewEmptyPDFMockExtractor(t *testing.T) *MockExtractor {
	return NewFailingMockExtractor(t, pdf.ErrPDFEmpty)
}

// WithExtractionResult configura un mock para retornar un resultado específico
func WithExtractionResult(mockExtractor *MockExtractor, result *pdf.ExtractionResult, err error) *MockExtractor {
	mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
		Return(result, err).
		Once()
	return mockExtractor
}

// WithCustomText configura un mock para retornar texto personalizado
func WithCustomText(mockExtractor *MockExtractor, text string, pageCount int) *MockExtractor {
	wordCount := len(text) / 5 // Aproximación: ~5 caracteres por palabra

	mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
		Return(&pdf.ExtractionResult{
			Text:      text,
			PageCount: pageCount,
			WordCount: wordCount,
		}, nil).
		Once()

	return mockExtractor
}

// NewFlakeyMockExtractor crea un mock que falla las primeras N veces y luego tiene éxito
// Útil para testing de retry logic
func NewFlakeyMockExtractor(t *testing.T, failCount int, failErr error) *MockExtractor {
	mockExtractor := NewMockExtractor(t)

	// Configurar para fallar N veces
	for i := 0; i < failCount; i++ {
		mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
			Return(nil, failErr).
			Once()
	}

	// Luego tener éxito
	mockExtractor.On("ExtractWithMetadata", mock.Anything, mock.Anything).
		Return(&pdf.ExtractionResult{
			Text:      "Texto extraído después de reintentos exitosos.",
			PageCount: 3,
			WordCount: 50,
		}, nil).
		Maybe()

	return mockExtractor
}

// NewTimeoutMockExtractor crea un mock que simula timeout
func NewTimeoutMockExtractor(t *testing.T) *MockExtractor {
	return NewFailingMockExtractor(t, context.DeadlineExceeded)
}

// NewSuccessfulMockCleaner crea un mock de cleaner que siempre tiene éxito
func NewSuccessfulMockCleaner(t *testing.T) *MockCleaner {
	mockCleaner := NewMockCleaner(t)

	mockCleaner.On("Clean", mock.Anything).
		Return("Texto limpiado para testing").
		Maybe()

	mockCleaner.On("RemoveHeaders", mock.Anything).
		Return("Texto sin encabezados para testing").
		Maybe()

	return mockCleaner
}

// WithCleanResponse configura un mock cleaner para retornar texto específico
func WithCleanResponse(mockCleaner *MockCleaner, input string, output string) *MockCleaner {
	mockCleaner.On("Clean", input).
		Return(output).
		Once()

	return mockCleaner
}

// WithRemoveHeadersResponse configura un mock cleaner para retornar texto sin headers
func WithRemoveHeadersResponse(mockCleaner *MockCleaner, input string, output string) *MockCleaner {
	mockCleaner.On("RemoveHeaders", input).
		Return(output).
		Once()

	return mockCleaner
}

// CreateMockExtractionResult crea un resultado de extracción para testing
func CreateMockExtractionResult(text string, pageCount int) *pdf.ExtractionResult {
	wordCount := len(text) / 5 // Aproximación simple

	return &pdf.ExtractionResult{
		Text:      text,
		PageCount: pageCount,
		WordCount: wordCount,
	}
}

// CreateMockReader crea un io.Reader de prueba con contenido PDF simulado
func CreateMockReader(content string) io.Reader {
	return strings.NewReader(content)
}
