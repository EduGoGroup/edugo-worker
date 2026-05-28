# Tareas - Fase 5: Integraciones Core Avanzadas

> **Rama:** `feature/fase-5-integraciones-core`
> **Origen:** PT-008

---

## 📋 Lista de Tareas

### 1. Cliente AWS S3
- [ ] Crear interface `S3Client`
- [ ] Implementar `DownloadFile(bucket, key)`
- [ ] Implementar `GetFileSize(bucket, key)`
- [ ] Agregar configuración AWS en config.yaml
- [ ] Implementar retry con backoff exponencial
- [ ] Tests unitarios para S3Client
- [ ] Documentar uso y ejemplos

### 2. Extractor de PDF
- [ ] Investigar y elegir librería (pdfcpu vs unidoc)
- [ ] Crear interface `PDFExtractor`
- [ ] Implementar `ExtractText(reader)`
- [ ] Implementar `ExtractMetadata(reader)`
- [ ] Agregar limpieza de texto extraído
- [ ] Tests con PDFs de ejemplo (simple, complejo, escaneado)
- [ ] Manejo de errores para PDFs sin texto
- [ ] Documentar limitaciones

### 3. Cliente OpenAI
- [ ] Crear interface `OpenAIClient`
- [ ] Implementar `GenerateSummary(content, options)`
- [ ] Implementar `GenerateQuiz(content, options)`
- [ ] Crear prompts optimizados para resúmenes
- [ ] Crear prompts optimizados para quizzes
- [ ] Implementar parser de respuestas JSON
- [ ] Agregar manejo de rate limits (429)
- [ ] Agregar manejo de timeouts
- [ ] Tests con mocks
- [ ] Documentar estructura de prompts

### 4. Integración en Processors
- [ ] Actualizar `MaterialUploadedProcessor`
  - [ ] Agregar dependencia S3Client
  - [ ] Agregar dependencia PDFExtractor
  - [ ] Agregar dependencia OpenAIClient
  - [ ] Implementar flujo: S3 → PDF → OpenAI → MongoDB
  - [ ] Actualizar estados del material (processing, completed, error)
  - [ ] Agregar logging detallado por etapa
- [ ] Actualizar `MaterialReprocessProcessor` (mismo flujo)
- [ ] Eliminar datos hardcoded/simulados
- [ ] Tests de integración end-to-end

### 5. Configuración y Documentación
- [ ] Actualizar `config/config.yaml`
- [ ] Documentar variables de entorno
- [ ] Crear archivo `.env.example`
- [ ] Actualizar README con instrucciones de configuración
- [ ] Documentar costos estimados
- [ ] Documentar limitaciones conocidas

### 6. Tests y Calidad
- [ ] Tests unitarios para cada componente
- [ ] Tests de integración
- [ ] Validar cobertura >70%
- [ ] Ejecutar `make lint`
- [ ] Ejecutar `make build`
- [ ] Ejecutar `make test`

---

## 🔄 Orden de Ejecución Recomendado

1. **Semana 1:** S3Client + PDFExtractor
2. **Semana 2:** OpenAIClient + Prompts
3. **Semana 3:** Integración + Tests

---

## ✅ Checklist Final

Antes de crear PR:

- [ ] Todos los tests unitarios pasan
- [ ] Tests de integración pasan
- [ ] Cobertura >70%
- [ ] `make build` exitoso
- [ ] `make lint` sin errores
- [ ] Documentación actualizada
- [ ] Variables de entorno documentadas
- [ ] Commits atómicos y bien descritos
- [ ] Branch actualizado con `dev`

---

**Última actualización:** 2025-12-23
