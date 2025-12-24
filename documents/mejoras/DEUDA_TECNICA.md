# Deuda Técnica - EduGo Worker

> **Propósito:** Documentar la deuda técnica identificada, su impacto y plan de resolución.  
> **Última revisión:** Diciembre 2024

---

## 📊 Resumen de Deuda Técnica

| Categoría | Items | Severidad Promedio | Esfuerzo Total Estimado |
|-----------|-------|-------------------|------------------------|
| Funcionalidad Incompleta | 5 | 🔴 Alta | 40-60 horas |
| Arquitectura | 3 | 🟡 Media | 20-30 horas |
| Testing | 4 | 🟡 Media | 30-40 horas |
| Documentación Código | 3 | 🟢 Baja | 10-15 horas |
| Dependencias | 2 | 🟡 Media | 5-10 horas |

**Deuda Total Estimada:** 105-155 horas de desarrollo

---

## 🔴 Severidad Alta - Funcionalidad Incompleta

### DT-001: Procesamiento de Eventos No Implementado

**Descripción:**  
El worker consume mensajes de RabbitMQ pero NO los procesa realmente. La función `processMessage()` en `cmd/main.go` siempre retorna `nil` sin ejecutar ningún processor.

**Ubicación:** `cmd/main.go:134-151`

**Código Actual:**
```go
func processMessage(msg amqp.Delivery, resources *bootstrap.Resources, cfg *config.Config) error {
    // Solo hace log y retorna nil
    resources.Logger.Info("✅ Evento procesado", "type", event["event_type"])
    
    // TODO: Implementar procesamiento real con processors
    // processor := container.GetProcessor(event["event_type"])
    // return processor.Process(ctx, event)
    
    return nil  // ⚠️ NO HACE NADA
}
```

**Impacto:**
- El worker es completamente inútil en producción
- Los materiales subidos nunca generan resúmenes ni quizzes
- Los eventos son consumidos y descartados

**Esfuerzo Estimado:** 8-12 horas

**Plan de Resolución:**
1. Implementar `ProcessorRegistry` (ver RF-002)
2. Conectar registry a `processMessage()`
3. Agregar tests de integración
4. Verificar en ambiente local

---

### DT-002: Integración OpenAI No Implementada

**Descripción:**  
El worker debería usar OpenAI para generar resúmenes y evaluaciones, pero esta funcionalidad solo está simulada con datos hardcoded.

**Ubicación:** `internal/application/processor/material_uploaded_processor.go:55-100`

**Código Actual:**
```go
// Solo hace log, no llama a OpenAI
p.logger.Debug("generating summary with AI")

// Datos hardcoded en lugar de generados
summary := bson.M{
    "main_ideas": []string{"Idea 1", "Idea 2", "Idea 3"},  // HARDCODED
}
```

**Impacto:**
- No hay generación real de contenido con IA
- Los datos guardados son inútiles
- No se aprovecha el potencial de la plataforma

**Esfuerzo Estimado:** 16-24 horas

**Plan de Resolución:**
1. Crear `internal/infrastructure/nlp/openai/client.go`
2. Implementar prompts para resumen y quiz
3. Agregar manejo de rate limits y errores
4. Agregar configuración de modelos
5. Implementar tests con mocks

**Dependencias:**
- API Key de OpenAI configurada
- Definición de prompts de calidad

---

### DT-003: Extracción de PDF No Implementada

**Descripción:**  
El worker debería descargar PDFs de S3 y extraer su texto, pero esta funcionalidad no existe.

**Ubicación:** `internal/infrastructure/pdf/` (carpeta vacía)

**Código Actual:**
```go
// Solo hace log
p.logger.Debug("extracting PDF text", "s3_key", event.S3Key)
// No hay implementación real
```

**Impacto:**
- No se puede procesar el contenido de los materiales
- La IA no tiene texto para analizar

**Esfuerzo Estimado:** 12-16 horas

**Plan de Resolución:**
1. Implementar `internal/infrastructure/storage/s3/client.go`
2. Implementar `internal/infrastructure/pdf/extractor.go`
3. Usar librería como `pdfcpu` o `unidoc`
4. Manejar diferentes tipos de PDF
5. Agregar tests

---

### DT-004: Carpeta Container Vacía

**Descripción:**  
La carpeta `internal/container/` existe pero está vacía. Debería contener la configuración de inyección de dependencias.

**Ubicación:** `internal/container/`

**Impacto:**
- No hay patrón de DI establecido
- Dificulta testing y mantenimiento
- Acoplamiento alto entre componentes

**Esfuerzo Estimado:** 4-6 horas

---

### DT-005: Procesadores de Eventos Incompletos

**Descripción:**  
`AssessmentAttemptProcessor` y `StudentEnrolledProcessor` solo hacen logging, no implementan lógica real.

**Ubicación:**
- `internal/application/processor/assessment_attempt_processor.go`
- `internal/application/processor/student_enrolled_processor.go`

**Código Actual:**
```go
func (p *AssessmentAttemptProcessor) Process(ctx context.Context, event dto.AssessmentAttemptEvent) error {
    p.logger.Info("processing assessment attempt", ...)
    
    // Aquí se podría:
    // - Enviar notificación al docente si score bajo
    // - Actualizar estadísticas
    // - Registrar en tabla de analytics
    
    p.logger.Info("assessment attempt processed successfully")
    return nil  // Solo log, no hace nada más
}
```

**Impacto:**
- No hay analytics de intentos de quiz
- No hay notificaciones a docentes
- No hay registro de progreso de estudiantes

**Esfuerzo Estimado:** 8-12 horas (por cada processor)

---

## 🟡 Severidad Media - Arquitectura

### DT-006: Bootstrap Complejo con Doble Puntero

**Descripción:**  
El patrón de factories usa doble puntero para retener referencias, lo cual es innecesariamente complejo.

**Ubicación:** `internal/bootstrap/custom_factories.go`

**Código Problemático:**
```go
type customPostgreSQLFactory struct {
    sqlDB  **sql.DB  // Doble puntero
}

func (f *customPostgreSQLFactory) CreateRawConnection(...) (*sql.DB, error) {
    db, err := f.shared.CreateRawConnection(ctx, config)
    *f.sqlDB = db  // Asignación indirecta
    return db, nil
}
```

**Impacto:**
- Código difícil de entender
- Difícil de debuggear
- Propenso a errores

**Esfuerzo Estimado:** 8-12 horas

---

### DT-007: Falta Interfaces para Dependencias

**Descripción:**  
Los processors dependen directamente de tipos concretos en lugar de interfaces.

**Código Actual:**
```go
type MaterialUploadedProcessor struct {
    db      *sql.DB           // Tipo concreto
    mongodb *mongo.Database   // Tipo concreto
    logger  logger.Logger     // ✓ Interfaz
}
```

**Impacto:**
- Difícil de testear unitariamente
- Acoplamiento alto
- No permite mocks fácilmente

**Esfuerzo Estimado:** 6-8 horas

---

### DT-008: No Hay Métricas ni Observabilidad

**Descripción:**  
El worker no expone métricas para monitoreo (Prometheus, etc).

**Impacto:**
- No hay visibilidad del rendimiento
- Difícil detectar problemas en producción
- No hay alertas posibles

**Esfuerzo Estimado:** 8-12 horas

---

## 🟡 Severidad Media - Testing

### DT-009: Cobertura de Tests Baja

**Descripción:**  
Hay pocos tests y la cobertura es baja.

**Estado Actual:**
```bash
# Tests existentes:
internal/infrastructure/persistence/mongodb/repository/
    - material_event_repository_test.go
    - material_summary_repository_test.go
    - config_test.go

# Faltantes:
- Tests de processors
- Tests de servicios de dominio
- Tests de integración
- Tests de bootstrap
```

**Impacto:**
- Riesgo de regresiones
- Difícil refactorizar con confianza
- No hay documentación ejecutable

**Esfuerzo Estimado:** 20-30 horas

---

### DT-010: No Hay Mocks Definidos

**Descripción:**  
No existen mocks o test doubles para facilitar testing.

**Impacto:**
- Tests requieren infraestructura real
- Tests lentos y frágiles
- Difícil testear casos edge

**Esfuerzo Estimado:** 8-12 horas

---

### DT-011: Tests de Integración Incompletos

**Descripción:**  
No hay tests que verifiquen el flujo completo de procesamiento.

**Esfuerzo Estimado:** 12-16 horas

---

### DT-012: No Hay Tests de Carga

**Descripción:**  
No se ha verificado el rendimiento bajo carga.

**Esfuerzo Estimado:** 8-12 horas

---

## 🟢 Severidad Baja - Documentación de Código

### DT-013: Funciones sin Documentación

**Descripción:**  
Muchas funciones públicas no tienen comentarios de documentación.

**Ejemplos:**
```go
// Sin documentación
func (r *MaterialSummaryRepository) FindByLanguage(ctx context.Context, language string, limit int64) ([]*entities.MaterialSummary, error) {

// Debería tener:
// FindByLanguage busca resúmenes por idioma.
// Retorna hasta 'limit' resúmenes ordenados por fecha de creación descendente.
// Parámetros:
//   - language: código ISO del idioma (es, en, pt)
//   - limit: máximo de resultados a retornar
// Retorna error si la conexión a MongoDB falla.
func (r *MaterialSummaryRepository) FindByLanguage(ctx context.Context, language string, limit int64) ([]*entities.MaterialSummary, error) {
```

**Esfuerzo Estimado:** 4-6 horas

---

### DT-014: README Desactualizado

**Descripción:**  
El README principal del proyecto (si existe) puede estar desactualizado.

**Esfuerzo Estimado:** 2-4 horas

---

### DT-015: No Hay Ejemplos de Uso

**Descripción:**  
No hay ejemplos de cómo usar los componentes del worker.

**Esfuerzo Estimado:** 4-6 horas

---

## 🟡 Severidad Media - Dependencias

### DT-016: Librería streadway/amqp Deprecada

**Descripción:**  
La dependencia `github.com/streadway/amqp` está archivada y no recibe actualizaciones.

**Ubicación:** `go.mod:20`

**Estado:**
- Ya se usa `rabbitmq/amqp091-go` como principal
- `streadway/amqp` puede ser una dependencia transitiva

**Acción:**
```bash
# Verificar uso
grep -r "streadway/amqp" --include="*.go" .

# Eliminar si no se usa
go mod tidy
```

**Esfuerzo Estimado:** 1-2 horas

---

### DT-017: Dependencias sin Versiones Pinneadas

**Descripción:**  
Algunas dependencias indirectas podrían no tener versiones específicas.

**Acción:**
```bash
# Verificar dependencias
go list -m all | wc -l

# Actualizar y verificar
go get -u ./...
go mod tidy
```

**Esfuerzo Estimado:** 2-4 horas

---

## 📅 Plan de Resolución por Sprints

### Sprint 1 (Crítico - 2 semanas)
| ID | Tarea | Estimación | Asignado |
|----|-------|------------|----------|
| DT-001 | Implementar routing a processors | 12h | - |
| DT-006 | Simplificar bootstrap | 12h | - |
| DT-016 | Eliminar streadway/amqp | 2h | - |

### Sprint 2 (OpenAI - 2 semanas)
| ID | Tarea | Estimación | Asignado |
|----|-------|------------|----------|
| DT-002 | Implementar cliente OpenAI | 24h | - |
| DT-003 | Implementar extracción PDF | 16h | - |

### Sprint 3 (Testing - 2 semanas)
| ID | Tarea | Estimación | Asignado |
|----|-------|------------|----------|
| DT-009 | Aumentar cobertura de tests | 24h | - |
| DT-010 | Crear mocks y test doubles | 12h | - |
| DT-007 | Agregar interfaces | 8h | - |

### Sprint 4 (Observabilidad - 1 semana)
| ID | Tarea | Estimación | Asignado |
|----|-------|------------|----------|
| DT-008 | Agregar métricas Prometheus | 12h | - |
| DT-013 | Documentar funciones | 6h | - |

### Backlog (Cuando haya tiempo)
- DT-004: Container/DI
- DT-005: Processors completos
- DT-011: Tests de integración
- DT-012: Tests de carga
- DT-014: README
- DT-015: Ejemplos

---

## 📈 Métricas de Seguimiento

```
Deuda Técnica Inicial: 105-155 horas
Meta Sprint 1: Reducir 26 horas (17-25%)
Meta Sprint 2: Reducir 40 horas (adicional 26-38%)
Meta Sprint 3: Reducir 44 horas (adicional 28-42%)
Meta Sprint 4: Reducir 18 horas (adicional 12-17%)

Total después de 4 sprints: ~95% resuelto
```

---

## 🔄 Proceso de Gestión de Deuda

1. **Identificación:** Agregar items a este documento con formato DT-XXX
2. **Priorización:** Asignar severidad basada en impacto
3. **Planificación:** Incluir en sprint planning
4. **Resolución:** Crear PR con referencia a DT-XXX
5. **Verificación:** Actualizar estado en este documento
6. **Retrospectiva:** Revisar mensualmente para evitar nueva deuda
