# MongoDB Schema Design - EduGo Worker

**Proyecto:** edugo-worker - Worker de procesamiento asíncrono con IA
**Versión:** 1.0.0
**Fecha:** 2025-11-18
**Autor:** Claude Code Web - Sprint-01 Fase 1
**Database:** `edugo` (MongoDB)

---

## 📋 Tabla de Contenidos

1. [Visión General](#visión-general)
2. [Collections](#collections)
   - [material_summary](#1-collection-material_summary)
   - [material_assessment](#2-collection-material_assessment)
   - [material_event](#3-collection-material_event)
3. [Diagrama de Relaciones](#diagrama-de-relaciones)
4. [Queries Comunes](#queries-comunes-optimizadas)
5. [Estrategias de Optimización](#estrategias-de-optimización)
6. [Tamaños Estimados](#tamaños-estimados-de-documentos)
7. [Backup y Mantenimiento](#estrategia-de-backup)

---

## 🎯 Visión General

### Propósito

Este documento define los schemas MongoDB para el sistema **edugo-worker**, que procesa materiales educativos con IA (OpenAI GPT-4) para generar:
- **Resúmenes** inteligentes de contenido
- **Evaluaciones** (quizzes) automáticos
- **Auditoría** de eventos procesados

### Arquitectura

```
PostgreSQL (edugo)           MongoDB (edugo)
┌─────────────────┐         ┌──────────────────────┐
│ materials       │         │ material_summary     │
│  - id (UUID)    │ ──1:1──→│  - material_id (idx) │
│  - title        │         │  - summary           │
│  - author_id    │         │  - key_points        │
│  - s3_key       │         └──────────────────────┘
│  - status       │                    │
└─────────────────┘                    │ 1:1
                                       ▼
                            ┌──────────────────────┐
                            │ material_assessment  │
                            │  - material_id (idx) │
                            │  - questions[]       │
                            │  - total_points      │
                            └──────────────────────┘

RabbitMQ Events                        │
┌──────────────────┐                   │
│ material_*       │                   │ Auditoría
│ assessment_*     │ ────────────────→ ▼
│ student_*        │         ┌──────────────────────┐
└──────────────────┘         │ material_event       │
                             │  - event_type        │
                             │  - payload           │
                             │  - status            │
                             └──────────────────────┘
```

### Filosofía de Diseño

1. **Relación 1:1 con PostgreSQL**: Cada material en PostgreSQL tiene exactamente un summary y un assessment en MongoDB
2. **Versionado**: Los schemas soportan reprocesamiento incrementando `version`
3. **Auditoría completa**: Todos los eventos se registran en `material_event` con TTL de 90 días
4. **Validación estricta**: MongoDB validation schemas garantizan integridad de datos
5. **Performance**: Índices optimizados para queries frecuentes

---

## 📊 Collections

---

## 1. Collection: `material_summary`

### Propósito
Almacena resúmenes de materiales educativos generados con IA (OpenAI GPT-4).

### Schema Definition

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `_id` | ObjectId | Auto | Identificador único de MongoDB |
| `material_id` | String (UUID) | ✅ | UUID del material en PostgreSQL (UNIQUE INDEX) |
| `summary` | String | ✅ | Resumen completo generado por OpenAI |
| `key_points` | Array\<String\> | ✅ | Puntos clave extraídos (min 1, max 10) |
| `language` | String | ✅ | Idioma del resumen: "es", "en", "pt" |
| `word_count` | Number | ✅ | Número de palabras del resumen |
| `version` | Number | ✅ | Versión del resumen (incrementa en reprocesos) |
| `ai_model` | String | ✅ | Modelo IA usado: "gpt-4", "gpt-3.5-turbo", "gpt-4-turbo" |
| `processing_time_ms` | Number | ✅ | Tiempo de procesamiento en milisegundos |
| `token_usage` | Object | ❌ | Metadata de tokens consumidos |
| `token_usage.prompt_tokens` | Number | ❌ | Tokens del prompt |
| `token_usage.completion_tokens` | Number | ❌ | Tokens de la respuesta |
| `token_usage.total_tokens` | Number | ❌ | Total de tokens |
| `metadata` | Object | ❌ | Metadata adicional |
| `metadata.source_length` | Number | ❌ | Longitud del texto fuente |
| `metadata.has_images` | Boolean | ❌ | Si el material tiene imágenes |
| `created_at` | Date | ✅ | Fecha de creación |
| `updated_at` | Date | ✅ | Fecha de última actualización |

### Índices

```javascript
// Índice único en material_id (para búsquedas rápidas y garantizar 1:1)
db.material_summary.createIndex(
  { material_id: 1 },
  { unique: true, name: "idx_material_id" }
);

// Índice en created_at (para queries temporales y ordenamiento)
db.material_summary.createIndex(
  { created_at: -1 },
  { name: "idx_created_at" }
);

// Índice en version (para consultas de versiones)
db.material_summary.createIndex(
  { version: 1 },
  { name: "idx_version" }
);

// Índice compuesto para búsquedas por idioma y fecha
db.material_summary.createIndex(
  { language: 1, created_at: -1 },
  { name: "idx_language_created" }
);
```

### Validation Schema (MongoDB)

```javascript
{
  $jsonSchema: {
    bsonType: "object",
    required: ["material_id", "summary", "key_points", "language", "word_count", "version", "ai_model", "processing_time_ms", "created_at", "updated_at"],
    properties: {
      material_id: {
        bsonType: "string",
        pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
        description: "UUID v4 del material en PostgreSQL"
      },
      summary: {
        bsonType: "string",
        minLength: 10,
        maxLength: 5000,
        description: "Resumen generado por IA (min 10 caracteres)"
      },
      key_points: {
        bsonType: "array",
        minItems: 1,
        maxItems: 10,
        items: {
          bsonType: "string",
          minLength: 5,
          maxLength: 500
        },
        description: "Array de puntos clave (1-10 elementos)"
      },
      language: {
        enum: ["es", "en", "pt"],
        description: "Idioma del resumen"
      },
      word_count: {
        bsonType: "int",
        minimum: 1,
        description: "Número de palabras del resumen"
      },
      version: {
        bsonType: "int",
        minimum: 1,
        description: "Versión del resumen (>= 1)"
      },
      ai_model: {
        enum: ["gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"],
        description: "Modelo de IA utilizado"
      },
      processing_time_ms: {
        bsonType: "int",
        minimum: 0,
        description: "Tiempo de procesamiento en ms"
      },
      token_usage: {
        bsonType: "object",
        properties: {
          prompt_tokens: { bsonType: "int", minimum: 0 },
          completion_tokens: { bsonType: "int", minimum: 0 },
          total_tokens: { bsonType: "int", minimum: 0 }
        }
      },
      metadata: {
        bsonType: "object",
        properties: {
          source_length: { bsonType: "int", minimum: 0 },
          has_images: { bsonType: "bool" }
        }
      },
      created_at: {
        bsonType: "date",
        description: "Fecha de creación"
      },
      updated_at: {
        bsonType: "date",
        description: "Fecha de última actualización"
      }
    }
  }
}
```

### Ejemplo de Documento

```javascript
{
  "_id": ObjectId("65a1b2c3d4e5f6a7b8c9d0e1"),
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "summary": "Este material introduce los conceptos fundamentales de MongoDB, una base de datos NoSQL orientada a documentos. Se explican las diferencias con bases de datos relacionales, los casos de uso apropiados y las ventajas de escalabilidad horizontal. El documento cubre la estructura de documentos BSON, collections y las operaciones CRUD básicas.",
  "key_points": [
    "MongoDB es una base de datos NoSQL orientada a documentos",
    "Usa BSON (Binary JSON) para almacenar datos",
    "Soporta escalabilidad horizontal mediante sharding",
    "No requiere schema fijo como SQL",
    "Ideal para datos semi-estructurados y alta concurrencia"
  ],
  "language": "es",
  "word_count": 87,
  "version": 1,
  "ai_model": "gpt-4",
  "processing_time_ms": 2340,
  "token_usage": {
    "prompt_tokens": 850,
    "completion_tokens": 120,
    "total_tokens": 970
  },
  "metadata": {
    "source_length": 4500,
    "has_images": true
  },
  "created_at": ISODate("2025-11-18T10:30:00Z"),
  "updated_at": ISODate("2025-11-18T10:30:00Z")
}
```

---

## 2. Collection: `material_assessment`

### Propósito
Almacena evaluaciones (quizzes) generados automáticamente con IA para cada material educativo.

### Schema Definition

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `_id` | ObjectId | Auto | Identificador único de MongoDB |
| `material_id` | String (UUID) | ✅ | UUID del material en PostgreSQL (UNIQUE INDEX) |
| `title` | String | ✅ | Título del quiz |
| `description` | String | ❌ | Descripción del quiz |
| `questions` | Array\<Question\> | ✅ | Array de preguntas (min 1, max 50) |
| `total_questions` | Number | ✅ | Cantidad total de preguntas |
| `total_points` | Number | ✅ | Suma total de puntos del quiz |
| `passing_score` | Number | ✅ | Puntaje mínimo para aprobar (% de total_points) |
| `time_limit_minutes` | Number | ❌ | Tiempo límite en minutos (null = sin límite) |
| `difficulty_distribution` | Object | ❌ | Distribución de dificultad |
| `version` | Number | ✅ | Versión del assessment (incrementa en reprocesos) |
| `ai_model` | String | ✅ | Modelo IA usado |
| `processing_time_ms` | Number | ✅ | Tiempo de procesamiento en ms |
| `created_at` | Date | ✅ | Fecha de creación |
| `updated_at` | Date | ✅ | Fecha de última actualización |

### Sub-Schema: Question

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `id` | String (UUID) | ✅ | UUID único de la pregunta |
| `text` | String | ✅ | Texto de la pregunta |
| `type` | String | ✅ | Tipo: "multiple_choice", "true_false", "open" |
| `difficulty` | String | ✅ | Dificultad: "easy", "medium", "hard" |
| `points` | Number | ✅ | Puntaje de la pregunta |
| `options` | Array\<Option\> | Condicional | Opciones (requerido para multiple_choice) |
| `correct_answer` | String | Condicional | Respuesta correcta (para true_false: "true"/"false") |
| `explanation` | String | ✅ | Explicación de la respuesta correcta |
| `bloom_taxonomy_level` | String | ❌ | Nivel de taxonomía de Bloom |
| `order` | Number | ✅ | Orden de la pregunta en el quiz |

### Sub-Schema: Option (para preguntas multiple_choice)

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `id` | String (UUID) | ✅ | UUID único de la opción |
| `text` | String | ✅ | Texto de la opción |
| `is_correct` | Boolean | ✅ | Si es la respuesta correcta |
| `order` | Number | ✅ | Orden de visualización |

### Índices

```javascript
// Índice único en material_id
db.material_assessment.createIndex(
  { material_id: 1 },
  { unique: true, name: "idx_material_id" }
);

// Índice en created_at
db.material_assessment.createIndex(
  { created_at: -1 },
  { name: "idx_created_at" }
);

// Índice en version
db.material_assessment.createIndex(
  { version: 1 },
  { name: "idx_version" }
);

// Índice en questions.difficulty (para filtrar por dificultad)
db.material_assessment.createIndex(
  { "questions.difficulty": 1 },
  { name: "idx_questions_difficulty" }
);

// Índice compuesto para queries complejas
db.material_assessment.createIndex(
  { total_questions: 1, created_at: -1 },
  { name: "idx_total_questions_created" }
);
```

### Validation Schema (MongoDB)

```javascript
{
  $jsonSchema: {
    bsonType: "object",
    required: ["material_id", "title", "questions", "total_questions", "total_points", "passing_score", "version", "ai_model", "processing_time_ms", "created_at", "updated_at"],
    properties: {
      material_id: {
        bsonType: "string",
        pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
        description: "UUID v4 del material"
      },
      title: {
        bsonType: "string",
        minLength: 3,
        maxLength: 200,
        description: "Título del quiz"
      },
      description: {
        bsonType: "string",
        maxLength: 1000,
        description: "Descripción opcional del quiz"
      },
      questions: {
        bsonType: "array",
        minItems: 1,
        maxItems: 50,
        items: {
          bsonType: "object",
          required: ["id", "text", "type", "difficulty", "points", "explanation", "order"],
          properties: {
            id: {
              bsonType: "string",
              pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
            },
            text: {
              bsonType: "string",
              minLength: 10,
              maxLength: 1000
            },
            type: {
              enum: ["multiple_choice", "true_false", "open"]
            },
            difficulty: {
              enum: ["easy", "medium", "hard"]
            },
            points: {
              bsonType: "int",
              minimum: 1,
              maximum: 100
            },
            options: {
              bsonType: "array",
              minItems: 2,
              maxItems: 6,
              items: {
                bsonType: "object",
                required: ["id", "text", "is_correct", "order"],
                properties: {
                  id: { bsonType: "string" },
                  text: { bsonType: "string", minLength: 1, maxLength: 500 },
                  is_correct: { bsonType: "bool" },
                  order: { bsonType: "int", minimum: 1 }
                }
              }
            },
            correct_answer: {
              bsonType: "string"
            },
            explanation: {
              bsonType: "string",
              minLength: 10,
              maxLength: 1000
            },
            bloom_taxonomy_level: {
              enum: ["remember", "understand", "apply", "analyze", "evaluate", "create"]
            },
            order: {
              bsonType: "int",
              minimum: 1
            }
          }
        }
      },
      total_questions: {
        bsonType: "int",
        minimum: 1,
        maximum: 50
      },
      total_points: {
        bsonType: "int",
        minimum: 1
      },
      passing_score: {
        bsonType: "int",
        minimum: 0
      },
      time_limit_minutes: {
        bsonType: ["int", "null"],
        minimum: 1
      },
      difficulty_distribution: {
        bsonType: "object",
        properties: {
          easy: { bsonType: "int", minimum: 0 },
          medium: { bsonType: "int", minimum: 0 },
          hard: { bsonType: "int", minimum: 0 }
        }
      },
      version: {
        bsonType: "int",
        minimum: 1
      },
      ai_model: {
        enum: ["gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"]
      },
      processing_time_ms: {
        bsonType: "int",
        minimum: 0
      },
      created_at: {
        bsonType: "date"
      },
      updated_at: {
        bsonType: "date"
      }
    }
  }
}
```

### Ejemplo de Documento

```javascript
{
  "_id": ObjectId("65a1b2c3d4e5f6a7b8c9d0e2"),
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "Quiz: Fundamentos de MongoDB",
  "description": "Evaluación sobre conceptos básicos de MongoDB y bases de datos NoSQL",
  "questions": [
    {
      "id": "q-f3e4d5c6-b7a8-4c3d-9e2f-1a0b9c8d7e6f",
      "text": "¿Qué es MongoDB?",
      "type": "multiple_choice",
      "difficulty": "easy",
      "points": 5,
      "options": [
        {
          "id": "opt-1",
          "text": "Una base de datos relacional como MySQL",
          "is_correct": false,
          "order": 1
        },
        {
          "id": "opt-2",
          "text": "Una base de datos NoSQL orientada a documentos",
          "is_correct": true,
          "order": 2
        },
        {
          "id": "opt-3",
          "text": "Un lenguaje de programación",
          "is_correct": false,
          "order": 3
        },
        {
          "id": "opt-4",
          "text": "Un sistema operativo",
          "is_correct": false,
          "order": 4
        }
      ],
      "explanation": "MongoDB es una base de datos NoSQL que almacena datos en documentos BSON (Binary JSON), permitiendo flexibilidad en el schema y escalabilidad horizontal.",
      "bloom_taxonomy_level": "remember",
      "order": 1
    },
    {
      "id": "q-a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
      "text": "MongoDB soporta transacciones ACID desde la versión 4.0",
      "type": "true_false",
      "difficulty": "medium",
      "points": 5,
      "correct_answer": "true",
      "explanation": "A partir de MongoDB 4.0, se introdujo soporte completo para transacciones multi-documento ACID, similar a bases de datos relacionales.",
      "bloom_taxonomy_level": "understand",
      "order": 2
    },
    {
      "id": "q-b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
      "text": "Explica la diferencia entre sharding y replicación en MongoDB",
      "type": "open",
      "difficulty": "hard",
      "points": 10,
      "explanation": "Sharding es la distribución horizontal de datos entre múltiples servidores para escalar, mientras que replicación crea copias de los datos para alta disponibilidad y redundancia.",
      "bloom_taxonomy_level": "analyze",
      "order": 3
    }
  ],
  "total_questions": 3,
  "total_points": 20,
  "passing_score": 12,
  "time_limit_minutes": 15,
  "difficulty_distribution": {
    "easy": 1,
    "medium": 1,
    "hard": 1
  },
  "version": 1,
  "ai_model": "gpt-4",
  "processing_time_ms": 3500,
  "created_at": ISODate("2025-11-18T10:30:05Z"),
  "updated_at": ISODate("2025-11-18T10:30:05Z")
}
```

---

## 3. Collection: `material_event`

### Propósito
Auditoría completa de todos los eventos procesados por el worker. Útil para debugging, monitoreo y análisis de rendimiento.

### Schema Definition

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| `_id` | ObjectId | Auto | Identificador único de MongoDB |
| `event_type` | String | ✅ | Tipo de evento procesado |
| `event_id` | String (UUID) | ❌ | ID único del evento (si aplicable) |
| `material_id` | String (UUID) | ❌ | UUID del material (nullable para eventos sin material) |
| `user_id` | String (UUID) | ❌ | UUID del usuario (nullable) |
| `payload` | Object | ✅ | Payload completo del evento (JSON original) |
| `status` | String | ✅ | Estado del procesamiento |
| `error_message` | String | ❌ | Mensaje de error si status = "failed" |
| `error_stack` | String | ❌ | Stack trace del error |
| `retry_count` | Number | ❌ | Número de reintentos |
| `processing_time_ms` | Number | ❌ | Tiempo de procesamiento en ms |
| `processed_at` | Date | ❌ | Fecha/hora de procesamiento completado |
| `created_at` | Date | ✅ | Fecha/hora de recepción del evento |

### Índices

```javascript
// Índice en event_type (para filtrar por tipo de evento)
db.material_event.createIndex(
  { event_type: 1 },
  { name: "idx_event_type" }
);

// Índice en material_id (para auditoría por material)
db.material_event.createIndex(
  { material_id: 1 },
  { name: "idx_material_id" }
);

// Índice en status (para monitoreo de fallos)
db.material_event.createIndex(
  { status: 1 },
  { name: "idx_status" }
);

// Índice en created_at (para queries temporales)
db.material_event.createIndex(
  { created_at: -1 },
  { name: "idx_created_at" }
);

// Índice en processed_at (para métricas de rendimiento)
db.material_event.createIndex(
  { processed_at: -1 },
  { name: "idx_processed_at" }
);

// Índice compuesto para queries de monitoreo
db.material_event.createIndex(
  { status: 1, created_at: -1 },
  { name: "idx_status_created" }
);

// ⚠️ TTL Index: Eliminar eventos después de 90 días
db.material_event.createIndex(
  { created_at: 1 },
  { expireAfterSeconds: 7776000, name: "idx_ttl_created_at" }
);
```

### Validation Schema (MongoDB)

```javascript
{
  $jsonSchema: {
    bsonType: "object",
    required: ["event_type", "payload", "status", "created_at"],
    properties: {
      event_type: {
        enum: [
          "material_uploaded",
          "material_reprocess",
          "material_deleted",
          "assessment_attempt",
          "student_enrolled"
        ],
        description: "Tipo de evento procesado"
      },
      event_id: {
        bsonType: "string",
        description: "ID único del evento"
      },
      material_id: {
        bsonType: ["string", "null"],
        pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
        description: "UUID del material (nullable)"
      },
      user_id: {
        bsonType: ["string", "null"],
        pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
        description: "UUID del usuario (nullable)"
      },
      payload: {
        bsonType: "object",
        description: "Payload completo del evento"
      },
      status: {
        enum: ["pending", "processing", "completed", "failed"],
        description: "Estado del procesamiento"
      },
      error_message: {
        bsonType: "string",
        maxLength: 1000,
        description: "Mensaje de error si falló"
      },
      error_stack: {
        bsonType: "string",
        maxLength: 5000,
        description: "Stack trace del error"
      },
      retry_count: {
        bsonType: "int",
        minimum: 0,
        maximum: 10,
        description: "Número de reintentos"
      },
      processing_time_ms: {
        bsonType: "int",
        minimum: 0,
        description: "Tiempo de procesamiento"
      },
      processed_at: {
        bsonType: ["date", "null"],
        description: "Fecha de procesamiento completado"
      },
      created_at: {
        bsonType: "date",
        description: "Fecha de recepción del evento"
      }
    }
  }
}
```

### Ejemplo de Documento (Success)

```javascript
{
  "_id": ObjectId("65a1b2c3d4e5f6a7b8c9d0e3"),
  "event_type": "material_uploaded",
  "event_id": "evt-550e8400-e29b-41d4-a716-446655440001",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user-123e4567-e89b-12d3-a456-426614174000",
  "payload": {
    "event_type": "material_uploaded",
    "material_id": "550e8400-e29b-41d4-a716-446655440000",
    "author_id": "user-123e4567-e89b-12d3-a456-426614174000",
    "s3_key": "materials/2025/11/18/mongodb-intro.pdf",
    "preferred_language": "es",
    "timestamp": "2025-11-18T10:29:45Z"
  },
  "status": "completed",
  "processing_time_ms": 5840,
  "processed_at": ISODate("2025-11-18T10:30:06Z"),
  "created_at": ISODate("2025-11-18T10:30:00Z")
}
```

### Ejemplo de Documento (Error)

```javascript
{
  "_id": ObjectId("65a1b2c3d4e5f6a7b8c9d0e4"),
  "event_type": "material_uploaded",
  "event_id": "evt-650e8400-e29b-41d4-a716-446655440002",
  "material_id": "650e8400-e29b-41d4-a716-446655440001",
  "user_id": null,
  "payload": {
    "event_type": "material_uploaded",
    "material_id": "650e8400-e29b-41d4-a716-446655440001",
    "author_id": "user-invalid-uuid",
    "s3_key": "materials/corrupted.pdf",
    "preferred_language": "es",
    "timestamp": "2025-11-18T11:00:00Z"
  },
  "status": "failed",
  "error_message": "failed to extract PDF text: file corrupted",
  "error_stack": "goroutine 42 [running]:\ngithub.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf.(*Extractor).Extract(...)\n\t/app/internal/infrastructure/pdf/extractor.go:45",
  "retry_count": 3,
  "processing_time_ms": 1200,
  "processed_at": ISODate("2025-11-18T11:00:08Z"),
  "created_at": ISODate("2025-11-18T11:00:00Z")
}
```

---

## 🔗 Diagrama de Relaciones

### Relación PostgreSQL ↔ MongoDB

```
┌─────────────────────────────────────────────────────────────────┐
│                        PostgreSQL (edugo)                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  materials                                                      │
│  ┌─────────────────────────────────────────────────────┐       │
│  │ id (UUID PK)                                         │       │
│  │ title                                                │       │
│  │ author_id (UUID FK → users.id)                      │       │
│  │ s3_key                                               │       │
│  │ processing_status (enum)                             │       │
│  │ created_at                                           │       │
│  │ updated_at                                           │       │
│  └─────────────────────────────────────────────────────┘       │
│              │                                                  │
└──────────────┼──────────────────────────────────────────────────┘
               │ 1:1
               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        MongoDB (edugo)                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  material_summary                                               │
│  ┌─────────────────────────────────────────────────────┐       │
│  │ _id (ObjectId)                                       │       │
│  │ material_id (String UUID, UNIQUE INDEX) ←───────────┼─────  │
│  │ summary                                              │       │
│  │ key_points[]                                         │       │
│  │ language                                             │       │
│  │ word_count                                           │       │
│  │ version                                              │       │
│  │ ai_model                                             │       │
│  │ processing_time_ms                                   │       │
│  │ created_at                                           │       │
│  │ updated_at                                           │       │
│  └─────────────────────────────────────────────────────┘       │
│              │ 1:1 (mismo material_id)                          │
│              ▼                                                  │
│  material_assessment                                            │
│  ┌─────────────────────────────────────────────────────┐       │
│  │ _id (ObjectId)                                       │       │
│  │ material_id (String UUID, UNIQUE INDEX) ←───────────┼─────  │
│  │ title                                                │       │
│  │ questions[]                                          │       │
│  │   - id (UUID)                                        │       │
│  │   - text                                             │       │
│  │   - type (enum)                                      │       │
│  │   - difficulty (enum)                                │       │
│  │   - points                                           │       │
│  │   - options[] (para multiple_choice)                 │       │
│  │     - id (UUID)                                      │       │
│  │     - text                                           │       │
│  │     - is_correct                                     │       │
│  │   - correct_answer (para true_false)                 │       │
│  │   - explanation                                      │       │
│  │ total_questions                                      │       │
│  │ total_points                                         │       │
│  │ passing_score                                        │       │
│  │ version                                              │       │
│  │ ai_model                                             │       │
│  │ created_at                                           │       │
│  │ updated_at                                           │       │
│  └─────────────────────────────────────────────────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     RabbitMQ Events                             │
├─────────────────────────────────────────────────────────────────┤
│  - material_uploaded                                            │
│  - material_reprocess                                           │
│  - material_deleted                                             │
│  - assessment_attempt                                           │
│  - student_enrolled                                             │
└──────────────────┬──────────────────────────────────────────────┘
                   │ Auditoría
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│  material_event                                                 │
│  ┌─────────────────────────────────────────────────────┐       │
│  │ _id (ObjectId)                                       │       │
│  │ event_type (enum)                                    │       │
│  │ material_id (nullable)                               │       │
│  │ payload (object)                                     │       │
│  │ status (enum)                                        │       │
│  │ error_message (nullable)                             │       │
│  │ processing_time_ms                                   │       │
│  │ processed_at                                         │       │
│  │ created_at (TTL: 90 días)                           │       │
│  └─────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

### Flujo de Datos

```
1. Material Uploaded (RabbitMQ)
   ↓
2. MaterialUploadedProcessor
   ↓
3. Extrae texto PDF (S3)
   ↓
4. Genera resumen (OpenAI GPT-4)
   ↓
5. INSERT material_summary (MongoDB)
   ↓
6. Genera quiz (OpenAI GPT-4)
   ↓
7. INSERT material_assessment (MongoDB)
   ↓
8. UPDATE materials.processing_status = 'completed' (PostgreSQL)
   ↓
9. INSERT material_event (Auditoría)
```

---

## 🔍 Queries Comunes Optimizadas

### 1. Buscar resumen por material_id

```javascript
// Optimizado con índice único idx_material_id
db.material_summary.findOne({ material_id: "550e8400-e29b-41d4-a716-446655440000" });

// Explicación de índice usado
db.material_summary.find({ material_id: "550e8400-e29b-41d4-a716-446655440000" }).explain("executionStats");
```

**Performance:**
- Complejidad: O(log n)
- Index Scan: `idx_material_id` (unique)
- Tiempo estimado: < 5ms para 1M documentos

---

### 2. Buscar quiz por material_id

```javascript
// Optimizado con índice único idx_material_id
db.material_assessment.findOne({ material_id: "550e8400-e29b-41d4-a716-446655440000" });
```

**Performance:**
- Complejidad: O(log n)
- Index Scan: `idx_material_id` (unique)
- Tiempo estimado: < 5ms para 1M documentos

---

### 3. Listar resúmenes recientes (paginado)

```javascript
// Optimizado con índice idx_created_at
db.material_summary
  .find()
  .sort({ created_at: -1 })
  .limit(20)
  .skip(0);
```

**Performance:**
- Index Scan: `idx_created_at`
- Tiempo estimado: < 10ms

---

### 4. Buscar resúmenes por idioma (últimos 30 días)

```javascript
// Optimizado con índice compuesto idx_language_created
const thirtyDaysAgo = new Date();
thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);

db.material_summary
  .find({
    language: "es",
    created_at: { $gte: thirtyDaysAgo }
  })
  .sort({ created_at: -1 });
```

**Performance:**
- Index Scan: `idx_language_created`
- Tiempo estimado: < 15ms

---

### 5. Auditar eventos fallidos (últimas 24 horas)

```javascript
// Optimizado con índice compuesto idx_status_created
const yesterday = new Date();
yesterday.setDate(yesterday.getDate() - 1);

db.material_event
  .find({
    status: "failed",
    created_at: { $gte: yesterday }
  })
  .sort({ created_at: -1 });
```

**Performance:**
- Index Scan: `idx_status_created`
- Tiempo estimado: < 10ms

---

### 6. Buscar todos los eventos de un material

```javascript
// Optimizado con índice idx_material_id
db.material_event
  .find({ material_id: "550e8400-e29b-41d4-a716-446655440000" })
  .sort({ created_at: -1 });
```

**Performance:**
- Index Scan: `idx_material_id`
- Tiempo estimado: < 10ms

---

### 7. Obtener estadísticas de procesamiento

```javascript
// Agregación optimizada con índices
db.material_event.aggregate([
  {
    $match: {
      created_at: { $gte: new Date("2025-11-01") },
      status: "completed"
    }
  },
  {
    $group: {
      _id: "$event_type",
      count: { $sum: 1 },
      avg_processing_time: { $avg: "$processing_time_ms" },
      max_processing_time: { $max: "$processing_time_ms" },
      min_processing_time: { $min: "$processing_time_ms" }
    }
  },
  {
    $sort: { count: -1 }
  }
]);
```

**Performance:**
- Index Scan: `idx_status_created`
- Tiempo estimado: < 50ms para 100K eventos

---

### 8. Buscar quizzes por dificultad

```javascript
// Optimizado con índice idx_questions_difficulty
db.material_assessment
  .find({ "questions.difficulty": "hard" })
  .limit(10);
```

**Performance:**
- Index Scan: `idx_questions_difficulty`
- Tiempo estimado: < 20ms

---

### 9. Reprocesamiento: Buscar última versión

```javascript
// Optimizado con índices idx_material_id + idx_version
db.material_summary
  .find({ material_id: "550e8400-e29b-41d4-a716-446655440000" })
  .sort({ version: -1 })
  .limit(1);
```

**Performance:**
- Index Scan: `idx_material_id` + `idx_version`
- Tiempo estimado: < 5ms

---

### 10. Limpieza: Eliminar material completo

```javascript
// Transacción multi-documento (MongoDB 4.0+)
session = db.getMongo().startSession();
session.startTransaction();

try {
  const materialId = "550e8400-e29b-41d4-a716-446655440000";

  // Eliminar summary
  db.material_summary.deleteOne(
    { material_id: materialId },
    { session }
  );

  // Eliminar assessment
  db.material_assessment.deleteOne(
    { material_id: materialId },
    { session }
  );

  session.commitTransaction();
} catch (error) {
  session.abortTransaction();
  throw error;
} finally {
  session.endSession();
}
```

---

## 🚀 Estrategias de Optimización

### 1. Índices

**Principios aplicados:**
- ✅ Índice único en `material_id` (relación 1:1 con PostgreSQL)
- ✅ Índices en campos frecuentemente consultados (`created_at`, `status`, `event_type`)
- ✅ Índices compuestos para queries complejas (`language + created_at`, `status + created_at`)
- ✅ TTL index en `material_event` para limpieza automática

**Monitoreo de índices:**
```javascript
// Ver índices en uso
db.material_summary.getIndexes();
db.material_assessment.getIndexes();
db.material_event.getIndexes();

// Analizar performance de query
db.material_summary.find({ language: "es" }).explain("executionStats");
```

---

### 2. Tamaño de Documentos

**Límites recomendados:**
- `material_summary`: ~5-15 KB (summary + key_points + metadata)
- `material_assessment`: ~20-50 KB (dependiendo de número de preguntas)
- `material_event`: ~2-10 KB (payload puede variar)

**Límite BSON de MongoDB:** 16 MB
**Margen de seguridad:** Todos los documentos < 100 KB

---

### 3. Sharding (Futuro)

**Estrategia recomendada:**

Si el volumen crece > 1M documentos o > 100 GB:

```javascript
// Shard key en material_id (distribución uniforme)
sh.enableSharding("edugo");

sh.shardCollection("edugo.material_summary", { material_id: "hashed" });
sh.shardCollection("edugo.material_assessment", { material_id: "hashed" });

// Para material_event, shard por created_at (range-based)
sh.shardCollection("edugo.material_event", { created_at: 1 });
```

**Ventajas:**
- Distribución uniforme con hashed shard key
- Queries por `material_id` son eficientes (single shard)
- TTL index funciona en entorno sharded

---

### 4. Write Concern

**Configuración recomendada:**

```javascript
// Para operaciones críticas (summary, assessment)
db.material_summary.insertOne(
  { /* document */ },
  { writeConcern: { w: "majority", j: true } }
);

// Para auditoría (material_event) - menor criticidad
db.material_event.insertOne(
  { /* document */ },
  { writeConcern: { w: 1 } }
);
```

---

### 5. Read Preference

**Configuración recomendada:**

```javascript
// Lecturas de material_summary/assessment (datos críticos)
db.material_summary.find().readPref("primaryPreferred");

// Lecturas de material_event (auditoría/analytics)
db.material_event.find().readPref("secondaryPreferred");
```

---

## 📏 Tamaños Estimados de Documentos

### material_summary

**Tamaño promedio:** ~8 KB

**Cálculo:**
- `material_id`: 36 bytes (UUID string)
- `summary`: ~2000 bytes (500 palabras promedio)
- `key_points`: ~500 bytes (5 puntos × 100 bytes)
- `language`: 2 bytes
- `word_count`: 8 bytes (int64)
- `version`: 8 bytes
- `ai_model`: 15 bytes
- `processing_time_ms`: 8 bytes
- `token_usage`: ~50 bytes
- `metadata`: ~100 bytes
- `created_at`: 8 bytes
- `updated_at`: 8 bytes
- **Overhead BSON:** ~500 bytes

**Total:** ~3,243 bytes ≈ **3-8 KB**

**Almacenamiento para 1M materiales:** ~8 GB

---

### material_assessment

**Tamaño promedio:** ~25 KB

**Cálculo:**
- `material_id`: 36 bytes
- `title`: 100 bytes
- `description`: 200 bytes
- `questions` (10 preguntas promedio):
  - Cada pregunta: ~200 bytes (texto, tipo, dificultad, puntos, explicación)
  - Cada opción (4 opciones × 10 preguntas): ~40 opciones × 100 bytes = 4,000 bytes
  - **Total questions:** ~6,000 bytes
- `total_questions`: 8 bytes
- `total_points`: 8 bytes
- `passing_score`: 8 bytes
- `time_limit_minutes`: 8 bytes
- `difficulty_distribution`: ~50 bytes
- `version`: 8 bytes
- `ai_model`: 15 bytes
- `processing_time_ms`: 8 bytes
- `created_at`: 8 bytes
- `updated_at`: 8 bytes
- **Overhead BSON:** ~1,000 bytes

**Total:** ~7,457 bytes ≈ **7-25 KB**

**Almacenamiento para 1M materiales:** ~25 GB

---

### material_event

**Tamaño promedio:** ~3 KB

**Cálculo:**
- `event_type`: 30 bytes
- `event_id`: 36 bytes
- `material_id`: 36 bytes
- `user_id`: 36 bytes
- `payload`: ~1,000 bytes (JSON del evento)
- `status`: 15 bytes
- `error_message`: ~200 bytes (si existe)
- `error_stack`: ~500 bytes (si existe)
- `retry_count`: 8 bytes
- `processing_time_ms`: 8 bytes
- `processed_at`: 8 bytes
- `created_at`: 8 bytes
- **Overhead BSON:** ~500 bytes

**Total:** ~2,385 bytes ≈ **2-5 KB**

**Almacenamiento para 1M eventos:** ~5 GB
**Con TTL (90 días):** ~1-2 GB (rotación automática)

---

### Resumen de Almacenamiento

| Collection | Docs (estimado) | Tamaño/doc | Total | Con Índices |
|------------|-----------------|------------|-------|-------------|
| `material_summary` | 1M | 8 KB | ~8 GB | ~10 GB |
| `material_assessment` | 1M | 25 KB | ~25 GB | ~30 GB |
| `material_event` | 500K (con TTL) | 3 KB | ~1.5 GB | ~2 GB |
| **TOTAL** | **2.5M docs** | - | **~35 GB** | **~42 GB** |

**Proyección a 5M materiales:** ~100 GB
**Proyección a 10M materiales:** ~200 GB

---

## 💾 Estrategia de Backup

### 1. Backup Completo Diario

**Herramienta:** `mongodump`

```bash
# Backup completo de la base de datos edugo
mongodump --uri="mongodb://user:pass@localhost:27017/edugo" \
  --out=/backups/mongodb/$(date +%Y%m%d) \
  --gzip

# Retención: 7 días
find /backups/mongodb -type d -mtime +7 -exec rm -rf {} \;
```

---

### 2. Backup Incremental (Oplog)

**Herramienta:** MongoDB Atlas Continuous Backup o `mongodump --oplog`

```bash
# Backup incremental con oplog
mongodump --uri="mongodb://user:pass@localhost:27017/edugo" \
  --oplog \
  --out=/backups/mongodb/incremental/$(date +%Y%m%d_%H%M%S) \
  --gzip
```

**Retención:**
- Últimas 24 horas: cada 1 hora
- Últimos 7 días: cada 12 horas
- Últimos 30 días: diario

---

### 3. Point-in-Time Recovery

**MongoDB Atlas:** Habilitar continuous backup
**Self-hosted:** Configurar replica set + oplog

```javascript
// Verificar tamaño del oplog
use local
db.oplog.rs.stats()

// Configurar oplog size (mínimo 24 horas de retención)
```

---

### 4. Estrategia de Restauración

**Restauración completa:**

```bash
mongorestore --uri="mongodb://user:pass@localhost:27017" \
  --gzip \
  /backups/mongodb/20251118
```

**Restauración selectiva (solo material_summary):**

```bash
mongorestore --uri="mongodb://user:pass@localhost:27017" \
  --gzip \
  --nsInclude="edugo.material_summary" \
  /backups/mongodb/20251118
```

**Restauración point-in-time:**

```bash
# 1. Restaurar backup base
mongorestore --uri="mongodb://user:pass@localhost:27017" \
  --gzip \
  /backups/mongodb/20251118

# 2. Aplicar oplog hasta timestamp específico
mongorestore --uri="mongodb://user:pass@localhost:27017" \
  --oplogReplay \
  --oplogLimit="1700308800:0" \
  /backups/mongodb/incremental/20251118_140000
```

---

### 5. Disaster Recovery

**RTO (Recovery Time Objective):** < 1 hora
**RPO (Recovery Point Objective):** < 15 minutos

**Estrategia:**
1. Replica set con 3 nodos (1 primary + 2 secondary)
2. Backups automáticos cada 4 horas
3. Replicación geográfica (multi-región)
4. Monitoreo con alertas de fallos

---

## 📊 Monitoreo y Mantenimiento

### 1. Métricas Clave

**A monitorear:**
- Tamaño de collections (`db.stats()`)
- Uso de índices (`db.collection.aggregate([{$indexStats:{}}])`)
- Queries lentas (> 100ms en logs)
- Eventos fallidos (`material_event.status = "failed"`)
- TTL index funcionando correctamente

**Script de monitoreo:**

```javascript
// monitor_stats.js
db.adminCommand({ serverStatus: 1 });

db.material_summary.stats();
db.material_assessment.stats();
db.material_event.stats();

// Verificar índices usados
db.material_summary.aggregate([{ $indexStats: {} }]);
db.material_assessment.aggregate([{ $indexStats: {} }]);
db.material_event.aggregate([{ $indexStats: {} }]);
```

---

### 2. Limpieza Manual

**Eliminar eventos antiguos manualmente (si TTL no está configurado):**

```javascript
// Eliminar eventos > 90 días
const ninetyDaysAgo = new Date();
ninetyDaysAgo.setDate(ninetyDaysAgo.getDate() - 90);

db.material_event.deleteMany({
  created_at: { $lt: ninetyDaysAgo }
});
```

---

### 3. Rebuild Índices

**Cuándo hacer rebuild:**
- Después de grandes inserciones/eliminaciones
- Si performance de queries se degrada
- Después de upgrade de MongoDB

```javascript
// Rebuild todos los índices de una collection
db.material_summary.reIndex();
db.material_assessment.reIndex();
db.material_event.reIndex();
```

---

## 🔒 Seguridad

### 1. Autenticación y Autorización

**Roles recomendados:**

```javascript
// Usuario para edugo-worker (read/write en edugo database)
db.createUser({
  user: "edugo_worker",
  pwd: "SECURE_PASSWORD",
  roles: [
    { role: "readWrite", db: "edugo" }
  ]
});

// Usuario para backups (read-only)
db.createUser({
  user: "edugo_backup",
  pwd: "SECURE_PASSWORD",
  roles: [
    { role: "read", db: "edugo" }
  ]
});

// Usuario para analytics (read-only en material_event)
db.createUser({
  user: "edugo_analytics",
  pwd: "SECURE_PASSWORD",
  roles: [
    { role: "read", db: "edugo" }
  ]
});
```

---

### 2. Encriptación

**Recomendaciones:**
- ✅ Habilitar encriptación en tránsito (TLS/SSL)
- ✅ Habilitar encriptación en reposo (MongoDB Enterprise)
- ✅ Rotar claves de encriptación cada 6 meses

---

### 3. Auditoría

**Habilitar auditoría de MongoDB:**

```javascript
// Configurar auditoría en mongod.conf
security:
  authorization: enabled
auditLog:
  destination: file
  format: JSON
  path: /var/log/mongodb/audit.json
  filter: '{ "atype": { $in: ["createCollection", "dropCollection", "dropDatabase"] } }'
```

---

## 📚 Referencias

- [MongoDB Schema Design Best Practices](https://www.mongodb.com/docs/manual/core/data-modeling-introduction/)
- [MongoDB Validation](https://www.mongodb.com/docs/manual/core/schema-validation/)
- [MongoDB Indexes](https://www.mongodb.com/docs/manual/indexes/)
- [MongoDB TTL Indexes](https://www.mongodb.com/docs/manual/core/index-ttl/)
- [MongoDB Backup Methods](https://www.mongodb.com/docs/manual/core/backups/)

---

## 📝 Notas de Implementación

### Para Sprint-02 (Implementación en Go)

**Tareas pendientes:**
1. Crear repositories Go para las 3 collections
2. Implementar validation en capa de aplicación
3. Migrar de `bson.M` a structs tipados
4. Agregar unit tests para repositories
5. Implementar retry logic con exponential backoff
6. Agregar logging de métricas de MongoDB

**Ejemplo de struct Go para material_summary:**

```go
type MaterialSummary struct {
    ID                primitive.ObjectID `bson:"_id,omitempty"`
    MaterialID        string             `bson:"material_id"`
    Summary           string             `bson:"summary"`
    KeyPoints         []string           `bson:"key_points"`
    Language          string             `bson:"language"`
    WordCount         int                `bson:"word_count"`
    Version           int                `bson:"version"`
    AIModel           string             `bson:"ai_model"`
    ProcessingTimeMS  int                `bson:"processing_time_ms"`
    TokenUsage        *TokenUsage        `bson:"token_usage,omitempty"`
    Metadata          *SummaryMetadata   `bson:"metadata,omitempty"`
    CreatedAt         time.Time          `bson:"created_at"`
    UpdatedAt         time.Time          `bson:"updated_at"`
}
```

---

**Fin del documento**

> **Autor:** Claude Code Web
> **Fecha:** 2025-11-18
> **Sprint:** Sprint-01 Fase 1 - Auditoría y Diseño de Schemas MongoDB
> **Próximo paso:** Ejecutar scripts de inicialización en Fase 2 (Claude Code Local)
