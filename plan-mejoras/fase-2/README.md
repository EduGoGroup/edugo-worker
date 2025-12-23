# Fase 2: Integraciones Externas

> **Objetivo:** Implementar las integraciones con servicios externos: OpenAI, extracción de PDF y AWS S3.
>
> **Duración estimada:** 3-4 semanas
> **Complejidad:** Alta
> **Riesgo:** Alto
> **Prerequisito:** Fase 1 completada

---

## 🎯 Objetivos

1. ✅ Implementar cliente OpenAI para generación de resúmenes y quizzes
2. ✅ Implementar extracción de texto desde PDFs
3. ✅ Implementar cliente S3 para descarga de archivos
4. ✅ Reemplazar datos hardcoded con generación real
5. ✅ Agregar manejo robusto de errores y retries

---

## 📦 Entregables

### E2.1: Cliente OpenAI
- `internal/infrastructure/nlp/openai/client.go`
- Prompts para resumen y quiz
- Parser de respuestas JSON
- Manejo de rate limits y errores
- Tests con mocks

### E2.2: Extractor PDF
- `internal/infrastructure/pdf/extractor.go`
- Soporte para diferentes tipos de PDF
- Limpieza y normalización de texto
- Tests con PDFs de ejemplo

### E2.3: Cliente S3
- `internal/infrastructure/storage/s3/client.go`
- Descarga de archivos con retry
- Validación de archivos
- Tests

### E2.4: Integración en Processors
- Actualizar MaterialUploadedProcessor
- Flujo completo: S3 → PDF → OpenAI → MongoDB
- Manejo de errores por etapa
- Tests de integración

---

## 🔄 Estructura de Archivos

```
internal/infrastructure/
├── nlp/
│   ├── interface.go              # Interfaz común NLP
│   ├── openai/
│   │   ├── client.go            # Cliente HTTP OpenAI
│   │   ├── client_test.go
│   │   ├── prompts.go           # Templates de prompts
│   │   ├── parser.go            # Parse respuestas
│   │   └── config.go            # Configuración
│   └── mock/
│       └── mock_client.go       # Mock para tests
├── pdf/
│   ├── extractor.go             # Extracción de texto
│   ├── extractor_test.go
│   ├── cleaner.go               # Limpieza de texto
│   └── testdata/
│       ├── simple.pdf
│       ├── complex.pdf
│       └── scanned.pdf
└── storage/
    ├── interface.go
    ├── s3/
    │   ├── client.go            # Cliente AWS S3
    │   ├── client_test.go
    │   └── downloader.go
    └── mock/
        └── mock_storage.go
```

---

## 🔑 Configuración Requerida

### Variables de Entorno

```bash
# OpenAI
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4-turbo
OPENAI_MAX_TOKENS=2000
OPENAI_TEMPERATURE=0.7

# AWS S3
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
S3_BUCKET_NAME=edugo-materials

# Opcionales
OPENAI_TIMEOUT=60s
S3_DOWNLOAD_TIMEOUT=120s
```

### Actualizar config.yaml

```yaml
nlp:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    model: ${OPENAI_MODEL:-gpt-4-turbo}
    max_tokens: ${OPENAI_MAX_TOKENS:-2000}
    temperature: ${OPENAI_TEMPERATURE:-0.7}
    timeout: ${OPENAI_TIMEOUT:-60s}

storage:
  provider: s3
  s3:
    region: ${AWS_REGION:-us-east-1}
    bucket: ${S3_BUCKET_NAME}
    timeout: ${S3_DOWNLOAD_TIMEOUT:-120s}

pdf:
  max_size_mb: 50
  allowed_types: [".pdf"]
```

---

## 📋 Commits Sugeridos

**Commit 1: Estructura base**
```
feat(fase-2): agregar estructura base para integraciones externas

- Crear carpetas nlp/, pdf/, storage/
- Definir interfaces comunes
- Agregar configuración base
```

**Commit 2: Cliente OpenAI**
```
feat(fase-2): implementar cliente OpenAI

- Cliente HTTP con manejo de errores
- Prompts para resumen y quiz
- Parser de respuestas JSON
- Tests con mocks
```

**Commit 3: Extractor PDF**
```
feat(fase-2): implementar extracción de PDF

- Extractor usando pdfcpu/unidoc
- Limpieza de texto
- Tests con PDFs ejemplo
```

**Commit 4: Cliente S3**
```
feat(fase-2): implementar cliente S3

- Descarga de archivos
- Retry con backoff
- Tests
```

**Commit 5: Integración**
```
feat(fase-2): integrar servicios en MaterialUploadedProcessor

- Flujo: S3 → PDF → OpenAI → MongoDB
- Reemplazar datos hardcoded
- Manejo de errores
- Tests de integración
```

---

## ✅ Checklist de Validación

### Cliente OpenAI
- [ ] Genera resúmenes coherentes
- [ ] Genera quizzes válidos
- [ ] Maneja rate limits (429)
- [ ] Maneja timeouts
- [ ] Tests con mocks pasan

### Extractor PDF
- [ ] Extrae texto de PDFs simples
- [ ] Extrae texto de PDFs complejos
- [ ] Maneja PDFs sin texto (error claro)
- [ ] Limpia texto correctamente
- [ ] Tests con PDFs reales

### Cliente S3
- [ ] Descarga archivos correctamente
- [ ] Maneja errores de red
- [ ] Retry funciona
- [ ] Valida tipos de archivo
- [ ] Tests pasan

### Integración
- [ ] Flujo completo funciona end-to-end
- [ ] Errores en cada etapa se manejan
- [ ] Datos guardados son reales (no hardcoded)
- [ ] Tests de integración pasan
- [ ] Cobertura >70%

---

## 🚨 Manejo de Riesgos

### Riesgo: Costo de OpenAI alto
**Mitigación:**
- Usar modelos más económicos en desarrollo
- Implementar caché de respuestas
- Limitar tokens por request

### Riesgo: PDFs sin texto
**Mitigación:**
- Detectar PDFs escaneados
- Retornar error claro
- Documentar necesidad de OCR (Fase futura)

### Riesgo: Archivos grandes en S3
**Mitigación:**
- Validar tamaño antes de descargar
- Timeout configurables
- Streaming para archivos grandes

---

## 🎯 Criterios de Aceptación

Fase 2 **COMPLETADA** cuando:

1. ✅ Cliente OpenAI funcional
2. ✅ Extracción de PDF funcional
3. ✅ Cliente S3 funcional
4. ✅ MaterialUploadedProcessor usa servicios reales
5. ✅ Datos guardados son generados (no hardcoded)
6. ✅ Tests >70% cobertura
7. ✅ PR aprobado y mergeado
8. ✅ Tag `fase-2-complete` creado

---

## 📚 Referencias

- [Deuda Técnica](../../documents/mejoras/DEUDA_TECNICA.md) - DT-002, DT-003
- [Roadmap](../../documents/mejoras/ROADMAP.md) - EP-002, EP-003

---

## ⏭️ Siguiente Fase

**Fase 3: Testing y Calidad**
Ver: `plan-mejoras/fase-3/README.md`
