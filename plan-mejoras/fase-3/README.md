# Fase 3: Testing y Calidad

> **Objetivo:** Aumentar significativamente la cobertura de tests, crear mocks robustos y establecer estándares de calidad.
>
> **Duración estimada:** 2-3 semanas
> **Complejidad:** Media
> **Riesgo:** Bajo
> **Prerequisito:** Fase 2 completada

---

## 🎯 Objetivos

1. ✅ Aumentar cobertura de tests a >80%
2. ✅ Crear mocks e interfaces para todas las dependencias
3. ✅ Implementar tests de integración robustos
4. ✅ Establecer estándares de testing
5. ✅ Documentación de testing

---

## 📦 Entregables

### E3.1: Interfaces y Mocks
- Extraer interfaces de todos los repositories
- Crear mocks para todas las dependencias externas
- Test doubles para servicios

### E3.2: Tests Unitarios Completos
- Tests para todos los processors
- Tests para domain services
- Tests para infrastructure
- Cobertura >80%

### E3.3: Tests de Integración
- Setup con Docker/Testcontainers
- Tests end-to-end de flujos completos
- Tests de error handling

### E3.4: Documentación y Estándares
- Guía de testing
- Ejemplos de tests
- CI/CD mejorado

---

## 🔄 Estructura de Tests

```
internal/
├── application/
│   └── processor/
│       ├── material_uploaded_processor.go
│       ├── material_uploaded_processor_test.go    # Tests unitarios
│       └── integration_test.go                    # Tests integración
├── domain/
│   ├── repository/
│   │   ├── interfaces.go                         # Interfaces extraídas
│   │   └── mock/
│   │       ├── material_summary_repository.go    # Mocks
│   │       └── material_assessment_repository.go
│   └── service/
│       ├── summary_validator.go
│       └── summary_validator_test.go
└── infrastructure/
    ├── nlp/
    │   └── mock/
    │       └── openai_client_mock.go
    └── storage/
        └── mock/
            └── s3_client_mock.go

testutil/                                          # Utilidades de testing
├── fixtures.go                                    # Datos de prueba
├── assertions.go                                  # Assertions custom
└── builders.go                                    # Test data builders
```

---

## 📋 Tareas Principales

### T3.1: Extraer Interfaces (Semana 1)
- Definir interfaces para repositories
- Definir interfaces para servicios externos
- Refactorizar código para usar interfaces

### T3.2: Crear Mocks (Semana 1)
- Mocks para repositories
- Mocks para OpenAI
- Mocks para S3
- Mocks para PDF extractor

### T3.3: Tests Unitarios (Semana 1-2)
- Tests de processors (100%)
- Tests de validators (100%)
- Tests de domain services (100%)
- Tests de infrastructure (>80%)

### T3.4: Tests de Integración (Semana 2)
- Setup con Testcontainers
- Tests de flujos completos
- Tests de error scenarios

### T3.5: Documentación (Semana 3)
- Guía de testing
- Ejemplos
- Mejoras en CI/CD

---

## 📊 Metas de Cobertura

| Paquete | Meta | Actual | Prioridad |
|---------|------|--------|-----------|
| `internal/application/processor` | 90% | ~30% | 🔴 Alta |
| `internal/domain/service` | 95% | ~40% | 🔴 Alta |
| `internal/domain/valueobject` | 100% | ~60% | 🟡 Media |
| `internal/infrastructure/persistence` | 80% | ~50% | 🟡 Media |
| `internal/infrastructure/nlp` | 85% | ~0% | 🔴 Alta |
| `internal/infrastructure/pdf` | 85% | ~0% | 🔴 Alta |
| `internal/infrastructure/storage` | 85% | ~0% | 🔴 Alta |
| `cmd/` | 70% | ~10% | 🟢 Baja |
| **Global** | **>80%** | **~35%** | **🔴 Alta** |

---

## ✅ Checklist de Validación

### Interfaces
- [ ] Todas las dependencias tienen interfaces
- [ ] Código usa interfaces en lugar de tipos concretos
- [ ] Interfaces documentadas

### Mocks
- [ ] Mocks para todos los repositories
- [ ] Mocks para servicios externos
- [ ] Mocks son fáciles de usar en tests

### Tests Unitarios
- [ ] >90% cobertura en processors
- [ ] >95% cobertura en domain services
- [ ] >80% cobertura en infrastructure
- [ ] Tests rápidos (<5s total)

### Tests de Integración
- [ ] Flujos completos testeados
- [ ] Error scenarios cubiertos
- [ ] Tests con Docker funcionan en CI

### Documentación
- [ ] Guía de testing creada
- [ ] Ejemplos documentados
- [ ] CI/CD actualizado

---

## 🎯 Criterios de Aceptación

Fase 3 **COMPLETADA** cuando:

1. ✅ Cobertura global >80%
2. ✅ Interfaces extraídas para todas las dependencias
3. ✅ Mocks creados y funcionando
4. ✅ Tests de integración implementados
5. ✅ Documentación de testing creada
6. ✅ CI/CD ejecuta todos los tests
7. ✅ PR aprobado y mergeado
8. ✅ Tag `fase-3-complete` creado

---

## 📚 Referencias

- [Deuda Técnica](../../documents/mejoras/DEUDA_TECNICA.md) - DT-009, DT-010, DT-011
- [Refactorizaciones](../../documents/mejoras/REFACTORING.md) - RF-003, RF-008
- [Roadmap](../../documents/mejoras/ROADMAP.md) - Sprint 3

---

## ⏭️ Siguiente Fase

**Fase 4: Observabilidad y Resiliencia**
Ver: `plan-mejoras/fase-4/README.md`
