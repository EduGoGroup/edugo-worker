# EduGo Worker - Procesamiento Asíncrono

![Go Version](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go)
![Coverage](https://img.shields.io/badge/coverage-33%25%20min-brightgreen)
![Workflows](https://img.shields.io/badge/workflows-4-blue)
![Pre--commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)

Worker que consume eventos de RabbitMQ para procesar materiales educativos con IA.

> 🚀 **CI/CD Automatizado**: Este proyecto incluye workflows de GitHub Actions para testing, linting y deployment automático de imágenes Docker.

## 📋 Recent Changes (Sprint 3 - Nov 2025)

### Workflows Consolidados
- ✅ Eliminados 3 workflows Docker duplicados
- ✅ Mantenido solo `manual-release.yml` con control fino
- ✅ Reducción de workflows: 7 → 4 (-43%)
- ✅ Reducción de código: ~250 líneas eliminadas

### Tecnología Actualizada
- ✅ Go 1.25.3 (anteriormente 1.24.10)
- ✅ Pre-commit hooks (12 hooks configurados)
- ✅ Coverage threshold 33% mínimo

### Guías Disponibles
- [Release Workflow](docs/RELEASE-WORKFLOW.md) - Cómo hacer releases
- [Coverage Standards](docs/COVERAGE-STANDARDS.md) - Estándares de cobertura
- [Pre-commit Hooks](#-pre-commit-hooks) - Validaciones automáticas

## Responsabilidades

1. **Generación de Resumen y Quiz** (`material_uploaded`):
   - Descarga PDF desde S3
   - Extrae texto (OCR si es necesario)
   - Llama API NLP (OpenAI GPT-4) para generar resumen
   - Genera cuestionario con IA
   - Persiste en MongoDB (`material_summary`, `material_assessment`)
   - Actualiza PostgreSQL
   - Notifica docente

2. **Reprocesamiento** (`material_reprocess`):
   - Regenera resumen/quiz de material existente
   - Incrementa versión en MongoDB

3. **Notificaciones** (`assessment_attempt_recorded`):
   - Notifica docentes cuando estudiante completa quiz

4. **Limpieza** (`material_deleted`):
   - Elimina archivos S3
   - Elimina documentos MongoDB

5. **Bienvenida** (`student_enrolled`):
   - Envía email/push de bienvenida a nuevos estudiantes

## Tecnología

- Go 1.25.3 + RabbitMQ + MongoDB + PostgreSQL

## Dependencias del Ecosistema

### edugo-infrastructure v0.8.0+
- **mongodb v0.6.0** - Migraciones MongoDB (material_summary, material_assessment_worker, material_event)
- **postgres v0.8.0** - Migraciones PostgreSQL + helpers de testing
- **schemas** - Schemas de validación de eventos RabbitMQ
- Contratos estandarizados de mensajería

### edugo-shared v0.7.0
- `bootstrap` - Inicialización de aplicaciones
- `common` - Utilidades compartidas
- `logger` - Logging estructurado
- `database/postgres` - Helpers de PostgreSQL
- `lifecycle` - Gestión de ciclo de vida
- `testing` - Utilidades de testing con testcontainers

### Módulos disponibles (para usar cuando se implemente)
- `evaluation` v0.7.0 - Modelos de evaluación (Assessment, Question)
- `messaging/rabbit` v0.7.0 - Cliente RabbitMQ con DLQ y retry logic
- `database/mongodb` v0.7.0 - Helpers de MongoDB

Para más información, ver: `docs/isolated/START_HERE.md`

## Instalación

```bash
go mod download
go run cmd/main.go
```

## Eventos Procesados

| Evento | Cola | Prioridad | Procesador |
|--------|------|-----------|------------|
| `material.uploaded` | material_processing_high | 10 | Summary + Quiz Generator |
| `material.reprocess` | material_processing_medium | 5 | Reprocessor |
| `assessment.attempt_recorded` | material_processing_medium | 5 | Notifier |
| `material.deleted` | material_processing_low | 1 | Cleanup |
| `student.enrolled` | material_processing_low | 1 | Welcome |

## Configuración

Variables de entorno:
```env
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
MONGODB_URL=mongodb://localhost:27017/edugo
POSTGRES_URL=postgresql://user:pass@localhost:5432/edugo
S3_ENDPOINT=https://s3.amazonaws.com
OPENAI_API_KEY=sk-...
```

## Estado: Código base con lógica MOCK

Implementar para producción:
- Clientes reales de S3, MongoDB, PostgreSQL
- Integración con OpenAI API
- Reintentos con backoff exponencial
- Dead Letter Queue para errores
- Logging estructurado
- Métricas de procesamiento

## 🔧 Pre-commit Hooks

edugo-worker usa pre-commit hooks para validar código antes de commits.

### Instalación

```bash
# Instalar pre-commit
pip install pre-commit

# Instalar hooks en el repo
pre-commit install
```

### Hooks Configurados

1. **no-commit-to-branch** - Previene commits directos a main
2. **end-of-file-fixer** - Agrega newline al final de archivos
3. **trailing-whitespace** - Remueve espacios en blanco
4. **check-added-large-files** - Previene archivos >500KB
5. **check-yaml** - Valida sintaxis YAML
6. **detect-private-key** - Detecta credenciales expuestas
7. **check-merge-conflict** - Detecta conflictos sin resolver
8. **go-fmt** - Formatea código Go
9. **go-imports** - Organiza imports
10. **go-vet** - Análisis estático
11. **go-mod-tidy** - Verifica go.mod actualizado
12. **go-test** - Ejecuta tests (opcional, solo archivos .go)

### Uso

```bash
# Automático en cada commit
git commit -m "mensaje"

# Manual en todos los archivos
pre-commit run --all-files

# Manual en archivos staged
pre-commit run

# Saltar hooks (NO recomendado)
git commit --no-verify -m "mensaje"
```

## 🔄 Workflows CI/CD

| Workflow | Trigger | Propósito | Estado |
|----------|---------|-----------|--------|
| `ci.yml` | PR + Push main | Tests y validaciones | ✅ Activo |
| `test.yml` | Manual + PR | Coverage con threshold 33% | ✅ Activo |
| `manual-release.yml` | Manual | Release completo controlado | ✅ Activo |
| `sync-main-to-dev.yml` | Push a main | Sincronización automática | ✅ Activo |

**Workflows eliminados en Sprint 3:**
- ❌ `build-and-push.yml` - Consolidado en manual-release.yml
- ❌ `docker-only.yml` - Consolidado en manual-release.yml
- ❌ `release.yml` - Consolidado en manual-release.yml

## 🚀 Release Process

edugo-worker usa un proceso de release manual controlado.

### Quick Start

```bash
# Ejecutar release desde GitHub UI
https://github.com/EduGoGroup/edugo-worker/actions/workflows/manual-release.yml

# O desde CLI
gh workflow run manual-release.yml -f version=0.1.0 -f bump_type=minor
```

Ver [RELEASE-WORKFLOW.md](docs/RELEASE-WORKFLOW.md) para guía completa.

### Release Types

- **patch** (0.0.1 → 0.0.2): Bugfixes
- **minor** (0.0.1 → 0.1.0): Features
- **major** (0.0.1 → 1.0.0): Breaking changes

### ¿Qué hace manual-release.yml?

1. ✅ Valida versión semver
2. ✅ Actualiza version.txt
3. ✅ Genera entrada de CHANGELOG
4. ✅ Commit a main
5. ✅ Crea y pushea tag
6. ✅ Ejecuta tests completos
7. ✅ Build Docker multi-platform (linux/amd64 + linux/arm64)
8. ✅ Push Docker a GHCR
9. ✅ Crea GitHub Release

## 📊 Coverage Standards

**Threshold mínimo:** 33%

> ⚠️ **Nota:** Los comandos de coverage requieren que existan archivos `*_test.go` implementados.  
> Actualmente el proyecto no tiene tests unitarios. Implementa tests antes de ejecutar estos comandos.

```bash
# Generar reporte de coverage
go test -coverprofile=coverage/coverage.out -covermode=atomic ./...

# Ver coverage total
go tool cover -func=coverage/coverage.out | tail -1

# Generar reporte HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html
```

Ver [COVERAGE-STANDARDS.md](docs/COVERAGE-STANDARDS.md) para guía completa.
