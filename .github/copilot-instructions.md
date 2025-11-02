# GitHub Copilot - Instrucciones Personalizadas: EduGo Worker

## 🌍 IDIOMA / LANGUAGE

**IMPORTANTE**: Todos los comentarios, sugerencias, code reviews y respuestas en chat deben estar **SIEMPRE EN ESPAÑOL**.

- ✅ Comentarios en Pull Requests: **español**
- ✅ Sugerencias de código: **español**
- ✅ Explicaciones en chat: **español**
- ✅ Mensajes de error: **español**

---

## 🏗️ Arquitectura del Proyecto

Este proyecto implementa **Clean Architecture (Hexagonal)** con Go 1.25:

```
internal/
├── domain/              # Entidades, Value Objects, Interfaces
├── application/         # Servicios, DTOs, Casos de uso
│   └── processor/      # Processors de jobs y tareas asíncronas
├── infrastructure/      # Implementaciones concretas
│   └── persistence/    # Repositorios (PostgreSQL, MongoDB)
├── container/          # Inyección de Dependencias
└── config/             # Configuración con Viper
```

### Principios Arquitectónicos
- **Dependency Inversion**: El dominio NO depende de infraestructura
- **Separation of Concerns**: Cada capa tiene responsabilidades claras
- **Dependency Injection**: Usar container/container.go para DI
- **Interface Segregation**: Interfaces pequeñas y específicas

### Características Específicas de Worker
- **Procesamiento Asíncrono**: Jobs y tareas en background
- **Processors**: Lógica de procesamiento de tareas
- **Sin HTTP Handlers**: No es una API REST
- **Cron Jobs**: Tareas programadas (pendiente implementar)
- **Message Consumers**: RabbitMQ consumers (pendiente implementar)

---

## 📦 Dependencia Compartida: edugo-shared

Usamos el módulo `github.com/EduGoGroup/edugo-shared` para funcionalidad compartida:

### Paquetes Disponibles
- **logger**: Logger Zap estructurado (`edugo-shared/logger`)
- **common/errors**: Tipos de error de aplicación (`edugo-shared/common/errors`)

### ⚠️ REGLA CRÍTICA: NO Reimplementar Funcionalidad

```go
// ❌ INCORRECTO: Reimplementar funcionalidad existente
type MyLogger struct { ... }
func (l *MyLogger) Info(msg string) { ... }

// ✅ CORRECTO: Usar edugo-shared
import "github.com/EduGoGroup/edugo-shared/logger"
logger.Info(ctx, "mensaje de log", zap.String("key", "value"))
```

---

## 🎯 Convenciones de Código

### Naming Conventions

```go
// DTOs
type JobDTO struct { ... }               // ✅ Termina en DTO
type ReportJobDataDTO struct { ... }     // ✅ Termina en DTO

// Servicios
type ReportService struct { ... }        // ✅ Termina en Service
type EmailService struct { ... }         // ✅ Termina en Service

// Repositorios
type JobRepository interface { ... }     // ✅ Termina en Repository
type PostgresJobRepository struct { ... } // ✅ Implementación específica

// Processors (Específico de Worker)
type ReportProcessor struct { ... }      // ✅ Termina en Processor
type EmailProcessor struct { ... }       // ✅ Termina en Processor
```

### Manejo de Errores

```go
// ✅ CORRECTO: Usar tipos de error de edugo-shared
import "github.com/EduGoGroup/edugo-shared/common/errors"

func (p *ReportProcessor) Process(ctx context.Context, jobID string) error {
    job, err := p.repo.FindByID(ctx, jobID)
    if err != nil {
        if errors.IsNotFound(err) {
            return errors.NewNotFoundError("job", jobID)
        }
        return errors.NewInternalError("failed to get job", err)
    }
    return nil
}

// ❌ INCORRECTO: NO usar fmt.Errorf directamente
return fmt.Errorf("job not found: %s", jobID)

// ❌ INCORRECTO: NO usar errors.New
return errors.New("job not found")
```

### Context en Todas las Funciones

```go
// ✅ CORRECTO: Siempre recibir context.Context como primer parámetro
func (p *ReportProcessor) Process(ctx context.Context, jobID string) error
func (s *JobService) CreateJob(ctx context.Context, dto CreateJobDTO) (*JobDTO, error)
func (r *PostgresJobRepository) Save(ctx context.Context, job *domain.Job) error

// ❌ INCORRECTO: Métodos sin context
func (p *ReportProcessor) Process(jobID string) error
```

### Logging Estructurado

```go
// ✅ CORRECTO: Usar logger de edugo-shared con campos estructurados
import (
    "github.com/EduGoGroup/edugo-shared/logger"
    "go.uber.org/zap"
)

func (p *ReportProcessor) Process(ctx context.Context, jobID string) error {
    logger.Info(ctx, "processing job",
        zap.String("job_id", jobID),
        zap.String("processor", "report"),
    )

    // ... lógica ...

    if err != nil {
        logger.Error(ctx, "failed to process job",
            zap.Error(err),
            zap.String("job_id", jobID),
        )
        return err
    }

    logger.Info(ctx, "job processed successfully", zap.String("job_id", jobID))
    return nil
}

// ❌ INCORRECTO: NO usar log estándar
log.Println("job processed:", jobID)
log.Printf("error: %v", err)

// ❌ INCORRECTO: NO usar fmt.Println
fmt.Println("processing job...")
```

---

## 🔄 Patrones de Workers

### Processors

```go
// ✅ CORRECTO: Processor con retry logic
type ReportProcessor struct {
    repo   ReportRepository
    logger logger.Logger
}

func (p *ReportProcessor) Process(ctx context.Context, jobID string) error {
    logger.Info(ctx, "processing report job",
        zap.String("job_id", jobID),
    )

    // Lógica de procesamiento...

    return nil
}
```

### Manejo de Errores y Reintentos

```go
// ✅ CORRECTO: Retry logic con backoff
func (p *Processor) ProcessWithRetry(ctx context.Context, job Job) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err := p.Process(ctx, job)
        if err == nil {
            return nil
        }

        logger.Warn(ctx, "job failed, retrying",
            zap.Int("attempt", i+1),
            zap.Error(err),
        )

        time.Sleep(time.Second * time.Duration(i+1))
    }

    return errors.NewInternalError("max retries exceeded", nil)
}
```

---

## 🗄️ Bases de Datos

### PostgreSQL (Datos Relacionales)

```go
// ✅ Usar lib/pq para queries
type PostgresJobRepository struct {
    db *sql.DB
}

func (r *PostgresJobRepository) FindByID(ctx context.Context, id string) (*domain.Job, error) {
    var job domain.Job
    query := `SELECT id, type, status, data, created_at FROM jobs WHERE id = $1`
    err := r.db.QueryRowContext(ctx, query, id).Scan(&job.ID, &job.Type, &job.Status, &job.Data, &job.CreatedAt)
    if err == sql.ErrNoRows {
        return nil, errors.NewNotFoundError("job", id)
    }
    return &job, err
}
```

---

## ✅ Testing

### Principios de Testing

```go
// ✅ Tests de integración con testcontainers
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestJobRepository_Integration(t *testing.T) {
    // Setup: Levantar PostgreSQL container
    ctx := context.Background()
    container, err := postgres.RunContainer(ctx, ...)
    require.NoError(t, err)
    defer container.Terminate(ctx)

    // Test: Usar repositorio real
    repo := NewPostgresJobRepository(db)
    // ...

    // Cleanup: Automático con defer
}

// ✅ Tests unitarios con mocks para dependencias externas
type MockJobRepository struct {
    mock.Mock
}

// ✅ Tests deben ser independientes y ejecutarse en paralelo
func TestReportProcessor_Process(t *testing.T) {
    t.Parallel()  // ✅ Permite ejecución paralela
    // ...
}
```

### Cobertura de Tests

- **Objetivo**: >70% de cobertura
- **Prioridad**: Processors y repositorios

---

## 🛠️ Tecnologías y Stack

### Framework y Bibliotecas Core
- **Config Management**: Viper
- **Logging**: Zap (via edugo-shared)
- **Database Drivers**:
  - PostgreSQL: `lib/pq`

### Testing
- **Framework**: Testing estándar de Go
- **Containers**: Testcontainers
- **Mocking**: Testify/mock

### DevOps
- **Containerización**: Docker + Docker Compose
- **CI/CD**: GitHub Actions
- **Registry**: GitHub Container Registry (ghcr.io)

---

## 🌐 Variables de Entorno

### Variables Requeridas

```bash
# Base de datos
POSTGRES_PASSWORD=<contraseña>

# Ambiente
APP_ENV=local|dev|qa|prod
```

### NO Hardcodear Secrets

```go
// ❌ INCORRECTO: Secrets hardcodeados
const dbPassword = "postgres123"

// ✅ CORRECTO: Leer de variables de entorno
dbPassword := viper.GetString("database.password")
```

---

## 🎨 Estilo de Código

### Formato

```bash
# ✅ SIEMPRE formatear con gofmt antes de commit
gofmt -w .

# ✅ Verificar con linter
golangci-lint run
```

### Comentarios

```go
// ✅ CORRECTO: Comentarios en español, explicativos
// ProcessReport procesa un job de generación de reportes.
// Lee los datos del job, genera el reporte y actualiza el estado.
func (p *ReportProcessor) ProcessReport(ctx context.Context, jobID string) error

// ❌ INCORRECTO: Comentarios obvios o redundantes
// ProcessReport procesa un reporte
func (p *ReportProcessor) ProcessReport(...)
```

### Imports

```go
// ✅ CORRECTO: Agrupar imports
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // Third party
    "go.uber.org/zap"

    // Internal - edugo-shared
    "github.com/EduGoGroup/edugo-shared/logger"
    "github.com/EduGoGroup/edugo-shared/common/errors"

    // Internal - este proyecto
    "github.com/EduGoGroup/edugo-worker/internal/domain"
    "github.com/EduGoGroup/edugo-worker/internal/application"
)
```

---

## ⚡ Mejores Prácticas Adicionales

### 1. Inyección de Dependencias

```go
// ✅ CORRECTO: Constructor con dependencias explícitas
func NewReportProcessor(
    repo JobRepository,
    logger logger.Logger,
) *ReportProcessor {
    return &ReportProcessor{
        repo:   repo,
        logger: logger,
    }
}

// ❌ INCORRECTO: Dependencias globales o singleton
var globalDB *sql.DB  // ❌ Evitar
```

### 2. Validación de DTOs

```go
// ✅ CORRECTO: Usar validaciones explícitas
import "github.com/go-playground/validator/v10"

type CreateJobDTO struct {
    Type     string                 `json:"type" validate:"required"`
    Data     map[string]interface{} `json:"data" validate:"required"`
    Priority int                    `json:"priority" validate:"gte=0,lte=10"`
}

func (s *JobService) CreateJob(ctx context.Context, dto CreateJobDTO) (*JobDTO, error) {
    if err := validate.Struct(dto); err != nil {
        return nil, errors.NewValidationError("invalid job data", err)
    }
    // ...
}
```

### 3. Transacciones de Base de Datos

```go
// ✅ CORRECTO: Usar transacciones para operaciones múltiples
func (p *ReportProcessor) ProcessWithTransaction(ctx context.Context, jobID string) error {
    tx, err := p.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()  // Rollback automático si no hay commit

    // Operación 1: Actualizar estado del job
    err = p.jobRepo.UpdateStatusTx(ctx, tx, jobID, "processing")
    if err != nil {
        return err
    }

    // Operación 2: Guardar resultado
    err = p.resultRepo.SaveTx(ctx, tx, result)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

---

## 🎓 Recursos de Referencia

- **Workflows CI/CD**: [.github/workflows/README.md](workflows/README.md)
- **CHANGELOG**: [CHANGELOG.md](../CHANGELOG.md)

---

## 📝 Notas Finales para Copilot

### Al Revisar Pull Requests

1. ✅ Verificar que se usen tipos de error de `edugo-shared`
2. ✅ Confirmar que todos los métodos reciben `context.Context`
3. ✅ Validar que se use logging estructurado
4. ✅ Señalar TODOs o funcionalidad incompleta
5. ✅ Verificar que no se reimplemente funcionalidad de `edugo-shared`
6. ✅ Revisar retry logic en processors
7. ✅ Validar manejo de timeouts y cancellation

### Al Sugerir Código

1. ✅ Seguir Clean Architecture (no mezclar capas)
2. ✅ Usar dependencias de `edugo-shared` cuando corresponda
3. ✅ Incluir logging adecuado en processors
4. ✅ Manejar errores con tipos apropiados
5. ✅ Agregar validaciones necesarias
6. ✅ Escribir código testeable
7. ✅ Implementar retry logic cuando sea apropiado
8. ✅ Considerar timeouts y graceful shutdown

### Recordatorio de Idioma

🌍 **TODOS los comentarios, sugerencias y explicaciones deben estar en ESPAÑOL.**

---

**Última actualización**: 2025-11-01
**Versión del proyecto**: v0.1.0 (en desarrollo)
**Go Version**: 1.25.3
**edugo-shared Version**: Usar tags cuando estén disponibles
