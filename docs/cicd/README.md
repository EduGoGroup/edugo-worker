# Plan de Implementación - edugo-worker

**Proyecto:** edugo-worker (Worker de procesamiento asíncrono)  
**Tipo:** Aplicación desplegable con Docker (Tipo A)  
**Fecha:** 19 de Noviembre, 2025  
**Versión:** 1.0  
**Estado:** Propuesta para Implementación

---

## 📑 Tabla de Contenidos

1. [Resumen Ejecutivo](#-resumen-ejecutivo)
2. [Contexto del Proyecto](#-contexto-del-proyecto)
3. [Análisis de Duplicación Docker](#-análisis-de-duplicación-docker-problema-crítico)
4. [Estado Actual](#-estado-actual)
5. [Problemas Identificados](#-problemas-identificados)
6. [Objetivos de la Implementación](#-objetivos-de-la-implementación)
7. [Sprints Planificados](#-sprints-planificados)
8. [Roadmap de Implementación](#-roadmap-de-implementación)
9. [Métricas y KPIs](#-métricas-y-kpis)
10. [Riesgos y Mitigación](#-riesgos-y-mitigación)

---

## 🎯 Resumen Ejecutivo

### En 60 Segundos

**Problema Principal:**  
edugo-worker tiene **3 workflows diferentes construyendo Docker images**, causando:
- Desperdicio de recursos CI/CD
- Confusión sobre cuál usar
- release.yml fallando actualmente
- Potencial de tags duplicados

**Solución:**  
Consolidar en 1 solo workflow (manual-release.yml) con control fino.

**Impacto:**
- ✅ Eliminar ~250 líneas duplicadas (42% de workflows)
- ✅ Resolver fallos en release.yml
- ✅ Claridad para el equipo
- ✅ Estandarización con Go 1.25
- ✅ Coverage threshold 33%

**Tiempo:** 28-36 horas en 2 sprints  
**Prioridad:** 🔴 Alta (por duplicación Docker)

---

## 📦 Contexto del Proyecto

### ¿Qué es edugo-worker?

**Descripción:**  
Worker de procesamiento asíncrono que consume mensajes de RabbitMQ, procesa tareas (generación de resúmenes con OpenAI, creación de quizzes automáticos), y persiste resultados en MongoDB.

**Repositorio:** https://github.com/EduGoGroup/edugo-worker  
**Ruta Local:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker`

**Tecnología:**
- **Lenguaje:** Go 1.24.10 (go.mod) vs 1.25 (workflows) ⚠️
- **Infraestructura:** RabbitMQ, MongoDB, PostgreSQL, OpenAI API
- **Deployment:** Docker (ghcr.io)
- **CI/CD:** GitHub Actions

**Dependencias clave:**
```go
github.com/EduGoGroup/edugo-shared/bootstrap v0.9.0
github.com/EduGoGroup/edugo-shared/logger v0.7.0
github.com/EduGoGroup/edugo-infrastructure/mongodb v0.9.0
github.com/rabbitmq/amqp091-go v1.10.0
go.mongodb.org/mongo-driver v1.17.6
```

---

## 🔴 Análisis de Duplicación Docker (PROBLEMA CRÍTICO)

### Situación Actual: 3 Workflows Construyendo Docker

```
edugo-worker/.github/workflows/
├── build-and-push.yml        (85 líneas) ⚠️ DUPLICADO
├── docker-only.yml           (73 líneas) ⚠️ DUPLICADO
├── release.yml              (283 líneas) ⚠️ DUPLICADO + FALLA
└── manual-release.yml       (340 líneas) ✅ EL BUENO
```

### Comparativa Detallada

| Aspecto | build-and-push.yml | docker-only.yml | release.yml | manual-release.yml |
|---------|-------------------|-----------------|-------------|-------------------|
| **Trigger** | Manual + Push main | Manual | Tag push (v*) | Manual |
| **Tests previos** | No | No | Sí | Sí |
| **Multi-platform** | No | Sí (amd64+arm64) | Sí | Sí |
| **Tags generados** | branch, sha, latest | custom, latest | semver, latest | versión, latest |
| **GitHub Release** | No | No | Sí | Sí |
| **Control fino** | No | No | No | Sí |
| **GitHub App Token** | No | No | No | Sí |
| **Estado actual** | Funcional | Funcional | ❌ Falla | ✅ Funcional |

### ¿Qué hace cada workflow?

#### 1. build-and-push.yml
**Propósito original:** Build y push rápido sin tests  
**Triggers:**
- Manual (workflow_dispatch) con input de environment
- Automático en push a main

**Qué hace:**
1. Checkout código
2. Setup Docker Buildx
3. Login a GHCR
4. Build y push imagen
5. Genera tags: `{branch}`, `{branch}-{sha}`, `latest`

**Problemas:**
- ❌ NO ejecuta tests antes de build
- ❌ NO crea GitHub Release
- ❌ Tags automáticos en push a main (puede ser inesperado)
- ❌ Solo soporta linux/amd64

**Cuándo se usa:** Desarrollo rápido sin validación

#### 2. docker-only.yml
**Propósito original:** Build Docker simple y rápido  
**Triggers:**
- Manual (workflow_dispatch) con input de tag personalizado
- Comentado: push a main

**Qué hace:**
1. Checkout código
2. Setup Docker Buildx
3. Login a GHCR
4. Build y push imagen
5. Genera tags: `{custom-tag}`, `latest`

**Problemas:**
- ❌ NO ejecuta tests
- ❌ NO crea GitHub Release
- ❌ Tags hardcoded a `edugogroup/edugo-worker` (minúsculas)
- ⚠️ Multi-platform (bueno) pero sin validación

**Cuándo se usa:** Build rápido con tag personalizado

#### 3. release.yml
**Propósito original:** Release completo automático  
**Triggers:**
- Automático en push de tags `v*` (ej: v1.0.0)

**Qué hace:**
1. ✅ Valida y ejecuta tests completos
2. ✅ Build binario
3. ✅ Build y push imagen Docker multi-platform
4. ✅ Crea GitHub Release con changelog
5. Genera tags semver: `v1.0.0`, `1.0.0`, `1`, `1.0`, `latest`

**Problemas:**
- ❌ **ESTÁ FALLANDO** (Run 19485700108)
- ⚠️ Versión Go 1.25 (vs 1.24.10 en go.mod)
- ⚠️ NO permite control manual (solo automático en tag)

**Cuándo se usa:** Release de producción

#### 4. manual-release.yml ✅
**Propósito:** Release completo MANUAL con control fino  
**Triggers:**
- Manual (workflow_dispatch) con inputs:
  - version (ej: 0.1.0)
  - bump_type (patch/minor/major)

**Qué hace:**
1. ✅ Genera token desde GitHub App (para disparar workflows subsecuentes)
2. ✅ Valida versión semver
3. ✅ Actualiza version.txt
4. ✅ Genera y actualiza CHANGELOG.md
5. ✅ Commit + push a main
6. ✅ Crea y push tag
7. ✅ Ejecuta tests completos
8. ✅ Build y push Docker multi-platform
9. ✅ Crea GitHub Release

**Ventajas:**
- ✅ Control total sobre el release
- ✅ Maneja CHANGELOG automáticamente
- ✅ GitHub App Token dispara sync-main-to-dev.yml
- ✅ Multi-platform (linux/amd64 + linux/arm64)
- ✅ Genera tags limpios: `v0.1.0`, `0.1.0`, `latest`

**Cuándo se usa:** Releases oficiales controlados

---

### Consecuencias de la Duplicación

1. **Confusión del equipo**
   - ¿Cuál workflow usar para desarrollo?
   - ¿Cuál para producción?
   - ¿Qué diferencia hay entre ellos?

2. **Desperdicio de recursos**
   - 3 workflows = 3x tiempo de CI/CD
   - Cache fragmentado entre workflows
   - Mayor consumo de GitHub Actions minutes

3. **Riesgo de conflictos**
   - Tags duplicados (ej: 2 workflows generando `latest`)
   - Imágenes sobrescritas sin control
   - Historial de releases confuso

4. **Mantenimiento multiplicado**
   - Cambios en Dockerfile requieren actualizar 3 workflows
   - Actualizar versión de Go en 3 lugares
   - Probar 3 workflows diferentes

5. **Fallos actuales**
   - release.yml fallando (Run 19485700108)
   - Inconsistencia en versión Go

---

### Solución Propuesta: Consolidación

**Mantener solo:** `manual-release.yml`

**Eliminar:**
- `build-and-push.yml`
- `docker-only.yml`
- `release.yml` (después de migrar funcionalidad)

**Justificación:**

| Criterio | manual-release.yml |
|----------|-------------------|
| Tests previos | ✅ Sí |
| Multi-platform | ✅ Sí |
| Control fino | ✅ Sí (inputs) |
| GitHub Release | ✅ Sí |
| CHANGELOG | ✅ Sí (automático) |
| GitHub App Token | ✅ Sí (dispara workflows) |
| Estado actual | ✅ Funcional |
| Extensibilidad | ✅ Alta |

**Funcionalidad cubierta:**
- ✅ Desarrollo rápido: Usar manual-release.yml con bump_type=patch
- ✅ Tags personalizados: Usar input version
- ✅ Release automático: Trigger desde API/UI
- ✅ CI/CD completo: Tests + Build + Release

**Migración:**
- Agregar variable de entorno `SKIP_TESTS` para desarrollo rápido
- Documentar uso de manual-release.yml en README
- Crear script helper `scripts/release.sh`

---

## 📊 Estado Actual

### Workflows Existentes (7 archivos)

```yaml
.github/workflows/
├── ci.yml                    # ✅ CI con tests + lint + docker build test
├── test.yml                  # ✅ Tests con coverage + servicios (PG, Mongo, RabbitMQ)
├── manual-release.yml        # ✅ Release manual completo (MANTENER)
├── build-and-push.yml        # ⚠️ Duplicado (ELIMINAR)
├── docker-only.yml           # ⚠️ Duplicado (ELIMINAR)
├── release.yml               # ❌ Falla + Duplicado (ELIMINAR)
└── sync-main-to-dev.yml      # ✅ Sincronización automática
```

### Métricas Actuales

| Métrica | Valor | Estado |
|---------|-------|--------|
| **Workflows totales** | 7 | ⚠️ Muchos |
| **Workflows Docker** | 3 | ❌ Duplicados |
| **Success rate** | 70% | ⚠️ Bajo |
| **Líneas código workflows** | ~600 | Normal |
| **Duplicación estimada** | ~250 líneas (42%) | ❌ Alta |
| **Go version (go.mod)** | 1.24.10 | ⚠️ Desactualizado |
| **Go version (workflows)** | 1.25 | ⚠️ Inconsistente |
| **Coverage threshold** | No definido | ❌ Falta |
| **Pre-commit hooks** | No | ❌ Falta |

### Fallos Recientes

```
Run ID: 19485700108
Workflow: Release CI/CD (release.yml)
Status: failure
Date: 2025-11-19T00:48:39Z
```

---

## 🚨 Problemas Identificados

### 🔴 Prioridad 0 (Críticos)

#### P0-1: 3 Workflows Docker Duplicados
**Impacto:** Alto  
**Esfuerzo:** Alto (3-4 horas)  
**Descripción:** Eliminar build-and-push.yml, docker-only.yml y release.yml  
**Solución:** Sprint 3 Tarea 1

#### P0-2: release.yml Fallando
**Impacto:** Alto  
**Esfuerzo:** Bajo (incluido en P0-1)  
**Descripción:** Último run falló  
**Solución:** Migrar funcionalidad a manual-release.yml y eliminar

---

### 🟡 Prioridad 1 (Altos)

#### P1-1: Sin Coverage Threshold
**Impacto:** Medio  
**Esfuerzo:** Bajo (45 min)  
**Descripción:** No hay umbral de cobertura definido (apis tienen 33%)  
**Solución:** Sprint 3 Tarea 5

#### P1-2: Go 1.25 Inconsistente
**Impacto:** Medio  
**Esfuerzo:** Bajo (45-60 min)  
**Descripción:** go.mod dice 1.24.10, workflows dicen 1.25  
**Solución:** Sprint 3 Tarea 2

#### P1-3: Pre-commit Hooks Faltantes
**Impacto:** Medio  
**Esfuerzo:** Medio (60-90 min)  
**Descripción:** No hay validación local antes de commit  
**Solución:** Sprint 3 Tarea 4

---

### 🟢 Prioridad 2 (Medios)

#### P2-1: Migrar a Workflows Reusables
**Impacto:** Bajo  
**Esfuerzo:** Alto (12-16 horas)  
**Descripción:** Centralizar lógica común en edugo-infrastructure  
**Solución:** Sprint 4 completo

---

## 🎯 Objetivos de la Implementación

### Objetivos Principales

1. **Eliminar duplicación Docker** (P0)
   - De 3 workflows a 1 solo
   - Reducir de ~441 líneas a ~340 líneas
   - Ahorro: ~101 líneas (23%)

2. **Estandarizar Go 1.25** (P1)
   - Actualizar go.mod de 1.24.10 → 1.25.3
   - Consistencia con shared e infrastructure
   - Aprovechar mejoras de Go 1.25

3. **Establecer coverage threshold 33%** (P1)
   - Alinear con api-mobile y api-administracion
   - Prevenir regresiones de calidad
   - Forzar mejora continua

4. **Implementar pre-commit hooks** (P1)
   - 7 validaciones automáticas
   - Reducir fallos en CI
   - Mejorar experiencia de desarrollo

5. **Migrar a workflows reusables** (P2)
   - Centralizar en edugo-infrastructure
   - Reducir duplicación cross-repo
   - Facilitar mantenimiento

---

### Objetivos Secundarios

- Documentar uso de workflows
- Crear scripts helper
- Mejorar mensajes de commit
- Optimizar cache
- Reducir tiempos de CI

---

## 📋 Sprints Planificados

### Sprint 3: Consolidación Docker + Go 1.25 (Prioridad 🔴)

**Duración:** 4-5 días  
**Esfuerzo:** 16-20 horas  
**Tareas:** 12 detalladas

**Objetivos:**
- ✅ Consolidar 3 workflows Docker en 1
- ✅ Migrar a Go 1.25.3
- ✅ Implementar pre-commit hooks
- ✅ Establecer coverage threshold 33%
- ✅ Resolver release.yml fallando

**Entregables:**
- Eliminación de build-and-push.yml
- Eliminación de docker-only.yml
- Migración y eliminación de release.yml
- go.mod actualizado a Go 1.25.3
- .pre-commit-config.yaml funcional
- test.yml con threshold 33%
- PR completo con tests pasando

**Archivo:** [SPRINT-3-TASKS.md](./SPRINT-3-TASKS.md)

---

### Sprint 4: Workflows Reusables

**Duración:** 3-4 días  
**Esfuerzo:** 12-16 horas  
**Tareas:** 8 detalladas

**Objetivos:**
- ✅ Migrar ci.yml a workflow reusable
- ✅ Migrar test.yml a workflow reusable
- ✅ Migrar manual-release.yml a workflow reusable
- ✅ Centralizar en edugo-infrastructure

**Entregables:**
- ci.yml usando workflow reusable
- test.yml usando workflow reusable
- manual-release.yml usando workflow reusable
- Workflows reusables en infrastructure
- PR completo con tests pasando

**Archivo:** [SPRINT-4-TASKS.md](./SPRINT-4-TASKS.md)

---

## 🗓️ Roadmap de Implementación

### Fase 1: Sprint 3 (Semana 1)

```
Día 1: Análisis y Consolidación Docker (3-4h)
├── Analizar 3 workflows Docker
├── Crear script de migración
├── Testear manual-release.yml
└── Eliminar build-and-push.yml y docker-only.yml

Día 2: Migración release.yml + Go 1.25 (4-5h)
├── Migrar funcionalidad de release.yml
├── Eliminar release.yml
├── Actualizar go.mod a 1.25.3
└── Actualizar workflows a Go 1.25.3

Día 3: Pre-commit Hooks (3-4h)
├── Crear .pre-commit-config.yaml
├── Agregar 7 hooks
├── Documentar en README
└── Testear hooks localmente

Día 4: Coverage Threshold + Ajustes (3-4h)
├── Agregar threshold 33% en test.yml
├── Ajustar CI si es necesario
├── Documentar estándares
└── Crear PR

Día 5: Review y Merge (2-3h)
├── Revisar feedback
├── Hacer ajustes
├── Merge a dev
└── Documentar cambios
```

**Total Sprint 3:** 16-20 horas

---

### Fase 2: Sprint 4 (Semana 2)

```
Día 1: Preparar Infrastructure (2-3h)
├── Crear workflows reusables en infrastructure
├── Definir interfaces
├── Documentar uso
└── Crear tests

Día 2: Migrar ci.yml (3-4h)
├── Adaptar ci.yml a workflow reusable
├── Testear localmente
├── Crear PR
└── Merge

Día 3: Migrar test.yml (3-4h)
├── Adaptar test.yml a workflow reusable
├── Testear con servicios
├── Crear PR
└── Merge

Día 4: Migrar manual-release.yml (4-5h)
├── Adaptar manual-release.yml
├── Testear release completo
├── Crear PR
└── Merge
```

**Total Sprint 4:** 12-16 horas

**Total General:** 28-36 horas

---

## 📈 Métricas y KPIs

### Antes vs Después - Sprint 3

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| **Workflows Docker** | 3 | 1 | -66% |
| **Líneas workflows Docker** | ~441 | ~340 | -23% |
| **Workflows con fallos** | 1 (release.yml) | 0 | -100% |
| **Go version consistente** | No | Sí | ✅ |
| **Coverage threshold** | No | 33% | ✅ |
| **Pre-commit hooks** | No | 7 hooks | ✅ |
| **Success rate esperado** | 70% | 85%+ | +15% |

### Antes vs Después - Sprint 4

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| **Workflows locales** | 7 | 4 | -43% |
| **Líneas workflows** | ~600 | ~200 | -66% |
| **Workflows reusables** | 0 | 3 | +3 |
| **Mantenibilidad** | Baja | Alta | ✅ |
| **Duplicación cross-repo** | Alta | Baja | ✅ |

### KPIs de Éxito

- ✅ Success rate > 85%
- ✅ 0 workflows Docker duplicados
- ✅ Go 1.25.3 en go.mod y workflows
- ✅ Coverage >= 33%
- ✅ 7 pre-commit hooks funcionando
- ✅ 3 workflows reusables activos
- ✅ Tiempo de CI reducido 10-15%

---

## ⚠️ Riesgos y Mitigación

### Riesgo 1: Eliminar workflow incorrecto
**Probabilidad:** Baja  
**Impacto:** Alto  
**Mitigación:**
- Analizar funcionalidad de cada workflow antes de eliminar
- Crear backup de workflows eliminados en docs
- Probar manual-release.yml extensivamente
- Implementar cambios en rama feature
- Review minucioso de PR

### Riesgo 2: Breaking changes en Go 1.25
**Probabilidad:** Media  
**Impacto:** Medio  
**Mitigación:**
- Ejecutar tests completos después de actualizar
- Revisar changelog de Go 1.25
- Actualizar dependencias gradualmente
- Monitorear performance

### Riesgo 3: Pre-commit hooks muy restrictivos
**Probabilidad:** Media  
**Impacto:** Bajo  
**Mitigación:**
- Hooks opcionales inicialmente
- Documentar cómo saltarlos si es necesario
- Ajustar según feedback del equipo
- Configurar límites razonables

### Riesgo 4: Coverage threshold 33% inalcanzable
**Probabilidad:** Baja  
**Impacto:** Medio  
**Mitigación:**
- Verificar coverage actual antes de establecer threshold
- Threshold como warning en lugar de error inicialmente
- Plan gradual de mejora de coverage
- Exclusiones razonables (mocks, main.go)

### Riesgo 5: Workflows reusables no funcionan
**Probabilidad:** Baja  
**Impacto:** Alto  
**Mitigación:**
- Implementar workflows reusables en infrastructure primero
- Probar con un repo de prueba
- Migración gradual (1 workflow a la vez)
- Mantener workflows originales hasta confirmar funcionamiento

---

## 📚 Referencias

### Documentación Relacionada
- [Análisis Estado Actual](../../01-ANALISIS-ESTADO-ACTUAL.md)
- [Propuestas de Mejora](../../02-PROPUESTAS-MEJORA.md)
- [Matriz Comparativa](../../04-MATRIZ-COMPARATIVA.md)

### Otros Planes de Implementación
- [01-shared](../01-shared/README.md) - Go 1.25 y releases por módulo
- [02-infrastructure](../02-infrastructure/README.md) - Workflows reusables
- [03-api-mobile](../03-api-mobile/README.md) - Pre-commit hooks
- [04-api-administracion](../04-api-administracion/README.md) - Coverage threshold

### Repositorio
- **GitHub:** https://github.com/EduGoGroup/edugo-worker
- **Local:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker`

---

## 🎉 Conclusión

Este plan detalla la estandarización y optimización de edugo-worker con enfoque en:

1. **Eliminar duplicación crítica** (3 workflows Docker → 1)
2. **Estandarizar tecnología** (Go 1.25, coverage 33%)
3. **Mejorar calidad** (pre-commit hooks, tests)
4. **Centralizar configuración** (workflows reusables)

**Prioridad máxima:** Sprint 3 Tarea 1 (Consolidación Docker)

**Próximo paso:** Abrir [SPRINT-3-TASKS.md](./SPRINT-3-TASKS.md) y comenzar implementación.

---

**Generado por:** Claude Code  
**Fecha:** 19 de Noviembre, 2025  
**Versión:** 1.0
