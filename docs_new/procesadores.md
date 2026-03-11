# Procesadores — EduGo Worker

> Documentación generada desde análisis directo del código fuente (marzo 2026).

---

## 1. Interfaz Processor

Todos los procesadores implementan esta interfaz mínima:

```go
// internal/application/processor/processor.go
type Processor interface {
    EventType() string
    Process(ctx context.Context, payload []byte) error
}
```

- `EventType()` retorna el string que identifica al processor (ej: `"material_uploaded"`)
- `Process()` recibe el payload JSON raw del mensaje RabbitMQ

---

## 2. ProcessorRegistry

**Archivo:** `internal/application/processor/registry.go`

El registry es un mapa `event_type → Processor` que rutea mensajes automáticamente.

```go
type Registry struct {
    processors map[string]Processor
    logger     logger.Logger
}
```

### Flujo de routing

```
Process(ctx, payload)
    │
    ├─ json.Unmarshal → extraer event_type
    │
    ├─ Buscar en map[event_type]
    │   ├─ Encontrado → processor.Process(ctx, payload)
    │   └─ No encontrado → WARN log, return nil (no error)
    │
    └─ Retorna error del processor o nil
```

**Comportamiento clave:** Si llega un `event_type` sin processor registrado, solo loguea WARNING y retorna `nil` — no hace NACK del mensaje.

### Registro en bootstrap

```go
// internal/bootstrap/resource_builder.go → WithProcessors()
registry := processor.NewRegistry(b.logger)
registry.Register(materialUploadedProc)
registry.Register(processor.NewMaterialReprocessProcessor(materialUploadedProc, b.logger))
registry.Register(materialDeletedProc)
registry.Register(assessmentAttemptProc)
registry.Register(studentEnrolledProc)
```

---

## 3. Tabla de Processors

| Processor | EventType | Estado | Dependencias | Descripción |
|-----------|-----------|--------|--------------|-------------|
| `MaterialUploadedProcessor` | `material_uploaded` | ✅ Completo | DB, MongoDB, S3, PDF, NLP | Procesa PDF: extrae texto, genera resumen + quiz con IA |
| `MaterialDeletedProcessor` | `material_deleted` | ✅ Completo | MongoDB | Elimina resumen y assessment de MongoDB |
| `MaterialReprocessProcessor` | `material_reprocess` | ✅ Completo | (wrapper) | Delega a MaterialUploadedProcessor |
| `AssessmentAttemptProcessor` | `assessment_attempt` | ⚠️ Stub | Logger | Solo loguea, sin lógica de negocio |
| `StudentEnrolledProcessor` | `student_enrolled` | ⚠️ Stub | Logger | Solo loguea, sin lógica de negocio |

---

## 4. MaterialUploadedProcessor (✅ Completo)

**Archivo:** `internal/application/processor/material_uploaded_processor.go` (403 líneas)

### Dependencias

```go
type MaterialUploadedProcessorConfig struct {
    DB            *sql.DB          // PostgreSQL
    MongoDB       *mongo.Database  // MongoDB
    Logger        logger.Logger
    StorageClient storage.Client   // S3
    PDFExtractor  pdf.Extractor    // pdfcpu
    NLPClient     nlp.Client       // OpenAI
    AIModel       string           // "gpt-4-turbo-preview"
}
```

### Flujo paso a paso

```
1. Validar MaterialID (UUID)
     │ Error → metrics: validation_error, return
     │
2. UPDATE materials SET processing_status = 'processing'
     │ PostgreSQL
     │
3. Descargar PDF de S3 (con retry)
     │ storageClient.Download(ctx, s3Key)
     │ Retry: 3 intentos, backoff 500ms → 1s → 2s
     │ Error → status='failed', metrics: s3_error
     │
4. Extraer texto del PDF (con retry)
     │ pdfExtractor.ExtractWithMetadata(ctx, reader)
     │ Retorna: texto, pageCount, wordCount
     │ Error → status='failed', metrics: pdf_error
     │
5. Generar resumen con NLP (con retry)
     │ nlpClient.GenerateSummary(ctx, text)
     │ Retorna: MainIdeas, KeyConcepts, WordCount
     │ Error → status='failed', metrics: nlp_summary_error
     │
6. Generar quiz con NLP (con retry)
     │ nlpClient.GenerateQuiz(ctx, text, 10)  ← 10 preguntas
     │ Retorna: Questions[], cada una con options, correctAnswer, explanation
     │ Error → status='failed', metrics: nlp_quiz_error
     │
7. Guardar en MongoDB (dentro de transacción PostgreSQL)
     │ ├─ Insert MaterialSummary → collection material_summaries
     │ ├─ Insert MaterialAssessment → collection material_assessments
     │ └─ UPDATE materials SET processing_status = 'completed'
     │
8. Registrar métricas de éxito
```

### Collections MongoDB

| Collection | Documento | Campos clave |
|-----------|-----------|-------------|
| `material_summaries` | `MaterialSummary` | material_id, summary, key_points, language, word_count, ai_model, version |
| `material_assessments` | `MaterialAssessment` | material_id, questions[], total_questions, total_points, ai_model, version |

### Estados en PostgreSQL

| Estado | Significado |
|--------|-------------|
| `pending` | Material subido, pendiente de procesamiento |
| `processing` | Worker procesando (paso 2) |
| `completed` | Procesamiento exitoso (paso 7) |
| `failed` | Error en cualquier paso (3-7) |

---

## 5. MaterialDeletedProcessor (✅ Completo)

**Archivo:** `internal/application/processor/material_deleted_processor.go` (62 líneas)

Limpia datos de MongoDB cuando se elimina un material:

```go
func (p *MaterialDeletedProcessor) processEvent(ctx context.Context, event dto.MaterialDeletedEvent) error {
    // Eliminar summary
    summaryCol := p.mongodb.Collection(mongoentities.MaterialSummary{}.CollectionName())
    summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})

    // Eliminar assessment
    assessmentCol := p.mongodb.Collection(mongoentities.MaterialAssessment{}.CollectionName())
    assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})

    return nil
}
```

**Nota:** Los errores de delete se loguean pero no se propagan (return nil). El delete es best-effort.

---

## 6. MaterialReprocessProcessor (✅ Wrapper)

**Archivo:** `internal/application/processor/material_reprocess_processor.go` (44 líneas)

Simplemente delega al `MaterialUploadedProcessor`:

```go
func (p *MaterialReprocessProcessor) processEvent(ctx context.Context, event dto.MaterialUploadedEvent) error {
    p.logger.Info("reprocessing material", "material_id", event.GetMaterialID())
    return p.uploadedProcessor.processEvent(ctx, event)
}
```

Recibe el mismo payload que `material_uploaded` y ejecuta el mismo flujo completo.

---

## 7. AssessmentAttemptProcessor (⚠️ Stub)

**Archivo:** `internal/application/processor/assessment_attempt_processor.go` (47 líneas)

```go
func (p *AssessmentAttemptProcessor) processEvent(ctx context.Context, event dto.AssessmentAttemptEvent) error {
    p.logger.Info("processing assessment attempt",
        "material_id", event.MaterialID,
        "user_id", event.UserID,
        "score", event.Score,
    )

    // Aquí se podría:
    // - Enviar notificación al docente si score bajo
    // - Actualizar estadísticas
    // - Registrar en tabla de analytics

    return nil
}
```

### TODOs pendientes

- Enviar notificación al docente cuando el score es bajo
- Actualizar estadísticas de la evaluación (promedio, intentos)
- Registrar el intento en tabla de analytics

---

## 8. StudentEnrolledProcessor (⚠️ Stub)

**Archivo:** `internal/application/processor/student_enrolled_processor.go` (46 líneas)

```go
func (p *StudentEnrolledProcessor) processEvent(ctx context.Context, event dto.StudentEnrolledEvent) error {
    p.logger.Info("processing student enrolled",
        "student_id", event.StudentID,
        "unit_id", event.UnitID,
    )

    // Aquí se podría:
    // - Enviar email de bienvenida
    // - Crear registro de onboarding
    // - Notificar al teacher

    return nil
}
```

### TODOs pendientes

- Enviar email de bienvenida al estudiante
- Crear registro de onboarding (tracking de progreso inicial)
- Notificar al teacher de la nueva inscripción

---

## 9. Retry Logic

**Archivo:** `internal/application/processor/retry.go`

### Configuración por defecto

```go
const (
    maxRetries      = 3
    initialBackoff  = 500 * time.Millisecond
    maxBackoff      = 10 * time.Second
    backoffMultiple = 2.0
)
```

### Secuencia de backoff

```
Intento 1: ejecutar inmediatamente
Intento 2: esperar 500ms
Intento 3: esperar 1s
Intento 4: esperar 2s (si maxRetries > 3)
```

### Clasificación de errores

```go
func classifyError(err error) ErrorType {
    // Errores permanentes (no reintentar):
    // - PDF corrupto, escaneado, muy grande, vacío
    if errors.Is(err, pdfErrors.ErrPDFCorrupt) || ... {
        return ErrorTypePermanent
    }

    // Todo lo demás es transitorio (reintentar)
    return ErrorTypeTransient
}
```

| Tipo | Comportamiento |
|------|---------------|
| `ErrorTypePermanent` | No reintenta, retorna error inmediatamente |
| `ErrorTypeTransient` | Reintenta con backoff exponencial |

### Uso en processors

```go
err = WithRetry(ctx, DefaultRetryConfig(p.logger), func() error {
    pdfReader, downloadErr = p.storageClient.Download(ctx, event.GetS3Key())
    return downloadErr
})
```

---

## 10. Flujo ACK/NACK

**Archivo:** `cmd/main.go` (líneas 129-181)

```
Mensaje RabbitMQ
    │
    ├─ Rate limiter wait
    │   └─ Error/cancelación → NACK + requeue
    │
    ├─ processMessage(msg, resources, cfg)
    │   └─ ProcessorRegistry.Process(ctx, msg.Body)
    │
    ├─ Éxito → msg.Ack(false)
    │   └─ Mensaje eliminado de la cola
    │
    └─ Error → msg.Nack(false, true)
        └─ Mensaje devuelto a la cola (requeue=true)
            └─ Si excede reintentos → Dead Letter Queue (x-dead-letter-exchange)
```

**Auto-ack deshabilitado:** `Channel.Consume(..., false /* auto-ack */, ...)` — el worker controla explícitamente ACK/NACK.

---

## 11. Cómo Agregar un Nuevo Processor

### Paso 1: Crear el DTO del evento

```go
// internal/application/dto/event_dto.go
type MiNuevoEvent struct {
    EventType string    `json:"event_type"`
    // ... campos específicos
    Timestamp time.Time `json:"timestamp"`
}
```

### Paso 2: Agregar constante

```go
// internal/domain/constants/event_constants.go
const EventTypeMiNuevoEvento = "mi_nuevo_evento"
```

### Paso 3: Crear el processor

```go
// internal/application/processor/mi_nuevo_processor.go
package processor

type MiNuevoProcessor struct {
    logger logger.Logger
    // ... dependencias necesarias
}

func NewMiNuevoProcessor(logger logger.Logger) *MiNuevoProcessor {
    return &MiNuevoProcessor{logger: logger}
}

func (p *MiNuevoProcessor) EventType() string {
    return "mi_nuevo_evento"
}

func (p *MiNuevoProcessor) Process(ctx context.Context, payload []byte) error {
    var event dto.MiNuevoEvent
    if err := json.Unmarshal(payload, &event); err != nil {
        return errors.NewValidationError("invalid event payload")
    }
    return p.processEvent(ctx, event)
}

func (p *MiNuevoProcessor) processEvent(ctx context.Context, event dto.MiNuevoEvent) error {
    p.logger.Info("processing mi_nuevo_evento", ...)
    // ... lógica de negocio
    return nil
}
```

### Paso 4: Registrar en el ResourceBuilder

```go
// internal/bootstrap/resource_builder.go → WithProcessors()
miNuevoProc := processor.NewMiNuevoProcessor(b.logger)
registry.Register(miNuevoProc)
```

### Paso 5: (Opcional) Configurar rate limiting

```yaml
# config/config.yaml
rate_limiter:
  by_event_type:
    mi_nuevo_evento:
      requests_per_second: 10.0
      burst_size: 20.0
```

### Paso 6: Agregar tests

Crear `mi_nuevo_processor_test.go` siguiendo el patrón de `material_uploaded_processor_test.go`.

---

*Generado: marzo 2026 | Basado en análisis directo del código fuente*
