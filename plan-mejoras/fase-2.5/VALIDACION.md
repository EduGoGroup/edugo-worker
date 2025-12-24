# Validación - Fase 2.5: Homologación Material Assessment

---

## ✅ Checklist de Validación

### 1. Verificación de Código

**Búsqueda de referencias incorrectas:**
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Buscar referencias a material_assessment sin sufijo _worker
grep -r "material_assessment" --include="*.go" . | grep -v "material_assessment_worker"
```

**Resultado esperado:**
- [ ] Solo aparecen referencias a `material_assessment_worker`
- [ ] No hay referencias a colección sin sufijo
- [ ] Repository usa la colección correcta

---

**Verificación de Repository:**
```bash
cat internal/infrastructure/persistence/mongodb/repository/material_assessment_repository.go
```

**Criterios:**
- [ ] Nombre de colección es `material_assessment_worker`
- [ ] No hay hardcoded de nombre incorrecto
- [ ] Factory/constructor usa la colección correcta

---

### 2. Verificación de Entity

**Revisar campos de entity:**
```bash
cat internal/domain/entity/material_assessment.go
```

**Campos requeridos:**
- [ ] `material_id` (string, bson:"material_id")
- [ ] `questions` ([]Question, bson:"questions")
- [ ] `total_questions` (int, bson:"total_questions")
- [ ] `total_points` (int, bson:"total_points")
- [ ] `version` (string, bson:"version")
- [ ] `ai_model` (string, bson:"ai_model")
- [ ] `processing_time_ms` (int64, bson:"processing_time_ms")
- [ ] `metadata` (map[string]interface{}, bson:"metadata")
- [ ] `created_at` (time.Time, bson:"created_at")
- [ ] `updated_at` (time.Time, bson:"updated_at")

---

### 3. Actualización de Dependencias

**Verificar versión actual:**
```bash
grep "edugo-infrastructure/mongodb" go.mod
```

**Actualizar a nueva versión:**
```bash
# Ejemplo: actualizar a v0.12.1
go get github.com/EduGoGroup/edugo-infrastructure/mongodb@v0.12.1
go mod tidy
```

**Validaciones post-actualización:**
- [ ] `go.mod` tiene la versión correcta
- [ ] `go mod tidy` ejecuta sin errores
- [ ] `go.sum` se actualiza correctamente
- [ ] No hay conflictos de versiones

---

### 4. Compilación

```bash
# Limpiar y compilar
make clean
make build
```

**Criterios:**
- [ ] Compilación exitosa sin errores
- [ ] No hay warnings de deprecación
- [ ] No hay warnings de campos no usados
- [ ] Binary se genera correctamente

---

### 5. Tests Unitarios

```bash
# Ejecutar todos los tests
make test

# Tests específicos de material_assessment
go test ./internal/infrastructure/persistence/mongodb/repository -v -run TestMaterialAssessment
go test ./internal/domain/entity -v -run TestMaterialAssessment
```

**Criterios:**
- [ ] Todos los tests pasan
- [ ] No hay tests skipped
- [ ] No hay errores de serialización BSON
- [ ] Repository crea/lee documentos correctamente

---

### 6. Validación de Integración (Opcional pero Recomendado)

**Setup ambiente local:**
```bash
# Levantar MongoDB
docker-compose up -d mongodb

# Esperar a que MongoDB esté listo
sleep 5
```

**Test manual de integración:**
```bash
# Ejecutar worker
make run

# En otra terminal, publicar evento de test
./scripts/publish-test-event.sh material_uploaded_for_assessment
```

**Verificar en MongoDB:**
```bash
# Conectar a MongoDB
docker exec -it edugo-mongodb mongosh

# Verificar colección y documentos
use edugo
db.material_assessment_worker.find().pretty()
```

**Criterios:**
- [ ] Documentos se crean en `material_assessment_worker`
- [ ] Estructura de documento es correcta
- [ ] Timestamps se generan automáticamente
- [ ] Metadata se serializa correctamente

---

### 7. Validación de Cobertura

```bash
# Generar reporte de cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Ver porcentaje
go tool cover -func=coverage.out | grep total
```

**Meta:**
- [ ] Cobertura total se mantiene o mejora
- [ ] `material_assessment_repository.go`: >70%
- [ ] `material_assessment.go`: >80%

---

### 8. Validación de PR

**Antes de crear PR:**
- [ ] Todos los tests locales pasan
- [ ] Código compila sin warnings
- [ ] Commits siguen convención
- [ ] Branch está actualizado con `dev`

**Template de PR completado:**
- [ ] Título descriptivo
- [ ] Descripción clara de cambios
- [ ] Checklist de validación marcado
- [ ] Referencias a documentos/issues
- [ ] Screenshots/evidencias (si aplica)

**CI/CD:**
- [ ] Pipeline de CI pasa
- [ ] Tests automáticos pasan
- [ ] Linters pasan
- [ ] No hay vulnerabilidades detectadas

---

## 🎯 Criterios de Aceptación Final

✅ **FASE 2.5 EXITOSA** si:

1. ✅ Repository usa `material_assessment_worker` (confirmado)
2. ✅ Entity tiene todos los campos del esquema completo
3. ✅ Dependencia `edugo-infrastructure` actualizada
4. ✅ Compilación exitosa sin errores ni warnings
5. ✅ Todos los tests unitarios pasan
6. ✅ (Opcional) Test manual de integración exitoso
7. ✅ PR aprobado y mergeado a `dev`
8. ✅ Tag `fase-2.5-complete` creado

---

## 🚨 Red Flags - Detener si:

⛔ **Compilación falla** después de actualizar dependencias
- Investigar breaking changes en nueva versión
- Revisar changelogs de `edugo-infrastructure`
- Considerar rollback temporal

⛔ **Tests masivamente fallan** (>30%)
- Probable incompatibilidad de esquema
- Revisar cambios en interfaces
- Consultar con equipo de infraestructura

⛔ **Repository usa colección incorrecta**
- Corregir antes de continuar
- Actualizar repository
- Agregar tests para prevenir regresión

⛔ **Campos faltantes en entity**
- Agregar campos al entity
- Actualizar tests
- Documentar cambios

---

## 📊 Reporte de Validación

Al completar la fase, generar reporte con:

```markdown
# Reporte de Validación - Fase 2.5

**Fecha:** YYYY-MM-DD
**Ejecutado por:** [nombre]

## Resultados

### Verificación de Código
- Colección correcta: ✅/❌
- Entity completa: ✅/❌

### Dependencias
- Versión anterior: vX.Y.Z
- Versión nueva: vA.B.C
- Actualización exitosa: ✅/❌

### Compilación y Tests
- Build: ✅/❌
- Tests unitarios: XX/XX pasaron
- Cobertura: XX%

### Integración (si aplica)
- Test manual: ✅/❌
- MongoDB verificado: ✅/❌

## Observaciones
- [Cualquier hallazgo relevante]

## Siguiente Paso
- PR creado: [link]
- Tag creado: fase-2.5-complete
```

---

**Última actualización:** 2025-12-23
