# Workflows Reusables - edugo-worker

**Sprint:** SPRINT-4
**Fecha:** 2025-11-22
**Estado:** FASE 1 - Implementación con Stubs

---

## 🎯 ¿Qué son Workflows Reusables?

Los **workflows reusables** son workflows de GitHub Actions centralizados en `edugo-infrastructure` y reutilizados desde múltiples repositorios (api-mobile, api-administracion, worker).

### Ventajas

1. **Centralización:** Lógica en un solo lugar
2. **Mantenibilidad:** Cambios en 1 archivo afectan todos los repos
3. **Consistencia:** Mismo comportamiento en todos los proyectos
4. **Reducción de código:** ~149 líneas eliminadas en worker (-54%)
5. **Aplicación de mejores prácticas:** Lecciones aprendidas compartidas

---

## 📊 Resumen de Migración

### Workflows Migrados

| Workflow | Job Migrado | Antes | Después | Reducción |
|----------|-------------|-------|---------|-----------|
| `ci.yml` | `lint` | 122 líneas | 109 líneas | -13 (-11%) |
| `test.yml` | `test-coverage` | 199 líneas | 63 líneas | -136 (-68%) |
| **Total** | - | **321 líneas** | **172 líneas** | **-149 (-46%)** |

### Jobs NO Migrados (específicos del proyecto)

- `ci.yml` → `test`, `docker-build-test`
- `test.yml` → `integration-tests`

---

## 📝 Workflows Reusables Utilizados

### 1. reusable-go-lint.yml

**Propósito:** Linter con golangci-lint para código Go.

**Ubicación (FASE 2):**
```
edugo-infrastructure/.github/workflows/reusable-go-lint.yml
```

**Uso en ci.yml:**
```yaml
lint:
  name: Lint & Format Check
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
```

**Funcionalidad:**
- ✅ Setup Go 1.25
- ✅ Setup EduGo Go (acceso a repos privados)
- ✅ Ejecutar golangci-lint v2.4.0 (compatible con Go 1.25)

**Estado FASE 1:** STUB - workflow reusable aún no existe en infrastructure

---

### 2. reusable-go-test.yml

**Propósito:** Tests con coverage threshold y servicios (PostgreSQL, MongoDB, RabbitMQ).

**Ubicación (FASE 2):**
```
edugo-infrastructure/.github/workflows/reusable-go-test.yml
```

**Uso en test.yml:**
```yaml
test-coverage:
  name: Tests with Coverage
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-test.yml@main
  with:
    go-version: "1.25"
    coverage-threshold: 0.0  # TODO: Aumentar a 33.0 cuando se implementen tests
    use-services: true  # PostgreSQL + MongoDB + RabbitMQ
```

**Funcionalidad:**
- ✅ Setup Go 1.25
- ✅ Servicios Docker: PostgreSQL 15, MongoDB 7, RabbitMQ 3
- ✅ Tests con race detection
- ✅ Coverage threshold configurable
- ✅ Reporte HTML de coverage
- ✅ Upload a Codecov
- ✅ Summary en GitHub Actions UI

**Estado FASE 1:** STUB - workflow reusable aún no existe en infrastructure

---

## 🔄 Estado Actual (FASE 1)

### ⚠️ Workflows Reusables son STUBS

**Razón:**
El repositorio `edugo-infrastructure` no está disponible localmente durante FASE 1.

**Implicaciones:**
- Los workflows migrados **referencian** a workflows reusables que **aún no existen**
- Si se ejecutan, **fallarán** porque no pueden encontrar los workflows reusables
- Esto es **esperado y correcto** en FASE 1

**Archivos de referencia (stubs):**
- `docs/cicd/stubs/infrastructure-workflows/reusable-go-lint.yml.stub`
- `docs/cicd/stubs/infrastructure-workflows/reusable-go-test.yml.stub`

**Decisión documentada:**
- `docs/cicd/tracking/decisions/TASK-1-BLOCKED.md`

---

## 🚀 Para FASE 2 - Implementación Real

**Cuando infrastructure esté disponible:**

### Paso 1: Crear Workflows Reusables en Infrastructure

1. Acceder a `edugo-infrastructure`
2. Crear branch: `feature/add-reusable-workflows`
3. Crear archivos basados en stubs:
   - `.github/workflows/reusable-go-lint.yml`
   - `.github/workflows/reusable-go-test.yml`
4. Crear PR y mergear a main

**Contenido:** Usar los stubs en `docs/cicd/stubs/infrastructure-workflows/` como base.

### Paso 2: Verificar Workflows en Worker

1. Los workflows migrados en worker **ya están listos** (referencias correctas)
2. Ejecutar workflows manualmente o via PR de prueba
3. Verificar que pasan correctamente
4. Resolver errores si los hay

**Tiempo estimado FASE 2:** 1-2 horas

---

## ✅ Lecciones Aprendidas Aplicadas

Durante la migración se aplicaron **5 lecciones críticas** aprendidas del proyecto piloto `api-mobile`:

### 1. ✅ NO usar subdirectorio

**❌ Incorrecto:**
```yaml
uses: .../workflows/reusable/go-lint.yml@main  # Subdirectorio
```

**✅ Correcto:**
```yaml
uses: .../workflows/reusable-go-lint.yml@main  # Raíz
```

### 2. ✅ NO declarar secret GITHUB_TOKEN

**Razón:** `GITHUB_TOKEN` es nombre reservado del sistema.

**❌ Incorrecto en workflow reusable:**
```yaml
on:
  workflow_call:
    secrets:
      GITHUB_TOKEN:  # ❌ Nombre reservado
        required: true
```

**✅ Correcto en workflow reusable:**
```yaml
steps:
  - uses: .../setup-edugo-go@main
    with:
      github-token: ${{ github.token }}  # ✅ Disponible automáticamente
```

### 3. ✅ Usar golangci-lint-action@v7

**Razón:** Compatible con Go 1.25 y golangci-lint v2.x

**❌ Incorrecto:**
```yaml
uses: golangci/golangci-lint-action@v6  # No soporta v2.x
```

**✅ Correcto:**
```yaml
uses: golangci/golangci-lint-action@v7  # Soporta v2.x
```

### 4. ✅ Default golangci-lint v2.4.0+

**Razón:** Compilado con Go 1.25, compatible con proyectos Go 1.25

**❌ Incorrecto:**
```yaml
with:
  version: v1.64.7  # Incompatible con Go 1.25
```

**✅ Correcto:**
```yaml
with:
  version: v2.4.0  # Compatible con Go 1.25
```

### 5. ✅ NO especificar golangci-lint-version en caller

**Razón:** El workflow reusable ya define la versión correcta

**✅ Correcto en worker:**
```yaml
lint:
  uses: .../reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
    # NO incluir: golangci-lint-version
```

**Referencia:** `docs/cicd/SPRINT-4-LESSONS-LEARNED.md`

---

## 🔍 Backups de Workflows Originales

Los workflows originales están respaldados en:

```
docs/workflows-migrated-sprint4/
├── ci.yml.backup       (122 líneas)
└── test.yml.backup     (199 líneas)
```

### Restauración

Si necesitas restaurar el workflow original:

```bash
cp docs/workflows-migrated-sprint4/ci.yml.backup .github/workflows/ci.yml
```

---

## 📋 Validación

### Validar Sintaxis YAML

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"
```

### Verificar Referencias Correctas

```bash
# Verificar ci.yml
grep "uses: EduGo" .github/workflows/ci.yml
# Debe mostrar: .../reusable-go-lint.yml@main (sin subdirectorio)

# Verificar test.yml
grep "uses: EduGo" .github/workflows/test.yml
# Debe mostrar: .../reusable-go-test.yml@main (sin subdirectorio)
```

### Verificar NO tiene secrets GITHUB_TOKEN

```bash
grep -A 2 "secrets:" .github/workflows/ci.yml
# NO debe mostrar GITHUB_TOKEN

grep -A 2 "secrets:" .github/workflows/test.yml
# NO debe mostrar GITHUB_TOKEN
```

---

## 📚 Referencias

- [SPRINT-4-TASKS.md](cicd/sprints/SPRINT-4-TASKS.md) - Plan completo del sprint
- [SPRINT-4-LESSONS-LEARNED.md](cicd/SPRINT-4-LESSONS-LEARNED.md) - Lecciones de api-mobile
- [TASK-1-BLOCKED.md](cicd/tracking/decisions/TASK-1-BLOCKED.md) - Decisión de uso de stubs
- [GitHub Docs - Reusing workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)

---

## 🎯 Próximos Pasos

### FASE 2 (Pendiente)

1. Acceder a `edugo-infrastructure`
2. Crear workflows reusables reales
3. Mergear a main en infrastructure
4. Probar workflows en worker

### FASE 3 (Pendiente)

1. Crear PR en worker (ya está listo con referencias correctas)
2. Validar que workflows pasan
3. Mergear a dev
4. Celebrar reducción de ~149 líneas

---

**Generado por:** Claude Code
**Fecha:** 2025-11-22
**Sprint:** SPRINT-4 - Workflows Reusables
**Fase:** FASE 1 - Implementación con Stubs
