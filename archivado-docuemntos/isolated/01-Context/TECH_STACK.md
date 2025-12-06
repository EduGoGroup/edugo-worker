# TECH STACK - Worker

## Resumen Ejecutivo

| Layer | Tecnología | Versión | Propósito |
|-------|-----------|---------|----------|
| **Language** | Go | 1.21+ | Backend, compilado a binario |
| **Message Queue** | RabbitMQ | 3.12+ | Consumidor de solicitudes |
| **AI API** | OpenAI GPT-4 | Latest | Generación de preguntas |
| **Cloud Storage** | Amazon S3 | Latest | Almacenamiento de PDFs |
| **PDF Processing** | pdfium-go | Latest | Extracción de texto |
| **Primary DB** | MongoDB | 7.0+ | Persistencia de resultados |
| **Audit DB** | PostgreSQL | 15+ | Logging de procesamiento |
| **Config Management** | Viper | Latest | Multi-environment config |
| **Containerization** | Docker | 20.10+ | Container runtime |
| **Orchestration** | Docker Compose | 2.0+ | Local development |

---

## Stack Detallado por Capa

### Capa 1: Aplicación (Go)

```go
package main

func main() {
    // 1. Inicializar configuración
    config := LoadConfig()
    
    // 2. Conectar a dependencias
    logger.Init(config.Logger)
    mongoClient := InitMongo(config.MongoDB)
    pgDB := InitPostgres(config.PostgreSQL)
    
    // 3. Inicializar clientes
    openaiClient := openai.NewClient(config.OpenAI.APIKey)
    s3Client := s3.NewFromConfig(awsConfig)
    
    // 4. Crear consumer
    subscriber := messaging.NewSubscriber()
    processor := NewProcessor(
        mongoClient,
        pgDB,
        openaiClient,
        s3Client,
        logger,
    )
    
    consumer := NewWorkerConsumer(subscriber, processor)
    
    // 5. Iniciar escucha de mensajes
    ctx := context.Background()
    consumer.Start(ctx)
}
```

**Características Go utilizadas:**
- Goroutines: Procesar múltiples solicitudes concurrentemente
- Channels: Comunicación entre goroutines
- Context: Timeouts y cancellations
- Error handling: Wrapping de errores con contexto
- Interfaces: Inyección de dependencias

---

### Capa 2: Message Queue (RabbitMQ)

**¿Qué es?** Broker de mensajes AMQP para comunicación confiable

**Topología en EduGo:**

```
Producers:
  └─ API Mobile

Exchanges:
  ├─ assessment.requests (direct)
  └─ assessment.responses (direct)

Queues:
  ├─ worker.assessment.requests (consumer = Worker)
  │   └─ Mensajes de API Mobile
  │
  ├─ api-mobile.assessment.responses (consumer = API Mobile)
  │   └─ Respuestas de Worker
  │
  └─ assessment.dlq (dead letter)
      └─ Mensajes con error

Consumers:
  ├─ Worker (procesa assessment.requests)
  └─ API Mobile (procesa assessment.responses)
```

**Consumo en Worker:**

```go
package consumer

import (
    "github.com/streadway/amqp"
)

type WorkerConsumer struct {
    conn      *amqp.Connection
    channel   *amqp.Channel
    processor *Processor
}

func (wc *WorkerConsumer) Start(ctx context.Context) error {
    // 1. Declarar queue (idempotente)
    queue, err := wc.channel.QueueDeclare(
        "worker.assessment.requests",
        true,   // durable
        false,  // exclusive
        false,  // no-wait
        nil,    // arguments
    )
    
    // 2. Configurar QoS (procesar 1 mensaje a la vez)
    wc.channel.Qos(1, 0, false)
    
    // 3. Consumir mensajes
    messages, err := wc.channel.Consume(
        queue.Name,
        "worker-consumer",
        false,  // auto-ack = false (manual acknowledge)
        false,  // exclusive
        false,  // no-local
        false,  // no-wait
        nil,    // args
    )
    
    // 4. Procesar cada mensaje
    for {
        select {
        case msg := <-messages:
            if err := wc.processor.Process(ctx, msg.Body); err != nil {
                logger.Error("Processing failed", map[string]interface{}{
                    "error": err.Error(),
                })
                msg.Nack(false, true)  // Requeue el mensaje
            } else {
                msg.Ack(false)  // Acknowledge exitoso
            }
            
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

**Publicación de Respuestas:**

```go
func (p *Processor) PublishResponse(ctx context.Context, resp *AssessmentResponse) error {
    ch := p.rabbitmq.Channel
    
    body, _ := json.Marshal(resp)
    
    return ch.PublishWithContext(ctx,
        "assessment.responses",              // exchange
        "api-mobile.assessment.responses",   // routing key
        false,                                // mandatory
        false,                                // immediate
        amqp.Publishing{
            ContentType: "application/json",
            Body:        body,
        },
    )
}
```

**Ventajas:**
- Persistencia: Mensajes no se pierden si Worker falla
- Escalabilidad: Múltiples Workers pueden procesar en paralelo
- Decoupling: API Mobile no espera respuesta síncrona
- Retry automático: Dead letter queue para errores

---

### Capa 3: AI Integration (OpenAI)

**¿Qué es?** API Cloud de OpenAI para generación de texto con IA

```go
package ai

import (
    "github.com/sashabaranov/go-openai"
)

type QuestionGenerator struct {
    client *openai.Client
    config GeneratorConfig
}

func (qg *QuestionGenerator) GenerateQuestions(
    ctx context.Context,
    material *Material,
) ([]*Question, error) {
    
    // 1. Preparar prompt
    systemPrompt := `Eres un profesor experto en crear preguntas educativas de alta calidad.
Tu tarea es generar preguntas basadas en contenido educativo.
Responde siempre en formato JSON válido.`
    
    userPrompt := fmt.Sprintf(`Basándote en el siguiente contenido, crea %d preguntas de tipo %s.
Idioma: %s
Dificultad: %s

Contenido:
---
%s
---

Responde con un JSON que tenga esta estructura:
{
  "questions": [
    {
      "text": "Pregunta aquí?",
      "type": "multiple_choice",
      "options": [
        {"text": "Opción A", "correct": false},
        {"text": "Opción B", "correct": true}
      ],
      "explanation": "Explicación...",
      "points": 5
    }
  ]
}`,
        qg.config.NumQuestions,
        qg.config.QuestionType,
        qg.config.Language,
        qg.config.Difficulty,
        material.ExtractedText,
    )
    
    // 2. Llamar a OpenAI
    resp, err := qg.client.CreateChatCompletion(ctx,
        openai.ChatCompletionRequest{
            Model:            openai.GPT4,
            Temperature:      qg.config.Temperature,
            MaxTokens:        qg.config.MaxTokens,
            TopP:             1.0,
            FrequencyPenalty: 0.0,
            PresencePenalty:  0.0,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleSystem,
                    Content: systemPrompt,
                },
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: userPrompt,
                },
            },
        },
    )
    
    if err != nil {
        return nil, fmt.Errorf("openai error: %w", err)
    }
    
    // 3. Parsear respuesta
    var result struct {
        Questions []*Question `json:"questions"`
    }
    
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result)
    
    return result.Questions, nil
}
```

**Configuración de modelo:**
```go
type GeneratorConfig struct {
    Model            string              // "gpt-4"
    Temperature      float32             // 0.7 (balance creativo/determinístico)
    MaxTokens        int                 // 2000
    NumQuestions     int                 // 10
    QuestionType     string              // "multiple_choice"
    Language         string              // "es"
    Difficulty       string              // "medium"
}
```

**Manejo de errores con reintentos:**

```go
func (qg *QuestionGenerator) GenerateWithRetry(
    ctx context.Context,
    material *Material,
) ([]*Question, error) {
    
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        questions, err := qg.GenerateQuestions(ctx, material)
        
        if err == nil {
            return questions, nil
        }
        
        // Analizar tipo de error
        if openai.IsRateLimitError(err) {
            // Esperar con backoff exponencial
            backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
            logger.Warn("Rate limited, retrying...", map[string]interface{}{
                "attempt": attempt + 1,
                "wait_seconds": backoff.Seconds(),
            })
            time.Sleep(backoff)
            continue
        }
        
        if openai.IsAuthenticationError(err) {
            // No reintentar, error fatal
            return nil, fmt.Errorf("authentication failed: %w", err)
        }
        
        if openai.IsServerError(err) {
            // Reintentar servidor
            logger.Warn("OpenAI server error, retrying...", nil)
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        
        return nil, err
    }
    
    return nil, errors.New("max retries exceeded")
}
```

---

### Capa 4: Storage (Amazon S3)

**¿Qué es?** Cloud object storage para archivos

```go
package storage

import (
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Manager struct {
    client *s3.Client
    bucket string
}

// Descargar PDF
func (sm *S3Manager) DownloadPDF(ctx context.Context, materialID int64) ([]byte, error) {
    key := fmt.Sprintf("raw/%d/content.pdf", materialID)
    
    result, err := sm.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(sm.bucket),
        Key:    aws.String(key),
    })
    
    if err != nil {
        return nil, fmt.Errorf("s3 download error: %w", err)
    }
    defer result.Body.Close()
    
    data, err := io.ReadAll(result.Body)
    return data, err
}

// Subir texto extraído
func (sm *S3Manager) UploadExtractedText(
    ctx context.Context,
    materialID int64,
    text string,
) error {
    key := fmt.Sprintf("extracted/%d/content.txt", materialID)
    
    _, err := sm.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(sm.bucket),
        Key:         aws.String(key),
        Body:        bytes.NewReader([]byte(text)),
        ContentType: aws.String("text/plain"),
    })
    
    return err
}

// Subir resultados
func (sm *S3Manager) UploadAssessment(
    ctx context.Context,
    materialID int64,
    assessment *Assessment,
) error {
    key := fmt.Sprintf("processed/%d/assessment.json", materialID)
    
    data, _ := json.Marshal(assessment)
    
    _, err := sm.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(sm.bucket),
        Key:         aws.String(key),
        Body:        bytes.NewReader(data),
        ContentType: aws.String("application/json"),
        Metadata: map[string]string{
            "processed-by":    "worker-v1.0",
            "processing-time": fmt.Sprintf("%d seconds", time.Now().Unix()),
        },
    })
    
    return err
}
```

---

### Capa 5: PDF Processing

**¿Qué es?** Extracción de texto de archivos PDF

```go
package pdf

import (
    "github.com/pdfium/pdfium-go"
)

type PDFExtractor struct {
    maxPageSize int  // bytes
}

func (pe *PDFExtractor) ExtractText(ctx context.Context, pdfBytes []byte) (string, error) {
    // 1. Verificar tamaño
    if len(pdfBytes) > pe.maxPageSize {
        return "", fmt.Errorf("pdf too large: %d bytes", len(pdfBytes))
    }
    
    // 2. Parsear PDF
    doc, err := pdfium.NewDocument(pdfBytes)
    if err != nil {
        return "", fmt.Errorf("pdf parse error: %w", err)
    }
    defer doc.Close()
    
    // 3. Extraer texto página por página
    var text strings.Builder
    pageCount := doc.GetPageCount()
    
    for i := 0; i < pageCount; i++ {
        page := doc.GetPage(i)
        pageText := page.GetText()
        
        text.WriteString(fmt.Sprintf("\n--- Page %d ---\n", i+1))
        text.WriteString(pageText)
        
        page.Close()
    }
    
    // 4. Limpiar texto
    cleanedText := pe.cleanText(text.String())
    
    return cleanedText, nil
}

// Limpieza de texto
func (pe *PDFExtractor) cleanText(text string) string {
    // Remover saltos de línea extras
    re := regexp.MustCompile(`\n\s*\n`)
    text = re.ReplaceAllString(text, "\n\n")
    
    // Remover espacios extras
    re = regexp.MustCompile(`\s+`)
    text = re.ReplaceAllString(text, " ")
    
    // Normalizar caracteres
    text = strings.TrimSpace(text)
    
    return text
}
```

---

### Capa 6: Base de Datos (MongoDB)

```go
package database

import (
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/bson"
)

type MongoDB struct {
    client *mongo.Client
    db     *mongo.Database
}

func (m *MongoDB) SaveAssessment(
    ctx context.Context,
    assessment *Assessment,
) error {
    collection := m.db.Collection("material_assessment")
    
    _, err := collection.InsertOne(ctx, assessment)
    return err
}

func (m *MongoDB) GetAssessment(
    ctx context.Context,
    requestID string,
) (*Assessment, error) {
    collection := m.db.Collection("material_assessment")
    
    var assessment Assessment
    err := collection.FindOne(ctx, bson.M{
        "request_id": requestID,
    }).Decode(&assessment)
    
    return &assessment, err
}
```

**Índices:**
```javascript
db.material_assessment.createIndex({ "request_id": 1 }, { unique: true })
db.material_assessment.createIndex({ "material_id": 1 })
db.material_assessment.createIndex({ "created_at": -1 })
```

---

### Capa 7: Configuración (Viper)

```go
package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    RabbitMQ RabbitMQConfig
    OpenAI   OpenAIConfig
    AWS      AWSConfig
    MongoDB  MongoDBConfig
    Postgres PostgresConfig
    Logger   LoggerConfig
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    
    viper.AutomaticEnv()
    viper.SetEnvPrefix("WORKER")
    
    // Defaults
    viper.SetDefault("openai.temperature", 0.7)
    viper.SetDefault("openai.max_tokens", 2000)
    viper.SetDefault("worker.concurrent_jobs", 5)
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var cfg Config
    viper.Unmarshal(&cfg)
    
    return &cfg, nil
}
```

---

### Capa 8: Containerización (Docker)

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

ENV WORKER_CONCURRENT_JOBS=5
ENV LOG_LEVEL=info

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./worker"]
```

**Tamaño:** ~60MB (pequeño para un servicio de procesamiento)

---

## Flujo de Procesamiento Completo

```
┌─ Recibir mensaje de RabbitMQ
│  └─ {request_id, material_id, material_url, config}
│
├─ Descargar desde S3
│  └─ s3.GetObject(raw/{material_id}/content.pdf)
│
├─ Extraer texto del PDF
│  ├─ pdfium.ParsePDF()
│  ├─ Iterar páginas
│  └─ Limpiar texto
│
├─ Almacenar texto extraído
│  └─ s3.PutObject(extracted/{material_id}/content.txt)
│
├─ Generar preguntas con OpenAI
│  ├─ Preparar prompt
│  ├─ CreateChatCompletion(GPT-4)
│  └─ Parsear JSON respuesta
│
├─ Guardar en MongoDB
│  └─ material_assessment.InsertOne(assessment)
│
├─ Guardar resultados en S3
│  └─ s3.PutObject(processed/{material_id}/assessment.json)
│
├─ Actualizar PostgreSQL (auditoría)
│  └─ INSERT processing_requests (status: "completed")
│
├─ Publicar respuesta a RabbitMQ
│  └─ Publish(assessment.responses, assessment data)
│
└─ Acknowledge mensaje original
   └─ msg.Ack()
```

---

## Benchmarks Esperados

**Por solicitud (1 documento de 10 páginas):**
- Descargar PDF: ~2s
- Extraer texto: ~5s
- Generar preguntas (OpenAI): ~45s
- Guardar resultados: ~2s
- **Total: ~54s**

**Costos:**
- OpenAI: ~$0.075 por quiz (1500 tokens)
- S3: ~$0.001 por operación
- **Total: ~$0.076 por quiz**

**Throughput:**
- Con 5 workers concurrentes: ~333 quizzes/hora
- Con 10 workers: ~666 quizzes/hora

---

## Monitoreo y Observabilidad

```go
// Métricas importantes
- Messages consumed/second
- Average processing time (P50, P95, P99)
- Error rate by type
- OpenAI tokens used and cost
- Queue depth
- S3 bandwidth
```

---

## Comparativa: Go vs Alternativas

| Aspecto | Go | Python | Node.js |
|--------|----|---------| --------|
| Performance | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Concurrency | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| Memory | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| Startup | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐ |
| Deployment | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ |

**Por qué Go:**
- Single binary: Fácil de deployar
- Goroutines: Maneja miles concurrentes eficientemente
- Performance: Bajo CPU/Memory para worker de larga duración
- Tipado: Errores detectados en compile-time
