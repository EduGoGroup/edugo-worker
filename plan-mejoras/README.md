# Plan de Mejoras - EduGo Worker

> **Objetivo:** Implementar todas las mejoras identificadas en la documentación técnica de forma incremental y segura.
> 
> **Última actualización:** 2024-12-23

---

## 📋 Resumen Ejecutivo

Este plan organiza la implementación de mejoras en **4 fases**, cada una con su propia rama y proceso de validación antes de integración a `dev`.

### Versiones de Dependencias Base

**Versiones Actuales:**
- `edugo-infrastructure/mongodb`: v0.10.1
- `edugo-shared/bootstrap`: v0.9.0
- `edugo-shared/common`: v0.7.0
- `edugo-shared/database/postgres`: v0.7.0
- `edugo-shared/lifecycle`: v0.7.0
- `edugo-shared/logger`: v0.7.0
- `edugo-shared/testing`: v0.7.0

---

## 🎯 Estructura de Fases

```
📂 ARCHIVADO (Fases Completadas)
  ├── FASE 0: Actualización de Dependencias ✅
  ├── FASE 1: Funcionalidad Crítica ✅
  ├── FASE 2: Integraciones Externas ✅
  └── FASE 2.5: Homologación Material Assessment ✅


FASE 3: Testing y Calidad (2-3 semanas)
  ├── Aumentar cobertura de tests
  ├── Crear mocks e interfaces
  └── Agregar tests de integración

FASE 4: Observabilidad y Resiliencia (2-3 semanas)
  ├── Métricas Prometheus
  ├── Circuit breakers
  └── Health checks

FASE 5: Integraciones Core Avanzadas (2-3 semanas) 🆕 [PT-008]
  ├── Implementar cliente AWS S3 real
  ├── Implementar extractor de PDF
  ├── Implementar cliente OpenAI
  └── Integrar en MaterialUploadedProcessor

FASE 6: Sistemas de Notificaciones (3-4 semanas) 🆕 [PT-009]
  ├── Implementar servicio de email (SendGrid)
  ├── Implementar push notifications (Firebase)
  ├── Crear templates de email
  ├── AssessmentAttemptProcessor (alertas de score bajo)
  └── StudentEnrolledProcessor (emails de bienvenida)
```

---

## 📂 Organización del Repositorio

```
plan-mejoras/
├── README.md                    # Este archivo
├── archivado/                   # Fases completadas (0, 1, 2, 2.5)

├── fase-3/                      # Testing y calidad
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
├── fase-4/                      # Observabilidad
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
├── fase-5/                      # 🆕 Integraciones Core (PT-008)
│   ├── README.md
│   ├── PLAN_TECNICO.md         # Plan técnico detallado
│   ├── TAREAS.md
│   └── VALIDACION.md
└── fase-6/                      # 🆕 Notificaciones (PT-009)
    ├── README.md
    ├── PLAN_TECNICO.md         # Plan técnico detallado
    ├── TAREAS.md
    └── VALIDACION.md
```

**Nota:** Las Fases 5 y 6 son nuevas tareas agregadas desde los planes PT-008 y PT-009 del repositorio edugo_analisis. Incluyen planes técnicos detallados con ejemplos de código completos.

---

## 📖 Documentación por Fase

Cada fase cuenta con dos niveles de documentación:

### Nivel 1: Documentación Organizacional
- **README.md**: Visión general de la fase, objetivos y criterios de aceptación
- **TAREAS.md**: Lista de tareas y checklist
- **VALIDACION.md**: Criterios de validación y testing

### Nivel 2: Documentación Técnica (Fases 5 y 6)
- **PLAN_TECNICO.md**: Implementación técnica detallada con:
  - Código de ejemplo completo
  - Paso a paso de implementación
  - Ejemplos de commits
  - Checklist técnico específico
  - Variables de entorno y configuración

**Uso recomendado:**
1. Lee primero el `README.md` para entender el contexto y objetivos
2. Consulta `PLAN_TECNICO.md` (si existe) para la implementación detallada
3. Usa `TAREAS.md` para tracking del progreso
4. Valida con `VALIDACION.md` antes de crear el PR

---

## 🔄 Workflow por Fase

### 1. Preparación
```bash
# Crear rama desde dev
git checkout dev
git pull origin dev
git checkout -b feature/fase-N-descripcion
```

### 2. Desarrollo
- Implementar tareas según `TAREAS.md`
- Hacer commits atómicos y descriptivos
- Seguir convenciones de commits

### 3. Validación Local
Antes de hacer PR, ejecutar:

```bash
# Compilación
make build

# Tests unitarios
make test

# Tests de integración (si aplica)
make test-integration

# Linters
make lint

# Cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 4. Pull Request
- Crear PR desde `feature/fase-N-*` hacia `dev`
- Seguir template de PR
- Incluir checklist de validación
- Referencias a issues/documentos

### 5. Revisión y Merge
- Code review por al menos 1 persona
- CI/CD debe pasar todos los checks
- Merge a `dev` solo si todo está verde

### 6. Commit de Fin de Fase
Una vez mergeado el PR:
```bash
git checkout dev
git pull origin dev
git tag fase-N-complete
git push origin fase-N-complete
```

---

## ✅ Criterios de Aceptación por Fase

### Fase 0
- [ ] Todas las dependencias actualizadas a última versión
- [ ] `make build` exitoso
- [ ] Todos los tests existentes pasan
- [ ] No hay warnings de deprecación

### Fase 1
- [ ] Worker procesa eventos realmente (no mock)
- [ ] Código deprecado eliminado o marcado
- [ ] Bootstrap simplificado
- [ ] Cobertura de tests >60%

### Fase 2
- [ ] Integración OpenAI funcional
- [ ] Extracción de PDF funcional
- [ ] Cliente S3 funcional
- [ ] Tests con mocks para servicios externos

### Fase 2.5
- [ ] Verificado uso correcto de colección material_assessment_worker
- [ ] Dependencia edugo-infrastructure actualizada
- [ ] Entity MaterialAssessment completa con todos los campos
- [ ] Todos los tests pasan sin errores

### Fase 3
- [ ] Cobertura de tests >80%
- [ ] Mocks e interfaces implementados
- [ ] Tests de integración pasando
- [ ] Documentación de testing actualizada

### Fase 4
- [ ] Métricas expuestas en `/metrics`
- [ ] Health checks en `/health`
- [ ] Circuit breakers configurados
- [ ] Dashboards Grafana creados

---

## 📊 Estimaciones de Tiempo

| Fase | Duración Estimada | Complejidad | Riesgo | Estado |
|------|-------------------|-------------|--------|---------|
| Fase 0 | 1-2 días | Baja | Bajo | ✅ Completada |
| Fase 1 | 2-3 semanas | Alta | Medio | ✅ Completada |
| Fase 2 | 3-4 semanas | Alta | Alto | ✅ Completada |
| Fase 2.5 | 1-2 días | Baja | Bajo | ✅ Completada |
| Fase 3 | 2-3 semanas | Media | Bajo | ⏸️ Pendiente |
| Fase 4 | 2-3 semanas | Media | Medio | ⏸️ Pendiente |
| Fase 5 🆕 | 2-3 semanas | Alta | Alto | ⏸️ Pendiente |
| Fase 6 🆕 | 3-4 semanas | Media-Alta | Medio | ⏸️ Pendiente |

**Total Original:** 10-14 semanas (~2.5-3.5 meses)
**Total con Fases 5-6:** 15-21 semanas (~3.5-5 meses)

---

## 🚨 Gestión de Riesgos

### Riesgo: Dependencias rompen funcionalidad existente
**Mitigación:** Fase 0 dedicada solo a actualización y validación

### Riesgo: Cambios en OpenAI API
**Mitigación:** Abstraer cliente detrás de interfaz, versionar prompts

### Riesgo: Tests lentos por integraciones reales
**Mitigación:** Usar test doubles, Docker para tests de integración

### Riesgo: Desviación del plan original
**Mitigación:** Revisión semanal del plan, ajustar si es necesario

---

## 📝 Convenciones de Commits

```
feat(fase-N): descripción corta

Descripción detallada de los cambios.

Refs: #issue-number
Fase: N
```

**Tipos:**
- `feat`: Nueva funcionalidad
- `fix`: Corrección de bug
- `refactor`: Refactorización sin cambio de funcionalidad
- `test`: Agregar o modificar tests
- `docs`: Cambios en documentación
- `chore`: Cambios en build, dependencias, etc.

**Ejemplos:**
```
feat(fase-1): implementar ProcessorRegistry para routing de eventos

- Crear interfaz Processor
- Implementar Registry con map de processors
- Conectar registry a processMessage()
- Agregar tests unitarios

Refs: documents/mejoras/REFACTORING.md RF-002
Fase: 1
```

```
refactor(fase-1): simplificar bootstrap eliminando doble puntero

- Reemplazar customFactoriesWrapper con ResourceBuilder
- Eliminar doble puntero en factories
- Simplificar cleanup con defer
- Actualizar tests

Refs: documents/mejoras/REFACTORING.md RF-001
Fase: 1
```

---

## 📚 Referencias

### Documentación de Mejoras
- [Código Deprecado](../documents/mejoras/CODIGO_DEPRECADO.md)
- [Deuda Técnica](../documents/mejoras/DEUDA_TECNICA.md)
- [Refactorizaciones](../documents/mejoras/REFACTORING.md)
- [Roadmap](../documents/mejoras/ROADMAP.md)

### Planes Técnicos Detallados
- [Fase 5 - Plan Técnico (PT-008)](./fase-5/PLAN_TECNICO.md) - Integraciones Core: S3, PDF y OpenAI
- [Fase 6 - Plan Técnico (PT-009)](./fase-6/PLAN_TECNICO.md) - Sistemas de Notificaciones: Email, Push y Processors

### Planes Originales (edugo_analisis)
- Origen PT-008: `/edugo_analisis/plan-trabajo/08-worker-fase1/worker/PLAN.md`
- Origen PT-009: `/edugo_analisis/plan-trabajo/09-worker-fase2/worker/PLAN.md`

---

## 📞 Contacto y Soporte

- **Owner:** Equipo de Desarrollo EduGo Worker
- **Revisión de Plan:** Semanal los lunes
- **Issues:** GitHub Issues con label `plan-mejoras`
- **Documentación:** Carpeta `documents/`

---

## 📈 Progreso

| Fase | Estado | Inicio | Fin | PR |
|------|--------|--------|-----|-----|
| Fase 0 | ✅ Completada | 2025-12-23 | 2025-12-23 | [#28](https://github.com/EduGoGroup/edugo-worker/pull/28) |
| Fase 1 | ✅ Completada | 2025-12-23 | 2025-12-23 | [#29](https://github.com/EduGoGroup/edugo-worker/pull/29), [#30](https://github.com/EduGoGroup/edugo-worker/pull/30) |
| Fase 2 | ✅ Completada | 2025-12-23 | 2025-12-23 | [#31](https://github.com/EduGoGroup/edugo-worker/pull/31) |
| Fase 2.5 | ✅ Completada | 2025-12-23 | 2025-12-23 | [#32](https://github.com/EduGoGroup/edugo-worker/pull/32) |
| Fase 3 | ⏸️ No iniciada | - | - | - |
| Fase 4 | ⏸️ No iniciada | - | - | - |
| Fase 5 🆕 | ⏸️ No iniciada | - | - | - |
| Fase 6 🆕 | ⏸️ No iniciada | - | - | - |

**Leyenda:**
- ⏸️ No iniciada
- ⏳ Pendiente / En preparación
- 🚧 En desarrollo
- 🔍 En revisión
- ✅ Completada

---

**Última actualización:** 2025-12-23
**Versión del plan:** 2.0

**Cambios en v2.0:**
- 🆕 Agregadas **Fase 5: Integraciones Core Avanzadas** (PT-008)
  - Cliente AWS S3, Extractor PDF, Cliente OpenAI
  - Integración en MaterialUploadedProcessor
- 🆕 Agregadas **Fase 6: Sistemas de Notificaciones** (PT-009)
  - Email Service (SendGrid), Push Notifications (Firebase)
  - AssessmentAttemptProcessor, StudentEnrolledProcessor
- ✅ Planes técnicos detallados con código completo
- ✅ Documentación completa: README, TAREAS, VALIDACION, PLAN_TECNICO
- ✅ Estimaciones de tiempo actualizadas (total: 15-21 semanas)
- ✅ Tabla de progreso actualizada con estados reales

**Cambios en v1.2:**
- ✅ Agregados planes técnicos detallados (PLAN_TECNICO.md)
- ✅ Integrados planes PT-008 y PT-009 de edugo_analisis
- ✅ Documentación de dos niveles: organizacional y técnica
