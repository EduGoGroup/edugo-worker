# EduGo Worker - Procesamiento Asíncrono

Worker que consume eventos de RabbitMQ para procesar materiales educativos con IA.

## Responsabilidades

1. **Generación de Resumen y Quiz** (`material_uploaded`):
   - Descarga PDF desde S3
   - Extrae texto (OCR si es necesario)
   - Llama API NLP (OpenAI GPT-4) para generar resumen
   - Genera cuestionario con IA
   - Persiste en MongoDB (`material_summary`, `material_assessment`)
   - Actualiza PostgreSQL
   - Notifica docente

2. **Reprocesamiento** (`material_reprocess`):
   - Regenera resumen/quiz de material existente
   - Incrementa versión en MongoDB

3. **Notificaciones** (`assessment_attempt_recorded`):
   - Notifica docentes cuando estudiante completa quiz

4. **Limpieza** (`material_deleted`):
   - Elimina archivos S3
   - Elimina documentos MongoDB

5. **Bienvenida** (`student_enrolled`):
   - Envía email/push de bienvenida a nuevos estudiantes

## Tecnología

- Go 1.21+ + RabbitMQ + MongoDB

## Instalación

```bash
go mod download
go run cmd/main.go
```

## Eventos Procesados

| Evento | Cola | Prioridad | Procesador |
|--------|------|-----------|------------|
| `material.uploaded` | material_processing_high | 10 | Summary + Quiz Generator |
| `material.reprocess` | material_processing_medium | 5 | Reprocessor |
| `assessment.attempt_recorded` | material_processing_medium | 5 | Notifier |
| `material.deleted` | material_processing_low | 1 | Cleanup |
| `student.enrolled` | material_processing_low | 1 | Welcome |

## Configuración

Variables de entorno:
```env
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
MONGODB_URL=mongodb://localhost:27017/edugo
POSTGRES_URL=postgresql://user:pass@localhost:5432/edugo
S3_ENDPOINT=https://s3.amazonaws.com
OPENAI_API_KEY=sk-...
```

## Estado: Código base con lógica MOCK

Implementar para producción:
- Clientes reales de S3, MongoDB, PostgreSQL
- Integración con OpenAI API
- Reintentos con backoff exponencial
- Dead Letter Queue para errores
- Logging estructurado
- Métricas de procesamiento
