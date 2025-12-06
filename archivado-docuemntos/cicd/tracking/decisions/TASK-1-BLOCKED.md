# Decisión: Tarea 1 Bloqueada - Infrastructure No Disponible

**Fecha:** 2025-11-22
**Tarea:** 1 - Preparar Infrastructure para Workflows Reusables
**Sprint:** SPRINT-4
**Fase:** FASE 1

---

## 🚫 Problema

El repositorio `edugo-infrastructure` no está disponible localmente en:
```
/home/user/edugo-infrastructure
```

La Tarea 1 requiere crear workflows reusables en este repositorio.

---

## 🎯 Contexto

**Objetivo de la Tarea 1:**
Crear workflows reusables en `edugo-infrastructure` para ser consumidos por:
- edugo-api-mobile
- edugo-api-administracion
- edugo-worker

**Workflows reusables a crear:**
1. `reusable-go-lint.yml` - Linter con golangci-lint v2.4.0
2. `reusable-go-test.yml` - Tests con coverage y servicios
3. `reusable-go-ci.yml` - CI completo

---

## ✅ Decisión: Usar STUB

**Implementación con STUB:**

En lugar de crear los workflows en infrastructure, voy a:

1. **Documentar** los workflows reusables que se crearían
2. **Crear archivos stub** en `docs/cicd/stubs/infrastructure-workflows/`
3. **Migrar workflows locales** en worker para que REFERENCIEN a estos stubs
4. **Marcar tarea como "✅ (stub)"**

---

## 📄 Workflows Reusables (STUB)

### 1. reusable-go-lint.yml

**Ubicación (cuando esté disponible):**
```
edugo-infrastructure/.github/workflows/reusable-go-lint.yml
```

**Contenido esperado:**
```yaml
name: Reusable - Go Lint

on:
  workflow_call:
    inputs:
      go-version:
        description: 'Go version'
        required: false
        type: string
        default: '1.25'
      args:
        description: 'golangci-lint args'
        required: false
        type: string
        default: '--timeout=5m'

jobs:
  lint:
    name: Run Linter
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Setup EduGo Go
        uses: EduGoGroup/edugo-infrastructure/.github/actions/setup-edugo-go@main
        with:
          github-token: ${{ github.token }}

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.4.0
          args: ${{ inputs.args }}
```

**Lecciones aplicadas:**
- ✅ NO subdirectorio (archivo en raíz)
- ✅ NO secret GITHUB_TOKEN (usa `github.token` directamente)
- ✅ golangci-lint-action@v7 (compatible con Go 1.25)
- ✅ Default golangci-lint v2.4.0

---

### 2. reusable-go-test.yml

**Ubicación (cuando esté disponible):**
```
edugo-infrastructure/.github/workflows/reusable-go-test.yml
```

**Contenido esperado:**
```yaml
name: Reusable - Go Tests with Coverage

on:
  workflow_call:
    inputs:
      go-version:
        description: 'Go version'
        required: false
        type: string
        default: '1.25'
      coverage-threshold:
        description: 'Coverage threshold'
        required: false
        type: number
        default: 33.0
      use-services:
        description: 'Use services (PostgreSQL, MongoDB, RabbitMQ)'
        required: false
        type: boolean
        default: true

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
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Setup EduGo Go
        uses: EduGoGroup/edugo-infrastructure/.github/actions/setup-edugo-go@main
        with:
          github-token: ${{ github.token }}

      - name: Wait for services
        if: ${{ inputs.use-services }}
        run: sleep 10

      - name: Run tests with coverage
        env:
          POSTGRES_URL: postgresql://postgres:postgres@localhost:5432/edugo_test?sslmode=disable
          MONGODB_URL: mongodb://mongo:mongo@localhost:27017/edugo_test?authSource=admin
          RABBITMQ_URL: amqp://guest:guest@localhost:5672/
        run: |
          mkdir -p coverage
          go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...

      - name: Verify coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
          THRESHOLD=${{ inputs.coverage-threshold }}

          echo "📊 Coverage: ${COVERAGE}%"
          echo "📊 Threshold: ${THRESHOLD}%"

          if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo "❌ Coverage ${COVERAGE}% below threshold ${THRESHOLD}%"
            exit 1
          else
            echo "✅ Coverage ${COVERAGE}% meets threshold ${THRESHOLD}%"
          fi

      - name: Upload coverage report
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: coverage-report
          path: coverage/
          retention-days: 30
```

---

## 🔄 Para FASE 2

**Cuando infrastructure esté disponible:**

1. Clonar o acceder a `edugo-infrastructure`
2. Crear branch feature: `feature/add-reusable-workflows`
3. Crear los 2-3 workflows reusables con el contenido documentado arriba
4. Crear PR en infrastructure
5. Mergear PR a main
6. Volver a edugo-worker y actualizar referencias de stub a reales

**Tiempo estimado FASE 2:** 1-2 horas

---

## 📊 Impacto

**Tareas afectadas:**
- Tarea 1: ✅ (stub) - Documentado
- Tarea 2: Puede continuar con referencias stub
- Tarea 3: Puede continuar con referencias stub
- Tarea 5: Requerirá infrastructure real para testing

**Estado:**
- ✅ FASE 1 puede continuar con stubs
- ⏳ FASE 2 requerirá acceso a infrastructure

---

## 🗂️ Archivos Creados

1. `docs/cicd/tracking/decisions/TASK-1-BLOCKED.md` (este archivo)
2. `docs/cicd/stubs/infrastructure-workflows/README.md` (siguiente paso)
3. `docs/cicd/stubs/infrastructure-workflows/reusable-go-lint.yml.stub`
4. `docs/cicd/stubs/infrastructure-workflows/reusable-go-test.yml.stub`

---

**Migaja actualizada en SPRINT-STATUS.md:**
- Tarea 1: ✅ (stub)
- Pendiente para FASE 2

---

**Generado por:** Claude Code
**Fecha:** 2025-11-22
**FASE:** 1 - Implementación con Stubs
