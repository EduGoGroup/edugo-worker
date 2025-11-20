# 🎯 Sistema de Seguimiento de Sprints

**Proyecto:** edugo-shared  
**Sistema:** 3 Fases con tracking automático  
**Ubicación:** `.sprint-tracking/`

---

## 🚀 Inicio Rápido

### Pregunta Rápida: "¿Dónde estoy?"

```bash
# Ver estado actual del sprint
cat .sprint-tracking/SPRINT-STATUS.md | head -30

# Ver siguiente tarea
grep "🔄\|⏳" .sprint-tracking/SPRINT-STATUS.md | head -1
```

### Pregunta: "¿Qué sprint sigue?"

```bash
# Ver sprints disponibles
ls docs/cicd/SPRINT-*-TASKS.md
```

### Pregunta: "Continúa con la siguiente tarea"

```markdown
Claude, por favor:
1. Lee .sprint-tracking/SPRINT-STATUS.md
2. Identifica la siguiente tarea pendiente
3. Continúa con esa tarea siguiendo .sprint-tracking/REGLAS.md
```

---

## 📚 Documentación

### Documentos Principales

| Documento | Propósito | Cuándo leer |
|-----------|-----------|-------------|
| **[REGLAS.md](.sprint-tracking/REGLAS.md)** | Reglas completas de ejecución | Antes de iniciar cualquier sprint |
| **[SPRINT-STATUS.md](.sprint-tracking/SPRINT-STATUS.md)** | Estado actual de tareas | Cada vez que necesites saber dónde estás |
| **Sprints en docs/cicd/** | Tareas detalladas por sprint | Al iniciar un sprint específico |

### Carpetas de Seguimiento

```
.sprint-tracking/
├── REGLAS.md                    ← 📖 LEE PRIMERO
├── SPRINT-STATUS.md             ← 📊 ESTADO ACTUAL
│
├── logs/                        ← Logs de ejecución
├── errors/                      ← Errores documentados
├── decisions/                   ← Decisiones de stubs/bloqueos
└── reviews/                     ← Revisiones de código
```

---

## 🎯 Las 3 Fases

### FASE 1: Implementación (con stubs si es necesario)
**Objetivo:** Completar todas las tareas del sprint

- ✅ Implementar cada tarea
- ✅ Si hay bloqueo (Docker, BD, etc.) → usar stub
- ✅ Marcar tarea inmediatamente al completar
- ✅ Compilar y testear después de cada tarea
- ✅ Revisión de código al final

**Salida:** Todas las tareas completadas (algunas con stubs)

---

### FASE 2: Resolución de Stubs
**Objetivo:** Reemplazar stubs con implementación real

- ✅ Identificar todos los stubs de Fase 1
- ✅ Verificar disponibilidad de recursos externos
- ✅ Reemplazar stubs con código real
- ✅ Tests de integración
- ✅ Documentar errores si los hay
- ✅ Revisión de código al final

**Salida:** Todos los stubs resueltos o marcados como permanentes

---

### FASE 3: Validación y CI/CD
**Objetivo:** Validar, crear PR, pasar CI/CD, mergear

- ✅ Validación local completa (build, tests, lint, coverage)
- ✅ Push y crear PR a `dev`
- ✅ Monitorear CI/CD (máx 5 min)
- ✅ Resolver comentarios de Copilot
- ✅ Merge a `dev`
- ✅ Monitorear CI/CD post-merge (máx 5 min)
- ✅ (Opcional) PR a `main` y release

**Salida:** Código en `dev` (o `main`), CI/CD pasando

---

## 📋 Reglas Esenciales

### 1. Tarea Completada = Tarea Marcada
Actualiza `SPRINT-STATUS.md` inmediatamente después de completar cada tarea.

### 2. Código que Compila
Después de CADA cambio de código:
```bash
go build ./...
go test ./...
```

### 3. Branch Strategy
```bash
# Siempre desde dev
git checkout dev
git pull origin dev

# Crear feature branch
git checkout -b sprint-X-$(date +%Y-%m-%d)
```

### 4. Manejo de Bloqueos
Si una tarea requiere Docker/BD/RabbitMQ y no está disponible:
1. Implementar con **stub/mock**
2. Documentar en `.sprint-tracking/decisions/TASK-XX-BLOCKED.md`
3. Marcar como `✅ (stub)` en SPRINT-STATUS.md
4. Resolver en Fase 2

### 5. Documentación de Errores
Si un error toma >10 minutos resolver:
1. Crear `.sprint-tracking/errors/ERROR-YYYY-MM-DD-HH-MM.md`
2. Documentar: síntoma, causa, intentos, solución

---

## 🚨 Cuándo DETENER

Claude debe **DETENER** e informarte si:

1. ❌ Compilación falla después de 3 intentos
2. ❌ Tests fallan después de 3 intentos
3. ❌ CI/CD toma >5 minutos
4. ❌ CI/CD falla en PR o post-merge
5. ❌ Copilot sugiere mejora >3 puntos Fibonacci
6. ❌ Error toma >30 minutos resolver
7. ❌ Recurso externo no disponible en Fase 2 sin alternativa

**Qué hace Claude al detener:**
- Documenta estado actual
- Lista opciones para continuar
- Espera tu decisión

---

## 💬 Comandos de Chat Comunes

### Iniciar un Sprint
```
Claude, vamos a iniciar el Sprint 1:
1. Lee docs/cicd/SPRINT-1-TASKS.md
2. Prepara la rama desde dev
3. Inicializa SPRINT-STATUS.md
4. Comienza con la primera tarea siguiendo REGLAS.md
```

### Continuar donde quedamos
```
Claude:
1. Lee .sprint-tracking/SPRINT-STATUS.md
2. Identifica dónde estamos
3. Continúa con la siguiente tarea pendiente
```

### Cambiar a Fase 2
```
Claude:
1. Cierra Fase 1 (crear FASE-1-COMPLETE.md)
2. Lista todos los stubs
3. Comienza Fase 2 resolviendo stubs
```

### Ir a Fase 3
```
Claude:
1. Cierra Fase 2 (crear FASE-2-COMPLETE.md)
2. Ejecuta validación completa
3. Crea PR siguiendo REGLAS.md
```

---

## 📊 Ver Progreso

### Progreso General
```bash
cat .sprint-tracking/SPRINT-STATUS.md | grep -A 10 "Progreso Global"
```

### Tareas Pendientes
```bash
grep "⏳" .sprint-tracking/SPRINT-STATUS.md
```

### Tareas en Progreso
```bash
grep "🔄" .sprint-tracking/SPRINT-STATUS.md
```

### Tareas Completadas
```bash
grep "✅" .sprint-tracking/SPRINT-STATUS.md | wc -l
```

### Stubs Activos
```bash
grep "✅ (stub)" .sprint-tracking/SPRINT-STATUS.md
```

### Errores Documentados
```bash
ls -la .sprint-tracking/errors/
```

---

## 🎓 Ejemplos de Uso

### Ejemplo 1: Sprint Completo sin Bloqueos
```bash
# Inicio
Claude, inicia Sprint 1

# Claude ejecuta Fase 1
# - Completa tareas 1-10
# - Marca cada una al terminar
# - Todo compila y testea

# Claude ejecuta Fase 2
# - No hay stubs (no hubo bloqueos)
# - Salta directamente a Fase 3

# Claude ejecuta Fase 3
# - Valida todo
# - Crea PR
# - Monitorea CI/CD
# - Merge a dev
# ✅ Sprint completado
```

### Ejemplo 2: Sprint con Stubs
```bash
# Inicio
Claude, inicia Sprint 1

# Claude ejecuta Fase 1
# - Tarea 5 necesita RabbitMQ → stub
# - Tarea 8 necesita PostgreSQL → stub
# - Otras tareas OK
# - Fase 1 completada con 2 stubs

# Claude ejecuta Fase 2
# - Verifica RabbitMQ disponible → reemplaza stub
# - Verifica PostgreSQL disponible → reemplaza stub
# - Tests de integración pasan
# - Fase 2 completada

# Claude ejecuta Fase 3
# - Valida, PR, CI/CD, merge
# ✅ Sprint completado
```

### Ejemplo 3: Sprint con Error
```bash
# Inicio
Claude, inicia Sprint 1

# Claude ejecuta Fase 1
# - Tareas 1-3 OK
# - Tarea 4 → error de compilación
# - Intento 1: falló
# - Intento 2: falló
# - Intento 3: falló
# - Documenta en errors/ERROR-2025-11-20-14-30.md
# ❌ DETIENE e informa al usuario

Usuario: "Intentemos con enfoque X"
# Claude resuelve con enfoque X
# ✅ Continúa con Tarea 5...
```

---

## 📁 Archivos Generados Durante un Sprint

```
.sprint-tracking/
├── SPRINT-1-COMPLETE.md           ← Al terminar sprint
├── FASE-1-COMPLETE.md             ← Al cerrar Fase 1
├── FASE-2-COMPLETE.md             ← Al cerrar Fase 2
├── FASE-3-VALIDATION.md           ← Durante Fase 3
├── PR-DESCRIPTION.md              ← Para el PR
├── RELEASE-NOTES.md               ← Si hay release
│
├── logs/
│   └── SPRINT-1-LOG.md            ← Log detallado
│
├── errors/
│   ├── ERROR-2025-11-20-10-15.md ← Si hay errores
│   └── ERROR-2025-11-20-14-30.md
│
├── decisions/
│   ├── TASK-05-BLOCKED.md         ← Decisiones de stubs
│   └── TASK-08-BLOCKED.md
│
└── reviews/
    ├── FASE-1-REVIEW.md           ← Revisión código Fase 1
    ├── FASE-2-REVIEW.md           ← Revisión código Fase 2
    ├── COPILOT-COMMENTS.md        ← Comentarios Copilot
    └── DISCARDED-COMMENTS.md      ← Comentarios descartados
```

---

## 🔗 Links Útiles

- **Reglas completas:** [.sprint-tracking/REGLAS.md](.sprint-tracking/REGLAS.md)
- **Estado actual:** [.sprint-tracking/SPRINT-STATUS.md](.sprint-tracking/SPRINT-STATUS.md)
- **Sprints disponibles:** [docs/cicd/](docs/cicd/)
- **Documentación CI/CD:** [docs/cicd/INDEX.md](docs/cicd/INDEX.md)

---

**Última actualización:** 20 de Noviembre, 2025  
**Sistema creado por:** Claude Code  
**Versión:** 1.0
