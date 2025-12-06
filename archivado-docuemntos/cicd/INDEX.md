# Índice - Plan de Implementación edugo-worker

**🎯 Punto de Entrada Principal**

⚠️ **UBICACIÓN DE ESTE ARCHIVO:**
```
📍 Ruta: 05-worker/INDEX.md (dentro de cicd-analysis)
📍 Todas las rutas son relativas a: 05-worker/
⚠️ NO uses archivos de otros proyectos (01-shared, 03-api-mobile, etc.)
```

---

## 🗺️ Navegación Rápida

### ⭐ Para Ejecutar Sprints (RECOMENDADO)
1. **[PROMPTS.md](./PROMPTS.md)** ⭐⭐⭐ - Prompts estándar para Fase 1, 2, 3
2. **[tracking/SPRINT-STATUS.md](./tracking/SPRINT-STATUS.md)** - Estado actual del sprint
3. **[START-HERE.md](./START-HERE.md)** - Punto de entrada rápido

### Para Entender el Contexto
4. **[README.md](./README.md)** ⭐ - Contexto completo del proyecto (20-25 min)
5. Este archivo (INDEX.md) - Navegación y resumen (5 min)

### Para Implementar
3. **[SPRINT-3-TASKS.md](./sprints/SPRINT-3-TASKS.md)** ⭐⭐⭐ - Plan detallado Sprint 3 (~2,500 líneas)
4. **[SPRINT-4-TASKS.md](./sprints/SPRINT-4-TASKS.md)** - Plan detallado Sprint 4 (~800 líneas)

---

## 📊 Resumen Ultra-Rápido

```
Plan Completo: ~3,600 líneas en 4 archivos
├── Sprint 3: CONSOLIDACIÓN DOCKER + GO 1.25 (~2,500 líneas) ⭐ PRIORIDAD
│   ├── 4-5 días / 16-20 horas
│   ├── 12 tareas detalladas
│   ├── ~35 scripts bash
│   └── CRÍTICO: Eliminar 2 workflows Docker duplicados
│
└── Sprint 4: WORKFLOWS REUSABLES (~800 líneas)
    ├── 3-4 días / 12-16 horas
    ├── 8 tareas detalladas
    └── ~20 scripts bash

Total Estimado: 28-36 horas de implementación
```

---

## 🔴 PROBLEMA CRÍTICO DEL WORKER

### Duplicación de Workflows Docker (Máxima Prioridad)

**Situación actual:**
```
3 workflows construyendo Docker images:
├── build-and-push.yml    (85 líneas) - Manual + Push a main
├── docker-only.yml       (73 líneas) - Manual
└── release.yml          (283 líneas) - Push de tags v*

Resultado: Desperdicio de recursos + Confusión + Tags duplicados
```

**Problemas derivados:**
- ❌ release.yml fallando (Run 19485700108)
- ❌ Sin coverage threshold (apis tienen 33%)
- ❌ Go 1.25 inconsistente (1.24.10 en go.mod vs 1.25 en workflows)
- ❌ Sin pre-commit hooks
- ❌ Sin workflows reusables

**Solución Sprint 3:**
Consolidar en 1 solo workflow (manual-release.yml) con control por variables.

---

## 🚀 Quick Actions

### ⭐ Acción 1: Ejecutar Sprint con Prompts (RECOMENDADO)
```
1. Abre PROMPTS.md
2. Elige tu fase (1, 2, o 3)
3. Copia el prompt correspondiente
4. Reemplaza X con número de sprint
5. Pégalo en Claude y ejecuta
```

### Acción 2: Ver Estado Actual
```bash
cat tracking/SPRINT-STATUS.md | head -40
# Ver: Sprint activo, Fase, Progreso, Próxima tarea
```

### Acción 3: Comenzar Sprint 3 AHORA
```bash
open sprints/SPRINT-3-TASKS.md
# Ir a Tarea 1: Análisis y Consolidación Docker
# Seguir instrucciones paso a paso
```

### Acción 4: Ver el Problema Docker en Detalle
```bash
open README.md
# Ir a sección "Análisis de Duplicación Docker"
```

### Acción 5: Ver Solo Resumen de Tareas
```bash
grep "^### Tarea" sprints/SPRINT-3-TASKS.md
grep "^### Tarea" sprints/SPRINT-4-TASKS.md
```

---

## 📁 Estructura de Archivos

```
05-worker/
├── INDEX.md                    ← Estás aquí
├── README.md                   ← Contexto completo (~350 líneas)
├── docs/
│   ├── RESUMEN.md
│   └── RESUMEN-ANALISIS.md
├── sprints/
│   ├── SPRINT-3-TASKS.md       ← ⭐ Sprint 3 detallado (~2,500 líneas)
│   ├── SPRINT-4-TASKS.md       ← Sprint 4 detallado (~800 líneas)
│   ├── SPRINT-ENTITIES-ADAPTATION.md
│   └── SPRINT-TRACKING.md
├── tracking/
│   ├── REGLAS.md
│   ├── SPRINT-STATUS.md
│   ├── logs/
│   ├── errors/
│   ├── decisions/
│   └── reviews/
└── assets/
    ├── workflows/
    └── scripts/

Total: ~3,650 líneas de documentación
```

---

## 🎯 Por Rol

### Soy el Implementador
→ Lee: **README.md** (sección "Análisis de Duplicación Docker")  
→ Ejecuta: **sprints/SPRINT-3-TASKS.md** tarea por tarea  
→ Tiempo: 16-20 horas Sprint 3

### Soy el Planificador
→ Lee: **README.md** completo  
→ Revisa: Estructura de sprints en INDEX.md  
→ Tiempo: 1-2 horas de lectura

### Soy el Reviewer
→ Lee: **INDEX.md** + README.md (métricas)  
→ Valida: Enfoque de consolidación Docker  
→ Tiempo: 30-60 minutos

### Quiero Entender el Problema Docker
→ Lee: **README.md** sección "Duplicación Docker"  
→ Ve: Tabla comparativa de 3 workflows  
→ Tiempo: 15-20 minutos

---

## 📈 Roadmap de Lectura

### Nivel 1: Overview (15 min)
1. INDEX.md (este archivo) - 5 min
2. README.md (solo "Resumen Ejecutivo") - 10 min

### Nivel 2: Entender Problema Docker (30 min)
1. README.md sección "Análisis de Duplicación Docker" - 20 min
2. sprints/SPRINT-3-TASKS.md Tarea 1 (solo leer) - 10 min

### Nivel 3: Contexto Completo (1 hora)
1. README.md completo - 25 min
2. sprints/SPRINT-3-TASKS.md (solo estructura) - 20 min
3. sprints/SPRINT-4-TASKS.md (solo estructura) - 15 min

### Nivel 4: Detalle Completo para Implementar (4-5 horas)
1. README.md - 25 min
2. sprints/SPRINT-3-TASKS.md completo - 3-4 horas
3. sprints/SPRINT-4-TASKS.md completo - 1 hora

---

## 🔥 Top 5 Tareas Críticas (Sprint 3)

Si solo tienes tiempo limitado, ejecuta estas:

1. **Tarea 1: Consolidar workflows Docker** (3-4 horas) 🔴
   - Archivo: sprints/SPRINT-3-TASKS.md, línea ~50
   - La MÁS CRÍTICA de worker
   - Eliminar build-and-push.yml y docker-only.yml

2. **Tarea 2: Migrar a Go 1.25** (45-60 min) 🟡
   - Archivo: sprints/SPRINT-3-TASKS.md, línea ~800
   - Actualizar go.mod de 1.24.10 → 1.25.3
   - Script incluido

3. **Tarea 4: Pre-commit hooks** (60-90 min) 🟡
   - Archivo: sprints/SPRINT-3-TASKS.md, línea ~1400
   - 7 validaciones automáticas
   - Copiar de api-mobile

4. **Tarea 5: Coverage threshold 33%** (45 min) 🟡
   - Archivo: sprints/SPRINT-3-TASKS.md, línea ~1800
   - Estandarizar con apis
   - Script incluido

5. **Tarea 10: Crear PR Sprint 3** (30 min)
   - Archivo: sprints/SPRINT-3-TASKS.md, línea ~2400
   - Template incluido

**Total:** ~6-8 horas (en lugar de 16-20h)

---

## 💡 Datos Clave de Worker

### Estado Actual
```yaml
Repositorio: edugo-worker
Tipo: Aplicación desplegable (Tipo A)
Workflows: 7 archivos
Success Rate: 70% (necesita atención)
Go Version: 1.24.10 (go.mod) vs 1.25 (workflows) ⚠️
Coverage: Sin threshold definido
Pre-commit: No configurado
```

### Problemas Identificados
```
🔴 P0: 3 workflows Docker (desperdicio crítico)
🔴 P0: release.yml fallando (Run 19485700108)
🟡 P1: Sin coverage threshold (vs 33% en apis)
🟡 P1: Go 1.25 inconsistente
🟡 P1: Pre-commit hooks faltantes
🟢 P2: Migrar a workflows reusables
```

### Métricas del Proyecto
```
Workers totales: 7 workflows
├── Duplicados Docker: 3 ❌
├── CI válido: 1 ✅
├── Tests: 1 ✅
├── Release manual: 1 ✅
└── Sync: 1 ✅

Líneas de código workflows: ~600 líneas
Duplicación estimada: ~250 líneas (42%)
```

---

## 🎯 Diferencias con Otros Proyectos

### vs api-mobile / api-administracion
```diff
+ Worker usa Go 1.25 en workflows (apis usan 1.23)
+ Worker tiene 3 workflows Docker (apis tienen 1)
- Worker NO tiene coverage threshold (apis tienen 33%)
- Worker NO tiene pre-commit hooks (apis sí tienen)
= Mismo patrón manual-release.yml
= Mismo patrón sync-main-to-dev.yml
```

### vs shared
```diff
+ Worker es aplicación desplegable (shared es librería)
+ Worker publica Docker images (shared publica Go modules)
- Worker NO tiene release por módulos
- Worker NO tiene compatibility matrix
= Ambos usan Go 1.25 como target
= Ambos tienen test.yml con coverage
```

### vs infrastructure
```diff
+ Worker tiene releases funcionales (infrastructure 80% fallo)
+ Worker tiene mejor CI (infrastructure falla)
- Worker duplica workflows Docker
= Ambos tienen CLI tools (worker es CLI en sí)
```

---

## 📋 Checklist Pre-Implementación

Antes de comenzar Sprint 3:
- [ ] Leer README.md completo (25 min)
- [ ] Entender análisis de duplicación Docker (15 min)
- [ ] Tener acceso al repositorio edugo-worker
- [ ] Tener rama dev actualizada
- [ ] Tener permisos para crear PR
- [ ] Tener tiempo disponible (mínimo 4-5 horas para Tarea 1)

---

## 🆘 Ayuda Rápida

### Pregunta: ¿Por dónde empiezo?
**Respuesta:** README.md → sprints/SPRINT-3-TASKS.md línea 50 (Tarea 1)

### Pregunta: ¿Por qué 3 workflows Docker?
**Respuesta:** README.md sección "Análisis de Duplicación Docker" explica en detalle.

### Pregunta: ¿Cuál workflow Docker mantener?
**Respuesta:** manual-release.yml (el más completo y con control fino).

### Pregunta: ¿Qué workflows eliminar?
**Respuesta:** build-and-push.yml y docker-only.yml (redundantes).

### Pregunta: ¿Cuánto tiempo necesito?
**Respuesta:** Sprint 3 completo = 16-20h. Modo rápido = 6-8h (top 5 tareas).

### Pregunta: ¿Puedo saltar tareas?
**Respuesta:** Tarea 1 es OBLIGATORIA (consolidación Docker). Resto según prioridad.

### Pregunta: ¿Los scripts funcionan?
**Respuesta:** Sí, diseñados para copiar/pegar y ejecutar.

### Pregunta: ¿Por qué Go 1.25 si go.mod dice 1.24.10?
**Respuesta:** Inconsistencia detectada. Sprint 3 Tarea 2 resuelve esto.

---

## 🔗 Referencias Externas

### Documentación Base
- [Análisis Estado Actual](../../01-ANALISIS-ESTADO-ACTUAL.md)
- [Propuestas de Mejora](../../02-PROPUESTAS-MEJORA.md)
- [Matriz Comparativa](../../04-MATRIZ-COMPARATIVA.md)

### Repositorio
- **URL:** https://github.com/EduGoGroup/edugo-worker
- **Ruta Local:** `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker`
- **Workflows:** `.github/workflows/`

### Otros Planes de Implementación
- [01-shared](../01-shared/INDEX.md) - Referencia para Go 1.25
- [03-api-mobile](../03-api-mobile/INDEX.md) - Referencia para pre-commit hooks
- [04-api-administracion](../04-api-administracion/INDEX.md) - Referencia para coverage

---

## 📊 Métricas del Plan

| Métrica | Valor |
|---------|-------|
| Archivos totales | 4 markdown |
| Líneas totales | ~3,650 |
| Tamaño total | ~95 KB |
| Scripts incluidos | ~55 bash scripts |
| Tareas detalladas | 20 (12+8) |
| Tiempo estimado | 28-36 horas |
| Sprints cubiertos | 2 de 4 |
| Nivel de detalle | Ultra-alto |

---

## 🎉 ¡Listo para Comenzar!

Has llegado al final del índice. Ahora tienes una visión completa de lo que hay disponible.

**Siguiente paso recomendado:**
```bash
open README.md
# Leer contexto completo (25 min)
```

O si ya estás listo:
```bash
open sprints/SPRINT-3-TASKS.md
# Ir a línea 50 y comenzar con Tarea 1: Consolidación Docker
```

---

## ⚠️ Aviso Importante

**CRÍTICO:** La Tarea 1 de Sprint 3 (Consolidación de workflows Docker) es la MÁS IMPORTANTE de todo el plan de worker. No saltarla ni posponerla.

**Razones:**
1. Desperdicio de recursos (3 workflows haciendo lo mismo)
2. Confusión para el equipo (¿cuál usar?)
3. Potencial de tags duplicados/conflictivos
4. release.yml fallando actualmente

**Tiempo estimado Tarea 1:** 3-4 horas
**ROI:** Alto (elimina ~250 líneas duplicadas, resuelve fallos)

---

**Generado por:** Claude Code  
**Fecha:** 19 de Noviembre, 2025  
**Versión:** 1.0  
**Para:** edugo-worker - Worker de procesamiento asíncrono


---

## 🎯 Sistema de Seguimiento de Sprints

**Nuevo:** Sistema completo de tracking y control de ejecución de sprints.

### Documentación:
- **[SPRINT-TRACKING.md](sprints/SPRINT-TRACKING.md)** - Punto de entrada, guía de uso
- **[REGLAS.md](tracking/REGLAS.md)** - Reglas completas de ejecución
- **[SPRINT-STATUS.md](tracking/SPRINT-STATUS.md)** - Estado en tiempo real

### Características:
- 🎯 **3 Fases:** Implementación → Resolución Stubs → Validación/CI/CD
- 📊 **Tracking tiempo real:** Siempre sabes dónde estás
- 📝 **Documentación automática:** Errores y decisiones registradas
- ⏱️ **Control CI/CD:** Timeout de 5 minutos con polling
- 🤖 **Clasificación Copilot:** Manejo inteligente de comentarios

**Ver:** [SPRINT-TRACKING.md](sprints/SPRINT-TRACKING.md) para comenzar.


---

## 🎓 Lecciones Aprendidas de api-mobile

**NUEVO:** El proyecto piloto **api-mobile** completó SPRINT-4 FASE 1 y FASE 2, documentando 5 problemas críticos y sus soluciones.

### 📚 Documento Clave

**[SPRINT-4-LESSONS-LEARNED.md](./SPRINT-4-LESSONS-LEARNED.md)** ⭐⭐⭐

**Lee este documento ANTES de empezar Sprint 4 para:**
- ✅ Evitar 90 minutos de debugging
- ✅ Aplicar fixes desde el inicio
- ✅ Migrar en ~50 minutos (vs 105 min de api-mobile)

### Problemas que Evitarás

1. ✅ Permisos de workflows reusables
2. ✅ Subdirectorio no permitido
3. ✅ GITHUB_TOKEN reservado
4. ✅ Incompatibilidad golangci-lint-action
5. ✅ Incompatibilidad Go 1.25

### Plantilla Correcta

```yaml
lint:
  uses: EduGoGroup/edugo-infrastructure/.github/workflows/reusable-go-lint.yml@main
  with:
    go-version: "1.25"
    args: "--timeout=5m"
```

**Lee el documento completo para detalles y comandos de validación.**

---

