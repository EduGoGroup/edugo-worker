# 🔄 Workflows de CI/CD - edugo-api-mobile

## 🎯 Estrategia de Ejecución por Branch

Esta tabla muestra **qué workflows se ejecutan en cada tipo de branch** para evitar ejecuciones innecesarias y notificaciones de falsos positivos:

| Workflow | feature/* | main | PR a main/dev | Tags v* | Manual |
|----------|-----------|------|---------------|---------|--------|
| **ci.yml** | ❌ | ✅ | ✅ | ❌ | ❌ |
| **manual-release.yml** | ❌ | ❌ | ❌ | ❌ | ✅ (RECOMENDADO) |
| **docker-only.yml** | ❌ | ❌ | ❌ | ❌ | ✅ |
| **release.yml** | ❌ | ❌ | ❌ | ✅ | ❌ |
| **sync-main-to-dev.yml** | ❌ | ✅ | ❌ | ✅ | ❌ |

### 📌 Resumen por Escenario

```bash
# Push a feature/* → SIN workflows automáticos
git push origin feature/mi-feature
# ✅ Sin ejecuciones, sin notificaciones

# Crear PR desde feature/* → CI completo
gh pr create --base main --head feature/mi-feature
# ✅ ci.yml (tests, linter, security)
# ✅ Copilot code review

# Merge PR a main → Solo CI
# ✅ ci.yml se ejecuta
# ⏸️ Espera a crear release manualmente

# Crear release manualmente (RECOMENDADO)
# Actions → Manual Release → Run workflow
#   - Versión: 0.1.0
#   - Tipo: minor
# ✅ manual-release.yml (actualiza version.txt, CHANGELOG, crea tag)
# ✅ release.yml (build Docker, GitHub Release) - AUTOMÁTICO
# ✅ sync-main-to-dev.yml (sincroniza con dev) - AUTOMÁTICO

# Build manual de Docker → Usar workflow_dispatch
# Actions → Docker Build and Push → Run workflow
# ✅ docker-only.yml (solo cuando lo necesites)
```

### ⚠️ Nota sobre GitHub Actions

GitHub Actions **evalúa** todos los workflows en cualquier evento, pero solo **ejecuta** los que cumplen las condiciones de trigger. Esto es comportamiento normal de GitHub y no indica un error.

---

## 🤖 Configuración: GitHub App para Sincronización Automática

Los workflows `manual-release.yml` y `sync-main-to-dev.yml` utilizan una **GitHub App** en lugar de `GITHUB_TOKEN` para poder disparar workflows subsecuentes.

### ¿Por qué GitHub App?

**Problema con GITHUB_TOKEN**:
- ❌ Los commits realizados con `GITHUB_TOKEN` NO disparan workflows automáticamente
- ❌ Esto es una limitación de seguridad de GitHub Actions
- ❌ Sin esto, `sync-main-to-dev.yml` nunca se ejecutaba después de `manual-release.yml`

**Solución con GitHub App**:
- ✅ Los commits con App Token SÍ disparan workflows
- ✅ Sincronización automática de main → dev funciona
- ✅ Permisos granulares y seguros
- ✅ Tokens expiran automáticamente

### Secretos Requeridos

A nivel de **organización** (EduGoGroup):

| Secret | Valor | Descripción |
|--------|-------|-------------|
| `APP_ID` | Número (ej: 123456) | ID de la GitHub App |
| `APP_PRIVATE_KEY` | Contenido del .pem | Private key de la App |

### Cómo Crear la GitHub App

1. **Crear App**:
   - Settings → Developer settings → GitHub Apps → **New GitHub App**
   - Name: `EduGo Sync Bot` (o cualquier nombre)
   - Homepage URL: `https://github.com/EduGoGroup`
   - Webhook: Desactivar (no necesario)

2. **Configurar Permisos**:
   ```
   Repository permissions:
   - Contents: Read and write ✅
   - Workflows: Read and write ✅
   - Metadata: Read-only (automático)
   ```

3. **Generar Private Key**:
   - Scroll a sección "Private keys"
   - Click **"Generate a private key"**
   - Se descarga archivo `.pem` automáticamente
   - **Guardar en lugar seguro** (se necesita para configurar secretos)

4. **Instalar la App**:
   - Click "Install App" (lado izquierdo)
   - Seleccionar organización: **EduGoGroup**
   - Repository access: **Selected repositories**
     - Seleccionar: edugo-api-mobile, edugo-shared, edugo-worker, edugo-api-administracion
   - Click **"Install"**

5. **Configurar Secretos**:
   - Ir a: https://github.com/organizations/EduGoGroup/settings/secrets/actions
   - Click **"New organization secret"**

   **Secreto 1 - APP_ID**:
   - Name: `APP_ID`
   - Secret: `123456` (el App ID visible en la página de la App)
   - Repository access: **Selected repositories** (los 4 repos)

   **Secreto 2 - APP_PRIVATE_KEY**:
   - Name: `APP_PRIVATE_KEY`
   - Secret: Abrir el .pem en editor y copiar TODO el contenido
   ```
   -----BEGIN RSA PRIVATE KEY-----
   MIIEpAIBAAKCAQEA...
   ...
   -----END RSA PRIVATE KEY-----
   ```
   - Repository access: **Selected repositories** (los 4 repos)

6. **Verificar**:
   - Los 4 repos deben tener acceso a ambos secretos
   - La App debe estar instalada en los 4 repos
   - Los permisos deben estar correctos

### Uso en Workflows

```yaml
- name: Generar token desde GitHub App
  id: generate_token
  uses: actions/create-github-app-token@v1
  with:
    app-id: ${{ secrets.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}

- name: Checkout con App Token
  uses: actions/checkout@v4
  with:
    token: ${{ steps.generate_token.outputs.token }}
```

**Beneficio**: Los commits realizados con este token SÍ disparan workflows subsecuentes.

---

## 📋 Workflows Configurados

### 1️⃣ **ci.yml** - Pipeline de Integración Continua

**Trigger:**
- ✅ Pull Requests a `main` o `develop`
- ✅ Push directo a `main` (red de seguridad)

**Ejecuta:**
- ✅ Verificación de formato (gofmt)
- ✅ Verificación de go.mod y go.sum sincronizados
- ✅ Análisis estático (go vet)
- ✅ Tests con race detection
- ✅ Build verification
- ✅ Verificación de Swagger docs
- ✅ Linter (opcional, no bloquea)
- ✅ Security scan con gosec

**Cuándo se ejecuta:**
```bash
# Cuando creas un PR
gh pr create --title "..." --body "..."  # ← AQUÍ se ejecuta

# O cuando alguien hace push directo a main (no recomendado)
git push origin main  # ← AQUÍ se ejecuta
```

**Duración estimada:** 3-4 minutos

---

### 2️⃣ **test.yml** - BORRADO (plan 054, S.2)

`test.yml` («Tests with Coverage») se borró el 2026-11-14. Terminaba en `startup_failure` en cada PR
(bug 0088) y, aun arreglado, duplicaba a `ci.yml`: corría `go test -short -race` con umbral de cobertura
`0.0`, cuando `ci.yml` ya corre `go test -race ./...` completo. La cobertura se mide en local con
`make test-coverage`, y la integración es un **gate local obligatorio** antes de cada PR
(`make test-integration`, con Docker): no corre en CI por decisión del dueño.

---

### 3️⃣ **manual-release.yml** - Crear Release Manual ⭐ RECOMENDADO

**Trigger:**
- ✅ Manual únicamente (workflow_dispatch)

**Ejecuta:**
- ✅ Validación de formato de versión (semver)
- ✅ Verificación de que el tag no exista
- ✅ Actualización de `.github/version.txt`
- ✅ Generación automática de entrada en CHANGELOG.md
- ✅ Commit de cambios de versión a main
- ✅ Creación y push de tag (dispara release.yml automáticamente)

**Cómo usarlo:**
```bash
# Desde GitHub UI:
# 1. Ir a: Actions → Manual Release → Run workflow
# 2. Seleccionar branch: main
# 3. Ingresar versión: 0.1.0 (sin 'v')
# 4. Seleccionar tipo: patch / minor / major
# 5. Click "Run workflow"

# El workflow automáticamente:
# - Actualiza version.txt
# - Actualiza CHANGELOG.md
# - Crea commit en main
# - Crea tag v0.1.0
# - Dispara release.yml (que construye Docker)
```

**Inputs requeridos:**
- `version`: Versión a crear (formato: 0.1.0)
- `bump_type`: patch (bugfix) / minor (feature) / major (breaking/producción)

**Qué dispara automáticamente:**
- ✅ **release.yml** → Build Docker + GitHub Release
- ✅ **sync-main-to-dev.yml** → Sincroniza cambios a dev

**Duración estimada:** 1 minuto (luego dispara release.yml)

**Ventajas:**
- ✅ Control total sobre cuándo y qué versión crear
- ✅ No depende de auto-version inestable
- ✅ Proceso predecible y auditable
- ✅ Actualiza CHANGELOG automáticamente
- ✅ Dispara release completo (Docker + GitHub Release)

---

### 4️⃣ **build-and-push.yml** - Build y Push de Docker

**Trigger:**
- ✅ Manual (workflow_dispatch con selección de ambiente)
- ✅ Push a `main` (automático)

**Ejecuta:**
- ✅ Tests antes del build
- ✅ Build de imagen Docker
- ✅ Push a GitHub Container Registry (ghcr.io)
- ✅ Tags automáticos (latest, branch, sha, environment)
- ✅ Resumen detallado del deployment

**Cuándo se ejecuta:**
```bash
# Automático cuando haces push a main
git push origin main  # ← AQUÍ se ejecuta

# Manual desde GitHub UI con selección de ambiente:
# Actions → Build and Push Docker Image → Run workflow
# Seleccionar: development, staging, o production
```

**Tags generados:**
- `latest` - Último build de main
- `main-<sha>` - Build específico por commit
- `<environment>` - Tag del ambiente seleccionado (manual)
- `<environment>-YYYYMMDD-HHmmss` - Tag con timestamp (manual)

**Duración estimada:** 5-8 minutos

---

### 4️⃣ **release.yml** - Release Completo (TAGS)

**Trigger:** Solo cuando creas un tag `v*` (ej: `v1.0.0`, `v2.1.3`)

**Ejecuta:**
- ✅ Validación completa del código
- ✅ Tests con cobertura
- ✅ Build de imagen Docker con tags versionados
- ✅ Creación automática de GitHub Release
- ✅ Generación de changelog desde commits o CHANGELOG.md
- ✅ Documentación de deployment en el release

**Cuándo se ejecuta:**
```bash
# Crear y pushear tag
git tag -a v1.0.0 -m "Release 1.0.0: Primera versión estable"
git push origin v1.0.0  # ← AQUÍ se ejecuta
```

**Tags Docker generados:**
- `v1.0.0` - Versión semántica completa
- `v1.0` - Major.Minor
- `v1` - Major
- `latest` - Última versión
- `v1.0.0-<sha>` - Con commit hash

**Duración estimada:** 6-10 minutos

---

## 🎯 Estrategia de CI/CD Optimizada

### **Flujo Normal de Desarrollo:**

```
┌─────────────────────────────────────────────────────────────┐
│  1. Desarrollo Local                                        │
│     - Hacer cambios en tu branch                           │
│     - Ejecutar tests localmente: go test ./...             │
│     - Verificar formato: gofmt -w .                        │
│     - git commit                                            │
│     ✅ NO GASTA MINUTOS DE GITHUB                           │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  2. Crear Pull Request                                      │
│     - gh pr create                                          │
│     - CI automático (ci.yml)                                │
│     - Revisar resultados y cobertura                        │
│     - Aprobar y mergear                                     │
│     ✅ VALIDA ANTES DE MERGE                                │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Merge a Main                                            │
│     - gh pr merge                                           │
│     - CI de seguridad (ci.yml)                             │
│     - Build automático de imagen Docker                     │
│     ✅ CÓDIGO VALIDADO + IMAGEN EN GHCR                     │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Crear Release (cuando estés listo)                     │
│     - Actualizar CHANGELOG.md (opcional)                    │
│     - git tag -a v1.2.0 -m "Release 1.2.0"                  │
│     - git push origin v1.2.0                                │
│     - Release automático (release.yml)                      │
│     - Imagen Docker con tags versionados                    │
│     ✅ RELEASE COMPLETO CON DOCUMENTACIÓN                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 🐳 Gestión de Imágenes Docker

### **Después de cada push a main:**
```bash
# La imagen se publica automáticamente como:
docker pull ghcr.io/edugogroup/edugo-api-mobile:latest
docker pull ghcr.io/edugogroup/edugo-api-mobile:main-abc1234
```

### **Cuando creas un release (tag):**
```bash
# Se publican múltiples tags versionados:
docker pull ghcr.io/edugogroup/edugo-api-mobile:v1.2.0
docker pull ghcr.io/edugogroup/edugo-api-mobile:v1.2
docker pull ghcr.io/edugogroup/edugo-api-mobile:v1
docker pull ghcr.io/edugogroup/edugo-api-mobile:latest
```

### **Deploy manual de ambiente específico:**
```bash
# Desde GitHub UI: Actions → Build and Push → Run workflow
# Seleccionar ambiente: production

# Resultado:
docker pull ghcr.io/edugogroup/edugo-api-mobile:production
docker pull ghcr.io/edugogroup/edugo-api-mobile:production-20251031-143000
```

---

## 💰 Ahorro de Minutos de GitHub Actions

### **Estrategia Optimizada:**

| Escenario | Workflows Ejecutados | Minutos Estimados |
|-----------|---------------------|-------------------|
| Push a branch feature | 0 (no ejecuta nada) | 0 min |
| Crear PR | ci.yml | ~4 min |
| Merge a main | ci.yml + build-and-push.yml | ~12 min |
| Crear release (tag) | release.yml | ~10 min |

**Mes típico (10 PRs, 3 releases):**
- 10 PRs = 80 minutos
- 10 merges a main = 120 minutos
- 3 releases = 30 minutos
- **Total = 230 minutos/mes** (✅ Solo ~10% del plan gratuito de 2,000 min)

---

## 🚀 Guía Rápida

### **Para desarrollo normal:**
```bash
# 1. Crear branch de feature
git checkout -b feature/nueva-funcionalidad

# 2. Desarrollar y probar localmente
go test ./...
gofmt -w .

# 3. Commit y push
git commit -m "feat: nueva funcionalidad"
git push origin feature/nueva-funcionalidad

# 4. Crear PR (ejecuta ci.yml automáticamente)
gh pr create --title "Nueva funcionalidad" --body "..."

# 5. Esperar aprobación y merge
# Al hacer merge, se ejecuta automáticamente build-and-push.yml
```

### **Para crear una release:**
```bash
# 1. Asegurarse de estar en main actualizado
git checkout main
git pull origin main

# 2. Actualizar CHANGELOG.md (opcional pero recomendado)
vim CHANGELOG.md
git add CHANGELOG.md
git commit -m "chore: actualizar changelog para v1.2.0"
git push origin main

# 3. Crear y pushear tag (ejecuta release.yml automáticamente)
git tag -a v1.2.0 -m "Release 1.2.0: Nuevas funcionalidades X, Y, Z"
git push origin v1.2.0

# 4. GitHub Actions:
#    - Valida todo el código
#    - Ejecuta tests
#    - Construye imagen Docker
#    - Crea GitHub Release automáticamente
#    - Publica documentación
```

### **Para deploy manual a un ambiente:**
```bash
# Opción 1: Desde GitHub UI
# 1. Ir a Actions → Build and Push Docker Image
# 2. Click en "Run workflow"
# 3. Seleccionar ambiente (development/staging/production)
# 4. Click "Run workflow"

# Opción 2: Desde CLI con gh
gh workflow run build-and-push.yml -f environment=production
```

---

## 📊 Monitoreo de Workflows

### **Ver estado de workflows:**
```bash
# Listar últimos workflows ejecutados
gh run list --limit 10

# Ver detalles de un workflow específico
gh run view <run-id>

# Ver logs de un workflow
gh run view <run-id> --log

# Re-ejecutar un workflow fallido
gh run rerun <run-id>

# Ver workflows en ejecución
gh run watch
```

### **Ver imagen Docker publicada:**
```bash
# Autenticarse en GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Ver tags disponibles
gh api /orgs/EduGoGroup/packages/container/edugo-api-mobile/versions

# Pull de la imagen
docker pull ghcr.io/edugogroup/edugo-api-mobile:latest
```

---

## 🛡️ Branch Protection (Recomendado)

Para forzar el uso de PRs y garantizar calidad:

1. GitHub → Settings → Branches → Add rule
2. Branch name pattern: `main`
3. Configurar:
   - ✅ Require pull request before merging
   - ✅ Require approvals: 1
   - ✅ Require status checks to pass:
     - `Tests and Validation`
   - ✅ Require branches to be up to date
   - ✅ Do not allow bypassing the above settings

---

## 🔍 Troubleshooting

### **Error: "GOPRIVATE no configurado"**
```bash
# Asegúrate de que el workflow tiene acceso a repos privados
# Ya está configurado en los workflows con:
git config --global url."https://${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
```

### **Error: "No se puede pushear imagen Docker"**
```bash
# Verifica permisos del workflow
# Los workflows necesitan: permissions.packages: write
# Ya está configurado en build-and-push.yml y release.yml
```

### **Workflow no se ejecuta en tag:**
```bash
# Asegúrate de que el tag tenga el prefijo 'v'
git tag v1.0.0  # ✅ Correcto
git tag 1.0.0   # ❌ No ejecutará release.yml

# Push del tag
git push origin v1.0.0
```

---

## 🤖 GitHub Copilot - Code Review Automático

Este repositorio incluye **instrucciones personalizadas para GitHub Copilot** que mejoran:
- ✅ Sugerencias de código en tu IDE
- ✅ Code reviews automáticos en Pull Requests
- ✅ Comentarios contextuales según tu arquitectura

### 📄 Archivo de Configuración

**Ubicación:** `.github/copilot-instructions.md`

Este archivo contiene:
- Arquitectura del proyecto (Clean Architecture)
- Convenciones de código y naming
- Reglas de uso de `edugo-shared`
- TODOs y deuda técnica conocida
- **Configuración de idioma:** Todos los comentarios en español

### 🎯 Copilot en Pull Requests

Cuando creas un PR, Copilot **automáticamente**:

1. **Analiza el código** según las instrucciones personalizadas
2. **Genera comentarios** sobre mejoras, bugs potenciales, o mejores prácticas
3. **Reporta cobertura** de tests (si está configurado)
4. **Sugiere implementaciones** alineadas con tu arquitectura

**Ejemplo de comentario de Copilot:**
```
⚠️ Considera usar errors.NewValidationError() de edugo-shared
en lugar de fmt.Errorf() para mantener consistencia con la
arquitectura del proyecto.
```

### 📝 Actualizar Instrucciones

Si cambias patrones arquitectónicos o agregas nuevas convenciones:

```bash
# Editar instrucciones
vim .github/copilot-instructions.md

# Commit
git add .github/copilot-instructions.md
git commit -m "docs: actualizar instrucciones de Copilot"

# Las nuevas instrucciones se aplicarán en el próximo PR
```

### 📖 Más Información

- [Documentación oficial de Copilot Custom Instructions](https://docs.github.com/en/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot)
- [Archivo de instrucciones actual](../copilot-instructions.md)

---

## 📚 Recursos Adicionales

- [Documentación de GitHub Actions](https://docs.github.com/en/actions)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Codecov Documentation](https://docs.codecov.com/)
- [Guía de Migración edugo-shared](../../MIGRACION_EDUGO_SHARED_V2.0.5.md)

---

## 📝 Checklist para Nuevos Proyectos

Si vas a replicar estos workflows en otros proyectos:

- [ ] Copiar los 4 archivos de workflows
- [ ] Actualizar `GO_VERSION` a la versión de Go del proyecto
- [ ] Actualizar `IMAGE_NAME` si es necesario
- [ ] Verificar que existe Swagger (o comentar esa sección)
- [ ] Configurar branch protection en GitHub
- [ ] Hacer un PR de prueba para validar ci.yml
- [ ] Crear un tag de prueba para validar release.yml
- [ ] Documentar workflows específicos del proyecto

---

**Última actualización:** 2025-11-01
**Mantenedor:** Equipo EduGo
**Proyecto:** edugo-api-mobile
