# Preguntas Sprint 02

## Q001: ¿pdftotext o librería Go para PDFs?
**Decisión por Defecto:** **pdftotext (comando)**

**Justificación:**
- Más rápido que librerías Go
- Mejor extracción de layout
- Battle-tested

**Implementación:**
```go
cmd := exec.Command("pdftotext", "-layout", inputPath, "-")
```

## Q002: ¿Qué hacer si PDF es imagen escaneada?
**Decisión por Defecto:** **OCR con Tesseract**

**Implementación:**
```go
if pdftotext falla {
    return extractWithOCR(pdfData)
}
```

## Q003: ¿Validar tamaño máximo de PDF?
**Decisión por Defecto:** **Sí, 50MB máximo**

**Justificación:** Prevenir DoS, timeouts
