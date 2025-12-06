# Modelo de Datos
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. MONGODB COLLECTIONS

### Collection: material_summary

```javascript
{
  "_id": ObjectId("507f1f77bcf86cd799439011"),
  "material_id": "01936d9a-7f8e-7000-a000-123456789abc",
  "version": 1,
  "status": "completed",
  "sections": [
    {
      "title": "Introducción",
      "content": "...",
      "difficulty": "beginner"
    }
  ],
  "glossary": [
    {
      "term": "Variable",
      "definition": "..."
    }
  ],
  "reflection_questions": [
    "¿Qué es una variable?",
    "..."
  ],
  "processing_metadata": {
    "tokens_used": 3500,
    "cost_usd": 0.15,
    "duration_seconds": 45,
    "model": "gpt-4-turbo-preview"
  },
  "created_at": ISODate("2025-11-14T12:00:00Z"),
  "updated_at": ISODate("2025-11-14T12:00:00Z")
}
```

**Índices:**
```javascript
db.material_summary.createIndex({ material_id: 1 }, { unique: true })
db.material_summary.createIndex({ status: 1 })
db.material_summary.createIndex({ created_at: -1 })
```

---

### Collection: material_assessment

```javascript
{
  "_id": ObjectId("507f1f77bcf86cd799439012"),
  "material_id": "01936d9a-7f8e-7000-a000-123456789abc",
  "questions": [
    {
      "id": "q1",
      "text": "¿Qué es una variable?",
      "type": "multiple_choice",
      "options": [
        {"id": "a", "text": "..."},
        {"id": "b", "text": "..."},
        {"id": "c", "text": "..."},
        {"id": "d", "text": "..."}
      ],
      "correct_answer": "a",
      "feedback": {
        "correct": "¡Correcto! ...",
        "incorrect": "No exactamente. ..."
      }
    }
  ],
  "total_questions": 5,
  "created_at": ISODate("2025-11-14T12:00:00Z")
}
```

**Índices:**
```javascript
db.material_assessment.createIndex({ material_id: 1 }, { unique: true })
```

---

## 2. POSTGRESQL TABLES

### Tabla: assessment
**Ya existe** desde spec-01. Worker hace INSERT después de generar quiz.

### Tabla: material_summary_link
**Ya existe** desde spec-01. Worker hace INSERT para enlazar MongoDB.

---

## 3. RABBITMQ SCHEMA

### Event: material_uploaded

```json
{
  "event_type": "material_uploaded",
  "material_id": "uuid",
  "author_id": "uuid",
  "s3_key": "school-1/unit-5/material-123/original.pdf",
  "preferred_language": "es",
  "timestamp": "2025-11-14T12:00:00Z"
}
```

### Queue Configuration
- **Queue:** material_processing_high
- **Durable:** true
- **Auto-delete:** false
- **Prefetch:** 1
- **Priority:** 10 (alta)

---

**Generado con:** Claude Code
