# 🔄 Plan GitFlow Profesional - EduGo Worker

**Fecha de creación:** 2025-10-31
**Proyecto:** edugo-worker (y futuros: edugo-api-administracion, edugo-api-mobile, edugo-shared)

---

## 🎯 Objetivo

Implementar un flujo de trabajo GitFlow profesional con:
- Protección de ramas `main` y `dev`
- CI/CD automatizado según el contexto
- Versionado automático en releases
- Generación de imágenes Docker solo en tags/releases
- Sincronización bidireccional entre `main` y `dev`

---

## 📊 Flujo de Trabajo Propuesto

### 1. Estructura de Ramas

```
main (producción)
  ↑
  PR (solo)
  ↑
dev (desarrollo)
  ↑
  PR (solo)
  ↑
feature/*, bugfix/*, hotfix/* (ramas de trabajo)
```

**Reglas:**
- ✅ `main`: Solo PRs, protegida, requiere aprobación
- ✅ `dev`: Solo PRs, protegida, rama principal de desarrollo
- ✅ `feature/*`, `bugfix/*`, `hotfix/*`: Ramas de trabajo, push directo permitido

---

## 🚀 Flujo de Desarrollo Normal

### Paso 1: Crear Feature Branch
```bash
git checkout dev
git pull origin dev
git checkout -b feature/nueva-funcionalidad
```

### Paso 2: Desarrollar y Pushear
```bash
git add .
git commit -m "feat: implementar nueva funcionalidad"
git push origin feature/nueva-funcionalidad
```
**Resultado:**
- ✅ Push directo permitido en `feature/*`
- ❌ No se ejecuta CI/CD (solo en PRs)

### Paso 3: Crear PR a `dev`
```bash
gh pr create --base dev --title "feat: nueva funcionalidad"
```

**CI/CD que se ejecuta en PR → dev:**
- ✅ Tests unitarios
- ✅ Tests de integración
- ✅ Cobertura de código
- ✅ Linter
- ✅ Build/compilación
- ✅ Docker build (test, no push)
- ❌ NO genera imagen Docker
- ❌ NO crea tags

**Requisitos para merge:**
- ✅ CI Pipeline: SUCCESS
- ✅ Test Coverage: SUCCESS
- ✅ Code Review aprobado (opcional pero recomendado)

### Paso 4: Merge PR a `dev`
```bash
# Desde GitHub UI o CLI
gh pr merge <PR_NUMBER> --squash
```

**Resultado:**
- ✅ Feature integrada en `dev`
- ❌ NO se genera imagen Docker
- ✅ Se puede borrar la rama `feature/*`

---

## 🏷️ Flujo de Release (dev → main)

### Paso 1: Crear PR de dev a main
```bash
# Cuando dev esté estable y listo para release
gh pr create --base main --head dev --title "Release v1.2.0"
```

**CI/CD que se ejecuta en PR → main:**
- ✅ Tests completos (unitarios + integración)
- ✅ Tests de cobertura
- ✅ Linter
- ✅ Build/compilación
- ✅ Docker build (test, no push)
- ✅ Verificación de seguridad (opcional)
- ❌ NO genera imagen Docker aún
- ❌ NO crea tags aún

**Requisitos para merge:**
- ✅ Todos los tests passing
- ✅ Code review aprobado
- ✅ Build exitoso

### Paso 2: Merge PR a `main`

**IMPORTANTE:** Al hacer merge, el workflow debe:

1. **Auto-incrementar versión:**
   - Lee el último tag (ej: `v1.1.5`)
   - Incrementa según tipo de cambios:
     - `feat:` → Minor (v1.2.0)
     - `fix:` → Patch (v1.1.6)
     - `BREAKING CHANGE:` → Major (v2.0.0)
   - Crea commit de bump: `chore: bump version to v1.2.0`
   - Pushea a `main`

2. **Crear tag automáticamente:**
   - Crea tag `v1.2.0` en el commit de merge
   - Pushea tag a origin

3. **Trigger Release Workflow:**
   - El tag `v1.2.0` dispara workflow `release.yml`

**CI/CD que se ejecuta en push de tag:**
- ✅ Tests de validación final
- ✅ Build de Docker con versión semántica
- ✅ Push de imagen con múltiples tags:
  - `ghcr.io/edugogroup/edugo-worker:v1.2.0`
  - `ghcr.io/edugogroup/edugo-worker:1.2.0`
  - `ghcr.io/edugogroup/edugo-worker:1.2`
  - `ghcr.io/edugogroup/edugo-worker:1`
  - `ghcr.io/edugogroup/edugo-worker:latest`
- ✅ Creación de GitHub Release con notas
- ✅ Generación de CHANGELOG automático

### Paso 3: Sincronizar main → dev

**Después del release, sincronizar cambios a dev:**

**OPCIÓN A - Workflow Automático (RECOMENDADO):**
```yaml
# En workflow post-merge de main
- Crear PR automático: main → dev
- Título: "chore: sync main to dev after release v1.2.0"
- Auto-merge si no hay conflictos
```

**OPCIÓN B - Manual (ALTERNATIVA):**
```bash
git checkout dev
git merge main --ff-only
git push origin dev
```

---

## 🚨 Flujo de Hotfix (urgente en producción)

### Caso: Bug crítico en producción (main)

```bash
# 1. Crear rama de hotfix desde main
git checkout main
git pull origin main
git checkout -b hotfix/critical-bug

# 2. Hacer fix
git commit -m "fix: resolver bug crítico en producción"
git push origin hotfix/critical-bug

# 3. PR a main (proceso acelerado)
gh pr create --base main --title "hotfix: bug crítico"

# 4. Merge a main
# → Ejecuta auto-versionado (v1.2.1)
# → Crea tag automáticamente
# → Genera imagen Docker
# → Crea release

# 5. Sincronizar a dev
# → Workflow automático crea PR: main → dev
# → Auto-merge
```

---

## 🔐 Protecciones de Rama Requeridas

### Configuración en GitHub

**Para `main`:**
```
Settings > Branches > Branch protection rules > main

☑ Require a pull request before merging
  ☑ Require approvals (1 mínimo)
  ☑ Dismiss stale pull request approvals when new commits are pushed
☑ Require status checks to pass before merging
  ☑ Require branches to be up to date before merging
  Status checks required:
    - CI Pipeline
    - Tests and Validations
    - Docker Build Test
☑ Require conversation resolution before merging
☑ Do not allow bypassing the above settings
☐ Allow force pushes (DESHABILITADO)
☐ Allow deletions (DESHABILITADO)
```

**Para `dev`:**
```
Settings > Branches > Branch protection rules > dev

☑ Require a pull request before merging
  ☐ Require approvals (opcional, 0 o 1)
☑ Require status checks to pass before merging
  ☑ Require branches to be up to date before merging
  Status checks required:
    - CI Pipeline
    - Test Coverage
☑ Require conversation resolution before merging
☐ Allow force pushes (DESHABILITADO)
☐ Allow deletions (DESHABILITADO)
```

---

## 🤖 Workflows Necesarios

### 1. `ci.yml` - CI Pipeline ✅ YA EXISTE
**Trigger:** PR a `main` o `dev`, push a `main` (red de seguridad)
**Jobs:** Tests, linter, build, docker test

### 2. `test.yml` - Tests with Coverage ✅ YA EXISTE
**Trigger:** PR a `main` o `dev`, manual
**Jobs:** Tests con servicios, cobertura

### 3. `pr-to-dev.yml` - 🆕 NUEVO
**Trigger:** PR a `dev`
**Jobs:**
- Tests rápidos
- Linter
- Build
- Docker build (test only)

### 4. `pr-to-main.yml` - 🆕 NUEVO
**Trigger:** PR a `main`
**Jobs:**
- Tests completos
- Cobertura obligatoria
- Security scan
- Docker build (test only)
- Validación de versión en package.json o similar

### 5. `auto-version-and-release.yml` - 🆕 NUEVO (MÁS IMPORTANTE)
**Trigger:** Merge de PR a `main`
**Jobs:**
1. Analizar commits desde último tag
2. Determinar bump (major/minor/patch)
3. Actualizar versión en archivos
4. Crear commit de bump
5. Crear tag automáticamente
6. Pushear tag (esto dispara release.yml)

### 6. `release.yml` - ✅ YA EXISTE (MEJORAR)
**Trigger:** Push de tag `v*`
**Jobs:**
- Validación completa
- Build Docker con versión semántica
- Push a GHCR con múltiples tags
- Crear GitHub Release

### 7. `sync-main-to-dev.yml` - 🆕 NUEVO
**Trigger:** Push a `main` (después de merge)
**Jobs:**
1. Crear PR: `main` → `dev`
2. Título: "chore: sync main v{version} to dev"
3. Auto-merge si no hay conflictos
4. Notificar si hay conflictos

### 8. `docker-only.yml` - ✅ YA EXISTE
**Trigger:** Manual (emergencias)
**Jobs:** Solo build y push Docker

---

## 📝 Archivos de Configuración Necesarios

### 1. `.github/version.txt` - 🆕 NUEVO
```
1.0.0
```
Archivo que mantiene la versión actual del proyecto.

### 2. `.github/PULL_REQUEST_TEMPLATE.md` - 🆕 NUEVO
Template para PRs con checklist de validación.

### 3. `.github/auto-version.config.json` - 🆕 NUEVO (opcional)
```json
{
  "versionFile": ".github/version.txt",
  "changelogFile": "CHANGELOG.md",
  "defaultBump": "patch",
  "bumpRules": {
    "breaking": "major",
    "feat": "minor",
    "fix": "patch"
  }
}
```

---

## 🔄 Matriz de CI/CD por Evento

| Evento | CI Tests | Coverage | Linter | Build | Docker Build | Docker Push | Tag/Version | Release |
|--------|----------|----------|--------|-------|--------------|-------------|-------------|---------|
| Push a `feature/*` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| PR → `dev` | ✅ | ✅ | ✅ | ✅ | ✅ test | ❌ | ❌ | ❌ |
| Merge → `dev` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| PR → `main` | ✅✅ | ✅✅ | ✅ | ✅ | ✅ test | ❌ | ❌ | ❌ |
| Merge → `main` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ auto | ❌ |
| Push tag `v*` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| Manual (emergencia) | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |

---

## 📋 Diagrama de Flujo Completo

```
┌─────────────────────────────────────────────────────────────┐
│ DESARROLLO NORMAL                                           │
└─────────────────────────────────────────────────────────────┘

Developer crea feature/nueva-funcionalidad
         ↓
    Push directo OK (sin CI/CD)
         ↓
    Crea PR → dev
         ↓
    CI/CD se ejecuta:
    - Tests ✓
    - Coverage ✓
    - Linter ✓
    - Build ✓
    - Docker build test ✓
         ↓
    Aprobación + Merge → dev
         ↓
    Feature en dev (sin Docker image)


┌─────────────────────────────────────────────────────────────┐
│ RELEASE A PRODUCCIÓN                                        │
└─────────────────────────────────────────────────────────────┘

Developer crea PR: dev → main
         ↓
    CI/CD COMPLETO se ejecuta:
    - Tests completos ✓
    - Coverage ✓
    - Linter ✓
    - Build ✓
    - Docker build test ✓
    - Security scan ✓ (opcional)
         ↓
    Aprobación + Merge → main
         ↓
    ┌──────────────────────────────────────┐
    │ WORKFLOW AUTO-VERSION se ejecuta:    │
    ├──────────────────────────────────────┤
    │ 1. Lee commits desde último tag      │
    │ 2. Determina bump (major/minor/patch)│
    │ 3. Nueva versión: v1.2.0             │
    │ 4. Actualiza .github/version.txt     │
    │ 5. Actualiza CHANGELOG.md            │
    │ 6. Commit: "chore: bump v1.2.0"      │
    │ 7. Crea tag v1.2.0                   │
    │ 8. Push tag                          │
    └──────────────────────────────────────┘
         ↓
    Tag v1.2.0 creado
         ↓
    ┌──────────────────────────────────────┐
    │ WORKFLOW RELEASE se ejecuta:         │
    ├──────────────────────────────────────┤
    │ 1. Tests de validación               │
    │ 2. Build Docker                      │
    │ 3. Push a GHCR con tags:             │
    │    - v1.2.0                          │
    │    - 1.2.0                           │
    │    - 1.2                             │
    │    - 1                               │
    │    - latest                          │
    │ 4. Crear GitHub Release              │
    │ 5. Adjuntar CHANGELOG                │
    └──────────────────────────────────────┘
         ↓
    Imagen Docker en producción ✓
         ↓
    ┌──────────────────────────────────────┐
    │ WORKFLOW SYNC se ejecuta:            │
    ├──────────────────────────────────────┤
    │ 1. Detecta cambios en main           │
    │ 2. Crea PR: main → dev               │
    │ 3. Título: "chore: sync v1.2.0"      │
    │ 4. Auto-merge si no hay conflictos   │
    │ 5. Notifica si hay conflictos        │
    └──────────────────────────────────────┘
         ↓
    dev sincronizado con main ✓


┌─────────────────────────────────────────────────────────────┐
│ HOTFIX URGENTE                                              │
└─────────────────────────────────────────────────────────────┘

Bug crítico en producción
         ↓
    git checkout -b hotfix/critical-fix main
         ↓
    Fix + commit + push
         ↓
    PR → main (proceso acelerado)
         ↓
    CI/CD completo
         ↓
    Merge → main
         ↓
    Auto-version (v1.2.1 patch)
         ↓
    Tag → Release → Docker image
         ↓
    Sync main → dev automático
         ↓
    Hotfix en prod y dev ✓
```

---

## 🛠️ Componentes Técnicos Necesarios

### A. Workflows Nuevos a Crear

#### 1. `auto-version.yml` ⭐ CRÍTICO
```yaml
name: Auto Version and Tag

on:
  pull_request:
    types: [closed]
    branches: [main]

jobs:
  auto-version:
    if: github.event.pull_request.merged == true
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - Checkout código
      - Obtener último tag
      - Analizar commits (feat/fix/breaking)
      - Calcular nueva versión
      - Actualizar version.txt
      - Actualizar CHANGELOG.md
      - Commit de bump
      - Crear y pushear tag
```

#### 2. `sync-main-to-dev.yml` ⭐ CRÍTICO
```yaml
name: Sync Main to Dev

on:
  push:
    branches: [main]
    # Solo cuando hay nuevo tag

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write

    steps:
      - Checkout
      - Obtener versión actual
      - Crear PR: main → dev
      - Auto-merge si es fast-forward
      - Comment si hay conflictos
```

#### 3. `pr-checks.yml` - Validaciones por rama
```yaml
name: PR Checks

on:
  pull_request:
    branches: [main, dev]

jobs:
  determine-target:
    # Detectar rama target (main o dev)

  checks-for-dev:
    if: target == 'dev'
    # Tests básicos + build

  checks-for-main:
    if: target == 'main'
    # Tests completos + coverage + security
```

### B. Scripts de Soporte

#### 1. `.github/scripts/bump-version.sh`
```bash
#!/bin/bash
# Calcula nueva versión basado en commits
# Actualiza version.txt
# Actualiza CHANGELOG.md
```

#### 2. `.github/scripts/generate-changelog.sh`
```bash
#!/bin/bash
# Genera CHANGELOG desde último tag
# Agrupa por tipo (feat, fix, breaking)
```

### C. Archivos de Configuración

#### 1. `.github/version.txt`
```
1.0.0
```

#### 2. `CHANGELOG.md`
```markdown
# Changelog

## [Unreleased]

## [1.0.0] - 2025-10-31
### Added
- Sistema CI/CD completo
...
```

---

## ⚠️ Problemas y Soluciones

### Problema 1: "¿Cómo actualizar dev después de merge a main?"

**Solución:** Workflow `sync-main-to-dev.yml`
- Se ejecuta automáticamente después de push a main
- Crea PR de main → dev
- Auto-merge si no hay conflictos
- Si hay conflictos, crea PR y notifica para resolución manual

### Problema 2: "¿Qué pasa si alguien hace push directo a main o dev?"

**Solución:** Protección de ramas en GitHub
- Configurar "Require pull request before merging"
- GitHub bloqueará push directo
- Solo permite merge via PR aprobado

### Problema 3: "¿Cómo se determina el bump de versión?"

**Solución:** Conventional Commits
- Analizar commits en el PR
- `feat:` → minor bump
- `fix:` → patch bump
- `BREAKING CHANGE:` → major bump
- Default: patch

### Problema 4: "¿Qué pasa si el auto-version falla?"

**Solución:** Fallback manual
- Workflow notifica del error
- Developer crea tag manualmente
- Tag manual dispara release normal

### Problema 5: "¿Imagen Docker en cada commit a dev?"

**Solución:** NO
- En dev solo se valida (test build)
- Imagen solo se genera en tags (releases)
- Para testing en dev, usar workflow manual `docker-only.yml`

---

## 🎯 Diferencias vs Estado Actual

### Estado Actual ❌
- Sin rama `dev`
- Push directo a `main` permitido
- Docker image en cada push a main
- Sin auto-versionado
- Sin sincronización main ↔ dev

### Estado Propuesto ✅
- Rama `dev` como principal de desarrollo
- `main` y `dev` protegidas, solo PR
- Docker image SOLO en tags/releases
- Auto-versionado en merge a main
- Sincronización automática main → dev

---

## 📦 Implementación por Fases

### Fase 1: Preparación (10 min)
1. Crear rama `dev` desde `main`
2. Crear `.github/version.txt`
3. Crear `CHANGELOG.md`
4. Configurar protecciones de rama en GitHub

### Fase 2: Workflows Core (20 min)
1. Crear `auto-version.yml`
2. Crear `sync-main-to-dev.yml`
3. Modificar `ci.yml` para detectar rama target
4. Modificar `release.yml` para mejorar output

### Fase 3: Scripts de Soporte (15 min)
1. Crear `bump-version.sh`
2. Crear `generate-changelog.sh`
3. Hacer ejecutables

### Fase 4: Validación (15 min)
1. Crear feature de prueba
2. PR a dev → validar CI
3. Merge a dev
4. PR dev → main → validar auto-version
5. Validar imagen Docker generada
6. Validar sync main → dev

### Fase 5: Documentación (10 min)
1. Actualizar README.md
2. Crear CONTRIBUTING.md con flujo
3. Actualizar .github/workflows/README.md

---

## 🎓 Convenciones de Commits (Requeridas)

Para que el auto-versionado funcione:

```
feat: nueva funcionalidad → MINOR bump
fix: corrección de bug → PATCH bump
perf: mejora de performance → PATCH bump
docs: cambios en documentación → PATCH bump
style: formato de código → PATCH bump
refactor: refactorización → PATCH bump
test: agregar tests → PATCH bump
chore: cambios de build/tools → PATCH bump

BREAKING CHANGE: en el cuerpo → MAJOR bump
```

**Ejemplos:**
```bash
git commit -m "feat: agregar autenticación OAuth"
# → v1.1.0 → v1.2.0 (minor)

git commit -m "fix: corregir validación de email"
# → v1.1.0 → v1.1.1 (patch)

git commit -m "feat: nueva API

BREAKING CHANGE: API v1 deprecada"
# → v1.1.0 → v2.0.0 (major)
```

---

## 🚦 Estados de Rama

### `main` (producción)
- Solo código estable
- Solo via PR aprobado
- Cada merge → auto-version → release
- Siempre buildeable
- Siempre con imagen Docker

### `dev` (desarrollo)
- Código en desarrollo
- Solo via PR
- Puede tener features incompletas
- No genera imágenes Docker
- Se sincroniza desde main después de releases

### `feature/*`, `bugfix/*`, `hotfix/*`
- Ramas de trabajo
- Push directo permitido
- No ejecutan CI/CD hasta PR
- Se borran después de merge

---

## ✅ Checklist de Validación

Después de implementar, validar:

- [ ] Push directo a `main` está bloqueado
- [ ] Push directo a `dev` está bloqueado
- [ ] PR a `dev` ejecuta CI básico
- [ ] PR a `main` ejecuta CI completo
- [ ] Merge a `main` crea tag automáticamente
- [ ] Tag dispara release workflow
- [ ] Release workflow genera Docker image
- [ ] Imagen tiene múltiples tags semánticos
- [ ] GitHub Release se crea automáticamente
- [ ] Main se sincroniza a dev automáticamente
- [ ] CHANGELOG se genera automáticamente

---

## 🎯 Próximos Pasos

1. **Revisar este plan contigo**
2. **Crear rama `dev`**
3. **Implementar workflows nuevos**
4. **Configurar protecciones de rama**
5. **Hacer prueba end-to-end**
6. **Replicar en proyectos hermanos**

---

## 📚 Proyectos para Aplicar Este Flujo

1. ✅ edugo-worker (este)
2. ⏳ edugo-api-administracion
3. ⏳ edugo-api-mobile
4. ⏳ edugo-shared (requiere adaptación para mono-repo)

---

**Generado por:** Claude Code
**Fecha:** 2025-10-31
**Versión del plan:** 1.0
