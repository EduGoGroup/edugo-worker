# ECOSYSTEM CONTEXT - Worker

## Posición en EduGo

**Rol:** Microservicio Backend - Procesamiento asíncrono especializado  
**Interacción:** Consumidor de mensajes de API Mobile, productor de resultados

---

## Mapa de Ecosistema

```
┌──────────────────────────┐
│   API MOBILE (8080)      │
│  Solicita quiz generado  │
└────────────┬─────────────┘
             │ RabbitMQ message
             ▼
        ┌────────────┐
        │ RabbitMQ   │
        │ (Broker)   │
        └────────────┘
             │
             ├──────────────────────────┐
             │                          │
             ▼                          ▼
    ┌─────────────────┐      ┌──────────────────┐
    │   WORKER        │      │   Other Services │
    │ (Este proyecto) │      │  (Audit, etc)    │
    └────────┬────────┘      └──────────────────┘
             │
             ├─────────────────┬──────────────┬───────────────┐
             │                 │              │               │
             ▼                 ▼              ▼               ▼
        ┌────────┐      ┌──────────┐   ┌─────────┐    ┌──────────┐
        │ OpenAI │      │    S3    │   │PostgreSQL    │MongoDB   │
        │ GPT-4  │      │(Storage) │   │(Audit)       │(Results) │
        └────────┘      └──────────┘   └─────────┘    └──────────┘

        │
        └────── reply─────────────────────────────┐
                                                   │
                                    ┌──────────────▼──────────┐
                                    │    API MOBILE consumes  │
                                    │    Respuesta del Worker │
                                    └─────────────────────────┘
```

---

## Interacciones con Otros Servicios

### 1. Integración con API MOBILE (Productor/Consumidor)

**Canal:** RabbitMQ Message Broker

#### Mensaje de Solicitud (API Mobile → Worker)
```json
{
  "request_id": "uuid-12345",
  "type": "generate_assessment",
  "material_id": 42,
  "material_path": "s3://edugo-materials/raw/42/content.pdf",
  "config": {
    "num_questions": 10,
    "difficulty": "medium",
    "language": "es",
    "question_types": ["multiple_choice", "true_false"]
  },
  "timestamp": "2025-11-15T10:30:00Z"
}
```

**Routing RabbitMQ:**
- Exchange: `assessment.requests` (topic)
- Routing Key: `worker.assessment.requests`
- Queue: `worker.assessment.requests` (durable)

#### Mensaje de Respuesta (Worker → API Mobile)
```json
{
  "request_id": "uuid-12345",
  "status": "success",
  "evaluation_id": 1001,
  "questions_generated": 10,
  "questions": [
    {
      "id": "q-1",
      "text": "¿Cuál es la capital de España?",
      "type": "multiple_choice",
      "difficulty": "medium",
      "points": 5,
      "options": [
        {
          "id": "opt-1",
          "text": "Madrid",
          "correct": true,
          "order": 1
        },
        {
          "id": "opt-2",
          "text": "Barcelona",
          "correct": false,
          "order": 2
        }
      ],
      "explanation": "Madrid es la capital de España desde el siglo XVI..."
    }
  ],
  "metadata": {
    "model_used": "gpt-4",
    "processing_time_seconds": 45,
    "tokens_used": 1500,
    "cost_usd": 0.075
  },
  "timestamp": "2025-11-15T10:45:00Z"
}
```

**Routing respuesta:**
- Exchange: `assessment.responses` (topic)
- Routing Key: `api-mobile.assessment.responses`
- Queue: `api-mobile.assessment.responses` (durable)

#### Error Response
```json
{
  "request_id": "uuid-12345",
  "status": "error",
  "error_code": "PDF_EXTRACTION_FAILED",
  "error_message": "No se pudo extraer texto del PDF. El archivo puede estar corrupto.",
  "timestamp": "2025-11-15T10:45:00Z"
}
```

**Dead Letter Queue:** Si hay error, va a `assessment.dlx`

---

### 2. Integración con SHARED (v1.4.0+)

**Módulos utilizados:**

#### a) Logger Module
```go
import "github.com/EduGoGroup/edugo-shared/logger"

// Logging de procesamiento
logger.Info("Procesando solicitud de quiz", map[string]interface{}{
    "request_id": req.ID,
    "material_id": req.MaterialID,
    "timestamp": time.Now(),
})

logger.Warn("Timeout con OpenAI", map[string]interface{}{
    "request_id": req.ID,
    "attempt": 2,
    "retry_in_seconds": 4,
})
```

#### b) Database Module (PostgreSQL)
```go
import "github.com/EduGoGroup/edugo-shared/database"

db := database.GetDB()

// Guardar estado de procesamiento
var procReq ProcessingRequest
db.Create(&procReq)

// Actualizar progreso
db.Model(&procReq).Update("status", "processing").Update("progress_percent", 50)
```

#### c) Messaging Module (RabbitMQ)
```go
import "github.com/EduGoGroup/edugo-shared/messaging"

// Consumir mensajes
subscriber := messaging.NewSubscriber()
messages := subscriber.Subscribe("assessment.requests", "worker.assessment.requests")

// Publicar respuesta
publisher := messaging.NewPublisher()
publisher.Publish("assessment.responses", "api-mobile.assessment.responses", payload)
```

---

### 3. Integración con OpenAI API

**Endpoint:** https://api.openai.com/v1/chat/completions

**Modelo:** GPT-4

**Request:**
```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "system",
      "content": "Eres un profesor experto en crear preguntas educativas..."
    },
    {
      "role": "user",
      "content": "Basándote en este contenido, genera 10 preguntas..."
    }
  ],
  "temperature": 0.7,
  "max_tokens": 2000,
  "top_p": 1.0,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0
}
```

**Response:**
```json
{
  "id": "chatcmpl-8J7vZ9Z7Z7Z7Z7Z7Z7Z7Z7Z7",
  "object": "chat.completion",
  "created": 1700139000,
  "model": "gpt-4",
  "usage": {
    "prompt_tokens": 500,
    "completion_tokens": 1000,
    "total_tokens": 1500
  },
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"questions\": [...]}"
      },
      "finish_reason": "stop"
    }
  ]
}
```

**Configuración:**
```bash
OPENAI_API_KEY=sk-...
OPENAI_ORGANIZATION=org-...    # Opcional, para facturación
OPENAI_MODEL=gpt-4
OPENAI_TEMPERATURE=0.7         # Creatividad (0=determinístico, 1=creativo)
OPENAI_MAX_TOKENS=2000
OPENAI_TIMEOUT=60s
OPENAI_MAX_RETRIES=3
OPENAI_RETRY_DELAY=2s
```

**Manejo de errores:**
```
429 - Rate limited → Esperar según headers
401 - Invalid key → Revisar OPENAI_API_KEY
500 - Server error → Reintentar con backoff
```

---

### 4. Integración con Amazon S3

**Bucket:** `edugo-materials`

**Estructura:**
```
s3://edugo-materials/
├── raw/{material_id}/
│   └── content.pdf              # PDF original
├── extracted/{material_id}/
│   └── content.txt              # Texto extraído
└── processed/{material_id}/
    └── assessment.json          # Preguntas generadas
```

**Descargar archivo:**
```go
import "github.com/aws/aws-sdk-go-v2/service/s3"

// Path recibido de API Mobile: s3://edugo-materials/raw/42/content.pdf
// Descargar a /tmp/
s3Client := s3.NewFromConfig(cfg)
result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
    Bucket: aws.String("edugo-materials"),
    Key:    aws.String("raw/42/content.pdf"),
})

// Guardar localmente
file, _ := os.Create("/tmp/material-42.pdf")
io.Copy(file, result.Body)
```

**Guardar resultados:**
```go
// Después de procesar: guardar en S3
s3Client.PutObject(ctx, &s3.PutObjectInput{
    Bucket:      aws.String("edugo-materials"),
    Key:         aws.String("processed/42/assessment.json"),
    Body:        bytes.NewReader(jsonData),
    ContentType: aws.String("application/json"),
})
```

**Configuración:**
```bash
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
AWS_S3_BUCKET=edugo-materials
AWS_S3_ENDPOINT=https://s3.amazonaws.com
AWS_TIMEOUT=30s
```

---

### 5. Bases de Datos

#### PostgreSQL (Auditoría y Control)

**Tablas:**
```sql
-- Solicitudes de procesamiento
CREATE TABLE processing_requests (
  id VARCHAR(36) PRIMARY KEY,
  request_type VARCHAR(50), -- 'generate_quiz', 'summarize'
  material_id BIGINT NOT NULL,
  status VARCHAR(50), -- 'pending', 'processing', 'completed', 'error'
  progress_percent INT DEFAULT 0,
  started_at TIMESTAMP WITH TIME ZONE,
  completed_at TIMESTAMP WITH TIME ZONE,
  error_message TEXT,
  error_code VARCHAR(100),
  openai_tokens_used INT,
  openai_cost_usd DECIMAL(10, 6),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Índices
CREATE INDEX idx_pr_status ON processing_requests(status);
CREATE INDEX idx_pr_material_id ON processing_requests(material_id);
CREATE INDEX idx_pr_created_at ON processing_requests(created_at);
```

#### MongoDB (Resultados y Evaluaciones)

**Colección: material_assessment**
```javascript
{
  _id: ObjectId,
  request_id: "uuid-12345",
  material_id: 42,
  status: "success",
  questions: [
    {
      id: "q-1",
      text: "¿Pregunta?",
      type: "multiple_choice",
      options: [...],
      explanation: "..."
    }
  ],
  metadata: {
    model: "gpt-4",
    temperature: 0.7,
    prompt_tokens: 500,
    completion_tokens: 1000,
    generation_time_seconds: 45,
    cost_usd: 0.075
  },
  created_at: ISODate("2025-11-15T10:45:00Z"),
  expires_at: ISODate("2026-11-15T10:45:00Z")  // TTL de 1 año
}

// Índices
db.material_assessment.createIndex({ "request_id": 1 }, { unique: true })
db.material_assessment.createIndex({ "material_id": 1 })
db.material_assessment.createIndex({ "created_at": -1 })
db.material_assessment.createIndex({ "expires_at": 1 }, { expireAfterSeconds: 0 })  // TTL
```

**Colección: material_summary**
```javascript
{
  _id: ObjectId,
  material_id: 42,
  title: "Resumen: Capítulo 1 - Introducción",
  summary: "Este capítulo trata sobre los conceptos fundamentales...",
  key_points: [
    "Punto 1 importante",
    "Punto 2 importante"
  ],
  estimated_reading_time_minutes: 10,
  generated_at: ISODate("2025-11-15T10:45:00Z"),
  expires_at: ISODate("2026-11-15T10:45:00Z")
}

// Índices
db.material_summary.createIndex({ "material_id": 1 }, { unique: true })
db.material_summary.createIndex({ "generated_at": -1 })
```

---

## Flujos de Datos Inter-servicios

### Flujo Completo: Generar Quiz

```
┌─ API Mobile recibe solicitud del docente
│  └─ POST /api/v1/evaluaciones/material/{id}/generate-quiz
│
├─ API Mobile publica a RabbitMQ
│  ├─ Exchange: assessment.requests
│  ├─ Queue: worker.assessment.requests
│  └─ Payload: {request_id, material_id, config}
│
├─ Worker consume mensaje
│  ├─ Valida payload
│  ├─ Crea registro en PostgreSQL (status: "processing")
│  └─ Comienza procesamiento
│
├─ Worker descarga de S3
│  ├─ Descarga content.pdf
│  └─ Verifica integridad
│
├─ Worker extrae texto
│  ├─ Convierte PDF a texto
│  ├─ Limpia contenido
│  └─ Guarda en S3 (extracted/)
│
├─ Worker llama OpenAI
│  ├─ Prepara prompt con contenido extraído
│  ├─ Envía a GPT-4
│  └─ Recibe preguntas generadas (max 3 reintentos)
│
├─ Worker guarda resultados
│  ├─ Persiste en MongoDB (material_assessment)
│  ├─ Actualiza PostgreSQL (status: "completed")
│  └─ Guarda JSON en S3 (processed/)
│
├─ Worker publica respuesta
│  ├─ Exchange: assessment.responses
│  ├─ Queue: api-mobile.assessment.responses
│  └─ Payload: {request_id, status, questions}
│
└─ API Mobile consume respuesta
   ├─ Crea Evaluation en PostgreSQL
   ├─ Persiste preguntas
   └─ Notifica a cliente (app móvil)
```

---

## Dependencias Directas

| Servicio | Versión | Tipo | Críticidad |
|----------|---------|------|-----------|
| SHARED | v1.4.0+ | Librería Go | CRÍTICA |
| PostgreSQL | 15+ | Base datos | MEDIA |
| MongoDB | 7.0+ | Base datos | CRÍTICA |
| RabbitMQ | 3.12+ | Message broker | CRÍTICA |
| OpenAI API | Latest | API externa | CRÍTICA |
| Amazon S3 | Latest | Cloud storage | CRÍTICA |

---

## Ciclo de Vida de Solicitud

```
1. RECIBIDA (RabbitMQ)
   └─ PostgreSQL: status = "pending"

2. PROCESANDO (Worker activo)
   └─ PostgreSQL: status = "processing", progress = 25%

3. GENERANDO (OpenAI activo)
   └─ PostgreSQL: progress = 75%

4. GUARDANDO (Persistencia)
   ├─ MongoDB: material_assessment
   ├─ S3: processed/
   └─ PostgreSQL: progress = 95%

5. COMPLETADA
   ├─ PostgreSQL: status = "completed", completed_at = NOW()
   ├─ RabbitMQ: publica respuesta
   └─ MongoDB: success = true

O EN ERROR:
5. ERROR
   ├─ PostgreSQL: status = "error", error_message = "..."
   ├─ RabbitMQ: Dead Letter Queue
   └─ Reintentos: hasta 3 veces
```

---

## Compatibilidad de Versiones

| SHARED | Worker | Compatibilidad |
|--------|--------|----------------|
| v1.4.0 | v1.0.0 | ✅ Compatible |
| v1.5.0 | v1.0.0 | ✅ Compatible |
| v2.0.0 | v1.0.0 | ❌ Breaking |

| API Mobile | Worker | Compatibilidad |
|-----------|--------|----------------|
| v1.0+ | v1.0+ | ✅ Compatible |
| v1.0+ | v2.0+ | ⚠️ Message format change |

---

## Puntos de Fallo Críticos

| Fallo | Impacto | Mitigation |
|------|--------|-----------|
| OpenAI offline | Quizzes no se generan | Reintentos + queue |
| S3 inaccesible | No se descarga PDF | Reintentos + notificar |
| RabbitMQ caída | Solicitudes se pierden | Dead letter queue |
| MongoDB caída | Resultados no persisten | Retry + fallback |
| Worker crash | Solicitud colgada | Timeout + requeue |

---

## Monitoreo Crítico

```
Métricas a alertar:
- Queue size > 100
- Mensaje sin procesar > 5 min
- Error rate > 5%
- OpenAI latency > 60s
- Costos de OpenAI diarios
- Espacio en S3 usado
```

---

## Checklist de Integración

- [ ] SHARED v1.4.0+ importado en go.mod
- [ ] Conexión PostgreSQL funcionando
- [ ] Conexión MongoDB funcionando
- [ ] Conexión RabbitMQ funcionando
- [ ] OpenAI API key configurada
- [ ] S3 credentials configuradas
- [ ] Logger centralizado activo
- [ ] Health checks implementados
- [ ] Retry logic con backoff
- [ ] Dead letter queue configurada
- [ ] Monitoreo de costos OpenAI
- [ ] Tests de integración pasando
