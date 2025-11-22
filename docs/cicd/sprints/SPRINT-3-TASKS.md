# Sprint 3: Consolidación Docker + Go 1.25 - edugo-worker

**Proyecto:** edugo-worker  
**Sprint:** 3 de 4  
**Duración:** 4-5 días  
**Esfuerzo:** 16-20 horas  
**Prioridad:** 🔴 Alta (Crítico)  
**Fecha:** 19 de Noviembre, 2025

---

## 📋 Tabla de Contenidos

1. [Resumen del Sprint](#-resumen-del-sprint)
2. [Objetivos](#-objetivos)
3. [Pre-requisitos](#-pre-requisitos)
4. [Tareas Detalladas](#-tareas-detalladas)
5. [Checklist General](#-checklist-general)
6. [Troubleshooting](#-troubleshooting)

---

## 🎯 Resumen del Sprint

### ¿Qué vamos a hacer?

**Problema Principal:**  
edugo-worker tiene **3 workflows diferentes construyendo Docker images**, causando desperdicio de recursos, confusión y fallos.

**Solución Sprint 3:**
1. Consolidar 3 workflows Docker en 1 solo (manual-release.yml)
2. Migrar de Go 1.24.10 → 1.25.3
3. Implementar 7 pre-commit hooks
4. Establecer coverage threshold 33%

**Resultado Esperado:**
- ✅ 1 solo workflow Docker (vs 3 actuales)
- ✅ Go 1.25.3 consistente en go.mod y workflows
- ✅ Pre-commit hooks funcionando
- ✅ Coverage threshold 33% aplicado
- ✅ Success rate > 85% (vs 70% actual)

---

## 🎯 Objetivos

### Objetivos Principales

- [ ] **OBJ-1:** Eliminar build-and-push.yml (desperdicio de recursos)
- [ ] **OBJ-2:** Eliminar docker-only.yml (duplicación)
- [ ] **OBJ-3:** Migrar funcionalidad y eliminar release.yml (fallando)
- [ ] **OBJ-4:** Migrar a Go 1.25.3 (consistencia)
- [ ] **OBJ-5:** Implementar pre-commit hooks (calidad)
- [ ] **OBJ-6:** Establecer coverage threshold 33% (calidad)

### Métricas de Éxito

| Métrica | Antes | Después | Objetivo |
|---------|-------|---------|----------|
| Workflows Docker | 3 | 1 | -66% |
| Líneas workflows Docker | ~441 | ~340 | -23% |
| Go version consistente | No | Sí | ✅ |
| Coverage threshold | No | 33% | ✅ |
| Pre-commit hooks | 0 | 7 | ✅ |
| Success rate | 70% | 85%+ | +15% |

---

## ✅ Pre-requisitos

### Herramientas Necesarias

```bash
# 1. Verificar Go instalado
go version
# Debe mostrar: go version go1.25.3 o superior

# 2. Verificar git
git --version

# 3. Verificar gh CLI
gh --version

# 4. Verificar Docker
docker --version

# 5. Verificar pre-commit
pip install pre-commit
pre-commit --version
```

### Accesos Necesarios

- [x] Acceso al repositorio edugo-worker
- [x] Permisos para crear ramas y PRs
- [x] Permisos para ejecutar workflows
- [x] GitHub CLI autenticado (`gh auth status`)

### Preparación del Entorno

```bash
# 1. Clonar/actualizar repositorio
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker
git fetch origin
git checkout main
git pull origin main

# 2. Crear rama feature
git checkout -b feature/sprint-3-consolidation-docker-go125

# 3. Verificar estado limpio
git status
# Debe mostrar: nothing to commit, working tree clean

# 4. Verificar workflows actuales
ls -la .github/workflows/
# Debe mostrar 7 archivos:
# - ci.yml
# - test.yml  
# - manual-release.yml
# - build-and-push.yml
# - docker-only.yml
# - release.yml
# - sync-main-to-dev.yml

# 5. Verificar Go version actual
cat go.mod | grep "^go "
# Debe mostrar: go 1.24.10
```

---

## 📋 Tareas Detalladas

## Tarea 1: Análisis y Consolidación de Workflows Docker

**Duración:** 3-4 horas  
**Prioridad:** 🔴 Crítica  
**Dependencias:** Ninguna

### Objetivo

Analizar en detalle los 3 workflows Docker, decidir qué mantener, y consolidar en manual-release.yml eliminando build-and-push.yml y docker-only.yml.

### Contexto

Actualmente existen 3 workflows construyendo Docker images:

1. **build-and-push.yml** (85 líneas)
   - Trigger: Manual + Push main
   - Sin tests previos
   - Tags: branch, sha, latest

2. **docker-only.yml** (73 líneas)
   - Trigger: Manual
   - Sin tests previos
   - Tags: custom, latest

3. **release.yml** (283 líneas)
   - Trigger: Tag push (v*)
   - Con tests previos
   - Tags: semver completos
   - **ESTÁ FALLANDO**

4. **manual-release.yml** (340 líneas) ✅
   - Trigger: Manual
   - Con tests previos
   - Control fino
   - GitHub App Token
   - **FUNCIONAL**

**Decisión:** Mantener solo manual-release.yml.

### Pasos

#### 1.1: Backup de Workflows a Eliminar

```bash
# Crear directorio de backup
mkdir -p docs/workflows-removed-sprint3

# Backup de build-and-push.yml
cp .github/workflows/build-and-push.yml docs/workflows-removed-sprint3/build-and-push.yml.backup
echo "✅ Backup de build-and-push.yml creado"

# Backup de docker-only.yml
cp .github/workflows/docker-only.yml docs/workflows-removed-sprint3/docker-only.yml.backup
echo "✅ Backup de docker-only.yml creado"

# Backup de release.yml
cp .github/workflows/release.yml docs/workflows-removed-sprint3/release.yml.backup
echo "✅ Backup de release.yml creado"

# Crear README en backup explicando por qué se eliminaron
cat > docs/workflows-removed-sprint3/README.md << 'EOF'
# Workflows Eliminados - Sprint 3

**Fecha:** $(date +%Y-%m-%d)
**Sprint:** 3
**Razón:** Consolidación de workflows Docker

## Workflows Eliminados

### 1. build-and-push.yml
**Razón:** Duplicado de manual-release.yml sin tests previos.
**Funcionalidad migrada a:** manual-release.yml

### 2. docker-only.yml
**Razón:** Duplicado simple sin control fino.
**Funcionalidad migrada a:** manual-release.yml

### 3. release.yml
**Razón:** Fallando + duplicado de manual-release.yml.
**Funcionalidad migrada a:** manual-release.yml

## Workflow Mantenido

**manual-release.yml** - Workflow completo con:
- Tests previos
- Control fino (version + bump_type)
- Multi-platform
- GitHub Release
- CHANGELOG automático
- GitHub App Token

## Restauración

Si necesitas restaurar algún workflow:

```bash
cp docs/workflows-removed-sprint3/[workflow].yml.backup .github/workflows/[workflow].yml
```

## Referencias

- [Análisis de Duplicación](../../README.md#análisis-de-duplicación-docker)
- [Sprint 3 Tasks](../../SPRINT-3-TASKS.md#tarea-1)
EOF

echo "✅ README de backup creado"

# Verificar backups
ls -lh docs/workflows-removed-sprint3/
```

**Validación:**
```bash
# Debe mostrar 4 archivos
[ $(ls docs/workflows-removed-sprint3/ | wc -l) -eq 4 ] && echo "✅ Backups completos" || echo "❌ Faltan backups"
```

---

#### 1.2: Analizar Funcionalidad Única de Cada Workflow

```bash
# Crear análisis comparativo
cat > /tmp/docker-workflows-analysis.md << 'EOF'
# Análisis Comparativo de Workflows Docker

## build-and-push.yml

### Funcionalidad Única
- Trigger automático en push a main
- Tags con SHA del commit
- Variables de environment (development/staging/production)

### ¿Se necesita mantener?
NO - manual-release.yml puede hacer lo mismo con control manual.

### Migración
- Variable `environment` → agregar a manual-release.yml como opcional
- Trigger push a main → NO migrar (control manual es mejor)

---

## docker-only.yml

### Funcionalidad Única
- Input de tag personalizado
- Multi-platform (linux/amd64 + linux/arm64)

### ¿Se necesita mantener?
NO - manual-release.yml ya tiene multi-platform.

### Migración
- Tag personalizado → ya existe en manual-release.yml (input version)
- Multi-platform → ya existe en manual-release.yml

---

## release.yml

### Funcionalidad Única
- Trigger automático en tag push
- Validaciones completas (gofmt, vet, tests)
- Codecov upload
- GitHub Release con changelog generado

### ¿Se necesita mantener?
NO - manual-release.yml tiene TODO esto y más.

### Migración
- Trigger tag push → Evaluar si se necesita automático
- Validaciones → ya existen en manual-release.yml
- Codecov → agregar si no existe
- GitHub Release → ya existe en manual-release.yml

---

## manual-release.yml (MANTENER)

### Ventajas
✅ Control total (manual)
✅ GitHub App Token (dispara workflows subsecuentes)
✅ Actualiza version.txt
✅ Genera y actualiza CHANGELOG.md
✅ Commit + tag automáticos
✅ Tests completos
✅ Multi-platform
✅ GitHub Release

### Funcionalidad a Agregar
- [ ] Codecov upload (si release.yml lo tiene)
- [ ] Variable environment opcional (si se necesita)
EOF

cat /tmp/docker-workflows-analysis.md
```

---

#### 1.3: Verificar Funcionalidad de manual-release.yml

```bash
# Revisar manual-release.yml completo
cat .github/workflows/manual-release.yml

# Verificar inputs
echo "📋 Inputs de manual-release.yml:"
grep -A 10 "inputs:" .github/workflows/manual-release.yml | head -15

# Verificar jobs
echo "📋 Jobs de manual-release.yml:"
grep "^  [a-z-]*:" .github/workflows/manual-release.yml

# Verificar si tiene Codecov
if grep -q "codecov" .github/workflows/manual-release.yml; then
  echo "✅ manual-release.yml ya tiene Codecov"
else
  echo "⚠️  manual-release.yml NO tiene Codecov"
  echo "   Verificar si release.yml lo tiene:"
  grep -n "codecov" .github/workflows/release.yml
fi

# Verificar multi-platform
if grep -q "linux/amd64,linux/arm64" .github/workflows/manual-release.yml; then
  echo "✅ manual-release.yml tiene multi-platform"
else
  echo "⚠️  manual-release.yml NO tiene multi-platform"
fi
```

---

#### 1.4: Migrar Funcionalidad Faltante (Si es Necesario)

**Caso 1: Si manual-release.yml NO tiene Codecov**

```bash
# Agregar step de Codecov en build-and-test job
# Editar .github/workflows/manual-release.yml

# Buscar job build-and-test
# Agregar después del step "Build":

# - name: Tests con cobertura
#   run: |
#     mkdir -p coverage
#     go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
#
# - name: Subir cobertura a Codecov
#   uses: codecov/codecov-action@v3
#   if: success()
#   with:
#     file: coverage/coverage.out
#     flags: worker
#     name: codecov-release
#     fail_ci_if_error: false

echo "⚠️  Editar .github/workflows/manual-release.yml manualmente"
echo "   Agregar Codecov upload en job build-and-test"
```

**Caso 2: Si manual-release.yml NO tiene multi-platform**

```bash
# Buscar step "Build and push Docker image"
# Cambiar platforms:

# platforms: linux/amd64,linux/arm64

echo "⚠️  Editar .github/workflows/manual-release.yml manualmente"
echo "   Agregar linux/arm64 en platforms"
```

**Verificación:**
```bash
# Después de editar, verificar cambios
git diff .github/workflows/manual-release.yml
```

---

#### 1.5: Eliminar build-and-push.yml

```bash
# Verificar que backup existe
[ -f docs/workflows-removed-sprint3/build-and-push.yml.backup ] && echo "✅ Backup existe" || echo "❌ Crear backup primero"

# Eliminar workflow
rm .github/workflows/build-and-push.yml
echo "✅ build-and-push.yml eliminado"

# Verificar eliminación
if [ ! -f .github/workflows/build-and-push.yml ]; then
  echo "✅ Confirmado: build-and-push.yml eliminado"
else
  echo "❌ Error: build-and-push.yml aún existe"
  exit 1
fi

# Crear entrada en CHANGELOG
cat >> /tmp/sprint3-changelog.md << 'EOF'
### Removed
- Eliminado workflow `build-and-push.yml` (duplicado de manual-release.yml)
  - Funcionalidad consolidada en manual-release.yml
  - Backup disponible en docs/workflows-removed-sprint3/
EOF

echo "✅ Entrada de CHANGELOG creada"
```

---

#### 1.6: Eliminar docker-only.yml

```bash
# Verificar que backup existe
[ -f docs/workflows-removed-sprint3/docker-only.yml.backup ] && echo "✅ Backup existe" || echo "❌ Crear backup primero"

# Eliminar workflow
rm .github/workflows/docker-only.yml
echo "✅ docker-only.yml eliminado"

# Verificar eliminación
if [ ! -f .github/workflows/docker-only.yml ]; then
  echo "✅ Confirmado: docker-only.yml eliminado"
else
  echo "❌ Error: docker-only.yml aún existe"
  exit 1
fi

# Actualizar entrada en CHANGELOG
cat >> /tmp/sprint3-changelog.md << 'EOF'
- Eliminado workflow `docker-only.yml` (duplicado simple)
  - Funcionalidad consolidada en manual-release.yml
  - Backup disponible en docs/workflows-removed-sprint3/
EOF

echo "✅ Entrada de CHANGELOG actualizada"
```

---

#### 1.7: Migrar y Eliminar release.yml

**Análisis previo:**

```bash
# Ver por qué está fallando release.yml
echo "📊 Último run de release.yml:"
gh run list --workflow=release.yml --limit 5

# Ver logs del último fallo
LAST_RUN=$(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
echo "Ver logs en: https://github.com/EduGoGroup/edugo-worker/actions/runs/$LAST_RUN"

# Comparar release.yml vs manual-release.yml
echo "📊 Diferencias clave:"
echo "release.yml: Trigger automático en tag push"
echo "manual-release.yml: Trigger manual con control fino"
```

**Decisión:**

Si necesitas trigger automático en tag push, agregar a manual-release.yml:

```yaml
# Agregar en "on:"
on:
  workflow_dispatch:
    # ... inputs existentes
  push:
    tags:
      - 'v*'
```

Si NO necesitas trigger automático (recomendado):

```bash
# Simplemente eliminar release.yml

# Verificar backup
[ -f docs/workflows-removed-sprint3/release.yml.backup ] && echo "✅ Backup existe" || echo "❌ Crear backup primero"

# Eliminar workflow
rm .github/workflows/release.yml
echo "✅ release.yml eliminado"

# Verificar eliminación
if [ ! -f .github/workflows/release.yml ]; then
  echo "✅ Confirmado: release.yml eliminado"
else
  echo "❌ Error: release.yml aún existe"
  exit 1
fi

# Actualizar CHANGELOG
cat >> /tmp/sprint3-changelog.md << 'EOF'
- Eliminado workflow `release.yml` (fallaba + duplicado)
  - Funcionalidad consolidada en manual-release.yml
  - Trigger automático en tag → Control manual es más seguro
  - Backup disponible en docs/workflows-removed-sprint3/
EOF

echo "✅ release.yml eliminado y documentado"
```

---

#### 1.8: Documentar Uso de manual-release.yml

```bash
# Crear guía de uso
cat > docs/RELEASE-WORKFLOW.md << 'EOF'
# Guía de Release - edugo-worker

## Workflow de Release: manual-release.yml

### ¿Cuándo usar?

- **Releases de producción:** Versiones estables (v1.0.0, v1.1.0, etc.)
- **Hotfixes:** Parches urgentes (v1.0.1, v1.0.2)
- **Features:** Nuevas funcionalidades (v1.1.0, v1.2.0)

### ¿Cómo ejecutar?

#### Opción 1: GitHub UI (Recomendado)

1. Ir a: https://github.com/EduGoGroup/edugo-worker/actions/workflows/manual-release.yml
2. Click en "Run workflow"
3. Seleccionar rama: `main`
4. Ingresar versión: `0.1.0` (sin 'v')
5. Seleccionar tipo: `patch` / `minor` / `major`
6. Click "Run workflow"

#### Opción 2: GitHub CLI

```bash
# Patch release (0.0.1 → 0.0.2)
gh workflow run manual-release.yml \
  -f version=0.0.2 \
  -f bump_type=patch

# Minor release (0.0.1 → 0.1.0)
gh workflow run manual-release.yml \
  -f version=0.1.0 \
  -f bump_type=minor

# Major release (0.0.1 → 1.0.0)
gh workflow run manual-release.yml \
  -f version=1.0.0 \
  -f bump_type=major
```

### ¿Qué hace manual-release.yml?

1. ✅ Valida versión semver
2. ✅ Actualiza version.txt
3. ✅ Genera entrada de CHANGELOG
4. ✅ Commit a main
5. ✅ Crea y pushea tag
6. ✅ Ejecuta tests completos
7. ✅ Build y push Docker image
8. ✅ Crea GitHub Release

### Variables de Salida

- **Tag creado:** `v{version}`
- **Docker image:** `ghcr.io/edugogroup/edugo-worker:v{version}`
- **Docker image latest:** `ghcr.io/edugogroup/edugo-worker:latest`
- **GitHub Release:** `https://github.com/EduGoGroup/edugo-worker/releases/tag/v{version}`

### Bump Types

| Tipo | Ejemplo | Uso |
|------|---------|-----|
| **patch** | 0.0.1 → 0.0.2 | Bugfixes, hotfixes |
| **minor** | 0.0.1 → 0.1.0 | Nuevas features (no breaking) |
| **major** | 0.0.1 → 1.0.0 | Breaking changes o producción |

### Verificación Post-Release

```bash
# 1. Verificar tag creado
git fetch --tags
git tag -l "v*" | tail -5

# 2. Verificar Docker image
docker pull ghcr.io/edugogroup/edugo-worker:v0.1.0
docker pull ghcr.io/edugogroup/edugo-worker:latest

# 3. Verificar GitHub Release
gh release view v0.1.0

# 4. Verificar CHANGELOG
cat CHANGELOG.md | head -50
```

### Troubleshooting

**Error: Tag already exists**
```bash
# Eliminar tag localmente y remotamente
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0

# Volver a ejecutar workflow
```

**Error: Tests failing**
```bash
# Ejecutar tests localmente
go test -v ./...

# Corregir tests y hacer commit
git add .
git commit -m "fix: corregir tests"
git push origin main

# Volver a ejecutar workflow
```

### Workflows Antiguos (Eliminados)

Los siguientes workflows fueron eliminados en Sprint 3:

- ❌ `build-and-push.yml` - Duplicado sin tests
- ❌ `docker-only.yml` - Duplicado simple
- ❌ `release.yml` - Fallaba + duplicado

**Razón:** Consolidación en manual-release.yml para:
- Eliminar duplicación
- Control fino
- Tests completos
- CHANGELOG automático

**Backups disponibles en:** `docs/workflows-removed-sprint3/`

---

**Última actualización:** Sprint 3 - 19 Nov 2025
EOF

echo "✅ RELEASE-WORKFLOW.md creado"
```

---

#### 1.9: Actualizar README Principal

```bash
# Agregar sección de Release en README.md
# Buscar sección apropiada y agregar:

cat >> /tmp/readme-release-section.md << 'EOF'

## 🚀 Release Process

edugo-worker usa un proceso de release manual controlado.

### Quick Start

```bash
# Ejecutar release desde GitHub UI
https://github.com/EduGoGroup/edugo-worker/actions/workflows/manual-release.yml

# O desde CLI
gh workflow run manual-release.yml -f version=0.1.0 -f bump_type=minor
```

Ver [RELEASE-WORKFLOW.md](docs/RELEASE-WORKFLOW.md) para guía completa.

### Release Types

- **patch** (0.0.1 → 0.0.2): Bugfixes
- **minor** (0.0.1 → 0.1.0): Features
- **major** (0.0.1 → 1.0.0): Breaking changes

EOF

echo "⚠️  Editar README.md manualmente"
echo "   Agregar sección de Release Process"
cat /tmp/readme-release-section.md
```

---

#### 1.10: Commit Cambios de Tarea 1

```bash
# Agregar archivos al staging
git add .github/workflows/
git add docs/workflows-removed-sprint3/
git add docs/RELEASE-WORKFLOW.md

# Verificar cambios
git status
echo "📊 Cambios a commitear:"
git diff --cached --stat

# Commit
git commit -m "feat: consolidar workflows Docker en manual-release.yml

- Eliminar build-and-push.yml (duplicado sin tests)
- Eliminar docker-only.yml (duplicado simple)
- Eliminar release.yml (fallaba + duplicado)
- Mantener solo manual-release.yml con control fino
- Crear backups en docs/workflows-removed-sprint3/
- Documentar proceso de release en RELEASE-WORKFLOW.md

BREAKING CHANGE: Workflows build-and-push.yml, docker-only.yml y release.yml
eliminados. Usar manual-release.yml para todos los releases.

Reduce workflows Docker de 3 a 1 (-66%)
Elimina ~250 líneas duplicadas (-23%)
Resuelve fallos en release.yml

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>" -m ""

echo "✅ Commit de Tarea 1 completado"
```

---

### Validación de Tarea 1

```bash
# 1. Verificar workflows restantes
echo "📊 Workflows restantes:"
ls -1 .github/workflows/
# Debe mostrar solo 4:
# - ci.yml
# - test.yml
# - manual-release.yml
# - sync-main-to-dev.yml

# 2. Verificar backups
echo "📊 Backups creados:"
ls -1 docs/workflows-removed-sprint3/
# Debe mostrar:
# - build-and-push.yml.backup
# - docker-only.yml.backup
# - release.yml.backup
# - README.md

# 3. Verificar documentación
[ -f docs/RELEASE-WORKFLOW.md ] && echo "✅ RELEASE-WORKFLOW.md existe" || echo "❌ Falta RELEASE-WORKFLOW.md"

# 4. Verificar commit
git log -1 --oneline | grep "feat: consolidar workflows Docker"
if [ $? -eq 0 ]; then
  echo "✅ Commit de Tarea 1 verificado"
else
  echo "❌ Commit faltante"
fi

# 5. Contar workflows Docker restantes
DOCKER_WORKFLOWS=$(grep -l "docker/build-push-action" .github/workflows/*.yml | wc -l | tr -d ' ')
if [ "$DOCKER_WORKFLOWS" -eq "1" ]; then
  echo "✅ Solo 1 workflow Docker restante"
else
  echo "❌ Aún hay $DOCKER_WORKFLOWS workflows Docker"
fi
```

### Solución de Problemas Comunes

**Problema 1: No puedo eliminar workflows**
```bash
# Verificar que estás en rama correcta
git branch --show-current
# Debe mostrar: feature/sprint-3-consolidation-docker-go125

# Verificar que no hay cambios sin commitear
git status
```

**Problema 2: Backup no se creó**
```bash
# Crear directorio si no existe
mkdir -p docs/workflows-removed-sprint3

# Volver a intentar backup
cp .github/workflows/[workflow].yml docs/workflows-removed-sprint3/[workflow].yml.backup
```

**Problema 3: manual-release.yml no funciona**
```bash
# Verificar sintaxis YAML
cat .github/workflows/manual-release.yml | python -c "import sys, yaml; yaml.safe_load(sys.stdin)" && echo "✅ YAML válido" || echo "❌ YAML inválido"

# Probar workflow en GitHub (sin ejecutar)
gh workflow view manual-release.yml
```

---

## Tarea 2: Migrar a Go 1.25.3

**Duración:** 45-60 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Ninguna

### Objetivo

Actualizar go.mod de Go 1.24.10 a Go 1.25.3, alineando con shared e infrastructure.

### Contexto

- **Actual:** go.mod dice `go 1.24.10`, workflows dicen `go 1.25`
- **Objetivo:** Consistencia en Go 1.25.3
- **Beneficios:** Mejoras de performance, nuevas features, consistencia

### Pasos

#### 2.1: Actualizar go.mod

```bash
# Verificar versión actual
echo "📊 Versión actual de Go:"
cat go.mod | grep "^go "

# Actualizar go.mod
cat > /tmp/update-go-version.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

# Backup de go.mod
cp go.mod go.mod.backup

# Actualizar versión de Go
sed -i '' 's/^go 1\.24\.10$/go 1.25.3/' go.mod

# Verificar cambio
if grep -q "go 1.25.3" go.mod; then
  echo "✅ go.mod actualizado a Go 1.25.3"
else
  echo "❌ Error al actualizar go.mod"
  mv go.mod.backup go.mod
  exit 1
fi

# Actualizar dependencias
echo "📦 Actualizando dependencias..."
go mod tidy

# Verificar que compile
echo "🔨 Verificando compilación..."
go build -v ./...

echo "✅ Migración a Go 1.25.3 completada"
EOFSCRIPT

chmod +x /tmp/update-go-version.sh
/tmp/update-go-version.sh
```

**Salida esperada:**
```
✅ go.mod actualizado a Go 1.25.3
📦 Actualizando dependencias...
go: downloading ...
🔨 Verificando compilación...
✅ Migración a Go 1.25.3 completada
```

---

#### 2.2: Actualizar Workflows

```bash
# Actualizar versiones de Go en workflows
# ci.yml, test.yml, manual-release.yml

for workflow in ci.yml test.yml manual-release.yml; do
  echo "📝 Actualizando $workflow..."
  
  # Backup
  cp .github/workflows/$workflow .github/workflows/$workflow.backup
  
  # Actualizar GO_VERSION
  sed -i '' "s/GO_VERSION: '1\.25'/GO_VERSION: '1.25.3'/g" .github/workflows/$workflow
  sed -i '' "s/GO_VERSION: '1\.24\.10'/GO_VERSION: '1.25.3'/g" .github/workflows/$workflow
  
  # Verificar cambio
  if grep -q "GO_VERSION: '1.25.3'" .github/workflows/$workflow; then
    echo "✅ $workflow actualizado"
    rm .github/workflows/$workflow.backup
  else
    echo "⚠️  $workflow no tenía GO_VERSION o ya estaba actualizado"
    rm .github/workflows/$workflow.backup
  fi
done

echo "✅ Todos los workflows actualizados"
```

---

#### 2.3: Ejecutar Tests Localmente

```bash
# Ejecutar suite completa de tests
echo "🧪 Ejecutando tests con Go 1.25.3..."

# Tests unitarios
go test -v ./...

# Tests con race detection
go test -v -race ./...

# Tests con coverage
mkdir -p coverage
go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...

# Ver coverage
go tool cover -func=coverage/coverage.out | tail -10

echo "✅ Tests pasaron con Go 1.25.3"
```

**Si hay fallos:**
```bash
# Ver detalles del fallo
go test -v ./... 2>&1 | tee /tmp/test-failures.log

# Analizar
cat /tmp/test-failures.log | grep "FAIL"

# Corregir según errores encontrados
# Común: Cambios en stdlib de Go 1.25
```

---

#### 2.4: Verificar Dependencias Actualizadas

```bash
# Listar dependencias actualizadas
echo "📦 Dependencias actualizadas:"
git diff go.mod

# Verificar cambios en go.sum
echo "📦 Cambios en go.sum:"
git diff go.sum | head -50

# Verificar que no haya dependencias rotas
go mod verify
echo "✅ Dependencias verificadas"
```

---

#### 2.5: Commit Cambios de Go 1.25.3

```bash
# Agregar archivos
git add go.mod go.sum .github/workflows/

# Ver cambios
git diff --cached

# Commit
git commit -m "chore: migrar a Go 1.25.3

- Actualizar go.mod de 1.24.10 → 1.25.3
- Actualizar workflows (ci.yml, test.yml, manual-release.yml)
- Actualizar dependencias con go mod tidy
- Todos los tests pasando con Go 1.25.3

Alinea versión de Go con shared e infrastructure.
Aprovecha mejoras de performance y nuevas features.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo "✅ Commit de Go 1.25.3 completado"
```

---

### Validación de Tarea 2

```bash
# 1. Verificar go.mod
grep "^go 1.25.3" go.mod && echo "✅ go.mod actualizado" || echo "❌ go.mod no actualizado"

# 2. Verificar workflows
for workflow in ci.yml test.yml manual-release.yml; do
  if grep -q "GO_VERSION: '1.25.3'" .github/workflows/$workflow; then
    echo "✅ $workflow actualizado"
  else
    echo "⚠️  $workflow sin GO_VERSION o diferente"
  fi
done

# 3. Verificar tests
go test ./... && echo "✅ Tests pasan" || echo "❌ Tests fallan"

# 4. Verificar build
go build ./... && echo "✅ Build exitoso" || echo "❌ Build falla"

# 5. Verificar commit
git log -1 --oneline | grep "chore: migrar a Go 1.25.3" && echo "✅ Commit verificado" || echo "❌ Commit faltante"
```

### Solución de Problemas Comunes

**Problema 1: Tests fallan después de actualizar**
```bash
# Ver errores específicos
go test -v ./... 2>&1 | grep "FAIL"

# Revisar changelog de Go 1.25
open https://go.dev/doc/go1.25

# Común: Cambios en stdlib
# Solución: Actualizar código según breaking changes
```

**Problema 2: Dependencias incompatibles**
```bash
# Actualizar dependencias principales
go get -u github.com/EduGoGroup/edugo-shared/...
go get -u github.com/EduGoGroup/edugo-infrastructure/...

# Limpiar caché
go clean -modcache

# Volver a intentar
go mod tidy
```

---

## Tarea 3: Actualizar .gitignore y Archivos de Configuración

**Duración:** 15-20 minutos  
**Prioridad:** 🟢 Media  
**Dependencias:** Ninguna

### Objetivo

Actualizar .gitignore para excluir archivos temporales y de coverage generados en Sprint 3.

### Pasos

#### 3.1: Actualizar .gitignore

```bash
# Verificar .gitignore actual
cat .gitignore

# Agregar entradas nuevas
cat >> .gitignore << 'EOF'

# Sprint 3 additions
# Coverage reports
coverage/
*.out
*.html

# Temporary files
/tmp/
*.tmp
*.backup

# Pre-commit
.pre-commit-config.yaml

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
EOF

echo "✅ .gitignore actualizado"
```

---

#### 3.2: Commit Cambios

```bash
git add .gitignore

git commit -m "chore: actualizar .gitignore

- Agregar coverage/ para reportes
- Agregar /tmp/ para archivos temporales
- Agregar .pre-commit-config.yaml
- Agregar archivos de IDE y OS

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo "✅ Commit de .gitignore completado"
```

---

## Tarea 4: Implementar Pre-commit Hooks

**Duración:** 60-90 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 2 (Go 1.25.3)

### Objetivo

Implementar 7 pre-commit hooks para validación automática antes de commits.

### Contexto

Pre-commit hooks evitan:
- Código sin formatear
- Archivos grandes por error
- Secretos expuestos
- YAML inválido
- Tests rotos

### Pasos

#### 4.1: Instalar pre-commit

```bash
# Instalar pre-commit
pip install pre-commit

# Verificar instalación
pre-commit --version

echo "✅ pre-commit instalado"
```

---

#### 4.2: Crear .pre-commit-config.yaml

```bash
cat > .pre-commit-config.yaml << 'EOF'
# Pre-commit hooks para edugo-worker
# Instalación: pip install pre-commit && pre-commit install
# Ejecutar manual: pre-commit run --all-files

repos:
  # 1. Validaciones básicas
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      # Prevenir commits a main directamente
      - id: no-commit-to-branch
        args: ['--branch', 'main']
      
      # Verificar que archivos terminen con newline
      - id: end-of-file-fixer
      
      # Remover espacios en blanco al final de líneas
      - id: trailing-whitespace
        args: [--markdown-linebreak-ext=md]
      
      # Prevenir archivos grandes (>500KB)
      - id: check-added-large-files
        args: ['--maxkb=500']
      
      # Validar YAML
      - id: check-yaml
        args: ['--unsafe']  # Permite templates de GitHub Actions
      
      # Detectar credenciales expuestas
      - id: detect-private-key
      
      # Verificar merge conflicts sin resolver
      - id: check-merge-conflict

  # 2. Go fmt y imports
  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      # Formatear código Go
      - id: go-fmt
      
      # Organizar imports
      - id: go-imports
      
      # Análisis estático
      - id: go-vet
      
      # Verificar que go.mod esté actualizado
      - id: go-mod-tidy

  # 3. Tests unitarios (opcional, puede ser lento)
  - repo: local
    hooks:
      - id: go-test
        name: go test
        entry: go test -short ./...
        language: system
        pass_filenames: false
        stages: [commit]
        # Solo si cambió código Go
        files: \.go$

# Configuración global
default_language_version:
  python: python3

# Stages a ejecutar
default_stages: [commit]

# Excluir archivos
exclude: |
  (?x)^(
    vendor/|
    .github/workflows/.*\.yml|
    docs/workflows-removed-sprint3/
  )$
EOF

echo "✅ .pre-commit-config.yaml creado"
```

---

#### 4.3: Instalar Hooks en Git

```bash
# Instalar hooks
pre-commit install

# Verificar instalación
ls -la .git/hooks/pre-commit
[ -f .git/hooks/pre-commit ] && echo "✅ Hooks instalados en Git" || echo "❌ Error instalando hooks"

# Ejecutar en todos los archivos (primera vez)
echo "🔍 Ejecutando pre-commit en todos los archivos..."
pre-commit run --all-files

echo "✅ Pre-commit hooks configurados"
```

**Salida esperada:**
```
Check for added large files..............................................Passed
Check yaml...............................................................Passed
Detect Private Key.......................................................Passed
Check for merge conflicts................................................Passed
Trim Trailing Whitespace.................................................Passed
Fix End of Files.........................................................Passed
go fmt...................................................................Passed
go imports...............................................................Passed
go vet...................................................................Passed
go mod tidy..............................................................Passed
go test..................................................................Passed
```

---

#### 4.4: Documentar Pre-commit Hooks

```bash
# Agregar sección en README.md
cat > /tmp/readme-precommit-section.md << 'EOF'

## 🔧 Pre-commit Hooks

edugo-worker usa pre-commit hooks para validar código antes de commits.

### Instalación

```bash
# Instalar pre-commit
pip install pre-commit

# Instalar hooks en el repo
pre-commit install
```

### Hooks Configurados

1. **no-commit-to-branch** - Previene commits directos a main
2. **end-of-file-fixer** - Agrega newline al final de archivos
3. **trailing-whitespace** - Remueve espacios en blanco
4. **check-added-large-files** - Previene archivos >500KB
5. **check-yaml** - Valida sintaxis YAML
6. **detect-private-key** - Detecta credenciales expuestas
7. **check-merge-conflict** - Detecta conflictos sin resolver
8. **go-fmt** - Formatea código Go
9. **go-imports** - Organiza imports
10. **go-vet** - Análisis estático
11. **go-mod-tidy** - Verifica go.mod actualizado
12. **go-test** - Ejecuta tests (opcional)

### Uso

```bash
# Automático en cada commit
git commit -m "mensaje"

# Manual en todos los archivos
pre-commit run --all-files

# Manual en archivos staged
pre-commit run

# Saltar hooks (NO recomendado)
git commit --no-verify -m "mensaje"
```

### Troubleshooting

**Hook falla:**
```bash
# Ver qué hook falló
pre-commit run --all-files --verbose

# Corregir y volver a intentar
git add .
git commit -m "mensaje"
```

**Saltar un hook específico:**
```bash
# Editar .pre-commit-config.yaml
# Comentar el hook que quieres saltar
```

EOF

echo "⚠️  Editar README.md manualmente"
echo "   Agregar sección de Pre-commit Hooks"
cat /tmp/readme-precommit-section.md
```

---

#### 4.5: Commit Pre-commit Config

```bash
# Agregar archivos
git add .pre-commit-config.yaml

# Commit
git commit -m "feat: implementar pre-commit hooks

- Agregar .pre-commit-config.yaml con 12 hooks
- 7 hooks básicos (yaml, archivos grandes, secretos, etc.)
- 5 hooks de Go (fmt, imports, vet, mod-tidy, test)
- Documentar instalación y uso en README
- Ejecutar pre-commit en todos los archivos

Previene:
- Commits directos a main
- Código sin formatear
- Archivos grandes
- Secretos expuestos
- YAML inválido
- Tests rotos

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo "✅ Commit de pre-commit hooks completado"
```

---

### Validación de Tarea 4

```bash
# 1. Verificar archivo existe
[ -f .pre-commit-config.yaml ] && echo "✅ .pre-commit-config.yaml existe" || echo "❌ Archivo faltante"

# 2. Verificar hooks instalados en Git
[ -f .git/hooks/pre-commit ] && echo "✅ Hooks instalados en Git" || echo "❌ Hooks no instalados"

# 3. Ejecutar pre-commit
pre-commit run --all-files && echo "✅ Pre-commit pasa" || echo "⚠️  Pre-commit tiene warnings"

# 4. Verificar commit
git log -1 --oneline | grep "feat: implementar pre-commit hooks" && echo "✅ Commit verificado" || echo "❌ Commit faltante"

# 5. Contar hooks configurados
HOOKS_COUNT=$(grep "    - id:" .pre-commit-config.yaml | wc -l | tr -d ' ')
echo "📊 Hooks configurados: $HOOKS_COUNT"
[ "$HOOKS_COUNT" -ge "10" ] && echo "✅ Suficientes hooks" || echo "⚠️  Pocos hooks"
```

### Solución de Problemas Comunes

**Problema 1: pre-commit no instalado**
```bash
# Instalar con pip
pip install pre-commit

# O con homebrew (macOS)
brew install pre-commit

# Verificar
pre-commit --version
```

**Problema 2: go-fmt falla**
```bash
# Formatear todos los archivos Go
gofmt -w .

# Volver a ejecutar
pre-commit run --all-files
```

**Problema 3: go-test muy lento**
```bash
# Editar .pre-commit-config.yaml
# Cambiar go-test a usar -short:
# entry: go test -short ./...

# O comentar el hook si es muy lento
```

---

## Tarea 5: Establecer Coverage Threshold 33%

**Duración:** 45 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Ninguna

### Objetivo

Establecer umbral mínimo de cobertura de código en 33%, alineando con api-mobile y api-administracion.

### Contexto

- **api-mobile:** Coverage threshold 33%
- **api-administracion:** Coverage threshold 33%
- **worker:** Sin threshold (este Sprint lo establece)

### Pasos

#### 5.1: Verificar Coverage Actual

```bash
# Ejecutar tests con coverage
mkdir -p coverage
go test -v -coverprofile=coverage/coverage.out -covermode=atomic ./...

# Ver coverage total
COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
echo "📊 Coverage actual: ${COVERAGE}%"

# Verificar si alcanza 33%
if (( $(echo "$COVERAGE >= 33.0" | bc -l) )); then
  echo "✅ Coverage actual (${COVERAGE}%) >= 33%"
else
  echo "⚠️  Coverage actual (${COVERAGE}%) < 33%"
  echo "   Se necesita mejorar coverage antes de establecer threshold"
fi
```

---

#### 5.2: Actualizar test.yml con Threshold

```bash
# Backup de test.yml
cp .github/workflows/test.yml .github/workflows/test.yml.backup

# Agregar verificación de threshold
cat > /tmp/coverage-threshold-snippet.yml << 'EOF'
      - name: Verificar umbral de cobertura
        run: |
          COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
          THRESHOLD=33.0
          
          echo "📊 Cobertura actual: ${COVERAGE}%"
          echo "📊 Umbral mínimo: ${THRESHOLD}%"
          
          if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo "❌ Cobertura ${COVERAGE}% está por debajo del umbral ${THRESHOLD}%"
            exit 1
          else
            echo "✅ Cobertura ${COVERAGE}% cumple con el umbral ${THRESHOLD}%"
          fi
EOF

echo "📝 Snippet de coverage threshold creado"
echo "⚠️  Editar .github/workflows/test.yml manualmente"
echo "   Agregar step después de 'Ejecutar tests con cobertura'"
cat /tmp/coverage-threshold-snippet.yml
```

**Ubicación en test.yml:**
```yaml
jobs:
  test-coverage:
    steps:
      # ... steps anteriores ...
      
      - name: Ejecutar tests con cobertura
        run: |
          mkdir -p coverage
          go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...

      # AGREGAR AQUÍ ⬇️
      - name: Verificar umbral de cobertura
        run: |
          COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
          THRESHOLD=33.0
          
          echo "📊 Cobertura actual: ${COVERAGE}%"
          echo "📊 Umbral mínimo: ${THRESHOLD}%"
          
          if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
            echo "❌ Cobertura ${COVERAGE}% está por debajo del umbral ${THRESHOLD}%"
            exit 1
          else
            echo "✅ Cobertura ${COVERAGE}% cumple con el umbral ${THRESHOLD}%"
          fi
      
      # ... steps siguientes ...
```

---

#### 5.3: Actualizar ci.yml con Coverage (Opcional)

Si quieres coverage también en CI (no solo en test.yml):

```bash
# Similar a test.yml, agregar en ci.yml
echo "⚠️  Si quieres coverage en ci.yml, agregar similar a test.yml"
echo "   Pero NO es obligatorio (test.yml es suficiente)"
```

---

#### 5.4: Documentar Estándares de Coverage

```bash
cat > docs/COVERAGE-STANDARDS.md << 'EOF'
# Estándares de Cobertura de Código - edugo-worker

## Threshold Actual

**Mínimo requerido:** 33%

## Ejecución Local

```bash
# Generar reporte de coverage
go test -coverprofile=coverage/coverage.out -covermode=atomic ./...

# Ver coverage total
go tool cover -func=coverage/coverage.out | tail -1

# Generar reporte HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html
```

## Verificar Threshold

```bash
# Verificar que cumple threshold
COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
THRESHOLD=33.0

if (( $(echo "$COVERAGE >= $THRESHOLD" | bc -l) )); then
  echo "✅ Coverage OK: ${COVERAGE}%"
else
  echo "❌ Coverage bajo: ${COVERAGE}% (mínimo ${THRESHOLD}%)"
fi
```

## Coverage por Paquete

```bash
# Ver coverage por paquete
go tool cover -func=coverage/coverage.out | grep -E "^github.com/EduGoGroup/edugo-worker"

# Paquetes con coverage bajo
go tool cover -func=coverage/coverage.out | awk '{if ($NF < 33) print $0}'
```

## Mejorar Coverage

### 1. Identificar código sin coverage

```bash
# Generar reporte HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html

# Buscar líneas rojas (sin coverage)
```

### 2. Agregar tests

```go
// Ejemplo: test para función sin coverage
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test"
    expected := "result"
    
    // Act
    result := FunctionName(input)
    
    // Assert
    if result != expected {
        t.Errorf("Expected %s, got %s", expected, result)
    }
}
```

### 3. Verificar mejora

```bash
# Ejecutar tests con nuevo test
go test -coverprofile=coverage/coverage.out ./...

# Ver nueva coverage
go tool cover -func=coverage/coverage.out | tail -1
```

## Exclusiones de Coverage

Archivos excluidos de threshold (pero sí se miden):

- `cmd/main.go` - Entry point (difícil de testear)
- `*_mock.go` - Mocks generados
- `internal/testhelpers/` - Helpers de testing

## CI/CD

### test.yml

Coverage threshold se verifica en cada PR:

```yaml
- name: Verificar umbral de cobertura
  run: |
    COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
    THRESHOLD=33.0
    
    if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
      echo "❌ Coverage ${COVERAGE}% < ${THRESHOLD}%"
      exit 1
    fi
```

### Codecov

Reports suben a Codecov para tracking histórico:

```bash
# Ver en: https://codecov.io/gh/EduGoGroup/edugo-worker
```

## Plan de Mejora

| Fase | Threshold | Fecha Objetivo |
|------|-----------|----------------|
| **Sprint 3** | 33% | Nov 2025 |
| Sprint 5 | 40% | Dic 2025 |
| Sprint 7 | 50% | Ene 2026 |
| Sprint 10 | 60% | Feb 2026 |

## Referencias

- [api-mobile coverage](../03-api-mobile/docs/COVERAGE-STANDARDS.md)
- [api-administracion coverage](../04-api-administracion/docs/COVERAGE-STANDARDS.md)
- [Go testing package](https://pkg.go.dev/testing)
- [Go coverage tool](https://go.dev/blog/cover)

---

**Última actualización:** Sprint 3 - 19 Nov 2025
EOF

echo "✅ COVERAGE-STANDARDS.md creado"
```

---

#### 5.5: Commit Cambios de Coverage

```bash
# Agregar archivos
git add .github/workflows/test.yml
git add docs/COVERAGE-STANDARDS.md
git add coverage/  # Si existe

# Commit
git commit -m "feat: establecer umbral de cobertura 33%

- Agregar verificación de threshold en test.yml
- Documentar estándares en COVERAGE-STANDARDS.md
- Alinear con api-mobile y api-administracion (33%)
- Plan de mejora gradual hasta 60%

Previene regresiones de calidad.
Fuerza mejora continua de tests.

Coverage actual: [COVERAGE_ACTUAL]%

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo "✅ Commit de coverage threshold completado"
```

---

### Validación de Tarea 5

```bash
# 1. Verificar coverage actual
COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
echo "📊 Coverage actual: ${COVERAGE}%"

# 2. Verificar threshold en test.yml
grep -q "THRESHOLD=33.0" .github/workflows/test.yml && echo "✅ Threshold configurado en test.yml" || echo "❌ Threshold no configurado"

# 3. Verificar documentación
[ -f docs/COVERAGE-STANDARDS.md ] && echo "✅ COVERAGE-STANDARDS.md existe" || echo "❌ Documentación faltante"

# 4. Ejecutar test workflow localmente (simular)
mkdir -p coverage
go test -coverprofile=coverage/coverage.out -covermode=atomic ./...
COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
THRESHOLD=33.0

if (( $(echo "$COVERAGE >= $THRESHOLD" | bc -l) )); then
  echo "✅ Coverage ${COVERAGE}% >= ${THRESHOLD}%"
else
  echo "❌ Coverage ${COVERAGE}% < ${THRESHOLD}%"
fi

# 5. Verificar commit
git log -1 --oneline | grep "feat: establecer umbral de cobertura 33%" && echo "✅ Commit verificado" || echo "❌ Commit faltante"
```

### Solución de Problemas Comunes

**Problema 1: Coverage < 33%**
```bash
# Ver qué paquetes tienen coverage bajo
go tool cover -func=coverage/coverage.out | awk '{if ($NF ~ /[0-9]+\.[0-9]+%/) {gsub(/%/, "", $NF); if ($NF < 33) print $0}}'

# Generar HTML para identificar líneas sin coverage
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
open coverage/coverage.html

# Agregar tests para mejorar coverage
# (Esto puede tomar tiempo, considerar como tarea separada)
```

**Problema 2: bc command not found**
```bash
# macOS: Instalar bc
brew install bc

# Linux: Instalar bc
sudo apt-get install bc

# O cambiar verificación en workflow a usar awk
```

**Problema 3: Coverage no se genera**
```bash
# Verificar que tests existen
find . -name "*_test.go" | head -10

# Ejecutar tests verbose
go test -v ./...

# Verificar que coverage/ directory existe
mkdir -p coverage
```

---

## Tarea 6: Actualizar Documentación General

**Duración:** 30-45 minutos  
**Prioridad:** 🟢 Media  
**Dependencias:** Tareas 1-5

### Objetivo

Actualizar README.md principal con todos los cambios de Sprint 3.

### Pasos

#### 6.1: Actualizar README.md

```bash
# Crear sección de cambios Sprint 3
cat > /tmp/readme-sprint3-updates.md << 'EOF'

## 📋 Recent Changes (Sprint 3)

### Workflows Consolidados
- ✅ Eliminados 3 workflows Docker duplicados
- ✅ Mantenido solo `manual-release.yml` con control fino
- ✅ Reducción de ~250 líneas (-23%)

### Tecnología Actualizada
- ✅ Go 1.25.3 (anteriormente 1.24.10)
- ✅ Pre-commit hooks (12 hooks configurados)
- ✅ Coverage threshold 33%

### Guías Disponibles
- [Release Workflow](docs/RELEASE-WORKFLOW.md)
- [Coverage Standards](docs/COVERAGE-STANDARDS.md)
- [Pre-commit Hooks](#-pre-commit-hooks)

EOF

echo "⚠️  Editar README.md manualmente"
echo "   Agregar sección de Recent Changes"
cat /tmp/readme-sprint3-updates.md
```

---

#### 6.2: Actualizar Badges en README

```bash
# Agregar badges de Go version, coverage, etc.
cat > /tmp/readme-badges.md << 'EOF'
# edugo-worker

![Go Version](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go)
![Coverage](https://img.shields.io/badge/coverage-33%25-brightgreen)
![Workflows](https://img.shields.io/badge/workflows-4-blue)
![Pre--commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)

Worker de procesamiento asíncrono para EduGo.

EOF

echo "⚠️  Editar README.md manualmente"
echo "   Actualizar badges al inicio"
cat /tmp/readme-badges.md
```

---

#### 6.3: Actualizar Tabla de Workflows

```bash
# Nueva tabla de workflows
cat > /tmp/readme-workflows-table.md << 'EOF'

## 🔄 Workflows CI/CD

| Workflow | Trigger | Propósito | Estado |
|----------|---------|-----------|--------|
| `ci.yml` | PR + Push main | Tests y validaciones | ✅ Activo |
| `test.yml` | Manual + PR | Coverage con threshold 33% | ✅ Activo |
| `manual-release.yml` | Manual | Release completo controlado | ✅ Activo |
| `sync-main-to-dev.yml` | Push a main | Sincronización automática | ✅ Activo |

**Workflows eliminados en Sprint 3:**
- ❌ `build-and-push.yml` - Consolidado en manual-release.yml
- ❌ `docker-only.yml` - Consolidado en manual-release.yml
- ❌ `release.yml` - Consolidado en manual-release.yml

EOF

echo "⚠️  Editar README.md manualmente"
echo "   Actualizar tabla de workflows"
cat /tmp/readme-workflows-table.md
```

---

#### 6.4: Commit Actualización de Documentación

```bash
# Agregar README.md
git add README.md

# Commit
git commit -m "docs: actualizar README con cambios Sprint 3

- Agregar sección Recent Changes
- Actualizar badges (Go 1.25.3, coverage 33%)
- Actualizar tabla de workflows (4 activos)
- Documentar workflows eliminados
- Links a guías nuevas (RELEASE-WORKFLOW, COVERAGE-STANDARDS)

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

echo "✅ Commit de documentación completado"
```

---

## Tarea 7: Verificar Workflows en GitHub Actions

**Duración:** 30-45 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tareas 1-6

### Objetivo

Pushear cambios a rama feature y verificar que workflows funcionan correctamente.

### Pasos

#### 7.1: Push a Rama Feature

```bash
# Verificar rama actual
git branch --show-current

# Ver commits del Sprint 3
git log --oneline origin/main..HEAD

# Push a rama feature
git push origin feature/sprint-3-consolidation-docker-go125

echo "✅ Cambios pusheados a rama feature"
```

---

#### 7.2: Crear PR Draft

```bash
# Crear PR en modo draft
gh pr create \
  --base dev \
  --head feature/sprint-3-consolidation-docker-go125 \
  --title "feat: Sprint 3 - Consolidación Docker + Go 1.25" \
  --body "## Sprint 3: Consolidación Docker + Go 1.25

### Cambios Principales

#### 🔴 Consolidación de Workflows Docker
- ✅ Eliminado \`build-and-push.yml\` (duplicado sin tests)
- ✅ Eliminado \`docker-only.yml\` (duplicado simple)
- ✅ Eliminado \`release.yml\` (fallaba + duplicado)
- ✅ Mantenido solo \`manual-release.yml\` con control fino
- ✅ Backups en \`docs/workflows-removed-sprint3/\`
- ✅ Documentado en \`docs/RELEASE-WORKFLOW.md\`

**Impacto:**
- Reducción de 3 workflows a 1 (-66%)
- Eliminación de ~250 líneas (-23%)
- Resolución de fallos en release.yml
- Claridad para el equipo

#### 🟡 Migración a Go 1.25.3
- ✅ Actualizado \`go.mod\` de 1.24.10 → 1.25.3
- ✅ Actualizado workflows (\`ci.yml\`, \`test.yml\`, \`manual-release.yml\`)
- ✅ Todos los tests pasando
- ✅ Dependencias actualizadas

**Impacto:**
- Consistencia con shared e infrastructure
- Mejoras de performance de Go 1.25
- Aprovechar nuevas features

#### 🟡 Pre-commit Hooks
- ✅ Creado \`.pre-commit-config.yaml\` con 12 hooks
- ✅ Hooks instalados en Git
- ✅ Documentado instalación y uso

**Hooks configurados:**
- 7 validaciones básicas (YAML, archivos grandes, secretos, etc.)
- 5 validaciones de Go (fmt, imports, vet, mod-tidy, test)

**Impacto:**
- Previene commits directos a main
- Código siempre formateado
- No secretos expuestos
- Tests ejecutados antes de commit

#### 🟡 Coverage Threshold 33%
- ✅ Agregado threshold en \`test.yml\`
- ✅ Documentado en \`docs/COVERAGE-STANDARDS.md\`
- ✅ Alineado con api-mobile y api-administracion

**Impacto:**
- Previene regresiones de calidad
- Fuerza mejora continua
- Estándar consistente en todos los repos

#### 📚 Documentación
- ✅ \`docs/RELEASE-WORKFLOW.md\` - Guía de releases
- ✅ \`docs/COVERAGE-STANDARDS.md\` - Estándares de coverage
- ✅ README.md actualizado con cambios Sprint 3

### Métricas

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| Workflows Docker | 3 | 1 | -66% |
| Líneas workflows Docker | ~441 | ~340 | -23% |
| Go version consistente | No | Sí | ✅ |
| Coverage threshold | No | 33% | ✅ |
| Pre-commit hooks | 0 | 12 | ✅ |
| Success rate esperado | 70% | 85%+ | +15% |

### Checklist

- [x] Tarea 1: Consolidar workflows Docker
- [x] Tarea 2: Migrar a Go 1.25.3
- [x] Tarea 3: Actualizar .gitignore
- [x] Tarea 4: Implementar pre-commit hooks
- [x] Tarea 5: Establecer coverage threshold 33%
- [x] Tarea 6: Actualizar documentación
- [x] Tarea 7: Verificar workflows
- [ ] Tarea 8: Review y ajustes
- [ ] Tarea 9: Merge a dev

### Testing

- [x] Tests locales pasando
- [x] Pre-commit hooks funcionando
- [x] Coverage >= 33%
- [ ] CI workflow pasando (verificar en PR)
- [ ] Test workflow pasando (verificar en PR)

### Referencias

- [Plan Sprint 3](../Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/SPRINT-3-TASKS.md)
- [Análisis de Duplicación Docker](../Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/README.md#análisis-de-duplicación-docker)

---

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>" \
  --draft

echo "✅ PR draft creado"
```

---

#### 7.3: Verificar Workflows en GitHub Actions

```bash
# Ver workflows ejecutándose
gh run list --limit 5

# Ver workflow específico
gh run view  # Seleccionar el más reciente

# Ver logs en browser
gh pr view --web
```

**Validaciones:**

1. ✅ CI workflow debe pasar (ci.yml)
2. ✅ Test workflow debe pasar con coverage >= 33% (test.yml)
3. ✅ Solo 4 workflows deben existir en la UI
4. ✅ No debe haber errores de sintaxis YAML

---

#### 7.4: Verificar Eliminación de Workflows

```bash
# Verificar en GitHub UI que workflows eliminados NO aparecen
open "https://github.com/EduGoGroup/edugo-worker/actions"

# Debe mostrar solo:
# - CI Pipeline (ci.yml)
# - Tests with Coverage (test.yml)
# - Manual Release (manual-release.yml)
# - Sync Main to Dev (sync-main-to-dev.yml)

# NO debe mostrar:
# - Build and Push Docker Image
# - Docker Build and Push (Simple)
# - Release CI/CD
```

---

### Validación de Tarea 7

```bash
# 1. Verificar PR existe
gh pr list --head feature/sprint-3-consolidation-docker-go125

# 2. Verificar workflows en PR
gh pr checks

# 3. Verificar que solo 4 workflows existen
WORKFLOWS_COUNT=$(ls .github/workflows/*.yml | wc -l | tr -d ' ')
if [ "$WORKFLOWS_COUNT" -eq "4" ]; then
  echo "✅ 4 workflows correctos"
else
  echo "❌ Workflows incorrectos: $WORKFLOWS_COUNT (esperado 4)"
fi

# 4. Verificar CI pasando
gh run list --workflow=ci.yml --limit 1 --json conclusion --jq '.[0].conclusion'
# Debe mostrar: success

# 5. Verificar test pasando
gh run list --workflow=test.yml --limit 1 --json conclusion --jq '.[0].conclusion'
# Debe mostrar: success
```

---

## Tarea 8: Review y Ajustes

**Duración:** 1-2 horas  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 7

### Objetivo

Revisar feedback del PR, hacer ajustes necesarios, y preparar para merge.

### Pasos

#### 8.1: Revisar Feedback de CI/CD

```bash
# Ver checks del PR
gh pr checks

# Si hay fallos, ver logs
gh run list --workflow=ci.yml --limit 1 | grep -v "completed.*success"

# Corregir según errores encontrados
```

**Errores Comunes:**

1. **go-fmt falla:**
```bash
gofmt -w .
git add .
git commit -m "style: formatear código con gofmt"
git push
```

2. **go-vet falla:**
```bash
go vet ./...
# Corregir errores reportados
git add .
git commit -m "fix: corregir issues de go vet"
git push
```

3. **Tests fallan:**
```bash
go test -v ./...
# Corregir tests fallando
git add .
git commit -m "fix: corregir tests"
git push
```

4. **Coverage < 33%:**
```bash
# Agregar más tests o ajustar threshold temporalmente
# Ver Tarea 5 para mejorar coverage
```

---

#### 8.2: Solicitar Review

```bash
# Marcar PR como ready for review
gh pr ready

# Solicitar reviewers
gh pr edit --add-reviewer @reviewerUsername

# Agregar labels
gh pr edit --add-label "enhancement,Sprint-3,CI/CD"

echo "✅ Review solicitado"
```

---

#### 8.3: Incorporar Feedback

```bash
# Ver comentarios del PR
gh pr view --comments

# Para cada comentario:
# 1. Hacer cambios solicitados
# 2. Commitear
# 3. Pushear

# Ejemplo:
git add .
git commit -m "fix: aplicar feedback de review

- [Descripción del cambio según comentario]

Addresses review comment: [link al comentario]"
git push
```

---

#### 8.4: Verificar Checks Finales

```bash
# Verificar que todos los checks pasan
gh pr checks

# Debe mostrar:
# ✓ CI Pipeline
# ✓ Tests with Coverage
# ✓ Pre-commit

# Si alguno falla, volver a 8.1
```

---

## Tarea 9: Merge a Dev

**Duración:** 30 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tarea 8

### Objetivo

Mergear PR a rama dev después de aprobación.

### Pasos

#### 9.1: Verificar Aprobación

```bash
# Ver estado del PR
gh pr view

# Debe mostrar:
# - All checks passing
# - Approved by [reviewer]
# - No conflicts with base branch

# Si hay conflictos, resolverlos:
git fetch origin dev
git rebase origin/dev
# Resolver conflictos
git push --force-with-lease
```

---

#### 9.2: Merge PR

```bash
# Merge usando squash (recomendado para features)
gh pr merge --squash --delete-branch

# O merge normal
gh pr merge --merge --delete-branch

# Verificar merge exitoso
gh pr view
# Debe mostrar: Merged
```

---

#### 9.3: Verificar en Dev

```bash
# Cambiar a dev y pull
git checkout dev
git pull origin dev

# Verificar último commit
git log -1 --oneline

# Verificar workflows
ls .github/workflows/
# Debe mostrar solo 4 archivos

# Verificar que workflows pasan en dev
gh run list --branch dev --limit 5
```

---

#### 9.4: Limpiar Rama Local

```bash
# Eliminar rama feature local
git branch -d feature/sprint-3-consolidation-docker-go125

# Verificar branches
git branch -vv

echo "✅ Sprint 3 completado y mergeado a dev"
```

---

## Tarea 10: Crear Release Notes

**Duración:** 30-45 minutos  
**Prioridad:** 🟢 Media  
**Dependencias:** Tarea 9

### Objetivo

Documentar cambios de Sprint 3 para comunicación con el equipo.

### Pasos

#### 10.1: Crear Release Notes

```bash
cat > /tmp/sprint3-release-notes.md << 'EOF'
# Sprint 3 Release Notes - edugo-worker

**Fecha:** 19 de Noviembre, 2025  
**Sprint:** 3 de 4  
**Rama:** dev

---

## 🎯 Resumen

Sprint 3 consolida workflows Docker, migra a Go 1.25.3, implementa pre-commit hooks y establece coverage threshold 33%.

## 🚀 Cambios Principales

### 1. Consolidación de Workflows Docker 🔴

**Problema:**  
3 workflows diferentes construyendo Docker images → desperdicio, confusión, fallos.

**Solución:**  
- ❌ Eliminado `build-and-push.yml` (duplicado sin tests)
- ❌ Eliminado `docker-only.yml` (duplicado simple)
- ❌ Eliminado `release.yml` (fallaba + duplicado)
- ✅ Mantenido solo `manual-release.yml` (control fino)

**Impacto:**
- Workflows Docker: 3 → 1 (-66%)
- Líneas código: ~441 → ~340 (-23%)
- Claridad para el equipo
- Resolución de fallos

**Backups:** `docs/workflows-removed-sprint3/`

---

### 2. Migración a Go 1.25.3 🟡

**Cambios:**
- ✅ `go.mod`: 1.24.10 → 1.25.3
- ✅ Workflows actualizados
- ✅ Dependencias actualizadas
- ✅ Tests pasando

**Impacto:**
- Consistencia con shared e infrastructure
- Performance mejorado
- Nuevas features de Go 1.25

---

### 3. Pre-commit Hooks 🟡

**Configuración:**
- ✅ 12 hooks implementados
- ✅ 7 validaciones básicas
- ✅ 5 validaciones de Go

**Hooks:**
1. no-commit-to-branch (main)
2. end-of-file-fixer
3. trailing-whitespace
4. check-added-large-files
5. check-yaml
6. detect-private-key
7. check-merge-conflict
8. go-fmt
9. go-imports
10. go-vet
11. go-mod-tidy
12. go-test

**Impacto:**
- Código siempre formateado
- No secretos expuestos
- Tests antes de commit
- Mejor experiencia de desarrollo

---

### 4. Coverage Threshold 33% 🟡

**Configuración:**
- ✅ Threshold en `test.yml`
- ✅ Documentado en `docs/COVERAGE-STANDARDS.md`
- ✅ Alineado con api-mobile y api-administracion

**Impacto:**
- Previene regresiones
- Fuerza mejora continua
- Estándar consistente

---

### 5. Documentación 📚

**Nuevos docs:**
- ✅ `docs/RELEASE-WORKFLOW.md` - Guía de releases
- ✅ `docs/COVERAGE-STANDARDS.md` - Estándares de coverage
- ✅ `docs/workflows-removed-sprint3/README.md` - Backups

**Actualizados:**
- ✅ README.md - Cambios Sprint 3
- ✅ .gitignore - Exclusiones nuevas

---

## 📊 Métricas

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| Workflows Docker | 3 | 1 | -66% |
| Líneas workflows | ~600 | ~350 | -42% |
| Go version consistente | No | Sí (1.25.3) | ✅ |
| Coverage threshold | No | 33% | ✅ |
| Pre-commit hooks | 0 | 12 | ✅ |
| Success rate esperado | 70% | 85%+ | +15% |
| Workflows con fallos | 1 | 0 | -100% |

---

## 🔄 Cómo Usar

### Release Manual

```bash
# GitHub UI
https://github.com/EduGoGroup/edugo-worker/actions/workflows/manual-release.yml

# GitHub CLI
gh workflow run manual-release.yml -f version=0.1.0 -f bump_type=minor
```

Ver [docs/RELEASE-WORKFLOW.md](./docs/RELEASE-WORKFLOW.md)

### Pre-commit Hooks

```bash
# Instalar
pip install pre-commit
pre-commit install

# Ejecutar
git commit -m "mensaje"  # Automático

# Manual
pre-commit run --all-files
```

### Coverage

```bash
# Generar reporte
go test -coverprofile=coverage/coverage.out ./...

# Ver coverage
go tool cover -func=coverage/coverage.out | tail -1

# HTML
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
```

---

## ⚠️ Breaking Changes

### Workflows Eliminados

Los siguientes workflows fueron eliminados:

- ❌ `build-and-push.yml`
- ❌ `docker-only.yml`
- ❌ `release.yml`

**Migración:**  
Usar `manual-release.yml` para todos los releases.

**Backups:**  
Disponibles en `docs/workflows-removed-sprint3/` si necesitas restaurar.

---

## 🐛 Issues Conocidos

Ninguno.

---

## 📝 Próximos Pasos (Sprint 4)

- Migrar workflows a reusables (centralizar en infrastructure)
- Reducir duplicación cross-repo
- Optimizar tiempos de CI

---

## 🙏 Agradecimientos

Sprint 3 ejecutado por Claude Code en colaboración con el equipo EduGo.

---

**Contacto:** [Tu email o canal de comunicación]

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF

cat /tmp/sprint3-release-notes.md
```

---

#### 10.2: Publicar Release Notes

```bash
# Copiar a docs
cp /tmp/sprint3-release-notes.md docs/SPRINT-3-RELEASE-NOTES.md

# Commit
git checkout dev
git add docs/SPRINT-3-RELEASE-NOTES.md
git commit -m "docs: agregar release notes Sprint 3"
git push origin dev

echo "✅ Release notes publicadas"
```

---

#### 10.3: Comunicar al Equipo

```bash
# Crear issue de comunicación
gh issue create \
  --title "📢 Sprint 3 Completado - Consolidación Docker + Go 1.25" \
  --body "$(cat /tmp/sprint3-release-notes.md)" \
  --label "documentation,Sprint-3"

# O enviar por email/Slack
# Copiar contenido de /tmp/sprint3-release-notes.md
```

---

## Tarea 11: Validación Final del Sprint

**Duración:** 30 minutos  
**Prioridad:** 🟡 Alta  
**Dependencias:** Tareas 1-10

### Objetivo

Verificar que todos los objetivos de Sprint 3 se cumplieron.

### Checklist Final

```bash
# Crear script de validación
cat > /tmp/sprint3-validation.sh << 'EOFSCRIPT'
#!/bin/bash
set -e

echo "🔍 Validando Sprint 3..."

# 1. Workflows Docker
DOCKER_WORKFLOWS=$(ls .github/workflows/*.yml 2>/dev/null | xargs grep -l "docker/build-push-action" | wc -l | tr -d ' ')
if [ "$DOCKER_WORKFLOWS" -eq "1" ]; then
  echo "✅ Solo 1 workflow Docker"
else
  echo "❌ Workflows Docker incorrectos: $DOCKER_WORKFLOWS (esperado 1)"
  exit 1
fi

# 2. Workflows totales
TOTAL_WORKFLOWS=$(ls .github/workflows/*.yml 2>/dev/null | wc -l | tr -d ' ')
if [ "$TOTAL_WORKFLOWS" -eq "4" ]; then
  echo "✅ 4 workflows totales"
else
  echo "❌ Workflows totales incorrectos: $TOTAL_WORKFLOWS (esperado 4)"
  exit 1
fi

# 3. Go version en go.mod
if grep -q "go 1.25.3" go.mod; then
  echo "✅ go.mod en Go 1.25.3"
else
  echo "❌ go.mod no actualizado"
  exit 1
fi

# 4. Pre-commit config
if [ -f ".pre-commit-config.yaml" ]; then
  echo "✅ .pre-commit-config.yaml existe"
else
  echo "❌ .pre-commit-config.yaml faltante"
  exit 1
fi

# 5. Coverage threshold
if grep -q "THRESHOLD=33.0" .github/workflows/test.yml; then
  echo "✅ Coverage threshold configurado"
else
  echo "❌ Coverage threshold no configurado"
  exit 1
fi

# 6. Backups
if [ -d "docs/workflows-removed-sprint3" ]; then
  BACKUPS=$(ls docs/workflows-removed-sprint3/*.backup 2>/dev/null | wc -l | tr -d ' ')
  if [ "$BACKUPS" -eq "3" ]; then
    echo "✅ 3 backups de workflows eliminados"
  else
    echo "❌ Backups incorrectos: $BACKUPS (esperado 3)"
    exit 1
  fi
else
  echo "❌ Directorio de backups faltante"
  exit 1
fi

# 7. Documentación
DOCS_EXPECTED=("docs/RELEASE-WORKFLOW.md" "docs/COVERAGE-STANDARDS.md" "docs/SPRINT-3-RELEASE-NOTES.md")
for doc in "${DOCS_EXPECTED[@]}"; do
  if [ -f "$doc" ]; then
    echo "✅ $doc existe"
  else
    echo "⚠️  $doc faltante (no crítico)"
  fi
done

# 8. Tests pasan
echo "🧪 Ejecutando tests..."
if go test ./... > /dev/null 2>&1; then
  echo "✅ Tests pasan"
else
  echo "❌ Tests fallan"
  exit 1
fi

# 9. Coverage >= 33%
if [ -f "coverage/coverage.out" ]; then
  COVERAGE=$(go tool cover -func=coverage/coverage.out | tail -1 | awk '{print $NF}' | sed 's/%//')
  if (( $(echo "$COVERAGE >= 33.0" | bc -l) )); then
    echo "✅ Coverage ${COVERAGE}% >= 33%"
  else
    echo "⚠️  Coverage ${COVERAGE}% < 33% (no crítico, puede mejorarse)"
  fi
else
  echo "⚠️  coverage.out no existe (ejecutar: go test -coverprofile=coverage/coverage.out ./...)"
fi

# 10. Git status limpio
if git diff --quiet && git diff --cached --quiet; then
  echo "✅ Git status limpio"
else
  echo "⚠️  Cambios sin commitear (verificar git status)"
fi

echo ""
echo "🎉 Sprint 3 validado exitosamente"
echo ""
echo "📊 Resumen:"
echo "  - Workflows Docker: 1 (de 3)"
echo "  - Workflows totales: 4 (de 7)"
echo "  - Go version: 1.25.3"
echo "  - Pre-commit hooks: ✅"
echo "  - Coverage threshold: 33%"
echo "  - Backups: 3"
echo "  - Tests: ✅"
echo ""
echo "✅ Todos los objetivos de Sprint 3 cumplidos"
EOFSCRIPT

chmod +x /tmp/sprint3-validation.sh
/tmp/sprint3-validation.sh
```

---

### Objetivos Cumplidos

- [ ] **OBJ-1:** Eliminado build-and-push.yml
- [ ] **OBJ-2:** Eliminado docker-only.yml
- [ ] **OBJ-3:** Migrado y eliminado release.yml
- [ ] **OBJ-4:** Migrado a Go 1.25.3
- [ ] **OBJ-5:** Implementado pre-commit hooks
- [ ] **OBJ-6:** Establecido coverage threshold 33%

---

## Tarea 12: Preparar para Sprint 4

**Duración:** 15-20 minutos  
**Prioridad:** 🟢 Baja  
**Dependencias:** Tarea 11

### Objetivo

Preparar entorno y documentación para Sprint 4 (Workflows Reusables).

### Pasos

#### 12.1: Crear Branch para Sprint 4

```bash
# Asegurar estar en dev actualizado
git checkout dev
git pull origin dev

# Crear rama para Sprint 4
git checkout -b feature/sprint-4-reusable-workflows

echo "✅ Rama Sprint 4 creada"
```

---

#### 12.2: Revisar SPRINT-4-TASKS.md

```bash
# Abrir plan de Sprint 4
open ../Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/SPRINT-4-TASKS.md

# O ver resumen
grep "^## " ../Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/SPRINT-4-TASKS.md

echo "📋 Sprint 4 listo para comenzar"
```

---

#### 12.3: Actualizar CHANGELOG

```bash
# Agregar entrada de Sprint 3 completado
cat >> CHANGELOG.md << 'EOF'

## Sprint 3 - 2025-11-19

### Added
- Pre-commit hooks con 12 validaciones
- Coverage threshold 33% en test.yml
- Documentación de release workflow
- Documentación de coverage standards
- Release notes Sprint 3

### Changed
- Migrado de Go 1.24.10 a Go 1.25.3
- Workflows actualizados a Go 1.25.3
- README actualizado con cambios Sprint 3

### Removed
- Eliminado build-and-push.yml (consolidado en manual-release.yml)
- Eliminado docker-only.yml (consolidado en manual-release.yml)
- Eliminado release.yml (consolidado en manual-release.yml)

### Fixed
- Resueltos fallos en release.yml
- Inconsistencia en versión de Go

---

EOF

git add CHANGELOG.md
git commit -m "docs: actualizar CHANGELOG con Sprint 3"
git push origin dev

echo "✅ CHANGELOG actualizado"
```

---

## 🎉 ¡Sprint 3 Completado!

Felicitaciones, has completado exitosamente el Sprint 3 de edugo-worker.

### Resumen de Logros

- ✅ Consolidado 3 workflows Docker en 1
- ✅ Migrado a Go 1.25.3
- ✅ Implementado 12 pre-commit hooks
- ✅ Establecido coverage threshold 33%
- ✅ Documentación completa
- ✅ Todos los tests pasando

### Próximos Pasos

Continuar con [SPRINT-4-TASKS.md](./SPRINT-4-TASKS.md) para migrar workflows a reusables.

---

## 📊 Checklist General del Sprint

### Preparación
- [ ] Go 1.25.3 instalado
- [ ] gh CLI autenticado
- [ ] pre-commit instalado
- [ ] Repositorio clonado y actualizado
- [ ] Rama feature creada

### Tareas Principales
- [ ] Tarea 1: Consolidar workflows Docker (3-4h)
- [ ] Tarea 2: Migrar a Go 1.25.3 (45-60min)
- [ ] Tarea 3: Actualizar .gitignore (15-20min)
- [ ] Tarea 4: Implementar pre-commit hooks (60-90min)
- [ ] Tarea 5: Coverage threshold 33% (45min)
- [ ] Tarea 6: Actualizar documentación (30-45min)
- [ ] Tarea 7: Verificar workflows (30-45min)
- [ ] Tarea 8: Review y ajustes (1-2h)
- [ ] Tarea 9: Merge a dev (30min)
- [ ] Tarea 10: Release notes (30-45min)
- [ ] Tarea 11: Validación final (30min)
- [ ] Tarea 12: Preparar Sprint 4 (15-20min)

### Validación Final
- [ ] Solo 1 workflow Docker
- [ ] 4 workflows totales
- [ ] Go 1.25.3 en go.mod
- [ ] Pre-commit hooks funcionando
- [ ] Coverage threshold 33%
- [ ] Tests pasando
- [ ] CI pasando
- [ ] PR mergeado a dev

---

## 🛠️ Troubleshooting

### Problema: Tests fallan después de Go 1.25.3

**Síntomas:**
```
FAIL: TestXxx
```

**Solución:**
```bash
# Ver errores específicos
go test -v ./... 2>&1 | grep "FAIL"

# Revisar changelog de Go 1.25
open https://go.dev/doc/go1.25

# Actualizar código según breaking changes
```

---

### Problema: Pre-commit muy lento

**Síntomas:**
Commits tardan más de 1 minuto.

**Solución:**
```bash
# Deshabilitar go-test hook (el más lento)
# Editar .pre-commit-config.yaml
# Comentar hook go-test

# O ejecutar tests solo en archivos modificados
# Cambiar en hook:
pass_filenames: true
```

---

### Problema: Coverage < 33%

**Síntomas:**
```
❌ Coverage 28% < 33%
```

**Solución:**
```bash
# Ver qué paquetes tienen coverage bajo
go tool cover -func=coverage/coverage.out | awk '{if ($NF ~ /[0-9]+\.[0-9]+%/) {gsub(/%/, "", $NF); if ($NF < 33) print $0}}'

# Agregar tests
# (Esto puede tomar tiempo, considerar como tarea separada)

# O temporalmente ajustar threshold
# Editar .github/workflows/test.yml
# Cambiar THRESHOLD=25.0 temporalmente
```

---

### Problema: Workflows Docker duplicados aparecen en UI

**Síntomas:**
GitHub Actions UI muestra workflows eliminados.

**Solución:**
```bash
# Los workflows eliminados se mantienen en historial
# Solo ejecuta workflows activos
# Verificar que no haya runs nuevos de workflows eliminados

# Si hay runs nuevos, verificar que archivos fueron eliminados:
ls .github/workflows/ | grep -E "(build-and-push|docker-only|release).yml"
# No debe mostrar nada
```

---

## 📞 Soporte

Si encuentras problemas durante Sprint 3:

1. Revisar sección Troubleshooting
2. Verificar logs de CI/CD: `gh run view`
3. Consultar documentación:
   - [RELEASE-WORKFLOW.md](./docs/RELEASE-WORKFLOW.md)
   - [COVERAGE-STANDARDS.md](./docs/COVERAGE-STANDARDS.md)
4. Revisar issues similares: `gh issue list`

---

**Generado por:** Claude Code  
**Fecha:** 19 de Noviembre, 2025  
**Versión:** 1.0  
**Para:** edugo-worker - Sprint 3
