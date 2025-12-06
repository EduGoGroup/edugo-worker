# 🎯 Sistema de Tracking para Ejecución Desatendida

**Versión**: 1.0.0  
**Proyecto**: baileys-go - Resolución de Deuda Técnica  
**Modo**: Ejecución Desatendida por IA

---

## 📋 Introducción

Este documento define el sistema de tracking que permite a una IA ejecutar los sprints de manera completamente autónoma, con capacidad de:
- ✅ Continuar desde interrupciones
- ✅ Manejar errores de manera inteligente
- ✅ Reportar progreso automáticamente
- ✅ Validar completitud antes de proceder

## 🔄 Flujo de Ejecución

### 1. Inicio de Sesión

```javascript
// Pseudocódigo del flujo de inicio
function iniciarSesion() {
    // 1. Leer estado actual
    const progress = leerArchivo("PROGRESS.json");
    
    // 2. Identificar punto de continuación
    const sprintActual = progress.summary.current_sprint;
    const tareaActual = progress.summary.current_task;
    
    // 3. Cargar contexto
    const sprint = cargarSprint(sprintActual);
    const tarea = sprint.tasks[tareaActual];
    
    // 4. Verificar dependencias
    if (tarea.blocked) {
        reportarBloqueado(tarea.blocker_reason);
        return;
    }
    
    // 5. Continuar ejecución
    ejecutarTarea(sprintActual, tareaActual);
}
```

**Comandos Reales**:
```bash
# Leer estado actual
cat AnalisisEstandarizado/PROGRESS.json | jq '.summary'

# Ver sprint actual
cat AnalisisEstandarizado/PROGRESS.json | jq '.summary.current_sprint'

# Ver tarea actual
cat AnalisisEstandarizado/PROGRESS.json | jq '.summary.current_task'

# Navegar a tarea
cd AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E
cat TASKS.md | grep -A 50 "TASK-001"
```

### 2. Ejecución de Tareas

**Reglas de Ejecución**:

1. **Orden Secuencial Estricto**: Ejecutar tareas en orden TASK-001, TASK-002, TASK-003...
2. **Validar Dependencias**: No ejecutar tarea si dependencias no están completadas
3. **Actualizar Estado**: Modificar PROGRESS.json después de cada tarea
4. **Commits Atómicos**: Un commit por tarea completada exitosamente
5. **No Saltar**: No saltar tareas a menos que estén explícitamente en `skipped_tasks`

**Ejemplo de Ejecución de Tarea**:

```bash
# 1. Marcar tarea como in_progress
jq '.sprints["Sprint-01-Tests-E2E"].tasks["TASK-001"].status = "in_progress"' PROGRESS.json > temp.json && mv temp.json PROGRESS.json

# 2. Actualizar timestamp de inicio
jq '.sprints["Sprint-01-Tests-E2E"].tasks["TASK-001"].started_at = now' PROGRESS.json > temp.json && mv temp.json PROGRESS.json

# 3. Ejecutar pasos de la tarea
# (Ver TASKS.md para pasos específicos)

# 4. Validar tarea
bash scripts/validate-task-001.sh

# 5. Si exitoso, marcar como completed
jq '.sprints["Sprint-01-Tests-E2E"].tasks["TASK-001"].status = "completed"' PROGRESS.json > temp.json && mv temp.json PROGRESS.json

# 6. Actualizar timestamp de completado
jq '.sprints["Sprint-01-Tests-E2E"].tasks["TASK-001"].completed_at = now' PROGRESS.json > temp.json && mv temp.json PROGRESS.json

# 7. Agregar a execution_history
# ... (ver sección de History Management)

# 8. Crear commit
git add .
git commit -m "feat(sprint-01): complete TASK-001 - Setup infraestructura E2E"

# 9. Proceder a siguiente tarea
# Actualizar current_task en PROGRESS.json
```

### 3. Manejo de Errores

**Estrategia de Reintentos**: 3 intentos antes de marcar como failed

```bash
#!/bin/bash
# Ejemplo de script con reintentos

TASK_ID="TASK-001"
MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    echo "Intento $((RETRY_COUNT + 1))/$MAX_RETRIES..."
    
    # Ejecutar tarea
    if ejecutar_tarea; then
        echo "✅ Tarea completada exitosamente"
        actualizar_progress_completed "$TASK_ID"
        exit 0
    else
        echo "❌ Tarea falló"
        RETRY_COUNT=$((RETRY_COUNT + 1))
        
        if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
            echo "⏳ Esperando 10 segundos antes de reintentar..."
            sleep 10
        fi
    fi
done

# Si llegamos aquí, todos los intentos fallaron
echo "💥 Tarea falló después de $MAX_RETRIES intentos"
actualizar_progress_failed "$TASK_ID" "Failed after $MAX_RETRIES retries"
reportar_a_usuario
exit 1
```

**Acciones en caso de Fallo**:

1. **Documentar Error**: Agregar a `failed_tasks` con timestamp y razón
2. **Evaluar Impacto**: ¿Bloquea otras tareas?
3. **Continuar si Posible**: Si tarea no es bloqueante, continuar con tareas independientes
4. **Reportar**: Generar reporte de error para revisión manual

**Ejemplo de PROGRESS.json con tarea fallida**:

```json
{
  "summary": {
    "failed_tasks": 1
  },
  "sprints": {
    "Sprint-01-Tests-E2E": {
      "tasks": {
        "TASK-002": {
          "status": "failed",
          "failed_at": "2025-11-15T14:30:00Z",
          "failure_reason": "Cannot bind to port 9000: address already in use",
          "retry_count": 3,
          "last_error": "listen tcp :9000: bind: address already in use"
        }
      }
    }
  },
  "execution_history": [
    {
      "timestamp": "2025-11-15T14:30:00Z",
      "event": "task_failed",
      "task": "TASK-002",
      "reason": "Port conflict",
      "action_taken": "Documented and reported"
    }
  ]
}
```

### 4. Validación

**Antes de Marcar Sprint como Completado**:

```bash
# Ejecutar VALIDATION.md del sprint
cd AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E
bash VALIDATION.md

# Si todos los checks pasan:
# ✅ Marcar sprint como completed
# ✅ Crear PR
# ✅ Proceder a siguiente sprint

# Si algún check falla:
# ❌ Identificar tarea que causó el fallo
# ❌ Re-ejecutar tarea o marcar como failed
# ❌ No proceder al siguiente sprint
```

**Validaciones Críticas**:
- [ ] Todos los tests del sprint pasan
- [ ] Cobertura cumple threshold (si aplica)
- [ ] Linter sin errores críticos
- [ ] Build exitoso
- [ ] Documentación actualizada
- [ ] Resources limpiados (no hay containers huérfanos)

### 5. Commits y PRs

**Formato de Commits**:

```
<type>(sprint-XX): <description>

<body (opcional)>

<footer (opcional)>
```

**Types**:
- `feat`: Nueva funcionalidad
- `fix`: Bug fix
- `refactor`: Refactorización sin cambio funcional
- `test`: Agregar o modificar tests
- `docs`: Documentación
- `ci`: Cambios en CI/CD

**Ejemplos**:

```bash
# Tarea completada
git commit -m "feat(sprint-01): complete TASK-001 - Setup E2E infrastructure"

# Bug fix durante sprint
git commit -m "fix(sprint-01): resolve port conflict in mock WhatsApp server"

# Actualización de docs
git commit -m "docs(sprint-01): add troubleshooting guide for E2E tests"
```

**Formato de PRs**:

```markdown
# [Sprint-01] Implementar Tests End-to-End

## 📋 Descripción

Implementación de 3 tests E2E críticos para validar flujos completos del sistema.

## ✅ Tareas Completadas

- [x] TASK-001: Setup infraestructura E2E
- [x] TASK-002: Mock WhatsApp server
- [x] TASK-003: Test de Pairing
- [x] TASK-004: Test de Envío de Mensaje
- [x] TASK-005: Test de Reconexión
- [x] TASK-006: Integración CI/CD
- [x] TASK-007: Documentación

## 🧪 Validación

- [x] Tests E2E pasan: `go test -tags=e2e ./tests/e2e/...`
- [x] Estabilidad: 10/10 ejecuciones exitosas
- [x] Performance: Suite completa en 18s (target: <20s)
- [x] CI/CD integrado y funcionando

## 📊 Métricas

| Métrica | Objetivo | Real | Status |
|---------|----------|------|--------|
| Tests E2E | 3 | 3 | ✅ |
| Tiempo Suite | < 20s | 18s | ✅ |
| Estabilidad | 100% | 100% | ✅ |

## 🔗 Referencias

- [Sprint README](./AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E/README.md)
- [TASKS.md](./AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E/TASKS.md)
- [VALIDATION.md](./AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E/VALIDATION.md)

## 👀 Checklist para Reviewer

- [ ] Tests E2E ejecutan y pasan
- [ ] Código sigue convenciones del proyecto
- [ ] Documentación clara y completa
- [ ] No hay containers huérfanos después de tests
```

**Comandos para PR**:

```bash
# 1. Crear branch
git checkout -b feature/sprint-01-e2e-tests

# 2. Todos los commits del sprint ya están hechos

# 3. Push
git push origin feature/sprint-01-e2e-tests

# 4. Crear PR (usando GitHub CLI)
gh pr create \
  --title "[Sprint-01] Implementar Tests End-to-End" \
  --body-file AnalisisEstandarizado/03-Sprints/Sprint-01-Tests-E2E/PR_TEMPLATE.md \
  --base dev \
  --head feature/sprint-01-e2e-tests
```

## 📊 Gestión de Estado (PROGRESS.json)

### Estructura de Estado

```json
{
  "summary": {
    "current_sprint": "string",
    "current_task": "string",
    "overall_progress_percent": number
  },
  "sprints": {
    "Sprint-XX": {
      "status": "pending|in_progress|completed|failed",
      "tasks": {
        "TASK-XXX": {
          "status": "pending|in_progress|completed|failed|skipped",
          "started_at": "ISO-8601 timestamp",
          "completed_at": "ISO-8601 timestamp",
          "blocked": boolean,
          "dependencies": ["TASK-XXX", ...]
        }
      }
    }
  },
  "execution_history": [
    {
      "timestamp": "ISO-8601",
      "event": "string",
      "details": "string"
    }
  ]
}
```

### Actualización de Estado

**Helper Script**: `scripts/update-progress.sh`

```bash
#!/bin/bash

FUNCTION=$1
SPRINT=$2
TASK=$3
VALUE=$4

case $FUNCTION in
  start_task)
    jq --arg sprint "$SPRINT" --arg task "$TASK" \
      '.sprints[$sprint].tasks[$task].status = "in_progress" |
       .sprints[$sprint].tasks[$task].started_at = now |
       .summary.current_task = $task' \
      PROGRESS.json > temp.json && mv temp.json PROGRESS.json
    ;;
    
  complete_task)
    jq --arg sprint "$SPRINT" --arg task "$TASK" \
      '.sprints[$sprint].tasks[$task].status = "completed" |
       .sprints[$sprint].tasks[$task].completed_at = now |
       .summary.completed_tasks += 1' \
      PROGRESS.json > temp.json && mv temp.json PROGRESS.json
    ;;
    
  fail_task)
    jq --arg sprint "$SPRINT" --arg task "$TASK" --arg reason "$VALUE" \
      '.sprints[$sprint].tasks[$task].status = "failed" |
       .sprints[$sprint].tasks[$task].failed_at = now |
       .sprints[$sprint].tasks[$task].failure_reason = $reason |
       .summary.failed_tasks += 1' \
      PROGRESS.json > temp.json && mv temp.json PROGRESS.json
    ;;
esac
```

**Uso**:
```bash
# Iniciar tarea
bash scripts/update-progress.sh start_task Sprint-01-Tests-E2E TASK-001

# Completar tarea
bash scripts/update-progress.sh complete_task Sprint-01-Tests-E2E TASK-001

# Fallar tarea
bash scripts/update-progress.sh fail_task Sprint-01-Tests-E2E TASK-002 "Port already in use"
```

## 🔄 Recuperación de Interrupciones

### Escenarios de Interrupción

#### 1. Interrupción Limpia (Ctrl+C)

```bash
# Estado: Tarea TASK-003 en progreso
# Acción al reiniciar:
# 1. Leer PROGRESS.json
# 2. Ver que TASK-003 está "in_progress"
# 3. Opciones:
#    a) Re-ejecutar TASK-003 desde el inicio
#    b) Continuar desde último paso (si tarea es idempotente)
```

#### 2. Crash de Sistema

```bash
# Estado: Sistema crasheó durante TASK-005
# Acción al reiniciar:
# 1. Leer PROGRESS.json
# 2. Ver última tarea "completed"
# 3. Ver tarea "in_progress" (puede estar corrupta)
# 4. Validar estado del código:
#    - ¿Hay archivos parcialmente creados?
#    - ¿Tests pasan?
# 5. Decidir si re-ejecutar o continuar
```

#### 3. Error de Red/Dependencias

```bash
# Estado: Testcontainers no pudo descargar imagen
# Acción:
# 1. Marcar tarea como "blocked"
# 2. Documentar razón en PROGRESS.json
# 3. Reportar a usuario
# 4. Intentar continuar con tareas independientes (si las hay)
# 5. Al resolver el blocker, re-ejecutar tarea bloqueada
```

### Script de Recuperación

**Archivo**: `scripts/recover.sh`

```bash
#!/bin/bash

echo "🔄 Iniciando recuperación..."

# 1. Leer estado
CURRENT_SPRINT=$(jq -r '.summary.current_sprint' PROGRESS.json)
CURRENT_TASK=$(jq -r '.summary.current_task' PROGRESS.json)

echo "📍 Último estado conocido:"
echo "  Sprint: $CURRENT_SPRINT"
echo "  Tarea: $CURRENT_TASK"

# 2. Verificar estado de la tarea actual
TASK_STATUS=$(jq -r ".sprints[\"$CURRENT_SPRINT\"].tasks[\"$CURRENT_TASK\"].status" PROGRESS.json)

echo "  Status: $TASK_STATUS"

# 3. Decidir acción
if [ "$TASK_STATUS" == "in_progress" ]; then
    echo "⚠️  Tarea estaba en progreso. Opciones:"
    echo "  1) Re-ejecutar tarea desde el inicio"
    echo "  2) Continuar desde último punto (solo si idempotente)"
    echo "  3) Marcar como failed y continuar"
    
    # Por defecto: Re-ejecutar
    echo "📝 Re-ejecutando $CURRENT_TASK..."
    # Resetear status
    jq ".sprints[\"$CURRENT_SPRINT\"].tasks[\"$CURRENT_TASK\"].status = \"pending\"" PROGRESS.json > temp.json && mv temp.json PROGRESS.json
    
elif [ "$TASK_STATUS" == "failed" ]; then
    echo "❌ Tarea previamente falló"
    FAILURE_REASON=$(jq -r ".sprints[\"$CURRENT_SPRINT\"].tasks[\"$CURRENT_TASK\"].failure_reason" PROGRESS.json)
    echo "  Razón: $FAILURE_REASON"
    echo "  Resolver el issue antes de continuar"
    exit 1
    
elif [ "$TASK_STATUS" == "completed" ]; then
    echo "✅ Tarea completada. Proceder a siguiente."
    # Encontrar siguiente tarea
    # ... lógica para siguiente tarea ...
fi

echo "✅ Recuperación completada"
```

## 📈 Reportes de Progreso

### Reporte Diario Automático

**Archivo**: `scripts/daily-report.sh`

```bash
#!/bin/bash

echo "📊 Reporte de Progreso - $(date '+%Y-%m-%d')"
echo "==========================================="
echo ""

# Estadísticas generales
TOTAL_SPRINTS=$(jq '.summary.total_sprints' PROGRESS.json)
COMPLETED_SPRINTS=$(jq '.summary.completed_sprints' PROGRESS.json)
TOTAL_TASKS=$(jq '.summary.total_tasks' PROGRESS.json)
COMPLETED_TASKS=$(jq '.summary.completed_tasks' PROGRESS.json)
FAILED_TASKS=$(jq '.summary.failed_tasks' PROGRESS.json)

echo "📈 Estadísticas Generales:"
echo "  Sprints: $COMPLETED_SPRINTS/$TOTAL_SPRINTS completados"
echo "  Tareas: $COMPLETED_TASKS/$TOTAL_TASKS completadas"
echo "  Fallidas: $FAILED_TASKS"
echo "  Progreso: $(jq '.summary.overall_progress_percent' PROGRESS.json)%"
echo ""

# Sprint actual
CURRENT_SPRINT=$(jq -r '.summary.current_sprint' PROGRESS.json)
echo "🎯 Sprint Actual: $CURRENT_SPRINT"

# Tareas del sprint actual
echo "  Tareas:"
jq -r ".sprints[\"$CURRENT_SPRINT\"].tasks | to_entries[] | \"    [\(.value.status)] \(.key): \(.value.name)\"" PROGRESS.json

echo ""
echo "⏱️  Métricas de Tiempo:"
TOTAL_ESTIMATED=$(jq '.metrics.total_estimated_hours' PROGRESS.json)
TOTAL_ACTUAL=$(jq '.metrics.total_actual_hours' PROGRESS.json)
echo "  Estimado total: ${TOTAL_ESTIMATED}h"
echo "  Actual total: ${TOTAL_ACTUAL}h"

echo ""
echo "📝 Últimos 5 Eventos:"
jq -r '.execution_history[-5:] | .[] | "  [\(.timestamp)] \(.event): \(.details // .task)"' PROGRESS.json

echo ""
echo "==========================================="
```

## ✅ Checklist de Ejecución Desatendida

### Antes de Iniciar

- [ ] PROGRESS.json existe y es válido JSON
- [ ] Sprints están definidos en orden correcto
- [ ] Dependencias entre tareas están documentadas
- [ ] Scripts de validación existen para cada sprint
- [ ] Git está configurado correctamente

### Durante Ejecución

- [ ] Leer PROGRESS.json al inicio de cada sesión
- [ ] Actualizar estado después de cada tarea
- [ ] Validar cada tarea antes de marcar como completed
- [ ] Crear commits atómicos (uno por tarea)
- [ ] Manejar errores con reintentos (máx 3)

### Después de Cada Sprint

- [ ] Ejecutar VALIDATION.md completo
- [ ] Crear PR con formato estándar
- [ ] Actualizar PROGRESS.json con sprint completed
- [ ] Generar reporte de sprint
- [ ] Proceder a siguiente sprint solo si validación pasó

---

**Para Executor de IA**: Este es tu manual de operación. Sigue estas reglas estrictamente para asegurar ejecución correcta y recuperable.

**Para Usuarios**: Este documento explica cómo la IA ejecutará los sprints. Puedes monitorear el progreso leyendo PROGRESS.json en cualquier momento.
