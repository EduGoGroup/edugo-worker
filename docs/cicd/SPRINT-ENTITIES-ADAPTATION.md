# Sprint: Adaptar Worker a Entities Centralizadas

**Proyecto:** edugo-worker  
**Fecha:** 20 de Noviembre, 2025  
**Dependencia:** edugo-infrastructure Sprint ENTITIES completado  
**Prioridad:** ALTA - Elimina duplicación de entities MongoDB

---

## 🎯 Contexto

**Problema actual:**
- Worker tiene 3 entities MongoDB duplicadas en `internal/domain/entity/`
- Estas entities deben moverse a `infrastructure/mongodb/entities/`
- Worker debe importar entities desde infrastructure

**Solución:**
- Eliminar entities locales de worker
- Importar entities centralizadas desde infrastructure
- Actualizar imports en repositorios y código

---

## 📊 Análisis de Entities Actuales

### Entities Encontradas en Worker

| # | Archivo | Ruta Completa | LOC | Structs Embebidos |
|---|---------|---------------|-----|-------------------|
| 1 | `material_assessment.go` | `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/internal/domain/entity/material_assessment.go` | 172 | `Question`, `Option`, `AssessmentMetadata` |
| 2 | `material_summary.go` | `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/internal/domain/entity/material_summary.go` | 104 | `TokenUsage`, `SummaryMetadata` |
| 3 | `material_event.go` | `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/internal/domain/entity/material_event.go` | 145 | Ninguno |

**Total:** 3 entities con 421 líneas de código

### Contenido de Entities

#### 1. MaterialAssessment
```go
// Struct principal
type MaterialAssessment struct {
    ID               primitive.ObjectID
    MaterialID       string
    Questions        []Question
    TotalQuestions   int
    TotalPoints      int
    Version          int
    AIModel          string
    ProcessingTimeMs int
    TokenUsage       *TokenUsage
    Metadata         *AssessmentMetadata
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// Structs embebidos
type Question struct {
    QuestionID    string
    QuestionText  string
    QuestionType  string
    Options       []Option
    CorrectAnswer string
    Explanation   string
    Points        int
    Difficulty    string
    Tags          []string
}

type Option struct {
    OptionID   string
    OptionText string
}

type AssessmentMetadata struct {
    AverageDifficulty string
    EstimatedTimeMin  int
}
```

**Métodos con lógica de negocio:**
- ✅ `NewMaterialAssessment()` - Constructor
- ✅ `NewQuestion()` - Constructor de pregunta
- ✅ `AddOption()` - Agregar opción a pregunta
- ⚠️ `IsValid()` - Validación (tiene lógica)
- ⚠️ `IncrementVersion()` - Incrementar versión
- ⚠️ `CalculateAverageDifficulty()` - Calcular dificultad promedio

#### 2. MaterialSummary
```go
// Struct principal
type MaterialSummary struct {
    ID               primitive.ObjectID
    MaterialID       string
    Summary          string
    KeyPoints        []string
    Language         string
    WordCount        int
    Version          int
    AIModel          string
    ProcessingTimeMs int
    TokenUsage       *TokenUsage
    Metadata         *SummaryMetadata
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// Structs embebidos
type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}

type SummaryMetadata struct {
    SourceLength int
    HasImages    bool
}
```

**Métodos con lógica de negocio:**
- ✅ `NewMaterialSummary()` - Constructor
- ⚠️ `countWords()` - Función privada de conteo
- ⚠️ `IsValid()` - Validación (tiene lógica)
- ⚠️ `IncrementVersion()` - Incrementar versión

#### 3. MaterialEvent
```go
// Struct principal
type MaterialEvent struct {
    ID          primitive.ObjectID
    EventType   string
    MaterialID  string
    UserID      string
    Payload     primitive.M
    Status      string
    ErrorMsg    string
    StackTrace  string
    RetryCount  int
    ProcessedAt *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Constantes:**
- `EventTypeMaterialUploaded`, `EventTypeMaterialReprocess`, etc.
- `EventStatusPending`, `EventStatusProcessing`, etc.

**Métodos con lógica de negocio:**
- ✅ `NewMaterialEvent()` - Constructor
- ✅ `NewMaterialEventWithMaterialID()` - Constructor con materialID
- ⚠️ `IsValid()` - Validación
- ⚠️ `isValidEventType()` - Validación de tipo
- ⚠️ `isValidEventStatus()` - Validación de estado
- ⚠️ `MarkAsProcessing()` - Cambiar estado
- ⚠️ `MarkAsCompleted()` - Cambiar estado
- ⚠️ `MarkAsFailed()` - Cambiar estado con error
- ⚠️ `IncrementRetry()` - Incrementar reintentos
- ⚠️ `CanRetry()` - Verificar si puede reintentar

### Mapeo a Infrastructure Entities

| Entity Actual (Worker) | Entity Infrastructure | Ubicación Infrastructure |
|-------------------------|----------------------|--------------------------|
| `MaterialAssessment` | `MaterialAssessment` | `mongodb/entities/material_assessment.go` |
| `MaterialSummary` | `MaterialSummary` | `mongodb/entities/material_summary.go` |
| `MaterialEvent` | `MaterialEvent` | `mongodb/entities/material_event.go` |

**Nota:** Infrastructure debe incluir TODOS los structs embebidos y constantes.

---

## 📋 Dependencias Actuales

### Archivos que Importan Entities

| Archivo | Import Actual | Usos |
|---------|---------------|------|
| `internal/infrastructure/persistence/mongodb/repository/material_assessment_repository.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | 15 referencias a `entity.MaterialAssessment` |
| `internal/infrastructure/persistence/mongodb/repository/material_summary_repository.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | 12 referencias a `entity.MaterialSummary` |
| `internal/infrastructure/persistence/mongodb/repository/material_event_repository.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | 18 referencias a `entity.MaterialEvent` |
| `internal/infrastructure/persistence/mongodb/repository/material_assessment_repository_test.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | Tests |
| `internal/infrastructure/persistence/mongodb/repository/material_summary_repository_test.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | Tests |
| `internal/infrastructure/persistence/mongodb/repository/material_event_repository_test.go` | `github.com/EduGoGroup/edugo-worker/internal/domain/entity` | Tests |

**Total de archivos afectados:** 6 archivos (3 repositorios + 3 tests)

**Total de referencias a `entity.*`:** ~66 referencias en el código

---

## ⚠️ Complejidad Detectada

### 🔴 ALTA COMPLEJIDAD: Lógica de Negocio en Entities

**Problema:** Las entities actuales tienen MUCHA lógica de negocio (validaciones, cálculos, cambios de estado)

**Impacto:** 
- Infrastructure entities deben ser SOLO estructura (sin lógica)
- La lógica debe moverse a **domain services** o **value objects** en worker

**Métodos que NO pueden ir en infrastructure:**
1. `IsValid()` en MaterialAssessment, MaterialSummary, MaterialEvent
2. `CalculateAverageDifficulty()` en MaterialAssessment
3. `countWords()` en MaterialSummary
4. `MarkAsProcessing/Completed/Failed()` en MaterialEvent
5. `IncrementVersion()`, `IncrementRetry()`, `CanRetry()`

**Solución propuesta:**
- Infrastructure entities = SOLO structs con tags bson
- Worker crea **domain services** para lógica de negocio
- Ejemplo: `AssessmentValidator`, `EventStateMachine`, `SummaryCalculator`

---

## 🏗️ Tareas del Sprint

### Fase 1: Validar Infrastructure Entities

#### Tarea 1.1: Verificar que infrastructure tenga entities completas

**Prerequisito:** Sprint ENTITIES de infrastructure debe estar completado

Verificar que `edugo-infrastructure` tenga:

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-infrastructure

# Verificar que existen
ls mongodb/entities/material_assessment.go
ls mongodb/entities/material_summary.go
ls mongodb/entities/material_event.go

# Verificar que tienen tag de release
git tag | grep mongodb/entities
```

**Esperado:**
- ✅ 3 entities existen
- ✅ Tienen structs embebidos (Question, Option, TokenUsage, etc.)
- ✅ Tienen constantes (EventType*, EventStatus*)
- ✅ Tag `mongodb/entities/v0.1.0` o superior existe

**Criterio de éxito:** Entities disponibles en GitHub para `go get`

---

### Fase 2: Preparar Worker para Cambio

#### Tarea 2.1: Crear domain services para lógica de negocio

**Ubicación:** `internal/domain/service/` (crear carpeta)

Crear servicios que contengan la lógica actualmente en entities:

**Archivo 1: `internal/domain/service/assessment_validator.go`**
```go
package service

import (
    mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// AssessmentValidator valida assessments
type AssessmentValidator struct{}

func NewAssessmentValidator() *AssessmentValidator {
    return &AssessmentValidator{}
}

// IsValid valida MaterialAssessment (lógica movida desde entity)
func (v *AssessmentValidator) IsValid(assessment *mongoentities.MaterialAssessment) bool {
    // Copiar lógica de IsValid() de entity actual
    if assessment.MaterialID == "" {
        return false
    }
    // ... resto de validación
    return true
}

// CalculateAverageDifficulty calcula dificultad promedio
func (v *AssessmentValidator) CalculateAverageDifficulty(assessment *mongoentities.MaterialAssessment) string {
    // Copiar lógica de CalculateAverageDifficulty() de entity actual
    // ...
}
```

**Archivo 2: `internal/domain/service/summary_validator.go`**
```go
package service

import (
    mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// SummaryValidator valida summaries
type SummaryValidator struct{}

func NewSummaryValidator() *SummaryValidator {
    return &SummaryValidator{}
}

// IsValid valida MaterialSummary
func (v *SummaryValidator) IsValid(summary *mongoentities.MaterialSummary) bool {
    // Copiar lógica de IsValid()
}

// CountWords cuenta palabras en texto
func (v *SummaryValidator) CountWords(text string) int {
    // Copiar lógica de countWords()
}
```

**Archivo 3: `internal/domain/service/event_state_machine.go`**
```go
package service

import (
    "time"
    mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
)

// EventStateMachine maneja cambios de estado de eventos
type EventStateMachine struct{}

func NewEventStateMachine() *EventStateMachine {
    return &EventStateMachine{}
}

// MarkAsProcessing cambia estado a processing
func (sm *EventStateMachine) MarkAsProcessing(event *mongoentities.MaterialEvent) {
    event.Status = mongoentities.EventStatusProcessing
    event.UpdatedAt = time.Now()
}

// MarkAsCompleted cambia estado a completed
func (sm *EventStateMachine) MarkAsCompleted(event *mongoentities.MaterialEvent) {
    event.Status = mongoentities.EventStatusCompleted
    now := time.Now()
    event.ProcessedAt = &now
    event.UpdatedAt = now
}

// MarkAsFailed cambia estado a failed
func (sm *EventStateMachine) MarkAsFailed(event *mongoentities.MaterialEvent, errorMsg, stackTrace string) {
    event.Status = mongoentities.EventStatusFailed
    event.ErrorMsg = errorMsg
    event.StackTrace = stackTrace
    now := time.Now()
    event.ProcessedAt = &now
    event.UpdatedAt = now
}

// IncrementRetry incrementa contador de reintentos
func (sm *EventStateMachine) IncrementRetry(event *mongoentities.MaterialEvent) {
    event.RetryCount++
    event.UpdatedAt = time.Now()
}

// CanRetry verifica si puede reintentar
func (sm *EventStateMachine) CanRetry(event *mongoentities.MaterialEvent, maxRetries int) bool {
    return event.Status == mongoentities.EventStatusFailed && event.RetryCount < maxRetries
}

// IsValid valida evento
func (sm *EventStateMachine) IsValid(event *mongoentities.MaterialEvent) bool {
    // Copiar lógica de IsValid(), isValidEventType(), isValidEventStatus()
}
```

**Criterio de éxito:** 3 servicios creados con toda la lógica de negocio

---

### Fase 3: Actualizar go.mod

#### Tarea 3.1: Agregar dependencia a infrastructure

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Agregar infrastructure/mongodb
go get github.com/EduGoGroup/edugo-infrastructure/mongodb@mongodb/entities/v0.1.0

go mod tidy
```

**Criterio de éxito:** `go.mod` tiene dependency de infrastructure/mongodb

---

### Fase 4: Actualizar Imports en Repositorios

#### Tarea 4.1: Actualizar imports en repositories

**Archivos a modificar:** 6 archivos en `internal/infrastructure/persistence/mongodb/repository/`

**Buscar y reemplazar:**
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Buscar líneas de import actual
grep -n '"github.com/EduGoGroup/edugo-worker/internal/domain/entity"' internal/infrastructure/persistence/mongodb/repository/*.go

# Reemplazar import
find internal/infrastructure/persistence/mongodb/repository/ -name "*.go" -exec sed -i '' 's|"github.com/EduGoGroup/edugo-worker/internal/domain/entity"|mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"|g' {} \;

# Reemplazar referencias entity. por mongoentities.
find internal/infrastructure/persistence/mongodb/repository/ -name "*.go" -exec sed -i '' 's|entity\.|mongoentities.|g' {} \;
```

**Archivos afectados:**
1. `material_assessment_repository.go`
2. `material_summary_repository.go`
3. `material_event_repository.go`
4. `material_assessment_repository_test.go`
5. `material_summary_repository_test.go`
6. `material_event_repository_test.go`

**Ejemplo de cambio:**
```diff
// ANTES
- import "github.com/EduGoGroup/edugo-worker/internal/domain/entity"
- func (r *Repo) Create(ctx context.Context, assessment *entity.MaterialAssessment) error {

// DESPUÉS
+ import mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
+ func (r *Repo) Create(ctx context.Context, assessment *mongoentities.MaterialAssessment) error {
```

**Criterio de éxito:** 6 archivos actualizados, 0 referencias a `internal/domain/entity`

---

### Fase 5: Actualizar Código que Usa Lógica de Negocio

#### Tarea 5.1: Inyectar domain services en repositorios

Los repositorios que llaman a `assessment.IsValid()` deben usar el nuevo validator.

**Ejemplo en `material_assessment_repository.go`:**

```diff
type MaterialAssessmentRepository struct {
    collection *mongo.Collection
+   validator  *service.AssessmentValidator
}

- func NewMaterialAssessmentRepository(db *mongo.Database) *MaterialAssessmentRepository {
+ func NewMaterialAssessmentRepository(db *mongo.Database, validator *service.AssessmentValidator) *MaterialAssessmentRepository {
    return &MaterialAssessmentRepository{
        collection: db.Collection("material_assessment"),
+       validator:  validator,
    }
}

func (r *MaterialAssessmentRepository) Create(ctx context.Context, assessment *mongoentities.MaterialAssessment) error {
-   if !assessment.IsValid() {
+   if !r.validator.IsValid(assessment) {
        return errors.New("invalid material assessment")
    }
    // ...
}
```

**Archivos a modificar:**
- `material_assessment_repository.go` - Inyectar `AssessmentValidator`
- `material_summary_repository.go` - Inyectar `SummaryValidator`
- `material_event_repository.go` - Inyectar `EventStateMachine`

**Criterio de éxito:** Repositorios usan services en lugar de métodos de entity

---

#### Tarea 5.2: Actualizar container de DI

Actualizar `internal/container/container.go` o `internal/bootstrap/custom_factories.go` para instanciar services.

```go
// Crear validators
assessmentValidator := service.NewAssessmentValidator()
summaryValidator := service.NewSummaryValidator()
eventStateMachine := service.NewEventStateMachine()

// Inyectar en repositorios
assessmentRepo := repository.NewMaterialAssessmentRepository(mongodb, assessmentValidator)
summaryRepo := repository.NewMaterialSummaryRepository(mongodb, summaryValidator)
eventRepo := repository.NewMaterialEventRepository(mongodb, eventStateMachine)
```

**Criterio de éxito:** Container instancia services correctamente

---

### Fase 6: Eliminar Entities Locales

#### Tarea 6.1: Eliminar carpeta de entities

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Verificar que no hay referencias antes de eliminar
grep -r "internal/domain/entity" internal/ --include="*.go"
# Debe retornar 0 resultados

# Eliminar entities
rm -rf internal/domain/entity/
```

**Criterio de éxito:** Carpeta `internal/domain/entity/` eliminada

---

### Fase 7: Tests

#### Tarea 7.1: Ejecutar tests de repositorios

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

go test ./internal/infrastructure/persistence/mongodb/repository/... -v
```

**Esperado:** Todos los tests pasan

**Si fallan:** Revisar que los tests también usan `mongoentities.` en lugar de `entity.`

**Criterio de éxito:** Tests pasan sin errores

---

#### Tarea 7.2: Ejecutar tests de domain services

```bash
# Crear tests para los nuevos services
go test ./internal/domain/service/... -v
```

**Criterio de éxito:** Tests de services pasan

---

### Fase 8: Validación Final

#### Tarea 8.1: Compilación completa

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Limpiar cache
go clean -cache

# Build completo
go build -o bin/worker ./cmd/worker

# Verificar binario
./bin/worker --version
```

**Criterio de éxito:** Compilación exitosa sin errores

---

#### Tarea 8.2: Verificar imports

```bash
# Listar todas las dependencies
go list -m all | grep infrastructure

# Debe mostrar:
# github.com/EduGoGroup/edugo-infrastructure/mongodb v0.1.0
```

**Criterio de éxito:** Infrastructure aparece en dependencies

---

#### Tarea 8.3: Ejecutar suite completa de tests

```bash
go test ./... -v -race -coverprofile=coverage.out

# Ver cobertura
go tool cover -html=coverage.out
```

**Criterio de éxito:** Tests pasan con cobertura adecuada

---

## 📊 Estimación de Esfuerzo

| Fase | Tareas | Tiempo Estimado |
|------|--------|-----------------|
| Fase 1: Validar Infrastructure | 1 tarea | 10 min |
| Fase 2: Crear Domain Services | 3 archivos | 2-3 horas |
| Fase 3: Actualizar go.mod | 1 tarea | 5 min |
| Fase 4: Actualizar Imports | 6 archivos | 30 min |
| Fase 5: Actualizar Lógica | 3 repos + container | 1-2 horas |
| Fase 6: Eliminar Entities | 1 tarea | 5 min |
| Fase 7: Tests | Repos + services | 1 hora |
| Fase 8: Validación Final | Build + tests | 30 min |
| **TOTAL** | | **5-7 horas** |

---

## 🔗 Dependencias

**Antes de este sprint:**
- ✅ Infrastructure Sprint ENTITIES completado
- ✅ Tag `mongodb/entities/v0.1.0` publicado

**Después de este sprint:**
- ➡️ Worker usa entities centralizadas
- ➡️ Lógica de negocio movida a domain services
- ➡️ Eliminada duplicación de 421 líneas de código

---

## ⚠️ Notas Importantes

### 1. Separación de Responsabilidades

**Entities en Infrastructure:**
- ✅ SOLO structs con tags bson
- ✅ Métodos simples como `TableName()`, `CollectionName()`
- ❌ NO validaciones
- ❌ NO lógica de negocio
- ❌ NO cálculos

**Domain Services en Worker:**
- ✅ Validaciones de negocio
- ✅ Cálculos y transformaciones
- ✅ Cambios de estado
- ✅ Reglas de negocio complejas

### 2. Constructores

**Opción A:** Mantener constructores simples en infrastructure
```go
// En infrastructure/mongodb/entities/material_assessment.go
func NewMaterialAssessment(...) *MaterialAssessment {
    // Solo asignación de campos, sin lógica
}
```

**Opción B (RECOMENDADA):** Crear factories en worker
```go
// En worker/internal/domain/factory/assessment_factory.go
func CreateMaterialAssessment(...) *mongoentities.MaterialAssessment {
    // Lógica de construcción compleja aquí
}
```

### 3. Constantes

**Constantes van en infrastructure:**
```go
// mongodb/entities/material_event.go
const (
    EventTypeMaterialUploaded  = "material_uploaded"
    EventStatusPending         = "pending"
    // ...
)
```

**Worker las importa:**
```go
import mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"

event.Status = mongoentities.EventStatusPending
```

### 4. Compatibilidad de BSON Tags

**CRÍTICO:** Los tags bson en infrastructure DEBEN ser idénticos a los actuales para no romper BD.

**Verificación requerida:**
```bash
# Comparar tags actuales vs infrastructure
diff <(grep "bson:" internal/domain/entity/material_assessment.go) \
     <(grep "bson:" /path/to/infrastructure/mongodb/entities/material_assessment.go)
```

### 5. Tests de Integración

**Después de adaptar:** Ejecutar tests de integración con MongoDB real:

```bash
# Levantar MongoDB local
docker-compose up -d mongodb

# Tests de integración
go test ./internal/infrastructure/persistence/mongodb/... -tags=integration -v
```

---

## 📈 Criterios de Éxito del Sprint

- [ ] Infrastructure entities disponibles con tag release
- [ ] 3 domain services creados con lógica de negocio
- [ ] go.mod actualizado con dependency de infrastructure
- [ ] 6 archivos de repositorios actualizados (imports + uso)
- [ ] Container de DI actualizado
- [ ] Carpeta `internal/domain/entity/` eliminada
- [ ] Tests de repositorios pasan
- [ ] Tests de domain services pasan
- [ ] Compilación exitosa sin errores
- [ ] Suite completa de tests pasa
- [ ] 421 líneas de código duplicadas eliminadas

---

## 🔄 Rollback Plan

Si algo falla durante la adaptación:

```bash
# Revertir cambios en worker
git checkout -- .
git clean -fd

# Volver a entities locales
git checkout HEAD~1 internal/domain/entity/
```

---

## 📝 Checklist de Ejecución

### Pre-Sprint
- [ ] Leer este documento completo
- [ ] Verificar que infrastructure Sprint ENTITIES está completado
- [ ] Verificar acceso a repos de infrastructure y worker
- [ ] Backup de rama actual: `git branch backup/pre-entities-adaptation`

### Durante Sprint
- [ ] Crear rama: `feature/adapt-infrastructure-entities`
- [ ] Ejecutar Fase 1 (Validar Infrastructure)
- [ ] Ejecutar Fase 2 (Domain Services)
- [ ] Ejecutar Fase 3 (go.mod)
- [ ] Ejecutar Fase 4 (Imports)
- [ ] Ejecutar Fase 5 (Lógica)
- [ ] Ejecutar Fase 6 (Eliminar)
- [ ] Ejecutar Fase 7 (Tests)
- [ ] Ejecutar Fase 8 (Validación)

### Post-Sprint
- [ ] PR a `dev` con descripción detallada
- [ ] Code review aprobado
- [ ] Merge a `dev`
- [ ] Tag release: `git tag v1.x.x`
- [ ] Actualizar CHANGELOG.md
- [ ] Documentar cambios en README

---

## 🎯 Próximos Pasos (Post-Sprint)

1. **Adaptar api-mobile:** Similar sprint para entities PostgreSQL
2. **Adaptar api-administracion:** Similar sprint para entities PostgreSQL
3. **Consolidar testing:** Tests de integración entre proyectos
4. **Documentar patrones:** Guía de cómo usar infrastructure entities

---

**Generado por:** Claude Code  
**Fecha:** 20 de Noviembre, 2025  
**Basado en:** Análisis completo de edugo-worker entities actuales
