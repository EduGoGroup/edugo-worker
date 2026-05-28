# Testing - edugo-worker

Documentacion tecnica del framework de testing, patrones, mocks y guias para el worker de EduGo.

---

## 1. Framework de Testing

### Dependencias principales

| Herramienta | Version | Proposito |
|-------------|---------|-----------|
| `github.com/stretchr/testify` | -- | Assertions (`assert`, `require`) y mocks (`mock`) |
| `github.com/vektra/mockery` | v2.53.5 | Generacion automatica de mocks desde interfaces |
| `github.com/DATA-DOG/go-sqlmock` | -- | Mock de `database/sql` para PostgreSQL |
| `github.com/EduGoGroup/edugo-shared/testing/containers` | -- | Testcontainers para integracion (PostgreSQL, MongoDB, RabbitMQ) |
| `github.com/prometheus/client_golang` | -- | Verificacion de metricas Prometheus en tests |

### Comandos de ejecucion

```bash
# Todos los tests unitarios (con race detector)
make test
# Equivale a: go test -v -race ./...

# Tests con reporte de cobertura HTML
make test-coverage
# Equivale a:
# go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
# go tool cover -html=coverage/coverage.out -o coverage/coverage.html

# Solo tests unitarios (modo short)
make test-unit
# Equivale a: go test -v -short ./...

# Tests de integracion (requiere Docker)
make test-integration
# Equivale a: go test -v -tags=integration ./...

# Benchmarks
make benchmark
# Equivale a: go test -bench=. -benchmem ./...

# Linter
make lint
# Equivale a: golangci-lint run --timeout=5m

# Auditoria completa (verify + fmt + vet + tests)
make audit
```

---

## 2. Estructura de Tests

### Tests unitarios vs integracion

| Tipo | Ubicacion | Build tag | Dependencias externas |
|------|-----------|-----------|----------------------|
| Unitarios | `*_test.go` junto al codigo fuente | Ninguno | Solo mocks |
| Integracion | `test/integration/` y `*_integration_test.go` | `//go:build integration` | Docker (testcontainers) |

### Archivos de test por componente

```
internal/
  application/
    dto/event_dto_test.go
    processor/
      registry_test.go
      retry_test.go
      material_uploaded_processor_test.go
      student_enrolled_processor_test.go
  bootstrap/
    adapter/logger_test.go
    resource_builder_test.go
  client/auth_client_test.go
  config/config_test.go
  container/container_test.go
  infrastructure/
    factory_test.go
    circuitbreaker/circuit_breaker_test.go
    health/
      postgres_check_test.go
      rabbitmq_check_test.go
      mongodb_check_test.go
    http/
      health_handler_test.go
      metrics_server_test.go
    metrics/metrics_test.go
    nlp/
      client_with_cb_test.go
      fallback/client_test.go
      openai/client_test.go
      mocks/helpers_test.go
    pdf/
      extractor_test.go
      extractor_integration_test.go
      mocks/helpers_test.go
    ratelimiter/
      rate_limiter_test.go
      multi_rate_limiter_test.go
      integration_test.go
    shutdown/graceful_shutdown_test.go
    storage/
      client_with_cb_test.go
      s3/client_test.go
      mocks/helpers_test.go
    persistence/mongodb/repository/
      config_test.go
      material_event_repository_test.go
      material_summary_repository_test.go
test/
  integration/
    config.go            # Control de ejecucion (env vars)
    setup.go             # Setup de testcontainers
    setup_test.go        # Verificacion de containers
```

---

## 3. Interfaces Mockeadas

### Mocks generados automaticamente (mockery)

Configuracion en `.mockery.yaml`:

```yaml
with-expecter: true
dir: "{{.InterfaceDir}}/mocks"
mockname: "Mock{{.InterfaceName}}"
outpkg: mocks
filename: "mock_{{.InterfaceName | snakecase}}.go"
packages:
  github.com/EduGoGroup/edugo-worker/internal/domain/repository:
    interfaces:
      MaterialSummaryRepository:
      MaterialAssessmentRepository:
      MaterialEventRepository:
  github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage:
    interfaces:
      Client:
  github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf:
    interfaces:
      Extractor:
      Cleaner:
  github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp:
    interfaces:
      Client:
```

| Interfaz | Mock generado | Ubicacion |
|----------|--------------|-----------|
| `repository.MaterialSummaryRepository` | `MockMaterialSummaryRepository` | `internal/domain/repository/mocks/` |
| `repository.MaterialAssessmentRepository` | `MockMaterialAssessmentRepository` | `internal/domain/repository/mocks/` |
| `repository.MaterialEventRepository` | `MockMaterialEventRepository` | `internal/domain/repository/mocks/` |
| `storage.Client` | `MockClient` | `internal/infrastructure/storage/mocks/mock_client.go` |
| `pdf.Extractor` | `MockExtractor` | `internal/infrastructure/pdf/mocks/mock_extractor.go` |
| `pdf.Cleaner` | `MockCleaner` | `internal/infrastructure/pdf/mocks/mock_cleaner.go` |
| `nlp.Client` | `MockClient` | `internal/infrastructure/nlp/mocks/client.go` |

### Mocks manuales (en archivos de test)

| Mock | Archivo | Interfaz que implementa |
|------|---------|------------------------|
| `MockPostgresDB` | `health/postgres_check_test.go` | `health.PostgresDB` |
| `MockCleaner` | `pdf/extractor_test.go` | `pdf.Cleaner` |
| `MockLogger` | `pdf/extractor_test.go` | `logger.Logger` |
| `nopLogger` | `processor/registry_test.go` | `logger.Logger` |
| `mockProcessor` | `processor/registry_test.go` | `processor.Processor` |

---

## 4. Patrones de Testing Usados

### Patron 1: Table-Driven Tests

Usado extensivamente en rate limiter, cleaner y PDF extractor. Cada caso de test se define como struct en un slice.

Ejemplo real del rate limiter (`rate_limiter_test.go`):

```go
func TestNew(t *testing.T) {
    tests := []struct {
        name              string
        requestsPerSecond float64
        burstSize         float64
        expectedRate      float64
        expectedMaxTokens float64
    }{
        {
            name:              "valores normales",
            requestsPerSecond: 10,
            burstSize:         20,
            expectedRate:      10,
            expectedMaxTokens: 20,
        },
        {
            name:              "valores cero deben usar defaults",
            requestsPerSecond: 0,
            burstSize:         0,
            expectedRate:      1,
            expectedMaxTokens: 1,
        },
        {
            name:              "valores negativos deben usar defaults",
            requestsPerSecond: -5,
            burstSize:         -10,
            expectedRate:      1,
            expectedMaxTokens: 1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            rl := New(tt.requestsPerSecond, tt.burstSize)
            assert.Equal(t, tt.expectedRate, rl.refillRate)
            assert.Equal(t, tt.expectedMaxTokens, rl.maxTokens)
        })
    }
}
```

### Patron 2: Arrange-Act-Assert con Mocks (mockery + EXPECT)

Los mocks generados por mockery soportan `EXPECT()` para configurar expectativas con type safety.

Ejemplo real del processor test (`material_uploaded_processor_test.go`):

```go
func TestMaterialUploadedProcessor_Process_StorageDownloadError(t *testing.T) {
    // Arrange
    db, dbMock, err := sqlmock.New()
    assert.NoError(t, err)
    defer func() { _ = db.Close() }()

    // Mock para actualizar estado a processing
    dbMock.ExpectExec("UPDATE materials SET processing_status").
        WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
        WillReturnResult(sqlmock.NewResult(0, 1))

    // Mock para actualizar estado a failed (cuando falla el download)
    dbMock.ExpectExec("UPDATE materials SET processing_status").
        WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
        WillReturnResult(sqlmock.NewResult(0, 1))

    storageClient := storageMocks.NewMockClient(t)
    storageClient.EXPECT().
        Download(mock.Anything, "test.pdf").
        Return(nil, assert.AnError)

    processor := &MaterialUploadedProcessor{
        db:            db,
        logger:        newTestLogger(),
        storageClient: storageClient,
    }

    event := dto.MaterialUploadedEvent{
        EventID:      "evt-test-123",
        EventType:    "material.uploaded",
        EventVersion: "1.0.0",
        Timestamp:    time.Now(),
        Payload: dto.MaterialUploadedPayload{
            MaterialID:    "550e8400-e29b-41d4-a716-446655440000",
            SchoolID:      "school-test-789",
            TeacherID:     "teacher-test-101",
            FileURL:       "test.pdf",
            FileSizeBytes: 1024000,
            FileType:      "application/pdf",
        },
    }
    payload, _ := json.Marshal(event)

    // Act
    err = processor.Process(context.Background(), payload)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to download PDF")
    assert.NoError(t, dbMock.ExpectationsWereMet())
}
```

### Patron 3: Mock con interfaz propia (sin mockery)

Para interfaces simples, se crean mocks manuales directamente en el archivo de test.

Ejemplo real del health check (`postgres_check_test.go`):

```go
// MockPostgresDB es un mock simple de la interfaz PostgresDB
type MockPostgresDB struct {
    pingError error
    pingDelay time.Duration
    stats     sql.DBStats
}

func (m *MockPostgresDB) PingContext(ctx context.Context) error {
    if m.pingDelay > 0 {
        select {
        case <-time.After(m.pingDelay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return m.pingError
}

func (m *MockPostgresDB) Stats() sql.DBStats {
    return m.stats
}

func TestPostgreSQLCheck_Check_Success(t *testing.T) {
    mockDB := &MockPostgresDB{
        pingError: nil,
        stats: sql.DBStats{
            OpenConnections:    5,
            InUse:              2,
            Idle:               3,
            WaitCount:          0,
            MaxOpenConnections: 10,
        },
    }

    check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
    result := check.Check(context.Background())

    assert.Equal(t, "postgresql", result.Component)
    assert.Equal(t, StatusHealthy, result.Status)
    assert.Contains(t, result.Message, "healthy")
    assert.Equal(t, 5, result.Metadata["open_connections"])
}
```

### Patron 4: Helper Factories para mocks pre-configurados

Cada paquete de mocks incluye un archivo `helpers.go` con factories para escenarios comunes.

Ejemplo real de NLP mocks (`nlp/mocks/helpers.go`):

```go
// NewSuccessfulMockClient crea un mock NLP que siempre tiene exito
func NewSuccessfulMockClient(t *testing.T) *MockClient {
    mockClient := NewMockClient(t)
    mockClient.On("GenerateSummary", mock.Anything, mock.Anything).
        Return(CreateMockSummary(), nil).Maybe()
    mockClient.On("GenerateQuiz", mock.Anything, mock.Anything, mock.Anything).
        Return(CreateMockQuiz(5), nil).Maybe()
    mockClient.On("HealthCheck", mock.Anything).
        Return(nil).Maybe()
    return mockClient
}

// NewFailingMockClient crea un mock NLP que siempre falla
func NewFailingMockClient(t *testing.T, err error) *MockClient { ... }

// NewTimeoutMockClient crea un mock que simula timeout
func NewTimeoutMockClient(t *testing.T) *MockClient { ... }

// NewFlakeyMockClient falla N veces y luego tiene exito (para retry logic)
func NewFlakeyMockClient(t *testing.T, failCount int, failErr error) *MockClient { ... }
```

Factories similares existen para storage (`storage/mocks/helpers.go`) y PDF (`pdf/mocks/helpers.go`):

```go
// Storage helpers
storageMocks.NewSuccessfulMockClient(t)
storageMocks.NewFailingMockClient(t, err)
storageMocks.NewTimeoutMockClient(t)
storageMocks.NewNotFoundMockClient(t)
storageMocks.NewNetworkErrorMockClient(t)
storageMocks.NewFlakeyMockClient(t, failCount, failErr)
storageMocks.WithDownloadResponse(mockClient, key, content, err)

// PDF helpers
pdfMocks.NewSuccessfulMockExtractor(t)
pdfMocks.NewFailingMockExtractor(t, err)
pdfMocks.NewCorruptPDFMockExtractor(t)
pdfMocks.NewScannedPDFMockExtractor(t)
pdfMocks.NewTooLargePDFMockExtractor(t)
pdfMocks.NewFlakeyMockExtractor(t, failCount, failErr)
pdfMocks.WithCustomText(mockExtractor, text, pageCount)
```

### Patron 5: Tests de concurrencia

Se usan `sync.WaitGroup` y `sync.Mutex` para verificar correctitud bajo acceso concurrente.

Ejemplo real del rate limiter (`rate_limiter_test.go`):

```go
func TestConcurrency(t *testing.T) {
    rl := New(100, 100)

    var wg sync.WaitGroup
    successCount := 0
    var mu sync.Mutex

    numGoroutines := 200
    wg.Add(numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        go func() {
            defer wg.Done()
            if rl.Allow() {
                mu.Lock()
                successCount++
                mu.Unlock()
            }
        }()
    }

    wg.Wait()

    // Solo debe haber permitido exactamente 100
    assert.Equal(t, 100, successCount,
        "debe permitir exactamente el numero de tokens disponibles")
}
```

### Patron 6: Tests con circuit breaker y tiempos

Se usan constantes de timeout para evitar flakiness y se espera explicitamente la transicion entre estados.

Ejemplo real del circuit breaker (`circuit_breaker_test.go`):

```go
const (
    testCircuitBreakerTimeout = 100 * time.Millisecond
    testWaitForTimeout        = testCircuitBreakerTimeout + 50*time.Millisecond
)

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
    config := DefaultConfig("test")
    config.MaxFailures = 1
    config.Timeout = testCircuitBreakerTimeout
    cb := New(config)
    ctx := context.Background()

    // Abrir el circuit
    _ = cb.Execute(ctx, func(ctx context.Context) error {
        return errors.New("test error")
    })
    assert.Equal(t, StateOpen, cb.State())

    // Esperar a que pase el timeout
    time.Sleep(testWaitForTimeout)

    // Ejecutar despues del timeout
    err := cb.Execute(ctx, func(ctx context.Context) error {
        return nil
    })

    assert.NoError(t, err)
    assert.Equal(t, StateHalfOpen, cb.State())
}
```

### Patron 7: nopLogger para tests

Un logger que descarta toda la salida, evitando ruido en los tests:

```go
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Info(msg string, keysAndValues ...interface{})   {}
func (l *nopLogger) Warn(msg string, keysAndValues ...interface{})   {}
func (l *nopLogger) Error(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Fatal(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Sync() error                                     { return nil }
func (l *nopLogger) With(keysAndValues ...interface{}) logger.Logger { return l }

func newTestLogger() logger.Logger {
    return &nopLogger{}
}
```

---

## 5. Tests de Integracion con Testcontainers

### Ubicacion

`test/integration/` con build tag `//go:build integration`.

### Control de ejecucion

Archivo: `test/integration/config.go`

Los tests de integracion **NO se ejecutan por defecto** localmente. Se habilitan con variables de entorno:

| Variable | Valor | Efecto |
|----------|-------|--------|
| `RUN_INTEGRATION_TESTS` | `true` | Habilita tests de integracion |
| `INTEGRATION_TESTS` | `1` o `true` | Alternativa para habilitar |
| `CI` | `true` | En CI se ejecutan por defecto |
| `SKIP_INTEGRATION_TESTS` | `true` | Deshabilita en CI |

Uso en cada test:

```go
func TestSomething(t *testing.T) {
    SkipIfIntegrationTestsDisabled(t)
    // ... resto del test
}
```

### Setup de containers

Archivo: `test/integration/setup.go`

Usa `github.com/EduGoGroup/edugo-shared/testing/containers` (wrapper propio sobre testcontainers).

```go
// Todos los containers
func setupAllContainers(t *testing.T) (*containers.Manager, func()) {
    cfg := containers.NewConfig().
        WithPostgreSQL(&containers.PostgresConfig{
            Database: "edugo",
            Username: "edugo_user",
            Password: "edugo_pass",
        }).
        WithMongoDB(&containers.MongoConfig{
            Database: "edugo",
            Username: "edugo_admin",
            Password: "edugo_pass",
        }).
        WithRabbitMQ(&containers.RabbitConfig{
            Username: "edugo_user",
            Password: "edugo_pass",
        }).
        Build()

    manager, err := containers.GetManager(t, cfg)
    // ...
}

// Solo PostgreSQL
func setupPostgres(t *testing.T) (*sql.DB, func()) { ... }

// Solo MongoDB (con migraciones aplicadas)
func setupMongoDB(t *testing.T) (*mongo.Database, func()) { ... }

// Solo RabbitMQ
func setupRabbitMQ(t *testing.T) (*amqp.Channel, func()) { ... }
```

### Tests de verificacion

El archivo `setup_test.go` verifica que cada container se inicialice correctamente:

```go
func TestSetupPostgres(t *testing.T) {
    SkipIfIntegrationTestsDisabled(t)
    db, cleanup := setupPostgres(t)
    defer cleanup()

    var result int
    err := db.QueryRow("SELECT 1").Scan(&result)
    if err != nil {
        t.Fatalf("Failed to query Postgres: %v", err)
    }
}

func TestSetupMongoDB(t *testing.T) {
    SkipIfIntegrationTestsDisabled(t)
    db, cleanup := setupMongoDB(t)
    defer cleanup()

    ctx := context.Background()
    collections, err := db.ListCollectionNames(ctx, map[string]interface{}{})
    // ...
}

func TestSetupRabbitMQ(t *testing.T) {
    SkipIfIntegrationTestsDisabled(t)
    channel, cleanup := setupRabbitMQ(t)
    defer cleanup()

    _, err := channel.QueueDeclare("test_queue", false, true, false, false, nil)
    // ...
}
```

---

## 6. Como Ejecutar Tests

### Comandos principales

```bash
# --- Tests unitarios ---

# Todos los tests (recomendado)
make test

# Solo tests unitarios rapidos (modo short)
make test-unit

# Tests de un paquete especifico
go test -v ./internal/infrastructure/circuitbreaker/...
go test -v ./internal/infrastructure/ratelimiter/...
go test -v ./internal/infrastructure/pdf/...
go test -v ./internal/application/processor/...

# Un test especifico
go test -v -run TestCircuitBreaker_OpensAfterMaxFailures ./internal/infrastructure/circuitbreaker/

# --- Tests de integracion ---

# Requiere Docker corriendo
RUN_INTEGRATION_TESTS=true make test-integration

# --- Cobertura ---

make test-coverage
# Genera: coverage/coverage.html
# Abrir: open coverage/coverage.html

# --- Benchmarks ---

make benchmark

# Benchmark especifico
go test -bench=BenchmarkAllow -benchmem ./internal/infrastructure/ratelimiter/

# --- Linter ---

make lint

# --- Auditoria completa ---

make audit
```

### Variables de entorno para tests

```bash
export OPENAI_API_KEY=sk-test-key    # Necesaria para tests del OpenAI client
export APP_ENV=local                  # Ambiente (default en Makefile)
```

---

## 7. Cobertura por Componente

La cobertura se genera con `make test-coverage` y se guarda en `coverage/coverage.out`. Para ver la cobertura actual:

```bash
go test -coverprofile=coverage/coverage.out ./...
go tool cover -func=coverage/coverage.out
```

### Componentes con tests existentes

| Componente | Archivos de test | Tipos de test |
|-----------|------------------|---------------|
| Circuit Breaker | `circuit_breaker_test.go` | Unitarios (estados, transiciones, limites) |
| Rate Limiter | `rate_limiter_test.go`, `multi_rate_limiter_test.go`, `integration_test.go` | Unitarios + integracion con metricas |
| Health Checks | `postgres_check_test.go`, `mongodb_check_test.go`, `rabbitmq_check_test.go` | Unitarios con mocks de BD |
| Health Handler | `health_handler_test.go` | Unitarios HTTP |
| Metrics Server | `metrics_server_test.go` | Unitarios HTTP |
| Metrics | `metrics_test.go` | Unitarios (registro de metricas) |
| PDF Extractor | `extractor_test.go`, `extractor_integration_test.go` | Unitarios + integracion con PDFs reales |
| NLP OpenAI | `openai/client_test.go` | Unitarios (validacion, prompts) |
| NLP Fallback | `fallback/client_test.go` | Unitarios (generacion) |
| NLP CB Wrapper | `client_with_cb_test.go` | Unitarios (integracion con CB) |
| Storage S3 | `s3/client_test.go` | Unitarios (validaciones) |
| Storage CB Wrapper | `client_with_cb_test.go` | Unitarios (integracion con CB) |
| Processor Registry | `registry_test.go` | Unitarios (routing, errores) |
| MaterialUploaded Processor | `material_uploaded_processor_test.go` | Unitarios con sqlmock + mockery mocks |
| MongoDB Repositories | `material_event_repository_test.go`, `material_summary_repository_test.go` | Unitarios con MongoDB |
| Graceful Shutdown | `graceful_shutdown_test.go` | Unitarios |

---

## 8. Como Escribir Tests para un Nuevo Processor

Guia paso a paso para agregar tests a un nuevo processor de eventos.

### Paso 1: Crear el processor

```go
// internal/application/processor/my_event_processor.go
package processor

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    "github.com/EduGoGroup/edugo-shared/logger"
    "github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
    "github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
)

type MyEventProcessor struct {
    db            *sql.DB
    logger        logger.Logger
    storageClient storage.Client
    nlpClient     nlp.Client
}

type MyEventProcessorConfig struct {
    DB            *sql.DB
    Logger        logger.Logger
    StorageClient storage.Client
    NLPClient     nlp.Client
}

func NewMyEventProcessor(cfg MyEventProcessorConfig) *MyEventProcessor {
    return &MyEventProcessor{
        db:            cfg.DB,
        logger:        cfg.Logger,
        storageClient: cfg.StorageClient,
        nlpClient:     cfg.NLPClient,
    }
}

func (p *MyEventProcessor) EventType() string {
    return "my_event"
}

func (p *MyEventProcessor) Process(ctx context.Context, payload []byte) error {
    // 1. Deserializar payload
    // 2. Validar datos
    // 3. Descargar archivo de S3
    // 4. Procesar con NLP
    // 5. Guardar resultados en BD
    return nil
}
```

### Paso 2: Crear el archivo de test

```go
// internal/application/processor/my_event_processor_test.go
package processor

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    nlpMocks "github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp/mocks"
    storageMocks "github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage/mocks"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)
```

### Paso 3: Test del EventType

```go
func TestMyEventProcessor_EventType(t *testing.T) {
    processor := &MyEventProcessor{}
    assert.Equal(t, "my_event", processor.EventType())
}
```

### Paso 4: Test de JSON invalido

```go
func TestMyEventProcessor_Process_InvalidJSON(t *testing.T) {
    // Arrange
    db, _, err := sqlmock.New()
    assert.NoError(t, err)
    defer func() { _ = db.Close() }()

    processor := &MyEventProcessor{
        db:     db,
        logger: newTestLogger(), // nopLogger definido en registry_test.go
    }

    // Act
    err = processor.Process(context.Background(), []byte("invalid json {"))

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid")
}
```

### Paso 5: Test de error en dependencia externa (storage)

```go
func TestMyEventProcessor_Process_StorageError(t *testing.T) {
    // Arrange
    db, dbMock, err := sqlmock.New()
    assert.NoError(t, err)
    defer func() { _ = db.Close() }()

    // Configurar expectativas de SQL
    dbMock.ExpectExec("UPDATE my_table SET status").
        WithArgs("processing", "my-id-123").
        WillReturnResult(sqlmock.NewResult(0, 1))
    dbMock.ExpectExec("UPDATE my_table SET status").
        WithArgs("failed", "my-id-123").
        WillReturnResult(sqlmock.NewResult(0, 1))

    // Mock de storage que falla
    storageClient := storageMocks.NewMockClient(t)
    storageClient.EXPECT().
        Download(mock.Anything, "archivo.pdf").
        Return(nil, assert.AnError)

    processor := &MyEventProcessor{
        db:            db,
        logger:        newTestLogger(),
        storageClient: storageClient,
    }

    payload := createTestPayload(t, "my-id-123", "archivo.pdf")

    // Act
    err = processor.Process(context.Background(), payload)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "download")
    assert.NoError(t, dbMock.ExpectationsWereMet())
}
```

### Paso 6: Test de exito completo con multiples mocks

```go
func TestMyEventProcessor_Process_Success(t *testing.T) {
    // Arrange
    db, dbMock, err := sqlmock.New()
    assert.NoError(t, err)
    defer func() { _ = db.Close() }()

    dbMock.ExpectExec("UPDATE my_table SET status").
        WithArgs("processing", "my-id-123").
        WillReturnResult(sqlmock.NewResult(0, 1))
    dbMock.ExpectExec("UPDATE my_table SET status").
        WithArgs("completed", "my-id-123").
        WillReturnResult(sqlmock.NewResult(0, 1))

    // Mock storage exitoso
    mockReader := io.NopCloser(bytes.NewReader([]byte("pdf content")))
    storageClient := storageMocks.NewMockClient(t)
    storageClient.EXPECT().
        Download(mock.Anything, "archivo.pdf").
        Return(mockReader, nil)

    // Mock NLP exitoso
    nlpClient := nlpMocks.NewMockClient(t)
    nlpClient.EXPECT().
        GenerateSummary(mock.Anything, mock.Anything).
        Return(nlpMocks.CreateMockSummary(), nil)

    processor := &MyEventProcessor{
        db:            db,
        logger:        newTestLogger(),
        storageClient: storageClient,
        nlpClient:     nlpClient,
    }

    payload := createTestPayload(t, "my-id-123", "archivo.pdf")

    // Act
    err = processor.Process(context.Background(), payload)

    // Assert
    assert.NoError(t, err)
    assert.NoError(t, dbMock.ExpectationsWereMet())
}

// Helper para crear payloads de test
func createTestPayload(t *testing.T, id, fileURL string) []byte {
    t.Helper()
    event := map[string]interface{}{
        "event_type":    "my_event",
        "event_id":      "evt-test-" + id,
        "event_version": "1.0.0",
        "timestamp":     time.Now(),
        "payload": map[string]interface{}{
            "id":       id,
            "file_url": fileURL,
        },
    }
    data, err := json.Marshal(event)
    assert.NoError(t, err)
    return data
}
```

### Paso 7: Registrar en el Registry y agregar test de integracion

```go
func TestMyEventProcessor_RegisteredInRegistry(t *testing.T) {
    logger := newTestLogger()
    registry := NewRegistry(logger)

    processor := &MyEventProcessor{logger: logger}
    registry.Register(processor)

    assert.Equal(t, 1, registry.Count())

    types := registry.RegisteredTypes()
    assert.Contains(t, types, "my_event")
}
```

### Paso 8: Usar helpers pre-construidos para escenarios comunes

```go
func TestMyEventProcessor_Process_WithPrebuiltMocks(t *testing.T) {
    // Usar factories de mocks para escenarios rapidos
    storageClient := storageMocks.NewSuccessfulMockClient(t)    // Siempre exito
    nlpClient := nlpMocks.NewSuccessfulMockClient(t)            // Siempre exito

    // O para escenarios de error:
    // storageClient := storageMocks.NewTimeoutMockClient(t)     // Simula timeout
    // storageClient := storageMocks.NewNotFoundMockClient(t)    // Simula not found
    // nlpClient := nlpMocks.NewFlakeyMockClient(t, 2, someErr)  // Falla 2 veces, luego exito

    // ... usar en el processor
}
```

### Checklist para tests de un nuevo processor

- [ ] Test de `EventType()` retorna el string correcto
- [ ] Test de JSON invalido en payload
- [ ] Test de validacion de campos obligatorios
- [ ] Test de error en cada dependencia externa (storage, NLP, BD)
- [ ] Test de exito completo (happy path)
- [ ] Test de que `sqlmock.ExpectationsWereMet()` pasa en cada test
- [ ] Test de cancelacion de contexto (si el proceso es largo)
- [ ] Test de que el processor se registra correctamente en el Registry
- [ ] Usar `newTestLogger()` para evitar ruido en la salida
- [ ] Usar `t.Helper()` en funciones helper
- [ ] Usar `-race` flag (incluido en `make test`)
