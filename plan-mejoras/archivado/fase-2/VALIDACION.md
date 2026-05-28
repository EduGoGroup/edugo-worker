# Validación - Fase 2: Integraciones Externas

---

## ✅ Checklist de Validación

### 1. Cliente OpenAI

**Tests Unitarios:**
```bash
go test ./internal/infrastructure/nlp/openai -v
```

- [ ] Cliente se conecta a OpenAI
- [ ] Prompts se envían correctamente
- [ ] Respuestas se parsean
- [ ] Rate limits se manejan (429)
- [ ] Timeouts se manejan
- [ ] Mocks funcionan en tests

**Test Manual (con API key real):**
```bash
export OPENAI_API_KEY=sk-...
go test ./internal/infrastructure/nlp/openai -v -run TestRealAPI
```

- [ ] Resumen generado es coherente
- [ ] Quiz generado tiene formato correcto
- [ ] Tokens consumidos dentro de límite

---

### 2. Extractor PDF

**Tests con PDFs de ejemplo:**
```bash
go test ./internal/infrastructure/pdf -v
```

- [ ] Extrae texto de PDF simple
- [ ] Extrae texto de PDF complejo
- [ ] Detecta PDF sin texto (escaneado)
- [ ] Limpieza de texto funciona
- [ ] Maneja PDFs corruptos con error claro

**Validación Manual:**
```bash
# Con un PDF real
./bin/pdf-test /path/to/sample.pdf
```

---

### 3. Cliente S3

**Tests (con Localstack o mocks):**
```bash
go test ./internal/infrastructure/storage/s3 -v
```

- [ ] Descarga archivos correctamente
- [ ] Maneja errores 404
- [ ] Retry funciona ante fallos
- [ ] Timeout configurado funciona
- [ ] Valida tipos de archivo

---

### 4. Integración Completa

**Test End-to-End:**
```bash
# Setup ambiente de test
docker-compose -f docker-compose.test.yml up -d

# Ejecutar test de integración
go test ./internal/application/processor -v -run TestMaterialUploadedE2E
```

**Criterios:**
- [ ] Flujo completo: S3 → PDF → OpenAI → MongoDB
- [ ] Datos guardados son generados (no hardcoded)
- [ ] Errores en cada etapa se capturan
- [ ] Logs muestran progreso claro

**Validación Manual:**
```bash
# Publicar evento de test a RabbitMQ
./scripts/publish-test-event.sh material_uploaded

# Verificar en MongoDB
mongo edugo_test
db.material_summaries.findOne({material_id: "test-123"})
```

---

### 5. Cobertura y Calidad

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Metas:**
- [ ] `internal/infrastructure/nlp/`: >80%
- [ ] `internal/infrastructure/pdf/`: >75%
- [ ] `internal/infrastructure/storage/`: >75%
- [ ] `internal/application/processor/`: >75%
- [ ] Global: >70%

---

## 🎯 Criterios de Aceptación

✅ **FASE 2 EXITOSA** si:

1. Cliente OpenAI genera contenido real
2. PDF extractor funciona con PDFs reales
3. Cliente S3 descarga archivos
4. Integración end-to-end funciona
5. Datos guardados son generados (no hardcoded)
6. Tests >70% cobertura
7. CI/CD pasa
8. PR aprobado y mergeado
