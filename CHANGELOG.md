# Changelog

Todos los cambios notables en edugo-worker serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2025-11-22

### Tipo de Release: minor

- Release: Sprint Entities Adaptation v0.5.0 (#25)
- fix: corregir 2 errores finales de errcheck en material_summary_repository.go
- fix: corregir 7 errores adicionales de errcheck en material_event_repository.go
- fix: corregir 10 errores de errcheck (verificación de valores de retorno de error)
- test: re-ejecutar workflows después de fix en infrastructure
- docs(sprint-4): agregar sección PR a main (FASE 3 extendida)
- feat(sprint-4): completar FASE 1 - workflows reusables con stubs
- docs(sprint-4): agregar sección FASE 3 a lecciones aprendidas
- docs(sprint-4): completar Tarea 5 - plan de testing (stub)
- docs(sprint-4): completar Tarea 4 - documentación workflows reusables
- docs(sprint-4): actualizar SPRINT-STATUS tras Tarea 3
- refactor(sprint-4): completar Tarea 3 - migrar job test-coverage
- refactor(sprint-4): completar Tarea 2 - migrar job lint de ci.yml
- feat(sprint-4): completar Tarea 1 con stub - workflows reusables
- docs(sprint-4): inicializar SPRINT-4 FASE 1
- docs(sprint-4): agregar lecciones aprendidas de api-mobile
- docs: completar documentación FASE 3
- fix: resolver 4 comentarios de Copilot
- fix: ajustar threshold de coverage a 0% temporalmente (sin tests)
- fix: corregir branches en workflows (develop → dev)


---

## [0.4.2] - 2025-11-19

### Tipo de Release: patch

- Actualización de dev (#20)
- chore: actualizar a edugo-infrastructure mongodb@v0.9.0 (#19)

---

## [0.4.1] - 2025-11-18

### Tipo de Release: patch

- fix: actualizar edugo-shared/bootstrap a v0.9.0 (#18)

---

## [0.3.0] - 2025-11-18

### Tipo de Release: minor

- chore: sync dev to main - Sprint 01 release
- chore: sync dev to main - testing v0.6.2 migration (#14)

---

## [0.2.0] - 2025-11-13

### Tipo de Release: minor

- release: v0.2.0 - Go 1.24.10 estandarizado (#12)

---

## [0.1.2] - 2025-11-12

### Tipo de Release: patch

- chore: actualizar shared a v0.4.0 + migración completada (#10)
- fix: verificar commits en sync-main-to-dev
- docs: documentar GitHub App y actualizar a v2.1.4
- feat: implementar GitHub App Token para sincronización automática

---

## [Unreleased]

### Changed
- **Integración con edugo-infrastructure v0.8.0** (2025-11-18)
  - Actualizar mongodb a v0.6.0 (usa migraciones de infrastructure)
  - Actualizar postgres a v0.8.0
  - Eliminar scripts locales redundantes (ahora en infrastructure)
  - Collections MongoDB ahora provistas por infrastructure:
    - material_summary
    - material_assessment_worker
    - material_event

### Added
- **Sprint-01 Fase 2 - MongoDB Schema & Repositories** (2025-11-18)
  - Schemas MongoDB para material_summary, material_assessment y material_event
  - Scripts de inicialización: init_collections.js (651 líneas)
  - Scripts de datos de prueba: seed_data.js (753 líneas)
  - Documentación completa: MONGODB_SCHEMA.md (1539 líneas)
  - Auditoría del código: AUDITORIA_RESULTADOS.md (1660 líneas)
  - Entidades de dominio MongoDB:
    - MaterialSummary con validación y versionado
    - MaterialAssessment con preguntas y opciones
    - MaterialEvent para auditoría con TTL
  - Repositories MongoDB implementados:
    - MaterialSummaryRepository (CRUD completo + queries por idioma)
    - MaterialAssessmentRepository (CRUD + agregaciones)
    - MaterialEventRepository (auditoría + estadísticas)
  - Tests de integración con testcontainers (9 tests, 100% passing)
  - Collections MongoDB creadas con índices optimizados
  - TTL index en material_event (90 días de retención)

## [0.1.1] - 2025-11-03

### Tipo de Release: patch

- fix: corregir workflow sync-main-to-dev (#5)
- Dev (#4)

---

## [0.1.0] - 2025-11-01

### Added
- Sistema GitFlow profesional implementado
- Workflows de CI/CD automatizados:
  - CI Pipeline con tests y validaciones
  - Tests con cobertura y servicios de infraestructura
  - Manual Release workflow (TODO-EN-UNO) para control total de releases
  - Docker only workflow para builds manuales
  - Release automático con versionado semántico
- GitHub Copilot custom instructions en español (adaptado para workers)
- Migración a edugo-shared con arquitectura modular
- Submódulos: common, logger, database/postgres
- .gitignore completo para Go
- Documentación completa de workflows
- Patrones específicos de workers (processors, retry logic)

### Changed
- Actualizado a Go 1.25.3
- Versionado corregido a v0.x.x (proyecto en desarrollo)
- Eliminado auto-version.yml (reemplazado por manual-release.yml)
- Adaptaciones específicas para workers (sin HTTP handlers)

### Fixed
- Corrección de errores de linter (errcheck)
- Permisos de GitHub Container Registry configurados

[Unreleased]: https://github.com/EduGoGroup/edugo-worker/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EduGoGroup/edugo-worker/releases/tag/v0.1.0
