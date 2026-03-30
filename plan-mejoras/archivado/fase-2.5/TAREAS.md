# Tareas - Fase 2.5: Homologación Material Assessment

---

## 📋 Resumen de Tareas

### Día 1: Verificación y Análisis

**T2.5.1: Verificar uso de colección correcta** (1h)
- Buscar todas las referencias a `material_assessment` en el código
- Confirmar que repository usa `material_assessment_worker`
- Verificar que no hay colecciones sin sufijo
- Documentar hallazgos

```bash
# Comando de verificación
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker
grep -r "material_assessment" --include="*.go" . | grep -v "material_assessment_worker"
```

**Resultado esperado:** Solo referencias a `material_assessment_worker`

---

**T2.5.2: Revisar entity MaterialAssessment** (1h)
- Leer `internal/domain/entity/material_assessment.go`
- Verificar que incluye todos los campos del esquema completo:
  - material_id
  - questions
  - total_questions
  - total_points
  - version
  - ai_model
  - processing_time_ms
  - metadata
  - created_at
  - updated_at
- Verificar BSON tags correctos
- Documentar campos faltantes (si los hay)

---

**T2.5.3: Identificar versión actual de infraestructura** (30min)
- Revisar `go.mod`
- Identificar versión actual de `edugo-infrastructure/mongodb`
- Verificar changelogs para identificar versión target
- Documentar versión actual y target

```bash
# Ver versión actual
grep "edugo-infrastructure/mongodb" go.mod
```

---

### Día 1-2: Actualización

**T2.5.4: Actualizar dependencia de infraestructura** (30min)
- Actualizar `go.mod` a la nueva versión con el esquema
- Ejecutar `go mod tidy`
- Revisar cambios en `go.sum`
- Verificar que no hay conflictos de versiones

```bash
# Actualizar a versión específica (ejemplo)
go get github.com/EduGoGroup/edugo-infrastructure/mongodb@v0.12.1
go mod tidy
```

---

**T2.5.5: Compilar proyecto** (15min)
- Ejecutar `make build`
- Verificar que compila sin errores
- Revisar warnings (si los hay)
- Documentar cualquier problema

```bash
make build
```

---

**T2.5.6: Ejecutar tests unitarios** (30min)
- Ejecutar `make test`
- Verificar que todos los tests pasan
- Revisar cobertura
- Documentar tests fallidos (si los hay)

```bash
make test
```

---

**T2.5.7: Prueba de integración manual (opcional)** (1h)
- Levantar MongoDB local
- Ejecutar worker
- Verificar que crea documentos en la colección correcta
- Revisar estructura de documentos guardados
- Confirmar timestamps y metadata

```bash
# Levantar entorno local
docker-compose up -d mongodb

# Ejecutar worker
make run

# Conectar a MongoDB y verificar
docker exec -it edugo-mongodb mongosh
> use edugo
> db.material_assessment_worker.findOne()
```

---

### Día 2: Validación Final

**T2.5.8: Revisar cambios y crear PR** (1h)
- Revisar todos los cambios realizados
- Crear commit con mensaje descriptivo
- Pushear rama a remoto
- Crear PR hacia `dev`
- Completar template de PR con checklist

```bash
git add go.mod go.sum
git commit -m "chore(fase-2.5): actualizar edugo-infrastructure a v0.12.1

- Actualizar dependencia con nuevo esquema material_assessment
- Verificar uso correcto de colección material_assessment_worker
- Validar compilación y tests

Refs: plan-trabajo/03-homologar-material-assessment
Fase: 2.5"
git push origin feature/fase-2.5-homologar-material-assessment
```

---

**T2.5.9: Code Review y Ajustes** (1-2h)
- Responder comentarios de revisión
- Hacer ajustes solicitados
- Re-ejecutar tests si hay cambios
- Aprobar y mergear a `dev`

---

**T2.5.10: Documentación y cierre** (30min)
- Actualizar README principal con estado de fase 2.5
- Documentar lecciones aprendidas
- Notificar a equipo

---

## ✅ Total Estimado: 8 horas (~1 día)

**Desglose por categoría:**
- Verificación: 2.5 horas
- Actualización: 1.25 horas
- Testing: 1.5 horas
- Documentación y PR: 2.5 horas
- Buffer: 0.25 horas

---

## 📝 Notas Importantes

### ⚠️ Antes de Comenzar
- Asegurar que nuevo release de `edugo-infrastructure` está disponible
- Coordinar con otros equipos (api-mobile, api-administracion)
- Tener acceso a MongoDB local para pruebas

### ⚠️ Durante Ejecución
- Si se encuentran campos faltantes en entity, agregarlos
- Si tests fallan, investigar antes de continuar
- Documentar cualquier desviación del plan

### ⚠️ Después de Completar
- Notificar a equipo que fase 2.5 está completa
- Coordinar con otros proyectos para sincronizar versiones
- Actualizar documentación de dependencias

---

## 🔗 Referencias Rápidas

- Ubicación del repository: `internal/infrastructure/persistence/mongodb/repository/material_assessment_repository.go`
- Ubicación de entity: `internal/domain/entity/material_assessment.go`
- Plan original: `/Users/jhoanmedina/source/EduGo/repos-separados/edugo_analisis/plan-trabajo/03-homologar-material-assessment/worker/PLAN.md`

---

**Última actualización:** 2025-12-23
