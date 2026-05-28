package pdf

// NOTA: Los tests de integración del PDF extractor están comentados temporalmente
// porque requieren PDFs fixture reales con texto extraíble.
//
// TODO(fase-5): Crear PDFs fixture reales para tests de integración
// - PDF con texto suficiente (>50 palabras)
// - PDF escaneado simulado (imagen sin texto)
// - PDF con diferentes formatos y encodings
//
// Por ahora, usamos los mocks mejorados creados en internal/infrastructure/pdf/mocks/
// para tests unitarios del processor y otros componentes.
//
// Los tests unitarios de validaciones (errores, tamaños, etc.) permanecen activos.
