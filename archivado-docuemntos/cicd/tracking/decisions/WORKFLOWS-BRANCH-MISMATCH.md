# Decisión: Discrepancia en Nombre de Branch para Workflows

**Fecha:** 2025-11-22
**Sprint:** SPRINT-3
**Fase:** FASE 3
**Severidad:** Media (No bloquea merge, pero afecta CI/CD automático)

---

## 🔍 Hallazgo

Durante la creación del PR #21 hacia `dev`, se detectó que los workflows CI/CD no se ejecutan automáticamente.

### Síntoma

```bash
$ gh pr checks 21
no checks reported on the 'claude/start-sprint-3-01Rbn5p78mT73Q3C5qoN8wwF' branch
```

### Causa Raíz

**Discrepancia de nombres:**
- **Branch real:** `dev` (confirmado con `git branch -r`)
- **Workflows configurados para:** `develop`

**Evidencia en workflows:**

`.github/workflows/ci.yml`:
```yaml
on:
  pull_request:
    branches: [ main, develop ]  # ⚠️ Dice "develop"
```

`.github/workflows/test.yml`:
```yaml
on:
  pull_request:
    branches: [ main, develop ]  # ⚠️ Dice "develop"
```

**Resultado:** Los workflows no se activan porque el PR es hacia `dev`, no `develop`.

---

## 📊 Impacto

### Impacto Actual
- ❌ CI/CD no se ejecuta automáticamente en PRs hacia `dev`
- ❌ No hay validación automática de tests en PRs
- ❌ No hay validación automática de coverage en PRs
- ✅ PR puede mergearse manualmente sin bloqueos

### Impacto Potencial
- ⚠️ Código sin validar podría mergearse a `dev`
- ⚠️ Errores podrían pasar desapercibidos
- ⚠️ Coverage podría degradarse sin detección

---

## ✅ Opciones de Solución

### Opción 1: Actualizar Workflows para Usar "dev" ⭐ RECOMENDADO

**Cambios requeridos:**
```yaml
# ci.yml y test.yml
on:
  pull_request:
    branches: [ main, dev ]  # Cambiar "develop" → "dev"
```

**Pros:**
- ✅ Alineado con estructura de branches real
- ✅ Mínimo cambio (2 archivos)
- ✅ Solución inmediata

**Contras:**
- ⚠️ Requiere commit adicional en el PR

---

### Opción 2: Renombrar Branch "dev" → "develop"

**Comandos:**
```bash
git branch -m dev develop
git push origin :dev develop
git push origin -u develop
```

**Pros:**
- ✅ Workflows quedan como están
- ✅ Nombre más estándar ("develop")

**Contras:**
- ❌ Requiere coordinación con todo el equipo
- ❌ Puede romper integraciones existentes
- ❌ Más complejo y riesgoso

---

### Opción 3: Agregar Ambos Nombres a Workflows

**Cambios:**
```yaml
on:
  pull_request:
    branches: [ main, dev, develop ]  # Soportar ambos
```

**Pros:**
- ✅ Flexible, soporta ambos nombres
- ✅ Sin breaking changes

**Contras:**
- ⚠️ Redundancia innecesaria
- ⚠️ Confusión a largo plazo

---

## 🎯 Decisión Tomada

**NO tomar acción inmediata** en este PR por las siguientes razones:

1. **Scope del Sprint 3:** Este sprint se enfoca en consolidación Docker y Go 1.25
2. **Validaciones locales exitosas:** Build, tests, fmt, vet pasaron localmente
3. **PR puede mergearse:** No hay bloqueos técnicos para el merge
4. **Mejor momento:** Resolver en Sprint 4 o tarea independiente

### Acción Recomendada para el Usuario

El usuario debe decidir:

**a) Corregir ahora (en este PR):**
- Actualizar `ci.yml` y `test.yml` para usar `dev`
- Commit adicional
- Push y re-verificar workflows

**b) Corregir después (Sprint 4 o task independiente):**
- Mergear PR #21 sin CI/CD automático
- Crear issue/task para corregir en futuro
- Documentar en backlog

**c) Ejecutar workflows manualmente:**
- Ir a GitHub Actions UI
- Ejecutar `ci.yml` y `test.yml` manualmente con workflow_dispatch
- Validar resultados antes de mergear

---

## 📝 Documentación Temporal

### Workaround para Este PR

**Ejecutar workflows manualmente:**

1. Ir a: https://github.com/EduGoGroup/edugo-worker/actions
2. Seleccionar "CI Pipeline"
3. Click en "Run workflow"
4. Seleccionar branch: `claude/start-sprint-3-01Rbn5p78mT73Q3C5qoN8wwF`
5. Repetir para "Tests with Coverage"

**Validación alternativa:**
- ✅ Build local exitoso (confirmado)
- ✅ Tests locales exitosos (confirmado)
- ✅ go fmt exitoso (confirmado)
- ✅ go vet exitoso (confirmado)

---

## 🔄 Actualización de SPRINT-STATUS.md

Este hallazgo debe documentarse en:
- `SPRINT-STATUS.md` - Sección "Bloqueos y Decisiones"
- `FASE-3-COMPLETE.md` - Sección "Limitaciones Conocidas"

---

## 📋 Checklist de Resolución Futura

Cuando se decida resolver:

- [ ] Decidir opción (1, 2, o 3)
- [ ] Actualizar workflows (si Opción 1 o 3)
- [ ] Renombrar branch (si Opción 2)
- [ ] Probar con PR de prueba
- [ ] Verificar que CI/CD se ejecuta automáticamente
- [ ] Actualizar documentación
- [ ] Comunicar cambio al equipo (si Opción 2)

---

## 🎯 Conclusión

**Estado:** DOCUMENTADO - NO BLOQUEANTE
**Acción Inmediata:** Ninguna (decisión del usuario)
**Recomendación:** Opción 1 (actualizar workflows para usar "dev")
**Prioridad:** Media (puede resolverse después del merge)

---

**Última actualización:** 2025-11-22
**Autor:** Claude Code
**Requiere decisión del usuario:** Sí
