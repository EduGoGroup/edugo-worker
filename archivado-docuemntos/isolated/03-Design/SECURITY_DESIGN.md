# Diseño de Seguridad
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. THREAT MODEL (STRIDE)

### Spoofing
**Amenaza:** Eventos falsos en RabbitMQ  
**Mitigación:** Validar material_id existe en PostgreSQL antes de procesar

### Tampering
**Amenaza:** PDFs maliciosos (PDF bombs, scripts embebidos)  
**Mitigación:** Timeout en pdftotext (30s), sandboxing

### Repudiation
**Amenaza:** No poder rastrear quién publicó material  
**Mitigación:** Logs con author_id, timestamps, event_id

### Information Disclosure
**Amenaza:** API keys de OpenAI expuestas  
**Mitigación:** Variables de entorno, nunca en código o logs

### Denial of Service
**Amenaza:** PDFs gigantes saturan worker  
**Mitigación:** Límite de tamaño 50MB, timeout de procesamiento

### Elevation of Privilege
**Amenaza:** Worker accede a recursos no autorizados  
**Mitigación:** IAM con permisos mínimos (solo S3 read, no write)

---

## 2. SECRETOS Y CREDENCIALES

### OpenAI API Key
```bash
export OPENAI_API_KEY="sk-..."
# Rotar cada 90 días
# Monitoring de uso para detectar abusos
```

### RabbitMQ
```bash
export RABBITMQ_URL="amqp://user:pass@host:5672/"
# Usuario con permisos SOLO de consume (no publish)
```

### MongoDB/PostgreSQL
```bash
# TLS en producción
export MONGO_URI="mongodb://user:pass@host:27017/?tls=true"
export DB_URL="postgres://user:pass@host:5432/db?sslmode=require"
```

---

## 3. RATE LIMITING

### OpenAI API
- **Límite:** 60 requests/min
- **Implementación:** Token bucket algorithm
- **Backoff:** Respetar header `Retry-After`

---

**Generado con:** Claude Code
