# Validación - Fase 5: Integraciones Core Avanzadas

> **Objetivo:** Validar que todas las integraciones funcionen correctamente antes de mergear a `dev`

---

## 🧪 Tests Unitarios

### Cliente S3
```bash
go test -v ./internal/infrastructure/storage/... -run TestS3Client
```

**Verificar:**
- [ ] DownloadFile retorna reader válido
- [ ] GetFileSize retorna tamaño correcto
- [ ] Errores de AWS manejados apropiadamente
- [ ] Retry funciona con errores transitorios

### Extractor PDF
```bash
go test -v ./internal/infrastructure/pdf/... -run TestPDFExtractor
```

**Verificar:**
- [ ] ExtractText extrae contenido correcto
- [ ] ExtractMetadata retorna datos válidos
- [ ] Maneja PDFs simples
- [ ] Maneja PDFs complejos
- [ ] Error claro con PDFs escaneados

### Cliente OpenAI
```bash
go test -v ./internal/infrastructure/ai/... -run TestOpenAIClient
```

**Verificar:**
- [ ] GenerateSummary retorna resumen coherente
- [ ] GenerateQuiz retorna JSON válido
- [ ] Rate limits manejados (mock 429)
- [ ] Timeouts manejados
- [ ] Prompts correctos

---

## 🔗 Tests de Integración

### Flujo Completo
```bash
go test -v ./internal/application/processor/... -run TestMaterialUploadedProcessor_Integration
```

**Verificar:**
- [ ] Descarga desde S3 funciona
- [ ] Extracción de PDF funciona
- [ ] Generación con OpenAI funciona
- [ ] Datos guardados en MongoDB
- [ ] Estados actualizados correctamente

### Test Manual (Opcional)
1. Subir PDF real a S3
2. Triggear evento `material.uploaded`
3. Verificar procesamiento completo
4. Validar resumen en MongoDB
5. Validar quiz en MongoDB

---

## 📊 Cobertura de Tests

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Criterio:** Cobertura >70%

**Verificar áreas críticas:**
- [ ] S3Client: >80%
- [ ] PDFExtractor: >75%
- [ ] OpenAIClient: >75%
- [ ] MaterialUploadedProcessor: >70%

---

## 🏗️ Build y Lint

### Build
```bash
make build
```

**Verificar:**
- [ ] Compilación sin errores
- [ ] Binario generado correctamente
- [ ] Sin warnings de imports no usados

### Lint
```bash
make lint
```

**Verificar:**
- [ ] Sin errores de golangci-lint
- [ ] Sin errores de gofmt
- [ ] Sin imports circulares

---

## 🔒 Validación de Seguridad

### Credenciales
- [ ] No hay API keys hardcoded
- [ ] Variables de entorno documentadas
- [ ] `.env.example` creado
- [ ] Secrets no en commits

### Dependencias
```bash
go list -m all | grep -i security
```

- [ ] Dependencias actualizadas
- [ ] Sin vulnerabilidades conocidas

---

## 📝 Documentación

### Código
- [ ] Comentarios en interfaces públicas
- [ ] Ejemplos de uso documentados
- [ ] Errores documentados

### README
- [ ] Configuración de AWS documentada
- [ ] Configuración de OpenAI documentada
- [ ] Costos estimados documentados
- [ ] Limitaciones documentadas

---

## 🚀 Pre-PR Checklist

Antes de crear el Pull Request:

- [ ] Todos los tests pasan
- [ ] Cobertura >70%
- [ ] Build exitoso
- [ ] Lint limpio
- [ ] Documentación actualizada
- [ ] Commits bien escritos
- [ ] Branch sincronizado con `dev`
- [ ] No hay conflictos
- [ ] Variables de entorno documentadas
- [ ] Plan técnico revisado y completo

---

## ✅ Criterios de Aceptación Final

La fase está lista para merge cuando:

1. ✅ Todos los tests unitarios pasan
2. ✅ Tests de integración pasan
3. ✅ Cobertura >70%
4. ✅ Build y lint exitosos
5. ✅ Documentación completa
6. ✅ Code review aprobado
7. ✅ CI/CD verde

---

**Última actualización:** 2025-12-23
