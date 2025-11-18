# Tareas Sprint 02 - PDF Processing

## TASK-02-001: Implementar PDFProcessor
**Prioridad:** HIGH | **Estimación:** 4h

#### Implementación
Archivo: `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/internal/services/pdf_processor.go`

```go
package services

import (
    "bytes"
    "context"
    "os/exec"
    "strings"
)

type PDFProcessor struct {
    s3Client *S3Client
}

// ExtractText extrae texto de PDF
func (p *PDFProcessor) ExtractText(ctx context.Context, s3Key string) (string, error) {
    // 1. Descargar de S3
    pdfData, err := p.s3Client.Download(ctx, s3Key)
    if err != nil {
        return "", err
    }
    
    // 2. Ejecutar pdftotext
    cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-", "-")
    cmd.Stdin = bytes.NewReader(pdfData)
    
    output, err := cmd.Output()
    if err != nil {
        // Fallback a OCR
        return p.extractWithOCR(ctx, pdfData)
    }
    
    // 3. Limpiar texto
    text := p.cleanText(string(output))
    
    // 4. Validar longitud
    if len(strings.Fields(text)) < 500 {
        return "", errors.New("texto insuficiente")
    }
    
    return text, nil
}

func (p *PDFProcessor) cleanText(text string) string {
    // Remover espacios múltiples
    // Remover caracteres especiales
    // Normalizar saltos de línea
    return cleaned
}
```

#### Criterios
- [ ] Descarga de S3 funcional
- [ ] pdftotext ejecuta correctamente
- [ ] OCR fallback con Tesseract
- [ ] Limpieza de texto
- [ ] Validación >500 palabras
- [ ] Tests con PDFs reales

---

## TASK-02-002: S3 Client
**Prioridad:** HIGH | **Estimación:** 2h

```go
// internal/infrastructure/s3/client.go
func (c *S3Client) Download(ctx context.Context, key string) ([]byte, error) {
    result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    })
    // ...
}
```

---

## TASK-02-003: Tests con PDFs Reales
**Prioridad:** MEDIUM | **Estimación:** 2h

```bash
# Tests
go test ./internal/services -v -run TestPDFProcessor
```

**Tiempo total:** 8h
