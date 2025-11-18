# 📋 Plan de Reorganización - edugo-worker
## Fecha: 17 de Noviembre, 2025

## 🎯 Objetivo

Reorganizar la documentación de `edugo-worker` siguiendo el patrón exitoso implementado en `edugo-api-administracion`, eliminando duplicación y mejorando la estructura organizacional.

---

## 📊 Situación Actual vs Deseada

### ANTES (Situación Actual)
```
docs/
└── isolated/
    ├── START_HERE.md
    ├── EXECUTION_PLAN.md
    ├── WORKFLOW_ORCHESTRATION.md      ← Mezclado con contenido específico
    ├── TRACKING_SYSTEM.md             ← Mezclado con contenido específico
    ├── PHASE2_BRIDGE_TEMPLATE.md      ← Mezclado con contenido específico
    ├── PROGRESS_TEMPLATE.json         ← Mezclado con contenido específico
    ├── README.md
    ├── 01-Context/
    ├── 02-Requirements/
    ├── 03-Design/
    ├── 04-Implementation/
    │   ├── Sprint-00-Integrar-Infrastructure/
    │   └── Sprint-01 al 06/
    ├── 05-Testing/
    ├── 06-Deployment/
    └── worker/                         ❌ DUPLICACIÓN COMPLETA (~50 archivos)
        ├── START_HERE.md              ← IDÉNTICO al de arriba
        ├── EXECUTION_PLAN.md          ← IDÉNTICO
        ├── 01-Context/                ← IDÉNTICO
        ├── 02-Requirements/           ← IDÉNTICO
        ├── 03-Design/                 ← IDÉNTICO
        ├── 04-Implementation/         ⚠️ Sin Sprint-00
        │   └── Sprint-01 al 06/
        ├── 05-Testing/                ← IDÉNTICO
        └── 06-Deployment/             ← IDÉNTICO

Problemas:
- ❌ ~95% de duplicación (~600KB)
- ❌ Templates mezclados con contenido específico
- ❌ Dos puntos de entrada (confusión)
- ❌ Sprint-00 faltante en worker/
- ❌ Estado real no documentado
```

### DESPUÉS (Situación Deseada)
```
docs/
├── ANALISIS_DOCUMENTACION_2025-11-17.md       ✅ Nuevo
├── PLAN_REORGANIZACION_2025-11-17.md          ✅ Nuevo
├── REORGANIZACION_EJECUTADA_2025-11-17.md     ✅ Nuevo (después de ejecutar)
│
├── workflow-templates/                         ✅ Nuevo - Templates reutilizables
│   ├── README.md
│   ├── WORKFLOW_ORCHESTRATION.md
│   ├── TRACKING_SYSTEM.md
│   ├── PHASE2_BRIDGE_TEMPLATE.md
│   └── PROGRESS_TEMPLATE.json
│
└── isolated/                                   ✅ Limpio - Sin duplicación
    ├── START_HERE.md                          ✅ Actualizado con estado real
    ├── EXECUTION_PLAN.md
    ├── README.md
    ├── 01-Context/
    ├── 02-Requirements/
    ├── 03-Design/
    ├── 04-Implementation/
    │   ├── Sprint-00-Integrar-Infrastructure/  ✅ Único sprint-00
    │   └── Sprint-01 al 06/
    ├── 05-Testing/
    └── 06-Deployment/

Beneficios:
- ✅ 0% duplicación (50 archivos eliminados)
- ✅ Templates separados (reutilizables)
- ✅ Un solo punto de entrada (claridad)
- ✅ Estado real documentado
- ✅ ~600KB liberados
```

---

## 📝 Plan de Ejecución

### FASE 1: Backup y Preparación (5 minutos)

#### Paso 1.1: Crear backup de seguridad
```bash
# Crear backup completo de docs/
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker
cp -r docs docs_backup_2025-11-17

# Verificar backup
ls -lah docs_backup_2025-11-17/
```

**Criterio de éxito:**
- [ ] Carpeta `docs_backup_2025-11-17/` existe
- [ ] Contiene todos los archivos de `docs/`

---

### FASE 2: Eliminar Duplicación (5 minutos)

#### Paso 2.1: Eliminar carpeta worker/ duplicada
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Eliminar duplicación
rm -rf docs/isolated/worker/

# Verificar eliminación
ls -la docs/isolated/ | grep worker  # Debe retornar vacío
```

**Criterio de éxito:**
- [ ] `docs/isolated/worker/` no existe
- [ ] ~50 archivos eliminados
- [ ] ~600KB liberados

---

### FASE 3: Crear workflow-templates/ (15 minutos)

#### Paso 3.1: Crear estructura de workflow-templates/
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Crear carpeta
mkdir -p docs/workflow-templates/

# Mover templates genéricos
mv docs/isolated/WORKFLOW_ORCHESTRATION.md docs/workflow-templates/
mv docs/isolated/TRACKING_SYSTEM.md docs/workflow-templates/
mv docs/isolated/PHASE2_BRIDGE_TEMPLATE.md docs/workflow-templates/
mv docs/isolated/PROGRESS_TEMPLATE.json docs/workflow-templates/
```

**Criterio de éxito:**
- [ ] Carpeta `docs/workflow-templates/` existe
- [ ] Contiene 4 archivos movidos
- [ ] `docs/isolated/` ya no contiene esos archivos

---

#### Paso 3.2: Crear README.md en workflow-templates/
```bash
cat > docs/workflow-templates/README.md << 'EOF'
# 📚 Workflow Templates - EduGo

## 🎯 Propósito

Templates reutilizables para mantener consistencia en la documentación de todos los proyectos del ecosistema EduGo.

---

## 📁 Archivos Disponibles

### 1. WORKFLOW_ORCHESTRATION.md
**Descripción:** Sistema de orquestación de 2 fases (Claude Code Web + Local)

**Uso:**
- Workflow para desarrollo distribuido
- Coordinación entre fase de análisis (Web) y ejecución (Local)
- Sistema de bridge para transferir contexto

**Cuándo usar:** Proyectos que requieren análisis previo antes de implementación

---

### 2. TRACKING_SYSTEM.md
**Descripción:** Sistema de tracking de progreso con PROGRESS.json

**Uso:**
- Monitoreo de progreso en tiempo real
- Tracking de sprints y tareas
- Métricas de avance

**Cuándo usar:** Proyectos con múltiples sprints o fases

---

### 3. PHASE2_BRIDGE_TEMPLATE.md
**Descripción:** Template para documentos puente entre fases

**Uso:**
- Transferir contexto de Fase 1 (Análisis) a Fase 2 (Ejecución)
- Documentar decisiones tomadas
- Checklist de pre-requisitos

**Cuándo usar:** Al iniciar Fase 2 de un proyecto con workflow de 2 fases

---

### 4. PROGRESS_TEMPLATE.json
**Descripción:** Template JSON estructurado para tracking

**Uso:**
- Formato estándar de tracking
- Integración con herramientas de monitoreo
- Generación de reportes automáticos

**Cuándo usar:** Proyectos que requieren tracking automatizado

---

## 🚀 Cómo Usar Estos Templates

### Opción 1: Copiar a Nuevo Proyecto

```bash
# Copiar todos los templates a un nuevo proyecto
cp -r docs/workflow-templates/* /path/to/nuevo-proyecto/docs/workflow-templates/
```

### Opción 2: Referenciar Desde Otro Proyecto

```markdown
<!-- En docs de otro proyecto -->
## Workflows

Ver workflows estándar en:
- [edugo-worker/docs/workflow-templates/](../edugo-worker/docs/workflow-templates/)
```

### Opción 3: Adaptar Según Necesidad

1. Copiar template específico
2. Modificar según contexto del proyecto
3. Mantener estructura base
4. Documentar cambios

---

## 📋 Proyectos Usando Estos Templates

- ✅ `edugo-worker` (este proyecto)
- ✅ `edugo-api-administracion`
- ✅ `edugo-api-mobile`

---

## 🔄 Versionado

**Versión actual:** 1.0.0
**Última actualización:** 17 de Noviembre, 2025
**Mantenido por:** Equipo EduGo

---

## 📞 Soporte

Si necesitas ayuda con estos templates:
1. Revisar ejemplos en proyectos listados arriba
2. Consultar documentación específica dentro de cada template
3. Contactar al equipo de arquitectura

---

## 🎓 Filosofía

> **"Documentación consistente = Onboarding rápido + Mantenimiento sencillo"**

Estos templates existen para:
- ✅ Mantener coherencia entre proyectos
- ✅ Reducir tiempo de setup de nuevos proyectos
- ✅ Facilitar transferencia de conocimiento
- ✅ Estandarizar procesos de desarrollo

EOF
```

**Criterio de éxito:**
- [ ] `docs/workflow-templates/README.md` creado
- [ ] Contiene documentación de todos los templates
- [ ] Incluye ejemplos de uso

---

### FASE 4: Actualizar START_HERE.md (20 minutos)

#### Paso 4.1: Leer START_HERE.md actual
```bash
cat docs/isolated/START_HERE.md
```

#### Paso 4.2: Agregar sección de estado al inicio
Agregar después del título principal:

```markdown
## ⚠️ ESTADO ACTUAL DEL PROYECTO

**Última actualización:** 17 de Noviembre, 2025

### Estado Funcional
✅ **CÓDIGO FUNCIONANDO**
- Tests de integración pasando
- Estructura de proyecto completa
- Lógica de negocio implementada

### Estado Técnico
⚠️ **REQUIERE INTEGRACIÓN CON INFRASTRUCTURE**

---

### 📊 Integraciones Pendientes

#### 1. edugo-infrastructure v0.2.0
- **Estado actual:** ❌ NO INTEGRADO
- **Versión en go.mod:** N/A (no existe)
- **Versión requerida:** v0.2.0
- **Propósito:** Validación de schemas de eventos RabbitMQ
- **Acción:** Ejecutar Sprint-00

**¿Qué incluye infrastructure?**
- Schemas de validación de eventos (`material-uploaded-v1`, `assessment-generated-v1`)
- Validador de eventos centralizado
- Contratos de mensajería estandarizados

---

#### 2. edugo-shared (DESACTUALIZADO)
- **Estado actual:** ⚠️ v0.5.0 (desactualizado)
- **Versión requerida:** v0.7.0
- **Acción:** Ejecutar Sprint-00

**Módulos actuales (v0.5.0):**
- ✅ `bootstrap` - Inicialización
- ✅ `common` - Utilidades comunes
- ✅ `database/postgres` - Helpers de PostgreSQL
- ✅ `lifecycle` - Gestión de ciclo de vida
- ✅ `logger` - Logging estructurado
- ✅ `testing` v0.6.2 - Testing utilities

**Módulos faltantes (requiere v0.7.0):**
- ❌ `evaluation` - Modelos compartidos de evaluación (Assessment, Question, etc.)
- ❌ `messaging/rabbit` - Dead Letter Queue, retry logic, configuración estandarizada
- ❌ `database/mongodb` - Helpers de MongoDB, conexión centralizada

---

### ⚠️ ACCIÓN REQUERIDA

**EJECUTAR SPRINT-00 ANTES DE CONTINUAR CON DESARROLLO**

```bash
# Ver plan completo de Sprint-00
cat docs/isolated/04-Implementation/Sprint-00-Integrar-Infrastructure/README.md
cat docs/isolated/04-Implementation/Sprint-00-Integrar-Infrastructure/TASKS.md

# Duración estimada: 1 hora
# Prioridad: CRÍTICA
```

**¿Por qué es crítico?**
1. ✅ Validación de eventos evita errores en producción
2. ✅ DLQ de shared maneja errores automáticamente
3. ✅ Modelos compartidos evitan duplicación
4. ✅ Helpers de MongoDB reducen código boilerplate

---

### 📚 Documentación de Reorganización

**Documentos de análisis:**
- `docs/ANALISIS_DOCUMENTACION_2025-11-17.md` - Análisis de duplicación y estado
- `docs/PLAN_REORGANIZACION_2025-11-17.md` - Plan de reorganización
- `docs/REORGANIZACION_EJECUTADA_2025-11-17.md` - Cambios ejecutados (post-reorganización)

**Workflow templates:**
- `docs/workflow-templates/` - Templates reutilizables para todos los proyectos EduGo

---
```

**Criterio de éxito:**
- [ ] Sección de estado agregada al inicio de START_HERE.md
- [ ] Estado real documentado (v0.5.0, sin infrastructure)
- [ ] Acción requerida clara (ejecutar Sprint-00)
- [ ] Referencias a documentos de análisis

---

### FASE 5: Crear Documentos de Análisis (Ya Completado)

**Documentos creados:**
- ✅ `docs/ANALISIS_DOCUMENTACION_2025-11-17.md` (ya existe)
- ✅ `docs/PLAN_REORGANIZACION_2025-11-17.md` (este documento)

**Pendiente (post-ejecución):**
- ⏳ `docs/REORGANIZACION_EJECUTADA_2025-11-17.md` (crear después de ejecutar el plan)

---

### FASE 6: Validación (10 minutos)

#### Paso 6.1: Verificar estructura final
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Verificar eliminación de duplicados
test ! -d docs/isolated/worker && echo "✅ worker/ eliminado" || echo "❌ worker/ aún existe"

# Verificar creación de workflow-templates
test -d docs/workflow-templates && echo "✅ workflow-templates/ creado" || echo "❌ Falta workflow-templates/"

# Contar archivos en workflow-templates
ls -1 docs/workflow-templates/ | wc -l  # Debe ser 5 (4 templates + README)

# Verificar archivos movidos NO están en isolated/
test ! -f docs/isolated/WORKFLOW_ORCHESTRATION.md && echo "✅ Template movido" || echo "❌ Template no movido"
test ! -f docs/isolated/TRACKING_SYSTEM.md && echo "✅ Template movido" || echo "❌ Template no movido"
test ! -f docs/isolated/PHASE2_BRIDGE_TEMPLATE.md && echo "✅ Template movido" || echo "❌ Template no movido"
test ! -f docs/isolated/PROGRESS_TEMPLATE.json && echo "✅ Template movido" || echo "❌ Template no movido"

# Verificar archivos en workflow-templates/
test -f docs/workflow-templates/README.md && echo "✅ README creado" || echo "❌ Falta README"
test -f docs/workflow-templates/WORKFLOW_ORCHESTRATION.md && echo "✅ Template existe" || echo "❌ Template falta"
test -f docs/workflow-templates/TRACKING_SYSTEM.md && echo "✅ Template existe" || echo "❌ Template falta"
test -f docs/workflow-templates/PHASE2_BRIDGE_TEMPLATE.md && echo "✅ Template existe" || echo "❌ Template falta"
test -f docs/workflow-templates/PROGRESS_TEMPLATE.json && echo "✅ Template existe" || echo "❌ Template falta"
```

**Criterio de éxito:**
- [ ] Todos los checks retornan ✅
- [ ] 0 errores en validación

---

#### Paso 6.2: Verificar START_HERE.md actualizado
```bash
# Verificar que START_HERE.md tiene la sección de estado
grep -q "ESTADO ACTUAL DEL PROYECTO" docs/isolated/START_HERE.md && \
  echo "✅ START_HERE.md actualizado" || \
  echo "❌ START_HERE.md no actualizado"

# Verificar mención de Sprint-00
grep -q "Sprint-00" docs/isolated/START_HERE.md && \
  echo "✅ Menciona Sprint-00" || \
  echo "❌ No menciona Sprint-00"

# Verificar mención de v0.5.0 (estado actual)
grep -q "v0.5.0" docs/isolated/START_HERE.md && \
  echo "✅ Documenta versión actual" || \
  echo "❌ No documenta versión actual"
```

**Criterio de éxito:**
- [ ] Todos los checks retornan ✅

---

#### Paso 6.3: Verificar documentos de análisis
```bash
# Verificar existencia de documentos
test -f docs/ANALISIS_DOCUMENTACION_2025-11-17.md && \
  echo "✅ Análisis existe" || \
  echo "❌ Falta análisis"

test -f docs/PLAN_REORGANIZACION_2025-11-17.md && \
  echo "✅ Plan existe" || \
  echo "❌ Falta plan"
```

**Criterio de éxito:**
- [ ] Todos los documentos existen

---

### FASE 7: Crear Documento de Ejecución (15 minutos)

#### Paso 7.1: Crear REORGANIZACION_EJECUTADA_2025-11-17.md

Este documento se creará DESPUÉS de ejecutar todas las fases anteriores, documentando:
- Cambios realizados
- Métricas antes/después
- Validaciones ejecutadas
- Próximos pasos

**Plantilla:**
```markdown
# 📋 Reorganización Ejecutada - 17 Noviembre 2025

## 🎯 Resumen Ejecutivo

Se completó la reorganización de documentación de edugo-worker siguiendo el patrón de edugo-api-administracion.

## ✅ Cambios Realizados

### 1. Eliminación de Duplicación
- ✅ Eliminada carpeta `docs/isolated/worker/`
- ✅ ~50 archivos duplicados eliminados
- ✅ ~600KB liberados

### 2. Creación de workflow-templates/
- ✅ Carpeta creada
- ✅ 4 templates movidos
- ✅ README.md creado

### 3. Actualización de START_HERE.md
- ✅ Sección de estado agregada
- ✅ Estado real documentado
- ✅ Acción requerida clara

### 4. Documentos de Análisis
- ✅ ANALISIS_DOCUMENTACION_2025-11-17.md
- ✅ PLAN_REORGANIZACION_2025-11-17.md
- ✅ REORGANIZACION_EJECUTADA_2025-11-17.md (este)

## 📊 Métricas Antes/Después

| Métrica | Antes | Después | Mejora |
|---------|-------|---------|--------|
| Archivos duplicados | ~50 | 0 | ✅ 100% |
| Tamaño duplicado | ~600KB | 0 | ✅ 600KB |
| Puntos de entrada | 2 | 1 | ✅ 50% |
| Templates separados | No | Sí | ✅ |
| Estado documentado | No | Sí | ✅ |

## 🎯 Próximos Pasos

1. Ejecutar Sprint-00 (integración con infrastructure)
2. Actualizar go.mod a shared v0.7.0
3. Validar tests después de integración

**Fecha:** 17 de Noviembre, 2025
**Ejecutado por:** [Tu nombre]
**Aprobado por:** Jhoan Medina
```

---

### FASE 8: Git Commit (5 minutos)

#### Paso 8.1: Verificar cambios
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Ver estado
git status

# Ver cambios
git diff docs/
```

#### Paso 8.2: Crear commit
```bash
# Agregar cambios
git add docs/

# Commit con mensaje descriptivo
git commit -m "docs: reorganizar documentación siguiendo patrón de api-admin

- Eliminar duplicación docs/isolated/worker/ (~50 archivos, ~600KB)
- Crear docs/workflow-templates/ con templates reutilizables
- Actualizar START_HERE.md con estado real de integración
- Documentar análisis y plan de reorganización

Refs: ANALISIS_DOCUMENTACION_2025-11-17.md, PLAN_REORGANIZACION_2025-11-17.md
Patrón seguido: edugo-api-administracion"
```

**Criterio de éxito:**
- [ ] Commit creado exitosamente
- [ ] Mensaje descriptivo y claro
- [ ] Referencias a documentos de análisis

---

## ⏱️ Estimación Total

| Fase | Duración | Acumulado |
|------|----------|-----------|
| FASE 1: Backup | 5 min | 5 min |
| FASE 2: Eliminar duplicación | 5 min | 10 min |
| FASE 3: Crear workflow-templates/ | 15 min | 25 min |
| FASE 4: Actualizar START_HERE.md | 20 min | 45 min |
| FASE 5: Documentos análisis | 0 min (ya hecho) | 45 min |
| FASE 6: Validación | 10 min | 55 min |
| FASE 7: Doc de ejecución | 15 min | 70 min |
| FASE 8: Git commit | 5 min | **75 min** |

**Duración total:** ~1.5 horas

---

## ✅ Checklist General

### Pre-Ejecución
- [ ] Leer `ANALISIS_DOCUMENTACION_2025-11-17.md`
- [ ] Leer este plan completo
- [ ] Tener backup de docs/
- [ ] Estar en rama correcta (dev)

### Durante Ejecución
- [ ] FASE 1: Backup creado
- [ ] FASE 2: Duplicación eliminada
- [ ] FASE 3: workflow-templates/ creado
- [ ] FASE 4: START_HERE.md actualizado
- [ ] FASE 5: (ya completado)
- [ ] FASE 6: Todas las validaciones pasan
- [ ] FASE 7: Doc de ejecución creado
- [ ] FASE 8: Commit realizado

### Post-Ejecución
- [ ] Verificar estructura final con `tree docs/`
- [ ] Leer START_HERE.md actualizado
- [ ] Verificar que no hay duplicación
- [ ] Planificar ejecución de Sprint-00

---

## 🎯 Beneficios Esperados

### Inmediatos
- ✅ Eliminación de ~600KB duplicados
- ✅ Un solo punto de entrada (claridad)
- ✅ Estado real documentado
- ✅ Templates reutilizables separados

### A Largo Plazo
- ✅ Mantenimiento simplificado (cambios en un solo lugar)
- ✅ Onboarding más rápido de nuevos desarrolladores
- ✅ Consistencia con api-administracion y api-mobile
- ✅ Templates pueden reutilizarse en futuros proyectos

---

## 📞 Soporte

Si encuentras problemas durante la ejecución:

1. **Verificar backup:** `ls -la docs_backup_2025-11-17/`
2. **Restaurar si es necesario:** `rm -rf docs/ && cp -r docs_backup_2025-11-17/ docs/`
3. **Revisar logs de git:** `git status` y `git diff`
4. **Consultar ANALISIS_DOCUMENTACION_2025-11-17.md** para contexto

---

**Plan creado:** 17 de Noviembre, 2025
**Proyecto:** edugo-worker
**Patrón seguido:** edugo-api-administracion
**Estado:** ✅ PLAN LISTO PARA EJECUCIÓN
**Aprobación pendiente:** Jhoan Medina
