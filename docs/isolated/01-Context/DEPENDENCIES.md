# DEPENDENCIES - Worker

## Matriz de Dependencias

```
┌──────────────────────────────────────────────────────────┐
│                   WORKER                                 │
├──────────────────────────────────────────────────────────┤
│ Dependencias Críticas (sin estas NO funciona)            │
│ ├─ SHARED v1.4.0+                                        │
│ ├─ RabbitMQ 3.12+                                        │
│ ├─ OpenAI API (API Key requerida)                        │
│ ├─ Amazon S3 (credenciales AWS)                          │
│ └─ MongoDB 7.0+ (persistencia resultados)                │
│                                                          │
│ Dependencias Opcionales/Mejoras                          │
│ ├─ PostgreSQL 15+ (auditoría, opcional)                 │
│ ├─ Redis (caché de PDFs procesados)                     │
│ └─ Datadog/ELK (monitoreo centralizado)                 │
│                                                          │
│ Dependencias de Desarrollo/Ops                           │
│ ├─ Docker                                                │
│ ├─ Docker Compose                                        │
│ └─ Go 1.21+                                              │
└──────────────────────────────────────────────────────────┘
```

---

## Dependencias Críticas

### 1. SHARED v1.4.0+

**¿Qué es?** Librería Go compartida con utilidades reutilizables

**Módulos requeridos:**

#### a) Logger Module
```go
import "github.com/EduGoGroup/edugo-shared/logger"

logger.Info("Procesando solicitud", map[string]interface{}{
    "request_id": req.ID,
    "material_id": req.MaterialID,
})

logger.Error("Error al procesar", map[string]interface{}{
    "error": err.Error(),
    "request_id": req.ID,
})
```

#### b) Database Module (PostgreSQL opcional)
```go
import "github.com/EduGoGroup/edugo-shared/database"

db := database.GetDB()
db.Create(&ProcessingRequest{
    ID: req.ID,
    Status: "processing",
})
```

#### c) Messaging Module (RabbitMQ)
```go
import "github.com/EduGoGroup/edugo-shared/messaging"

// Consumidor
subscriber := messaging.NewSubscriber()
messages := subscriber.Subscribe("assessment.requests", "worker.assessment.requests")

// Publicador
publisher := messaging.NewPublisher()
publisher.Publish("assessment.responses", "api-mobile.assessment.responses", payload)
```

**Variables de entorno:**
```bash
SHARED_LOG_LEVEL=info
SHARED_CONTEXT_TIMEOUT=120s  # Timeout global para operaciones
```

---

### 2. RabbitMQ 3.12+

**¿Qué es?** Message broker para comunicación asíncrona confiable

**Requisitos:**
- Versión: 3.12 o superior
- Puerto: 5672 (AMQP)
- Puerto 15672 (Management UI)
- Max queues: 10000

**Exchanges y Queues necesarios:**

```bash
# Crear exchange para requests
rabbitmqctl declare_exchange \
  assessment.requests \
  type:direct \
  durable:true

# Crear queue para worker (consumer)
rabbitmqctl declare_queue \
  worker.assessment.requests \
  durable:true

# Bindear
rabbitmqctl bind_queue \
  assessment.requests \
  worker.assessment.requests \
  worker.assessment.requests

# Crear exchange para responses
rabbitmqctl declare_exchange \
  assessment.responses \
  type:direct \
  durable:true

# Crear queue para API Mobile
rabbitmqctl declare_queue \
  api-mobile.assessment.responses \
  durable:true

# Bindear
rabbitmqctl bind_queue \
  assessment.responses \
  api-mobile.assessment.responses \
  api-mobile.assessment.responses

# Dead letter exchange (para errores)
rabbitmqctl declare_exchange \
  assessment.dlx \
  type:direct \
  durable:true

rabbitmqctl declare_queue \
  assessment.dlq \
  durable:true

rabbitmqctl bind_queue \
  assessment.dlx \
  assessment.dlq \
  assessment.dlx
```

**Configuración en Worker:**
```bash
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_VHOST=/
RABBITMQ_CONNECTION_TIMEOUT=10s
RABBITMQ_MAX_RETRIES=3
RABBITMQ_RETRY_DELAY=5s

# Consumer configuration
RABBITMQ_PREFETCH_COUNT=1          # Un mensaje a la vez
RABBITMQ_AUTO_ACK=false             # Manual acknowledge
RABBITMQ_CONSUMER_TAG=worker-consumer
```

**Código de consumo:**
```go
package consumer

import (
    "github.com/EduGoGroup/edugo-shared/messaging"
)

type WorkerConsumer struct {
    subscriber messaging.Subscriber
    processor  *Processor
}

func (wc *WorkerConsumer) Start(ctx context.Context) error {
    messages := wc.subscriber.Subscribe(
        "assessment.requests",
        "worker.assessment.requests",
    )
    
    for {
        select {
        case msg := <-messages:
            var req AssessmentRequest
            json.Unmarshal(msg.Body, &req)
            
            if err := wc.processor.Process(ctx, &req); err != nil {
                logger.Error("Processing error", map[string]interface{}{
                    "error": err.Error(),
                })
                msg.Nack(false, true)  // Requeue
            } else {
                msg.Ack(false)  // Manual acknowledge
            }
            
        case <-ctx.Done():
            return nil
        }
    }
}
```

---

### 3. OpenAI API

**¿Qué es?** Servicio en la nube de IA para generar contenido

**Requisitos:**
- API Key: `sk-...` (obtener de https://platform.openai.com/api-keys)
- Modelo: GPT-4 (u otra versión disponible)
- Account con créditos/billing activo

**Request hacia OpenAI:**
```go
import "github.com/sashabaranov/go-openai"

client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

resp, err := client.CreateChatCompletion(
    context.Background(),
    openai.ChatCompletionRequest{
        Model: openai.GPT4,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "Eres un profesor experto...",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: "Genera 10 preguntas basadas en: " + content,
            },
        },
        Temperature:      0.7,
        MaxTokens:        2000,
        TopP:             1.0,
        FrequencyPenalty: 0.0,
        PresencePenalty:  0.0,
    },
)

if err != nil {
    logger.Error("OpenAI error", map[string]interface{}{
        "error": err.Error(),
    })
    return nil, err
}

// Parsear respuesta
var questions map[string]interface{}
json.Unmarshal([]byte(resp.Choices[0].Message.Content), &questions)
```

**Manejo de errores:**
```go
func handleOpenAIError(err error, retryCount int) error {
    if openai.IsRateLimitError(err) {
        // 429: Rate limit, esperar exponencial backoff
        backoff := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
        logger.Warn("Rate limited by OpenAI", map[string]interface{}{
            "wait_seconds": backoff.Seconds(),
        })
        time.Sleep(backoff)
        return nil  // Retry
    }
    
    if openai.IsAuthenticationError(err) {
        // 401: Invalid key
        logger.Error("Invalid OpenAI API key", nil)
        return errors.New("authentication failed")
    }
    
    if openai.IsServerError(err) {
        // 500: Reintentar
        return nil  // Retry
    }
    
    return err
}
```

**Configuración:**
```bash
OPENAI_API_KEY=sk-...                      # Requerido
OPENAI_ORGANIZATION=org-...                 # Opcional, para facturación de org
OPENAI_MODEL=gpt-4                          # O gpt-3.5-turbo si presupuesto es limitado
OPENAI_TEMPERATURE=0.7                      # 0 = determinístico, 1 = creativo
OPENAI_MAX_TOKENS=2000                      # Máximo de tokens en respuesta
OPENAI_TIMEOUT=60s                          # Timeout por request
OPENAI_MAX_RETRIES=3                        # Máximo reintentos
OPENAI_RETRY_DELAY=2s                       # Delay inicial entre reintentos
```

**Costos estimados:**
```
GPT-4:
  - Input:  $0.03 / 1K tokens
  - Output: $0.06 / 1K tokens
  
Estimado por quiz (10 preguntas):
  - Tokens: ~1500 (500 input + 1000 output)
  - Costo: ~$0.075 por quiz

Ejemplo de gasto mensual (1000 quizzes/mes):
  - Costo: $75/mes
```

---

### 4. Amazon S3

**¿Qué es?** Cloud storage para archivos (PDFs, resultados)

**Requisitos:**
- Bucket: `edugo-materials` (pre-creado)
- Credenciales AWS: Access Key ID + Secret Access Key
- Permisos: GetObject, PutObject

**Setup:**
```bash
# Crear bucket (una sola vez)
aws s3 mb s3://edugo-materials

# Crear structure de carpetas
aws s3api put-object --bucket edugo-materials --key raw/
aws s3api put-object --bucket edugo-materials --key extracted/
aws s3api put-object --bucket edugo-materials --key processed/

# Configurar lifecycles (cleanup automático)
aws s3api put-bucket-lifecycle-configuration \
  --bucket edugo-materials \
  --lifecycle-configuration file://lifecycle.json
```

**Lifecycle policy (lifecycle.json):**
```json
{
  "Rules": [
    {
      "Id": "DeleteOldExtracted",
      "Filter": { "Prefix": "extracted/" },
      "Expiration": { "Days": 7 },
      "Status": "Enabled"
    },
    {
      "Id": "DeleteOldProcessed",
      "Filter": { "Prefix": "processed/" },
      "Expiration": { "Days": 90 },
      "Status": "Enabled"
    }
  ]
}
```

**Uso en Worker:**
```go
import "github.com/aws/aws-sdk-go-v2/service/s3"

// Descargar PDF
s3Client := s3.NewFromConfig(cfg)

result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
    Bucket: aws.String("edugo-materials"),
    Key:    aws.String("raw/42/content.pdf"),
})
defer result.Body.Close()

file, _ := os.Create("/tmp/material-42.pdf")
io.Copy(file, result.Body)

// Subir resultado
jsonData, _ := json.Marshal(assessment)

_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
    Bucket:      aws.String("edugo-materials"),
    Key:         aws.String("processed/42/assessment.json"),
    Body:        bytes.NewReader(jsonData),
    ContentType: aws.String("application/json"),
    Metadata: map[string]string{
        "processed-by": "worker-v1.0",
        "timestamp": time.Now().String(),
    },
})
```

**Configuración:**
```bash
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
AWS_S3_BUCKET=edugo-materials
AWS_S3_ENDPOINT=https://s3.amazonaws.com
AWS_S3_TIMEOUT=30s
AWS_S3_MAX_RETRIES=3
```

---

### 5. MongoDB 7.0+

**¿Qué es?** Base de datos documental para almacenar resultados

**Requisitos:**
- Versión: 7.0 o superior
- Puerto: 27017 (default)
- Base de datos: `edugo_assessments`
- Colecciones: `material_assessment`, `material_summary`

**Setup:**
```javascript
// Crear colecciones con validación
db.createCollection("material_assessment", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["request_id", "material_id", "status"],
      properties: {
        request_id: { bsonType: "string" },
        material_id: { bsonType: "int" },
        status: { bsonType: "string" },
        questions: {
          bsonType: "array",
          items: {
            bsonType: "object",
            properties: {
              id: { bsonType: "string" },
              text: { bsonType: "string" },
              type: { bsonType: "string" }
            }
          }
        }
      }
    }
  }
})

// Crear índices
db.material_assessment.createIndex({ "request_id": 1 }, { unique: true })
db.material_assessment.createIndex({ "material_id": 1 })
db.material_assessment.createIndex({ "created_at": -1 })
db.material_assessment.createIndex(
  { "expires_at": 1 },
  { expireAfterSeconds: 0 }  // TTL index
)
```

**Uso en Worker:**
```go
import "go.mongodb.org/mongo-driver/mongo"

collection := mongoClient.Database("edugo_assessments").Collection("material_assessment")

// Insertar resultado
result, err := collection.InsertOne(ctx, assessment)

// Actualizar si ya existe
_, err = collection.UpdateOne(ctx,
    bson.M{"request_id": req.ID},
    bson.M{"$set": bson.M{"status": "completed"}},
    options.Update().SetUpsert(true),
)

// Leer resultado
var assessment MaterialAssessment
err = collection.FindOne(ctx, bson.M{
    "request_id": req.ID,
}).Decode(&assessment)
```

**Configuración:**
```bash
MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=edugo_assessments
MONGO_USER=admin
MONGO_PASSWORD=password
MONGO_TIMEOUT=10s
MONGO_MAX_POOL_SIZE=100
```

---

## Dependencias Opcionales (Mejoras)

### PostgreSQL 15+ (Auditoría)

**No crítica** pero recomendada para logging de procesamiento

```sql
CREATE TABLE processing_requests (
  id VARCHAR(36) PRIMARY KEY,
  status VARCHAR(50),
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE processed_materials (
  id BIGSERIAL PRIMARY KEY,
  material_id BIGINT,
  text_extracted TEXT,
  processed_at TIMESTAMP DEFAULT NOW()
);
```

**Variables de entorno:**
```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=edugo_user
DB_PASSWORD=password
DB_NAME=edugo_worker
```

---

### Redis (Caché Opcional)

**Usar para:** Caché de PDFs ya procesados, resultados frecuentes

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_TTL=3600  # Cache 1 hora
```

```go
import "github.com/redis/go-redis/v9"

rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Caché de hit/miss
key := fmt.Sprintf("pdf:extracted:%d", materialID)
val, err := rdb.Get(ctx, key).Result()
if err == nil {
    return val  // Cache hit
}

// Guardar en caché
rdb.Set(ctx, key, extractedText, 1*time.Hour)
```

---

## Dependencias de Desarrollo

### Go 1.21+

**Setup local:**
```bash
# Verificar versión
go version

# Descargar dependencias
go mod download

# Limpiar
go mod tidy

# Compilar
go build -o worker ./cmd/worker

# Tests
go test ./...

# Coverage
go test -cover ./...
```

---

### Docker

**Dockerfile:**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/worker .

EXPOSE 8080
ENV WORKER_CONCURRENT_JOBS=5

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./worker"]
```

---

## Matriz de Compatibilidad

| Componente | Versión Requerida | Versión Máxima |
|-----------|------------------|----------------|
| Go | 1.21+ | Latest |
| SHARED | v1.4.0+ | v1.x |
| RabbitMQ | 3.12+ | Latest |
| OpenAI API | Latest | Latest |
| S3 | Latest | Latest |
| MongoDB | 7.0+ | Latest |
| PostgreSQL | 15+ | Latest |

---

## Checklist de Instalación

```markdown
## Dependencias Críticas
- [ ] RabbitMQ 3.12 levantado y accesible
- [ ] Exchanges y queues creados
- [ ] MongoDB 7.0 levantado en localhost:27017
- [ ] Colecciones creadas (material_assessment, material_summary)
- [ ] OpenAI API key obtenida y configurada
- [ ] Bucket S3 creado y accesible
- [ ] AWS credenciales configuradas

## Dependencias de Código
- [ ] Go 1.21+ instalado
- [ ] go mod download ejecutado
- [ ] go mod tidy ejecutado
- [ ] SHARED v1.4.0+ en go.mod
- [ ] go-openai importado
- [ ] aws-sdk-go-v2 importado
- [ ] mongo-go-driver importado
- [ ] go build compila sin errores

## Variables de Entorno
- [ ] OPENAI_API_KEY configurada
- [ ] RABBITMQ_HOST, PORT, USER, PASSWORD
- [ ] AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
- [ ] MONGO_URI configurada
- [ ] LOG_LEVEL configurado

## Testing
- [ ] go test ./... pasa
- [ ] Mocks de OpenAI funcionan
- [ ] Mocks de S3 funcionan
- [ ] Mocks de RabbitMQ funcionan
```

---

## Resolución de Problemas

### OpenAI no responde
```bash
# Verificar API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Verificar saldo/límite de rate
# https://platform.openai.com/account/usage/overview
```

### RabbitMQ no conecta
```bash
# Verificar conexión
docker logs rabbitmq

# Ver queues
rabbitmqctl list_queues

# Ver exchanges
rabbitmqctl list_exchanges
```

### S3 not found
```bash
# Verificar bucket existe
aws s3 ls s3://edugo-materials

# Verificar credenciales
aws sts get-caller-identity
```

### MongoDB no conecta
```bash
# Verificar colecciones
mongosh
> use edugo_assessments
> show collections
```

---

## Dependencias Go principales

```bash
go get github.com/EduGoGroup/edugo-shared@v1.4.0
go get github.com/streadway/amqp@latest
go get github.com/sashabaranov/go-openai@latest
go get github.com/aws/aws-sdk-go-v2@latest
go get github.com/aws/aws-sdk-go-v2/service/s3@latest
go get go.mongodb.org/mongo-driver@latest
go get github.com/spf13/viper@latest
```
