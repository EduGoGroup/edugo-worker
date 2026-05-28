# Tareas - Fase 3: Testing y Calidad

---

## 📋 Resumen de Tareas

### Semana 1: Interfaces y Mocks

**T3.1: Extraer interfaces de repositories** (8h)
- Definir interfaz Repository genérica
- Interfaces específicas por repository
- Refactorizar código para usar interfaces

**T3.2: Crear mocks de repositories** (8h)
- Mock para MaterialSummaryRepository
- Mock para MaterialAssessmentRepository
- Mock para MaterialEventRepository
- Tests de los mocks

**T3.3: Crear mocks de servicios externos** (6h)
- Mock de OpenAI client
- Mock de S3 client
- Mock de PDF extractor

**T3.4: Utilidades de testing** (4h)
- Test fixtures (datos de prueba)
- Test builders
- Assertions custom

### Semana 2: Tests Unitarios

**T3.5: Tests de processors** (12h)
- MaterialUploadedProcessor (completo)
- MaterialDeletedProcessor (completo)
- AssessmentAttemptProcessor (completo)
- StudentEnrolledProcessor (completo)

**T3.6: Tests de domain services** (8h)
- SummaryValidator (completo)
- AssessmentValidator (completo)
- Otros servicios

**T3.7: Tests de infrastructure** (10h)
- Tests de repositories (con DB test)
- Tests de NLP client
- Tests de PDF extractor
- Tests de S3 client

### Semana 2-3: Tests de Integración

**T3.8: Setup Testcontainers** (6h)
- PostgreSQL container
- MongoDB container
- RabbitMQ container
- Configuración en CI

**T3.9: Tests end-to-end** (12h)
- Test de flujo completo material_uploaded
- Test de flujo material_deleted
- Tests de error scenarios
- Tests de concurrencia

**T3.10: Documentación** (6h)
- Guía de testing
- Ejemplos de tests
- README actualizado
- CI/CD mejorado

---

## ✅ Total Estimado: 80 horas (~3 semanas)
