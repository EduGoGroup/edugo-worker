# Base de Datos - EduGo Worker

## 📊 Visión General

El worker utiliza **dos bases de datos** con responsabilidades distintas:

| Base de Datos | Tipo | Propósito |
|--------------|------|-----------|
| **PostgreSQL** | Relacional | Estado de materiales, transacciones ACID |
| **MongoDB** | Documental | Contenido generado (resúmenes, evaluaciones) |

---

## 🗃️ PostgreSQL - Esquema

El worker **NO define** tablas propias en PostgreSQL. Utiliza tablas definidas por otros servicios (API Mobile/Admin) para actualizar estados.

### Tabla: `materials` (definida por API)

```sql
┌─────────────────────────────────────────────────────────────────────────────┐
│                              materials                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│ Columna            │ Tipo          │ Descripción                            │
├────────────────────┼───────────────┼────────────────────────────────────────┤
│ id                 │ UUID          │ PK, identificador único                │
│ title              │ VARCHAR(255)  │ Título del material                    │
│ description        │ TEXT          │ Descripción                            │
│ s3_key             │ VARCHAR(500)  │ Ruta del archivo en S3                 │
│ file_type          │ VARCHAR(50)   │ Tipo de archivo (pdf, docx, etc)       │
│ file_size          │ BIGINT        │ Tamaño en bytes                        │
│ author_id          │ UUID          │ FK → users.id                          │
│ unit_id            │ UUID          │ FK → units.id                          │
│ processing_status  │ VARCHAR(50)   │ Estado del procesamiento               │
│ created_at         │ TIMESTAMP     │ Fecha de creación                      │
│ updated_at         │ TIMESTAMP     │ Última actualización                   │
└─────────────────────────────────────────────────────────────────────────────┘

Estados de processing_status:
┌─────────────┬─────────────────────────────────────────────────────────────┐
│ Estado      │ Descripción                                                  │
├─────────────┼─────────────────────────────────────────────────────────────┤
│ pending     │ Material subido, pendiente de procesamiento                  │
│ processing  │ Worker está procesando (extracción, IA, etc)                 │
│ completed   │ Procesamiento exitoso, resumen y quiz generados             │
│ failed      │ Error en el procesamiento                                    │
└─────────────┴─────────────────────────────────────────────────────────────┘
```

### Operaciones SQL del Worker

```sql
-- 1. Marcar material como "en procesamiento"
UPDATE materials 
SET processing_status = 'processing', updated_at = NOW() 
WHERE id = $1;

-- 2. Marcar material como "completado"
UPDATE materials 
SET processing_status = 'completed', updated_at = NOW() 
WHERE id = $1;

-- 3. Marcar material como "fallido"
UPDATE materials 
SET processing_status = 'failed', updated_at = NOW() 
WHERE id = $1;
```

### Transacciones

El worker usa transacciones PostgreSQL (via `edugo-shared/database/postgres`) para garantizar consistencia:

```go
// Ejemplo de uso en MaterialUploadedProcessor
err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
    // 1. Actualizar estado a processing
    _, err := tx.ExecContext(ctx, "UPDATE materials SET processing_status = $1...", "processing", materialID)
    
    // ... procesar material ...
    
    // 2. Actualizar estado a completed
    _, err = tx.ExecContext(ctx, "UPDATE materials SET processing_status = $1...", "completed", materialID)
    
    return err
})
```

---

## 🍃 MongoDB - Colecciones

MongoDB almacena el contenido generado por el worker (resúmenes, evaluaciones, eventos).

### Diagrama de Colecciones

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MongoDB Database: edugo                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                       material_summary                                 │  │
│  │                                                                        │  │
│  │  Almacena resúmenes generados por IA para cada material               │  │
│  │                                                                        │  │
│  │  Índices:                                                              │  │
│  │  • material_id (unique)                                                │  │
│  │  • language                                                            │  │
│  │  • created_at                                                          │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                    material_assessment_worker                          │  │
│  │                                                                        │  │
│  │  Almacena evaluaciones/quizzes generados por IA                       │  │
│  │                                                                        │  │
│  │  Índices:                                                              │  │
│  │  • material_id (unique)                                                │  │
│  │  • created_at                                                          │  │
│  │  • questions.difficulty                                                │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                       material_events                                  │  │
│  │                                                                        │  │
│  │  Log de eventos procesados con sus estados                            │  │
│  │                                                                        │  │
│  │  Índices:                                                              │  │
│  │  • material_id                                                         │  │
│  │  • event_type                                                          │  │
│  │  • status                                                              │  │
│  │  • created_at                                                          │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Colección: `material_summary`

```javascript
// Estructura del documento
{
  "_id": ObjectId("..."),
  "material_id": "uuid-string",        // Referencia al material en PostgreSQL
  
  // Contenido del resumen
  "main_ideas": [                       // Ideas principales extraídas
    "Primera idea principal",
    "Segunda idea principal",
    "Tercera idea principal"
  ],
  
  "key_concepts": {                     // Conceptos clave con definiciones
    "concepto_1": "Definición del concepto 1",
    "concepto_2": "Definición del concepto 2"
  },
  
  "sections": [                         // Secciones del documento
    {
      "title": "Introducción",
      "summary": "Resumen de la introducción...",
      "page_range": "1-5"
    },
    {
      "title": "Desarrollo",
      "summary": "Resumen del desarrollo...",
      "page_range": "6-20"
    }
  ],
  
  "glossary": {                         // Glosario de términos
    "término_técnico": "Explicación simple"
  },
  
  // Metadatos
  "language": "es",                     // Idioma del resumen
  "source_pages": 25,                   // Páginas del documento original
  "word_count": 1500,                   // Palabras en el resumen
  "ai_model": "gpt-4",                  // Modelo de IA usado
  "ai_tokens_used": 3500,               // Tokens consumidos
  
  // Timestamps
  "created_at": ISODate("2024-01-15T10:30:00Z"),
  "updated_at": ISODate("2024-01-15T10:30:00Z")
}
```

#### Operaciones del Repository

```go
// MaterialSummaryRepository methods
type MaterialSummaryRepository interface {
    Create(ctx, *MaterialSummary) error
    FindByMaterialID(ctx, string) (*MaterialSummary, error)
    FindByID(ctx, ObjectID) (*MaterialSummary, error)
    Update(ctx, *MaterialSummary) error
    Delete(ctx, materialID string) error
    FindByLanguage(ctx, language string, limit int64) ([]*MaterialSummary, error)
    FindRecent(ctx, limit int64) ([]*MaterialSummary, error)
    CountByLanguage(ctx, language string) (int64, error)
    Exists(ctx, materialID string) (bool, error)
}
```

---

### Colección: `material_assessment_worker`

```javascript
// Estructura del documento
{
  "_id": ObjectId("..."),
  "material_id": "uuid-string",        // Referencia al material
  
  "questions": [                        // Array de preguntas
    {
      "id": "q1",
      "question_text": "¿Cuál es la idea principal del texto?",
      "question_type": "multiple_choice",  // multiple_choice, true_false, open_ended
      "difficulty": "medium",              // easy, medium, hard
      "points": 10,
      "options": [
        { "id": "a", "text": "Opción A", "is_correct": false },
        { "id": "b", "text": "Opción B", "is_correct": true },
        { "id": "c", "text": "Opción C", "is_correct": false },
        { "id": "d", "text": "Opción D", "is_correct": false }
      ],
      "correct_answer": "b",
      "explanation": "La respuesta correcta es B porque...",
      "related_section": "Introducción",
      "bloom_level": "comprehension"       // Taxonomía de Bloom
    },
    {
      "id": "q2",
      "question_text": "El autor afirma que X es verdadero.",
      "question_type": "true_false",
      "difficulty": "easy",
      "points": 5,
      "correct_answer": "false",
      "explanation": "Es falso porque..."
    }
  ],
  
  // Metadatos del assessment
  "total_questions": 10,
  "total_points": 100,
  "estimated_time_minutes": 15,
  "passing_score": 60,
  
  // Distribución de dificultad
  "difficulty_distribution": {
    "easy": 3,
    "medium": 5,
    "hard": 2
  },
  
  // Metadatos de generación
  "ai_model": "gpt-4",
  "ai_tokens_used": 4200,
  "generation_prompt_version": "v2.1",
  
  // Timestamps
  "created_at": ISODate("2024-01-15T10:30:00Z"),
  "updated_at": ISODate("2024-01-15T10:30:00Z")
}
```

#### Operaciones del Repository

```go
// MaterialAssessmentRepository methods
type MaterialAssessmentRepository interface {
    Create(ctx, *MaterialAssessment) error
    FindByMaterialID(ctx, string) (*MaterialAssessment, error)
    FindByID(ctx, ObjectID) (*MaterialAssessment, error)
    Update(ctx, *MaterialAssessment) error
    Delete(ctx, materialID string) error
    FindByDifficulty(ctx, difficulty string, limit int64) ([]*MaterialAssessment, error)
    FindByTotalQuestions(ctx, min, max int, limit int64) ([]*MaterialAssessment, error)
    FindRecent(ctx, limit int64) ([]*MaterialAssessment, error)
    CountByTotalPoints(ctx, minPoints, maxPoints int) (int64, error)
    Exists(ctx, materialID string) (bool, error)
    GetAverageQuestionCount(ctx) (float64, error)
}
```

---

### Colección: `material_events`

```javascript
// Estructura del documento (log de eventos)
{
  "_id": ObjectId("..."),
  "material_id": "uuid-string",
  
  "event_type": "material_uploaded",    // Tipo de evento
  // Valores válidos:
  // - material_uploaded
  // - material_reprocess
  // - material_deleted
  // - assessment_attempt
  // - student_enrolled
  // - student_unenrolled
  
  "status": "completed",                // Estado del procesamiento
  // Valores válidos:
  // - pending
  // - processing
  // - completed
  // - failed
  
  "payload": {                          // Datos originales del evento
    "author_id": "uuid",
    "s3_key": "materials/...",
    "preferred_language": "es"
  },
  
  "retry_count": 0,                     // Intentos de procesamiento
  "max_retries": 3,                     // Máximo de reintentos configurado
  
  // En caso de error
  "error_msg": null,                    // Mensaje de error si falló
  "stack_trace": null,                  // Stack trace si falló
  
  // Timestamps
  "created_at": ISODate("2024-01-15T10:30:00Z"),
  "updated_at": ISODate("2024-01-15T10:30:00Z"),
  "processed_at": ISODate("2024-01-15T10:32:00Z")  // Cuándo terminó
}
```

---

## 🔗 Relaciones entre Bases de Datos

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        RELACIONES CROSS-DATABASE                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   PostgreSQL                              MongoDB                             │
│   ┌─────────────────┐                    ┌─────────────────────────────────┐ │
│   │    materials    │                    │     material_summary             │ │
│   │                 │     material_id    │                                  │ │
│   │  id (UUID) ─────│───────────────────>│  material_id (string)           │ │
│   │  processing_    │                    │  main_ideas, key_concepts...    │ │
│   │    status       │                    │                                  │ │
│   └─────────────────┘                    └─────────────────────────────────┘ │
│                                                                               │
│                                          ┌─────────────────────────────────┐ │
│                          material_id     │  material_assessment_worker     │ │
│             ─────────────────────────────│                                  │ │
│                                          │  material_id (string)           │ │
│                                          │  questions[]                     │ │
│                                          │                                  │ │
│                                          └─────────────────────────────────┘ │
│                                                                               │
│                                          ┌─────────────────────────────────┐ │
│                          material_id     │      material_events            │ │
│             ─────────────────────────────│                                  │ │
│                                          │  material_id (string)           │ │
│                                          │  event_type, status...          │ │
│                                          │                                  │ │
│                                          └─────────────────────────────────┘ │
│                                                                               │
│   Nota: material_id en MongoDB es STRING (UUID serializado)                  │
│   Se usa como referencia lógica, no hay FK física entre bases               │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 📈 Queries Frecuentes

### MongoDB - Ejemplos de Queries

```javascript
// Buscar resumen por material_id
db.material_summary.findOne({ material_id: "uuid-here" })

// Buscar los 10 resúmenes más recientes en español
db.material_summary.find({ language: "es" })
  .sort({ created_at: -1 })
  .limit(10)

// Contar assessments con más de 10 preguntas
db.material_assessment_worker.countDocuments({ total_questions: { $gt: 10 } })

// Buscar eventos fallidos para retry
db.material_events.find({ 
  status: "failed", 
  retry_count: { $lt: 3 } 
})

// Promedio de preguntas por assessment
db.material_assessment_worker.aggregate([
  { $group: { _id: null, avg_questions: { $avg: "$total_questions" } } }
])

// Eventos por tipo y estado
db.material_events.aggregate([
  { $group: { _id: { event_type: "$event_type", status: "$status" }, count: { $sum: 1 } } }
])
```

---

## 🔧 Configuración de Conexión

### PostgreSQL

```yaml
# config/config.yaml
database:
  postgres:
    host: "localhost"
    port: 5432
    database: "edugo"
    user: "edugo_user"
    password: "${POSTGRES_PASSWORD}"  # Variable de entorno
    max_connections: 10
    ssl_mode: "disable"
```

### MongoDB

```yaml
# config/config.yaml
database:
  mongodb:
    uri: "${MONGODB_URI}"  # mongodb://user:pass@host:27017/edugo?authSource=admin
    database: "edugo"
    timeout: 10s
```

---

## 🛡️ Validaciones

### MaterialSummary Validator

```go
// Reglas de validación (service/summary_validator.go)
func (v *SummaryValidator) IsValid(summary *MaterialSummary) bool {
    // material_id requerido
    // Al menos una main_idea
    // language no vacío
    // created_at no cero
}
```

### MaterialAssessment Validator

```go
// Reglas de validación (service/assessment_validator.go)
func (v *AssessmentValidator) IsValid(assessment *MaterialAssessment) bool {
    // material_id requerido
    // Al menos una pregunta
    // Cada pregunta debe tener texto y tipo válido
    // total_questions debe coincidir con len(questions)
}
```

---

## 📊 Índices Recomendados

### MongoDB

```javascript
// material_summary
db.material_summary.createIndex({ material_id: 1 }, { unique: true })
db.material_summary.createIndex({ language: 1 })
db.material_summary.createIndex({ created_at: -1 })

// material_assessment_worker
db.material_assessment_worker.createIndex({ material_id: 1 }, { unique: true })
db.material_assessment_worker.createIndex({ created_at: -1 })
db.material_assessment_worker.createIndex({ "questions.difficulty": 1 })
db.material_assessment_worker.createIndex({ total_questions: 1 })

// material_events
db.material_events.createIndex({ material_id: 1, event_type: 1 })
db.material_events.createIndex({ status: 1, retry_count: 1 })
db.material_events.createIndex({ created_at: -1 })
```
