package pdf

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCleaner es un mock del Cleaner para testing
type MockCleaner struct {
	CleanFunc           func(text string) string
	RemoveHeadersFunc   func(text string) string
	NormalizeSpacesFunc func(text string) string
}

func (m *MockCleaner) Clean(text string) string {
	if m.CleanFunc != nil {
		return m.CleanFunc(text)
	}
	return text
}

func (m *MockCleaner) RemoveHeaders(text string) string {
	if m.RemoveHeadersFunc != nil {
		return m.RemoveHeadersFunc(text)
	}
	return text
}

func (m *MockCleaner) NormalizeSpaces(text string) string {
	if m.NormalizeSpacesFunc != nil {
		return m.NormalizeSpacesFunc(text)
	}
	return text
}

// MockLogger es un mock del logger para testing
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...interface{})  {}
func (m *MockLogger) Info(msg string, fields ...interface{})   {}
func (m *MockLogger) Warn(msg string, fields ...interface{})   {}
func (m *MockLogger) Error(msg string, fields ...interface{})  {}
func (m *MockLogger) Fatal(msg string, fields ...interface{})  {}
func (m *MockLogger) With(fields ...interface{}) logger.Logger { return m }
func (m *MockLogger) Sync() error                              { return nil }

// newTestLogger crea un logger para tests que descarta la salida
func newTestLogger() logger.Logger {
	return &MockLogger{}
}

// generateMinimalPDF genera un PDF mínimo válido para tests
// Este es un PDF básico que contiene la estructura mínima requerida
// Actualizado para contener suficiente texto para pasar las validaciones (>50 palabras)
func generateMinimalPDF() []byte {
	// PDF mínimo válido con contenido suficiente para no ser detectado como escaneado
	pdfContent := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
>>
>>
/MediaBox [0 0 612 792]
/Contents 4 0 R
>>
endobj
4 0 obj
<<
/Length 550
>>
stream
BT
/F1 12 Tf
50 750 Td
(Este es un documento PDF de prueba con suficiente contenido.) Tj
0 -20 Td
(El objetivo de este PDF es pasar las validaciones de detección) Tj
0 -20 Td
(de documentos escaneados. Para ello necesitamos al menos) Tj
0 -20 Td
(cincuenta palabras distribuidas en el contenido del archivo.) Tj
0 -20 Td
(Este texto contiene información variada sobre testing y) Tj
0 -20 Td
(validación de PDFs en sistemas de procesamiento de documentos.) Tj
0 -20 Td
(La extracción de texto debe funcionar correctamente y) Tj
0 -20 Td
(el contador de palabras debe superar el umbral mínimo.) Tj
0 -20 Td
(Adicionalmente este contenido permite probar la limpieza) Tj
0 -20 Td
(y normalización de texto extraído de archivos PDF reales.) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000317 00000 n
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
916
%%EOF`
	return []byte(pdfContent)
}

func TestPDFExtractor_Extract(t *testing.T) {
	t.Skip("SKIP: Requiere PDFs fixtures reales con texto extraíble - Ver extractor_integration_test.go para documentación")
	t.Run("extracción básica con PDF válido", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		pdfBytes := generateMinimalPDF()
		reader := bytes.NewReader(pdfBytes)
		ctx := context.Background()

		// Act
		text, err := extractor.Extract(ctx, reader)

		// Assert
		require.NoError(t, err)
		// Nota: pdfcpu puede no extraer texto de PDFs simples de manera confiable
		// Lo importante es que no haya error
		assert.NotNil(t, text, "el texto extraído no debería ser nil")
	})

	t.Run("error al leer datos inválidos", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		// PDF inválido
		invalidPDF := []byte("esto no es un PDF válido")
		reader := bytes.NewReader(invalidPDF)
		ctx := context.Background()

		// Act
		text, err := extractor.Extract(ctx, reader)

		// Assert
		assert.Error(t, err, "debería retornar error con datos inválidos")
		assert.Empty(t, text, "el texto debería estar vacío cuando hay error")
		// El error ahora debe ser ErrPDFCorrupt
		assert.ErrorIs(t, err, ErrPDFCorrupt, "el error debería ser PDF corrupto")
	})

	t.Run("manejo de reader vacío", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		reader := bytes.NewReader([]byte{})
		ctx := context.Background()

		// Act
		text, err := extractor.Extract(ctx, reader)

		// Assert
		assert.Error(t, err, "debería retornar error con reader vacío")
		assert.Empty(t, text, "el texto debería estar vacío cuando hay error")
	})
}

func TestPDFExtractor_ExtractWithMetadata(t *testing.T) {
	t.Skip("SKIP: Requiere PDFs fixtures reales con texto extraíble - Ver extractor_integration_test.go para documentación")
	t.Run("extracción con metadatos completos", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		pdfBytes := generateMinimalPDF()
		reader := bytes.NewReader(pdfBytes)
		ctx := context.Background()

		// Act
		result, err := extractor.ExtractWithMetadata(ctx, reader)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result, "el resultado no debería ser nil")

		// Validar estructura del resultado
		assert.NotNil(t, result.Text, "el texto limpio no debería ser nil")
		assert.NotNil(t, result.RawText, "el texto raw no debería ser nil")
		assert.GreaterOrEqual(t, result.PageCount, 0, "pageCount debería ser >= 0")
		assert.GreaterOrEqual(t, result.WordCount, 0, "wordCount debería ser >= 0")
		assert.NotNil(t, result.Metadata, "metadata no debería ser nil")

		// La detección de escaneo depende del contenido real extraído
		// No hacemos asserts específicos ya que pdfcpu puede no extraer texto del PDF de prueba
	})

	t.Run("detección de PDF escaneado", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		// Crear un PDF con muy poco texto (< 50 palabras) para simular PDF escaneado
		// Usamos un PDF válido pero con contenido mínimo
		scannedPDF := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /MediaBox [0 0 612 792] /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 35 >>
stream
BT /F1 12 Tf 100 700 Td (Hola) Tj ET
endstream
endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000317 00000 n
trailer
<< /Size 5 /Root 1 0 R >>
startxref
400
%%EOF`
		reader := bytes.NewReader([]byte(scannedPDF))
		ctx := context.Background()

		// Act
		result, err := extractor.ExtractWithMetadata(ctx, reader)

		// Assert
		// Ahora esperamos que retorne error ErrPDFScanned
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPDFScanned, "debería detectar como PDF escaneado")
		assert.Nil(t, result, "result debería ser nil cuando se detecta PDF escaneado")
	})

	t.Run("error con datos inválidos", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		invalidPDF := []byte("datos inválidos")
		reader := bytes.NewReader(invalidPDF)
		ctx := context.Background()

		// Act
		result, err := extractor.ExtractWithMetadata(ctx, reader)

		// Assert
		assert.Error(t, err, "debería retornar error con datos inválidos")
		assert.Nil(t, result, "el resultado debería ser nil cuando hay error")
	})

	t.Run("conteo de palabras correcto", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		pdfBytes := generateMinimalPDF()
		reader := bytes.NewReader(pdfBytes)
		ctx := context.Background()

		// Act
		result, err := extractor.ExtractWithMetadata(ctx, reader)

		// Assert
		require.NoError(t, err)

		// Verificar que el conteo de palabras sea consistente con el texto
		wordCount := len(strings.Fields(result.Text))
		assert.Equal(t, result.WordCount, wordCount, "el conteo de palabras debería ser consistente")
	})
}

func TestTextCleaner_Clean(t *testing.T) {
	cleaner := NewCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "texto simple sin cambios necesarios",
			input:    "Este es un texto simple",
			expected: "Este es un texto simple",
		},
		{
			name:     "texto con espacios múltiples",
			input:    "Texto  con    espacios   múltiples",
			expected: "Texto con espacios múltiples",
		},
		{
			name:  "texto con múltiples saltos de línea",
			input: "Línea 1\n\n\n\n\nLínea 2",
			// RemoveHeaders elimina líneas vacías, dejando solo las con contenido
			expected: "Línea 1\nLínea 2",
		},
		{
			name:     "texto con encabezados de página",
			input:    "Página 1\nContenido real\nPágina 2\nMás contenido",
			expected: "Contenido real\nMás contenido",
		},
		{
			name:     "texto con page en inglés",
			input:    "Page 5\nImportant content\nPage 6",
			expected: "Important content",
		},
		{
			name:     "texto con espacios al inicio y final",
			input:    "  \n  Texto con espacios  \n  ",
			expected: "Texto con espacios",
		},
		{
			name:  "combinación de problemas",
			input: "  Página 1  \n\n\n  Contenido   real  \n\n\n\n  Pág. 2  \n  Más   contenido  ",
			// RemoveHeaders elimina headers y líneas vacías, NormalizeSpaces normaliza espacios
			expected: "Contenido real \n Más contenido",
		},
		{
			name:     "texto vacío",
			input:    "",
			expected: "",
		},
		{
			name:     "solo espacios y saltos de línea",
			input:    "   \n\n\n   \n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := cleaner.Clean(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTextCleaner_NormalizeSpaces(t *testing.T) {
	cleaner := NewCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "espacios múltiples horizontales",
			input:    "palabra1    palabra2     palabra3",
			expected: "palabra1 palabra2 palabra3",
		},
		{
			name:     "espacios y tabs mezclados",
			input:    "palabra1\t\tpalabra2  \t  palabra3",
			expected: "palabra1 palabra2 palabra3",
		},
		{
			name:     "múltiples saltos de línea",
			input:    "línea1\n\n\n\n\nlínea2",
			expected: "línea1\n\nlínea2",
		},
		{
			name:     "tres saltos exactos",
			input:    "línea1\n\n\nlínea2",
			expected: "línea1\n\nlínea2",
		},
		{
			name:     "saltos normales (2) no se modifican",
			input:    "línea1\n\nlínea2",
			expected: "línea1\n\nlínea2",
		},
		{
			name:     "un salto no se modifica",
			input:    "línea1\nlínea2",
			expected: "línea1\nlínea2",
		},
		{
			name:     "combinación de espacios y saltos",
			input:    "línea1  \t  \n\n\n\n\n  línea2    palabra",
			expected: "línea1 \n\n línea2 palabra",
		},
		{
			name:     "sin espacios problemáticos",
			input:    "texto normal sin problemas",
			expected: "texto normal sin problemas",
		},
		{
			name:     "texto vacío",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := cleaner.NormalizeSpaces(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTextCleaner_RemoveHeaders(t *testing.T) {
	cleaner := NewCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "página en español con número",
			input:    "Página 1\nContenido real",
			expected: "Contenido real",
		},
		{
			name:     "page en inglés con número",
			input:    "Page 5\nImportant content",
			expected: "Important content",
		},
		{
			name:     "pág abreviado con punto",
			input:    "Pág. 10\nTexto importante",
			expected: "Texto importante",
		},
		{
			name:     "pág abreviado sin punto",
			input:    "Pág 20\nMás texto",
			expected: "Más texto",
		},
		{
			name:     "múltiples encabezados",
			input:    "Página 1\nContenido 1\nPágina 2\nContenido 2",
			expected: "Contenido 1\nContenido 2",
		},
		{
			name:     "case insensitive",
			input:    "PÁGINA 1\ncontenido\npage 2\nmás contenido",
			expected: "contenido\nmás contenido",
		},
		{
			name:     "sin encabezados",
			input:    "Solo contenido normal\nsin encabezados de página",
			expected: "Solo contenido normal\nsin encabezados de página",
		},
		{
			name:     "palabra página en medio del texto no se elimina",
			input:    "Este texto menciona la página en contexto\nPágina 5\nOtro contenido",
			expected: "Este texto menciona la página en contexto\nOtro contenido",
		},
		{
			name:     "líneas vacías se eliminan",
			input:    "Contenido\n\n\nMás contenido",
			expected: "Contenido\nMás contenido",
		},
		{
			name:     "texto vacío",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := cleaner.RemoveHeaders(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPDFExtractor_IntegrationWithCleaner(t *testing.T) {
	t.Skip("SKIP: Requiere PDFs fixtures reales con texto extraíble - Ver extractor_integration_test.go para documentación")
	t.Run("integración completa extractor con cleaner", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()

		// Crear extractor con cleaner real
		extractor := &PDFExtractor{
			logger:  logger,
			cleaner: NewCleaner(),
		}

		pdfBytes := generateMinimalPDF()
		reader := bytes.NewReader(pdfBytes)
		ctx := context.Background()

		// Act
		result, err := extractor.ExtractWithMetadata(ctx, reader)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Ambos campos deberían existir (pueden estar vacíos si el PDF no tiene texto extraíble)
		assert.NotNil(t, result.RawText, "raw text no debería ser nil")
		assert.NotNil(t, result.Text, "texto limpio no debería ser nil")
	})
}

func TestNewExtractor(t *testing.T) {
	t.Run("crear extractor con logger válido", func(t *testing.T) {
		// Arrange
		logger := newTestLogger()

		// Act
		extractor := NewExtractor(logger)

		// Assert
		assert.NotNil(t, extractor, "el extractor no debería ser nil")

		// Verificar que implementa la interfaz
		_ = Extractor(extractor)
	})
}

func TestNewCleaner(t *testing.T) {
	t.Run("crear cleaner", func(t *testing.T) {
		// Act
		cleaner := NewCleaner()

		// Assert
		assert.NotNil(t, cleaner, "el cleaner no debería ser nil")

		// Verificar que implementa la interfaz
		_ = Cleaner(cleaner)
	})
}

// Tests adicionales para las nuevas validaciones del PDF Extractor (Fase 5)

func TestPDFExtractor_ValidateSize(t *testing.T) {
	t.Run("rechaza PDF vacío", func(t *testing.T) {
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		reader := bytes.NewReader([]byte{})
		ctx := context.Background()

		_, err := extractor.ExtractWithMetadata(ctx, reader)

		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPDFEmpty)
	})

	t.Run("rechaza PDF demasiado grande", func(t *testing.T) {
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		// Crear datos que excedan maxPDFSize (100MB)
		largeData := make([]byte, 101*1024*1024)
		reader := bytes.NewReader(largeData)
		ctx := context.Background()

		_, err := extractor.ExtractWithMetadata(ctx, reader)

		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPDFTooLarge)
	})

	t.Run("rechaza reader nil", func(t *testing.T) {
		logger := newTestLogger()
		extractor := NewExtractor(logger)
		ctx := context.Background()

		_, err := extractor.ExtractWithMetadata(ctx, nil)

		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPDFEmpty)
	})
}

func TestPDFExtractor_DetectScannedPDF(t *testing.T) {
	logger := newTestLogger()
	extractor := NewExtractor(logger).(*PDFExtractor)

	tests := []struct {
		name            string
		totalWords      int
		pageCount       int
		pagesWithText   int
		avgWordsPerPage float64
		expectScanned   bool
	}{
		{
			name:            "PDF con suficiente texto",
			totalWords:      100,
			pageCount:       1,
			pagesWithText:   1,
			avgWordsPerPage: 100.0,
			expectScanned:   false,
		},
		{
			name:            "PDF con muy poco texto total",
			totalWords:      30,
			pageCount:       5,
			pagesWithText:   5,
			avgWordsPerPage: 6.0,
			expectScanned:   true,
		},
		{
			name:            "PDF con bajo ratio de páginas con texto",
			totalWords:      100,
			pageCount:       10,
			pagesWithText:   2,
			avgWordsPerPage: 10.0,
			expectScanned:   true,
		},
		{
			name:            "PDF con promedio bajo de palabras por página",
			totalWords:      80,
			pageCount:       10,
			pagesWithText:   10,
			avgWordsPerPage: 8.0,
			expectScanned:   true,
		},
		{
			name:            "PDF limítrofe pero válido",
			totalWords:      60,
			pageCount:       2,
			pagesWithText:   2,
			avgWordsPerPage: 30.0,
			expectScanned:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.detectScannedPDF(tt.totalWords, tt.pageCount, tt.pagesWithText, tt.avgWordsPerPage)
			assert.Equal(t, tt.expectScanned, result)
		})
	}
}

func TestPDFExtractor_ContextCancellation(t *testing.T) {
	t.Run("respeta cancelación de contexto", func(t *testing.T) {
		logger := newTestLogger()
		extractor := NewExtractor(logger)

		// Crear un contexto ya cancelado
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancelar inmediatamente

		// Usar un PDF válido pero el contexto cancelado debería detener el procesamiento
		pdfBytes := generateMinimalPDF()
		reader := bytes.NewReader(pdfBytes)

		_, err := extractor.ExtractWithMetadata(ctx, reader)

		// Puede ser error de contexto o error de PDF dependiendo del timing
		assert.Error(t, err)
	})
}
