# 🚀 Plan de Ejecución - GitFlow para 3 Proyectos

**Fecha:** 2025-10-31
**Proyectos:** edugo-worker, edugo-api-mobile, edugo-api-administracion
**Objetivo:** Implementar GitFlow profesional con auto-versionado y CI/CD optimizado
**Versión inicial para todos:** v1.0.0

---

## 📊 Análisis de Proyectos

### Proyecto 1: edugo-worker
**Ubicación:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker`
**Tipo:** Worker/Background processor
**Tecnología:** Go 1.25.3 + RabbitMQ + MongoDB + PostgreSQL
**Estado actual:**
- ✅ edugo-shared v2.0.5 modular (common, logger, database/postgres)
- ✅ Workflows: ci.yml, test.yml, build-and-push.yml, release.yml, docker-only.yml
- ✅ Docker image funcionando en GHCR
- ✅ .gitignore completo
- ❌ Sin rama dev
- ❌ Sin protecciones de rama
- ❌ Sin auto-versionado

**Dockerfile:** ✅ Existe
**Tests:** ✅ Básicos implementados
**Complejidad CI:** Media (sin servicios pesados)

---

### Proyecto 2: edugo-api-mobile
**Ubicación:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-api-mobile`
**Tipo:** REST API para mobile
**Tecnología:** Go 1.25.3 + Gin + PostgreSQL + MongoDB + Swagger
**Estado actual:**
- ✅ edugo-shared v2.0.5 modular (auth, common, logger)
- ✅ Workflows: ci.yml, test.yml, build-and-push.yml, release.yml
- ✅ .gitignore parcial
- ❌ Sin rama dev
- ❌ Sin protecciones de rama
- ❌ Sin auto-versionado

**Dockerfile:** ✅ Existe
**Tests:** ✅ Con testcontainers (postgres, mongodb, rabbitmq)
**Complejidad CI:** Alta (servicios de infra + swagger)

---

### Proyecto 3: edugo-api-administracion
**Ubicación:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-api-administracion`
**Tipo:** REST API para administración
**Tecnología:** Go 1.25.3 + Gin + PostgreSQL + MongoDB + Swagger
**Estado actual:**
- ✅ edugo-shared v2.0.5 modular (common, logger)
- ✅ Workflows: ci.yml, test.yml, build-and-push.yml, release.yml
- ✅ .gitignore con .DS_Store
- ❌ Sin rama dev
- ❌ Sin protecciones de rama
- ❌ Sin auto-versionado

**Dockerfile:** ✅ Existe
**Tests:** ✅ Con testcontainers (postgres, mongodb)
**Complejidad CI:** Alta (servicios de infra + swagger)

---

## 🎯 Resumen Comparativo

| Aspecto | worker | api-mobile | api-admin |
|---------|--------|------------|-----------|
| edugo-shared v2.0.5 | ✅ | ✅ | ✅ |
| Workflows CI/CD | ✅ (5) | ✅ (4) | ✅ (4) |
| .gitignore | ✅ | ⚠️ parcial | ✅ |
| Docker funcional | ✅ | ❓ | ❓ |
| Rama dev | ❌ | ❌ | ❌ |
| Auto-version | ❌ | ❌ | ❌ |
| Swagger docs | ❌ | ✅ | ✅ |

---

## 🛠️ Tareas por Proyecto

### Comunes a los 3 proyectos:

1. **Crear rama `dev`** desde main
2. **Crear `.github/version.txt`** con `1.0.0`
3. **Crear `CHANGELOG.md`** inicial
4. **Crear workflow `auto-version.yml`**
5. **Crear workflow `sync-main-to-dev.yml`**
6. **Crear scripts:**
   - `.github/scripts/bump-version.sh`
   - `.github/scripts/generate-changelog.sh`
7. **Modificar `ci.yml`** para detectar rama target
8. **Modificar `release.yml`** para validar versión
9. **Configurar protecciones de rama** en GitHub
10. **Crear tag v1.0.0** inicial
11. **Validar generación de imagen Docker**
12. **Validar sync main → dev**

### Específicas por proyecto:

**edugo-worker:**
- Mejorar `.gitignore` (ya está completo)
- Mantener `docker-only.yml`

**edugo-api-mobile:**
- Mejorar `.gitignore` (agregar más exclusiones)
- Mantener generación de Swagger docs en CI

**edugo-api-administracion:**
- Mantener generación de Swagger docs en CI

---

## 📅 Plan de Ejecución Secuencial

### PROYECTO 1: edugo-worker (1.5 horas estimadas)

**Fase 1: Preparación (15 min)**
1. Crear rama dev
2. Crear version.txt (1.0.0)
3. Crear CHANGELOG.md
4. Crear scripts de bump y changelog
5. Commit y push preparación

**Fase 2: Workflows (30 min)**
1. Crear auto-version.yml
2. Crear sync-main-to-dev.yml
3. Modificar ci.yml
4. Modificar release.yml
5. Commit y push workflows

**Fase 3: Configuración GitHub (10 min)**
1. Configurar protección de main
2. Configurar protección de dev
3. Verificar permisos GHCR

**Fase 4: Validación End-to-End (30 min)**
1. Crear feature/test-gitflow
2. PR a dev → validar CI
3. Merge a dev
4. PR dev → main
5. Merge → validar auto-version
6. Validar tag v1.0.0 creado
7. Validar imagen Docker generada
8. Validar GitHub Release
9. Validar sync main → dev

**Fase 5: Ajustes (15 min)**
- Corregir errores encontrados
- Optimizar workflows
- Documentar aprendizajes

---

### PROYECTO 2: edugo-api-mobile (1 hora estimada)

**Aplicar mismas fases que worker pero:**
- Copiar workflows ya validados
- Adaptar nombres en scripts
- Validación más rápida (aprendimos del primero)

**Tiempo estimado:** 1 hora

---

### PROYECTO 3: edugo-api-administracion (1 hora estimada)

**Aplicar mismo proceso que api-mobile**

**Tiempo estimado:** 1 hora

---

## 🔐 Permisos Requeridos

### Permisos que necesito tu autorización ANTES de ejecutar:

#### 1. Git Operations
- ✅ Crear ramas (dev, feature/*)
- ✅ Crear commits
- ✅ Push a origin
- ✅ Crear tags
- ✅ Push tags
- ✅ Crear y merge PRs (via gh CLI)

#### 2. GitHub Configuration
- ❓ **Configurar branch protections** (requiere permisos de admin)
  - ¿Tienes permisos de admin en los 3 repos?
  - Si no, te daré los comandos para que lo hagas manual

#### 3. GHCR (GitHub Container Registry)
- ✅ Push de imágenes Docker (ya configurado en worker)
- ❓ ¿Los otros 2 proyectos tienen permisos GHCR configurados?
  - Si no, necesitaremos configurarlos

#### 4. Workflow Executions
- ✅ Ejecutar workflows via gh CLI
- ✅ Monitorear workflows
- ✅ Re-ejecutar workflows si fallan

---

## ⚠️ Decisiones Necesarias ANTES de Empezar

### 1. Auto-versionado
**¿Cómo determinar el bump?**

**OPCIÓN A (RECOMENDADA):** Conventional Commits
- Analiza mensajes: feat→minor, fix→patch, BREAKING→major
- Totalmente automático
- Requiere disciplina en commits

**OPCIÓN B:** Basado en etiquetas en PR
- En el título del PR: `[major]`, `[minor]`, `[patch]`
- Manual pero explícito
- Más control

**OPCIÓN C:** Siempre patch
- Cada merge a main → v1.0.0 → v1.0.1 → v1.0.2
- Simple pero menos semántico

**¿Cuál prefieres?** (Recomiendo A)

---

### 2. Sincronización main → dev

**¿Cómo resolver conflictos?**

**OPCIÓN A (RECOMENDADA):** PR automático + notificación
- Crea PR automático
- Si no hay conflictos → auto-merge
- Si hay conflictos → notifica y espera resolución manual

**OPCIÓN B:** Merge directo con --no-ff
- Siempre crea merge commit
- Si hay conflictos → falla y notifica

**¿Cuál prefieres?** (Recomiendo A)

---

### 3. Protecciones de Rama

**¿Requiere aprobaciones de PR?**

**Para `main`:**
- ☑ Require 1 approval ← ¿SÍ o NO?
- ☑ Require CI passing
- ☑ Require conversation resolution

**Para `dev`:**
- ☐ Require 0 approvals (más ágil) ← ¿SÍ o NO?
- ☑ Require CI passing

**¿Qué prefieres?**

---

### 4. CHANGELOG Automático

**¿Cómo generar el CHANGELOG?**

**OPCIÓN A:** Desde commits
```
## [1.0.0] - 2025-10-31

### Features
- feat: agregar autenticación OAuth
- feat: implementar cache Redis

### Fixes
- fix: corregir validación de email
```

**OPCIÓN B:** Desde PRs mergeados
```
## [1.0.0] - 2025-10-31

- #123: Agregar autenticación OAuth
- #124: Implementar cache Redis
- #125: Corregir validación email
```

**¿Cuál prefieres?** (Recomiendo A)

---

## 🎬 Orden de Ejecución Propuesto

```
1. edugo-worker (primero)
   ├─ Implementar completo
   ├─ Validar funcionamiento
   ├─ Documentar problemas/soluciones
   └─ Tiempo: 1.5 horas

2. edugo-api-mobile (segundo)
   ├─ Replicar workflows validados
   ├─ Adaptar a sus necesidades
   ├─ Validar
   └─ Tiempo: 1 hora

3. edugo-api-administracion (tercero)
   ├─ Replicar workflows validados
   ├─ Adaptar a sus necesidades
   ├─ Validar
   └─ Tiempo: 1 hora

TOTAL ESTIMADO: 3.5 horas
```

---

## 📋 Checklist Pre-Ejecución

Antes de empezar, confirma:

### Permisos GitHub:
- [ ] ¿Tienes permisos de **admin** en los 3 repositorios?
- [ ] ¿Tienes permisos de **admin** en la organización EduGoGroup?
- [ ] ¿Los paquetes GHCR de api-mobile y api-admin existen?
- [ ] ¿Los repos tienen "Read and write permissions" en Actions?

### Decisiones de Configuración:
- [ ] Auto-versionado: ¿Opción A, B o C?
- [ ] Sync main→dev: ¿Opción A o B?
- [ ] Aprobaciones PR a main: ¿SÍ (1) o NO (0)?
- [ ] Aprobaciones PR a dev: ¿SÍ o NO?
- [ ] CHANGELOG: ¿Opción A o B?

### Validaciones:
- [ ] ¿Los 3 proyectos compilan localmente?
- [ ] ¿Los tests pasan en los 3 proyectos?
- [ ] ¿Tienes acceso a GitHub CLI (`gh`) configurado?

---

## 🎯 Resultado Esperado Final

Al terminar, los 3 proyectos tendrán:

```
✅ Rama dev creada y protegida
✅ Rama main protegida (solo PRs)
✅ Workflow auto-version.yml funcionando
✅ Workflow sync-main-to-dev.yml funcionando
✅ Tag v1.0.0 creado en los 3 proyectos
✅ Imagen Docker v1.0.0 en GHCR:
   - ghcr.io/edugogroup/edugo-worker:1.0.0
   - ghcr.io/edugogroup/edugo-api-mobile:1.0.0
   - ghcr.io/edugogroup/edugo-api-administracion:1.0.0
✅ GitHub Release v1.0.0 en los 3 repos
✅ CHANGELOG.md generado
✅ CI/CD optimizado por contexto
✅ Flujo GitFlow profesional documentado
```

---

## 📦 Artefactos Generados

Para cada proyecto se generará:

```
.github/
├── version.txt (1.0.0)
├── workflows/
│   ├── ci.yml (modificado)
│   ├── test.yml (existente)
│   ├── build-and-push.yml (modificado)
│   ├── release.yml (modificado)
│   ├── docker-only.yml (nuevo en worker, opcional en APIs)
│   ├── auto-version.yml (nuevo) ⭐
│   └── sync-main-to-dev.yml (nuevo) ⭐
└── scripts/
    ├── bump-version.sh (nuevo)
    └── generate-changelog.sh (nuevo)

CHANGELOG.md (nuevo)
GITFLOW_PLAN.md (documentación)
```

---

## ⏱️ Estimación de Tiempos por Fase

### Por cada proyecto:

| Fase | Tiempo | Descripción |
|------|--------|-------------|
| Preparación | 10 min | Crear archivos base, scripts |
| Workflows | 20 min | Crear/modificar workflows |
| Config GitHub | 5 min | Protecciones (si tienes permisos) |
| Validación | 20 min | Feature → dev → main → release |
| Ajustes | 10 min | Correcciones si hay errores |
| **Total por proyecto** | **65 min** | ~1 hora |

**Con aprendizajes:**
- Proyecto 1 (worker): 1.5 horas (primero, más lento)
- Proyecto 2 (mobile): 1 hora (copiamos del 1)
- Proyecto 3 (admin): 1 hora (copiamos del 1)

**TOTAL: 3.5 horas**

---

## 🚨 Riesgos y Mitigaciones

### Riesgo 1: Tests fallan en CI
**Mitigación:**
- Ejecutar tests localmente ANTES de implementar
- Si fallan, corregir primero

### Riesgo 2: Docker build falla en CI
**Mitigación:**
- Validar Dockerfile local antes
- Verificar GITHUB_TOKEN en build args

### Riesgo 3: Sin permisos de admin para branch protection
**Mitigación:**
- Te daré instrucciones paso a paso
- Tú ejecutas manualmente en GitHub UI

### Riesgo 4: Conflictos en sync main → dev
**Mitigación:**
- Primera ejecución no debería tener conflictos
- Workflow notificará si ocurre

### Riesgo 5: Auto-version genera versión incorrecta
**Mitigación:**
- Primera versión es manual (v1.0.0)
- Validamos lógica con segundo release

---

## 📝 Orden de Operaciones Detallado

### PASO 0: Pre-validación (15 min)

```bash
# En cada proyecto, verificar:
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker
go build ./...
go test ./...

cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-api-mobile
go build ./...
go test ./...

cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-api-administracion
go build ./...
go test ./...
```

---

### PASO 1: edugo-worker (1.5 horas)

**1.1 Preparación**
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Crear rama dev
git checkout -b dev
git push -u origin dev

# Crear archivos base
echo "1.0.0" > .github/version.txt
# Crear CHANGELOG.md inicial
# Crear scripts

git add .
git commit -m "chore: preparar estructura GitFlow"
git push origin dev
```

**1.2 Workflows**
- Crear auto-version.yml
- Crear sync-main-to-dev.yml
- Modificar workflows existentes
- Commit y push

**1.3 Protecciones** (Manual en GitHub UI)
- Configurar main: require PR + CI
- Configurar dev: require PR + CI

**1.4 Tag Inicial**
```bash
git checkout main
git pull origin main
git tag -a v1.0.0 -m "Release v1.0.0 - GitFlow implementado"
git push origin v1.0.0
```

**1.5 Validación**
- Monitorear release workflow
- Validar imagen Docker en GHCR
- Validar GitHub Release
- Validar sync a dev

---

### PASO 2: edugo-api-mobile (1 hora)

**Copiar archivos validados de worker:**
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-api-mobile

# Copiar workflows
cp ../edugo-worker/.github/workflows/auto-version.yml .github/workflows/
cp ../edugo-worker/.github/workflows/sync-main-to-dev.yml .github/workflows/

# Copiar scripts
mkdir -p .github/scripts
cp ../edugo-worker/.github/scripts/* .github/scripts/

# Copiar archivos base
cp ../edugo-worker/.github/version.txt .github/
cp ../edugo-worker/CHANGELOG.md .

# Adaptar nombres en archivos
# Crear dev, configurar, tag v1.0.0, validar
```

---

### PASO 3: edugo-api-administracion (1 hora)

**Mismo proceso que api-mobile**

---

## 🎨 Optimizaciones de CI/CD por Proyecto

### edugo-worker (ligero)
```yaml
# test.yml - Sin muchos servicios pesados
services:
  postgres: opcional
  mongodb: opcional
  rabbitmq: requerido

# Tiempo esperado: ~2 min
```

### edugo-api-mobile (medio)
```yaml
# test.yml - APIs necesitan DBs
services:
  postgres: requerido
  mongodb: requerido
  rabbitmq: opcional

# Tiempo esperado: ~3 min
```

### edugo-api-administracion (medio)
```yaml
# test.yml - APIs necesitan DBs
services:
  postgres: requerido
  mongodb: requerido

# Swagger generation: ~30s adicionales
# Tiempo esperado: ~3 min
```

---

## 🎯 Matriz de CI/CD Optimizada

| Trigger | worker | api-mobile | api-admin | Docker | Release |
|---------|--------|------------|-----------|--------|---------|
| PR → dev | Tests básicos (~2min) | Tests + Swagger (~3min) | Tests + Swagger (~3min) | Build test | ❌ |
| PR → main | Tests completos (~3min) | Tests + Coverage (~4min) | Tests + Coverage (~4min) | Build test | ❌ |
| Merge main | Auto-version (~30s) | Auto-version (~30s) | Auto-version (~30s) | ❌ | Tag |
| Push tag v* | Validate + Docker (~2min) | Validate + Docker (~2min) | Validate + Docker (~2min) | ✅ Push | ✅ |

**Tiempos totales por flujo completo:**
- Feature → dev: 2-4 min
- Dev → main → release: 6-8 min
- **Total: ~10 min por release** (mucho más rápido que monolito)

---

## 📊 Estado Final de los 3 Proyectos

```
edugo-worker v1.0.0
├── Rama: main (protegida) ✅
├── Rama: dev (protegida) ✅
├── GitFlow: implementado ✅
├── Auto-version: funcionando ✅
├── Docker: ghcr.io/edugogroup/edugo-worker:1.0.0 ✅
├── Release: v1.0.0 publicado ✅
└── Sync: main ↔ dev automático ✅

edugo-api-mobile v1.0.0
├── Rama: main (protegida) ✅
├── Rama: dev (protegida) ✅
├── GitFlow: implementado ✅
├── Auto-version: funcionando ✅
├── Docker: ghcr.io/edugogroup/edugo-api-mobile:1.0.0 ✅
├── Release: v1.0.0 publicado ✅
└── Sync: main ↔ dev automático ✅

edugo-api-administracion v1.0.0
├── Rama: main (protegida) ✅
├── Rama: dev (protegida) ✅
├── GitFlow: implementado ✅
├── Auto-version: funcionando ✅
├── Docker: ghcr.io/edugogroup/edugo-api-administracion:1.0.0 ✅
├── Release: v1.0.0 publicado ✅
└── Sync: main ↔ dev automático ✅
```

---

## ✅ Validación Final Cross-Proyecto

Al terminar los 3, validar:

- [ ] 3 ramas `dev` creadas y sincronizadas
- [ ] 3 tags `v1.0.0` creados
- [ ] 3 imágenes Docker en GHCR con tags:
  - `1.0.0`, `1.0`, `1`, `latest`
- [ ] 3 GitHub Releases publicados
- [ ] 3 CHANGELOGs generados
- [ ] Protecciones de rama activas en los 3
- [ ] CI/CD pasando en los 3
- [ ] Documentación actualizada en los 3

---

## 🚀 ¿Listo para Empezar?

**Antes de proceder, por favor responde:**

1. **Decisiones de configuración:**
   - Auto-versionado: ¿A, B o C?
   - Sync main→dev: ¿A o B?
   - Aprobaciones PR a main: ¿1 o 0?
   - Aprobaciones PR a dev: ¿1 o 0?
   - CHANGELOG: ¿A o B?

2. **Permisos:**
   - ¿Tienes admin en los 3 repos? (para branch protection)
   - ¿Los paquetes GHCR de api-mobile y api-admin existen?

3. **Confirmación:**
   - ¿Empezamos con edugo-worker primero?
   - ¿Procedo de forma secuencial (no paralelo)?

**Una vez confirmes, empezaré con edugo-worker inmediatamente.**

---

**Generado por:** Claude Code
**Plan versión:** 2.0 - Actualizado con análisis de 3 proyectos
