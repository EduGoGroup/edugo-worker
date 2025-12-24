# Fase 5: Integraciones Core Avanzadas

> **Objetivo:** Implementar integraciones reales con servicios externos: AWS S3, extracción de PDF y OpenAI para procesamiento de materiales educativos.
>
> **Duración estimada:** 2-3 semanas
> **Complejidad:** Alta
> **Riesgo:** Alto
> **Prerequisito:** Fases 0, 1, 2 y 2.5 completadas
> **Origen:** Plan de trabajo PT-008 de edugo_analisis

---

## 🎯 Objetivos

1. ✅ Implementar cliente AWS S3 para descarga de archivos
2. ✅ Implementar extracción de texto desde archivos PDF
3. ✅ Implementar cliente OpenAI para generación de contenido educativo
4. ✅ Integrar servicios en MaterialUploadedProcessor
5. ✅ Reemplazar procesamiento simulado con servicios reales

---

## 📦 Entregables

### E5.1: Cliente S3
- `internal/infrastructure/storage/s3_client.go`
- Descarga de archivos desde AWS S3
- Validación de tamaño y tipo de archivo
- Manejo de errores y retry
- Tests unitarios

### E5.2: Extractor de PDF
- `internal/infrastructure/pdf/pdf_extractor.go`
- Extracción de texto usando pdfcpu o unidoc
- Extracción de metadata (páginas, autor, etc.)
- Limpieza y normalización de texto
- Tests con PDFs de ejemplo

### E5.3: Cliente OpenAI
- `internal/infrastructure/ai/openai_client.go`
- Generación de resúmenes educativos
- Generación de quizzes/evaluaciones
- Prompts optimizados para educación
- Manejo de rate limits
- Tests con mocks

### E5.4: Integración en Processors
- Actualizar MaterialUploadedProcessor
- Flujo completo: S3 → PDF → OpenAI → MongoDB
- Actualizar MaterialReprocessProcessor
- Manejo de errores por etapa
- Tests de integración end-to-end

---

## 🔑 Tecnologías y Dependencias

### Nuevas Dependencias Go
```bash
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/pdfcpu/pdfcpu
go get github.com/sashabaranov/go-openai
```

### Variables de Entorno Requeridas
```bash
# AWS S3
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
S3_BUCKET=edugo-materials

# OpenAI
OPENAI_API_KEY=sk-xxx
OPENAI_MODEL=gpt-4
```

---

## 📋 Commits Sugeridos

**Commit 1: Cliente S3**
```
feat(fase-5): implementar cliente S3 para descarga de archivos

- Agregar S3Client interface
- Implementar DownloadFile y GetFileSize
- Configuración de AWS
- Tests unitarios

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-003
```

**Commit 2: Extractor PDF**
```
feat(fase-5): implementar extractor de texto PDF

- Agregar PDFExtractor interface
- Implementar ExtractText con pdfcpu
- Agregar ExtractMetadata
- Tests con PDFs de ejemplo

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-003
```

**Commit 3: Cliente OpenAI**
```
feat(fase-5): implementar cliente OpenAI real

- Agregar OpenAIClient interface
- Implementar GenerateSummary
- Implementar GenerateQuiz con respuesta JSON
- Prompts optimizados para educación

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-002
```

**Commit 4: Integración**
```
feat(fase-5): integrar servicios en MaterialUploadedProcessor

- Integrar S3Client para descarga
- Integrar PDFExtractor para texto
- Integrar OpenAI para resumen y quiz
- Manejo de errores con actualización de estado

Refs: PT-008
```

**Commit 5: Tests**
```
test(fase-5): agregar tests de integración

- Tests unitarios con mocks
- Tests de integración end-to-end (opcional)
- Cobertura >70%

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-009
```

---

## ✅ Checklist de Validación

### Cliente S3
- [ ] Interface S3Client definida
- [ ] DownloadFile implementado
- [ ] GetFileSize implementado
- [ ] Manejo de errores AWS
- [ ] Tests unitarios pasan

### Extractor PDF
- [ ] Interface PDFExtractor definida
- [ ] ExtractText implementado
- [ ] ExtractMetadata implementado
- [ ] Maneja PDFs simples y complejos
- [ ] Tests con PDFs reales

### Cliente OpenAI
- [ ] Interface OpenAIClient definida
- [ ] GenerateSummary implementado
- [ ] GenerateQuiz implementado
- [ ] Prompts validados
- [ ] Manejo de rate limits
- [ ] Tests con mocks

### Integración
- [ ] MaterialUploadedProcessor actualizado
- [ ] Flujo completo funciona
- [ ] Errores manejados apropiadamente
- [ ] Estados actualizados correctamente
- [ ] Tests de integración pasan

### General
- [ ] `make build` exitoso
- [ ] `make test` todos pasan
- [ ] `make lint` sin errores
- [ ] Cobertura >70%
- [ ] Documentación actualizada

---

## 🎯 Criterios de Aceptación

La Fase 5 se considera **COMPLETADA** cuando:

1. ✅ Cliente S3 descarga archivos correctamente
2. ✅ Extractor PDF extrae texto de PDFs
3. ✅ Cliente OpenAI genera resúmenes y quizzes
4. ✅ MaterialUploadedProcessor usa servicios reales
5. ✅ No hay datos hardcoded/simulados
6. ✅ Tests >70% cobertura
7. ✅ PR aprobado y mergeado a `dev`
8. ✅ Tag `fase-5-complete` creado

---

## 💰 Costos Estimados

| Servicio | Costo por Material |
|----------|-------------------|
| S3 Download (1MB) | ~$0.0001 |
| OpenAI GPT-4 (resumen + quiz) | ~$0.03-0.06 |
| **Total** | **~$0.03-0.06** |

**Costo mensual estimado** (1000 materiales): ~$30-60/mes

---

## 🚨 Gestión de Riesgos

### Riesgo: Costos altos de OpenAI
**Mitigación:**
- Usar modelos más económicos en desarrollo
- Implementar caché de respuestas
- Limitar tokens por request

### Riesgo: PDFs sin texto (escaneados)
**Mitigación:**
- Detectar PDFs escaneados
- Retornar error claro
- Documentar necesidad de OCR (fase futura)

### Riesgo: Archivos grandes en S3
**Mitigación:**
- Validar tamaño antes de descargar
- Timeouts configurables
- Streaming para archivos grandes

---

## 📚 Referencias

- **Plan Técnico Detallado:** [PLAN_TECNICO.md](./PLAN_TECNICO.md)
- **Tareas:** [TAREAS.md](./TAREAS.md)
- **Validación:** [VALIDACION.md](./VALIDACION.md)
- **Deuda Técnica:** [../../documents/mejoras/DEUDA_TECNICA.md](../../documents/mejoras/DEUDA_TECNICA.md)

---

## ⏭️ Siguiente Fase

**Fase 6: Sistemas de Notificaciones**
Ver: `plan-mejoras/fase-6/README.md`

---

**Última actualización:** 2025-12-23
**Versión:** 1.0
**Origen:** PT-008 edugo_analisis
