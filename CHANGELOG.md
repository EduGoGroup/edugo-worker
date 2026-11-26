# Changelog

Todos los cambios notables en edugo-worker serán documentados en este archivo.

El formato está basado en [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- feat(prep): la preparación de una pregunta acaba en «lista» o en «falló» (HU-IA.4, bugs 0139 y 0156, deuda 163). Un artefacto inválido, o un error de calidad (`llm.ErrLLMQuality`), se reintenta una vez a `llm.InvalidOutputRetryTemperature`; si persiste es `ErrInvalidPrep`, ahora permanente en `classifyError`, y la pregunta queda `failed` (PUT `prep-status` nuevo) antes de devolver el error. Un fallo de servicio deja `pending`. Con la IA apagada queda `none`. Una pregunta ya `ready` en el `prep-source` no vuelve a llamar al modelo (`preparación ya lista, no se repite`). `llm.PrepRequest.Temperature` nuevo, que Ollama respeta. Guiones: P3 invertido, P3n nuevo (sin modelo), R13 y P2 esperan `none`.
- feat(llm): tope de inferencias simultáneas contra el modelo (HU-IA.10). Paquete nuevo `internal/llm/inferencecap`: un limitador por backend (`local`, generación y embeddings contra Ollama; `api`, el proveedor de pago), en orden de llegada. El turno se pide justo antes de abrir la petición HTTP en los tres puntos de salida (`generateWith`, `Embed` y `complete`), así que la espera queda fuera del timeout de la llamada. La revisión toma el turno de su modo para todo el mensaje **antes** del candado y lo suelta en toda salida; el material, la preparación y los embeddings lo piden por llamada. Config nueva `llm.local.max_in_flight` (`LLM_LOCAL_MAX_IN_FLIGHT`, por defecto 1) y `llm.api.max_in_flight` (`LLM_API_MAX_IN_FLIGHT`, por defecto 4); un valor negativo no arranca. Histograma `worker_inference_wait_seconds` por limitador y una línea de log por turno. `prefetch_count` no cambia. Guion nuevo `scripts/e2e/concurrency/run.py` (caso `C1`, con modelo: se mide, no es gate).

### Changed

- chore(054): saneamiento de rojos heredados (plan 054, ola S). `make lint` falla de verdad: sin `|| echo`, sale con error si falta `golangci-lint` y propaga su código de salida (S.3, MEJ-07). `.golangci.yml` con `run.build-tags: [integration]`, `allow-parallel-runners` y sin recorte de hallazgos (`max-same-issues: 0`, `max-issues-per-linter: 0`), y los 3 `errcheck` de `test/integration/setup.go` que destapó (S.4). `make test-integration` fuerza `GOWORK=off` y `RUN_INTEGRATION_TESTS=true`, corre con `-race` y falla si Docker no responde; queda documentado como gate local obligatorio antes de cada PR (S.6).
- chore(054): homologación a Go 1.27.4 (plan 054, E3.6). Módulos de `edugo-shared` a la banda `v0.900.x` publicada en Go 1.27.4; `amqp091-go v1.16.0`, `prometheus/client_golang v1.25.0`, `client_model v0.6.3`, `ledongthuc/pdf` a la última y `testify v1.12.1`. golangci-lint `v2.14.0` con `.golangci.yml` (`version: "2"`) nuevo; `govulncheck v1.9.0` como paso de la CI y como `make vuln` (dentro de `ci-local` y `ci-docker`); `actions/checkout@v7` y `actions/setup-go@v7`; `GOWORK=off` en los workflows.
- feat(notifications): el worker delega la entrega al Notification Gateway (plan 020 F2.3, D13). Nuevo `client.ServiceTokenProvider` (firma service JWT M2M HS256 con `auth.ServiceJWTManager` de edugo-shared, con caché) y `client.NotificationDispatchClient` (HTTP POST a `/api/v1/internal/notifications/dispatch` con `Authorization: Bearer <service-jwt>`). Los processors `assessment.assigned` (N alumnos → 1 dispatch con N recipients), `assessment.attempt_recorded` (docente) y `assessment.reviewed` (alumno) ahora resuelven destinatarios y delegan; un fallo de dispatch (5xx/timeout) se propaga para que RabbitMQ reintente (la idempotencia del gateway evita duplicados). Log `dispatch_requested`.
- Config nueva `notification_gateway` (`base_url`, `timeout`, `service_jwt.{secret,issuer,audience,client_id,ttl}`).

### Removed

- `.github/workflows/test.yml` («Tests with Coverage»): terminaba en `startup_failure` en cada PR (bug 0088) y duplicaba a `ci.yml` con un umbral de cobertura de `0.0` (plan 054, S.2).
- `NotificationCreator` y todo SQL directo a `notifications.notifications` en el worker (migrado a edugo-api-platform en F2.2). El worker ya no escribe in-app ni tiene credenciales FCM/APNs.

### Tests

- test(notifications): cierre F2.4 — roundtrip cruzado worker→gateway a través del `NotificationDispatchClient` real (service JWT firmado) contra un httptest que simula `/api/v1/internal/notifications/dispatch`; verifica el contrato `DispatchRequest` (recipients/fan-out, type, resource_id, idempotency_key, push_data, source) y el header `Authorization`.

---

## [0.7.0] - 2025-12-24

### Tipo de Release: patch

- feat(fase-5): Mejoras a Integraciones Core - S3, PDF y NLP Multi-Provider (#36)

---

## [0.6.0] - 2025-11-25

### Tipo de Release: patch

- release: Auth centralizada Sprint 4 con WorkerAuthClient (#27)
- chore: actualizar infraestructura a v0.10.1

---

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
- Actualizado a Go 1.25.0
- Versionado corregido a v0.x.x (proyecto en desarrollo)
- Eliminado auto-version.yml (reemplazado por manual-release.yml)
- Adaptaciones específicas para workers (sin HTTP handlers)

### Fixed
- Corrección de errores de linter (errcheck)
- Permisos de GitHub Container Registry configurados

[Unreleased]: https://github.com/EduGoGroup/edugo-worker/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/EduGoGroup/edugo-worker/releases/tag/v0.1.0
