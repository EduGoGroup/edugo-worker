# Código Deprecado y Candidato a Eliminación

> **Propósito:** Este documento identifica código que debería ser eliminado, reemplazado o marcado como deprecado.  
> **Última revisión:** Diciembre 2024

---

## 📋 Resumen Ejecutivo

| Prioridad | Archivo | Problema | Acción Recomendada |
|-----------|---------|----------|-------------------|
| 🔴 Alta | `cmd/main.go` | `processMessage()` con TODO sin implementar | Implementar o eliminar |
| 🔴 Alta | Varios processors | Código simulado sin implementación real | Implementar integración real |
| 🟡 Media | `custom_factories.go` | Patrón complejo de doble puntero | Refactorizar |
| 🟡 Media | `streadway/amqp` | Librería deprecada | Migrar completamente a `rabbitmq/amqp091-go` |
| 🟢 Baja | Comentarios TODO | Múltiples TODOs sin resolver | Resolver o documentar |

---

## 🔴 Prioridad Alta

### 1. Código Simulado en `processMessage()` - cmd/main.go

**Ubicación:** `cmd/main.go:134-151`

```go
// processMessage procesa un mensaje de RabbitMQ
func processMessage(msg amqp.Delivery, resources *bootstrap.Resources, cfg *config.Config) error {
    resources.Logger.Info("📥 Mensaje recibido", "size", len(msg.Body))

    var event map[string]interface{}
    if err := json.Unmarshal(msg.Body, &event); err != nil {
        resources.Logger.Error("Error parseando evento", "error", err.Error())
        return err
    }

    resources.Logger.Info("✅ Evento procesado", "type", event["event_type"])

    // TODO: Implementar procesamiento real con processors
    // processor := container.GetProcessor(event["event_type"])
    // return processor.Process(ctx, event)

    return nil  // ⚠️ SIEMPRE RETORNA nil - NO PROCESA NADA
}
```

**Problema:**
- El worker **NO está procesando eventos realmente**
- Siempre retorna `nil` sin hacer nada
- Los processors existen pero no se usan
- Hay un TODO comentado que nunca se implementó

**Impacto:**
- Los materiales subidos NO generan resúmenes ni evaluaciones
- El worker consume mensajes pero no hace nada útil
- Desperdicio de recursos

**Acción Requerida:**
```go
// IMPLEMENTAR: Routing a processors basado en event_type
func processMessage(msg amqp.Delivery, resources *bootstrap.Resources, cfg *config.Config) error {
    var event dto.BaseEvent
    if err := json.Unmarshal(msg.Body, &event); err != nil {
        return err
    }

    switch event.EventType {
    case "material_uploaded":
        var uploadEvent dto.MaterialUploadedEvent
        json.Unmarshal(msg.Body, &uploadEvent)
        processor := processor.NewMaterialUploadedProcessor(resources.PostgreSQL, resources.MongoDB, resources.Logger)
        return processor.Process(context.Background(), uploadEvent)
    case "material_deleted":
        // ... implementar
    default:
        resources.Logger.Warn("unknown event type", "type", event.EventType)
    }
    return nil
}
```

---

### 2. Procesadores con Lógica Simulada

**Ubicación:** `internal/application/processor/material_uploaded_processor.go:55-100`

```go
// PASO 4: Extraer Texto del PDF - SIMULADO
p.logger.Debug("extracting PDF text", "s3_key", event.S3Key)
// ⚠️ NO HAY IMPLEMENTACIÓN REAL - Solo log

// PASO 5: Generar Resumen con IA - SIMULADO  
p.logger.Debug("generating summary with AI")
// ⚠️ NO HAY LLAMADA A OPENAI - Solo log

// PASO 6: Datos hardcodeados en lugar de generados
summary := bson.M{
    "material_id":  event.MaterialID,
    "main_ideas":   []string{"Idea 1", "Idea 2", "Idea 3"},  // ⚠️ HARDCODED
    "key_concepts": bson.M{"concept1": "definition1"},       // ⚠️ HARDCODED
    // ...
}

// PASO 7: Quiz también hardcodeado
assessment := bson.M{
    "questions": []bson.M{
        {
            "question_text":  "Pregunta de ejemplo",  // ⚠️ HARDCODED
            "correct_answer": "A",                    // ⚠️ HARDCODED
        },
    },
}
```

**Problema:**
- No hay integración real con OpenAI
- No hay extracción real de texto PDF
- Los datos guardados son hardcoded, no generados

**Acción Requerida:**
1. Implementar `internal/infrastructure/nlp/openai_client.go`
2. Implementar `internal/infrastructure/pdf/extractor.go`
3. Implementar `internal/infrastructure/storage/s3_downloader.go`

---

### 3. Librería streadway/amqp Deprecada

**Ubicación:** `go.mod:20`

```go
github.com/streadway/amqp v1.1.0  // ⚠️ DEPRECADA
```

**Problema:**
- `streadway/amqp` está archivada y no recibe actualizaciones
- Ya se usa `rabbitmq/amqp091-go` pero `streadway` sigue en dependencias

**Acción Requerida:**
```bash
# Verificar si se usa en algún lugar
grep -r "streadway/amqp" --include="*.go" .

# Si no se usa, eliminar
go mod tidy
```

---

## 🟡 Prioridad Media

### 4. Patrón de Doble Puntero en custom_factories.go

**Ubicación:** `internal/bootstrap/custom_factories.go`

```go
// Patrón confuso y propenso a errores
type customFactoriesWrapper struct {
    sqlDB         *sql.DB       // Puntero simple
    mongoClient   *mongo.Client // Puntero simple
}

type customPostgreSQLFactory struct {
    sqlDB  **sql.DB  // ⚠️ Puntero a puntero - confuso
}

func (f *customPostgreSQLFactory) CreateRawConnection(...) (*sql.DB, error) {
    db, err := f.shared.CreateRawConnection(ctx, config)
    *f.sqlDB = db  // ⚠️ Asignación indirecta - difícil de seguir
    return db, nil
}
```

**Problema:**
- Patrón de doble puntero es confuso y difícil de mantener
- Dificulta el debugging
- No es idiomático en Go

**Solución Propuesta:**
```go
// Usar patrón más simple con callback o retorno de struct
type BootstrapResult struct {
    PostgreSQL *sql.DB
    MongoDB    *mongo.Database
    RabbitMQ   *amqp.Channel
    Logger     logger.Logger
}

func Bootstrap(ctx context.Context, cfg *config.Config) (*BootstrapResult, error) {
    // Crear recursos y retornar directamente
}
```

---

### 5. Uso de `log.Printf` en lugar de Logger Estructurado

**Ubicación:** Múltiples archivos

```go
// material_summary_repository.go:143
defer func() {
    if err := cursor.Close(ctx); err != nil {
        log.Printf("Error cerrando cursor: %v", err)  // ⚠️ log estándar
    }
}()

// bridge.go:71
if err := msg.Nack(false, true); err != nil {
    log.Printf("Error en Nack: %v", err)  // ⚠️ log estándar
}
```

**Problema:**
- Mezcla de `log.Printf` con logger estructurado
- Inconsistencia en formato de logs
- Dificulta el parsing en sistemas de monitoreo

**Solución:**
```go
// Usar siempre el logger estructurado inyectado
defer func() {
    if err := cursor.Close(ctx); err != nil {
        r.logger.Error("error closing cursor", "error", err)
    }
}()
```

---

### 6. Constantes Hardcoded sin Configuración

**Ubicación:** `internal/domain/service/summary_validator.go:49`

```go
// Idiomas válidos hardcoded
func (v *SummaryValidator) isValidLanguage(language string) bool {
    validLanguages := []string{"es", "en", "pt"}  // ⚠️ Hardcoded
    // ...
}

// Modelos de IA hardcoded
func (v *SummaryValidator) isValidAIModel(model string) bool {
    validModels := []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"}  // ⚠️ Hardcoded
    // ...
}
```

**Problema:**
- Si se agrega un nuevo idioma o modelo, hay que modificar código
- No es configurable desde config.yaml

**Solución:**
```go
// Mover a config/constants.go o config.yaml
type ValidationConfig struct {
    ValidLanguages []string `mapstructure:"valid_languages"`
    ValidAIModels  []string `mapstructure:"valid_ai_models"`
}
```

---

## 🟢 Prioridad Baja

### 7. TODOs sin Resolver

**Lista de TODOs encontrados:**

| Archivo | Línea | TODO |
|---------|-------|------|
| `material_uploaded_processor.go` | 55 | `// TODO: Implementar con PDF library` |
| `material_uploaded_processor.go` | 58 | `// TODO: Implementar con OpenAI API` |
| `summary_validator.go` | 75 | `// TODO: Mejorar para manejar múltiples espacios` |
| `cmd/main.go` | 146 | `// TODO: Implementar procesamiento real` |
| `assessment_attempt_processor.go` | 25 | `// Aquí se podría:` (implícito TODO) |
| `student_enrolled_processor.go` | 24 | `// Aquí se podría:` (implícito TODO) |

**Acción:** Resolver cada TODO o crear issues en GitHub para tracking.

---

### 8. Carpetas Vacías o con Solo .gitkeep

**Ubicación:**
```
internal/infrastructure/nlp/         # Vacía
internal/infrastructure/pdf/         # Vacía  
internal/infrastructure/storage/     # Vacía
internal/application/service/        # Vacía
internal/infrastructure/postgres/    # Vacía
```

**Problema:**
- Indican funcionalidad planeada pero no implementada
- Pueden confundir a nuevos desarrolladores

**Acción:**
- Implementar la funcionalidad faltante, o
- Eliminar carpetas y documentar en roadmap

---

### 9. MaterialReprocessProcessor Redundante

**Ubicación:** `internal/application/processor/material_reprocess_processor.go`

```go
func (p *MaterialReprocessProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    p.logger.Info("reprocessing material", "material_id", event.MaterialID)
    
    // Reprocesar es lo mismo que procesar por primera vez
    return p.uploadedProcessor.Process(ctx, event)  // ⚠️ Solo delega
}
```

**Problema:**
- El processor no agrega valor, solo delega
- No elimina datos anteriores antes de reprocesar
- Debería eliminar summary y assessment existentes primero

**Solución:**
```go
func (p *MaterialReprocessProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
    // 1. Eliminar datos existentes
    p.deletedProcessor.Process(ctx, dto.MaterialDeletedEvent{
        MaterialID: event.MaterialID,
    })
    
    // 2. Reprocesar
    return p.uploadedProcessor.Process(ctx, event)
}
```

---

## 📊 Plan de Acción

### Fase 1: Crítico (Sprint actual)
1. [ ] Implementar routing real en `processMessage()`
2. [ ] Crear issue para implementación de OpenAI
3. [ ] Crear issue para extracción de PDF

### Fase 2: Importante (Próximo sprint)
4. [ ] Refactorizar `custom_factories.go`
5. [ ] Eliminar `streadway/amqp` si no se usa
6. [ ] Unificar uso de logger

### Fase 3: Mejora Continua
7. [ ] Mover constantes a configuración
8. [ ] Resolver TODOs pendientes
9. [ ] Implementar carpetas vacías o eliminar

---

## 🔍 Cómo Encontrar Más Código Deprecado

```bash
# Buscar TODOs
grep -rn "TODO" --include="*.go" internal/

# Buscar FIXMEs
grep -rn "FIXME" --include="*.go" internal/

# Buscar código comentado
grep -rn "^[[:space:]]*//.*func\|^[[:space:]]*//.*return" --include="*.go" internal/

# Buscar imports no usados
go vet ./...

# Buscar código muerto con staticcheck
staticcheck ./...
```
