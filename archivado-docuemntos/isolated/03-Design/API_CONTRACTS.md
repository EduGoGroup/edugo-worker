# Contratos de API
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. RABBITMQ MESSAGES

### Event: material_uploaded (Input)

**Exchange:** `edugo.materials`  
**Routing Key:** `material.uploaded`  
**Queue:** `material_processing_high`

**Schema:**
```json
{
  "event_type": "material_uploaded",
  "material_id": "01936d9a-7f8e-7000-a000-123456789abc",
  "author_id": "01936d9a-7f8e-7000-a000-987654321cba",
  "s3_key": "school-1/unit-5/material-123/source/original.pdf",
  "preferred_language": "es",
  "content_type": "application/pdf",
  "timestamp": "2025-11-14T12:00:00Z"
}
```

---

## 2. OPENAI API

### Request: Generar Resumen

**Endpoint:** `POST https://api.openai.com/v1/chat/completions`

**Request Body:**
```json
{
  "model": "gpt-4-turbo-preview",
  "messages": [
    {
      "role": "system",
      "content": "Eres un asistente educativo..."
    },
    {
      "role": "user",
      "content": "Genera un resumen del siguiente texto: [TEXTO]"
    }
  ],
  "temperature": 0.3,
  "max_tokens": 4000,
  "response_format": {"type": "json_object"}
}
```

**Response:**
```json
{
  "sections": [...],
  "glossary": [...],
  "reflection_questions": [...]
}
```

---

## 3. S3 API

### Download File

```bash
# AWS SDK
s3.GetObject(bucket, key)
```

---

**Generado con:** Claude Code
