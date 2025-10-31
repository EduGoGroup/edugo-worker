# 🚀 GitHub Actions Workflows - EduGo Worker

Sistema de CI/CD completo para edugo-worker con integración continua, tests automatizados y despliegue de imágenes Docker.

---

## 📋 Workflows Disponibles

### 1. **CI Pipeline** (`ci.yml`)

Pipeline de integración continua que se ejecuta automáticamente en PRs y push a `main`.

**Triggers:**
- ✅ Pull Requests a `main` o `develop`
- ✅ Push directo a `main` (red de seguridad)

**Jobs:**
- **test**: Validaciones y tests principales
  - Verificación de formato con `gofmt`
  - Validación de `go.mod` y `go.sum`
  - Análisis estático con `go vet`
  - Tests con race detection
  - Build del proyecto y binario
- **lint**: Linter opcional (no falla el CI)
  - Ejecuta `golangci-lint`
  - Continúa aunque encuentre warnings
- **docker-build-test**: Prueba de construcción Docker
  - Valida que el Dockerfile funciona correctamente
  - No pushea la imagen (solo test)

**Configuración:**
```yaml
env:
  GO_VERSION: '1.25'
  GOPRIVATE: github.com/EduGoGroup/*
```

---

### 2. **Tests with Coverage** (`test.yml`)

Tests completos con servicios de infraestructura y reportes de cobertura.

**Triggers:**
- 🔄 Manual desde GitHub UI (`workflow_dispatch`)
- ✅ Pull Requests a `main` o `develop`

**Servicios de Infraestructura:**
- PostgreSQL 15
- MongoDB 7
- RabbitMQ 3

**Jobs:**
- **test-coverage**: Tests con cobertura
  - Inicia servicios Docker (postgres, mongo, rabbitmq)
  - Ejecuta tests con `-race` y `-coverprofile`
  - Genera reporte HTML de cobertura
  - Sube artefactos a GitHub Actions
  - Envía cobertura a Codecov
  - Genera resumen en GitHub
- **integration-tests**: Tests de integración (solo manual)
  - Ejecuta tests marcados con tag `integration`

**Variables de Entorno:**
```bash
POSTGRES_URL=postgresql://postgres:postgres@localhost:5432/edugo_test
MONGODB_URL=mongodb://mongo:mongo@localhost:27017/edugo_test
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
```

---

### 3. **Build and Push Docker Image** (`build-and-push.yml`)

Construcción y publicación de imágenes Docker en GitHub Container Registry.

**Triggers:**
- 🔄 Manual con selección de environment (`development`, `staging`, `production`)
- ✅ Push automático a `main`

**Registry:**
- `ghcr.io/edugogroup/edugo-worker`

**Tags Generados:**
- `latest` (solo en main)
- `main-<sha>` (commit SHA)
- `<environment>` (cuando es manual)

**Ejemplo de Uso:**
```bash
# Login a GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Pull imagen
docker pull ghcr.io/edugogroup/edugo-worker:latest
docker pull ghcr.io/edugogroup/edugo-worker:development
```

---

### 4. **Release CI/CD** (`release.yml`) ⭐

Pipeline completo para releases con creación automática de Docker images versionadas.

**Trigger:**
- 🏷️ **Creación de tags** con formato `v*` (ej: `v1.0.0`, `v1.2.3`, `v2.0.0`)

**Jobs:**
1. **validate-and-test**: Validación completa del código
   - Formato, análisis estático, tests
   - Cobertura de código
   - Build y verificación del binario

2. **build-and-push-docker**: Construcción de imagen Docker
   - Build multi-tag con versión semántica
   - Push a GitHub Container Registry
   - Tags automáticos:
     - `v1.2.3` (tag completo)
     - `1.2.3` (sin v)
     - `1.2` (major.minor)
     - `1` (major)
     - `latest`

3. **create-github-release**: Creación de GitHub Release
   - Extrae notas del CHANGELOG.md (si existe)
   - Genera changelog desde commits
   - Documenta cómo usar la imagen Docker
   - Incluye ejemplos de despliegue

**Ejemplo de Creación de Release:**

```bash
# 1. Crear tag localmente
git tag -a v1.0.0 -m "Release v1.0.0 - Primera versión estable"

# 2. Push del tag (esto trigger el workflow)
git push origin v1.0.0

# 3. El workflow automáticamente:
#    ✅ Ejecuta tests
#    ✅ Construye imagen Docker
#    ✅ Publica en ghcr.io con múltiples tags
#    ✅ Crea GitHub Release con notas
```

**Docker Images Generadas:**
```bash
# Todas estas imágenes apuntan a la misma build:
ghcr.io/edugogroup/edugo-worker:v1.0.0
ghcr.io/edugogroup/edugo-worker:1.0.0
ghcr.io/edugogroup/edugo-worker:1.0
ghcr.io/edugogroup/edugo-worker:1
ghcr.io/edugogroup/edugo-worker:latest
```

---

## 🎯 Flujos de Trabajo Recomendados

### Development Flow

```bash
# 1. Crear branch de feature
git checkout -b feature/nueva-funcionalidad

# 2. Hacer cambios y commits
git add .
git commit -m "feat: agregar nueva funcionalidad"

# 3. Push y crear PR
git push origin feature/nueva-funcionalidad
# Crear PR en GitHub → CI Pipeline se ejecuta automáticamente

# 4. Después de aprobar PR, merge a main
# → CI Pipeline se ejecuta en main
# → Build and Push crea imagen Docker con tag 'latest'
```

### Release Flow

```bash
# 1. Preparar release
# Actualizar CHANGELOG.md con notas de la versión
# Actualizar versiones en código si es necesario

# 2. Crear y pushear tag
git tag -a v1.2.0 -m "Release v1.2.0 - Mejoras de performance"
git push origin v1.2.0

# 3. El workflow release.yml se ejecuta automáticamente:
#    ✅ Valida y testea
#    ✅ Construye imagen Docker con tags versionados
#    ✅ Crea GitHub Release

# 4. Desplegar usando el tag específico
docker pull ghcr.io/edugogroup/edugo-worker:1.2.0
```

### Hotfix Flow

```bash
# 1. Crear tag de hotfix desde main
git checkout main
git pull origin main

# 2. Hacer fix crítico
git commit -m "fix: corregir bug crítico en procesamiento"

# 3. Crear tag de patch
git tag -a v1.2.1 -m "Hotfix v1.2.1 - Fix bug crítico"
git push origin main
git push origin v1.2.1

# → Release workflow construye y publica automáticamente
```

---

## 🔧 Configuración Requerida

### Secrets de GitHub

El proyecto usa `${{ secrets.GITHUB_TOKEN }}` que se proporciona automáticamente por GitHub Actions.

**Permisos Requeridos:**
- `contents: write` - Para crear releases
- `packages: write` - Para pushear a GHCR

### Variables de Entorno para Tests

En `test.yml` se configuran automáticamente:
```yaml
POSTGRES_URL: postgresql://postgres:postgres@localhost:5432/edugo_test
MONGODB_URL: mongodb://mongo:mongo@localhost:27017/edugo_test
RABBITMQ_URL: amqp://guest:guest@localhost:5672/
```

---

## 📊 Reportes y Artefactos

### Coverage Reports
- **Ubicación**: Artifacts de GitHub Actions
- **Formato**: HTML + TXT
- **Retención**: 30 días
- **Codecov**: Integración automática

### Docker Images
- **Registry**: `ghcr.io/edugogroup/edugo-worker`
- **Visibilidad**: Privada (requiere autenticación)
- **Cache**: GitHub Actions Cache para builds rápidos

---

## 🐛 Troubleshooting

### Error: "go.mod o go.sum desactualizados"
```bash
# Solución
go mod tidy
git add go.mod go.sum
git commit -m "chore: actualizar go.mod y go.sum"
```

### Error: "cannot access private repo edugo-shared"
- Verificar que `GOPRIVATE=github.com/EduGoGroup/*` está configurado
- El `GITHUB_TOKEN` tiene acceso al repo privado

### Error: "Docker build failed"
- Verificar que el `Dockerfile` tiene el ARG `GITHUB_TOKEN`
- Asegurarse de que todas las dependencias estén en `go.mod`

---

## 🔄 Actualizaciones

### Cambiar Versión de Go
Editar en cada workflow:
```yaml
env:
  GO_VERSION: '1.25'  # Cambiar aquí
```

### Agregar Nuevos Tests
Los nuevos tests se detectan automáticamente si siguen la convención:
- `*_test.go` para tests unitarios
- `test/integration/*_test.go` con build tag `integration`

### Modificar Tags de Docker
Editar `metadata-action` en `build-and-push.yml` y `release.yml`:
```yaml
tags: |
  type=semver,pattern={{version}}
  type=semver,pattern={{major}}.{{minor}}
  # Agregar más patrones aquí
```

---

## 📚 Referencias

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Build Push Action](https://github.com/docker/build-push-action)
- [Go Testing](https://golang.org/pkg/testing/)
- [Semantic Versioning](https://semver.org/)

---

**Última actualización:** 2025-10-31
**Versión:** 1.0.0
**Mantenedor:** EduGo Team
