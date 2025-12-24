# Guía de Testing - EduGo Worker

## Índice

1. [Introducción](#introducción)
2. [Arquitectura de Testing](#arquitectura-de-testing)
3. [Interfaces y Mocks](#interfaces-y-mocks)
4. [Testing Unitario](#testing-unitario)
5. [Testing de Integración](#testing-de-integración)
6. [Ejecutar Tests](#ejecutar-tests)
7. [Mejores Prácticas](#mejores-prácticas)

## Introducción

Este proyecto sigue principios de **Clean Architecture** y **Dependency Injection** para facilitar el testing. Usamos:

- **Interfaces**: Para abstraer dependencias externas
- **Mocks**: Generados automáticamente con [mockery](https://github.com/vektra/mockery)
- **go-sqlmock**: Para testear interacciones con SQL
- **testify**: Para assertions y mocking
- **Testcontainers**: Para tests de integración (futura implementación)

## Arquitectura de Testing

### Capas Testeables

```
internal/
├── application/
│   └── processor/          # 🧪 Tests unitarios con mocks
├── domain/
│   ├── repository/         # 📝 Interfaces (domain layer)
│   │   └── mocks/          # 🎭 Mocks generados automáticamente
│   └── service/            # 🧪 Tests unitarios de lógica de negocio
└── infrastructure/
    ├── storage/            # 📝 Interface + mocks
    ├── pdf/                # 📝 Interface + mocks
    ├── nlp/                # 📝 Interface + mocks
    └── persistence/
        └── mongodb/
            └── repository/ # 🔌 Tests de integración con MongoDB real
```

### Cobertura Objetivo

| Componente | Cobertura Objetivo | Estado Actual |
|------------|-------------------|---------------|
| **Processors** | >80% | 44.0% ✅ (+27.6%) |
| **Domain Services** | >90% | 0.0% ⏳ |
| **Repositories** | >70% | 0.0% ⏳ |
| **Infrastructure** | >60% | Variable |

## Interfaces y Mocks

### ¿Por qué Interfaces?

Las interfaces permiten:
- ✅ Inyección de dependencias
- ✅ Testing con mocks (sin dependencias reales)
- ✅ Cambio de implementaciones sin afectar el código
- ✅ Cumplir con el **Dependency Inversion Principle**

### Interfaces Disponibles

#### 1. Repositories (Domain Layer)

```go
// internal/domain/repository/material_summary_repository.go
type MaterialSummaryRepository interface {
    Create(ctx context.Context, summary *entities.MaterialSummary) error
    FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialSummary, error)
    FindByID(ctx context.Context, id primitive.ObjectID) (*entities.MaterialSummary, error)
    Update(ctx context.Context, summary *entities.MaterialSummary) error
    Delete(ctx context.Context, materialID string) error
    // ... más métodos
}
```

#### 2. Storage Client

```go
// internal/infrastructure/storage/interface.go
type Client interface {
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Upload(ctx context.Context, key string, content io.Reader) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    GetMetadata(ctx context.Context, key string) (*FileMetadata, error)
}
```

#### 3. PDF Extractor

```go
// internal/infrastructure/pdf/interface.go
type Extractor interface {
    Extract(ctx context.Context, reader io.Reader) (string, error)
    ExtractWithMetadata(ctx context.Context, reader io.Reader) (*ExtractionResult, error)
}
```

#### 4. NLP Client

```go
// internal/infrastructure/nlp/interface.go
type Client interface {
    GenerateSummary(ctx context.Context, text string) (*Summary, error)
    GenerateQuiz(ctx context.Context, text string, questionCount int) (*Quiz, error)
    HealthCheck(ctx context.Context) error
}
```

### Generar Mocks

Los mocks se generan automáticamente con **mockery**:

```bash
# Instalar mockery
go install github.com/vektra/mockery/v2@latest

# Generar todos los mocks (lee configuración de .mockery.yaml)
mockery
```

Configuración en `.mockery.yaml`:

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
  # ... más interfaces
```

## Testing Unitario

### Estructura de un Test

```go
func TestMaterialUploadedProcessor_Process_Success(t *testing.T) {
    // 1️⃣ Arrange - Preparar dependencias y datos
    db, dbMock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()
    
    storageClient := storageMocks.NewMockClient(t)
    pdfExtractor := pdfMocks.NewMockExtractor(t)
    nlpClient := nlpMocks.NewMockClient(t)
    
    // Configurar expectativas de mocks
    dbMock.ExpectExec("UPDATE materials").
        WithArgs("processing", "material-uuid").
        WillReturnResult(sqlmock.NewResult(0, 1))
    
    storageClient.EXPECT().
        Download(mock.Anything, "test.pdf").
        Return(mockReader, nil)
    
    pdfExtractor.EXPECT().
        Extract(mock.Anything, mock.Anything).
        Return("Extracted text", nil)
    
    // 2️⃣ Act - Ejecutar la función bajo test
    processor := NewMaterialUploadedProcessor(...)
    err = processor.Process(ctx, payload)
    
    // 3️⃣ Assert - Verificar resultados
    assert.NoError(t, err)
    assert.NoError(t, dbMock.ExpectationsWereMet())
}
```

### Ejemplo: Test de Processor con Mocks

```go
func TestMaterialUploadedProcessor_Process_PDFExtractionError(t *testing.T) {
    // Arrange
    db, dbMock, _ := sqlmock.New()
    defer db.Close()

    dbMock.ExpectExec("UPDATE materials SET processing_status").
        WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
        WillReturnResult(sqlmock.NewResult(0, 1))

    dbMock.ExpectExec("UPDATE materials SET processing_status").
        WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
        WillReturnResult(sqlmock.NewResult(0, 1))

    mockReader := io.NopCloser(bytes.NewReader([]byte("pdf content")))
    storageClient := storageMocks.NewMockClient(t)
    storageClient.EXPECT().
        Download(mock.Anything, "test.pdf").
        Return(mockReader, nil)

    pdfExtractor := pdfMocks.NewMockExtractor(t)
    pdfExtractor.EXPECT().
        Extract(mock.Anything, mock.Anything).
        Return("", errors.New("extraction failed"))

    processor := &MaterialUploadedProcessor{
        db:            db,
        logger:        newTestLogger(),
        storageClient: storageClient,
        pdfExtractor:  pdfExtractor,
    }

    event := dto.MaterialUploadedEvent{
        MaterialID: "550e8400-e29b-41d4-a716-446655440000",
        S3Key:      "test.pdf",
    }
    payload, _ := json.Marshal(event)

    // Act
    err := processor.Process(context.Background(), payload)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to extract PDF text")
    assert.NoError(t, dbMock.ExpectationsWereMet())
}
```

### Test Logger (Dummy)

Para tests unitarios, usa un logger que no hace nada:

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

## Testing de Integración

### Testcontainers (Próximamente)

Para tests de integración con MongoDB real:

```go
func TestMaterialSummaryRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Usar Testcontainers para levantar MongoDB
    ctx := context.Background()
    mongoC, err := testcontainers.GenericContainer(ctx, ...)
    require.NoError(t, err)
    defer mongoC.Terminate(ctx)
    
    // Conectar a MongoDB del contenedor
    mongoURI, _ := mongoC.Endpoint(ctx, "mongodb")
    client, _ := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
    db := client.Database("test")
    
    // Usar repositorio real
    repo := repository.NewMaterialSummaryRepository(db)
    
    // Test con base de datos real
    summary := &entities.MaterialSummary{...}
    err = repo.Create(ctx, summary)
    assert.NoError(t, err)
}
```

## Ejecutar Tests

### Tests Unitarios

```bash
# Ejecutar todos los tests
go test ./...

# Tests con cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Tests de un paquete específico
go test -v ./internal/application/processor/...

# Test específico
go test -v -run TestMaterialUploadedProcessor_Process_Success ./internal/application/processor/...
```

### Tests de Integración (cuando estén disponibles)

```bash
# Ejecutar solo tests cortos (unitarios)
go test -short ./...

# Ejecutar todos (unitarios + integración)
go test ./...
```

## Mejores Prácticas

### ✅ DO

1. **Usar interfaces para dependencias externas**
   ```go
   type Processor struct {
       nlpClient nlp.Client  // ✅ Interface
       logger    logger.Logger
   }
   ```

2. **Nombrar tests descriptivamente**
   ```go
   func TestProcessor_Process_WhenPDFExtractionFails_ShouldReturnError(t *testing.T)
   ```

3. **Seguir patrón AAA** (Arrange, Act, Assert)

4. **Limpiar recursos** (defer close, defer cleanup)
   ```go
   db, dbMock, _ := sqlmock.New()
   defer db.Close()
   ```

5. **Verificar que se cumplan las expectativas de mocks**
   ```go
   assert.NoError(t, dbMock.ExpectationsWereMet())
   ```

6. **Testear casos de error** (no solo happy path)

7. **Usar table-driven tests para múltiples casos**
   ```go
   tests := []struct {
       name string
       input string
       want error
   }{
       {"valid uuid", "550e8400-e29b-41d4-a716-446655440000", nil},
       {"invalid uuid", "invalid", ErrInvalidUUID},
   }
   ```

### ❌ DON'T

1. **NO usar implementaciones concretas en tests unitarios**
   ```go
   // ❌ Mal
   processor := &Processor{
       nlpClient: &openai.Client{}, // Implementación concreta
   }
   
   // ✅ Bien
   processor := &Processor{
       nlpClient: nlpMocks.NewMockClient(t),
   }
   ```

2. **NO hacer tests que dependan de orden de ejecución**

3. **NO ignorar errores en tests**
   ```go
   // ❌ Mal
   _ = repo.Create(ctx, entity)
   
   // ✅ Bien
   err := repo.Create(ctx, entity)
   require.NoError(t, err)
   ```

4. **NO hacer tests sin assertions**

5. **NO testear código de terceros** (solo tu lógica)

## Herramientas

- **mockery**: Generación automática de mocks
- **go-sqlmock**: Mocking de SQL
- **testify**: Assertions y mocking framework
- **Testcontainers**: Contenedores para tests de integración
- **go tool cover**: Análisis de cobertura

## Referencias

- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Mockery Documentation](https://vektra.github.io/mockery/)
- [go-sqlmock](https://github.com/DATA-DOG/go-sqlmock)
