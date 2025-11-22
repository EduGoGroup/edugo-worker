# Infrastructure Workflows Stubs - SPRINT-4 FASE 1

**Proyecto:** edugo-worker
**Sprint:** SPRINT-4
**Fase:** FASE 1 - Implementación con Stubs
**Fecha:** 2025-11-22

---

## 🎯 Propósito

Este directorio contiene **stubs** (documentación) de workflows reusables que deberían crearse en `edugo-infrastructure` pero que no están disponibles durante FASE 1.

---

## 📁 Contenido

### Workflows Reusables (Stubs)

1. **reusable-go-lint.yml.stub**
   - Linter con golangci-lint v2.4.0
   - Compatible con Go 1.25
   - Aplica lecciones aprendidas de api-mobile

2. **reusable-go-test.yml.stub**
   - Tests con coverage threshold
   - Servicios: PostgreSQL, MongoDB, RabbitMQ
   - Compatible con Go 1.25

---

## ⚠️ Estado STUB

**Razón:**
El repositorio `edugo-infrastructure` no está disponible localmente durante FASE 1.

**Decisión:**
Documentar workflows como stubs y continuar con migración en edugo-worker usando referencias a estos stubs.

**Archivo de decisión:**
`docs/cicd/tracking/decisions/TASK-1-BLOCKED.md`

---

## 🔄 Para FASE 2

**Cuando infrastructure esté disponible:**

1. Acceder a `edugo-infrastructure`
2. Crear workflows reusables basados en estos stubs
3. Mergear a main en infrastructure
4. Actualizar referencias en edugo-worker de stub a real
5. Probar workflows reusables funcionando

**Tiempo estimado:** 1-2 horas

---

## 📋 Lecciones Aprendidas Aplicadas

✅ **Problema 1:** NO usar subdirectorio
- Archivos deben estar en `.github/workflows/reusable-*.yml` (raíz)
- NO en `.github/workflows/reusable/go-lint.yml`

✅ **Problema 2:** NO declarar secret GITHUB_TOKEN
- Es nombre reservado
- Usar `github.token` directamente en steps

✅ **Problema 3:** Usar golangci-lint-action@v7
- Compatible con Go 1.25
- Soporta golangci-lint v2.x

✅ **Problema 4:** Default golangci-lint v2.4.0+
- Compilado con Go 1.25
- Compatible con proyectos Go 1.25

**Referencia:** `docs/cicd/SPRINT-4-LESSONS-LEARNED.md`

---

## 🔗 Referencias

- [SPRINT-4-TASKS.md](../../sprints/SPRINT-4-TASKS.md)
- [SPRINT-4-LESSONS-LEARNED.md](../../SPRINT-4-LESSONS-LEARNED.md)
- [Decisión TASK-1-BLOCKED.md](../tracking/decisions/TASK-1-BLOCKED.md)

---

**Generado por:** Claude Code
**Fecha:** 2025-11-22
**FASE:** 1 - Implementación con Stubs
