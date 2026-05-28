# Eventos — EduGo Worker

> Documentación generada desde análisis directo del código fuente (marzo 2026).

---

## 1. Arquitectura de Mensajería

El worker consume eventos de **RabbitMQ** usando un exchange de tipo `topic`. Los eventos son publicados por las APIs del ecosistema (principalmente API Mobile) y procesados asincrónicamente.

```
┌──────────────┐     ┌─────────────────────────┐     ┌──────────────────┐
│  API Mobile  │────▶│  Exchange: topic         │────▶│  Queue: durable  │
│  API Admin   │     │  "edugo.materials"       │     │  + DLQ           │
└──────────────┘     │                           │     └────────┬─────────┘
                     │  Routing: material.*      │              │
                     │           assessment.*    │     ┌────────▼─────────┐
                     │           student.*       │     │  EduGo Worker    │
                     └─────────────────────────┘     │  (consumer)      │
                                                      └──────────────────┘
```

---

## 2. Exchange y Routing Keys

| Exchange | Tipo | Durable | Routing Keys |
|----------|------|---------|-------------|
| `edugo.materials` | `topic` | Sí | `material.uploaded`, `material.deleted`, `material.reprocess`, `assessment.attempt`, `student.enrolled` |

### Constantes de Event Type

```go
// internal/domain/constants/event_constants.go
const (
    EventTypeMaterialUploaded  = "material_uploaded"
    EventTypeMaterialReprocess = "material_reprocess"
    EventTypeMaterialDeleted   = "material_deleted"
    EventTypeAssessmentAttempt = "assessment_attempt"
    EventTypeStudentEnrolled   = "student_enrolled"
    EventTypeStudentUnenrolled = "student_unenrolled"  // Definido pero sin processor
)
```

### Estados de Procesamiento

```go
const (
    EventStatusPending    = "pending"
    EventStatusProcessing = "processing"
    EventStatusCompleted  = "completed"
    EventStatusFailed     = "failed"
)
```

---

## 3. Queue Configuration

**Archivo:** `cmd/main.go` → `setupRabbitMQ()`

```go
// Queue declaration
ch.QueueDeclare(
    "edugo.material.uploaded",  // nombre
    true,                        // durable
    false,                       // delete when unused
    false,                       // exclusive
    false,                       // no-wait
    amqp.Table{
        "x-max-priority":         10,                // Soporte de prioridades
        "x-dead-letter-exchange": "edugo_dlq",       // Dead Letter Queue
    },
)
```

| Propiedad | Valor | Descripción |
|-----------|-------|-------------|
| Durable | `true` | Sobrevive reinicios del broker |
| Auto-delete | `false` | No se elimina cuando no hay consumers |
| Exclusive | `false` | Compartida entre consumers |
| Max Priority | `10` | Soporte para mensajes con prioridad 0-10 |
| DLQ Exchange | `edugo_dlq` | Mensajes que exceden reintentos van aquí |
| Prefetch Count | `5` | Configurable en `config.yaml` |
| Auto-ACK | `false` | ACK manual controlado por el worker |

### Queues configuradas

```yaml
# config/config.yaml
messaging:
  rabbitmq:
    queues:
      material_uploaded: "edugo.material.uploaded"
      assessment_attempt: "edugo.assessment.attempt"
    exchanges:
      materials: "edugo.materials"
    prefetch_count: 5
```

---

## 4. Schema JSON de Eventos

### MaterialUploadedEvent

**Archivo:** `internal/application/dto/event_dto.go`

```json
{
  "event_id": "uuid-v4",
  "event_type": "material_uploaded",
  "event_version": "1.0",
  "timestamp": "2026-03-08T10:00:00Z",
  "payload": {
    "material_id": "550e8400-e29b-41d4-a716-446655440000",
    "school_id": "school-uuid",
    "teacher_id": "teacher-uuid",
    "file_url": "s3://edugo-materials/materials/abc123.pdf",
    "file_size_bytes": 1048576,
    "file_type": "application/pdf",
    "metadata": {
      "s3_key": "materials/abc123.pdf"
    }
  }
}
```

**Struct Go:**

```go
type MaterialUploadedEvent struct {
    EventID      string                      `json:"event_id"`
    EventType    string                      `json:"event_type"`
    EventVersion string                      `json:"event_version"`
    Timestamp    time.Time                   `json:"timestamp"`
    Payload      MaterialUploadedPayload     `json:"payload"`
}

type MaterialUploadedPayload struct {
    MaterialID    string                 `json:"material_id"`
    SchoolID      string                 `json:"school_id"`
    TeacherID     string                 `json:"teacher_id"`
    FileURL       string                 `json:"file_url"`
    FileSizeBytes int64                  `json:"file_size_bytes"`
    FileType      string                 `json:"file_type"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
```

**Helpers:**
- `GetMaterialID()` → extrae `payload.material_id`
- `GetS3Key()` → extrae key S3 desde `metadata.s3_key` o parsea `file_url` (soporta `s3://`, URLs de S3)
- `GetAuthorID()` → mapea `teacher_id` a author

---

### MaterialDeletedEvent

```json
{
  "event_type": "material_deleted",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-03-08T10:00:00Z"
}
```

```go
type MaterialDeletedEvent struct {
    EventType  string    `json:"event_type"`
    MaterialID string    `json:"material_id"`
    Timestamp  time.Time `json:"timestamp"`
}
```

---

### AssessmentAttemptEvent

```json
{
  "event_type": "assessment_attempt",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "789e0123-e45b-67d8-a901-234567890123",
  "answers": {
    "q1": "b",
    "q2": "true",
    "q3": "respuesta libre"
  },
  "score": 85.5,
  "timestamp": "2026-03-08T10:00:00Z"
}
```

```go
type AssessmentAttemptEvent struct {
    EventType  string                 `json:"event_type"`
    MaterialID string                 `json:"material_id"`
    UserID     string                 `json:"user_id"`
    Answers    map[string]interface{} `json:"answers"`
    Score      float64                `json:"score"`
    Timestamp  time.Time              `json:"timestamp"`
}
```

---

### StudentEnrolledEvent

```json
{
  "event_type": "student_enrolled",
  "student_id": "student-uuid",
  "unit_id": "unit-uuid",
  "timestamp": "2026-03-08T10:00:00Z"
}
```

```go
type StudentEnrolledEvent struct {
    EventType string    `json:"event_type"`
    StudentID string    `json:"student_id"`
    UnitID    string    `json:"unit_id"`
    Timestamp time.Time `json:"timestamp"`
}
```

---

## 5. Rate Limiting por Event Type

**Archivo:** `config/config.yaml` → `rate_limiter`

```yaml
rate_limiter:
  enabled: true
  by_event_type:
    material.uploaded:
      requests_per_second: 5.0    # PDF + IA es costoso
      burst_size: 10.0
    material.updated:
      requests_per_second: 10.0
      burst_size: 20.0
    assessment.attempt:
      requests_per_second: 15.0   # Mayor throughput
      burst_size: 30.0
  default:
    requests_per_second: 10.0
    burst_size: 20.0
```

El rate limiter usa **token bucket** por event type. Si no hay configuración específica para un tipo, usa el `default`.

### Flujo en main.go

```go
// Extraer event type del routing key
eventType := m.RoutingKey

// Esperar token (bloquea si excede el rate)
if err := rateLimiter.Wait(consumerCtx, eventType); err != nil {
    // Cancelación → NACK + requeue
    m.Nack(false, true)
    return
}

// Registrar métricas
metrics.RecordRateLimiterWait(eventType, waitDuration)
metrics.RecordRateLimiterAllowed(eventType)
```

---

## 6. Dead Letter Queue

Mensajes que fallan repetidamente son enviados al exchange `edugo_dlq` (Dead Letter Queue):

```go
amqp.Table{
    "x-dead-letter-exchange": "edugo_dlq",
}
```

**Comportamiento:**
1. Mensaje falla → NACK con `requeue=true` → vuelve a la cola
2. Si RabbitMQ detecta que el mensaje excede el límite de reintentos, lo envía al DLQ
3. Los mensajes en DLQ pueden ser inspeccionados manualmente o reprocesados

---

## 7. Flujo Completo de un Mensaje

```
Producer (API Mobile)
    │
    ├─ Publish al exchange "edugo.materials"
    │   routing_key: "material.uploaded"
    │
    ▼
RabbitMQ Exchange (topic)
    │
    ├─ Routing: "material.uploaded" → Queue "edugo.material.uploaded"
    │
    ▼
Queue "edugo.material.uploaded" (durable, priority, DLQ)
    │
    ├─ Prefetch: 5 mensajes
    │
    ▼
Worker Consumer (goroutine por mensaje)
    │
    ├─ 1. Verificar contexto (shutdown check)
    │
    ├─ 2. Rate limiter wait (token bucket)
    │     └─ Timeout → NACK + requeue
    │
    ├─ 3. processMessage()
    │     │
    │     ├─ json.Unmarshal → extraer event_type
    │     │
    │     ├─ ProcessorRegistry.Process()
    │     │   │
    │     │   ├─ Buscar processor por event_type
    │     │   │
    │     │   └─ processor.Process(ctx, payload)
    │     │       ├─ Deserializar evento completo
    │     │       └─ Ejecutar lógica de negocio
    │     │
    │     ├─ Éxito → return nil
    │     └─ Error → return err
    │
    ├─ 4a. Éxito → msg.Ack(false)
    │     └─ Mensaje eliminado de la cola
    │
    └─ 4b. Error → msg.Nack(false, true)
          └─ Mensaje devuelto a la cola
              └─ Excede reintentos → Dead Letter Queue
```

---

## 8. Cómo Agregar un Nuevo Tipo de Evento

### Paso 1: Definir el DTO

```go
// internal/application/dto/event_dto.go
type MiNuevoEvent struct {
    EventType string    `json:"event_type"`
    EntityID  string    `json:"entity_id"`
    Data      string    `json:"data"`
    Timestamp time.Time `json:"timestamp"`
}
```

### Paso 2: Agregar constante

```go
// internal/domain/constants/event_constants.go
const EventTypeMiNuevoEvento = "mi_nuevo_evento"
```

### Paso 3: Crear el processor

Ver guía en [procesadores.md](procesadores.md#11-cómo-agregar-un-nuevo-processor).

### Paso 4: (Si nueva queue) Agregar config

```yaml
# config/config.yaml
messaging:
  rabbitmq:
    queues:
      mi_nuevo_evento: "edugo.mi_nuevo.evento"
```

### Paso 5: (Si nueva queue) Agregar binding en setupRabbitMQ

```go
// cmd/main.go → setupRabbitMQ()
ch.QueueBind(
    cfg.Messaging.RabbitMQ.Queues.MiNuevoEvento,
    "mi_nuevo.*",  // routing key pattern
    cfg.Messaging.RabbitMQ.Exchanges.Materials,
    false, nil,
)
```

### Paso 6: Configurar rate limiting

```yaml
rate_limiter:
  by_event_type:
    mi_nuevo_evento:
      requests_per_second: 10.0
      burst_size: 20.0
```

### Paso 7: Publicar desde la API

Desde la API que genera el evento:

```go
body, _ := json.Marshal(MiNuevoEvent{
    EventType: "mi_nuevo_evento",
    EntityID:  entityID,
    Timestamp: time.Now(),
})

ch.Publish(
    "edugo.materials",       // exchange
    "mi_nuevo.creado",       // routing key
    false, false,
    amqp.Publishing{
        ContentType: "application/json",
        Body:        body,
    },
)
```

---

*Generado: marzo 2026 | Basado en análisis directo del código fuente*
