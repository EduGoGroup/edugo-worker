# Fase 2.5: Homologación Material Assessment

> **Objetivo:** Actualizar dependencia de edugo-infrastructure y verificar que el worker utiliza correctamente la colección `material_assessment_worker`.
>
> **Duración estimada:** 1-2 días
> **Complejidad:** Baja
> **Riesgo:** Bajo
> **Prerequisito:** Fase 2 completada y nuevo release de edugo-infrastructure disponible

---

## 🎯 Objetivos

1. ✅ Verificar que el worker ya usa la colección correcta `material_assessment_worker`
2. ✅ Actualizar dependencia `edugo-infrastructure/mongodb` a la versión con el nuevo esquema
3. ✅ Validar que no hay referencias a colección incorrecta
4. ✅ Asegurar compatibilidad con el nuevo esquema de material_assessment

---

## 📦 Entregables

### E2.5.1: Verificación de Colección
- Confirmar uso de `material_assessment_worker` en repository
- Verificar que no hay referencias a colección sin sufijo

### E2.5.2: Actualización de Dependencias
- Actualizar `edugo-infrastructure/mongodb` a versión con nuevo esquema
- Ejecutar `go mod tidy`
- Verificar compatibilidad

### E2.5.3: Validación de Entity
- Verificar que entity `MaterialAssessment` incluye todos los campos del esquema completo
- Confirmar mapeo correcto con BSON tags

### E2.5.4: Tests de Compilación
- Ejecutar `go build ./...`
- Ejecutar tests unitarios
- Verificar que no hay warnings

---

## 🔄 Archivos Involucrados

```
internal/
├── domain/
│   └── entity/
│       └── material_assessment.go       # Verificar campos
└── infrastructure/
    └── persistence/
        └── mongodb/
            └── repository/
                └── material_assessment_repository.go  # Verificar colección

go.mod                                    # Actualizar versión infra
go.sum                                    # Actualizar checksums
```

---

## 🔑 Verificaciones Requeridas

### 1. Colección Correcta

El repository debe usar:
```go
collection := database.Collection("material_assessment_worker")
```

**NO debe usar:**
```go
collection := database.Collection("material_assessment")  // ❌ Incorrecto
```

### 2. Campos de Entity

La entity debe incluir todos estos campos:
```go
type MaterialAssessment struct {
    MaterialID        string                `bson:"material_id"`
    Questions         []Question            `bson:"questions"`
    TotalQuestions    int                   `bson:"total_questions"`
    TotalPoints       int                   `bson:"total_points"`
    Version           string                `bson:"version"`
    AIModel           string                `bson:"ai_model"`
    ProcessingTimeMs  int64                 `bson:"processing_time_ms"`
    Metadata          map[string]interface{} `bson:"metadata"`
    CreatedAt         time.Time             `bson:"created_at"`
    UpdatedAt         time.Time             `bson:"updated_at"`
}
```

### 3. Versión de Infraestructura

Actualizar a la versión que incluye el nuevo esquema (ejemplo):
```go
// go.mod
require (
    github.com/EduGoGroup/edugo-infrastructure/mongodb v0.12.1 // Nueva versión
)
```

---

## 📋 Commits Sugeridos

**Commit 1: Verificación de colección**
```
chore(fase-2.5): verificar uso correcto de material_assessment_worker

- Confirmar repository usa colección correcta
- Documentar estructura actual
```

**Commit 2: Actualización de dependencias**
```
chore(fase-2.5): actualizar edugo-infrastructure a v0.12.1

- Actualizar go.mod con nueva versión
- Ejecutar go mod tidy
- Verificar compilación exitosa
```

**Commit 3: Validación completa**
```
test(fase-2.5): validar compatibilidad con nuevo esquema

- Ejecutar tests unitarios
- Verificar integración con MongoDB
- Confirmar sin warnings
```

---

## ✅ Checklist de Validación

### Verificación de Código
- [ ] Repository usa `material_assessment_worker`
- [ ] No hay referencias a colección sin sufijo
- [ ] Entity tiene todos los campos requeridos
- [ ] BSON tags están correctos

### Actualización de Dependencias
- [ ] `go.mod` actualizado a nueva versión
- [ ] `go mod tidy` ejecutado sin errores
- [ ] `go.sum` actualizado

### Compilación y Tests
- [ ] `make build` ejecutado exitosamente
- [ ] `make test` todos los tests pasan
- [ ] No hay warnings de deprecación
- [ ] No hay conflictos de versiones

### Integración
- [ ] Repository crea/lee documentos correctamente
- [ ] Timestamps se generan automáticamente
- [ ] Metadata se serializa correctamente

---

## 🚨 Notas Importantes

### ⚠️ Sin Cambios de Código Requeridos

Esta fase es **principalmente de verificación**. El worker ya debería estar usando la colección correcta. Solo necesitamos:
1. Confirmar que está bien
2. Actualizar la dependencia
3. Validar que todo sigue funcionando

### ⚠️ Coordinación con Otros Proyectos

Esta homologación debe hacerse **después** de que:
- edugo-infrastructure tenga el nuevo release con el esquema
- Otros proyectos (api-mobile, api-administracion) también se actualicen

### ⚠️ No Afecta Datos Existentes

La actualización **no requiere migración de datos** porque:
- El worker siempre ha usado `material_assessment_worker`
- Solo cambia la versión de la dependencia
- El esquema es compatible hacia atrás

---

## 🎯 Criterios de Aceptación

Fase 2.5 **COMPLETADA** cuando:

1. ✅ Verificado uso de colección correcta
2. ✅ Dependencia `edugo-infrastructure` actualizada
3. ✅ Todos los tests pasan
4. ✅ No hay warnings ni errores
5. ✅ PR aprobado y mergeado a `dev`

---

## 📚 Referencias

- [Plan de Homologación](https://github.com/EduGoGroup/edugo-worker/tree/main/documents/analisis/03-homologar-material-assessment)
- Documento de análisis original: `/Users/jhoanmedina/source/EduGo/repos-separados/edugo_analisis/plan-trabajo/03-homologar-material-assessment/worker/PLAN.md`

---

## ⏭️ Siguiente Fase

**Fase 3: Testing y Calidad**
Ver: `plan-mejoras/fase-3/README.md`

---

**Última actualización:** 2025-12-23
**Estado:** ⏳ Pendiente / No iniciada
