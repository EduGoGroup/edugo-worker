# EXECUTION PLAN - Worker

## Información del Proyecto

**Proyecto:** EduGo Worker  
**Objetivo:** Procesamiento asíncrono con IA para generación de evaluaciones  
**Duración:** 6 Sprints (12 semanas)  
**Equipo:** Backend engineers + DevOps  
**Repositorio:** https://github.com/EduGoGroup/edugo-worker

---

## Fase 1: Setup + RabbitMQ Consumer (Sprint 1)

### 1.1 Configuración Inicial
- [ ] Clonar repo
- [ ] go mod download
- [ ] Configurar variables de entorno
- [ ] Conectar a RabbitMQ
- [ ] Health check

### 1.2 Consumer RabbitMQ
- [ ] Crear subscriber
- [ ] Escuchar assessment.requests
- [ ] Procesar mensajes básicos
- [ ] Manual acknowledge

### 1.3 PostgreSQL Auditoría
- [ ] Crear tabla processing_requests
- [ ] Log de solicitudes
- [ ] Actualizar status

### 1.4 Tests
- [ ] Mock RabbitMQ
- [ ] Tests unitarios
- [ ] Tests de integración

---

## Fase 2: PDF Extraction (Sprint 2)

### 2.1 PDF Processing
- [ ] Integración con pdfium-go
- [ ] Extraer texto de PDF
- [ ] Limpiar contenido
- [ ] Validar integridad

### 2.2 S3 Integration
- [ ] Descargar PDF desde S3
- [ ] Subir texto extraído
- [ ] Manejo de errores

### 2.3 Tests
- [ ] PDF ejemplo
- [ ] Extracción correcta
- [ ] Limpieza de texto

---

## Fase 3: OpenAI Integration (Sprint 3)

### 3.1 OpenAI Client
- [ ] Inicializar cliente
- [ ] Preparar prompts
- [ ] Manejar respuestas
- [ ] Retry logic

### 3.2 Error Handling
- [ ] Rate limiting (429)
- [ ] Authentication (401)
- [ ] Server errors (500)
- [ ] Timeout handling

### 3.3 Mocking
- [ ] Mock OpenAI para tests
- [ ] Tests offline

---

## Fase 4: Generación de Preguntas (Sprint 4)

### 4.1 Question Generator
- [ ] Parsear respuesta JSON
- [ ] Validar estructura
- [ ] Crear Question objects
- [ ] Guardar en MongoDB

### 4.2 Persistencia
- [ ] Guardar en material_assessment
- [ ] Crear índices
- [ ] TTL index si es necesario

### 4.3 Publishing
- [ ] Publicar respuesta a RabbitMQ
- [ ] Notificar API Mobile

---

## Fase 5: Resúmenes + Optimizaciones (Sprint 5)

### 5.1 Generación de Resúmenes
- [ ] Endpoint para summarize
- [ ] Guardar en material_summary
- [ ] Tests

### 5.2 Optimizaciones
- [ ] Batch processing si es necesario
- [ ] Caché de PDFs procesados
- [ ] Logging centralizado

---

## Fase 6: Producción + Monitoreo (Sprint 6)

### 6.1 Production Readiness
- [ ] Docker image
- [ ] Health checks
- [ ] Metrics/Monitoring
- [ ] Cost tracking (OpenAI)

### 6.2 Load Testing
- [ ] Test con múltiples workers
- [ ] Stress test RabbitMQ
- [ ] Verificar OpenAI costs

### 6.3 Documentation
- [ ] Deployment guide
- [ ] Troubleshooting
- [ ] Cost estimation

---

**Próxima revisión:** Después de Sprint 1  
**Última actualización:** 15 de Noviembre, 2025
