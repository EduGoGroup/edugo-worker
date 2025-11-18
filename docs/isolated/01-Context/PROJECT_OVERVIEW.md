# PROJECT OVERVIEW - Worker

## Información General

**Proyecto:** EduGo Worker  
**Tipo:** Servicio de Procesamiento Asíncrono  
**Tecnología:** Go 1.21+ + RabbitMQ + OpenAI  
**Especificación de Origen:** spec-02-worker  
**Estado:** En Desarrollo (Sprint 1/6)

---

## Propósito del Proyecto

Worker es un microservicio especializado en procesar solicitudes de generación automática de evaluaciones mediante IA. Procesa archivos PDF, extrae contenido, genera resúmenes y crea preguntas automáticas usando OpenAI GPT-4.

### Responsabilidades Principales
- Procesar archivos PDF subidos por docentes
- Extraer texto y contenido de documentos
- Generar resúmenes automáticos con IA
- Crear preguntas de evaluación automáticamente
- Generar quizzes diversificados (múltiple choice, verdadero/falso)
- Guardar resultados en MongoDB
- Publicar eventos de procesamiento

---

## Arquitectura del Proyecto

### Stack Tecnológico
```
┌─────────────────────────────────────────────────────┐
│ Worker (Go) - Servicio Asíncrono                   │
├─────────────────────────────────────────────────────┤
│ Listeners:                                          │
│ ├─ RabbitMQ Queue: worker.assessment.requests      │
│ └─ Procesa mensajes de API Mobile                  │
├─────────────────────────────────────────────────────┤
│ Procesamiento:                                      │
│ ├─ Descargar PDF (S3 o File Storage)               │
│ ├─ Extraer texto (PDFtoText)                       │
│ ├─ Procesar con OpenAI GPT-4                       │
│ └─ Generar preguntas                               │
├─────────────────────────────────────────────────────┤
│ Persistencia:                                       │
│ ├─ MongoDB: material_assessment collection         │
│ ├─ PostgreSQL: audit logs                          │
│ └─ RabbitMQ: eventos publicados                    │
└─────────────────────────────────────────────────────┘
```

### Estructura de Carpetas
```
worker/
├── cmd/
│   └── worker/
│       └── main.go              # Punto de entrada
├── internal/
│   ├── consumer/                # RabbitMQ consumer
│   ├── processor/               # Lógica de procesamiento
│   │   ├── pdf_extractor.go
│   │   ├── text_processor.go
│   │   └── question_generator.go
│   ├── ai/                      # Integración OpenAI
│   │   └── openai_client.go
│   ├── storage/                 # S3, Storage
│   │   └── s3_client.go
│   ├── models/
│   ├── repositories/            # Acceso a datos
│   └── config/
├── migrations/                  # Migraciones
├── docker/
│   └── Dockerfile
├── go.mod
├── go.sum
└── docker-compose.yml
```

---

## Flujos Principales

### Flujo 1: Procesar Solicitud de Generación de Quiz

```
1. API Mobile publica mensaje a RabbitMQ
   ├─ Exchange: assessment.requests
   └─ Payload: {request_id, material_id, material_url, config}

2. Worker consume mensaje
   ├─ Validar payload
   ├─ Crear registro de procesamiento en PostgreSQL
   └─ Iniciar procesamiento

3. Descargar archivo
   ├─ Obtener desde S3 usando material_url
   └─ Verificar formato (PDF)

4. Extraer contenido
   ├─ Convertir PDF a texto
   ├─ Limpiar texto
   └─ Dividir en secciones

5. Generar con OpenAI
   ├─ Enviar secciones a GPT-4
   ├─ Prompt: Generar 10 preguntas {tipo, dificultad}
   └─ Recibir preguntas generadas

6. Procesar respuesta
   ├─ Parsear preguntas JSON
   ├─ Validar estructura
   └─ Guardar en MongoDB

7. Publicar evento
   ├─ Publicar a RabbitMQ
   ├─ Exchange: assessment.responses
   └─ API Mobile consume respuesta

8. Actualizar estado
   └─ Marcar en PostgreSQL como completado
```

### Flujo 2: Generar Resumen del Material

```
1. Docente solicita resumen (vía API Mobile)
   └─ POST /api/v1/materials/{id}/summarize

2. API Mobile publica mensaje
   └─ Type: "summarize_material"

3. Worker procesa
   ├─ Descargar PDF
   ├─ Extraer contenido
   ├─ Enviar a GPT-4 con prompt de resumen
   └─ Guardar resumen en MongoDB

4. Publicar resultado
   └─ API Mobile notifica al docente
```

---

## Entidades

### PostgreSQL (Auditoría)

```sql
CREATE TABLE processing_requests (
  id VARCHAR(36) PRIMARY KEY,
  request_type VARCHAR(50), -- 'generate_quiz', 'summarize'
  material_id BIGINT,
  status VARCHAR(50), -- 'pending', 'processing', 'completed', 'error'
  progress_percent INT DEFAULT 0,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  error_message TEXT,
  created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE processed_materials (
  id BIGSERIAL PRIMARY KEY,
  material_id BIGINT,
  original_filename VARCHAR(255),
  content_size_bytes BIGINT,
  text_extracted TEXT,
  summary TEXT,
  questions_generated INT,
  processed_at TIMESTAMP DEFAULT NOW()
);
```

### MongoDB (Resultados)

```javascript
// Colección: material_assessment
{
  _id: ObjectId,
  request_id: "uuid-12345",
  material_id: 42,
  questions: [
    {
      id: "q-1",
      text: "¿Cuál es la capital de España?",
      type: "multiple_choice",
      difficulty: "easy",
      options: [
        { text: "Madrid", correct: true },
        { text: "Barcelona", correct: false }
      ],
      explanation: "Madrid es la capital...",
      source_text: "Párrafo [página 3]"
    }
  ],
  metadata: {
    model_used: "gpt-4",
    temperature: 0.7,
    max_tokens: 2000,
    generation_time_seconds: 45
  },
  created_at: "2025-11-15T10:30:00Z"
}

// Colección: material_summary
{
  _id: ObjectId,
  material_id: 42,
  title: "Resumen: Capítulo 1",
  content: "Este capítulo trata sobre...",
  key_points: ["Punto 1", "Punto 2"],
  estimated_reading_time: "10 minutos",
  generated_at: "2025-11-15T10:30:00Z"
}
```

---

## Dependencias Externas

### OpenAI API
```
Modelo: GPT-4
Características:
- Temperatura: 0.7 (creativo pero consistente)
- Max tokens: 2000 (respuestas completas)
- Timeout: 60 segundos
- Retry: 3 intentos con backoff exponencial

Prompts principales:
1. Generación de preguntas (10-20 preguntas)
2. Generación de resúmenes (500-1000 palabras)
3. Extracción de puntos clave
```

### Amazon S3 (Storage)
```
Bucket: edugo-materials
Estructura:
  ├── raw/              # PDFs originales
  │   └── {material_id}/content.pdf
  ├── extracted/        # Textos extraídos
  │   └── {material_id}/content.txt
  └── processed/        # Resultados
      └── {material_id}/assessment.json
```

### RabbitMQ (Messaging)
```
Exchanges:
  ├── assessment.requests (topic)
  └── assessment.responses (topic)
  
Queues:
  ├── worker.assessment.requests (durable)
  └── api-mobile.assessment.responses (durable)
```

---

## Procesamiento de PDF

### Extracción de Texto
```go
// Usar pdfium o similares para extraer texto
// Output: Texto limpio, preservando estructura

Input PDF:
  Chapter 1: Introduction
  ==================
  This is the first paragraph...

Output Text:
  Chapter 1: Introduction
  This is the first paragraph...
```

### Procesamiento de Contenido
```go
// 1. Dividir en secciones (máx 2000 tokens cada una)
// 2. Limpiar: remover saltos de línea extras, espacios
// 3. Detectar idioma (OCR si es necesario)
// 4. Normalizar caracteres especiales
```

---

## Integración con OpenAI

### Prompt para Generación de Preguntas

```
Sistema: Eres un profesor experto en crear preguntas educativas.

Usuario: 
Basándote en el siguiente contenido, crea 10 preguntas de múltiple opción y 5 de verdadero/falso.
Dificultad: medium
Idioma: español

Contenido:
[{contenido extraído del PDF}]

Formato requerido:
{
  "questions": [
    {
      "text": "¿Pregunta aquí?",
      "type": "multiple_choice",
      "options": [
        {"text": "Opción A", "correct": false},
        {"text": "Opción B", "correct": true},
        {"text": "Opción C", "correct": false}
      ],
      "explanation": "Explicación..."
    }
  ]
}
```

### Manejo de Errores

```go
// Si OpenAI timeout o error:
// 1. Guardar en cola de reintentos
// 2. Aplicar backoff exponencial (1s, 2s, 4s, 8s)
// 3. Máximo 3 reintentos
// 4. Si sigue fallando: notificar a API Mobile con error

// Si respuesta no es JSON válido:
// 1. Reintentar con temperature más baja
// 2. Reducir max_tokens
// 3. Simplificar prompt
```

---

## Configuración Requerida

### Variables de Entorno
```bash
# RabbitMQ
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest

# OpenAI
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4
OPENAI_TEMPERATURE=0.7
OPENAI_MAX_TOKENS=2000
OPENAI_TIMEOUT=60s

# Storage S3
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
AWS_S3_BUCKET=edugo-materials

# Base de datos
DB_HOST=localhost
DB_PORT=5432
DB_USER=edugo_user
DB_PASSWORD=password
DB_NAME=edugo_worker

MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=edugo_assessments

# Worker
WORKER_CONCURRENT_JOBS=5
WORKER_MAX_RETRIES=3
WORKER_TIMEOUT=120s
LOG_LEVEL=info
```

---

## Compilación y Despliegue

### Compilación Local
```bash
go mod download
go mod tidy
go build -o worker ./cmd/worker
./worker
```

### Compilación Docker
```bash
docker build -t edugo/worker:latest -f docker/Dockerfile .
docker run -e RABBITMQ_HOST=rabbitmq edugo/worker:latest
```

---

## Testing

### Pruebas Unitarias
```bash
# Mock de OpenAI
go test -v ./internal/ai

# Mock de S3
go test -v ./internal/storage

# Mock de RabbitMQ
go test -v ./internal/consumer
```

### Pruebas de Integración
```bash
# Con servicios reales
go test -tags=integration ./...
```

---

## Sprint Planning (6 Sprints)

| Sprint | Funcionalidad | Duración |
|--------|---------------|----------|
| 1 | Setup + Consumer RabbitMQ | 2 semanas |
| 2 | Extracción de PDF + limpieza | 2 semanas |
| 3 | Integración OpenAI | 2 semanas |
| 4 | Generación de Preguntas | 2 semanas |
| 5 | Generación de Resúmenes | 2 semanas |
| 6 | Optimización + Producción | 2 semanas |

---

## Métricas de Monitoreo

```
- Mensajes procesados/segundo
- Tiempo promedio de procesamiento (P50, P95, P99)
- Tasa de error
- Costos de OpenAI API
- Uso de CPU/Memoria
- Tamaño de queue
- Reintentos necesarios
```

---

## Contacto y Referencias

- **Repositorio GitHub:** https://github.com/EduGoGroup/edugo-worker
- **Especificación Completa:** docs/ESTADO_PROYECTO.md (repo análisis)
- **Documentación Técnica:** Este directorio (01-Context/)
