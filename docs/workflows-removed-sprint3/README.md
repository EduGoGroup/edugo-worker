# Workflows Eliminados - Sprint 3

**Fecha:** 2025-11-22
**Sprint:** SPRINT-3
**Razón:** Consolidación de workflows Docker

---

## Workflows Eliminados

### 1. build-and-push.yml
**Razón:** Duplicado de manual-release.yml sin tests previos.
**Funcionalidad migrada a:** manual-release.yml

**Características:**
- Trigger: Manual + Push a main
- Sin tests previos
- Tags: branch, sha, latest
- Variables de environment (development/staging/production)

**Por qué se eliminó:**
- No ejecutaba tests antes de build
- Funcionalidad duplicada en manual-release.yml
- manual-release.yml tiene mejor control y validaciones

---

### 2. docker-only.yml
**Razón:** Duplicado simple sin control fino.
**Funcionalidad migrada a:** manual-release.yml

**Características:**
- Trigger: Manual
- Sin tests previos
- Tags: custom, latest
- Multi-platform (linux/amd64 + linux/arm64)

**Por qué se eliminó:**
- No ejecutaba tests antes de build
- Funcionalidad duplicada en manual-release.yml
- manual-release.yml ya tiene multi-platform

---

### 3. release.yml
**Razón:** Fallando + duplicado de manual-release.yml.
**Funcionalidad migrada a:** manual-release.yml

**Características:**
- Trigger: Tag push (v*)
- Con tests previos
- Codecov upload
- GitHub Release con changelog generado
- Tags: semver completos

**Por qué se eliminó:**
- Workflow fallando (ver Run 19485700108)
- Trigger automático en tag push menos controlable que manual
- Funcionalidad completa disponible en manual-release.yml
- Control manual es más seguro para releases

---

## Workflow Mantenido

**manual-release.yml** - Workflow completo y funcional con:

✅ **Tests previos:** Ejecuta suite completa antes de build
✅ **Control fino:** Inputs para version + bump_type
✅ **Multi-platform:** linux/amd64, linux/arm64
✅ **GitHub Release:** Crea release con changelog automático
✅ **CHANGELOG automático:** Actualiza CHANGELOG.md
✅ **GitHub App Token:** Dispara workflows subsecuentes
✅ **version.txt:** Actualiza archivo de versión
✅ **Tags automáticos:** Crea y pushea tags semver

---

## Comparación de Funcionalidad

| Funcionalidad | build-and-push | docker-only | release | manual-release |
|---------------|----------------|-------------|---------|----------------|
| Tests previos | ❌ | ❌ | ✅ | ✅ |
| Multi-platform | ❌ | ✅ | ❌ | ✅ |
| Control manual | ⚠️ Parcial | ✅ | ❌ Auto | ✅ |
| GitHub Release | ❌ | ❌ | ✅ | ✅ |
| CHANGELOG | ❌ | ❌ | ⚠️ Limitado | ✅ Completo |
| version.txt | ❌ | ❌ | ❌ | ✅ |
| GitHub App Token | ❌ | ❌ | ❌ | ✅ |
| Codecov | ❌ | ❌ | ✅ | ⚠️ Via test.yml |
| Bump types | ❌ | ❌ | ❌ | ✅ patch/minor/major |

**Conclusión:** manual-release.yml tiene TODA la funcionalidad de los otros workflows y más.

---

## Impacto de la Consolidación

### Reducción
- **Workflows Docker:** 4 → 1 (-75%)
- **Líneas de código:** ~26,000 caracteres → ~11,500 (-56%)
- **Complejidad:** Múltiples flujos → Un flujo unificado

### Beneficios
1. **Claridad:** Un solo lugar para releases
2. **Mantenibilidad:** Cambios en un solo archivo
3. **Consistencia:** Mismo proceso para todos los releases
4. **Seguridad:** Control manual evita releases accidentales
5. **Validación:** Tests siempre ejecutados antes de build
6. **Trazabilidad:** CHANGELOG y version.txt siempre actualizados

### Riesgos Mitigados
- ❌ Tags duplicados por múltiples workflows
- ❌ Confusión sobre qué workflow usar
- ❌ Releases sin tests
- ❌ CHANGELOG desactualizado
- ❌ Workflows fallando

---

## Restauración

Si necesitas restaurar algún workflow (NO recomendado):

```bash
# Restaurar build-and-push.yml
cp docs/workflows-removed-sprint3/build-and-push.yml.backup .github/workflows/build-and-push.yml

# Restaurar docker-only.yml
cp docs/workflows-removed-sprint3/docker-only.yml.backup .github/workflows/docker-only.yml

# Restaurar release.yml
cp docs/workflows-removed-sprint3/release.yml.backup .github/workflows/release.yml

# Commit y push
git add .github/workflows/
git commit -m "Restaurar workflow [nombre]"
git push
```

**⚠️ ADVERTENCIA:** Restaurar estos workflows volverá a crear los problemas de duplicación.

---

## Uso del Nuevo Workflow Consolidado

### GitHub UI (Recomendado)

1. Ir a: https://github.com/EduGoGroup/edugo-worker/actions/workflows/manual-release.yml
2. Click en "Run workflow"
3. Seleccionar rama: `main`
4. Ingresar versión: `0.1.0` (sin 'v')
5. Seleccionar tipo: `patch` / `minor` / `major`
6. Click "Run workflow"

### GitHub CLI

```bash
# Patch release (0.0.1 → 0.0.2)
gh workflow run manual-release.yml -f version=0.0.2 -f bump_type=patch

# Minor release (0.0.1 → 0.1.0)
gh workflow run manual-release.yml -f version=0.1.0 -f bump_type=minor

# Major release (0.0.1 → 1.0.0)
gh workflow run manual-release.yml -f version=1.0.0 -f bump_type=major
```

Ver [docs/RELEASE-WORKFLOW.md](../RELEASE-WORKFLOW.md) para guía completa.

---

## Referencias

- [Análisis de Duplicación](../cicd/README.md#análisis-de-duplicación-docker)
- [Sprint 3 Tasks](../cicd/sprints/SPRINT-3-TASKS.md#tarea-1)
- [RELEASE-WORKFLOW.md](../RELEASE-WORKFLOW.md)
- [manual-release.yml](../../.github/workflows/manual-release.yml)

---

## Historial

| Fecha | Acción | Responsable |
|-------|--------|-------------|
| 2025-11-22 | Workflows respaldados | Claude Code |
| 2025-11-22 | Workflows eliminados | Claude Code |
| 2025-11-22 | README creado | Claude Code |

---

**Última actualización:** 2025-11-22
**Sprint:** SPRINT-3
**Generado por:** Claude Code
