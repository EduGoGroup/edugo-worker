# Especificaciones Técnicas
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. STACK TECNOLÓGICO

### Backend
- **Lenguaje:** Go 1.21+
- **Framework:** Ninguno (worker standalone)
- **RabbitMQ Client:** github.com/rabbitmq/amqp091-go v1.9.0
- **MongoDB Driver:** go.mongodb.org/mongo-driver v1.13.1
- **PostgreSQL Driver:** github.com/lib/pq v1.10.9
- **OpenAI Client:** github.com/sashabaranov/go-openai v1.17.9
- **AWS SDK:** github.com/aws/aws-sdk-go v1.48.0

### Herramientas de Sistema
- **PDF Processing:** pdftotext (poppler-utils)
- **OCR:** tesseract v5.0+
- **Message Queue:** RabbitMQ 3.12+
- **Bases de Datos:** MongoDB 7.0+, PostgreSQL 15+

---

## 2. ARQUITECTURA

### Patrón: Event-Driven Consumer

```
RabbitMQ → Worker (Consumer) → [PDF Download → Extract Text → OpenAI] → MongoDB + PostgreSQL
```

### Componentes

1. **Consumer:** Escucha cola RabbitMQ
2. **PDFProcessor:** Extrae texto de PDFs
3. **AIService:** Llama OpenAI para resumen + quiz
4. **Repository:** Persiste en MongoDB y PostgreSQL
5. **EventLogger:** Registra métricas y eventos

---

## 3. PERFORMANCE

### Objetivos
- **Latencia promedio:** <3 minutos
- **p95:** <5 minutos
- **p99:** <10 minutos
- **Throughput:** ~20 materiales/hora por worker

### Optimizaciones
- Procesar PDFs en memoria (sin disco)
- Conexiones persistentes a MongoDB/PostgreSQL
- Pool de goroutines limitado (max 10 concurrentes)

---

## 4. SEGURIDAD

### OpenAI API
- API Key en variable de entorno (nunca en código)
- Rate limiting: 60 requests/min
- Timeout: 60 segundos

### S3 Access
- IAM credentials (no access keys hardcoded)
- Pre-signed URLs con expiración

### MongoDB/PostgreSQL
- Conexiones con TLS en producción
- Credentials desde secrets

---

**Generado con:** Claude Code
