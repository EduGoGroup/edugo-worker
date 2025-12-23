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
FASE 0: Actualización de Dependencias (Prerequisito)
  ├── Actualizar edugo-infrastructure a última versión
  ├── Actualizar edugo-shared a última versión
  └── Validar compilación y tests

FASE 1: Funcionalidad Crítica (2-3 semanas)
  ├── Implementar routing real de eventos
  ├── Eliminar código deprecado
  └── Refactorizar bootstrap

FASE 2: Integraciones Externas (3-4 semanas)
  ├── Implementar cliente OpenAI
  ├── Implementar extracción PDF
  └── Implementar cliente S3

FASE 3: Testing y Calidad (2-3 semanas)
  ├── Aumentar cobertura de tests
  ├── Crear mocks e interfaces
  └── Agregar tests de integración

FASE 4: Observabilidad y Resiliencia (2-3 semanas)
  ├── Métricas Prometheus
  ├── Circuit breakers
  └── Health checks
```

---

## 📂 Organización del Repositorio

```
plan-mejoras/
├── README.md                    # Este archivo
├── fase-0/                      # Actualización dependencias
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
├── fase-1/                      # Funcionalidad crítica
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
├── fase-2/                      # Integraciones externas
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
├── fase-3/                      # Testing y calidad
│   ├── README.md
│   ├── TAREAS.md
│   └── VALIDACION.md
└── fase-4/                      # Observabilidad
    ├── README.md
    ├── TAREAS.md
    └── VALIDACION.md
```

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

| Fase | Duración Estimada | Complejidad | Riesgo |
|------|-------------------|-------------|--------|
| Fase 0 | 1-2 días | Baja | Bajo |
| Fase 1 | 2-3 semanas | Alta | Medio |
| Fase 2 | 3-4 semanas | Alta | Alto |
| Fase 3 | 2-3 semanas | Media | Bajo |
| Fase 4 | 2-3 semanas | Media | Medio |

**Total:** 10-14 semanas (~2.5-3.5 meses)

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

- [Código Deprecado](../documents/mejoras/CODIGO_DEPRECADO.md)
- [Deuda Técnica](../documents/mejoras/DEUDA_TECNICA.md)
- [Refactorizaciones](../documents/mejoras/REFACTORING.md)
- [Roadmap](../documents/mejoras/ROADMAP.md)

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
| Fase 0 | ⏳ Pendiente | - | - | - |
| Fase 1 | ⏸️ No iniciada | - | - | - |
| Fase 2 | ⏸️ No iniciada | - | - | - |
| Fase 3 | ⏸️ No iniciada | - | - | - |
| Fase 4 | ⏸️ No iniciada | - | - | - |

**Leyenda:**
- ⏸️ No iniciada
- ⏳ Pendiente / En preparación
- 🚧 En desarrollo
- 🔍 En revisión
- ✅ Completada

---

**Última actualización:** 2024-12-23
**Versión del plan:** 1.0
