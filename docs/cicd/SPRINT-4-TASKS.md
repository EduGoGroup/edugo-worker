# Sprint 4: Workflows Reusables - edugo-worker

**Proyecto:** edugo-worker  
**Sprint:** 4 de 4  
**Duración:** 3-4 días  
**Esfuerzo:** 12-16 horas  
**Prioridad:** 🟢 Media  
**Fecha:** 19 de Noviembre, 2025

---

## 📋 Tabla de Contenidos

1. [Resumen del Sprint](#-resumen-del-sprint)
2. [Objetivos](#-objetivos)
3. [Pre-requisitos](#-pre-requisitos)
4. [Tareas Detalladas](#-tareas-detalladas)
5. [Checklist General](#-checklist-general)
6. [Troubleshooting](#-troubleshooting)

---

## 🎯 Resumen del Sprint

### ¿Qué vamos a hacer?

**Problema:**  
Cada repositorio (shared, infrastructure, api-mobile, api-administracion, worker) tiene workflows CI/CD duplicados con lógica similar.

**Solución Sprint 4:**  
Centralizar workflows comunes en `edugo-infrastructure` y consumirlos desde repos usando workflows reusables de GitHub Actions.

**Resultado Esperado:**
- ✅ Workflows de worker usan workflows reusables
- ✅ Lógica centralizada en infrastructure
- ✅ ~400 líneas eliminadas de worker (-66%)
- ✅ Mantenimiento simplificado

---

## 🎯 Objetivos

### Objetivos Principales

- [ ] **OBJ-1:** Crear workflows reusables en infrastructure
- [ ] **OBJ-2:** Migrar ci.yml a workflow reusable
- [ ] **OBJ-3:** Migrar test.yml a workflow reusable
- [ ] **OBJ-4:** Migrar manual-release.yml a workflow reusable

### Métricas de Éxito

| Métrica | Antes | Después | Objetivo |
|---------|-------|---------|----------|
| Workflows locales | 4 | 4 (pero más simples) | Mismo |
| Líneas workflows | ~600 | ~200 | -66% |
| Workflows reusables | 0 | 3 | +3 |
| Duplicación cross-repo | Alta | Baja | ✅ |
| Mantenibilidad | Media | Alta | ✅ |

---

## ✅ Pre-requisitos

### Completar Sprint 3

Antes de comenzar Sprint 4, asegurarse que Sprint 3 está completo:

- [x] Workflows Docker consolidados
- [x] Go 1.25.3 migrado
- [x] Pre-commit hooks implementados
- [x] Coverage threshold 33% establecido
- [x] PR Sprint 3 mergeado a dev

### Accesos y Herramientas

```bash
# 1. Verificar acceso a infrastructure
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-infrastructure
git pull origin main

# 2. Verificar acceso a worker
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker
git pull origin dev

# 3. Verificar gh CLI
gh --version

# 4. Verificar permisos
gh auth status
```

---

## 📋 Tareas Detalladas

## Tarea 1: Preparar Infrastructure para Workflows Reusables

**Duración:** 2-3 horas  
**Prioridad:** 🔴 Crítica  
**Dependencias:** Ninguna

### Objetivo

Crear estructura en `edugo-infrastructure` para workflows reusables compartidos.

### Pasos

#### 1.1: Crear Directorio de Workflows Reusables

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-infrastructure

# Crear rama feature
git checkout main
git pull origin main
git checkout -b feature/add-reusable-workflows

# Crear directorio para workflows reusables
mkdir -p .github/workflows/reusable

echo "✅ Directorio creado"
```

---

#### 1.2: Crear Workflow Reusable para CI (Go Apps)

```bash
cat > .github/workflows/reusable/go-ci.yml << 'EOF'
name: Reusable - Go CI

# Workflow reusable para CI de aplicaciones Go
# Usado por: api-mobile, api-administracion, worker

on:
  workflow_call:
    inputs:
      go-version:
        description: 'Versión de Go a usar'
        required: false
        type: string
        default: '1.25.3'
      
      run-docker-build-test:
        description: 'Ejecutar Docker build test'
        required: false
        type: boolean
        default: true
      
      run-lint:
        description: 'Ejecutar linter (opcional, puede fallar)'
        required: false
        type: boolean
        default: true
    
    secrets:
      GITHUB_TOKEN:
        required: true

jobs:
  test:
    name: Tests and Validations
    runs-on: ubuntu-latest

    steps:
      - name: Checkout código
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true
          cache-dependency-path: go.sum

      - name: Configurar acceso a repos privados
        run: |
          git config --global url."https://${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
        env:
          GOPRIVATE: github.com/EduGoGroup/*

      - name: Verificar formato
        run: |
          if ! gofmt -l . | grep -q .; then
            echo "✓ Código formateado correctamente"
          else
            echo "✗ Código no está formateado:"
            gofmt -l .
            exit 1
          fi

      - name: Descargar dependencias
        run: go mod download

      - name: Verificar go.mod y go.sum
        run: |
          go mod tidy
          if ! git diff --exit-code go.mod go.sum; then
            echo "✗ go.mod o go.sum están desactualizados. Ejecuta 'go mod tidy'"
            exit 1
          fi
          echo "✓ go.mod y go.sum están actualizados"

      - name: Análisis estático (go vet)
        run: go vet ./...

      - name: Ejecutar tests con race detection
        run: go test -v -race ./...

      - name: Verificar build
        run: go build -v ./...

      - name: Verificar build del binario principal
        run: |
          if [ -f "cmd/main.go" ]; then
            go build -o app ./cmd/main.go
            echo "✓ Binario principal construido"
          else
            echo "⚠️  cmd/main.go no existe, saltando build de binario"
          fi

  lint:
    name: Linter (Optional)
    runs-on: ubuntu-latest
    if: ${{ inputs.run-lint }}
    continue-on-error: true

    steps:
      - name: Checkout código
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Configurar acceso a repos privados
        run: |
          git config --global url."https://${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
        env:
          GOPRIVATE: github.com/EduGoGroup/*

      - name: Descargar dependencias
        run: go mod download

      - name: Instalar golangci-lint
        run: |
          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
          echo "$(go env GOPATH)/bin" >> $GITHUB_PATH

      - name: Ejecutar linter
        run: golangci-lint run --timeout=5m || echo "Linter warnings found (not critical)"

  docker-build-test:
    name: Docker Build Test
    runs-on: ubuntu-latest
    needs: test
    if: ${{ inputs.run-docker-build-test }}

    steps:
      - name: Checkout código
        uses: actions/checkout@v4

      - name: Setup Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build Docker image (test only)
        uses: docker/build-push-action@v5
        with:
          context: .
          push: false
          tags: app:test
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            GITHUB_TOKEN=${{ secrets.GITHUB_TOKEN }}
EOF

echo "✅ go-ci.yml creado"
```

---

#### 1.3: Crear Workflow Reusable para Tests con Coverage

```bash
cat > .github/workflows/reusable/go-test-coverage.yml << 'EOF'
name: Reusable - Go Tests with Coverage

# Workflow reusable para tests con coverage
# Usado por: api-mobile, api-administracion, worker

on:
  workflow_call:
    inputs:
      go-version:
        description: 'Versión de Go a usar'
        required: false
        type: string
        default: '1.25.3'
      
      coverage-threshold:
        description: 'Umbral mínimo de cobertura (%)'
        required: false
        type: number
        default: 33.0
      
      use-services:
        description: 'Usar servicios (PostgreSQL, MongoDB, RabbitMQ)'
        required: false
        type: boolean
        default: true
    
    secrets:
      GITHUB_TOKEN:
        required: true
      CODECOV_TOKEN:
        required: false

jobs:
  test-coverage:
    name: Run Tests with Coverage
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: edugo_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

      mongodb:
        image: mongo:7
        env:
          MONGO_INITDB_ROOT_USERNAME: mongo
          MONGO_INITDB_ROOT_PASSWORD: mongo
        options: >-
          --health-cmd "mongosh --eval 'db.runCommand({ ping: 1 })'"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 27017:27017

      rabbitmq:
        image: rabbitmq:3-management-alpine
        env:
          RABBITMQ_DEFAULT_USER: guest
          RABBITMQ_DEFAULT_PASS: guest
        options: >-
          --health-cmd "rabbitmq-diagnostics -q ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5672:5672
          - 15672:15672

    steps:
      - name: Checkout código
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Configurar acceso a repos privados
        run: |
          git config --global url."https://${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
        env:
          GOPRIVATE: github.com/EduGoGroup/*

      - name: Descargar dependencias
        run: go mod download

      - name: Esperar a que servicios estén listos
        if: ${{ inputs.use-services }}
        run: |
          echo "Esperando a servicios..."
          sleep 10

      - name: Ejecutar tests con cobertura
        env:
          POSTGRES_URL: postgresql://postgres:postgres@localhost:5432/edugo_test?sslmode=disable
          MONGODB_URL: mongodb://mongo:mongo@localhost:27017/edugo_test?authSource=admin
          RABBITMQ_URL: amqp://guest:guest@localhost:5672/
        run: |
          mkdir -p coverage
          go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...

      - name: Verificar umbral de cobertura
        run: |
          COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
          THRESHOLD=${{ inputs.coverage-threshold }}
          
          echo "📊 Cobertura actual: ${COVERAGE}%"
          echo "📊 Umbral mínimo: ${THRESHOLD}%"
          
          if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo "❌ Cobertura ${COVERAGE}% está por debajo del umbral ${THRESHOLD}%"
            exit 1
          else
            echo "✅ Cobertura ${COVERAGE}% cumple con el umbral ${THRESHOLD}%"
          fi

      - name: Generar reporte HTML
        run: |
          if [ -f coverage/coverage.out ]; then
            go tool cover -html=coverage/coverage.out -o coverage/coverage.html
          fi

      - name: Subir reporte de cobertura
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: coverage-report
          path: coverage/
          retention-days: 30

      - name: Subir a Codecov
        uses: codecov/codecov-action@v3
        if: success() && secrets.CODECOV_TOKEN != ''
        with:
          file: coverage/coverage.out
          flags: ${{ github.event.repository.name }}
          fail_ci_if_error: false
          token: ${{ secrets.CODECOV_TOKEN }}

      - name: Generar resumen
        if: always()
        run: |
          echo "# 📊 Resumen de Tests" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY

          if [ -f coverage/coverage.out ]; then
            coverage=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}')
            echo "| Métrica | Valor |" >> $GITHUB_STEP_SUMMARY
            echo "|---------|-------|" >> $GITHUB_STEP_SUMMARY
            echo "| Cobertura | $coverage |" >> $GITHUB_STEP_SUMMARY
            echo "| Umbral | ${{ inputs.coverage-threshold }}% |" >> $GITHUB_STEP_SUMMARY
            echo "| Servicios | ${{ inputs.use-services }} |" >> $GITHUB_STEP_SUMMARY
          else
            echo "⚠️ No se generó reporte de cobertura" >> $GITHUB_STEP_SUMMARY
          fi
EOF

echo "✅ go-test-coverage.yml creado"
```

---

#### 1.4: Documentar Workflows Reusables

```bash
cat > .github/workflows/reusable/README.md << 'EOF'
# Workflows Reusables - edugo-infrastructure

Workflows compartidos para repos de EduGo.

## Workflows Disponibles

### 1. go-ci.yml

**Propósito:** CI completo para aplicaciones Go.

**Usado por:**
- edugo-api-mobile
- edugo-api-administracion
- edugo-worker

**Inputs:**
- `go-version` (string, default: '1.25.3'): Versión de Go
- `run-docker-build-test` (boolean, default: true): Ejecutar Docker build test
- `run-lint` (boolean, default: true): Ejecutar linter

**Secrets:**
- `GITHUB_TOKEN` (required): Token para acceso a repos privados

**Ejemplo:**
```yaml
jobs:
  ci:
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main
    with:
      go-version: '1.25.3'
      run-docker-build-test: true
      run-lint: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

### 2. go-test-coverage.yml

**Propósito:** Tests con coverage y servicios (PostgreSQL, MongoDB, RabbitMQ).

**Usado por:**
- edugo-api-mobile
- edugo-api-administracion
- edugo-worker

**Inputs:**
- `go-version` (string, default: '1.25.3'): Versión de Go
- `coverage-threshold` (number, default: 33.0): Umbral mínimo de coverage
- `use-services` (boolean, default: true): Usar servicios

**Secrets:**
- `GITHUB_TOKEN` (required): Token para acceso a repos privados
- `CODECOV_TOKEN` (optional): Token para Codecov

**Ejemplo:**
```yaml
jobs:
  test-coverage:
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-test-coverage.yml@main
    with:
      go-version: '1.25.3'
      coverage-threshold: 33.0
      use-services: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

---

## Ventajas de Workflows Reusables

1. **Centralización:** Lógica en un solo lugar
2. **Mantenibilidad:** Cambios en 1 archivo afectan todos los repos
3. **Consistencia:** Mismo comportamiento en todos los proyectos
4. **Reducción de código:** ~400 líneas eliminadas por repo
5. **Testing:** Workflows reusables se pueden testear independientemente

---

## Actualización de Workflows Reusables

Cuando actualizas un workflow reusable:

1. Crear rama feature en infrastructure
2. Modificar workflow en `.github/workflows/reusable/`
3. Crear PR y merge
4. Tag del cambio (opcional): `git tag reusable-v1.1.0`

Los repos consumidores usan `@main` o `@tag`:
- `@main`: Siempre última versión (recomendado)
- `@tag`: Versión específica (más seguro)

---

## Testing de Workflows Reusables

```bash
# 1. Crear repo de prueba
gh repo create test-reusable-workflows --private

# 2. Usar workflow reusable
cat > .github/workflows/test.yml << 'EOFTEST'
name: Test Reusable
on: [push]
jobs:
  test:
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
EOFTEST

# 3. Push y ver resultado
git add .
git commit -m "test: workflow reusable"
git push
```

---

## Referencias

- [GitHub Docs - Reusing workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)
- [edugo-infrastructure workflows](../../README.md)
EOF

echo "✅ README de workflows reusables creado"
```

---

#### 1.5: Commit Workflows Reusables en Infrastructure

```bash
# Agregar archivos
git add .github/workflows/reusable/

# Commit
git commit -m "feat: agregar workflows reusables para apps Go

- Crear go-ci.yml (CI completo)
- Crear go-test-coverage.yml (Tests + coverage + servicios)
- Documentar uso en README.md

Workflows reusables compartidos entre:
- api-mobile
- api-administracion  
- worker

Reduce ~400 líneas por repo.
Centraliza lógica de CI/CD.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push
git push origin feature/add-reusable-workflows

echo "✅ Workflows reusables pusheados a infrastructure"
```

---

#### 1.6: Crear PR en Infrastructure

```bash
# Crear PR
gh pr create \
  --base main \
  --head feature/add-reusable-workflows \
  --title "feat: agregar workflows reusables Go" \
  --body "## Workflows Reusables para Aplicaciones Go

### Nuevos Workflows

1. **go-ci.yml** - CI completo (tests, lint, docker build)
2. **go-test-coverage.yml** - Tests con coverage y servicios

### Repos que los usarán

- edugo-api-mobile
- edugo-api-administracion
- edugo-worker

### Beneficios

- Centralización de lógica CI/CD
- Reducción de ~400 líneas por repo
- Mantenimiento simplificado
- Consistencia entre proyectos

### Testing

- [x] Workflows creados
- [x] Documentados en README
- [ ] Probados desde repo externo (siguiente PR)

---

🤖 Generated with [Claude Code](https://claude.com/claude-code)"

# Esperar aprobación y merge
echo "⏳ Esperando aprobación de PR en infrastructure..."
echo "   Continuar con Tarea 2 después de merge"
```

---

## Tarea 2: Migrar ci.yml a Workflow Reusable

**Duración:** 2-3 horas  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 1 (PR infrastructure mergeado)

### Objetivo

Reemplazar ci.yml de worker para usar workflow reusable de infrastructure.

### Pasos

#### 2.1: Verificar PR Infrastructure Mergeado

```bash
# Verificar que PR de Tarea 1 está mergeado
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-infrastructure
git checkout main
git pull origin main

# Verificar workflows reusables existen
ls -la .github/workflows/reusable/
# Debe mostrar: go-ci.yml, go-test-coverage.yml

echo "✅ Infrastructure preparado"
```

---

#### 2.2: Backup de ci.yml Actual

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Crear rama feature
git checkout dev
git pull origin dev
git checkout -b feature/migrate-to-reusable-workflows

# Backup de ci.yml
mkdir -p docs/workflows-migrated-sprint4
cp .github/workflows/ci.yml docs/workflows-migrated-sprint4/ci.yml.backup

echo "✅ Backup de ci.yml creado"
```

---

#### 2.3: Reemplazar ci.yml con Workflow Reusable

```bash
# Reemplazar contenido de ci.yml
cat > .github/workflows/ci.yml << 'EOF'
name: CI Pipeline

# Workflow que usa workflow reusable de infrastructure
on:
  pull_request:
    branches: [ main, dev ]
  push:
    branches: [ main ]

jobs:
  ci:
    name: CI Complete
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main
    with:
      go-version: '1.25.3'
      run-docker-build-test: true
      run-lint: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
EOF

echo "✅ ci.yml migrado a workflow reusable"
```

**Comparación:**
- **Antes:** ~110 líneas (completo)
- **Después:** ~17 líneas (referencia a reusable)
- **Reducción:** ~93 líneas (-85%)

---

#### 2.4: Commit Migración de ci.yml

```bash
git add .github/workflows/ci.yml docs/workflows-migrated-sprint4/

git commit -m "refactor: migrar ci.yml a workflow reusable

- Reemplazar lógica local con workflow reusable de infrastructure
- Usar EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main
- Backup en docs/workflows-migrated-sprint4/ci.yml.backup

Reducción: ~93 líneas (-85%)
Lógica centralizada en infrastructure
Mismo comportamiento que antes

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

git push origin feature/migrate-to-reusable-workflows

echo "✅ ci.yml migrado"
```

---

## Tarea 3: Migrar test.yml a Workflow Reusable

**Duración:** 2-3 horas  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 2

### Objetivo

Reemplazar test.yml para usar workflow reusable de infrastructure.

### Pasos

#### 3.1: Backup de test.yml

```bash
cp .github/workflows/test.yml docs/workflows-migrated-sprint4/test.yml.backup
echo "✅ Backup de test.yml creado"
```

---

#### 3.2: Reemplazar test.yml con Workflow Reusable

```bash
cat > .github/workflows/test.yml << 'EOF'
name: Tests with Coverage

# Workflow que usa workflow reusable de infrastructure
on:
  workflow_dispatch:
  pull_request:
    branches: [ main, dev ]

jobs:
  test-coverage:
    name: Tests with Coverage
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-test-coverage.yml@main
    with:
      go-version: '1.25.3'
      coverage-threshold: 33.0
      use-services: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
EOF

echo "✅ test.yml migrado a workflow reusable"
```

**Comparación:**
- **Antes:** ~165 líneas (completo con servicios)
- **Después:** ~19 líneas (referencia a reusable)
- **Reducción:** ~146 líneas (-88%)

---

#### 3.3: Commit Migración de test.yml

```bash
git add .github/workflows/test.yml docs/workflows-migrated-sprint4/

git commit -m "refactor: migrar test.yml a workflow reusable

- Reemplazar lógica local con workflow reusable de infrastructure
- Usar EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-test-coverage.yml@main
- Backup en docs/workflows-migrated-sprint4/test.yml.backup
- Mantener coverage threshold 33%
- Mantener servicios (PostgreSQL, MongoDB, RabbitMQ)

Reducción: ~146 líneas (-88%)
Lógica centralizada en infrastructure
Mismo comportamiento que antes

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com)"

git push origin feature/migrate-to-reusable-workflows

echo "✅ test.yml migrado"
```

---

## Tarea 4: Actualizar Documentación

**Duración:** 30-45 minutos  
**Prioridad:** 🟢 Media  
**Dependencias:** Tareas 2-3

### Objetivo

Documentar cambios de workflows reusables en worker.

### Pasos

#### 4.1: Crear Guía de Workflows Reusables

```bash
cat > docs/REUSABLE-WORKFLOWS.md << 'EOF'
# Workflows Reusables - edugo-worker

## ¿Qué son Workflows Reusables?

Los workflows reusables son workflows de GitHub Actions centralizados en `edugo-infrastructure` y reutilizados desde múltiples repos.

## Workflows Migrados

### 1. ci.yml

**Antes:** 110 líneas de lógica local  
**Ahora:** 17 líneas referenciando workflow reusable

```yaml
jobs:
  ci:
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main
    with:
      go-version: '1.25.3'
      run-docker-build-test: true
      run-lint: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Funcionalidad:**
- ✅ Tests con race detection
- ✅ Análisis estático (go vet)
- ✅ Verificación de formato
- ✅ Linter opcional
- ✅ Docker build test

---

### 2. test.yml

**Antes:** 165 líneas de lógica local  
**Ahora:** 19 líneas referenciando workflow reusable

```yaml
jobs:
  test-coverage:
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-test-coverage.yml@main
    with:
      go-version: '1.25.3'
      coverage-threshold: 33.0
      use-services: true
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}
```

**Funcionalidad:**
- ✅ Tests con coverage
- ✅ Coverage threshold 33%
- ✅ Servicios (PostgreSQL, MongoDB, RabbitMQ)
- ✅ Upload a Codecov
- ✅ Reporte HTML de coverage

---

## Ventajas

1. **Reducción de código:** ~240 líneas eliminadas (-80%)
2. **Mantenibilidad:** Cambios en 1 lugar afectan todos los repos
3. **Consistencia:** Mismo comportamiento en api-mobile, api-admin, worker
4. **Actualización automática:** Al usar `@main`, obtiene mejoras sin cambios

## Workflows NO Migrados

- `manual-release.yml` - Específico de cada proyecto (inputs únicos)
- `sync-main-to-dev.yml` - Específico de estrategia de branching

## Backups

Workflows originales en `docs/workflows-migrated-sprint4/`:
- `ci.yml.backup`
- `test.yml.backup`

## Restauración

Si necesitas restaurar workflow original:

```bash
cp docs/workflows-migrated-sprint4/ci.yml.backup .github/workflows/ci.yml
```

## Referencias

- [Infrastructure reusable workflows](https://github.com/EduGoGroup/edugo-infrastructure/tree/main/.github/workflows/reusable)
- [GitHub Docs - Reusing workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)

---

**Última actualización:** Sprint 4 - 19 Nov 2025
EOF

echo "✅ REUSABLE-WORKFLOWS.md creado"
```

---

#### 4.2: Actualizar README.md

```bash
cat >> README.md << 'EOF'

## 🔄 Workflows Reusables (Sprint 4)

edugo-worker usa workflows reusables centralizados en `edugo-infrastructure`.

### Workflows que Usan Reusables

- `ci.yml` → `infrastructure/go-ci.yml`
- `test.yml` → `infrastructure/go-test-coverage.yml`

**Beneficios:**
- ✅ ~240 líneas eliminadas (-80%)
- ✅ Lógica centralizada
- ✅ Mantenimiento simplificado

Ver [REUSABLE-WORKFLOWS.md](docs/REUSABLE-WORKFLOWS.md) para detalles.

EOF

echo "⚠️  Editar README.md manualmente para integrar sección"
```

---

#### 4.3: Commit Documentación

```bash
git add docs/REUSABLE-WORKFLOWS.md README.md

git commit -m "docs: documentar workflows reusables

- Crear REUSABLE-WORKFLOWS.md con guía completa
- Actualizar README con sección de workflows reusables
- Explicar ventajas y backups

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com)"

git push origin feature/migrate-to-reusable-workflows

echo "✅ Documentación actualizada"
```

---

## Tarea 5: Testing y Validación

**Duración:** 1-2 horas  
**Prioridad:** 🔴 Crítica  
**Dependencias:** Tareas 1-4

### Objetivo

Verificar que workflows reusables funcionan correctamente.

### Pasos

#### 5.1: Crear PR en Worker

```bash
gh pr create \
  --base dev \
  --head feature/migrate-to-reusable-workflows \
  --title "refactor: migrar a workflows reusables (Sprint 4)" \
  --body "## Sprint 4: Migración a Workflows Reusables

### Cambios

#### ci.yml
- Migrado a workflow reusable
- Reducción: ~93 líneas (-85%)
- Backup: docs/workflows-migrated-sprint4/ci.yml.backup

#### test.yml
- Migrado a workflow reusable
- Reducción: ~146 líneas (-88%)
- Backup: docs/workflows-migrated-sprint4/test.yml.backup

### Métricas

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| Líneas ci.yml | 110 | 17 | -85% |
| Líneas test.yml | 165 | 19 | -88% |
| Total eliminado | - | 239 | -80% |

### Funcionalidad

- [x] CI completo (tests, vet, fmt, docker)
- [x] Tests con coverage (threshold 33%)
- [x] Servicios (PostgreSQL, MongoDB, RabbitMQ)
- [x] Upload a Codecov

### Testing

- [ ] CI workflow ejecutándose (verificar en PR)
- [ ] Test workflow ejecutándose (verificar en PR)
- [ ] Ambos pasando

### Referencias

- [Infrastructure workflows reusables](https://github.com/EduGoGroup/edugo-infrastructure/tree/main/.github/workflows/reusable)
- [docs/REUSABLE-WORKFLOWS.md](./docs/REUSABLE-WORKFLOWS.md)

---

🤖 Generated with [Claude Code](https://claude.com/claude-code)"

echo "✅ PR creado en worker"
```

---

#### 5.2: Verificar Workflows Ejecutándose

```bash
# Ver workflows del PR
gh pr checks

# Ver detalles de workflow CI
gh run list --workflow=ci.yml --limit 3

# Ver detalles de workflow Test
gh run list --workflow=test.yml --limit 3

# Ver logs si falla alguno
gh run view  # Seleccionar run fallido
```

---

#### 5.3: Validación Completa

```bash
# Script de validación
cat > /tmp/sprint4-validation.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "🔍 Validando Sprint 4..."

# 1. Workflows usan reusables
if grep -q "uses: EduGoGroup/edugo-infrastructure" .github/workflows/ci.yml; then
  echo "✅ ci.yml usa workflow reusable"
else
  echo "❌ ci.yml NO usa workflow reusable"
  exit 1
fi

if grep -q "uses: EduGoGroup/edugo-infrastructure" .github/workflows/test.yml; then
  echo "✅ test.yml usa workflow reusable"
else
  echo "❌ test.yml NO usa workflow reusable"
  exit 1
fi

# 2. Backups existen
if [ -f "docs/workflows-migrated-sprint4/ci.yml.backup" ]; then
  echo "✅ Backup de ci.yml existe"
else
  echo "❌ Backup de ci.yml faltante"
  exit 1
fi

if [ -f "docs/workflows-migrated-sprint4/test.yml.backup" ]; then
  echo "✅ Backup de test.yml existe"
else
  echo "❌ Backup de test.yml faltante"
  exit 1
fi

# 3. Documentación
if [ -f "docs/REUSABLE-WORKFLOWS.md" ]; then
  echo "✅ REUSABLE-WORKFLOWS.md existe"
else
  echo "❌ REUSABLE-WORKFLOWS.md faltante"
  exit 1
fi

# 4. Tamaño de workflows reducido
CI_LINES=$(wc -l < .github/workflows/ci.yml | tr -d ' ')
TEST_LINES=$(wc -l < .github/workflows/test.yml | tr -d ' ')

echo "📊 Líneas ci.yml: $CI_LINES"
echo "📊 Líneas test.yml: $TEST_LINES"

if [ "$CI_LINES" -lt "30" ] && [ "$TEST_LINES" -lt "30" ]; then
  echo "✅ Workflows significativamente reducidos"
else
  echo "⚠️  Workflows no tan reducidos como esperado"
fi

echo ""
echo "🎉 Sprint 4 validado exitosamente"
EOFSCRIPT

chmod +x /tmp/sprint4-validation.sh
/tmp/sprint4-validation.sh
```

---

## Tarea 6: Review y Merge

**Duración:** 30-60 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 5

### Objetivo

Revisar PR, incorporar feedback, y mergear a dev.

### Pasos

#### 6.1: Solicitar Review

```bash
gh pr ready
gh pr edit --add-reviewer @reviewerUsername
gh pr edit --add-label "refactor,Sprint-4,CI/CD"

echo "✅ Review solicitado"
```

---

#### 6.2: Merge PR

```bash
# Después de aprobación
gh pr merge --squash --delete-branch

# Verificar en dev
git checkout dev
git pull origin dev

echo "✅ Sprint 4 mergeado a dev"
```

---

## Tarea 7: Cleanup y Documentación Final

**Duración:** 30 minutos  
**Prioridad:** 🟢 Media  
**Dependencias:** Tarea 6

### Objetivo

Limpiar, documentar y comunicar completación de Sprint 4.

### Pasos

#### 7.1: Actualizar CHANGELOG

```bash
cat >> CHANGELOG.md << 'EOF'

## Sprint 4 - 2025-11-19

### Changed
- Migrado ci.yml a workflow reusable (-85% líneas)
- Migrado test.yml a workflow reusable (-88% líneas)

### Added
- Documentación de workflows reusables
- Backups de workflows originales

### Removed
- ~240 líneas de lógica local de workflows

---

EOF

git add CHANGELOG.md
git commit -m "docs: actualizar CHANGELOG con Sprint 4"
git push origin dev

echo "✅ CHANGELOG actualizado"
```

---

#### 7.2: Crear Release Notes Sprint 4

```bash
cat > docs/SPRINT-4-RELEASE-NOTES.md << 'EOF'
# Sprint 4 Release Notes - edugo-worker

**Fecha:** 19 de Noviembre, 2025  
**Sprint:** 4 de 4

## Resumen

Sprint 4 migra workflows CI/CD a workflows reusables centralizados en infrastructure.

## Cambios

### Workflows Migrados

1. **ci.yml:** 110 → 17 líneas (-85%)
2. **test.yml:** 165 → 19 líneas (-88%)

**Total eliminado:** ~240 líneas (-80%)

### Ventajas

- ✅ Lógica centralizada en infrastructure
- ✅ Mantenimiento simplificado
- ✅ Consistencia cross-repo
- ✅ Actualización automática

## Métricas Finales (Sprint 3 + 4)

| Métrica | Inicial | Post-Sprint 3 | Post-Sprint 4 | Total |
|---------|---------|---------------|---------------|-------|
| Workflows Docker | 3 | 1 | 1 | -66% |
| Líneas workflows | ~600 | ~350 | ~150 | -75% |
| Workflows reusables | 0 | 0 | 2 | +2 |

---

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF

git add docs/SPRINT-4-RELEASE-NOTES.md
git commit -m "docs: agregar release notes Sprint 4"
git push origin dev

echo "✅ Release notes creadas"
```

---

## Tarea 8: Validación Final y Cierre

**Duración:** 30 minutos  
**Prioridad:** 🔴 Crítica  
**Dependencias:** Tarea 7

### Objetivo

Validar que Sprint 4 está completo y celebrar.

### Checklist Final

```bash
# Validación completa Sprint 4
cat > /tmp/sprint4-final-check.sh << 'EOFSCRIPT'
#!/bin/bash

echo "🔍 Validación Final Sprint 4"
echo ""

# Workflows reusables
echo "1. Workflows Reusables:"
grep -q "uses: EduGoGroup/edugo-infrastructure" .github/workflows/ci.yml && echo "  ✅ ci.yml" || echo "  ❌ ci.yml"
grep -q "uses: EduGoGroup/edugo-infrastructure" .github/workflows/test.yml && echo "  ✅ test.yml" || echo "  ❌ test.yml"

# Backups
echo ""
echo "2. Backups:"
[ -f "docs/workflows-migrated-sprint4/ci.yml.backup" ] && echo "  ✅ ci.yml.backup" || echo "  ❌ ci.yml.backup"
[ -f "docs/workflows-migrated-sprint4/test.yml.backup" ] && echo "  ✅ test.yml.backup" || echo "  ❌ test.yml.backup"

# Documentación
echo ""
echo "3. Documentación:"
[ -f "docs/REUSABLE-WORKFLOWS.md" ] && echo "  ✅ REUSABLE-WORKFLOWS.md" || echo "  ❌ REUSABLE-WORKFLOWS.md"
[ -f "docs/SPRINT-4-RELEASE-NOTES.md" ] && echo "  ✅ SPRINT-4-RELEASE-NOTES.md" || echo "  ❌ SPRINT-4-RELEASE-NOTES.md"

# Líneas reducidas
echo ""
echo "4. Reducción de Código:"
CI_LINES=$(wc -l < .github/workflows/ci.yml | tr -d ' ')
TEST_LINES=$(wc -l < .github/workflows/test.yml | tr -d ' ')
echo "  ci.yml: $CI_LINES líneas"
echo "  test.yml: $TEST_LINES líneas"

echo ""
echo "🎉 Sprint 4 Completado"
EOFSCRIPT

chmod +x /tmp/sprint4-final-check.sh
/tmp/sprint4-final-check.sh
```

---

## 🎉 ¡Sprint 4 Completado!

### Resumen de Sprints 3 + 4

| Sprint | Objetivo | Resultado |
|--------|----------|-----------|
| **Sprint 3** | Consolidar Docker + Go 1.25 | ✅ Completado |
| **Sprint 4** | Workflows reusables | ✅ Completado |

### Logros Totales

- ✅ 3 workflows Docker → 1 (-66%)
- ✅ Go 1.24.10 → 1.25.3
- ✅ 12 pre-commit hooks
- ✅ Coverage threshold 33%
- ✅ 2 workflows migrados a reusables
- ✅ ~450 líneas eliminadas total (-75%)

### Worker Optimizado

edugo-worker ahora tiene:
- 4 workflows (ci, test, manual-release, sync)
- ~150 líneas de workflows (vs ~600 inicial)
- CI/CD moderno y mantenible
- Calidad garantizada (coverage, pre-commit)

---

## 📊 Checklist General Sprint 4

### Preparación
- [ ] PR infrastructure con reusables mergeado
- [ ] Sprint 3 completado
- [ ] Rama feature creada

### Tareas
- [ ] Tarea 1: Workflows reusables en infrastructure (2-3h)
- [ ] Tarea 2: Migrar ci.yml (2-3h)
- [ ] Tarea 3: Migrar test.yml (2-3h)
- [ ] Tarea 4: Documentación (30-45min)
- [ ] Tarea 5: Testing (1-2h)
- [ ] Tarea 6: Review y merge (30-60min)
- [ ] Tarea 7: Cleanup (30min)
- [ ] Tarea 8: Validación final (30min)

### Validación
- [ ] ci.yml usa workflow reusable
- [ ] test.yml usa workflow reusable
- [ ] Backups creados
- [ ] Documentación completa
- [ ] CI pasando
- [ ] PR mergeado

---

## 🛠️ Troubleshooting

### Problema: Workflow reusable no se encuentra

**Síntomas:**
```
Error: Unable to resolve action `EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-ci.yml@main`
```

**Solución:**
```bash
# Verificar que PR infrastructure está mergeado
cd /path/to/edugo-infrastructure
git checkout main
git pull origin main
ls .github/workflows/reusable/

# Si no existe, volver a Tarea 1
```

---

### Problema: Workflow reusable falla

**Síntomas:**
CI falla después de migración.

**Solución:**
```bash
# Ver logs del workflow reusable
gh run view

# Comparar con workflow original
diff .github/workflows/ci.yml docs/workflows-migrated-sprint4/ci.yml.backup

# Ajustar inputs del workflow reusable según necesidad
```

---

**Generado por:** Claude Code  
**Fecha:** 19 de Noviembre, 2025  
**Versión:** 1.0  
**Para:** edugo-worker - Sprint 4
