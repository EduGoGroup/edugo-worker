package pdf

import (
	"context"
	"io"
)

// Extractor define la interfaz para extractores de texto desde PDF
type Extractor interface {
	// Extract extrae texto de un PDF
	// Returns: texto extraído y limpiado
	Extract(ctx context.Context, reader io.Reader) (string, error)

	// ExtractWithMetadata extrae texto y metadatos del PDF
	ExtractWithMetadata(ctx context.Context, reader io.Reader) (*ExtractionResult, error)
}

// ExtractionResult contiene el resultado de la extracción
type ExtractionResult struct {
	Text      string            // Texto extraído y limpiado
	RawText   string            // Texto sin procesar
	PageCount int               // Número de páginas
	WordCount int               // Número de palabras
	Metadata  map[string]string // Metadatos del PDF (autor, título, etc.)
	HasImages bool              // Si el PDF contiene imágenes
	IsScanned bool              // Si es un PDF escaneado (sin texto)
}

// Cleaner define la interfaz para limpiadores de texto
type Cleaner interface {
	// Clean limpia y normaliza texto extraído de PDF
	Clean(text string) string

	// RemoveHeaders elimina encabezados y pies de página
	RemoveHeaders(text string) string

	// NormalizeSpaces normaliza espacios en blanco
	NormalizeSpaces(text string) string
}
