# Validación - Fase 3: Testing y Calidad

---

## ✅ Checklist de Validación

### 1. Cobertura Global

```bash
# Generar reporte de cobertura
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Ver reporte HTML
go tool cover -html=coverage.out
```

**Verificar metas:**
- [ ] `internal/application/processor`: >90%
- [ ] `internal/domain/service`: >95%
- [ ] `internal/infrastructure/nlp`: >85%
- [ ] `internal/infrastructure/pdf`: >85%
- [ ] `internal/infrastructure/storage`: >85%
- [ ] Global: >80%

---

### 2. Tests Unitarios

```bash
# Ejecutar todos los tests unitarios
go test ./... -v -short

# Verificar tiempo de ejecución
time go test ./... -short
```

**Criterios:**
- [ ] Todos los tests pasan
- [ ] Tiempo total <10 segundos
- [ ] Sin tests flaky
- [ ] Sin tests ignorados (skip) sin justificación

---

### 3. Tests de Integración

```bash
# Con Testcontainers
go test ./... -v -tags=integration

# O con Docker Compose
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

**Criterios:**
- [ ] Tests de integración pasan
- [ ] Cleanup de recursos funciona
- [ ] Tests en CI/CD funcionan
- [ ] Sin fugas de conexiones

---

### 4. Calidad de Mocks

**Verificar uso de mocks:**
```bash
# Buscar uso de mocks en tests
grep -r "mock\." --include="*_test.go" internal/
```

**Criterios:**
- [ ] Todos los tests usan mocks (no DB real)
- [ ] Mocks son fáciles de configurar
- [ ] Mocks verifican comportamiento esperado

---

### 5. CI/CD

**Verificar configuración:**
```yaml
# .github/workflows/test.yml debe incluir:
- go test -race -coverprofile=coverage.out ./...
- go tool cover -func=coverage.out
- Reportar cobertura a codecov/coveralls
```

**Criterios:**
- [ ] Tests se ejecutan en CI
- [ ] Cobertura se reporta
- [ ] CI falla si cobertura <80%
- [ ] Tests con race detector

---

## 🎯 Criterios de Aceptación

✅ **FASE 3 EXITOSA** si:

1. Cobertura >80% global
2. Tests unitarios rápidos (<10s)
3. Tests de integración funcionan
4. Mocks implementados
5. CI/CD ejecuta todos los tests
6. Documentación de testing creada
7. PR aprobado y mergeado
