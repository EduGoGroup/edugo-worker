# Resumen del Análisis: Entities en Worker

**Fecha:** 20 de Noviembre, 2025  
**Proyecto:** edugo-worker  
**Analista:** Claude Code

---

## 📊 Resultados del Análisis

### Entities Encontrados

✅ **3 entities MongoDB** en `internal/domain/entity/`:

1. **material_assessment.go** (172 LOC)
   - Struct principal: `MaterialAssessment`
   - Structs embebidos: `Question`, `Option`, `AssessmentMetadata`
   - 6 métodos con lógica de negocio

2. **material_summary.go** (104 LOC)
   - Struct principal: `MaterialSummary`
   - Structs embebidos: `TokenUsage`, `SummaryMetadata`
   - 3 métodos con lógica de negocio

3. **material_event.go** (145 LOC)
   - Struct principal: `MaterialEvent`
   - 6 constantes de EventType
   - 4 constantes de EventStatus
   - 9 métodos con lógica de negocio

**Total:** 421 líneas de código a eliminar tras migración

---

## 🔍 Dependencias Identificadas

### Archivos que Importan Entities

**6 archivos** en `internal/infrastructure/persistence/mongodb/repository/`:

1. `material_assessment_repository.go` (15 usos)
2. `material_summary_repository.go` (12 usos)
3. `material_event_repository.go` (18 usos)
4. `material_assessment_repository_test.go` (tests)
5. `material_summary_repository_test.go` (tests)
6. `material_event_repository_test.go` (tests)

**Total de referencias:** ~66 usos de `entity.*` en el código

---

## ⚠️ Complejidades Detectadas

### 🔴 ALTA COMPLEJIDAD: Lógica de Negocio en Entities

**Problema:** Las entities tienen métodos complejos que NO deben estar en infrastructure.

**Métodos con lógica:**
- `IsValid()` - Validaciones complejas (3 entities)
- `MarkAsProcessing/Completed/Failed()` - Máquina de estados
- `CalculateAverageDifficulty()` - Cálculos
- `IncrementVersion()`, `IncrementRetry()` - Mutaciones
- `CanRetry()` - Lógica de reintentos
- `countWords()` - Procesamiento de texto

**Solución requerida:**
- Crear **3 domain services** en worker:
  1. `AssessmentValidator`
  2. `SummaryValidator`
  3. `EventStateMachine`

---

## 📋 Mapeo a Infrastructure

| Worker Entity | Infrastructure Entity | Ubicación |
|---------------|----------------------|-----------|
| `MaterialAssessment` + embebidos | `MaterialAssessment` | `mongodb/entities/material_assessment.go` |
| `MaterialSummary` + embebidos | `MaterialSummary` | `mongodb/entities/material_summary.go` |
| `MaterialEvent` + constantes | `MaterialEvent` | `mongodb/entities/material_event.go` |

**IMPORTANTE:** Infrastructure debe incluir:
- ✅ Todos los structs embebidos (Question, Option, TokenUsage, etc.)
- ✅ Todas las constantes (EventType*, EventStatus*)
- ❌ SIN métodos de lógica de negocio

---

## 📝 Plan de Acción

**Documento completo:** `SPRINT-ENTITIES-ADAPTATION.md` (770 líneas)

**Fases principales:**

1. **Validar Infrastructure** (10 min)
2. **Crear Domain Services** (2-3 horas) ⚠️ Mayor esfuerzo
3. **Actualizar go.mod** (5 min)
4. **Actualizar Imports** (30 min)
5. **Adaptar Lógica de Negocio** (1-2 horas)
6. **Eliminar Entities Locales** (5 min)
7. **Tests** (1 hora)
8. **Validación Final** (30 min)

**Tiempo total estimado:** 5-7 horas

---

## ✅ Criterios de Éxito

- [ ] 3 domain services creados
- [ ] 6 repositorios actualizados
- [ ] 0 referencias a `internal/domain/entity`
- [ ] 421 líneas eliminadas
- [ ] Tests pasan
- [ ] Build exitoso

---

## 🎯 Recomendaciones

1. **Ejecutar Sprint de Infrastructure PRIMERO**
   - Asegurar que entities MongoDB estén completas
   - Verificar que incluyen structs embebidos y constantes

2. **Crear Domain Services con Cuidado**
   - Copiar lógica exacta desde entities actuales
   - Escribir tests unitarios para cada service
   - Validar que la lógica no se pierde en la migración

3. **Comparar BSON Tags**
   - Tags en infrastructure deben ser idénticos
   - Cualquier cambio romperá queries a MongoDB

4. **Testing Exhaustivo**
   - Tests unitarios de services
   - Tests de integración de repositories
   - Tests end-to-end con MongoDB real

---

**Documento generado:** `/Users/jhoanmedina/source/EduGo/Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/SPRINT-ENTITIES-ADAPTATION.md`

**Próximo paso:** Ejecutar Sprint de Infrastructure para crear entities MongoDB centralizadas.
