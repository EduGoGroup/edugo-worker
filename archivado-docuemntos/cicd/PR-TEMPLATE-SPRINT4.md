# Sprint 4: Migrar a Workflows Reusables

## 🎯 Objetivo

Migrar workflows CI/CD a workflows reusables centralizados en `edugo-infrastructure`.

---

## 📊 Cambios

### Workflows Migrados

| Workflow | Job Migrado | Antes | Después | Reducción |
|----------|-------------|-------|---------|-----------|
| `ci.yml` | `lint` | 122 líneas | 109 líneas | -13 (-11%) |
| `test.yml` | `test-coverage` | 199 líneas | 63 líneas | -136 (-68%) |
| **Total** | - | **321 líneas** | **172 líneas** | **-149 (-46%)** |

### Jobs NO Migrados (específicos del proyecto)

- `ci.yml` → `test`, `docker-build-test` (lógica específica de worker)
- `test.yml` → `integration-tests` (tests específicos de worker)

---

## 🔄 Workflows Reusables Utilizados

### 1. reusable-go-lint.yml

**Referencia:**
```yaml
lint:
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
```

**Funcionalidad:**
- ✅ golangci-lint v2.4.0 (compatible con Go 1.25)
- ✅ Setup Go 1.25
- ✅ Acceso a repos privados (setup-edugo-go)

---

### 2. reusable-go-test.yml

**Referencia:**
```yaml
test-coverage:
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-test.yml@main
  with:
    go-version: "1.25"
    coverage-threshold: 0.0  # TODO: Aumentar a 33.0 con tests
    use-services: true
```

**Funcionalidad:**
- ✅ Tests con race detection
- ✅ Coverage threshold configurable
- ✅ Servicios Docker: PostgreSQL 15, MongoDB 7, RabbitMQ 3
- ✅ Reporte HTML de coverage
- ✅ Upload a Codecov
- ✅ Summary en GitHub Actions UI

---

## ✅ Lecciones Aprendidas Aplicadas

Se aplicaron **5 lecciones críticas** del proyecto piloto `api-mobile`:

1. ✅ NO usar subdirectorio (`reusable-go-lint.yml` en raíz, no en `reusable/`)
2. ✅ NO declarar secret GITHUB_TOKEN (nombre reservado)
3. ✅ Usar golangci-lint-action@v7 (compatible con Go 1.25)
4. ✅ Default golangci-lint v2.4.0 (compilado con Go 1.25)
5. ✅ NO especificar golangci-lint-version en caller (usa default del reusable)

**Referencia:** [SPRINT-4-LESSONS-LEARNED.md](docs/cicd/SPRINT-4-LESSONS-LEARNED.md)

---

## 🎁 Beneficios

1. **Centralización:** Lógica CI/CD en un solo lugar (infrastructure)
2. **Mantenibilidad:** 1 cambio → afecta api-mobile, api-admin y worker
3. **Consistencia:** Mismo comportamiento en todos los repos
4. **Reducción de código:** -149 líneas en worker (-46%)
5. **Mejores prácticas:** Aplicación automática de fixes y mejoras

---

## 📁 Archivos Modificados

### Workflows Migrados
- `.github/workflows/ci.yml` - Job lint migrado
- `.github/workflows/test.yml` - Job test-coverage migrado

### Backups
- `docs/workflows-migrated-sprint4/ci.yml.backup`
- `docs/workflows-migrated-sprint4/test.yml.backup`

### Documentación
- `docs/REUSABLE-WORKFLOWS.md` - Guía completa (nuevo)
- `README.md` - Sección de workflows reusables (actualizado)

### Stubs (FASE 1)
- `docs/cicd/stubs/infrastructure-workflows/reusable-go-lint.yml.stub`
- `docs/cicd/stubs/infrastructure-workflows/reusable-go-test.yml.stub`

### Tracking
- `docs/cicd/tracking/SPRINT-STATUS.md` - Progreso del sprint
- `docs/cicd/tracking/decisions/TASK-1-BLOCKED.md` - Decisión de stubs

---

## ⚠️ Estado FASE 1 (IMPORTANTE)

**NOTA:** Este PR fue creado en **FASE 1** usando **STUBS**.

**Implicaciones:**
- Los workflows reusables referenciados **aún no existen** en infrastructure
- Si se ejecutan workflows, **fallarán** porque no pueden encontrar los reusables
- Esto es **esperado y correcto** en FASE 1

**Para FASE 2:**
1. Crear workflows reusables reales en `edugo-infrastructure`
2. Mergear PR en infrastructure a main
3. Re-ejecutar workflows en este PR
4. Verificar que pasan correctamente
5. Mergear este PR a dev

---

## 🧪 Testing

### Validación Sintáctica (✅ Hecho)

```bash
# Sintaxis YAML válida
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test.yml'))"

# Referencias correctas (sin subdirectorio)
grep "reusable-go-lint.yml@main" .github/workflows/ci.yml
grep "reusable-go-test.yml@main" .github/workflows/test.yml

# NO usa secrets GITHUB_TOKEN
! grep "GITHUB_TOKEN.*secrets" .github/workflows/ci.yml
! grep "GITHUB_TOKEN.*secrets" .github/workflows/test.yml
```

**Resultado:** ✅ Todos los checks pasan

### Testing Funcional (⏳ Pendiente FASE 2)

**Cuando workflows reusables existan en infrastructure:**

- [ ] CI workflow ejecuta lint correctamente
- [ ] Test workflow ejecuta tests con coverage
- [ ] Servicios Docker se levantan correctamente
- [ ] Coverage threshold se valida
- [ ] Reportes se generan correctamente

---

## 📚 Referencias

- [SPRINT-4-TASKS.md](docs/cicd/sprints/SPRINT-4-TASKS.md) - Plan completo
- [SPRINT-4-LESSONS-LEARNED.md](docs/cicd/SPRINT-4-LESSONS-LEARNED.md) - Lecciones de api-mobile
- [REUSABLE-WORKFLOWS.md](docs/REUSABLE-WORKFLOWS.md) - Guía completa
- [GitHub Docs - Reusing workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)

---

## ✅ Checklist

- [x] Workflows migrados a reusables
- [x] Backups creados
- [x] Lecciones aprendidas aplicadas
- [x] Documentación actualizada (REUSABLE-WORKFLOWS.md, README.md)
- [x] Validación sintáctica pasada
- [ ] Testing funcional (FASE 2)
- [ ] CI/CD pasando (FASE 2)
- [ ] Merge a dev (FASE 2)

---

**Sprint:** SPRINT-4 - Workflows Reusables
**Fase:** FASE 1 - Implementación con Stubs
**Progreso:** 5/8 tareas (62%)

🤖 Generated with Claude Code

Co-Authored-By: Claude <noreply@anthropic.com>
