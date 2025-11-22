# Estándares de Cobertura de Código - edugo-worker

## Threshold Actual

**Mínimo requerido:** 33%

Alineado con:
- api-mobile: 33%
- api-administracion: 33%

## Ejecución Local

```bash
# Generar reporte de coverage
go test -coverprofile=coverage/coverage.out -covermode=atomic ./...

# Ver coverage total
go tool cover -func=coverage/coverage.out | tail -1

# Generar reporte HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html
```

## Verificar Threshold

```bash
# Verificar que cumple threshold
COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
THRESHOLD=33.0

if (( $(echo "$COVERAGE >= $THRESHOLD" | bc -l) )); then
  echo "✅ Coverage OK: ${COVERAGE}%"
else
  echo "❌ Coverage bajo: ${COVERAGE}% (mínimo ${THRESHOLD}%)"
fi
```

## Coverage por Paquete

> ⚠️ **Nota:** Los siguientes comandos requieren que existan archivos `*_test.go` en el proyecto.  
> Si aún no hay tests implementados, estos comandos fallarán. Primero implementa tests unitarios antes de generar reportes de coverage.

```bash
# Ver coverage por paquete
go tool cover -func=coverage/coverage.out | grep -E "^github.com/EduGoGroup/edugo-worker"

# Paquetes con coverage bajo (<33%)
go tool cover -func=coverage/coverage.out | awk '{gsub(/%/, "", $NF); if ($NF < 33 && $NF ~ /^[0-9]/) print $0}'
```

## Mejorar Coverage

### 1. Identificar código sin coverage

```bash
# Generar reporte HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html

# Buscar líneas rojas (sin coverage)
```

### 2. Agregar tests

```go
// Ejemplo: test para función sin coverage
func TestProcessJob(t *testing.T) {
    // Arrange
    job := &Job{
        ID: "test-123",
        Type: "email",
    }

    // Act
    result, err := ProcessJob(job)

    // Assert
    if err != nil {
        t.Errorf("Expected no error, got %v", err)
    }
    if result == nil {
        t.Error("Expected result, got nil")
    }
}
```

### 3. Verificar mejora

```bash
# Ejecutar tests con nuevo test
go test -coverprofile=coverage/coverage.out ./...

# Ver nueva coverage
go tool cover -func=coverage/coverage.out | tail -1
```

## Exclusiones de Coverage

Archivos excluidos de threshold (pero sí se miden):

- `cmd/main.go` - Entry point (difícil de testear)
- `*_mock.go` - Mocks generados
- `internal/testhelpers/` - Helpers de testing

**Nota:** Estos archivos NO se excluyen del reporte, solo no se consideran críticos para el threshold.

## CI/CD

### test.yml

Coverage threshold se verifica en cada PR:

```yaml
- name: Verificar umbral de cobertura
  run: |
    COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
    THRESHOLD=33.0

    if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
      echo "❌ Coverage ${COVERAGE}% < ${THRESHOLD}%"
      exit 1
    fi
```

### Codecov

Reports suben a Codecov para tracking histórico:

```bash
# Ver en: https://codecov.io/gh/EduGoGroup/edugo-worker
```

Configurado en `.github/workflows/test.yml`:
- Flag: `worker`
- fail_ci_if_error: false (no falla CI si Codecov falla)

## Plan de Mejora

| Fase | Threshold | Fecha Objetivo | Estado |
|------|-----------|----------------|--------|
| **Sprint 3** | 33% | Nov 2025 | ✅ Implementado |
| Sprint 5 | 40% | Dic 2025 | ⏳ Pendiente |
| Sprint 7 | 50% | Ene 2026 | ⏳ Pendiente |
| Sprint 10 | 60% | Feb 2026 | ⏳ Pendiente |

### Roadmap

1. **Sprint 3 (actual):** Establecer baseline 33%
2. **Sprint 5:** Mejorar coverage crítico a 40%
   - Agregar tests para handlers principales
   - Agregar tests para servicios de negocio
3. **Sprint 7:** Alcanzar 50%
   - Agregar tests de integración
   - Mejorar tests de repositorios
4. **Sprint 10:** Objetivo 60%
   - Tests end-to-end
   - Tests de errores y edge cases

## Tipos de Tests

### Unit Tests (Unitarios)

```go
func TestCalculateTotal(t *testing.T) {
    result := CalculateTotal(100, 0.1)
    expected := 110.0

    if result != expected {
        t.Errorf("Expected %f, got %f", expected, result)
    }
}
```

**Coverage esperado:** 80-100% de funciones de negocio

### Integration Tests (Integración)

```go
func TestJobRepository_Create(t *testing.T) {
    // Setup: DB test container
    db := setupTestDB(t)
    defer db.Close()

    repo := NewJobRepository(db)

    // Test
    job, err := repo.Create(&Job{Type: "email"})

    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, job.ID)
}
```

**Coverage esperado:** 60-80% de repositorios y servicios

### E2E Tests (End-to-End)

```go
func TestEmailJobWorkflow(t *testing.T) {
    // Setup: Full stack
    app := setupTestApp(t)

    // Enqueue job
    jobID := app.EnqueueJob("email", payload)

    // Wait for processing
    time.Sleep(2 * time.Second)

    // Verify result
    job := app.GetJob(jobID)
    assert.Equal(t, "completed", job.Status)
}
```

**Coverage esperado:** 40-60% de flujos completos

## Comandos Útiles

### Ejecutar solo tests con coverage alta

```bash
# Tests que probablemente aumenten coverage
go test -cover -v ./internal/services/...
go test -cover -v ./internal/handlers/...
go test -cover -v ./internal/repositories/...
```

### Comparar coverage entre branches

```bash
# Main branch
git checkout main
go test -coverprofile=coverage/main.out ./...
MAIN_COV=$(go tool cover -func=coverage/main.out | tail -1 | awk '{print $NF}')

# Feature branch
git checkout feature/new-feature
go test -coverprofile=coverage/feature.out ./...
FEATURE_COV=$(go tool cover -func=coverage/feature.out | tail -1 | awk '{print $NF}')

echo "Main: $MAIN_COV"
echo "Feature: $FEATURE_COV"
```

### Coverage por archivo

```bash
# Ver coverage de un archivo específico
go test -coverprofile=coverage/coverage.out ./...
go tool cover -func=coverage/coverage.out | grep "internal/services/job_service.go"
```

## Troubleshooting

### Coverage reportado incorrectamente

```bash
# Limpiar caché
go clean -testcache

# Regenerar coverage
go test -coverprofile=coverage/coverage.out ./...
```

### Tests pasan pero coverage no aumenta

Verificar que:
1. Tests ejecutan el código (no solo declaran funciones)
2. No hay `t.Skip()` saltando tests
3. Coverage mode es `atomic` (mejor que `set`)

### bc command not found

```bash
# macOS
brew install bc

# Linux (Ubuntu/Debian)
sudo apt-get install bc

# Linux (RHEL/CentOS)
sudo yum install bc
```

## Referencias

- [Go Coverage Tool](https://go.dev/blog/cover)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Codecov Go Guide](https://docs.codecov.com/docs/go)
- [api-mobile coverage](../../../api-mobile/docs/COVERAGE-STANDARDS.md)
- [api-administracion coverage](../../../api-administracion/docs/COVERAGE-STANDARDS.md)

## Preguntas Frecuentes

**P: ¿Por qué 33% y no 80%?**
R: 33% es un baseline realista para comenzar. Mejorará gradualmente hasta 60%.

**P: ¿Qué pasa si un PR baja el coverage?**
R: El CI falla y el PR no se puede mergear hasta que se agreguen tests.

**P: ¿Puedo saltar el threshold temporalmente?**
R: No. Si necesitas mergear urgente, agrega tests stub que se mejorarán después.

**P: ¿Coverage 100% es el objetivo?**
R: No. 60-70% es óptimo. 100% puede llevar a tests poco útiles.

**P: ¿Cómo sé qué testear?**
R: Prioriza: 1) Lógica de negocio, 2) Handlers, 3) Servicios, 4) Repositorios

---

**Última actualización:** 2025-11-22 - Sprint 3
**Responsable:** Equipo DevOps EduGo
**Threshold actual:** 33%
**Próxima revisión:** Sprint 5 (Dic 2025 - objetivo 40%)
