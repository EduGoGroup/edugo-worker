# Especificaciones Funcionales
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. MÓDULO: CONSUMO DE EVENTOS

### RF-001: Consumir Eventos de RabbitMQ
**Prioridad:** MUST  
**Cola:** `material_processing_high`

#### Descripción
Worker debe conectarse a RabbitMQ y consumir eventos `material_uploaded` con ACK manual.

#### Criterios de Aceptación
- Worker se conecta a RabbitMQ al iniciar
- Consume mensajes de cola con prefetch=1 (un mensaje a la vez)
- ACK solo después de procesamiento exitoso
- NACK con requeue=true si error transitorio
- NACK con requeue=false si error permanente

---

## 2. MÓDULO: PROCESAMIENTO DE PDFs

### RF-002: Extraer Texto de PDFs
**Prioridad:** MUST

#### Descripción
Descargar PDF de S3 y extraer texto usando pdftotext.

#### Criterios
- Descarga desde S3 usando IAM credentials
- Extracción con `pdftotext -layout`
- Validación: mínimo 500 palabras
- Si falla pdftotext, intentar OCR con Tesseract

---

## 3. MÓDULO: IA - RESÚMENES

### RF-003: Generar Resumen con OpenAI GPT-4
**Prioridad:** MUST

#### Descripción
Llamar OpenAI API para generar resumen estructurado educativo.

#### Criterios
- Modelo: `gpt-4-turbo-preview`
- Temperature: 0.3 (determinístico)
- Max tokens: 4000
- Timeout: 60 segundos
- Formato response: JSON estructurado
- Validar JSON antes de guardar

---

## 4. MÓDULO: IA - EVALUACIONES

### RF-004: Generar Quiz Automático
**Prioridad:** MUST

#### Descripción
Generar 5-10 preguntas de opción múltiple con respuestas correctas.

#### Criterios
- 5-10 preguntas por material
- 4 opciones por pregunta
- 1 respuesta correcta por pregunta
- Distractores plausibles
- Feedback para respuestas correctas e incorrectas

---

## 5. MÓDULO: PERSISTENCIA

### RF-005: Guardar en MongoDB
**Prioridad:** MUST

#### Descripción
Guardar resumen en `material_summary` y quiz en `material_assessment`.

#### Criterios
- Upsert (actualizar si existe, crear si no)
- Validar schema antes de insertar
- Índices en `material_id`

### RF-006: Actualizar PostgreSQL
**Prioridad:** MUST

#### Descripción
Crear registros en `assessment` y `material_summary_link`.

---

## 6. MÓDULO: MANEJO DE ERRORES

### RF-007: Retry Logic con Backoff Exponencial
**Prioridad:** MUST

| Error | Reintentos | Backoff |
|-------|------------|---------|
| OpenAI timeout | 5 | 1min, 2min, 4min, 8min, 16min |
| OpenAI rate limit | Ilimitado | Según header Retry-After |
| MongoDB down | 5 | 10s, 30s, 1min, 5min, 15min |
| S3 download fail | 3 | 5s, 15s, 30s |

### RF-008: Dead Letter Queue
**Prioridad:** SHOULD

Enviar a DLQ después de 5 intentos fallidos.

---

**Total RF:** 8 (6 MUST, 2 SHOULD)
