# Propuestas de Refactorización

> **Propósito:** Documentar mejoras de código que mejorarían la mantenibilidad, legibilidad y rendimiento.  
> **Última revisión:** Diciembre 2024

---

## 📋 Resumen de Refactorizaciones

| ID | Área | Descripción | Esfuerzo | Impacto |
|----|------|-------------|----------|---------|
| RF-001 | Bootstrap | Simplificar patrón de factories | Alto | Alto |
| RF-002 | Processors | Implementar patrón Registry/Factory | Medio | Alto |
| RF-003 | Repositories | Extraer interfaz común | Medio | Medio |
| RF-004 | Validators | Unificar validadores duplicados | Bajo | Medio |
| RF-005 | Config | Implementar configuración type-safe | Medio | Alto |
| RF-006 | Logging | Estandarizar uso de logger | Bajo | Medio |
| RF-007 | Error Handling | Implementar error types consistentes | Medio | Alto |
| RF-008 | Testing | Agregar test doubles y mocks | Alto | Alto |

---

## RF-001: Simplificar Patrón de Factories

### Estado Actual

El código actual usa un patrón complejo con doble puntero para retener referencias:

```go
// internal/bootstrap/custom_factories.go

type customFactoriesWrapper struct {
    sqlDB         *sql.DB
    mongoClient   *mongo.Client
    rabbitChannel *amqp.Channel
}

type customPostgreSQLFactory struct {
    shared bootstrap.PostgreSQLFactory
    sqlDB  **sql.DB  // Doble puntero - confuso
}

func (f *customPostgreSQLFactory) CreateRawConnection(ctx context.Context, config bootstrap.PostgreSQLConfig) (*sql.DB, error) {
    db, err := f.shared.CreateRawConnection(ctx, config)
    if err != nil {
        return nil, err
    }
    *f.sqlDB = db  // Asignación indirecta
    return db, nil
}
```

### Problemas
1. **Complejidad innecesaria** - El doble puntero es difícil de entender
2. **Difícil de testear** - Muchas dependencias implícitas
3. **Acoplamiento alto** - Todo está conectado a través del wrapper

### Propuesta de Refactorización

```go
// internal/bootstrap/bootstrap.go - NUEVO DISEÑO

// ResourceBuilder construye recursos de forma fluida
type ResourceBuilder struct {
    cfg    *config.Config
    ctx    context.Context
    errors []error
}

// NewResourceBuilder crea un nuevo builder
func NewResourceBuilder(ctx context.Context, cfg *config.Config) *ResourceBuilder {
    return &ResourceBuilder{ctx: ctx, cfg: cfg}
}

// Build construye todos los recursos
func (b *ResourceBuilder) Build() (*Resources, func() error, error) {
    resources := &Resources{}
    cleanups := make([]func() error, 0)
    
    // PostgreSQL
    db, cleanup, err := b.buildPostgreSQL()
    if err != nil {
        return nil, nil, fmt.Errorf("postgresql: %w", err)
    }
    resources.PostgreSQL = db
    cleanups = append(cleanups, cleanup)
    
    // MongoDB
    mongo, cleanup, err := b.buildMongoDB()
    if err != nil {
        return nil, nil, fmt.Errorf("mongodb: %w", err)
    }
    resources.MongoDB = mongo
    cleanups = append(cleanups, cleanup)
    
    // RabbitMQ
    channel, cleanup, err := b.buildRabbitMQ()
    if err != nil {
        return nil, nil, fmt.Errorf("rabbitmq: %w", err)
    }
    resources.RabbitMQChannel = channel
    cleanups = append(cleanups, cleanup)
    
    // Logger
    logger, err := b.buildLogger()
    if err != nil {
        return nil, nil, fmt.Errorf("logger: %w", err)
    }
    resources.Logger = logger
    
    // Cleanup combinado
    cleanup := func() error {
        var errs []error
        for i := len(cleanups) - 1; i >= 0; i-- {
            if err := cleanups[i](); err != nil {
                errs = append(errs, err)
            }
        }
        if len(errs) > 0 {
            return errors.Join(errs...)
        }
        return nil
    }
    
    return resources, cleanup, nil
}

func (b *ResourceBuilder) buildPostgreSQL() (*sql.DB, func() error, error) {
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        b.cfg.Database.Postgres.Host,
        b.cfg.Database.Postgres.Port,
        b.cfg.Database.Postgres.User,
        b.cfg.Database.Postgres.Password,
        b.cfg.Database.Postgres.Database,
        b.cfg.Database.Postgres.SSLMode,
    )
    
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, nil, err
    }
    
    if err := db.PingContext(b.ctx); err != nil {
        db.Close()
        return nil, nil, err
    }
    
    db.SetMaxOpenConns(b.cfg.Database.Postgres.MaxConnections)
    
    cleanup := func() error {
        return db.Close()
    }
    
    return db, cleanup, nil
}

// Similar para MongoDB y RabbitMQ...
```

### Beneficios
- Código más claro y lineal
- Fácil de testear (cada build es independiente)
- Cleanup explícito y ordenado
- Sin punteros dobles

---

## RF-002: Implementar Patrón Registry para Processors

### Estado Actual

No hay routing real de eventos a processors. El código actual ignora los processors:

```go
// cmd/main.go
func processMessage(msg amqp.Delivery, resources *bootstrap.Resources, cfg *config.Config) error {
    // TODO: Implementar procesamiento real con processors
    return nil  // No hace nada
}
```

### Propuesta de Refactorización

```go
// internal/application/processor/registry.go

// Processor interface que todos los processors deben implementar
type Processor interface {
    EventType() string
    Process(ctx context.Context, payload []byte) error
}

// Registry mantiene un registro de processors por event type
type Registry struct {
    processors map[string]Processor
    logger     logger.Logger
}

// NewRegistry crea un nuevo registry con los processors configurados
func NewRegistry(
    db *sql.DB,
    mongodb *mongo.Database,
    logger logger.Logger,
) *Registry {
    r := &Registry{
        processors: make(map[string]Processor),
        logger:     logger,
    }
    
    // Registrar processors
    r.Register(NewMaterialUploadedProcessor(db, mongodb, logger))
    r.Register(NewMaterialDeletedProcessor(mongodb, logger))
    r.Register(NewAssessmentAttemptProcessor(logger))
    r.Register(NewStudentEnrolledProcessor(logger))
    
    return r
}

// Register registra un processor
func (r *Registry) Register(p Processor) {
    r.processors[p.EventType()] = p
    r.logger.Info("processor registered", "event_type", p.EventType())
}

// Process procesa un mensaje usando el processor correcto
func (r *Registry) Process(ctx context.Context, msg []byte) error {
    // Extraer event_type
    var base struct {
        EventType string `json:"event_type"`
    }
    if err := json.Unmarshal(msg, &base); err != nil {
        return fmt.Errorf("invalid message format: %w", err)
    }
    
    // Buscar processor
    processor, ok := r.processors[base.EventType]
    if !ok {
        r.logger.Warn("no processor for event type", "type", base.EventType)
        return nil // No es error, simplemente ignorar
    }
    
    // Procesar
    return processor.Process(ctx, msg)
}

// Actualizar cada processor para implementar la interfaz
func (p *MaterialUploadedProcessor) EventType() string {
    return "material_uploaded"
}

func (p *MaterialUploadedProcessor) Process(ctx context.Context, payload []byte) error {
    var event dto.MaterialUploadedEvent
    if err := json.Unmarshal(payload, &event); err != nil {
        return err
    }
    return p.process(ctx, event)  // método interno existente
}
```

### Uso en main.go

```go
// cmd/main.go
func main() {
    // ... inicialización ...
    
    // Crear registry de processors
    registry := processor.NewRegistry(
        resources.PostgreSQL,
        resources.MongoDB,
        resources.Logger,
    )
    
    // Procesar mensajes
    go func() {
        for msg := range msgs {
            if err := registry.Process(ctx, msg.Body); err != nil {
                resources.Logger.Error("processing failed", "error", err)
                msg.Nack(false, true)
            } else {
                msg.Ack(false)
            }
        }
    }()
}
```

---

## RF-003: Extraer Interfaz Común de Repositories

### Estado Actual

Los repositories tienen métodos similares pero no comparten interfaz:

```go
// Métodos repetidos en cada repository
func (r *MaterialSummaryRepository) Create(ctx, entity) error
func (r *MaterialSummaryRepository) FindByID(ctx, id) (*Entity, error)
func (r *MaterialSummaryRepository) Update(ctx, entity) error
func (r *MaterialSummaryRepository) Delete(ctx, id) error

func (r *MaterialAssessmentRepository) Create(ctx, entity) error
func (r *MaterialAssessmentRepository) FindByID(ctx, id) (*Entity, error)
// ... mismo patrón
```

### Propuesta de Refactorización

```go
// internal/domain/repository/interfaces.go

// Repository interfaz genérica para operaciones CRUD
type Repository[T any, ID any] interface {
    Create(ctx context.Context, entity *T) error
    FindByID(ctx context.Context, id ID) (*T, error)
    Update(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id ID) error
    Exists(ctx context.Context, id ID) (bool, error)
}

// MaterialSummaryRepository interfaz específica
type MaterialSummaryRepository interface {
    Repository[entities.MaterialSummary, primitive.ObjectID]
    FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialSummary, error)
    FindByLanguage(ctx context.Context, language string, limit int64) ([]*entities.MaterialSummary, error)
}

// internal/infrastructure/persistence/mongodb/base_repository.go

// BaseRepository implementación base con métodos comunes
type BaseRepository[T any] struct {
    collection *mongo.Collection
    logger     logger.Logger
}

func (r *BaseRepository[T]) closeCursor(ctx context.Context, cursor *mongo.Cursor) {
    if err := cursor.Close(ctx); err != nil {
        r.logger.Error("error closing cursor", "error", err)
    }
}

// Reusar en repositories concretos
type materialSummaryRepository struct {
    BaseRepository[entities.MaterialSummary]
    validator *service.SummaryValidator
}
```

---

## RF-004: Unificar Validadores Duplicados

### Estado Actual

Los validadores tienen código duplicado:

```go
// assessment_validator.go
func (v *AssessmentValidator) isValidAIModel(model string) bool {
    validModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"}
    for _, m := range validModels {
        if model == m {
            return true
        }
    }
    return false
}

// summary_validator.go - EXACTAMENTE IGUAL
func (v *SummaryValidator) isValidAIModel(model string) bool {
    validModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"}
    for _, m := range validModels {
        if model == m {
            return true
        }
    }
    return false
}
```

### Propuesta de Refactorización

```go
// internal/domain/service/common_validators.go

// CommonValidator contiene validaciones compartidas
type CommonValidator struct {
    validAIModels  []string
    validLanguages []string
}

// NewCommonValidator con valores por defecto o desde config
func NewCommonValidator() *CommonValidator {
    return &CommonValidator{
        validAIModels:  []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"},
        validLanguages: []string{"es", "en", "pt"},
    }
}

func (v *CommonValidator) IsValidAIModel(model string) bool {
    return slices.Contains(v.validAIModels, model)
}

func (v *CommonValidator) IsValidLanguage(lang string) bool {
    return slices.Contains(v.validLanguages, lang)
}

// Usar en otros validadores
type SummaryValidator struct {
    common *CommonValidator
}

func (v *SummaryValidator) IsValid(summary *entities.MaterialSummary) bool {
    // ...
    if !v.common.IsValidAIModel(summary.AIModel) {
        return false
    }
    if !v.common.IsValidLanguage(summary.Language) {
        return false
    }
    // ...
}
```

---

## RF-005: Configuración Type-Safe con Validación

### Estado Actual

La configuración se valida manualmente con strings:

```go
func (c *Config) Validate() error {
    if c.Database.Postgres.Password == "" {
        return fmt.Errorf("POSTGRES_PASSWORD is required")
    }
    // ... muchos if manuales
}
```

### Propuesta de Refactorización

```go
// internal/config/validation.go

import "github.com/go-playground/validator/v10"

type Config struct {
    Database  DatabaseConfig  `mapstructure:"database" validate:"required"`
    Messaging MessagingConfig `mapstructure:"messaging" validate:"required"`
    NLP       NLPConfig       `mapstructure:"nlp" validate:"required"`
}

type PostgresConfig struct {
    Host           string `mapstructure:"host" validate:"required,hostname|ip"`
    Port           int    `mapstructure:"port" validate:"required,min=1,max=65535"`
    Database       string `mapstructure:"database" validate:"required"`
    User           string `mapstructure:"user" validate:"required"`
    Password       string `mapstructure:"password" validate:"required,min=8"`
    MaxConnections int    `mapstructure:"max_connections" validate:"min=1,max=100"`
    SSLMode        string `mapstructure:"ssl_mode" validate:"oneof=disable require verify-full"`
}

// Validación automática
func (c *Config) Validate() error {
    validate := validator.New()
    if err := validate.Struct(c); err != nil {
        return formatValidationErrors(err)
    }
    return nil
}

func formatValidationErrors(err error) error {
    var messages []string
    for _, err := range err.(validator.ValidationErrors) {
        messages = append(messages, fmt.Sprintf(
            "field '%s' failed validation '%s'",
            err.Field(),
            err.Tag(),
        ))
    }
    return errors.New(strings.Join(messages, "; "))
}
```

---

## RF-006: Estandarizar Logging

### Estado Actual

Mezcla de `log.Printf` y logger estructurado:

```go
// Algunos lugares usan log estándar
log.Printf("Error en Nack: %v", err)

// Otros usan logger estructurado
resources.Logger.Info("mensaje", "key", value)
```

### Propuesta

```go
// internal/infrastructure/logging/context.go

// ContextKey para logger en context
type contextKey string
const loggerKey contextKey = "logger"

// WithLogger agrega logger al context
func WithLogger(ctx context.Context, logger logger.Logger) context.Context {
    return context.WithValue(ctx, loggerKey, logger)
}

// FromContext obtiene logger del context
func FromContext(ctx context.Context) logger.Logger {
    if l, ok := ctx.Value(loggerKey).(logger.Logger); ok {
        return l
    }
    return NewNopLogger() // Logger que no hace nada como fallback
}

// Uso en repositorios
func (r *MaterialSummaryRepository) FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialSummary, error) {
    logger := logging.FromContext(ctx)
    
    // ...
    
    defer func() {
        if err := cursor.Close(ctx); err != nil {
            logger.Error("error closing cursor", "error", err)
        }
    }()
}
```

---

## RF-007: Error Types Consistentes

### Propuesta

```go
// internal/domain/errors/errors.go

// WorkerError es el tipo base para errores del worker
type WorkerError struct {
    Code       string
    Message    string
    Cause      error
    Retryable  bool
    Context    map[string]interface{}
}

func (e *WorkerError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *WorkerError) Unwrap() error {
    return e.Cause
}

// Errores predefinidos
var (
    ErrInvalidMaterialID = &WorkerError{Code: "INVALID_MATERIAL_ID", Message: "material ID is invalid", Retryable: false}
    ErrDatabaseConnection = &WorkerError{Code: "DB_CONNECTION", Message: "database connection failed", Retryable: true}
    ErrOpenAITimeout = &WorkerError{Code: "OPENAI_TIMEOUT", Message: "OpenAI request timed out", Retryable: true}
)

// Constructores
func NewValidationError(field string, reason string) *WorkerError {
    return &WorkerError{
        Code:      "VALIDATION_ERROR",
        Message:   fmt.Sprintf("validation failed for %s: %s", field, reason),
        Retryable: false,
        Context:   map[string]interface{}{"field": field, "reason": reason},
    }
}

// Uso
func (p *MaterialUploadedProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    materialID, err := valueobject.MaterialIDFromString(event.MaterialID)
    if err != nil {
        return errors.NewValidationError("material_id", "invalid UUID format").WithCause(err)
    }
    // ...
}
```

---

## RF-008: Agregar Test Doubles y Mocks

### Propuesta

```go
// internal/mocks/repositories.go

// MockMaterialSummaryRepository mock para tests
type MockMaterialSummaryRepository struct {
    CreateFunc          func(ctx context.Context, summary *entities.MaterialSummary) error
    FindByMaterialIDFunc func(ctx context.Context, materialID string) (*entities.MaterialSummary, error)
    // ... otros métodos
    
    CreateCalls          []entities.MaterialSummary
    FindByMaterialIDCalls []string
}

func (m *MockMaterialSummaryRepository) Create(ctx context.Context, summary *entities.MaterialSummary) error {
    m.CreateCalls = append(m.CreateCalls, *summary)
    if m.CreateFunc != nil {
        return m.CreateFunc(ctx, summary)
    }
    return nil
}

// Uso en tests
func TestMaterialUploadedProcessor(t *testing.T) {
    mockRepo := &mocks.MockMaterialSummaryRepository{
        CreateFunc: func(ctx context.Context, summary *entities.MaterialSummary) error {
            return nil
        },
    }
    
    processor := NewMaterialUploadedProcessor(nil, mockRepo, nil)
    err := processor.Process(ctx, event)
    
    assert.NoError(t, err)
    assert.Len(t, mockRepo.CreateCalls, 1)
    assert.Equal(t, "expected-material-id", mockRepo.CreateCalls[0].MaterialID)
}
```

---

## 📊 Matriz de Priorización

| Refactorización | Urgencia | Impacto | Riesgo | Orden Sugerido |
|-----------------|----------|---------|--------|----------------|
| RF-002 (Registry) | Alta | Alto | Bajo | 1 |
| RF-007 (Errors) | Alta | Alto | Bajo | 2 |
| RF-001 (Bootstrap) | Media | Alto | Medio | 3 |
| RF-006 (Logging) | Media | Medio | Bajo | 4 |
| RF-004 (Validators) | Baja | Medio | Bajo | 5 |
| RF-003 (Repositories) | Baja | Medio | Medio | 6 |
| RF-005 (Config) | Baja | Alto | Medio | 7 |
| RF-008 (Testing) | Media | Alto | Bajo | 8 |
