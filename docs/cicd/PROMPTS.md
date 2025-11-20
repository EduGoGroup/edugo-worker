# 🎯 Prompts para Ejecutar Sprints

**Ubicación:** `00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/PROMPTS.md`  
**Propósito:** Definir prompts estándar y no ambiguos para cada fase  
**Fecha:** 20 de Noviembre, 2025

⚠️ **CONTEXTO DE UBICACIÓN:**
```
📍 Estás en: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/
📍 Todas las rutas mencionadas son relativas a esta carpeta
⚠️ NO uses archivos fuera de 05-worker/ (pueden ser de otros proyectos)
```

---

## 📖 CÓMO USAR ESTE ARCHIVO

### Para el Usuario:
1. Identifica la fase que quieres ejecutar (1, 2, o 3)
2. Copia el prompt correspondiente
3. Reemplaza `X` con el número de sprint (3, 4, etc.)
4. Pega el prompt en Claude

### Para Claude:
Este archivo define EXACTAMENTE qué hacer en cada fase.
**SIEMPRE lee primero:** `INDEX.md`

---

## 🎯 FASE 1: Implementación con Stubs

### Propósito:
Completar todas las tareas del sprint. Si una tarea requiere recursos externos (MongoDB, RabbitMQ, Docker), usar STUB y continuar.

### 📋 Prompt para el Usuario:

```
Ejecuta FASE 1 del SPRINT-X en edugo-worker.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/
- Sprint: SPRINT-X
- Fase: 1 - Implementación con Stubs
- Archivo de tareas: sprints/SPRINT-X-TASKS.md
- Reglas: tracking/REGLAS.md
- Estado: tracking/SPRINT-STATUS.md

Instrucciones:
1. Lee INDEX.md para orientarte
2. Lee tracking/SPRINT-STATUS.md para ver progreso actual
3. Lee sprints/SPRINT-X-TASKS.md
4. Ejecuta las tareas pendientes (marca ⏳ o 🔄)
5. Si una tarea requiere MongoDB/RabbitMQ/Docker:
   - Usa STUB/MOCK
   - Marca como ✅ (stub)
   - Documenta en tracking/decisions/TASK-X.X-BLOCKED.md
6. Si NO requiere recursos externos:
   - Implementa completamente
   - Marca como ✅
7. Actualiza tracking/SPRINT-STATUS.md después de CADA tarea
8. Haz commit después de cada tarea
9. Al terminar TODAS las tareas, reporta resumen de Fase 1

⚠️ Reemplaza X con el número de sprint (3, 4, etc.)
⚠️ Verifica que estés usando archivos en 05-worker/, NO en otros proyectos
```

---

## 🔄 FASE 2: Resolución de Stubs

### Propósito:
Reemplazar todos los stubs con implementación real, verificando que los recursos externos estén disponibles.

### 📋 Prompt para el Usuario:

```
Ejecuta FASE 2 del SPRINT-X en edugo-worker.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/
- Sprint: SPRINT-X
- Fase: 2 - Resolución de Stubs
- Tareas con stub: [ver en tracking/SPRINT-STATUS.md]
- Reglas: tracking/REGLAS.md

Pre-requisitos:
- Fase 1 debe estar completa (100%)
- Debe haber tareas marcadas con ✅ (stub)

Instrucciones:
1. Lee INDEX.md
2. Lee tracking/SPRINT-STATUS.md
3. Identifica tareas con marcador ✅ (stub)
4. Para cada tarea con stub:
   a. Lee la decisión: tracking/decisions/TASK-X.X-BLOCKED.md
   b. Verifica que MongoDB/RabbitMQ/Docker estén corriendo
   c. Reemplaza stub con código real
   d. Prueba integración: go test ./...
   e. Marca como ✅ (sin stub)
   f. Actualiza tracking/SPRINT-STATUS.md
   g. Haz commit
5. Al terminar, reporta resumen de Fase 2

⚠️ Reemplaza X con el número de sprint (3, 4, etc.)
⚠️ Si los recursos externos NO están disponibles, detente y reporta
```

---

## ✅ FASE 3: Validación y PR

### Propósito:
Validar todo el código, crear PR a `dev`, monitorear CI/CD, mergear.

### 📋 Prompt para el Usuario:

```
Ejecuta FASE 3 del SPRINT-X en edugo-worker.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/
- Sprint: SPRINT-X
- Fase: 3 - Validación y PR
- Reglas: tracking/REGLAS.md

Pre-requisitos:
- Fase 1 completa (100%)
- Fase 2 completa (100%)
- No hay tareas con stub pendientes

Instrucciones:
1. Validación local completa:
   a. Build: go build ./...
   b. Tests: go test ./... -race -coverprofile=coverage.out
   c. Lint: golangci-lint run ./...
   d. Coverage: verificar umbrales mínimos
2. Si TODO pasa:
   a. Push: git push origin feature/sprint-X-[fecha]
   b. Crear PR a dev usando template
   c. Monitorear CI/CD (máx 5 min, polling 30s)
3. Manejar comentarios de Copilot:
   a. Críticos (security/bugs): Resolver SIEMPRE
   b. Sugerencias: Evaluar caso por caso
   c. Documentar en tracking/reviews/
4. Si CI/CD está verde y sin comentarios críticos:
   a. Mergear PR (squash)
   b. Verificar CI/CD post-merge
5. Reportar resumen completo del Sprint

⚠️ Reemplaza X con el número de sprint (3, 4, etc.)
⚠️ NO mergear si CI/CD está en rojo o hay comentarios críticos pendientes
```

---

## 🔄 PROMPTS AUXILIARES

### Continuar desde donde quedó:

```
Continúa el trabajo de CI/CD en edugo-worker desde donde quedó.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/

Instrucciones:
1. Lee INDEX.md para orientarte
2. Lee tracking/SPRINT-STATUS.md
3. Identifica:
   - ¿Qué sprint está activo?
   - ¿En qué fase estamos (1, 2, o 3)?
   - ¿Cuál es la próxima tarea pendiente?
   - ¿Hay bloqueadores?
4. Continúa desde esa tarea usando las reglas de la fase actual
5. Si hay dudas, pregunta antes de continuar

⚠️ Verifica que estés usando archivos en 05-worker/
```

### Ver estado actual:

```
Muéstrame el estado actual del proyecto edugo-worker CI/CD.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/

Instrucciones:
1. Lee INDEX.md
2. Lee tracking/SPRINT-STATUS.md
3. Reporta:
   - Sprint activo: [número]
   - Fase actual: [1, 2, o 3]
   - Progreso: [X%]
   - Tareas completadas: [X/Y]
   - Tareas con stub: [X]
   - Próxima tarea: [número y nombre]
   - Bloqueadores: [sí/no, cuáles]
   - Última actividad: [fecha/hora]
   - Branch activo: [nombre]

⚠️ Usa solo archivos dentro de 05-worker/
```

### Iniciar nuevo sprint:

```
Iniciar SPRINT-X en edugo-worker.

Contexto:
- Ubicación: 00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/
- Sprint anterior: [estado]
- Nuevo sprint: SPRINT-X
- Archivo: sprints/SPRINT-X-TASKS.md

Pre-requisitos:
- Sprint anterior completo o pausado
- Rama dev actualizada

Instrucciones:
1. Lee INDEX.md
2. Lee sprints/SPRINT-X-TASKS.md
3. Verifica branch:
   a. git checkout dev
   b. git pull origin dev
4. Crea feature branch: feature/sprint-X-$(date +%Y-%m-%d)
5. Inicializa tracking/SPRINT-STATUS.md para SPRINT-X
6. Documenta inicio en tracking/logs/
7. Pregunta: ¿Inicio Fase 1 ahora? (esperar confirmación)

⚠️ Reemplaza X con el número de sprint
⚠️ NO inicies tareas sin confirmar con el usuario
```

---

## 🤖 INSTRUCCIONES PARA CLAUDE (LEER SIEMPRE)

### ⚠️ Regla #1: Orientación Primero
**ANTES de ejecutar cualquier fase:**
1. Verifica ubicación: estás en 05-worker/
2. Lee: `INDEX.md` (3-5 min)
3. Lee: `tracking/REGLAS.md` (5-10 min)
4. Lee: `tracking/SPRINT-STATUS.md` (2 min)

### ⚠️ Regla #2: Contexto de Ubicación
**TODAS las rutas son relativas a:** `05-worker/`

**Archivos CORRECTOS (usar):**
- ✅ `sprints/SPRINT-3-TASKS.md`
- ✅ `tracking/SPRINT-STATUS.md`
- ✅ `tracking/REGLAS.md`

**Archivos INCORRECTOS (NO usar):**
- ❌ `01-shared/sprints/*` (otro proyecto)
- ❌ `03-api-mobile/sprints/*` (otro proyecto)
- ❌ `../02-PROPUESTAS-MEJORA.md` (documentación general)

### ⚠️ Regla #3: Validar Antes de Ejecutar
```bash
# Antes de abrir un archivo de sprint:
ls -la /Users/jhoanmedina/source/EduGo/Analisys/00-Projects-Isolated/cicd-analysis/implementation-plans/05-worker/sprints/SPRINT-X-TASKS.md

# Debe existir. Si no existe, reportar error.
```

### ⚠️ Regla #4: Actualizar en Tiempo Real
- Después de CADA tarea → actualizar `tracking/SPRINT-STATUS.md`
- NO agrupar múltiples tareas antes de actualizar
- Hacer commit después de cada tarea

### ⚠️ Regla #5: Reportar Bloqueos
Si encuentras:
- Recursos externos no disponibles (MongoDB, RabbitMQ)
- Errores de compilación/tests que no puedes resolver
- Archivos faltantes
- Conflictos git

**Detente y reporta al usuario. NO continues sin resolver.**

---

## 📝 Template de PR

```markdown
# Sprint X: [Nombre del Sprint]

## 📊 Resumen
- **Fase 1:** X/X tareas completadas (X con stub)
- **Fase 2:** X/X stubs resueltos
- **Fase 3:** Validación completa ✅

## ✅ Tareas Completadas
- [x] Tarea X.1: [nombre]
- [x] Tarea X.2: [nombre]
- [x] Tarea X.3: [nombre]

## 🔴 Tareas con Stub (Fase 2)
- [x] Tarea X.X: [nombre] - Stub usado: [MongoDB/RabbitMQ/etc.]
  - Decisión: [link a tracking/decisions/]

## 🧪 Tests
- Tests agregados: X
- Tests modificados: X
- Cobertura: X% (umbral: X%)
- Tests pasando: ✅

## 📋 Validación
- [x] Build exitoso (`go build ./...`)
- [x] Tests pasando (`go test ./...`)
- [x] Lint sin errores (`golangci-lint run ./...`)
- [x] Cobertura >= umbral mínimo
- [x] Documentación actualizada
- [x] CHANGELOG.md actualizado (si aplica)

## 📎 Enlaces
- Plan completo: [sprints/SPRINT-X-TASKS.md](sprints/SPRINT-X-TASKS.md)
- Reglas: [tracking/REGLAS.md](tracking/REGLAS.md)
- Estado final: [tracking/SPRINT-STATUS.md](tracking/SPRINT-STATUS.md)
- Logs: [tracking/logs/](tracking/logs/)

## 🎯 Siguiente Sprint
- [ ] Sprint X+1: [nombre] (si aplica)

---

**Generado por:** Claude Code  
**Fecha:** [fecha]  
**Tiempo total:** [X horas]
```

---

## ✅ Checklist de Uso

### Para el Usuario (Antes de usar un prompt):
- [ ] Identificar qué fase quiero ejecutar (1, 2, o 3)
- [ ] Identificar número de sprint (X)
- [ ] Copiar el prompt correspondiente
- [ ] Reemplazar X con el número
- [ ] Verificar pre-requisitos (si es Fase 2 o 3)
- [ ] Pegar en Claude

### Para Claude (Antes de ejecutar):
- [ ] Leer INDEX.md completo
- [ ] Leer REGLAS.md (sección de la fase)
- [ ] Leer SPRINT-STATUS.md
- [ ] Verificar ubicación (05-worker/)
- [ ] Verificar que archivo de sprint existe
- [ ] Confirmar entendimiento con el usuario

---

## 🆘 Ayuda Rápida

| Situación | Acción |
|-----------|--------|
| No sé qué fase ejecutar | Lee `tracking/SPRINT-STATUS.md` |
| No sé qué sprint está activo | Lee `INDEX.md` |
| Quiero ver todas las tareas | Abre `sprints/SPRINT-X-TASKS.md` |
| Necesito las reglas | Lee `tracking/REGLAS.md` |
| ¿Dónde está el código? | `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker` |
| Claude se confunde con archivos | Verifica que use rutas `05-worker/` |

---

**Generado por:** Claude Code  
**Fecha:** 20 de Noviembre, 2025  
**Versión:** 1.0
