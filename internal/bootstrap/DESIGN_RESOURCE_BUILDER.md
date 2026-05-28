# Diseño: ResourceBuilder Pattern

> **Objetivo**: Eliminar el uso de doble puntero (`**Type`) y simplificar la inicialización de recursos con un patrón Builder limpio.

---

## 🎯 Problemas Actuales

### 1. Complejidad del Wrapper
```go
type customFactoriesWrapper struct {
    sqlDB         *sql.DB        // Referencia directa
    mongoClient   *mongo.Client  // Referencia directa
    rabbitChannel *amqp.Channel  // Referencia directa
    // ...
}

type customPostgreSQLFactory struct {
    sqlDB  **sql.DB  // ❌ Doble puntero - difícil de entender
}
```

**Problemas**:
- Doble puntero confuso y propenso a errores
- Lógica de retención de referencias mezclada con factories
- Difícil de testear y mantener

### 2. Función Bridge Muy Larga
La función `bridgeToSharedBootstrap()` tiene 150+ líneas con:
- Configuración de GORM logger
- Creación de wrapper y factories
- Llamada a shared/bootstrap
- Creación de adaptadores
- Creación de processors
- Construcción de Resources
- Setup de cleanup

**Problema**: Función monolítica con múltiples responsabilidades

### 3. Cleanup Desordenado
```go
cleanup := func() error {
    resources.Logger.Info("starting worker cleanup")
    err := lifecycleWithLogger.Cleanup()
    // ...
}
```

**Problema**: No hay garantía del orden de cleanup ni manejo de dependencias

---

## 🏗️ Diseño Propuesto: ResourceBuilder

### Arquitectura

```
┌─────────────────────────────────────────────────────┐
│                  ResourceBuilder                     │
├─────────────────────────────────────────────────────┤
│  - config: *config.Config                           │
│  - ctx: context.Context                             │
│  - logger: logger.Logger                            │
│  - postgresql: *sql.DB                              │
│  - mongodb: *mongo.Database                         │
│  - rabbitmq: *amqp.Channel                          │
│  - authClient: *client.AuthClient                   │
│  - processorRegistry: *processor.Registry           │
│  - cleanupFuncs: []func() error                     │
├─────────────────────────────────────────────────────┤
│  + New(ctx, cfg) *ResourceBuilder                   │
│  + WithLogger() *ResourceBuilder                    │
│  + WithPostgreSQL() *ResourceBuilder                │
│  + WithMongoDB() *ResourceBuilder                   │
│  + WithRabbitMQ() *ResourceBuilder                  │
│  + WithAuthClient() *ResourceBuilder                │
│  + WithProcessors() *ResourceBuilder                │
│  + Build() (*Resources, func() error, error)        │
└─────────────────────────────────────────────────────┘
```

### Interfaz Pública

```go
// ResourceBuilder construye Resources de forma incremental
type ResourceBuilder struct {
    // Campos privados
    config           *config.Config
    ctx              context.Context
    
    // Recursos construidos
    logger           logger.Logger
    postgresql       *sql.DB
    mongodb          *mongo.Database
    rabbitmq         *amqp.Channel
    authClient       *client.AuthClient
    processorRegistry *processor.Registry
    
    // Cleanup
    cleanupFuncs     []func() error
    
    // Estado interno
    err              error
}

// Constructores y Builders

func NewResourceBuilder(ctx context.Context, cfg *config.Config) *ResourceBuilder

func (b *ResourceBuilder) WithLogger() *ResourceBuilder

func (b *ResourceBuilder) WithPostgreSQL() *ResourceBuilder

func (b *ResourceBuilder) WithMongoDB() *ResourceBuilder

func (b *ResourceBuilder) WithRabbitMQ() *ResourceBuilder

func (b *ResourceBuilder) WithAuthClient() *ResourceBuilder

func (b *ResourceBuilder) WithProcessors() *ResourceBuilder

func (b *ResourceBuilder) Build() (*Resources, func() error, error)
```

### Uso

```go
// En main.go
resources, cleanup, err := bootstrap.NewResourceBuilder(ctx, cfg).
    WithLogger().
    WithPostgreSQL().
    WithMongoDB().
    WithRabbitMQ().
    WithAuthClient().
    WithProcessors().
    Build()

if err != nil {
    log.Fatal("Error inicializando recursos:", err)
}
defer cleanup()
```

---

## 🔧 Implementación Detallada

### 1. Estructura ResourceBuilder

```go
type ResourceBuilder struct {
    config *config.Config
    ctx    context.Context
    
    // Recursos de infraestructura
    logger           logger.Logger
    logrusLogger     *logrus.Logger      // Para adapter
    sqlDB            *sql.DB
    mongoClient      *mongo.Client
    mongodb          *mongo.Database
    rabbitConn       *amqp.Connection
    rabbitChannel    *amqp.Channel
    
    // Recursos de aplicación
    authClient        *client.AuthClient
    processorRegistry *processor.Registry
    
    // Lifecycle
    lifecycleManager *lifecycle.Manager
    cleanupFuncs     []func() error
    
    // Control de errores
    err error
}
```

### 2. Método WithLogger()

```go
func (b *ResourceBuilder) WithLogger() *ResourceBuilder {
    if b.err != nil {
        return b // Early return si ya hay error
    }
    
    // 1. Crear logger usando shared/bootstrap
    loggerFactory := bootstrap.NewDefaultLoggerFactory()
    logrusLogger, err := loggerFactory.CreateLogger(
        b.ctx,
        "production",
        "v1.0.0",
    )
    if err != nil {
        b.err = fmt.Errorf("failed to create logger: %w", err)
        return b
    }
    
    // 2. Guardar referencias
    b.logrusLogger = logrusLogger
    b.logger = adapter.NewLoggerAdapter(logrusLogger)
    
    // 3. Registrar cleanup
    b.addCleanup(func() error {
        b.logger.Info("closing logger")
        return b.logger.Sync()
    })
    
    return b
}
```

### 3. Método WithPostgreSQL()

```go
func (b *ResourceBuilder) WithPostgreSQL() *ResourceBuilder {
    if b.err != nil {
        return b
    }
    
    if b.logger == nil {
        b.err = fmt.Errorf("logger required before PostgreSQL")
        return b
    }
    
    // 1. Configurar GORM logger
    gormLogLevel := gormLogger.Silent
    if b.config.Logging.Level == "debug" {
        gormLogLevel = gormLogger.Info
    }
    gormLog := gormLogger.Default.LogMode(gormLogLevel)
    
    // 2. Crear factory y conexión
    pgFactory := bootstrap.NewDefaultPostgreSQLFactory(gormLog)
    sqlDB, err := pgFactory.CreateRawConnection(b.ctx, bootstrap.PostgreSQLConfig{
        Host:     b.config.Database.Postgres.Host,
        Port:     b.config.Database.Postgres.Port,
        User:     b.config.Database.Postgres.User,
        Password: b.config.Database.Postgres.Password,
        Database: b.config.Database.Postgres.Database,
        SSLMode:  b.config.Database.Postgres.SSLMode,
    })
    if err != nil {
        b.err = fmt.Errorf("failed to connect to PostgreSQL: %w", err)
        return b
    }
    
    // 3. Guardar referencia
    b.sqlDB = sqlDB
    
    // 4. Registrar cleanup
    b.addCleanup(func() error {
        b.logger.Info("closing PostgreSQL connection")
        return b.sqlDB.Close()
    })
    
    b.logger.Info("PostgreSQL connected successfully")
    return b
}
```

### 4. Método WithMongoDB()

```go
func (b *ResourceBuilder) WithMongoDB() *ResourceBuilder {
    if b.err != nil {
        return b
    }
    
    if b.logger == nil {
        b.err = fmt.Errorf("logger required before MongoDB")
        return b
    }
    
    // 1. Crear factory y cliente
    mongoFactory := bootstrap.NewDefaultMongoDBFactory()
    mongoClient, err := mongoFactory.CreateConnection(b.ctx, bootstrap.MongoDBConfig{
        URI:      b.config.Database.MongoDB.URI,
        Database: b.config.Database.MongoDB.Database,
    })
    if err != nil {
        b.err = fmt.Errorf("failed to connect to MongoDB: %w", err)
        return b
    }
    
    // 2. Obtener database
    mongoDB := mongoFactory.GetDatabase(mongoClient, b.config.Database.MongoDB.Database)
    
    // 3. Guardar referencias
    b.mongoClient = mongoClient
    b.mongodb = mongoDB
    
    // 4. Registrar cleanup
    b.addCleanup(func() error {
        b.logger.Info("closing MongoDB connection")
        return b.mongoClient.Disconnect(context.Background())
    })
    
    b.logger.Info("MongoDB connected successfully")
    return b
}
```

### 5. Método WithRabbitMQ()

```go
func (b *ResourceBuilder) WithRabbitMQ() *ResourceBuilder {
    if b.err != nil {
        return b
    }
    
    if b.logger == nil {
        b.err = fmt.Errorf("logger required before RabbitMQ")
        return b
    }
    
    // 1. Crear factory
    rabbitFactory := bootstrap.NewDefaultRabbitMQFactory()
    
    // 2. Crear conexión
    conn, err := rabbitFactory.CreateConnection(b.ctx, bootstrap.RabbitMQConfig{
        URL: b.config.Messaging.RabbitMQ.URL,
    })
    if err != nil {
        b.err = fmt.Errorf("failed to connect to RabbitMQ: %w", err)
        return b
    }
    
    // 3. Crear channel
    channel, err := rabbitFactory.CreateChannel(conn)
    if err != nil {
        conn.Close()
        b.err = fmt.Errorf("failed to create RabbitMQ channel: %w", err)
        return b
    }
    
    // 4. Guardar referencias
    b.rabbitConn = conn
    b.rabbitChannel = channel
    
    // 5. Registrar cleanup (orden inverso: channel antes que connection)
    b.addCleanup(func() error {
        b.logger.Info("closing RabbitMQ channel")
        if err := b.rabbitChannel.Close(); err != nil {
            return err
        }
        b.logger.Info("closing RabbitMQ connection")
        return b.rabbitConn.Close()
    })
    
    b.logger.Info("RabbitMQ connected successfully")
    return b
}
```

### 6. Método WithAuthClient()

```go
func (b *ResourceBuilder) WithAuthClient() *ResourceBuilder {
    if b.err != nil {
        return b
    }
    
    // AuthClient no tiene dependencias de infraestructura
    apiAdminCfg := b.config.GetAPIAdminConfigWithDefaults()
    b.authClient = client.NewAuthClient(client.AuthClientConfig{
        BaseURL:      apiAdminCfg.BaseURL,
        Timeout:      apiAdminCfg.Timeout,
        CacheTTL:     apiAdminCfg.CacheTTL,
        CacheEnabled: apiAdminCfg.CacheEnabled,
        MaxBulkSize:  apiAdminCfg.MaxBulkSize,
    })
    
    return b
}
```

### 7. Método WithProcessors()

```go
func (b *ResourceBuilder) WithProcessors() *ResourceBuilder {
    if b.err != nil {
        return b
    }
    
    // Verificar dependencias
    if b.logger == nil || b.sqlDB == nil || b.mongodb == nil {
        b.err = fmt.Errorf("logger, PostgreSQL and MongoDB required before processors")
        return b
    }
    
    // 1. Crear processors individuales
    materialUploadedProc := processor.NewMaterialUploadedProcessor(
        b.sqlDB,
        b.mongodb,
        b.logger,
    )
    materialDeletedProc := processor.NewMaterialDeletedProcessor(
        b.mongodb,
        b.logger,
    )
    assessmentAttemptProc := processor.NewAssessmentAttemptProcessor(b.logger)
    studentEnrolledProc := processor.NewStudentEnrolledProcessor(b.logger)
    
    // 2. Crear registry
    registry := processor.NewRegistry(b.logger)
    
    // 3. Registrar processors
    registry.Register(materialUploadedProc)
    registry.Register(processor.NewMaterialReprocessProcessor(materialUploadedProc, b.logger))
    registry.Register(materialDeletedProc)
    registry.Register(assessmentAttemptProc)
    registry.Register(studentEnrolledProc)
    
    // 4. Guardar referencia
    b.processorRegistry = registry
    
    b.logger.Info("processors registered", "count", registry.Count())
    return b
}
```

### 8. Método Build()

```go
func (b *ResourceBuilder) Build() (*Resources, func() error, error) {
    // Verificar si hubo errores durante la construcción
    if b.err != nil {
        // Ejecutar cleanup de recursos parcialmente inicializados
        b.cleanup()
        return nil, nil, b.err
    }
    
    // Verificar que todos los recursos requeridos están inicializados
    if b.logger == nil {
        return nil, nil, fmt.Errorf("logger is required")
    }
    
    // Crear lifecycle manager con logger
    b.lifecycleManager = lifecycle.NewManager(b.logger)
    
    // Construir Resources
    resources := &Resources{
        Logger:            b.logger,
        PostgreSQL:        b.sqlDB,
        MongoDB:           b.mongodb,
        RabbitMQChannel:   b.rabbitChannel,
        AuthClient:        b.authClient,
        LifecycleManager:  b.lifecycleManager,
        ProcessorRegistry: b.processorRegistry,
    }
    
    // Crear función de cleanup
    cleanup := func() error {
        return b.cleanup()
    }
    
    return resources, cleanup, nil
}
```

### 9. Método Auxiliar addCleanup()

```go
func (b *ResourceBuilder) addCleanup(fn func() error) {
    // Los cleanup se agregan al inicio para ejecutar en orden inverso (LIFO)
    b.cleanupFuncs = append([]func() error{fn}, b.cleanupFuncs...)
}
```

### 10. Método Auxiliar cleanup()

```go
func (b *ResourceBuilder) cleanup() error {
    if b.logger != nil {
        b.logger.Info("starting resource cleanup")
    }
    
    var errors []error
    
    // Ejecutar cleanups en orden (LIFO - último creado, primero cerrado)
    for i, cleanupFn := range b.cleanupFuncs {
        if err := cleanupFn(); err != nil {
            errors = append(errors, fmt.Errorf("cleanup %d failed: %w", i, err))
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("cleanup had %d errors: %v", len(errors), errors)
    }
    
    if b.logger != nil {
        b.logger.Info("resource cleanup completed successfully")
    }
    
    return nil
}
```

---

## ✅ Ventajas del Nuevo Diseño

### 1. **Sin Doble Punteros**
```go
// ❌ Antes
type customPostgreSQLFactory struct {
    sqlDB  **sql.DB
}

// ✅ Después
type ResourceBuilder struct {
    sqlDB *sql.DB  // Referencia directa
}
```

### 2. **Fluent API**
```go
resources, cleanup, err := bootstrap.NewResourceBuilder(ctx, cfg).
    WithLogger().
    WithPostgreSQL().
    WithMongoDB().
    Build()
```

### 3. **Cleanup Ordenado**
- Orden LIFO garantizado
- Cada recurso registra su propio cleanup
- Manejo robusto de errores

### 4. **Fácil de Testear**
```go
func TestResourceBuilder_WithLogger(t *testing.T) {
    builder := NewResourceBuilder(ctx, cfg)
    builder = builder.WithLogger()
    
    assert.Nil(t, builder.err)
    assert.NotNil(t, builder.logger)
}
```

### 5. **Detección Temprana de Dependencias**
```go
func (b *ResourceBuilder) WithPostgreSQL() *ResourceBuilder {
    if b.logger == nil {
        b.err = fmt.Errorf("logger required before PostgreSQL")
        return b
    }
    // ...
}
```

---

## 📋 Plan de Migración

### Fase 1: Implementar ResourceBuilder
1. Crear `resource_builder.go`
2. Implementar todos los métodos With*
3. Agregar tests unitarios

### Fase 2: Migrar main.go
1. Reemplazar `bridgeToSharedBootstrap` con builder
2. Validar funcionamiento
3. Mantener código antiguo comentado temporalmente

### Fase 3: Eliminar Código Antiguo
1. Eliminar `custom_factories.go`
2. Simplificar `bridge.go` (o eliminarlo)
3. Actualizar documentación

---

## 🎓 Lecciones del Diseño

1. **Builder Pattern** para construcción compleja
2. **Fluent Interface** para legibilidad
3. **LIFO Cleanup** para manejo robusto de recursos
4. **Early Return** para manejo de errores
5. **Dependency Injection** explícita
6. **Single Responsibility** - cada método hace una cosa

---

## 📝 Notas de Implementación

- **Contexto**: Usar `b.ctx` para operaciones asíncronas
- **Logging**: Cada With* loguea éxito/fallo
- **Errores**: Acumular en `b.err` y verificar al inicio de cada método
- **Cleanup**: Registrar inmediatamente después de crear recurso
- **Orden**: Logger siempre primero (necesario para otros recursos)
