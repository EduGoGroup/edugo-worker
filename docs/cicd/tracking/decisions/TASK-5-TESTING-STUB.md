# Decisión: Tarea 5 Testing - STUB en FASE 1

**Fecha:** 2025-11-22
**Tarea:** 5 - Testing y Validación
**Sprint:** SPRINT-4
**Fase:** FASE 1

---

## 🚫 Problema

Los workflows reusables referenciados en `ci.yml` y `test.yml` **no existen aún** en `edugo-infrastructure`.

Por lo tanto, no es posible ejecutar testing real en FASE 1.

---

## ✅ Decisión: Documentar Testing para FASE 2

**En lugar de ejecutar tests que fallarán, voy a:**

1. **Documentar** el plan de testing para FASE 2
2. **Preparar** instrucciones para validación
3. **Marcar** tarea como "✅ (stub)"

---

## 📋 Plan de Testing para FASE 2

### Pre-requisitos

1. ✅ Workflows reusables creados en infrastructure:
   - `edugo-infrastructure/.github/workflows/reusable-go-lint.yml`
   - `edugo-infrastructure/.github/workflows/reusable-go-test.yml`

2. ✅ PR en infrastructure mergeado a main

3. ✅ Permisos de Actions configurados en infrastructure

### Paso 1: Validación Sintáctica (Local)

**Comandos:**
```bash
# Validar sintaxis YAML
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"

# Verificar referencias correctas
grep "uses: EduGo" .github/workflows/ci.yml
grep "uses: EduGo" .github/workflows/test.yml

# Verificar NO usa subdirectorio
! grep "reusable/go-lint" .github/workflows/ci.yml
! grep "reusable/go-test" .github/workflows/test.yml

# Verificar NO pasa GITHUB_TOKEN
! grep "GITHUB_TOKEN.*secrets" .github/workflows/ci.yml
! grep "GITHUB_TOKEN.*secrets" .github/workflows/test.yml
```

**Resultado esperado:** Todos los comandos pasan ✅

---

### Paso 2: Crear PR de Prueba

**Comandos:**
```bash
# Asegurar que estamos en branch feature
git checkout claude/sprint-4-phase-1-stubs-01QvT5w6jHgvnKFL9FadvQKi

# Pushear si no lo hemos hecho
git push -u origin claude/sprint-4-phase-1-stubs-01QvT5w6jHgvnKFL9FadvQKi

# Crear PR a dev
gh pr create \
  --base dev \
  --title "Sprint 4: Migrar a workflows reusables" \
  --body "$(cat docs/cicd/PR-TEMPLATE-SPRINT4.md)"
```

**Resultado esperado:** PR creado ✅

---

### Paso 3: Monitorear Workflows

**Esperar 2-3 minutos para que workflows inicien:**

```bash
sleep 120
```

**Verificar estado de checks:**

```bash
# Ver checks del PR
gh pr checks

# Ver workflows ejecutándose
gh run list --limit 5

# Ver logs si falla
gh run view --log-failed
```

**Resultado esperado:**

```
✓ CI Pipeline / Lint & Format Check (workflow reusable)
✓ CI Pipeline / Tests and Validations
✓ CI Pipeline / Docker Build Test
✓ Tests with Coverage / Tests with Coverage (workflow reusable)
✓ Tests with Coverage / Integration Tests
```

---

### Paso 4: Validación de Funcionalidad

**Verificar que workflows reusables funcionan correctamente:**

1. **Lint workflow:**
   - ✅ Setup Go 1.25
   - ✅ golangci-lint v2.4.0 ejecutado
   - ✅ Sin errores de lint

2. **Test workflow:**
   - ✅ Setup Go 1.25
   - ✅ Servicios levantados (PostgreSQL, MongoDB, RabbitMQ)
   - ✅ Tests ejecutados con coverage
   - ✅ Coverage threshold verificado (0.0% en FASE 1)
   - ✅ Reporte HTML generado
   - ✅ Upload a Codecov (si configurado)

---

### Paso 5: Resolver Errores (Si los hay)

**Si lint falla:**

1. Ver error específico en logs
2. Buscar en `docs/cicd/SPRINT-4-LESSONS-LEARNED.md`
3. Aplicar fix correspondiente
4. Commit y push
5. Esperar nuevo run

**Errores conocidos y soluciones:**

| Error | Causa | Solución |
|-------|-------|----------|
| "Unable to resolve action" | Workflow reusable no existe | Verificar PR en infrastructure mergeado |
| "invalid value workflow reference" | Subdirectorio usado | Usar `reusable-go-lint.yml` (raíz) |
| "secret name GITHUB_TOKEN can not be used" | GITHUB_TOKEN declarado | Eliminar de secrets |
| "invalid version string v2.4.0" | golangci-lint-action v6 | Usar v7 en workflow reusable |

---

### Paso 6: Validación Exitosa

**Criterios de éxito:**

- ✅ Todos los checks pasan
- ✅ Lint ejecutado correctamente
- ✅ Tests ejecutados con coverage
- ✅ Sin errores en logs
- ✅ Tiempo de ejecución razonable (<10 min)

**Si todos los criterios se cumplen:**

```bash
echo "✅ Validación FASE 2 completada exitosamente"
```

---

## 🔄 Para FASE 2

**Tiempo estimado:** 30-60 minutos

**Pasos:**

1. Ejecutar validación sintáctica (5 min)
2. Crear PR de prueba (5 min)
3. Monitorear workflows (10 min)
4. Validar funcionalidad (10 min)
5. Resolver errores si hay (0-30 min)

---

## 📊 Métricas Esperadas

**Antes (workflows locales completos):**
- ci.yml: 122 líneas
- test.yml: 199 líneas
- Total: 321 líneas

**Después (workflows con referencias a reusables):**
- ci.yml: 109 líneas
- test.yml: 63 líneas
- Total: 172 líneas

**Reducción:** -149 líneas (-46%)

---

## 🗂️ Archivos de Referencia

1. **Plan completo:** `docs/cicd/sprints/SPRINT-4-TASKS.md`
2. **Lecciones aprendidas:** `docs/cicd/SPRINT-4-LESSONS-LEARNED.md`
3. **Workflows migrados:**
   - `.github/workflows/ci.yml`
   - `.github/workflows/test.yml`
4. **Backups:**
   - `docs/workflows-migrated-sprint4/ci.yml.backup`
   - `docs/workflows-migrated-sprint4/test.yml.backup`

---

**Migaja actualizada en SPRINT-STATUS.md:**
- Tarea 5: ✅ (stub) - Plan de testing documentado
- Pendiente para FASE 2

---

**Generado por:** Claude Code
**Fecha:** 2025-11-22
**FASE:** 1 - Implementación con Stubs
