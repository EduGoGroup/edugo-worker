package mocks

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/stretchr/testify/assert"
)

func TestNewSuccessfulMockExtractor(t *testing.T) {
	mockExtractor := NewSuccessfulMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Text)
	assert.Greater(t, result.PageCount, 0)
	assert.Greater(t, result.WordCount, 0)
}

func TestNewFailingMockExtractor(t *testing.T) {
	customErr := errors.New("custom error")
	mockExtractor := NewFailingMockExtractor(t, customErr)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, customErr)
	assert.Nil(t, result)
}

func TestNewCorruptPDFMockExtractor(t *testing.T) {
	mockExtractor := NewCorruptPDFMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, pdf.ErrPDFCorrupt)
	assert.Nil(t, result)
}

func TestNewScannedPDFMockExtractor(t *testing.T) {
	mockExtractor := NewScannedPDFMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, pdf.ErrPDFScanned)
	assert.Nil(t, result)
}

func TestNewTooLargePDFMockExtractor(t *testing.T) {
	mockExtractor := NewTooLargePDFMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, pdf.ErrPDFTooLarge)
	assert.Nil(t, result)
}

func TestNewEmptyPDFMockExtractor(t *testing.T) {
	mockExtractor := NewEmptyPDFMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, pdf.ErrPDFEmpty)
	assert.Nil(t, result)
}

func TestWithExtractionResult(t *testing.T) {
	mockExtractor := NewMockExtractor(t)
	customResult := &pdf.ExtractionResult{
		Text:      "Custom extracted text",
		PageCount: 10,
		WordCount: 200,
	}

	WithExtractionResult(mockExtractor, customResult, nil)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.NoError(t, err)
	assert.Equal(t, customResult, result)
}

func TestWithCustomText(t *testing.T) {
	mockExtractor := NewMockExtractor(t)
	customText := "Este es un texto personalizado de prueba"
	pageCount := 7

	WithCustomText(mockExtractor, customText, pageCount)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, customText, result.Text)
	assert.Equal(t, pageCount, result.PageCount)
	assert.Greater(t, result.WordCount, 0)
}

func TestNewFlakeyMockExtractor(t *testing.T) {
	failCount := 2
	customErr := errors.New("extraction failed")
	mockExtractor := NewFlakeyMockExtractor(t, failCount, customErr)
	ctx := context.Background()
	reader := strings.NewReader("fake pdf content")

	// Primeras 2 llamadas deben fallar
	for i := 0; i < failCount; i++ {
		result, err := mockExtractor.ExtractWithMetadata(ctx, reader)
		assert.Error(t, err, "intento %d debería fallar", i+1)
		assert.ErrorIs(t, err, customErr)
		assert.Nil(t, result)
	}

	// Tercera llamada debe tener éxito
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Text)
}

func TestNewTimeoutMockExtractor(t *testing.T) {
	mockExtractor := NewTimeoutMockExtractor(t)
	ctx := context.Background()

	reader := strings.NewReader("fake pdf content")
	result, err := mockExtractor.ExtractWithMetadata(ctx, reader)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, result)
}

func TestNewSuccessfulMockCleaner(t *testing.T) {
	mockCleaner := NewSuccessfulMockCleaner(t)

	cleaned := mockCleaner.Clean("texto sin limpiar")
	assert.NotEmpty(t, cleaned)

	withoutHeaders := mockCleaner.RemoveHeaders("texto con headers")
	assert.NotEmpty(t, withoutHeaders)
}

func TestWithCleanResponse(t *testing.T) {
	mockCleaner := NewMockCleaner(t)
	input := "texto original"
	output := "texto limpiado"

	WithCleanResponse(mockCleaner, input, output)

	cleaned := mockCleaner.Clean(input)
	assert.Equal(t, output, cleaned)
}

func TestWithRemoveHeadersResponse(t *testing.T) {
	mockCleaner := NewMockCleaner(t)
	input := "texto con headers"
	output := "texto sin headers"

	WithRemoveHeadersResponse(mockCleaner, input, output)

	cleaned := mockCleaner.RemoveHeaders(input)
	assert.Equal(t, output, cleaned)
}

func TestCreateMockExtractionResult(t *testing.T) {
	text := "Este es un texto de prueba con suficiente contenido"
	pageCount := 5

	result := CreateMockExtractionResult(text, pageCount)

	assert.NotNil(t, result)
	assert.Equal(t, text, result.Text)
	assert.Equal(t, pageCount, result.PageCount)
	assert.Greater(t, result.WordCount, 0)
}

func TestCreateMockReader(t *testing.T) {
	content := "PDF test content"
	reader := CreateMockReader(content)

	assert.NotNil(t, reader)

	// Verificar que podemos leer el contenido
	data, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}
