# Auditoría del Código - edugo-worker

**Sprint:** Sprint-01 Fase 1 - Auditoría y Diseño de Schemas MongoDB
**Fecha:** 2025-11-18
**Auditor:** Claude Code Web
**Versión del código:** commit 8d1bc1d (rama: dev)
**Repositorio:** https://github.com/EduGoGroup/edugo-worker

---

## 📊 Resumen Ejecutivo

### Estado General

El proyecto **edugo-worker** es un worker de procesamiento asíncrono con IA que presenta:

- ✅ **Arquitectura sólida**: Clean Architecture bien implementada con 3 capas claras
- ✅ **Integración moderna**: Uso correcto de shared/bootstrap v0.7.0
- ✅ **Base funcional**: RabbitMQ consumer funcionando correctamente
- ⚠️ **Funcionalidad limitada**: ~30% implementado, ~70% MOCK
- ⚠️ **Cobertura de tests baja**: ~10% estimado

### Nivel de Madurez

**Clasificación:** Prototipo funcional con estructura sólida

El proyecto está en fase de desarrollo temprano con:
- Arquitectura bien diseñada
- Infraestructura básica funcionando
- Procesadores implementados con lógica MOCK
- Pendiente: integración real con OpenAI, PDF, S3, MongoDB repositories

---

## 🏗️ Análisis de Arquitectura

### Cumplimiento de Clean Architecture

#### ✅ Fortalezas Arquitectónicas

**1. Separación de Capas Clara**

```
internal/
├── domain/                    # Capa de Dominio
│   ├── entity/               # ⚠️ Vacío (no hay entidades aún)
│   ├── service/              # ⚠️ Vacío
│   └── valueobject/          # ✅ MaterialID implementado
├── application/              # Capa de Aplicación
│   ├── dto/                  # ✅ Event DTOs bien definidos
│   ├── processor/            # ✅ 5 procesadores implementados
│   └── service/              # ⚠️ Vacío
├── infrastructure/           # Capa de Infraestructura
│   ├── messaging/            # ✅ RabbitMQ consumer funcionando
│   ├── persistence/          # ⚠️ Repositories pendientes
│   ├── nlp/                  # ⚠️ Cliente OpenAI pendiente
│   ├── pdf/                  # ⚠️ Procesador PDF pendiente
│   └── storage/              # ⚠️ Cliente S3 pendiente
├── bootstrap/                # ✅ Integración con shared/bootstrap
├── config/                   # ✅ Configuración bien estructurada
└── container/                # ✅ DI implementado correctamente
```

**2. Dependency Injection Correcta**

El proyecto usa correctamente DI mediante constructor injection:

```go
// internal/container/container.go:30-55
func NewContainer(db *sql.DB, mongodb *mongo.Database, logger logger.Logger) *Container {
    c := &Container{
        DB:      db,
        MongoDB: mongodb,
        Logger:  logger,
    }

    c.MaterialUploadedProc = processor.NewMaterialUploadedProcessor(db, mongodb, logger)
    // ... más processors

    c.EventConsumer = consumer.NewEventConsumer(
        c.MaterialUploadedProc,
        c.MaterialReprocessProc,
        // ... más procesadores
        logger,
    )

    return c
}
```

✅ **Buena práctica**: Todas las dependencias se inyectan, no hay creación de dependencias dentro de clases.

**3. Value Objects Implementados**

```go
// internal/domain/valueobject/material_id.go:7-21
type MaterialID struct {
    value types.UUID
}

func MaterialIDFromString(s string) (MaterialID, error) {
    uuid, err := types.ParseUUID(s)
    if err != nil {
        return MaterialID{}, err
    }
    return MaterialID{value: uuid}, nil
}
```

✅ **Buena práctica**: Encapsulación de UUID con validación en constructor.

---

#### ⚠️ Debilidades Arquitectónicas

**1. Capa de Dominio Incompleta**

**Problema:**
- ❌ No hay entidades de dominio (`internal/domain/entity/` vacío)
- ❌ No hay servicios de dominio (`internal/domain/service/` vacío)
- ❌ Solo existe un Value Object (MaterialID)

**Impacto:**
- La lógica de negocio está dispersa en procesadores (capa de aplicación)
- No hay encapsulación de reglas de negocio complejas

**Recomendación:**
Crear entidades como `MaterialSummary`, `Assessment`, `Question` con comportamiento de dominio.

**2. Application Services No Existen**

**Problema:**
- Los procesadores asumen responsabilidades de application services
- No hay separación entre orquestación (use cases) y procesamiento de eventos

**Recomendación:**
Crear services como `MaterialProcessingService` que encapsulen la lógica de orquestación.

---

### Integración con shared/bootstrap

#### ✅ Integración Correcta

El proyecto usa **shared/bootstrap v0.7.0** correctamente mediante un bridge pattern:

```go
// internal/bootstrap/bridge.go:27-122
func bridgeToSharedBootstrap(ctx context.Context, cfg *config.Config) (*Resources, func() error, error) {
    // 1. Configurar factories
    sharedFactories := &sharedBootstrap.Factories{
        Logger:     sharedBootstrap.NewDefaultLoggerFactory(),
        PostgreSQL: sharedBootstrap.NewDefaultPostgreSQLFactory(gormLog),
        MongoDB:    sharedBootstrap.NewDefaultMongoDBFactory(),
        RabbitMQ:   sharedBootstrap.NewDefaultRabbitMQFactory(),
    }

    // 2. Bootstrap con shared
    _, err := sharedBootstrap.Bootstrap(ctx, bootstrapConfig, customFactories, lifecycleManager, ...)

    // 3. Retornar recursos tipados para worker
    return &Resources{
        Logger:           loggerAdapter,
        PostgreSQL:       wrapper.sqlDB,
        MongoDB:          wrapper.mongoClient.Database(cfg.Database.MongoDB.Database),
        RabbitMQChannel:  wrapper.rabbitChannel,
        LifecycleManager: lifecycleWithLogger,
    }, cleanup, nil
}
```

✅ **Ventajas:**
- Usa factories de shared para inicialización consistente
- Lifecycle manager para cleanup ordenado
- Configuración centralizada

---

## 🔍 Análisis Detallado por Componente

---

## 1. RabbitMQ Consumer (✅ Funcionando)

### Archivo: `cmd/main.go:1-144`

**Implementación actual:**

```go
// cmd/main.go:44-56
msgs, err := resources.RabbitMQChannel.Consume(
    cfg.Messaging.RabbitMQ.Queues.MaterialUploaded,
    "",    // consumer
    false, // auto-ack
    false, // exclusive
    false, // no-local
    false, // no-wait
    nil,
)

// cmd/main.go:62-71
go func() {
    for msg := range msgs {
        if err := processMessage(msg, resources, cfg); err != nil {
            resources.Logger.Error("Error procesando mensaje", "error", err.Error())
            msg.Nack(false, true) // requeue
        } else {
            msg.Ack(false)
        }
    }
}()
```

### ✅ Fortalezas

1. **Manual ACK/NACK**: Usa `auto-ack: false` para control explícito
2. **Requeue en error**: `msg.Nack(false, true)` reintenta mensajes fallidos
3. **Graceful shutdown**: Señales SIGINT/SIGTERM manejadas correctamente
4. **Queue configuration**: Dead Letter Exchange configurado

### ⚠️ Debilidades

1. **No usa EventConsumer**: El código en `main.go:127-143` no usa `container.EventConsumer.RouteEvent()`
2. **Sin circuit breaker**: No hay protección contra fallos en cascada
3. **Sin rate limiting**: Puede consumir mensajes más rápido de lo que procesa
4. **Sin métricas**: No registra throughput ni latencias

### 📋 Código Crítico

**Línea 138-140 (MOCK):**
```go
// TODO: Implementar procesamiento real con processors
// processor := container.GetProcessor(event["event_type"])
// return processor.Process(ctx, event)
```

❌ **Problema**: El procesamiento real no está conectado. El consumer actual solo loguea eventos.

### 📊 Estado de Funcionalidad

| Aspecto | Estado | Notas |
|---------|--------|-------|
| Conexión RabbitMQ | ✅ Funcionando | Usando shared/bootstrap |
| Consumer activo | ✅ Funcionando | Escucha cola correctamente |
| Event routing | 🔴 MOCK | No llama a processors reales |
| ACK/NACK | ✅ Funcionando | Manual ACK implementado |
| Error handling | ⚠️ Básico | Requeue sin backoff |
| Graceful shutdown | ✅ Funcionando | SIGINT/SIGTERM manejados |

---

## 2. Event Router (✅ Implementado, ❌ No Usado)

### Archivo: `internal/infrastructure/messaging/consumer/event_consumer.go:1-97`

**Implementación:**

```go
// event_consumer.go:42-96
func (c *EventConsumer) RouteEvent(ctx context.Context, body []byte) error {
    var baseEvent struct {
        EventType string `json:"event_type"`
    }

    if err := json.Unmarshal(body, &baseEvent); err != nil {
        c.logger.Error("failed to parse event", "error", err)
        return err
    }

    switch enum.EventType(baseEvent.EventType) {
    case enum.EventMaterialUploaded:
        var event dto.MaterialUploadedEvent
        if err := json.Unmarshal(body, &event); err != nil {
            return err
        }
        return c.materialUploadedProc.Process(ctx, event)
    // ... más casos
    default:
        c.logger.Warn("unknown event type", "event_type", baseEvent.EventType)
        return nil
    }
}
```

### ✅ Fortalezas

1. **Type-safe routing**: Usa enums de shared (`enum.EventType`)
2. **Desacoplamiento**: Procesadores inyectados mediante DI
3. **Unknown events**: No falla, solo loguea warning

### ⚠️ Debilidades

1. **No usado en main.go**: Este código bien diseñado NO se está usando
2. **Sin métricas**: No registra eventos por tipo
3. **Sin tracing**: No hay correlation IDs para seguimiento

### 📊 Estado

| Aspecto | Estado |
|---------|--------|
| Código implementado | ✅ Completo |
| Usado en runtime | ❌ NO |
| Tests | ❌ Sin tests |

---

## 3. MaterialUploadedProcessor (⚠️ MOCK)

### Archivo: `internal/application/processor/material_uploaded_processor.go:1-120`

**Análisis detallado:**

### ✅ Funcionalidad Real (30%)

**1. Validación de material_id**
```go
// material_uploaded_processor.go:38-41
materialID, err := valueobject.MaterialIDFromString(event.MaterialID)
if err != nil {
    return errors.NewValidationError("invalid material_id")
}
```
✅ **Implementado**: Usa Value Object con validación

**2. Transacción PostgreSQL**
```go
// material_uploaded_processor.go:44-110
err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx,
        "UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
        enum.ProcessingStatusProcessing.String(),
        materialID.String(),
    )
    // ... más operaciones en transacción
    return err
})
```
✅ **Implementado**: Usa transacción de shared/database/postgres correctamente

**3. Logging estructurado**
```go
// material_uploaded_processor.go:33-36
p.logger.Info("processing material uploaded",
    "material_id", event.MaterialID,
    "s3_key", event.S3Key,
)
```
✅ **Implementado**: Logging con campos estructurados

---

### 🔴 Funcionalidad MOCK (70%)

**1. Extracción de texto PDF (Línea 55-56)**
```go
// MOCK - No implementado
p.logger.Debug("extracting PDF text", "s3_key", event.S3Key)
```
❌ **Pendiente**: Integración con biblioteca PDF (ej: pdfcpu, unipdf)

**2. Generación de resumen con OpenAI (Línea 58-59)**
```go
// MOCK - No implementado
p.logger.Debug("generating summary with AI")
```
❌ **Pendiente**: Cliente OpenAI para GPT-4

**3. Guardado en MongoDB (Línea 61-75)**
```go
// MOCK - Estructura básica
summaryCollection := p.mongodb.Collection("material_summaries")
summary := bson.M{
    "material_id":  event.MaterialID,
    "main_ideas":   []string{"Idea 1", "Idea 2", "Idea 3"},
    "key_concepts": bson.M{"concept1": "definition1"},
    // ...
}
_, err = summaryCollection.InsertOne(ctx, summary)
```
⚠️ **Problemas:**
- Usa `bson.M` en lugar de structs tipados
- Datos hardcodeados
- No usa validation schemas de MongoDB
- No maneja errores de duplicados (unique index en `material_id`)

**4. Generación de quiz con IA (Línea 77-78)**
```go
// MOCK - No implementado
p.logger.Debug("generating quiz with AI")
```
❌ **Pendiente**: Cliente OpenAI para generar preguntas

**5. Guardado de assessment (Línea 80-100)**
```go
// MOCK - Estructura básica
assessmentCollection := p.mongodb.Collection("material_assessments")
assessment := bson.M{
    "material_id": event.MaterialID,
    "questions": []bson.M{
        {
            "id":             "q1",
            "question_text":  "Pregunta de ejemplo",
            "question_type":  "multiple_choice",
            // ... datos hardcodeados
        },
    },
}
_, err = assessmentCollection.insertOne(ctx, assessment)
```
⚠️ **Problemas similares a summary**

---

### 📊 Matriz de Funcionalidad

| Función | Estado | Implementación Real | Estimación |
|---------|--------|---------------------|------------|
| Validación material_id | ✅ Real | Value Object | Completo |
| Actualizar estado PostgreSQL | ✅ Real | shared/database/postgres | Completo |
| Extraer texto PDF | 🔴 MOCK | Pendiente biblioteca | 2-3 días |
| Llamar OpenAI (summary) | 🔴 MOCK | Pendiente cliente API | 2-3 días |
| Guardar summary MongoDB | 🔴 MOCK | Usar repository + structs | 1-2 días |
| Llamar OpenAI (quiz) | 🔴 MOCK | Pendiente cliente API | 2-3 días |
| Guardar assessment MongoDB | 🔴 MOCK | Usar repository + structs | 1-2 días |
| Publicar evento completado | 🔴 MOCK | RabbitMQ publisher | 1 día |
| Logging | ✅ Real | shared/logger | Completo |

**Total estimado para implementación completa:** ~12-15 días

---

### 🐛 Bugs y Anti-patrones Detectados

**BUG-001: Sin manejo de duplicados en MongoDB**
```go
// material_uploaded_processor.go:72
_, err = summaryCollection.InsertOne(ctx, summary)
if err != nil {
    return err
}
```

**Problema:** Si el material ya tiene un summary (ej: reproceso), `InsertOne` fallará por violación de unique index en `material_id`.

**Solución:**
```go
// Usar upsert
filter := bson.M{"material_id": event.MaterialID}
update := bson.M{"$set": summary}
opts := options.Update().SetUpsert(true)
_, err = summaryCollection.UpdateOne(ctx, filter, update, opts)
```

---

**ANTI-PATTERN-001: God Function**

La función `Process()` tiene **110 líneas** y hace demasiadas cosas:
1. Validación
2. Transacción PostgreSQL
3. Extracción PDF
4. Llamada OpenAI (×2)
5. MongoDB (×2)
6. Actualización estado

**Recomendación:** Dividir en funciones más pequeñas:
- `extractPDFText(s3Key string) (string, error)`
- `generateSummary(text string) (*Summary, error)`
- `generateAssessment(text string) (*Assessment, error)`
- `saveSummary(summary *Summary) error`

---

**ANTI-PATTERN-002: bson.M en lugar de structs**

```go
summary := bson.M{
    "material_id":  event.MaterialID,
    "main_ideas":   []string{"Idea 1", "Idea 2"},
}
```

**Problema:**
- Sin type-safety
- Propenso a errores de typos
- Dificulta refactoring

**Solución:**
```go
type MaterialSummary struct {
    ID          primitive.ObjectID `bson:"_id,omitempty"`
    MaterialID  string             `bson:"material_id"`
    Summary     string             `bson:"summary"`
    KeyPoints   []string           `bson:"key_points"`
    Language    string             `bson:"language"`
    // ... más campos
}

summary := &MaterialSummary{
    MaterialID: event.MaterialID,
    Summary:    generatedSummary,
    // ...
}
```

---

## 4. Otros Procesadores

### MaterialReprocessProcessor (✅ Funcionando como wrapper)

```go
// material_reprocess_processor.go:22-27
func (p *MaterialReprocessProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    p.logger.Info("reprocessing material", "material_id", event.MaterialID)
    return p.uploadedProcessor.Process(ctx, event)
}
```

✅ **Correcto**: Delega a MaterialUploadedProcessor (DRY principle)

---

### MaterialDeletedProcessor (✅ Implementado, ⚠️ Sin error handling robusto)

```go
// material_deleted_processor.go:25-46
func (p *MaterialDeletedProcessor) Process(ctx context.Context, event dto.MaterialDeletedEvent) error {
    // Eliminar summary
    summaryCol := p.mongodb.Collection("material_summaries")
    _, err := summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
    if err != nil {
        p.logger.Error("failed to delete summary", "error", err)
    }

    // Eliminar assessment
    assessmentCol := p.mongodb.Collection("material_assessments")
    _, err = assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
    if err != nil {
        p.logger.Error("failed to delete assessment", "error", err)
    }

    return nil
}
```

⚠️ **Problemas:**
1. **Ignora errores**: Loguea pero no retorna error
2. **Sin transacción**: Si `summary` se elimina pero `assessment` falla, queda inconsistencia
3. **No elimina de S3**: Debería eliminar PDF de S3 también

---

### AssessmentAttemptProcessor (🔴 Vacío - Solo logging)

```go
// assessment_attempt_processor.go:18-32
func (p *AssessmentAttemptProcessor) Process(ctx context.Context, event dto.AssessmentAttemptEvent) error {
    p.logger.Info("processing assessment attempt",
        "material_id", event.MaterialID,
        "user_id", event.UserID,
        "score", event.Score,
    )

    // Comentarios sugieren funcionalidad futura
    // - Enviar notificación al docente si score bajo
    // - Actualizar estadísticas
    // - Registrar en tabla de analytics

    return nil
}
```

❌ **Sin funcionalidad**: Solo loguea el evento

---

### StudentEnrolledProcessor (🔴 Vacío - Solo logging)

```go
// student_enrolled_processor.go:18-31
func (p *StudentEnrolledProcessor) Process(ctx context.Context, event dto.StudentEnrolledEvent) error {
    p.logger.Info("processing student enrolled",
        "student_id", event.StudentID,
        "unit_id", event.UnitID,
    )

    // Comentarios sugieren funcionalidad futura
    // - Enviar email de bienvenida
    // - Crear registro de onboarding
    // - Notificar al teacher

    return nil
}
```

❌ **Sin funcionalidad**: Solo loguea el evento

---

## 📋 Inventario de Funcionalidad

### ✅ Implementado y Funcionando

| Componente | Ubicación | Estado | Notas |
|------------|-----------|--------|-------|
| Bootstrap con shared | `internal/bootstrap/` | ✅ | v0.7.0 integrado correctamente |
| Configuración | `internal/config/` | ✅ | Viper + mapstructure |
| RabbitMQ Connection | `internal/bootstrap/bridge.go:44` | ✅ | Usando shared factory |
| RabbitMQ Consumer | `cmd/main.go:44-56` | ✅ | Escuchando cola |
| Event Router | `internal/infrastructure/messaging/consumer/` | ✅ | Implementado pero no usado |
| Logging estructurado | Todos los processors | ✅ | shared/logger v0.7.0 |
| PostgreSQL connection | `internal/bootstrap/bridge.go:42` | ✅ | Usando shared factory |
| PostgreSQL transactions | `material_uploaded_processor.go:44` | ✅ | shared/database/postgres |
| MongoDB connection | `internal/bootstrap/bridge.go:43` | ✅ | Usando shared factory |
| Graceful shutdown | `cmd/main.go:74-78` | ✅ | SIGINT/SIGTERM |
| Event DTOs | `internal/application/dto/` | ✅ | Bien estructurados |
| Value Objects | `internal/domain/valueobject/` | ✅ | MaterialID implementado |
| Dependency Injection | `internal/container/` | ✅ | Constructor injection |

---

### ⚠️ Parcialmente Implementado (MOCK)

| Componente | Estado Real | Estado MOCK | Ubicación | Estimación Implementación |
|------------|-------------|-------------|-----------|---------------------------|
| PDF Text Extraction | 0% | 100% | `material_uploaded_processor.go:56` | 2-3 días |
| OpenAI Client | 0% | 100% | `material_uploaded_processor.go:59,78` | 2-3 días |
| MongoDB Repositories | 0% | 100% | `material_uploaded_processor.go:62-100` | 2-3 días |
| S3 Client | 0% | 0% | No existe | 1-2 días |
| Event Publisher | 0% | 0% | No existe | 1 día |
| Material Summary Save | 20% | 80% | `material_uploaded_processor.go:62-75` | 1-2 días |
| Material Assessment Save | 20% | 80% | `material_uploaded_processor.go:80-100` | 1-2 días |
| Material Deletion | 70% | 30% | `material_deleted_processor.go:25-46` | 1 día |
| Assessment Attempt Processing | 0% | 100% | `assessment_attempt_processor.go:18-32` | 3-5 días |
| Student Enrolled Processing | 0% | 100% | `student_enrolled_processor.go:18-31` | 3-5 días |

**Total estimado para completar:** ~20-30 días

---

### ❌ No Implementado

| Componente | Prioridad | Sprint Sugerido | Estimación |
|------------|-----------|-----------------|------------|
| Tests de integración completos | MEDIA | Sprint-05 | 5-7 días |
| CI/CD pipeline | BAJA | Sprint-06 | 2-3 días |
| Monitoring/Metrics (Prometheus) | MEDIA | Sprint-06 | 3-5 días |
| Rate limiting OpenAI | ALTA | Sprint-03 | 1-2 días |
| Retry con exponential backoff | ALTA | Sprint-02 | 1-2 días |
| Circuit breaker pattern | MEDIA | Sprint-04 | 2-3 días |
| Distributed tracing | BAJA | Sprint-06 | 3-5 días |
| Event auditing completo | MEDIA | Sprint-02 | 1-2 días |

---

## 🎯 Gaps Críticos Identificados

---

### GAP-001: MongoDB Persistence Layer (🔴 ALTA Prioridad)

**Severidad:** 🔴 CRÍTICA
**Impacto:** Sin MongoDB repositories, el worker no puede guardar resultados reales

#### Componentes Afectados

1. **material_summary repository** - No existe
2. **material_assessment repository** - No existe
3. **material_event repository** - No existe

#### Problema Actual

```go
// Código actual en material_uploaded_processor.go:62-75
summaryCollection := p.mongodb.Collection("material_summaries")
summary := bson.M{
    "material_id":  event.MaterialID,
    "main_ideas":   []string{"Idea 1", "Idea 2", "Idea 3"},  // ❌ Hardcoded
    "key_concepts": bson.M{"concept1": "definition1"},       // ❌ Hardcoded
}
_, err = summaryCollection.InsertOne(ctx, summary)
```

**Problemas:**
- Sin type-safety (usa `bson.M`)
- Sin validation de schemas
- Sin manejo de duplicados
- Sin separación de responsabilidades (repository pattern)

#### Solución Propuesta

**1. Crear structs tipados:**

```go
// internal/domain/entity/material_summary.go
type MaterialSummary struct {
    ID               primitive.ObjectID `bson:"_id,omitempty"`
    MaterialID       string             `bson:"material_id"`
    Summary          string             `bson:"summary"`
    KeyPoints        []string           `bson:"key_points"`
    Language         string             `bson:"language"`
    WordCount        int                `bson:"word_count"`
    Version          int                `bson:"version"`
    AIModel          string             `bson:"ai_model"`
    ProcessingTimeMS int                `bson:"processing_time_ms"`
    CreatedAt        time.Time          `bson:"created_at"`
    UpdatedAt        time.Time          `bson:"updated_at"`
}
```

**2. Crear repository interface:**

```go
// internal/domain/repository/material_summary_repository.go
type MaterialSummaryRepository interface {
    Save(ctx context.Context, summary *entity.MaterialSummary) error
    FindByMaterialID(ctx context.Context, materialID string) (*entity.MaterialSummary, error)
    Delete(ctx context.Context, materialID string) error
}
```

**3. Implementar repository:**

```go
// internal/infrastructure/persistence/mongodb/material_summary_repository.go
type materialSummaryRepository struct {
    db         *mongo.Database
    collection *mongo.Collection
}

func (r *materialSummaryRepository) Save(ctx context.Context, summary *entity.MaterialSummary) error {
    filter := bson.M{"material_id": summary.MaterialID}
    update := bson.M{"$set": summary}
    opts := options.Update().SetUpsert(true)

    _, err := r.collection.UpdateOne(ctx, filter, update, opts)
    return err
}
```

#### Estimación

- **Tiempo:** 2-3 días
- **Sprint sugerido:** Sprint-02
- **Dependencias:** MONGODB_SCHEMA.md (✅ completado en este sprint)

---

### GAP-002: PDF Text Extraction (🔴 ALTA Prioridad)

**Severidad:** 🔴 CRÍTICA
**Impacto:** Sin extracción de PDF, no hay texto para procesar con IA

#### Componente Afectado

`internal/infrastructure/pdf/` - No existe

#### Problema Actual

```go
// material_uploaded_processor.go:55-56
// MOCK - No implementado
p.logger.Debug("extracting PDF text", "s3_key", event.S3Key)
```

#### Solución Propuesta

**Biblioteca recomendada:** `github.com/pdfcpu/pdfcpu` o `github.com/unidoc/unipdf`

**Implementación:**

```go
// internal/infrastructure/pdf/extractor.go
type Extractor interface {
    Extract(ctx context.Context, pdfPath string) (string, error)
}

type pdfcpuExtractor struct {
    logger logger.Logger
}

func (e *pdfcpuExtractor) Extract(ctx context.Context, pdfPath string) (string, error) {
    // 1. Descargar PDF de S3
    // 2. Extraer texto con pdfcpu
    // 3. Limpiar y normalizar texto
    // 4. Retornar texto extraído
}
```

#### Estimación

- **Tiempo:** 2-3 días
- **Sprint sugerido:** Sprint-02
- **Complejidad:** Media (bibliotecas maduras disponibles)

---

### GAP-003: OpenAI Integration (🔴 ALTA Prioridad)

**Severidad:** 🔴 CRÍTICA
**Impacto:** Sin OpenAI, no hay generación de resúmenes ni quizzes

#### Componente Afectado

`internal/infrastructure/nlp/` - No existe

#### Problema Actual

```go
// material_uploaded_processor.go:58-59, 77-78
// MOCK - No implementado
p.logger.Debug("generating summary with AI")
p.logger.Debug("generating quiz with AI")
```

#### Solución Propuesta

**Biblioteca recomendada:** `github.com/sashabaranov/go-openai`

**Implementación:**

```go
// internal/infrastructure/nlp/openai_client.go
type NLPClient interface {
    GenerateSummary(ctx context.Context, text string, language string) (*Summary, error)
    GenerateAssessment(ctx context.Context, text string, questionCount int) (*Assessment, error)
}

type openAIClient struct {
    client *openai.Client
    model  string
    logger logger.Logger
}

func (c *openAIClient) GenerateSummary(ctx context.Context, text string, language string) (*Summary, error) {
    prompt := fmt.Sprintf("Generate a summary in %s of the following text:\n\n%s", language, text)

    resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: c.model,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are an educational content summarizer.",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: prompt,
            },
        },
        Temperature: 0.7,
    })

    // Parse response y retornar Summary
}
```

#### Consideraciones Importantes

**1. Rate Limiting:**
- OpenAI tiene límites de rate (ej: 3,500 requests/min para GPT-4)
- Implementar rate limiter: `golang.org/x/time/rate`

**2. Retry Logic:**
- Implementar exponential backoff para errores transitorios
- Biblioteca recomendada: `github.com/cenkalti/backoff`

**3. Cost Management:**
- Loguear token usage en MongoDB (`material_summary.token_usage`)
- Implementar límite de tokens por request

**4. Prompt Engineering:**
- Usar prompts estructurados para resultados consistentes
- Versionar prompts para A/B testing

#### Estimación

- **Tiempo:** 3-4 días (incluyendo rate limiting y retry logic)
- **Sprint sugerido:** Sprint-03
- **Complejidad:** Media-Alta

---

### GAP-004: Event Publishing (⚠️ MEDIA Prioridad)

**Severidad:** ⚠️ MEDIA
**Impacto:** Sin publicación de eventos, otros servicios no son notificados

#### Componente Afectado

`internal/infrastructure/messaging/publisher/` - No existe

#### Problema Actual

```go
// material_uploaded_processor.go no publica eventos al completar
```

#### Solución Propuesta

```go
// internal/infrastructure/messaging/publisher/event_publisher.go
type EventPublisher interface {
    PublishMaterialProcessed(ctx context.Context, event MaterialProcessedEvent) error
}

type rabbitMQPublisher struct {
    channel  *amqp.Channel
    exchange string
    logger   logger.Logger
}

func (p *rabbitMQPublisher) PublishMaterialProcessed(ctx context.Context, event MaterialProcessedEvent) error {
    body, err := json.Marshal(event)
    if err != nil {
        return err
    }

    return p.channel.PublishWithContext(ctx,
        p.exchange,
        "material.processed", // routing key
        false,
        false,
        amqp.Publishing{
            ContentType: "application/json",
            Body:        body,
        },
    )
}
```

#### Estimación

- **Tiempo:** 1-2 días
- **Sprint sugerido:** Sprint-03
- **Complejidad:** Baja

---

### GAP-005: S3 Client (⚠️ MEDIA Prioridad)

**Severidad:** ⚠️ MEDIA
**Impacto:** No puede descargar PDFs de S3

#### Componente Afectado

`internal/infrastructure/storage/` - No existe

#### Solución Propuesta

**Biblioteca:** AWS SDK v2 (`github.com/aws/aws-sdk-go-v2/service/s3`)

```go
// internal/infrastructure/storage/s3_client.go
type StorageClient interface {
    Download(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}
```

#### Estimación

- **Tiempo:** 1-2 días
- **Sprint sugerido:** Sprint-02
- **Complejidad:** Baja (SDK maduro)

---

## 📊 Análisis de Patrones y Anti-patrones

### ✅ Patrones Bien Implementados

#### 1. Repository Pattern (Diseño)

Aunque no implementado completamente, la estructura está preparada:

```
internal/
├── domain/
│   └── repository/      # Interfaces de repositories (pendiente)
└── infrastructure/
    └── persistence/
        └── mongodb/      # Implementaciones (pendiente)
```

✅ **Correcto**: Separación de interfaces (domain) e implementación (infrastructure)

---

#### 2. Dependency Injection (Container Pattern)

```go
// internal/container/container.go
func NewContainer(db *sql.DB, mongodb *mongo.Database, logger logger.Logger) *Container {
    // Constructor injection
}
```

✅ **Correcto**: Todas las dependencias se inyectan, facilitando testing

---

#### 3. Value Objects

```go
type MaterialID struct {
    value types.UUID
}

func MaterialIDFromString(s string) (MaterialID, error) {
    uuid, err := types.ParseUUID(s)
    if err != nil {
        return MaterialID{}, err
    }
    return MaterialID{value: uuid}, nil
}
```

✅ **Correcto**: Encapsulación + validación en constructor

---

#### 4. Transaction Pattern (usando shared)

```go
err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
    // Operaciones dentro de transacción
    return err
})
```

✅ **Correcto**: Usa transacciones de shared/database/postgres

---

### ⚠️ Anti-patrones Detectados

#### ANTI-PATTERN-001: God Function

**Ubicación:** `material_uploaded_processor.go:32-119`

**Problema:**
```go
func (p *MaterialUploadedProcessor) Process(...) error {
    // 1. Validación (5 líneas)
    // 2. Transacción PostgreSQL inicio (40 líneas)
    //    2.1 UPDATE status processing
    //    2.2 Extracción PDF MOCK
    //    2.3 OpenAI summary MOCK
    //    2.4 MongoDB summary MOCK
    //    2.5 OpenAI quiz MOCK
    //    2.6 MongoDB assessment MOCK
    //    2.7 UPDATE status completed
    // 3. Error handling (5 líneas)
}
```

**Complejidad ciclomática:** ~15 (alta)

**Solución:**

```go
func (p *MaterialUploadedProcessor) Process(...) error {
    materialID, err := p.validateMaterialID(event.MaterialID)
    if err != nil {
        return err
    }

    return postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
        if err := p.updateStatus(ctx, tx, materialID, "processing"); err != nil {
            return err
        }

        text, err := p.pdfExtractor.Extract(ctx, event.S3Key)
        if err != nil {
            return err
        }

        summary, err := p.nlpClient.GenerateSummary(ctx, text, event.PreferredLanguage)
        if err != nil {
            return err
        }

        if err := p.summaryRepo.Save(ctx, summary); err != nil {
            return err
        }

        // ... más operaciones modulares

        return p.updateStatus(ctx, tx, materialID, "completed")
    })
}
```

**Beneficios:**
- Funciones más pequeñas y testeables
- Responsabilidad única
- Mejor legibilidad

---

#### ANTI-PATTERN-002: Primitive Obsession (bson.M)

**Ubicación:** Múltiples lugares en processors

**Problema:**
```go
summary := bson.M{
    "material_id":  event.MaterialID,
    "main_ideas":   []string{"Idea 1"},
    "key_concepts": bson.M{"concept1": "definition1"},
}
```

**Issues:**
- Sin type-safety
- Typos no detectados en compile-time
- Dificulta refactoring
- No se puede usar para generar documentación

**Solución:** Usar structs tipados (ya documentado en GAP-001)

---

#### ANTI-PATTERN-003: Error Swallowing

**Ubicación:** `material_deleted_processor.go:31-42`

**Problema:**
```go
_, err := summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
if err != nil {
    p.logger.Error("failed to delete summary", "error", err)
    // ❌ No retorna error, continúa ejecutando
}

_, err = assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
if err != nil {
    p.logger.Error("failed to delete assessment", "error", err)
    // ❌ No retorna error
}

return nil  // ❌ Siempre retorna nil
```

**Impacto:**
- El caller no sabe que hubo errores
- Posible inconsistencia de datos
- Debugging difícil

**Solución:**

```go
// Opción 1: Retornar primer error
if err := p.deleteSummary(ctx, materialID); err != nil {
    return fmt.Errorf("failed to delete summary: %w", err)
}

// Opción 2: Acumular errores
var errs []error
if err := p.deleteSummary(ctx, materialID); err != nil {
    errs = append(errs, err)
}
if err := p.deleteAssessment(ctx, materialID); err != nil {
    errs = append(errs, err)
}
if len(errs) > 0 {
    return fmt.Errorf("failed to delete material data: %v", errs)
}
```

---

#### ANTI-PATTERN-004: Missing Context Cancellation

**Ubicación:** Varios processors

**Problema:**
```go
func (p *MaterialUploadedProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    // ❌ No verifica si context fue cancelado antes de operaciones costosas

    text := extractPDF()  // Operación larga

    // ❌ No propaga context a operaciones I/O
    summary := generateSummary(text)  // Debería usar ctx
}
```

**Impacto:**
- Operaciones pueden continuar después de timeout
- Goroutines pueden quedar colgadas
- Desperdicio de recursos

**Solución:**
```go
func (p *MaterialUploadedProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    // Verificar cancelación antes de operaciones costosas
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Propagar context a todas las operaciones
    text, err := p.pdfExtractor.Extract(ctx, event.S3Key)
    if err != nil {
        return err
    }

    summary, err := p.nlpClient.GenerateSummary(ctx, text, event.Language)
    if err != nil {
        return err
    }
}
```

---

#### ANTI-PATTERN-005: No Error Wrapping

**Ubicación:** Varios lugares

**Problema:**
```go
if err != nil {
    return err  // ❌ Pierde contexto de dónde ocurrió el error
}
```

**Solución:**
```go
if err != nil {
    return fmt.Errorf("failed to extract PDF text from %s: %w", s3Key, err)
}
```

O mejor, usar shared/common/errors:
```go
if err != nil {
    return errors.NewInternalError("failed to extract PDF text", err)
}
```

---

## 🔐 Análisis de Seguridad

### ✅ Fortalezas

1. **No hay secrets hardcodeados**: Todas las credenciales vienen de variables de entorno
2. **Variables de entorno para configuración**: `config/loader.go` usa Viper
3. **SQL Injection protección**: Usa prepared statements

```go
tx.ExecContext(ctx,
    "UPDATE materials SET processing_status = $1 WHERE id = $2",  // ✅ Parametrizado
    status, materialID,
)
```

---

### ⚠️ Vulnerabilidades Potenciales

#### VULN-001: Sin validación de entrada robusta

**Problema:**
```go
// event_consumer.go:48
if err := json.Unmarshal(body, &baseEvent); err != nil {
    c.logger.Error("failed to parse event", "error", err)
    return err
}
// ❌ No valida estructura del evento antes de procesar
```

**Riesgo:** Eventos malformados pueden causar panics

**Solución:**
```go
if err := json.Unmarshal(body, &baseEvent); err != nil {
    return errors.NewValidationError("invalid event format")
}

if baseEvent.EventType == "" {
    return errors.NewValidationError("event_type is required")
}
```

---

#### VULN-002: Sin rate limiting

**Problema:** No hay límites de rate para:
- Consumo de mensajes de RabbitMQ
- Llamadas a OpenAI (cuando se implemente)

**Riesgo:**
- DDoS via message flooding
- Costos elevados de OpenAI
- Agotamiento de recursos

**Solución:**
```go
import "golang.org/x/time/rate"

type rateLimitedProcessor struct {
    processor Processor
    limiter   *rate.Limiter
}

func (p *rateLimitedProcessor) Process(ctx context.Context, event Event) error {
    if err := p.limiter.Wait(ctx); err != nil {
        return err
    }
    return p.processor.Process(ctx, event)
}
```

---

#### VULN-003: Sin timeout en operaciones I/O

**Problema:**
```go
// material_uploaded_processor.go no usa timeouts
err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
    // ❌ Sin timeout, transacción puede quedar colgada indefinidamente
})
```

**Solución:**
```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
    // ...
})
```

---

#### VULN-004: Sin sanitización de logs

**Problema:**
```go
p.logger.Info("processing material uploaded",
    "material_id", event.MaterialID,
    "s3_key", event.S3Key,  // ❌ Puede contener datos sensibles en path
)
```

**Riesgo:** Logs pueden exponer información sensible

**Solución:**
```go
sanitizedKey := sanitizeS3Key(event.S3Key)  // Remover datos sensibles
p.logger.Info("processing material uploaded",
    "material_id", event.MaterialID,
    "s3_key", sanitizedKey,
)
```

---

## 📊 Métricas de Código

### Estadísticas Generales

| Métrica | Valor |
|---------|-------|
| Total archivos Go | 19 |
| Total líneas de código | ~1,500 (estimado) |
| Complejidad ciclomática promedio | Media (8-12) |
| Cobertura de tests | ~10% (estimado) |
| Deuda técnica | Media (MOCKs) |

---

### Distribución de Código por Capa

```
internal/
├── domain/              ~100 líneas (5%)
├── application/         ~600 líneas (40%)
├── infrastructure/      ~300 líneas (20%)
├── bootstrap/           ~300 líneas (20%)
├── config/              ~150 líneas (10%)
└── container/           ~50 líneas (5%)
```

---

### Complejidad por Archivo

| Archivo | Líneas | Complejidad | Comentario |
|---------|--------|-------------|------------|
| `material_uploaded_processor.go` | ~120 | Alta (15) | God function |
| `event_consumer.go` | ~97 | Baja (3) | Bien estructurado |
| `bridge.go` | ~123 | Media (8) | Lógica de bootstrap |
| `main.go` | ~144 | Media (7) | Setup + consumer |
| `container.go` | ~63 | Baja (1) | Simple DI |

---

### Deuda Técnica

**Total estimado:** ~15-20 días de desarrollo

**Desglose:**
- MOCKs a implementar: ~12-15 días
- Refactoring: ~2-3 días
- Tests: ~3-5 días

---

## 🚀 Recomendaciones

### Inmediatas (Sprint-02)

**PRIO-001: Implementar MongoDB Repositories**
- **Tiempo:** 2-3 días
- **Impacto:** Alto
- **Riesgo:** Bajo
- **Dependencias:** MONGODB_SCHEMA.md ✅

**PRIO-002: Implementar PDF Extractor**
- **Tiempo:** 2-3 días
- **Impacto:** Alto
- **Riesgo:** Bajo (bibliotecas maduras)
- **Biblioteca:** `github.com/pdfcpu/pdfcpu`

**PRIO-003: Implementar S3 Client**
- **Tiempo:** 1-2 días
- **Impacto:** Alto
- **Riesgo:** Bajo
- **Biblioteca:** AWS SDK v2

**PRIO-004: Conectar EventConsumer en main.go**
- **Tiempo:** 1 hora
- **Impacto:** Medio
- **Riesgo:** Muy bajo

---

### Corto Plazo (Sprint-03-04)

**PRIO-005: Implementar OpenAI Client**
- **Tiempo:** 3-4 días (con rate limiting)
- **Impacto:** Crítico
- **Riesgo:** Medio (API externa, costos)
- **Incluir:**
  - Rate limiter
  - Exponential backoff
  - Token usage tracking

**PRIO-006: Implementar Event Publisher**
- **Tiempo:** 1-2 días
- **Impacto:** Medio
- **Riesgo:** Bajo

**PRIO-007: Refactoring de MaterialUploadedProcessor**
- **Tiempo:** 1-2 días
- **Impacto:** Medio (mantenibilidad)
- **Riesgo:** Bajo

**PRIO-008: Agregar Error Wrapping consistente**
- **Tiempo:** 1 día
- **Impacto:** Medio (debugging)
- **Riesgo:** Muy bajo

---

### Mediano Plazo (Sprint-05-06)

**PRIO-009: Tests de Integración**
- **Tiempo:** 5-7 días
- **Impacto:** Alto (calidad)
- **Riesgo:** Bajo
- **Incluir:**
  - Testcontainers para RabbitMQ/MongoDB
  - Tests end-to-end de procesadores
  - Tests de event routing

**PRIO-010: CI/CD Pipeline**
- **Tiempo:** 2-3 días
- **Impacto:** Medio
- **Riesgo:** Bajo
- **Incluir:**
  - GitHub Actions
  - Linting (golangci-lint)
  - Tests automáticos
  - Build de Docker image

**PRIO-011: Observability**
- **Tiempo:** 3-5 días
- **Impacto:** Medio
- **Riesgo:** Bajo
- **Incluir:**
  - Prometheus metrics
  - Distributed tracing (OpenTelemetry)
  - Dashboards (Grafana)

---

## 📋 Conclusiones

### Fortalezas del Proyecto

1. ✅ **Arquitectura sólida y escalable**
   - Clean Architecture bien aplicada
   - Capas claramente separadas
   - Preparado para crecer

2. ✅ **Integración moderna con shared**
   - Usa shared/bootstrap v0.7.0 correctamente
   - Lifecycle management implementado
   - Configuración centralizada

3. ✅ **Base técnica robusta**
   - RabbitMQ consumer funcionando
   - PostgreSQL con transacciones
   - MongoDB conectado
   - Graceful shutdown

4. ✅ **Código bien estructurado**
   - Dependency Injection correcto
   - Value Objects implementados
   - Logging estructurado

---

### Debilidades del Proyecto

1. ⚠️ **Alta proporción de código MOCK**
   - ~70% de funcionalidad sin implementar
   - Bloquea puesta en producción

2. ⚠️ **Capa de dominio vacía**
   - No hay entidades de dominio
   - Lógica de negocio en processors

3. ⚠️ **Falta infraestructura crítica**
   - Sin cliente OpenAI (core del negocio)
   - Sin extractor PDF
   - Sin cliente S3
   - MongoDB repositories sin implementar

4. ⚠️ **Baja cobertura de tests**
   - ~10% de cobertura estimada
   - Sin tests de integración

5. ⚠️ **Anti-patrones detectados**
   - God functions
   - bson.M en lugar de structs
   - Error swallowing
   - Sin rate limiting

---

### Próximos Pasos Críticos

**Para Sprint-02:**

1. **Completar MongoDB Repositories** (2-3 días)
   - Usar schemas diseñados en este sprint
   - Implementar pattern repository completo
   - Agregar tests unitarios

2. **Implementar PDF Extractor** (2-3 días)
   - Integrar biblioteca pdfcpu
   - Conectar con S3 client
   - Manejar errores de PDFs corruptos

3. **Implementar S3 Client** (1-2 días)
   - AWS SDK v2
   - Download/Delete operations
   - Error handling robusto

4. **Conectar Event Router** (1 hora)
   - Usar `EventConsumer.RouteEvent()` en main.go
   - Eliminar procesamiento MOCK

**Total Sprint-02:** ~7-10 días

---

**Para Sprint-03:**

1. **Implementar OpenAI Client completo** (3-4 días)
   - Cliente GPT-4
   - Rate limiting
   - Exponential backoff
   - Token tracking

2. **Event Publisher** (1-2 días)
3. **Refactoring de processors** (1-2 días)

**Total Sprint-03:** ~5-8 días

---

### Roadmap Sugerido

```
Sprint-01 ✅ COMPLETADO
  - Auditoría del código
  - Diseño de schemas MongoDB
  - Scripts de inicialización

Sprint-02 (2 semanas)
  - MongoDB repositories
  - PDF extractor
  - S3 client
  - Conectar event router

Sprint-03 (2 semanas)
  - OpenAI client
  - Event publisher
  - Rate limiting
  - Refactoring

Sprint-04 (2 semanas)
  - Retry logic con backoff
  - Circuit breaker
  - Error handling mejorado

Sprint-05 (2 semanas)
  - Tests de integración
  - Tests end-to-end
  - Testcontainers

Sprint-06 (2 semanas)
  - CI/CD pipeline
  - Monitoring (Prometheus)
  - Distributed tracing
  - Documentación completa
```

---

## 📚 Referencias

- [Clean Architecture (Robert C. Martin)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [edugo-shared v0.7.0](https://github.com/EduGoGroup/edugo-shared)
- [MongoDB Go Driver](https://www.mongodb.com/docs/drivers/go/current/)
- [RabbitMQ Best Practices](https://www.rabbitmq.com/best-practices.html)
- [Go Best Practices](https://github.com/golang/go/wiki/CodeReviewComments)

---

**Fin de Auditoría**

> **Auditor:** Claude Code Web
> **Fecha:** 2025-11-18
> **Sprint:** Sprint-01 Fase 1
> **Próximo paso:** Ejecutar scripts MongoDB en Fase 2 (Claude Code Local)
> **Estado:** ✅ COMPLETADO
