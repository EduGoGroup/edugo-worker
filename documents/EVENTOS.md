# Eventos RabbitMQ - EduGo Worker

## 📋 Visión General

El worker es un **consumidor** de eventos. Recibe mensajes de RabbitMQ publicados por otros servicios (principalmente API Mobile y API Admin) y los procesa de forma asíncrona.

---

## 🔄 Configuración RabbitMQ

### Exchange

```yaml
Exchange: edugo.materials
Type: topic
Durable: true
Auto-deleted: false
```

### Queue Principal

```yaml
Queue: edugo.material.uploaded
Durable: true
Arguments:
  x-max-priority: 10           # Soporte para prioridad 0-10
  x-dead-letter-exchange: edugo_dlq  # DLQ para mensajes fallidos
```

### Bindings

| Exchange | Routing Key | Queue |
|----------|-------------|-------|
| `edugo.materials` | `material.uploaded` | `edugo.material.uploaded` |
| `edugo.materials` | `material.deleted` | `edugo.material.uploaded` |
| `edugo.materials` | `material.reprocess` | `edugo.material.uploaded` |

### Consumer Config

```yaml
Prefetch Count: 5              # Máximo mensajes sin ACK
Auto-ACK: false                # ACK manual después de procesar
Exclusive: false               # Múltiples consumers permitidos
```

---

## 📨 Eventos Consumidos

### 1. MaterialUploadedEvent

**Publicado por:** API Mobile / API Admin cuando un docente sube un material

**Routing Key:** `material.uploaded`

```json
{
  "event_type": "material_uploaded",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "author_id": "123e4567-e89b-12d3-a456-426614174000",
  "s3_key": "materials/courses/unit-123/document.pdf",
  "preferred_language": "es",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Campos:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `event_type` | string | ✅ | Siempre `"material_uploaded"` |
| `material_id` | UUID string | ✅ | ID del material en PostgreSQL |
| `author_id` | UUID string | ✅ | ID del docente que subió |
| `s3_key` | string | ✅ | Ruta del archivo en AWS S3 |
| `preferred_language` | string | ❌ | Idioma preferido para el resumen (`es`, `en`) |
| `timestamp` | ISO 8601 | ✅ | Momento del evento |

**DTO en Go:**

```go
// internal/application/dto/event_dto.go
type MaterialUploadedEvent struct {
    EventType         string    `json:"event_type"`
    MaterialID        string    `json:"material_id"`
    AuthorID          string    `json:"author_id"`
    S3Key             string    `json:"s3_key"`
    PreferredLanguage string    `json:"preferred_language"`
    Timestamp         time.Time `json:"timestamp"`
}
```

---

### 2. MaterialDeletedEvent

**Publicado por:** API Mobile / API Admin cuando un material es eliminado

**Routing Key:** `material.deleted`

```json
{
  "event_type": "material_deleted",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-15T14:30:00Z"
}
```

**Campos:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `event_type` | string | ✅ | Siempre `"material_deleted"` |
| `material_id` | UUID string | ✅ | ID del material eliminado |
| `timestamp` | ISO 8601 | ✅ | Momento de la eliminación |

**DTO en Go:**

```go
type MaterialDeletedEvent struct {
    EventType  string    `json:"event_type"`
    MaterialID string    `json:"material_id"`
    Timestamp  time.Time `json:"timestamp"`
}
```

---

### 3. AssessmentAttemptEvent

**Publicado por:** API Mobile cuando un estudiante completa un quiz

**Routing Key:** `assessment.attempt`

```json
{
  "event_type": "assessment_attempt",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "789e0123-e45b-67d8-a901-234567890123",
  "answers": {
    "q1": "b",
    "q2": "true",
    "q3": "a"
  },
  "score": 85.5,
  "timestamp": "2024-01-15T16:45:00Z"
}
```

**Campos:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `event_type` | string | ✅ | Siempre `"assessment_attempt"` |
| `material_id` | UUID string | ✅ | ID del material/quiz |
| `user_id` | UUID string | ✅ | ID del estudiante |
| `answers` | object | ✅ | Map de pregunta_id → respuesta |
| `score` | float64 | ✅ | Puntuación obtenida (0-100) |
| `timestamp` | ISO 8601 | ✅ | Momento del intento |

**DTO en Go:**

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

### 4. StudentEnrolledEvent

**Publicado por:** API Mobile / API Admin cuando un estudiante se inscribe

**Routing Key:** `student.enrolled`

```json
{
  "event_type": "student_enrolled",
  "student_id": "789e0123-e45b-67d8-a901-234567890123",
  "unit_id": "456e7890-a12b-34c5-d678-901234567890",
  "timestamp": "2024-01-15T09:00:00Z"
}
```

**Campos:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `event_type` | string | ✅ | Siempre `"student_enrolled"` |
| `student_id` | UUID string | ✅ | ID del estudiante |
| `unit_id` | UUID string | ✅ | ID de la unidad/curso |
| `timestamp` | ISO 8601 | ✅ | Momento de la inscripción |

**DTO en Go:**

```go
type StudentEnrolledEvent struct {
    EventType string    `json:"event_type"`
    StudentID string    `json:"student_id"`
    UnitID    string    `json:"unit_id"`
    Timestamp time.Time `json:"timestamp"`
}
```

---

### 5. MaterialReprocessEvent (Futuro)

**Publicado por:** API Admin para regenerar contenido

**Routing Key:** `material.reprocess`

```json
{
  "event_type": "material_reprocess",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "reason": "ai_model_update",
  "options": {
    "regenerate_summary": true,
    "regenerate_quiz": true,
    "new_language": "en"
  },
  "timestamp": "2024-01-15T11:00:00Z"
}
```

---

## 📊 Tipos de Evento Válidos

La máquina de estados del worker valida estos tipos:

```go
// internal/domain/service/event_state_machine.go
validTypes := []string{
    "material_uploaded",
    "material_reprocess",
    "material_deleted",
    "assessment_attempt",
    "student_enrolled",
    "student_unenrolled",
}
```

---

## 🔄 Flujo de Mensajes

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MESSAGE FLOW                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────────────────┐   │
│  │  API Mobile  │      │  RabbitMQ    │      │       Worker             │   │
│  │  API Admin   │      │              │      │                          │   │
│  └──────┬───────┘      └──────┬───────┘      └────────────┬─────────────┘   │
│         │                     │                           │                  │
│         │ 1. Publish          │                           │                  │
│         │ (JSON + routing key)│                           │                  │
│         │────────────────────>│                           │                  │
│         │                     │                           │                  │
│         │                     │ 2. Route to Queue         │                  │
│         │                     │ (topic exchange)          │                  │
│         │                     │──────────────────────────>│                  │
│         │                     │                           │                  │
│         │                     │                           │ 3. Consume       │
│         │                     │                           │ (prefetch: 5)    │
│         │                     │                           │                  │
│         │                     │                           │ 4. Process       │
│         │                     │                           │ (processor)      │
│         │                     │                           │                  │
│         │                     │ 5. ACK/NACK               │                  │
│         │                     │<──────────────────────────│                  │
│         │                     │                           │                  │
│         │                     │ Si NACK + requeue:        │                  │
│         │                     │ 6. Retry                  │                  │
│         │                     │──────────────────────────>│                  │
│         │                     │                           │                  │
│         │                     │ Si max retries:           │                  │
│         │                     │ 7. → Dead Letter Queue    │                  │
│         │                     │                           │                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 📤 Ejemplo de Publicación (desde API)

Cómo otros servicios publican eventos que el worker consume:

```go
// Ejemplo desde API Mobile
func (s *MaterialService) AfterUpload(ctx context.Context, material *Material) error {
    event := map[string]interface{}{
        "event_type":         "material_uploaded",
        "material_id":        material.ID.String(),
        "author_id":          material.AuthorID.String(),
        "s3_key":            material.S3Key,
        "preferred_language": material.Language,
        "timestamp":         time.Now().UTC(),
    }
    
    body, _ := json.Marshal(event)
    
    return channel.PublishWithContext(ctx,
        "edugo.materials",      // exchange
        "material.uploaded",    // routing key
        false,                  // mandatory
        false,                  // immediate
        amqp.Publishing{
            ContentType:  "application/json",
            Body:         body,
            DeliveryMode: amqp.Persistent,
            Priority:     5,  // Prioridad media (0-10)
        },
    )
}
```

---

## 🛡️ Validación de Eventos

El worker valida cada evento antes de procesarlo:

```go
// Validaciones realizadas:
1. JSON bien formado
2. event_type presente y válido
3. Campos requeridos presentes
4. material_id es UUID válido
5. timestamp es ISO 8601 válido
```

### Errores de Validación

```json
// Error por event_type inválido
{"level":"error","msg":"invalid event_type","received":"unknown_type"}

// Error por material_id inválido
{"level":"error","msg":"Error parseando evento","error":"invalid material_id"}

// Error por JSON malformado
{"level":"error","msg":"Error parseando evento","error":"invalid character..."}
```

---

## 📊 Prioridades de Mensajes

El worker soporta prioridad de mensajes (0-10):

| Prioridad | Uso Recomendado |
|-----------|-----------------|
| 0-2 | Bajo: Reprocess batch, analytics |
| 3-5 | Normal: Material uploaded estándar |
| 6-8 | Alto: Materiales de cursos activos |
| 9-10 | Crítico: Correcciones urgentes |

---

## 🔄 Dead Letter Queue

Mensajes que fallan después de max retries van a DLQ:

```yaml
Exchange: edugo_dlq
Queue: edugo.dlq.material
```

**Estructura del mensaje en DLQ:**

```json
{
  "original_message": {...},
  "x-death": [
    {
      "count": 3,
      "reason": "rejected",
      "queue": "edugo.material.uploaded",
      "time": "2024-01-15T10:35:00Z",
      "exchange": "edugo.materials",
      "routing-keys": ["material.uploaded"]
    }
  ]
}
```

---

## 📈 Monitoreo de Eventos

### Logs Estructurados

```json
// Evento recibido
{"level":"info","msg":"📥 Mensaje recibido","size":256}

// Evento parseado
{"level":"info","msg":"✅ Evento procesado","type":"material_uploaded"}

// Error en procesamiento
{"level":"error","msg":"Error procesando mensaje","error":"..."}
```

### Métricas Sugeridas

- `worker_events_received_total` - Contador por event_type
- `worker_events_processed_total` - Éxitos por event_type
- `worker_events_failed_total` - Fallos por event_type
- `worker_event_processing_duration_seconds` - Histograma de duración
