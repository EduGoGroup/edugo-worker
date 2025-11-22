# Lecciones Aprendidas: SPRINT-4 Workflows Reusables

**Proyecto Piloto:** edugo-api-mobile  
**Fecha:** 2025-11-21  
**Para:** edugo-api-administracion, edugo-worker  
**Propósito:** Evitar errores comunes al migrar a workflows reusables

---

## 🎯 Resumen Ejecutivo

El proyecto **api-mobile** completó SPRINT-4 FASE 1 y FASE 2, encontrando y resolviendo **5 problemas críticos** de configuración. Este documento te permitirá **evitar 90 minutos de debugging** y aplicar los fixes desde el inicio.

---

## ⚠️ PROBLEMAS CRÍTICOS Y SOLUCIONES

### Problema 1: Permisos de Workflows Reusables 🔴 CRÍTICO

**Síntoma:**
```
X This run likely failed because of a workflow file issue.
```

**Causa:**
Por defecto, workflows reusables en `edugo-infrastructure` NO son accesibles desde otros repos.

**✅ SOLUCIÓN ANTES DE EMPEZAR:**

1. **Verificar permisos (REQUIERE ADMIN):**
   ```
   Ir a: https://github.com/EduGoGroup/edugo-infrastructure/settings/actions
   ```

2. **Configuración correcta:**
   ```
   ● Allow EduGoGroup, and select non-EduGoGroup, actions and reusable workflows
   ```

3. **Si ves otra opción seleccionada, cámbiala a esta**

**Tiempo ahorrado:** 15 minutos

---

### Problema 2: Subdirectorio NO Permitido 🔴 CRÍTICO

**Síntoma:**
```
invalid value workflow reference: workflows must be defined at the top level 
of the .github/workflows/ directory
```

**Causa:**
GitHub Actions NO permite workflows reusables en subdirectorios.

**✅ SOLUCIÓN:**

**NO usar:**
```yaml
uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable/go-lint.yml@main
                                                      ^^^^^^^^^ subdirectorio
```

**SÍ usar:**
```yaml
uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
                                                      ^^^^^^^^^^^^^^^^^ sin subdirectorio
```

**Estructura correcta en infrastructure:**
```
.github/workflows/
├── reusable-go-lint.yml      ✅ CORRECTO
├── reusable-go-test.yml      ✅ CORRECTO
├── reusable-docker-build.yml ✅ CORRECTO
└── reusable/
    └── go-lint.yml            ❌ NO FUNCIONA
```

**Tiempo ahorrado:** 20 minutos

---

### Problema 3: GITHUB_TOKEN es Nombre Reservado 🔴 CRÍTICO

**Síntoma:**
```
secret name `GITHUB_TOKEN` within `workflow_call` can not be used since 
it would collide with system reserved name
```

**Causa:**
GITHUB_TOKEN es un nombre reservado del sistema.

**✅ SOLUCIÓN:**

**NO declarar en workflow reusable:**
```yaml
# ❌ NO HACER ESTO en infrastructure
on:
  workflow_call:
    secrets:
      GITHUB_TOKEN:    # ❌ Nombre reservado
        required: false
```

**NO pasar en workflow caller:**
```yaml
# ❌ NO HACER ESTO en api-administracion/worker
jobs:
  lint:
    uses: ...reusable-go-lint.yml@main
    secrets:
      GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}  # ❌ No permitido
```

**SÍ usar github.token directamente en el workflow reusable:**
```yaml
# ✅ HACER ESTO en infrastructure
steps:
  - name: Setup Go
    uses: .../setup-edugo-go@main
    with:
      github-token: ${{ github.token }}  # ✅ Disponible automáticamente
```

**Tiempo ahorrado:** 15 minutos

---

### Problema 4: Incompatibilidad golangci-lint-action 🟡 IMPORTANTE

**Síntoma:**
```
invalid version string 'v2.4.0', golangci-lint v2 is not supported by 
golangci-lint-action v6
```

**Causa:**
`golangci-lint-action v6` NO soporta `golangci-lint v2.x`

**✅ SOLUCIÓN:**

El workflow reusable en infrastructure YA ESTÁ ACTUALIZADO a:
- `golangci-lint-action@v7` (soporta golangci-lint v2.x)
- Default golangci-lint: `v2.4.0`

**En tus workflows, NO especificar golangci-lint-version:**
```yaml
# ✅ HACER ESTO
lint:
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
    # NO incluir: golangci-lint-version (usa default v2.4.0)
```

**Tiempo ahorrado:** 20 minutos

---

### Problema 5: Go Version de golangci-lint 🟡 IMPORTANTE

**Síntoma:**
```
Error: can't load config: the Go language version (go1.24) used to build 
golangci-lint is lower than the targeted Go version (1.25)
```

**Causa:**
golangci-lint v1.x fue compilado con Go 1.24, pero el proyecto usa Go 1.25.

**✅ SOLUCIÓN:**

Ya está resuelto en infrastructure con:
- golangci-lint v2.4.0+ (compilado con Go 1.25)

**No tienes que hacer nada, solo usar el workflow reusable sin especificar versión.**

**Tiempo ahorrado:** 30 minutos

---

## 📝 Checklist Pre-Migración

Antes de empezar Sprint 4, verificar:

### En edugo-infrastructure (Verificar primero)

- [ ] Permisos de Actions configurados correctamente
- [ ] Workflows reusables en `.github/workflows/reusable-*.yml` (raíz)
- [ ] Workflow reusable usa `golangci-lint-action@v7`
- [ ] Default golangci-lint es `v2.4.0` o superior
- [ ] NO tiene secret `GITHUB_TOKEN` declarado

**Comando de verificación:**
```bash
cd edugo-infrastructure
git checkout main
git pull origin main

# Verificar ubicación correcta
ls -la .github/workflows/reusable-go-lint.yml  # Debe existir

# Verificar versión de action
grep "golangci-lint-action@" .github/workflows/reusable-go-lint.yml
# Debe mostrar: @v7

# Verificar default de golangci-lint
grep "default: 'v" .github/workflows/reusable-go-lint.yml
# Debe mostrar: v2.4.0 o superior

# Verificar NO tiene GITHUB_TOKEN
grep -A 3 "secrets:" .github/workflows/reusable-go-lint.yml
# NO debe mostrar GITHUB_TOKEN
```

### En tu proyecto (api-administracion o worker)

- [ ] Tienes Sprint 4 documentado
- [ ] Entiendes qué workflows migrarás
- [ ] Has leído este documento completo
- [ ] Git está en branch `dev` actualizado

---

## 🚀 Plantilla de Migración Correcta

### Para pr-to-dev.yml y pr-to-main.yml

```yaml
# =====================================================
# MIGRADO: Job lint usando workflow reusable
# Migrado en SPRINT-4 para usar workflows centralizados
# Ver: edugo-infrastructure/.github/workflows/reusable-go-lint.yml
# =====================================================
lint:
  name: Lint & Format Check
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
    # NO incluir golangci-lint-version (usa default v2.4.0)
    # NO incluir secrets (github.token está disponible automáticamente)
```

**Cosas a EVITAR:**
```yaml
# ❌ NO HACER
lint:
  uses: .../reusable/go-lint.yml@main           # ❌ subdirectorio
  with:
    golangci-lint-version: "v1.64.7"            # ❌ versión incompatible
  secrets:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}   # ❌ nombre reservado
```

---

## ⏱️ Tiempo Estimado vs Realidad

### Estimación Original (SPRINT-4-TASKS.md)
- Tarea 4.6: Migrar pr-to-dev.yml → 60 min
- Tarea 4.7: Migrar pr-to-main.yml → 60 min
- **Total:** 120 min

### Realidad en api-mobile (SIN este documento)
- Migración: 15 min
- Debugging: 90 min ⚠️
- **Total:** 105 min

### Estimación con este documento
- Migración: 15 min
- Debugging: 0 min ✅ (evitado)
- **Total:** 15 min

**Ahorro:** 90 minutos por proyecto 🎉

---

## 📋 Orden Recomendado de Ejecución

### Opción A: Migración Rápida (Recomendado)

1. **Leer este documento** (10 min)
2. **Verificar infrastructure** con checklist (5 min)
3. **Hacer backup de workflows** (según Sprint 4 Tarea 4.5)
4. **Migrar pr-to-dev.yml** usando plantilla correcta (5 min)
5. **Migrar pr-to-main.yml** usando plantilla correcta (5 min)
6. **Commit y push** (5 min)
7. **Crear PR de prueba** (5 min)
8. **Validar que funciona** (5 min)
9. **Cerrar PR y documentar** (5 min)

**Total:** ~50 minutos (vs 105 min de api-mobile)

### Opción B: Migración Conservadora

Si prefieres seguir el plan original del Sprint 4:

1. Seguir SPRINT-4-TASKS.md línea por línea
2. PERO usar las plantillas correctas de este documento
3. PERO saltar los problemas ya resueltos

**Total:** ~3-4 horas (vs 12-15h estimadas)

---

## 🔍 Comandos de Validación

### Antes de crear PR de prueba:

```bash
# 1. Validar sintaxis YAML
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pr-to-dev.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pr-to-main.yml'))"

# 2. Verificar referencia correcta
grep "uses: EduGo" .github/workflows/pr-to-dev.yml
# Debe mostrar: .../reusable-go-lint.yml@main (sin subdirectorio)

# 3. Verificar NO tiene secrets GITHUB_TOKEN
grep -A 2 "secrets:" .github/workflows/pr-to-dev.yml
# NO debe mostrar GITHUB_TOKEN

# 4. Verificar NO especifica golangci-lint-version incompatible
grep "golangci-lint-version" .github/workflows/pr-to-dev.yml
# Mejor si no aparece nada (usa default)
```

### Después de crear PR:

```bash
# 1. Esperar 30 segundos para que workflow inicie
sleep 30

# 2. Verificar checks
gh pr checks <PR_NUMBER>

# 3. Si falla, ver error específico
gh run list --limit 1
gh run view <RUN_ID>

# 4. Si el error es uno de los 5 de arriba, usar la solución documentada
```

---

## 📊 Comparación de Proyectos

| Aspecto | api-mobile | api-administracion | worker |
|---------|------------|-------------------|--------|
| **Workflows actuales** | 5 | ? | ? |
| **Complejidad** | Media | Similar | Diferente |
| **Usa Docker** | Sí | Sí | Depende |
| **Patrón aplicable** | ✅ Validado | ✅ Muy probable | ⚠️ Revisar |

**Recomendación:**
- **api-administracion:** Seguir patrón de api-mobile 1:1
- **worker:** Revisar diferencias primero, adaptar según sea necesario

---

## 🎁 Bonus: Script de Migración Rápida

```bash
#!/bin/bash
# quick-migrate-sprint4.sh
# Migración rápida aplicando lecciones aprendidas de api-mobile

WORKFLOW_FILE=".github/workflows/pr-to-dev.yml"

echo "🚀 Migración rápida de pr-to-dev.yml"
echo ""

# Backup
cp "$WORKFLOW_FILE" "${WORKFLOW_FILE}.backup-$(date +%Y%m%d)"
echo "✅ Backup creado"

# Verificar que infrastructure tiene workflows correctos
echo ""
echo "⚠️  IMPORTANTE: Verificar que infrastructure tiene:"
echo "  1. Permisos configurados (Settings → Actions)"
echo "  2. Workflow en: .github/workflows/reusable-go-lint.yml (raíz)"
echo "  3. golangci-lint-action@v7"
echo "  4. Default golangci-lint v2.4.0+"
echo ""
read -p "¿Verificaste? (y/n): " confirmed

if [ "$confirmed" != "y" ]; then
  echo "❌ Abortado. Verifica infrastructure primero."
  exit 1
fi

echo ""
echo "📝 Plantilla correcta para job lint:"
cat << 'TEMPLATE'

  lint:
    name: Lint & Format Check
    uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
    with:
      go-version: "1.25"
      args: "--timeout=5m"

TEMPLATE

echo ""
echo "⚠️  Aplica esta plantilla manualmente al job lint en:"
echo "  $WORKFLOW_FILE"
echo ""
echo "✅ Guía completa en: SPRINT-4-LESSONS-LEARNED.md"
```

**Uso:**
```bash
chmod +x quick-migrate-sprint4.sh
./quick-migrate-sprint4.sh
```

---

## 📞 Contacto y Soporte

Si encuentras un problema NO documentado aquí:

1. **Revisar:** `edugo-api-mobile/docs/cicd/tracking/FASE-2-COMPLETE.md`
2. **Revisar:** `edugo-api-mobile/docs/cicd/tracking/errors/ERROR-2025-11-21-18-35.md`
3. **Documentar:** Crear tu propio `ERROR-YYYY-MM-DD.md` con detalles
4. **Compartir:** Actualizar este documento con la nueva lección

---

## ✅ Validación Rápida Post-Migración

Después de migrar, crear PR de prueba y verificar:

```bash
# Crear PR
gh pr create --base dev --title "Test: SPRINT-4 workflows reusables"

# Esperar 2 minutos
sleep 120

# Verificar checks
gh pr checks <PR_NUMBER>

# Resultado esperado:
# ✓ Lint & Format Check / Run Linter (workflow reusable)
# ✓ Unit Tests (custom)
# ✓ PR Summary (custom)
```

**Si algún check falla:**
1. Ver el error en GitHub UI
2. Buscar el error en este documento
3. Aplicar la solución documentada
4. Commit y push
5. Esperar nuevo run

---

## 🎯 TL;DR (Muy Corto)

### Configuración Correcta

1. **Infrastructure:**
   - Permisos habilitados ✅
   - Workflows en raíz (reusable-*.yml) ✅
   - golangci-lint-action@v7 ✅
   - NO secret GITHUB_TOKEN ✅

2. **Tu proyecto:**
   ```yaml
   lint:
     uses: .../reusable-go-lint.yml@main  # Sin subdirectorio
     with:
       go-version: "1.25"
       args: "--timeout=5m"
       # NO golangci-lint-version
       # NO secrets
   ```

3. **Resultado esperado:**
   ```
   ✓ Lint & Format Check / Run Linter
   ```

---

## 🔗 Referencias

- **Proyecto Piloto:** edugo-api-mobile
- **Documentación completa:** `edugo-api-mobile/docs/cicd/tracking/FASE-2-COMPLETE.md`
- **Errores detallados:** `edugo-api-mobile/docs/cicd/tracking/errors/`
- **GitHub Docs:** https://docs.github.com/en/actions/using-workflows/reusing-workflows

---

**✅ Usa este documento y ahorra 90 minutos de debugging**

**Generado por:** Claude Code (desde api-mobile)  
**Fecha:** 2025-11-21  
**Versión:** 1.0

---

## 🎯 FASE 3: Merge a Dev (IMPORTANTE)

**⚠️ DESPUÉS DE COMPLETAR FASE 2 (Validación Exitosa)**

Una vez que hayas validado que los workflows reusables funcionan correctamente en tu PR de prueba, **DEBES hacer merge a dev** para que los cambios queden permanentes.

---

### ❌ Error Común: Cerrar PR sin Merge

**Lo que NO debes hacer:**
```bash
# ❌ NO HACER ESTO
gh pr close <PR_NUMBER> --delete-branch

# Esto elimina el branch SIN mergear los cambios
# Los workflows migrados se pierden
```

**Resultado:** Workflows migrados NO quedan en dev, se pierden.

---

### ✅ Proceso Correcto de FASE 3

#### Paso 1: Validar que PR está exitoso

```bash
# Verificar que todos los checks pasaron
gh pr checks <PR_NUMBER>

# Resultado esperado:
# ✓ Lint & Format Check / Run Linter
# ✓ Unit Tests
# ✓ PR Summary (u otros checks custom)
```

#### Paso 2: Hacer Merge a Dev

```bash
# Opción A: Usando gh CLI (recomendado)
gh pr merge <PR_NUMBER> --merge --delete-branch

# Opción B: Desde GitHub UI
# Ir al PR y click en "Merge pull request"
```

#### Paso 3: Sincronizar dev local

```bash
git checkout dev
git pull origin dev

echo "✅ dev sincronizado con workflows migrados"
```

#### Paso 4: Verificar workflows en dev

```bash
# Verificar que workflows tienen referencias correctas
grep "uses: EduGo" .github/workflows/pr-to-dev.yml
grep "uses: EduGo" .github/workflows/pr-to-main.yml

# Resultado esperado:
# uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
```

#### Paso 5: Actualizar tracking

Marcar en tu `SPRINT-STATUS.md`:
```markdown
✅ FASE 3: COMPLETADA
- PR mergeado a dev
- Workflows migrados activos
- CI/CD post-merge: exitoso
```

---

### 📊 Checklist de FASE 3

Antes de dar por completado Sprint 4:

- [ ] PR de validación creado
- [ ] Todos los checks pasaron
- [ ] **PR mergeado a dev** (NO solo cerrado)
- [ ] dev local sincronizado
- [ ] Workflows verificados en dev
- [ ] Documentación actualizada
- [ ] SPRINT-STATUS.md marcado como completado

---

### ⏱️ Tiempo FASE 3

- Validación: Ya hecho en FASE 2
- Merge: ~5 minutos
- Sincronización: ~2 minutos
- Verificación: ~3 minutos

**Total FASE 3:** ~10 minutos

---

### 🔄 Flujo Completo (Resumen)

```
FASE 1: Migrar workflows
   ↓
FASE 2: Validar con PR de prueba
   ↓ (SI todos los checks pasan)
   ↓
FASE 3: MERGE a dev ← NO OLVIDAR ESTE PASO
   ↓
✅ SPRINT-4 COMPLETADO
```

---

**⚠️ RECORDATORIO CRÍTICO:**

Antes de empezar tu FASE 2, haz:
```bash
git checkout dev
git pull origin dev
```

Esto asegura que tengas las últimas actualizaciones, incluyendo este documento y cualquier fix de infrastructure.

---

---

## 📝 FASE 3 Extendida: PR a Main (Opcional)

**Después de mergear a dev exitosamente**

Si quieres llevar los workflows migrados a `main` (producción):

### Paso 1: Crear PR de dev a main

```bash
gh pr create \
  --base main \
  --head dev \
  --title "Release: Sprint 4 - Workflows Reusables" \
  --body "Migración de workflows a reusables centralizados (ver detalles en cuerpo del PR)"
```

### Paso 2: Monitorear CI/CD (máx 5 min)

```bash
# Esperar que checks ejecuten
sleep 120

# Verificar checks
gh pr checks <PR_NUMBER>

# Resultado esperado:
# ✓ Lint & Format Check / Run Linter (workflow reusable)
# ✓ Unit Tests  
# ✓ Integration Tests (si aplica)
# ✓ Security Scan (si aplica)
# ✓ PR Summary
```

### Paso 3: Mergear a main

```bash
gh pr merge <PR_NUMBER> --merge
```

### Paso 4: Sincronizar main local

```bash
git checkout main
git pull origin main

echo "✅ Workflows reusables activos en main"
```

---

**⏱️ Tiempo FASE 3 Extendida:** +10 minutos adicionales

**🎯 Beneficio:** Workflows reusables en producción (main)

---
