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

El workflow ejecuta los siguientes pasos automáticamente:

1. ✅ **Valida versión semver** - Verifica formato correcto (X.Y.Z)
2. ✅ **Actualiza version.txt** - Guarda nueva versión
3. ✅ **Genera entrada de CHANGELOG** - Extrae commits desde último tag
4. ✅ **Commit a main** - Sube cambios de versión
5. ✅ **Crea y pushea tag** - Crea tag vX.Y.Z
6. ✅ **Ejecuta tests completos** - go test -v -race ./...
7. ✅ **Build Docker multi-platform** - linux/amd64 + linux/arm64
8. ✅ **Push Docker a GHCR** - ghcr.io/edugogroup/edugo-worker
9. ✅ **Crea GitHub Release** - Con changelog automático

### Variables de Salida

- **Tag creado:** `v{version}`
- **Docker image:** `ghcr.io/edugogroup/edugo-worker:v{version}`
- **Docker image (versión):** `ghcr.io/edugogroup/edugo-worker:{version}`
- **Docker image (latest):** `ghcr.io/edugogroup/edugo-worker:latest`
- **GitHub Release:** `https://github.com/EduGoGroup/edugo-worker/releases/tag/v{version}`

### Bump Types

| Tipo | Ejemplo | Uso | Cuándo usar |
|------|---------|-----|-------------|
| **patch** | 0.0.1 → 0.0.2 | Bugfixes, hotfixes | Correcciones de bugs sin cambios de API |
| **minor** | 0.0.1 → 0.1.0 | Nuevas features (no breaking) | Nuevas funcionalidades compatibles |
| **major** | 0.0.1 → 1.0.0 | Breaking changes o producción | Cambios incompatibles o primera versión estable |

### Arquitecturas Soportadas

La imagen Docker se construye para múltiples plataformas:

- **linux/amd64** - Servidores x86-64 (AWS EC2, GCP, Azure, etc.)
- **linux/arm64** - ARM 64-bit (AWS Graviton, Apple Silicon, Raspberry Pi 4/5)

### Verificación Post-Release

```bash
# 1. Verificar tag creado
git fetch --tags
git tag -l "v*" | tail -5

# 2. Verificar Docker images
docker pull ghcr.io/edugogroup/edugo-worker:v0.1.0
docker pull ghcr.io/edugogroup/edugo-worker:latest

# 3. Verificar arquitecturas
docker manifest inspect ghcr.io/edugogroup/edugo-worker:v0.1.0 | grep -A 2 "platform"

# 4. Verificar GitHub Release
gh release view v0.1.0

# 5. Verificar CHANGELOG
cat CHANGELOG.md | head -50

# 6. Verificar version.txt
cat .github/version.txt
```

### Troubleshooting

#### Error: Tag already exists

```bash
# Eliminar tag localmente y remotamente
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0

# Volver a ejecutar workflow
gh workflow run manual-release.yml -f version=0.1.0 -f bump_type=minor
```

#### Error: Tests failing

```bash
# Ejecutar tests localmente
go test -v ./...

# Corregir tests y hacer commit
git add .
git commit -m "fix: corregir tests"
git push origin main

# Volver a ejecutar workflow
```

#### Error: Docker build failing

```bash
# Verificar Dockerfile localmente
docker build -t edugo-worker:test .

# Si falla, corregir Dockerfile y commit
git add Dockerfile
git commit -m "fix: corregir Dockerfile"
git push origin main
```

#### Error: Permission denied to create release

```bash
# Verificar que APP_ID y APP_PRIVATE_KEY están configurados
gh secret list

# Si no están, contactar administrador para configurar GitHub App
```

### Workflows Antiguos (Eliminados)

Los siguientes workflows fueron eliminados en Sprint 3 (2025-11-22):

- ❌ `build-and-push.yml` - Duplicado sin tests
- ❌ `docker-only.yml` - Duplicado simple
- ❌ `release.yml` - Fallaba + duplicado

**Razón:** Consolidación en manual-release.yml para:
- Eliminar duplicación (-75% workflows Docker)
- Control fino sobre releases
- Tests completos antes de build
- CHANGELOG automático
- Mejor trazabilidad

**Backups disponibles en:** `docs/workflows-removed-sprint3/`

### Workflow CI vs Release

| Aspecto | ci.yml | manual-release.yml |
|---------|--------|-------------------|
| **Propósito** | Validación continua | Releases oficiales |
| **Trigger** | Pull Request + Push | Manual |
| **Docker Build** | ✅ (local) | ✅ (push a registry) |
| **Docker Push** | ❌ No pushea | ✅ Pushea a GHCR |
| **Tests** | ✅ | ✅ |
| **Multi-platform** | ❌ Solo amd64 | ✅ amd64 + arm64 |
| **GitHub Release** | ❌ | ✅ |
| **CHANGELOG** | ❌ | ✅ |
| **Tags** | ❌ | ✅ |

### Mejores Prácticas

1. **Siempre usar main:** Ejecutar releases desde rama `main`
2. **Tests antes:** Asegurar que CI pasa antes de release
3. **Versión correcta:** Seguir semver estrictamente
4. **CHANGELOG:** Revisar entrada generada, editar si es necesario
5. **Comunicar:** Notificar al equipo después de release
6. **Verificar:** Validar que Docker image funciona antes de desplegar

### Integración con Otros Workflows

#### sync-main-to-dev.yml

Después de un release, `sync-main-to-dev.yml` se ejecuta automáticamente para sincronizar cambios de `main` a `dev`.

**Por qué funciona:** manual-release.yml usa GitHub App Token en lugar de GITHUB_TOKEN, lo que permite disparar workflows subsecuentes.

#### test.yml

`test.yml` ejecuta tests con coverage antes de merge a `main`. Asegura que código en `main` siempre tenga tests pasando.

**Threshold:** Coverage mínimo 33%

### Rollback

Si necesitas hacer rollback de un release:

```bash
# 1. Identificar versión anterior
git tag -l "v*" | tail -10

# 2. Crear hotfix desde versión anterior
git checkout v0.0.9
git checkout -b hotfix/rollback-v0.1.0

# 3. Hacer cambios necesarios
# ... editar código ...

# 4. Crear nuevo release (patch)
git commit -am "fix: rollback cambios de v0.1.0"
git push origin hotfix/rollback-v0.1.0

# 5. Crear PR a main
gh pr create --base main --head hotfix/rollback-v0.1.0 \
  --title "Rollback v0.1.0" \
  --body "Rollback de cambios problemáticos en v0.1.0"

# 6. Después de merge, ejecutar release v0.1.1
gh workflow run manual-release.yml -f version=0.1.1 -f bump_type=patch
```

### Monitoreo de Releases

#### GitHub Actions

```bash
# Ver últimos runs de manual-release
gh run list --workflow=manual-release.yml --limit 10

# Ver detalles de un run específico
gh run view <run-id>

# Ver logs de un run
gh run view <run-id> --log
```

#### Docker Registry

```bash
# Ver imágenes en GHCR
gh api /orgs/EduGoGroup/packages/container/edugo-worker/versions | jq '.[] | {name, created_at}'

# Ver tags
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://ghcr.io/v2/edugogroup/edugo-worker/tags/list | jq
```

#### GitHub Releases

```bash
# Listar releases
gh release list

# Ver release específico
gh release view v0.1.0

# Descargar assets de release
gh release download v0.1.0
```

### Preguntas Frecuentes

**P: ¿Puedo ejecutar releases desde rama dev?**
R: No, solo desde `main`. dev es para desarrollo activo.

**P: ¿Qué pasa si el workflow falla a mitad?**
R: Los jobs son independientes. Si falla en build Docker, el tag ya fue creado. Deberás eliminarlo manualmente y volver a ejecutar.

**P: ¿Puedo saltar la versión? (ej: 0.0.1 → 0.0.5)**
R: Sí, el workflow acepta cualquier versión válida semver. Pero sigue buenas prácticas de versionado.

**P: ¿Cuánto tarda un release?**
R: Aproximadamente 5-10 minutos:
- Create release: 1-2 min
- Build and test: 2-3 min
- Build Docker: 2-4 min
- Create GitHub Release: 1 min

**P: ¿Se puede automatizar releases?**
R: Sí, pero no se recomienda. Control manual evita releases accidentales y permite validación humana.

**P: ¿Los workflows antiguos seguirán apareciendo en UI?**
R: Sí, en historial. Pero no se ejecutarán porque los archivos fueron eliminados.

---

## Referencias

- [Semantic Versioning](https://semver.org/)
- [GitHub Actions workflow_dispatch](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#workflow_dispatch)
- [Docker Multi-platform builds](https://docs.docker.com/build/building/multi-platform/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

---

**Última actualización:** 2025-11-22 - Sprint 3
**Responsable:** Equipo DevOps EduGo
**Contacto:** [Canal de soporte]
