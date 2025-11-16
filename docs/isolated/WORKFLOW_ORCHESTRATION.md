# Workflow Orchestration - Ejecución Desatendida por Fases

**Proyecto**: baileys-go-debt-resolution  
**Versión**: 1.0.0  
**Fecha**: 2025-11-16

---

## 📋 Índice

1. [Objetivo](#objetivo)
2. [Arquitectura del Workflow](#arquitectura-del-workflow)
3. [Fase 1: Claude Code Web (Remoto)](#fase-1-claude-code-web-remoto)
4. [Fase 2: Claude Code Local](#fase-2-claude-code-local)
5. [Estado del Workflow](#estado-del-workflow)
6. [Prompts de Ejecución](#prompts-de-ejecución)
7. [Reglas de Trabajo Desatendido](#reglas-de-trabajo-desatendido)

---

## Objetivo

Ejecutar de manera **desatendida** la resolución de deuda técnica del proyecto baileys-go, sprint por sprint, utilizando dos fases:

- **Fase 1 (Web)**: Implementación máxima con stubs/mocks para recursos externos
- **Fase 2 (Local)**: Reemplazo de stubs por implementación real, validación CI/CD, y merge

### Resultado Final por Sprint
- ✅ Branch creado con implementación completa
- ✅ Pull Request creado y revisado
- ✅ CI/CD pasando al 100%
- ✅ Code review de GitHub Copilot atendido
- ✅ Merge a `dev` completado

---

## Arquitectura del Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                    INICIO DEL SPRINT                            │
│  Usuario lee: PROGRESS.json → identifica siguiente sprint       │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│              FASE 1: CLAUDE CODE WEB (REMOTO)                   │
│                                                                 │
│  1. Clona repositorio                                           │
│  2. Crea branch: feature/sprint-XX-nombre                       │
│  3. Lee TASKS.md del sprint                                     │
│  4. Ejecuta tareas:                                             │
│     - Si necesita Docker/DB/Redis → Crea STUB + documenta       │
│     - Si es código puro → Implementa completo                   │
│  5. Genera: PHASE2_BRIDGE.md (lista de stubs a completar)      │
│  6. Commit y push del branch                                    │
│  7. Genera PROMPT para Fase 2                                   │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     │ Usuario copia PROMPT y cambia a Local
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│              FASE 2: CLAUDE CODE LOCAL                          │
│                                                                 │
│  1. Lee PHASE2_BRIDGE.md                                        │
│  2. Elimina stubs, implementa código real (con Docker/DB/Redis) │
│  3. Ejecuta tests localmente                                    │
│  4. Crea Pull Request a dev                                     │
│  5. LOOP de validación (max 5 min):                             │
│     - Monitorea CI/CD cada 1 min                                │
│     - Lee comentarios de GitHub Copilot                         │
│     - Corrige errores (max 3 intentos por error)                │
│     - Atiende comentarios procedentes de Copilot                │
│  6. Si todo OK → Merge a dev                                    │
│  7. Genera reporte final                                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Fase 1: Claude Code Web (Remoto)

### Objetivo de la Fase 1
Implementar el máximo posible del sprint **sin** acceso a Docker, PostgreSQL, Redis u otros servicios externos.

### Capacidades en Web
- ✅ Lectura/escritura de archivos
- ✅ Refactorización de código
- ✅ Creación de tests unitarios (sin DB)
- ✅ Configuración de archivos (YAML, JSON)
- ✅ Git operations (clone, commit, push, branch)
- ❌ Docker / docker-compose
- ❌ PostgreSQL / Redis
- ❌ Integration tests con DB

### Flujo de Trabajo Fase 1

#### 1. Inicialización
```bash
# Leer estado actual
cat AnalisisEstandarizado/PROGRESS.json | jq '.sprints[] | select(.status=="pending") | .id' | head -1

# Resultado: "Sprint-01-Refactorizacion" (ejemplo)
```

#### 2. Preparación del Branch
```bash
# Clonar repo (si no está clonado)
git clone <repo-url>

# Crear branch según convención
SPRINT_ID="Sprint-01-Refactorizacion"
BRANCH_NAME="feature/sprint-01-refactorizacion"
git checkout -b "$BRANCH_NAME"
```

#### 3. Lectura del Sprint
```bash
# Leer documentación del sprint
SPRINT_DIR="AnalisisEstandarizado/03-Sprints/$SPRINT_ID"

# Archivos a leer:
- $SPRINT_DIR/README.md
- $SPRINT_DIR/TASKS.md
- $SPRINT_DIR/DEPENDENCIES.md
- $SPRINT_DIR/VALIDATION.md
```

#### 4. Ejecución de Tareas

**Para cada TASK en TASKS.md**:

```
SI tarea requiere recursos externos (Docker/DB/Redis):
  ├─ Crear STUB/MOCK con comentario: // TODO PHASE2: Implementar con DB real
  ├─ Documentar en PHASE2_BRIDGE.md
  └─ Marcar tarea como "partial" en estado

SI tarea es código puro (refactoring, config, tests unitarios):
  ├─ Implementar completamente
  └─ Marcar tarea como "completed" en estado

SI tarea es bloqueante (no se puede hacer sin recurso externo):
  ├─ Crear interfaz/contrato del código
  ├─ Documentar en PHASE2_BRIDGE.md con detalle
  └─ Marcar tarea como "blocked" en estado
```

#### 5. Generación de PHASE2_BRIDGE.md

**Ubicación**: `AnalisisEstandarizado/03-Sprints/$SPRINT_ID/PHASE2_BRIDGE.md`

**Estructura**:
```markdown
# Fase 2 Bridge - Sprint-XX

**Estado Fase 1**: Completada
**Fecha**: 2025-11-16
**Branch**: feature/sprint-XX-nombre

---

## Resumen de Fase 1

### Completado al 100%
- [x] TASK-001: Refactorizar session_service.go
- [x] TASK-002: Crear .golangci.yml

### Completado Parcialmente (Stubs creados)
- [~] TASK-003: Integration tests con PostgreSQL
  - **Archivo**: `internal/queue/postgres_queue_integration_test.go`
  - **Líneas**: 45-78
  - **Stub**: Usa mock DB en lugar de PostgreSQL real
  - **Acción Fase 2**: Reemplazar mockDB con testcontainers + PostgreSQL real

### Bloqueadas (Requieren Fase 2)
- [ ] TASK-004: Validar con docker-compose
  - **Razón**: No hay Docker en Web
  - **Acción Fase 2**: Ejecutar docker-compose up, validar services, down

---

## Stubs a Reemplazar

### 1. PostgreSQL Integration Test
**Archivo**: `internal/queue/postgres_queue_integration_test.go`
**Líneas**: 45-78

```go
// STUB ACTUAL (Fase 1):
func TestPostgresQueue_Enqueue_Integration(t *testing.T) {
    // TODO PHASE2: Implementar con PostgreSQL real usando testcontainers
    mockDB := &MockDB{
        EnqueueFunc: func(ctx context.Context, msg *Message) error {
            return nil
        },
    }
    
    queue := NewPostgresQueue(mockDB)
    // ... test con mock
}

// IMPLEMENTACIÓN REQUERIDA (Fase 2):
func TestPostgresQueue_Enqueue_Integration(t *testing.T) {
    // Levantar PostgreSQL con testcontainers
    ctx := context.Background()
    pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: "postgres:15-alpine",
            Env: map[string]string{
                "POSTGRES_PASSWORD": "test123",
                "POSTGRES_DB": "baileys_test",
            },
            ExposedPorts: []string{"5432/tcp"},
            WaitingFor: wait.ForListeningPort("5432/tcp"),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer pgContainer.Terminate(ctx)
    
    // Obtener DSN
    host, _ := pgContainer.Host(ctx)
    port, _ := pgContainer.MappedPort(ctx, "5432")
    dsn := fmt.Sprintf("postgres://postgres:test123@%s:%s/baileys_test?sslmode=disable", host, port.Port())
    
    // Conectar a DB real
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()
    
    // Crear schema
    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS messages (...)`)
    require.NoError(t, err)
    
    // Test real
    queue := NewPostgresQueue(db)
    err = queue.Enqueue(ctx, &Message{...})
    assert.NoError(t, err)
}
```

### 2. Redis Integration Test
**Archivo**: `internal/cache/redis_integration_test.go`
**Líneas**: 23-56

[Similar estructura]

---

## Validaciones Pendientes para Fase 2

### Validaciones con Docker
- [ ] Ejecutar `docker-compose -f docker-compose.test.yml up -d`
- [ ] Validar PostgreSQL: `docker exec postgres-test pg_isready`
- [ ] Validar Redis: `docker exec redis-test redis-cli ping`
- [ ] Ejecutar integration tests: `go test -tags=integration ./...`
- [ ] Cleanup: `docker-compose -f docker-compose.test.yml down`

### Validaciones con Race Detector
- [ ] `go test -race ./... -timeout 60s`
- [ ] Verificar 0 race conditions

### Validaciones de CI/CD
- [ ] Crear PR
- [ ] Esperar CI/CD (max 5 min, monitoreo cada 1 min)
- [ ] Revisar comentarios GitHub Copilot
- [ ] Corregir errores (max 3 intentos por error)
- [ ] Merge si todo OK

---

## Archivos Modificados en Fase 1

```
internal/session/service.go              - Refactorizado (100%)
internal/session/service_auth.go         - Creado (100%)
internal/session/service_handlers.go     - Creado (100%)
internal/queue/postgres_queue.go         - Refactorizado (100%)
internal/queue/postgres_queue_test.go    - Tests unitarios (100%)
internal/queue/postgres_queue_integration_test.go  - STUB (0% real)
.golangci.yml                            - Creado (100%)
```

---

## Estado de Tareas

| Task | Estado | Completado Fase 1 | Pendiente Fase 2 |
|------|--------|-------------------|------------------|
| TASK-001 | completed | 100% | - |
| TASK-002 | completed | 100% | - |
| TASK-003 | partial | 40% (stubs) | 60% (DB real) |
| TASK-004 | blocked | 0% | 100% (Docker) |

---

## Checklist para Fase 2

- [ ] Checkout branch `feature/sprint-XX-nombre`
- [ ] Leer este documento completo
- [ ] Reemplazar stubs por implementación real (sección "Stubs a Reemplazar")
- [ ] Ejecutar validaciones con Docker (sección "Validaciones con Docker")
- [ ] Ejecutar tests localmente: `go test ./...`
- [ ] Ejecutar race detector: `go test -race ./...`
- [ ] Crear Pull Request a `dev`
- [ ] Monitorear CI/CD (max 5 min, cada 1 min)
- [ ] Atender comentarios GitHub Copilot
- [ ] Merge si todo OK
- [ ] Actualizar PROGRESS.json

---

**Generado por**: Claude Code Web  
**Fase**: 1 completada  
**Próximo paso**: Ejecutar Fase 2 en Claude Code Local
```

#### 6. Commit y Push
```bash
# Commit de trabajo de Fase 1
git add .
git commit -m "feat(sprint-XX): Fase 1 - Implementación con stubs

- Completado: [lista de tareas completadas]
- Stubs creados: [lista de stubs]
- Pendiente Fase 2: Ver PHASE2_BRIDGE.md"

# Push
git push origin "$BRANCH_NAME"
```

#### 7. Actualizar Estado
```bash
# Actualizar PROGRESS.json
{
  "sprints": [
    {
      "id": "Sprint-01-Refactorizacion",
      "status": "phase1_completed",
      "phase1_completed_at": "2025-11-16T10:30:00Z",
      "branch": "feature/sprint-01-refactorizacion",
      "phase2_bridge": "AnalisisEstandarizado/03-Sprints/Sprint-01-Refactorizacion/PHASE2_BRIDGE.md"
    }
  ]
}
```

#### 8. Generar Prompt para Fase 2

**Archivo**: `PHASE2_PROMPT.txt`

```
Eres Claude Code Local ejecutando la Fase 2 del Sprint-01-Refactorizacion del proyecto baileys-go.

CONTEXTO:
- La Fase 1 (Web) ya completó el trabajo que no requiere Docker/DB/Redis
- Existe un branch: feature/sprint-01-refactorizacion
- Existe documentación de puente: AnalisisEstandarizado/03-Sprints/Sprint-01-Refactorizacion/PHASE2_BRIDGE.md

TU MISIÓN:
1. Checkout del branch: feature/sprint-01-refactorizacion
2. Leer PHASE2_BRIDGE.md completamente
3. Reemplazar TODOS los stubs/mocks por implementación real usando Docker/PostgreSQL/Redis
4. Ejecutar TODAS las validaciones con servicios reales
5. Crear Pull Request a dev
6. Monitorear CI/CD (max 5 min, revisar cada 1 min)
7. Atender comentarios de GitHub Copilot
8. Corregir errores de CI/CD (max 3 intentos por error único)
9. Mergear a dev si todo OK
10. Actualizar PROGRESS.json marcando sprint como "completed"

REGLAS:
- NO inventar código, seguir exactamente PHASE2_BRIDGE.md
- DETENTE si monitoreo CI/CD excede 5 minutos sin terminar
- DETENTE si un error se repite 3 veces
- DETENTE si comentarios de Copilot no son procedentes (genera informe)
- INFORMA al usuario al detenerte con contexto completo

¿Estás listo para iniciar Fase 2?
```

---

## Fase 2: Claude Code Local

### Objetivo de la Fase 2
Completar el sprint con implementación real de recursos externos, validar con CI/CD, y mergear a `dev`.

### Capacidades en Local
- ✅ Todo lo de Fase 1
- ✅ Docker / docker-compose
- ✅ PostgreSQL / Redis
- ✅ Integration tests completos
- ✅ Acceso a servicios locales
- ✅ GitHub CLI para PR y monitoreo

### Flujo de Trabajo Fase 2

#### 1. Inicialización
```bash
# Usuario ejecuta en Claude Code Local con PHASE2_PROMPT.txt

# Checkout del branch
git fetch origin
git checkout feature/sprint-01-refactorizacion  # (ejemplo)
```

#### 2. Lectura de PHASE2_BRIDGE.md
```bash
# Leer documento de puente
SPRINT_ID=$(git branch --show-current | sed 's/feature\/sprint-/Sprint-/' | sed 's/-/ /g' | awk '{for(i=1;i<=2;i++) printf "%s-", $i; for(i=3;i<=NF;i++) printf "%s", $i (i<NF?"-":"")}')

BRIDGE_DOC="AnalisisEstandarizado/03-Sprints/$SPRINT_ID/PHASE2_BRIDGE.md"

# Leer completamente
cat "$BRIDGE_DOC"

# Identificar:
# - Stubs a reemplazar (con código exacto)
# - Validaciones pendientes
# - Checklist de Fase 2
```

#### 3. Reemplazo de Stubs

**Para cada stub en PHASE2_BRIDGE.md**:

```bash
# Ejemplo: Reemplazar stub de PostgreSQL integration test

# Eliminar código stub
# Implementar código real según PHASE2_BRIDGE.md
# Verificar localmente:

# Levantar servicios
docker-compose -f docker-compose.test.yml up -d

# Ejecutar test específico
go test -tags=integration ./internal/queue/... -run TestPostgresQueue_Enqueue_Integration -v

# Si pasa, continuar
# Si falla, corregir (max 3 intentos)

# Limpiar
docker-compose -f docker-compose.test.yml down
```

#### 4. Validaciones Locales Completas

```bash
# Según checklist de PHASE2_BRIDGE.md:

# 1. Tests unitarios
go test ./... -v -count=1

# 2. Tests de integración
docker-compose -f docker-compose.test.yml up -d
sleep 5
go test -tags=integration ./... -v
docker-compose -f docker-compose.test.yml down

# 3. Race detector
go test -race ./... -timeout 60s

# 4. Build
go build ./...

# 5. Linters (si Sprint-05 completado)
golangci-lint run

# Si TODO pasa, continuar
# Si algo falla, corregir y re-ejecutar
```

#### 5. Commit de Fase 2
```bash
# Commit de trabajo de Fase 2
git add .
git commit -m "feat(sprint-XX): Fase 2 - Implementación real completa

- Stubs reemplazados con implementación real
- Integration tests con Docker/PostgreSQL/Redis
- Todas las validaciones pasando
- Listo para PR"

git push origin HEAD
```

#### 6. Creación de Pull Request

```bash
# Crear PR usando GitHub CLI
SPRINT_NAME=$(echo "$SPRINT_ID" | sed 's/Sprint-[0-9]*-//' | tr '-' ' ')
PR_TITLE="feat: $SPRINT_NAME"
PR_BODY=$(cat <<EOF
## Sprint: $SPRINT_ID

### Cambios Implementados
$(cat "$BRIDGE_DOC" | grep -A 20 "## Resumen de Fase 1")

### Tests
- ✅ Tests unitarios: Pasando
- ✅ Tests integración: Pasando
- ✅ Race detector: Sin races
- ✅ Build: Exitoso

### Validaciones
- ✅ Todas las tareas completadas
- ✅ Stubs reemplazados con implementación real
- ✅ Documentación actualizada

---
Generado automáticamente por Workflow Orchestration (Fase 2)
EOF
)

gh pr create \
  --title "$PR_TITLE" \
  --body "$PR_BODY" \
  --base dev \
  --head "$(git branch --show-current)"

# Obtener número de PR
PR_NUMBER=$(gh pr view --json number -q '.number')
echo "PR creado: #$PR_NUMBER"
```

#### 7. Monitoreo de CI/CD (CRÍTICO)

**Reglas**:
- Tiempo máximo de monitoreo: 5 minutos
- Intervalo de revisión: 1 minuto
- Si pasan 5 minutos y CI/CD no termina: DETENER e INFORMAR
- Si CI/CD falla: Corregir (max 3 intentos por error único)

```bash
# Script de monitoreo
#!/bin/bash
MAX_WAIT=300  # 5 minutos
CHECK_INTERVAL=60  # 1 minuto
START_TIME=$(date +%s)
ERROR_COUNTS=()

while true; do
  CURRENT_TIME=$(date +%s)
  ELAPSED=$((CURRENT_TIME - START_TIME))
  
  if [ $ELAPSED -gt $MAX_WAIT ]; then
    echo "❌ TIMEOUT: CI/CD no completó en 5 minutos"
    echo "Estado actual:"
    gh pr checks
    echo ""
    echo "ACCIÓN REQUERIDA: Revisar CI/CD manualmente"
    exit 1
  fi
  
  # Obtener estado de checks
  CHECKS_STATUS=$(gh pr checks --json state,name,conclusion)
  
  # Verificar si todos completaron
  IN_PROGRESS=$(echo "$CHECKS_STATUS" | jq '[.[] | select(.state=="IN_PROGRESS")] | length')
  
  if [ "$IN_PROGRESS" -eq 0 ]; then
    # Todos los checks terminaron
    FAILED=$(echo "$CHECKS_STATUS" | jq '[.[] | select(.conclusion=="FAILURE")] | length')
    
    if [ "$FAILED" -eq 0 ]; then
      echo "✅ CI/CD completado exitosamente"
      break
    else
      echo "❌ CI/CD falló. Revisando errores..."
      
      # Obtener detalles de errores
      gh run view --log | grep -E "(ERROR|FAIL)" | tail -20
      
      # Analizar error
      ERROR_MSG=$(gh run view --log | grep -E "(ERROR|FAIL)" | tail -1)
      
      # Contar ocurrencias de este error
      ERROR_HASH=$(echo "$ERROR_MSG" | md5sum | cut -d' ' -f1)
      
      # Buscar si error ya ocurrió
      ERROR_COUNT=1
      for err in "${ERROR_COUNTS[@]}"; do
        if [ "$err" == "$ERROR_HASH" ]; then
          ERROR_COUNT=$((ERROR_COUNT + 1))
        fi
      done
      
      if [ $ERROR_COUNT -gt 3 ]; then
        echo "❌ ERROR REPETIDO 3 VECES"
        echo "Error: $ERROR_MSG"
        echo ""
        echo "ACCIÓN REQUERIDA: Error no se puede resolver automáticamente"
        exit 1
      fi
      
      # Agregar error al tracking
      ERROR_COUNTS+=("$ERROR_HASH")
      
      # Intentar corregir (implementar lógica de corrección)
      echo "Intentando corregir (intento $ERROR_COUNT/3)..."
      # [Lógica de corrección según tipo de error]
      
      # Push de corrección
      git push origin HEAD
      
      # Reiniciar timer
      START_TIME=$(date +%s)
    fi
  else
    echo "⏳ CI/CD en progreso... ($ELAPSED/$MAX_WAIT segundos)"
    echo "Checks en progreso: $IN_PROGRESS"
  fi
  
  sleep $CHECK_INTERVAL
done
```

#### 8. Revisión de GitHub Copilot

```bash
# Obtener comentarios de Copilot
gh pr view $PR_NUMBER --json reviews,comments

# Filtrar comentarios de github-copilot bot
COPILOT_COMMENTS=$(gh api repos/:owner/:repo/pulls/$PR_NUMBER/comments | \
  jq '[.[] | select(.user.login=="github-copilot[bot]")]')

# Para cada comentario:
# 1. Leer comentario
# 2. Analizar si es procedente
# 3. Si es procedente: Corregir
# 4. Si NO es procedente o crea deuda técnica: Documentar en informe

# Ejemplo de análisis:
for comment in $(echo "$COPILOT_COMMENTS" | jq -c '.[]'); do
  BODY=$(echo "$comment" | jq -r '.body')
  FILE=$(echo "$comment" | jq -r '.path')
  LINE=$(echo "$comment" | jq -r '.line')
  
  echo "Comentario en $FILE:$LINE"
  echo "$BODY"
  echo ""
  
  # Análisis de procedencia
  if [[ "$BODY" =~ "complexity" ]] && [[ -f ".golangci.yml" ]]; then
    echo "✅ PROCEDENTE: Reducir complejidad"
    # [Implementar corrección]
    
  elif [[ "$BODY" =~ "error handling" ]]; then
    echo "✅ PROCEDENTE: Mejorar manejo de errores"
    # [Implementar corrección]
    
  elif [[ "$BODY" =~ "rename variable" ]]; then
    echo "⚠️  NO PROCEDENTE: Cambio cosmético, crear deuda técnica"
    # Agregar a informe de comentarios no atendidos
    
  else
    echo "❓ REQUIERE ANÁLISIS: Evaluar caso por caso"
  fi
done

# Si hay correcciones:
git add .
git commit -m "fix: atender comentarios de code review"
git push origin HEAD

# Reiniciar monitoreo CI/CD
```

#### 9. Merge a Dev

```bash
# Si TODO está OK:
# - CI/CD pasando
# - Comentarios Copilot atendidos o documentados
# - No errores repetidos

# Mergear
gh pr merge $PR_NUMBER --squash --delete-branch

echo "✅ Sprint completado y mergeado a dev"
```

#### 10. Actualización Final de Estado

```bash
# Actualizar PROGRESS.json
jq --arg sprint "$SPRINT_ID" \
   --arg date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
   '(.sprints[] | select(.id==$sprint)) |= {
     status: "completed",
     phase1_completed_at: .phase1_completed_at,
     phase2_completed_at: $date,
     merged_at: $date,
     pr_number: env.PR_NUMBER,
     branch: null
   }' \
   AnalisisEstandarizado/PROGRESS.json > tmp.json && mv tmp.json AnalisisEstandarizado/PROGRESS.json

git add AnalisisEstandarizado/PROGRESS.json
git commit -m "chore: actualizar estado sprint $SPRINT_ID a completed"
git push origin dev
```

#### 11. Reporte Final

**Archivo**: `AnalisisEstandarizado/03-Sprints/$SPRINT_ID/EXECUTION_REPORT.md`

```markdown
# Execution Report - Sprint-XX

**Sprint**: Sprint-XX-Nombre
**Estado**: Completado
**Fecha Inicio Fase 1**: 2025-11-16T09:00:00Z
**Fecha Fin Fase 2**: 2025-11-16T11:30:00Z
**Duración Total**: 2h 30m
**PR**: #123
**Merge Commit**: abc123def

---

## Fase 1 (Web)

### Completado
- ✅ TASK-001: Refactorizar session_service.go
- ✅ TASK-002: Crear .golangci.yml

### Stubs Creados
- 🔄 PostgreSQL integration test
- 🔄 Redis integration test

### Duración Fase 1
- Inicio: 2025-11-16T09:00:00Z
- Fin: 2025-11-16T09:45:00Z
- Duración: 45 minutos

---

## Fase 2 (Local)

### Stubs Reemplazados
- ✅ PostgreSQL integration test → Implementación con testcontainers
- ✅ Redis integration test → Implementación con testcontainers

### Validaciones Ejecutadas
- ✅ Tests unitarios: 156 passed
- ✅ Tests integración: 23 passed
- ✅ Race detector: 0 races
- ✅ Build: Exitoso
- ✅ Linters: 0 issues

### CI/CD
- PR creado: 2025-11-16T10:30:00Z
- CI/CD completado: 2025-11-16T10:35:00Z (5 minutos)
- Checks:
  - ✅ test (2m 15s)
  - ✅ integration-test (3m 45s)
  - ✅ golangci-lint (1m 30s)
  - ✅ codecov/project (passed)
  - ✅ codecov/patch (passed)

### Code Review (GitHub Copilot)
- Total comentarios: 8
- Atendidos: 6
- No procedentes: 2 (ver informe)

#### Comentarios Atendidos
1. Reducir complejidad en handleMessage → Refactorizado
2. Mejorar error handling en Enqueue → Agregado wrapping
3. Agregar validación null en GetSession → Agregado check
4. Documentar función ProcessEvent → Agregado godoc
5. Usar context.WithTimeout en vez de Background → Corregido
6. Agregar defer cancel en test → Agregado

#### Comentarios NO Atendidos (Informe)
1. **Renombrar variable `msg` a `message`**
   - Razón: Cambio cosmético, `msg` es convención en el proyecto
   - Impacto: Bajo
   - Recomendación: Postponer a sprint de refactoring general

2. **Mover constante a package level**
   - Razón: Constante solo usada en 1 función, crear scope innecesario
   - Impacto: Ninguno
   - Recomendación: Mantener como está

### Merge
- Merge exitoso: 2025-11-16T10:40:00Z
- Método: Squash
- Commit: abc123def456

### Duración Fase 2
- Inicio: 2025-11-16T10:00:00Z
- Fin: 2025-11-16T10:40:00Z
- Duración: 40 minutos

---

## Métricas

### Líneas de Código
- Agregadas: 1,234
- Eliminadas: 567
- Modificadas: 890

### Tests
- Tests agregados: 34
- Coverage antes: 72.3%
- Coverage después: 87.1%
- Mejora: +14.8%

### Complejidad (si Sprint-05)
- Funciones con complejidad >15 antes: 12
- Funciones con complejidad >15 después: 0
- Reducción: 100%

---

## Problemas Encontrados

### Fase 1
- Ninguno

### Fase 2
1. **Race condition en test de consumer**
   - Descripción: Variable compartida en loop
   - Solución: Captura de variable `tc := tc`
   - Intentos: 1
   - Resuelto: Sí

2. **Integration test timeout**
   - Descripción: PostgreSQL container no ready
   - Solución: Aumentar wait timeout de 10s a 30s
   - Intentos: 2
   - Resuelto: Sí

---

## Lecciones Aprendidas

1. Testcontainers requiere wait más largo en CI
2. Siempre capturar variables en table-driven tests paralelos
3. GitHub Copilot puede sugerir cambios cosméticos innecesarios

---

## Próximo Sprint

**ID**: Sprint-02-CICD
**Branch**: feature/sprint-02-cicd
**Estimación**: 8 horas
**Inicio programado**: 2025-11-17T09:00:00Z

---

**Generado por**: Claude Code Local (Fase 2)
**Fecha**: 2025-11-16T10:45:00Z
```

---

## Estado del Workflow

### Archivo Central: PROGRESS.json

```json
{
  "project": "baileys-go-debt-resolution",
  "version": "2.0.0",
  "current_sprint": "Sprint-02-CICD",
  "last_updated": "2025-11-16T10:45:00Z",
  
  "sprints": [
    {
      "id": "Sprint-01-Refactorizacion",
      "name": "Refactorización de Archivos Largos",
      "status": "completed",
      "phase1_started_at": "2025-11-16T09:00:00Z",
      "phase1_completed_at": "2025-11-16T09:45:00Z",
      "phase2_started_at": "2025-11-16T10:00:00Z",
      "phase2_completed_at": "2025-11-16T10:40:00Z",
      "merged_at": "2025-11-16T10:40:00Z",
      "branch": null,
      "pr_number": 123,
      "phase2_bridge": "AnalisisEstandarizado/03-Sprints/Sprint-01-Refactorizacion/PHASE2_BRIDGE.md",
      "execution_report": "AnalisisEstandarizado/03-Sprints/Sprint-01-Refactorizacion/EXECUTION_REPORT.md",
      "tasks": {
        "total": 4,
        "completed": 4,
        "failed": 0
      }
    },
    {
      "id": "Sprint-02-CICD",
      "name": "Reactivación CI/CD",
      "status": "pending",
      "estimated_hours": 8,
      "priority": 1
    },
    {
      "id": "Sprint-03-Context-Review",
      "status": "pending",
      "estimated_hours": 8,
      "priority": 2
    }
  ],
  
  "statistics": {
    "total_sprints": 6,
    "completed_sprints": 1,
    "in_progress_sprints": 0,
    "pending_sprints": 5,
    "total_hours_estimated": 46,
    "hours_completed": 8,
    "completion_percentage": 17.4
  }
}
```

### Estados Posibles de Sprint

| Estado | Descripción |
|--------|-------------|
| `pending` | Sprint no iniciado |
| `phase1_in_progress` | Fase 1 (Web) en ejecución |
| `phase1_completed` | Fase 1 completada, esperando Fase 2 |
| `phase2_in_progress` | Fase 2 (Local) en ejecución |
| `phase2_blocked` | Fase 2 bloqueada (error repetido 3x, timeout CI/CD, etc) |
| `completed` | Sprint completado y mergeado |
| `failed` | Sprint fallido (requiere intervención manual) |

---

## Prompts de Ejecución

### Prompt Inicial (Usuario determina Sprint)

```
Lee el archivo AnalisisEstandarizado/PROGRESS.json y dime:
1. ¿Cuál es el siguiente sprint pendiente?
2. ¿Cuál es su estado actual?
3. ¿Está listo para iniciar? (verifica dependencias)
```

### Prompt Fase 1 (Claude Code Web)

```
INICIO DE FASE 1 - EJECUCIÓN DESATENDIDA

Eres Claude Code Web ejecutando la Fase 1 del workflow de resolución de deuda técnica.

CONFIGURACIÓN:
- Proyecto: baileys-go
- Workflow: AnalisisEstandarizado/WORKFLOW_ORCHESTRATION.md
- Estado: AnalisisEstandarizado/PROGRESS.json
- Sprint a ejecutar: [SPRINT_ID determinado anteriormente]

REGLAS DE EJECUCIÓN DESATENDIDA:
1. Leer WORKFLOW_ORCHESTRATION.md sección "Fase 1"
2. Leer PROGRESS.json para obtener sprint actual
3. Leer documentación del sprint:
   - AnalisisEstandarizado/03-Sprints/[SPRINT_ID]/README.md
   - AnalisisEstandarizado/03-Sprints/[SPRINT_ID]/TASKS.md
   - AnalisisEstandarizado/03-Sprints/[SPRINT_ID]/DEPENDENCIES.md

4. Crear branch: feature/[sprint-id-lowercase]

5. Ejecutar TODAS las tareas de TASKS.md:
   - Si tarea NO requiere Docker/DB/Redis → Implementar al 100%
   - Si tarea REQUIERE Docker/DB/Redis → Crear STUB/MOCK con comentario "// TODO PHASE2: [descripción]"
   - Documentar TODOS los stubs en PHASE2_BRIDGE.md

6. Generar PHASE2_BRIDGE.md completo con:
   - Lista de stubs creados
   - Código stub actual
   - Código requerido para Fase 2
   - Validaciones pendientes
   - Checklist para Fase 2

7. Commit y push del branch

8. Actualizar PROGRESS.json:
   - Cambiar status a "phase1_completed"
   - Agregar phase1_completed_at
   - Agregar branch name
   - Agregar path a PHASE2_BRIDGE.md

9. Generar archivo PHASE2_PROMPT.txt con el prompt exacto para Fase 2

10. INFORMAR AL USUARIO:
    - Sprint procesado
    - Tareas completadas vs stubs creados
    - Ubicación de PHASE2_BRIDGE.md
    - Prompt para Fase 2 (contenido de PHASE2_PROMPT.txt)

IMPORTANTE:
- NO preguntar al usuario, ejecutar automáticamente
- NO saltarte pasos
- Documentar TODO en PHASE2_BRIDGE.md
- Genera el prompt de Fase 2 completo y específico

¿Listo para iniciar Fase 1?
```

### Prompt Fase 2 (Claude Code Local)

**Nota**: Este prompt se genera automáticamente en Fase 1 y se guarda en `PHASE2_PROMPT.txt`

Ejemplo generado:

```
INICIO DE FASE 2 - EJECUCIÓN DESATENDIDA

Eres Claude Code Local ejecutando la Fase 2 del Sprint-01-Refactorizacion del proyecto baileys-go.

CONTEXTO DE FASE 1 COMPLETADA:
- Branch creado: feature/sprint-01-refactorizacion
- Fase 1 completó: 2 tareas al 100%, 2 tareas con stubs
- Documento puente: AnalisisEstandarizado/03-Sprints/Sprint-01-Refactorizacion/PHASE2_BRIDGE.md

TU MISIÓN (EJECUCIÓN DESATENDIDA):

1. PREPARACIÓN:
   - git fetch origin
   - git checkout feature/sprint-01-refactorizacion
   - Leer PHASE2_BRIDGE.md completamente

2. IMPLEMENTACIÓN:
   - Reemplazar TODOS los stubs por implementación real
   - Usar Docker/PostgreSQL/Redis según PHASE2_BRIDGE.md
   - NO inventar código, seguir EXACTAMENTE las especificaciones del bridge

3. VALIDACIÓN LOCAL:
   - docker-compose -f docker-compose.test.yml up -d
   - go test ./... -v -count=1
   - go test -tags=integration ./... -v
   - go test -race ./... -timeout 60s
   - docker-compose -f docker-compose.test.yml down
   - Si algo falla: corregir y reintentar (max 3 veces por error único)

4. COMMIT:
   - git add .
   - git commit -m "feat(sprint-01): Fase 2 - Implementación real completa"
   - git push origin HEAD

5. PULL REQUEST:
   - gh pr create --title "feat: Refactorización de Archivos Largos" --base dev --head feature/sprint-01-refactorizacion --body "[generado automáticamente]"
   - Obtener PR number

6. MONITOREO CI/CD (CRÍTICO):
   REGLAS ESTRICTAS:
   - Tiempo máximo: 5 minutos
   - Revisar cada: 1 minuto
   - Si pasan 5 min y CI/CD no termina: DETENER e INFORMAR
   
   while true:
     - gh pr checks
     - Si todos completados:
       - Si todos OK: continuar a paso 7
       - Si hay fallos: analizar error
         - Si error ya ocurrió 3 veces: DETENER e INFORMAR
         - Si error es nuevo: corregir, push, reiniciar timer
     - Si aún en progreso:
       - Si elapsed > 5 min: DETENER e INFORMAR
       - Si elapsed <= 5 min: esperar 1 min, repetir

7. CODE REVIEW (GitHub Copilot):
   - gh pr view [PR_NUMBER] --json comments
   - Filtrar comentarios de github-copilot[bot]
   - Para cada comentario:
     - Si es procedente (mejora código, seguridad, performance): CORREGIR
     - Si NO es procedente o crea deuda técnica: DOCUMENTAR en informe (NO corregir)
   - Si hay correcciones: commit, push, reiniciar monitoreo CI/CD (volver a paso 6)

8. MERGE:
   - Si TODO OK:
     - gh pr merge [PR_NUMBER] --squash --delete-branch
     - git checkout dev
     - git pull origin dev

9. ACTUALIZACIÓN DE ESTADO:
   - Actualizar PROGRESS.json:
     - status: "completed"
     - phase2_completed_at: [timestamp]
     - merged_at: [timestamp]
     - pr_number: [PR_NUMBER]
   - git add AnalisisEstandarizado/PROGRESS.json
   - git commit -m "chore: sprint-01 completado"
   - git push origin dev

10. REPORTE FINAL:
    - Generar EXECUTION_REPORT.md
    - Incluir: duración, métricas, problemas, soluciones, comentarios no atendidos
    - INFORMAR AL USUARIO: "Sprint-01 COMPLETADO. Ver EXECUTION_REPORT.md"

REGLAS DE DETENCIÓN:
- DETENTE si monitoreo CI/CD excede 5 minutos sin terminar
- DETENTE si un error se repite 3 veces (mismo error hash)
- DETENTE si hay comentarios de Copilot que creen deuda técnica (genera informe y detente)
- Al detenerte: INFORMA con contexto completo (qué se hizo, qué falta, por qué se detuvo)

¿Listo para ejecutar Fase 2 en modo desatendido?
```

---

## Reglas de Trabajo Desatendido

### Reglas Generales

1. **No Interacción con Usuario**
   - Ejecutar automáticamente sin preguntar
   - Documentar decisiones en comentarios de código y PHASE2_BRIDGE.md
   - Solo informar al final o al detenerse

2. **Manejo de Errores**
   - Max 3 intentos por error único (usar hash MD5 del error para identificar)
   - Si error se repite 3 veces: DETENER e INFORMAR
   - Documentar cada error y solución intentada

3. **Timeouts**
   - CI/CD: Max 5 minutos de espera
   - Revisión cada 1 minuto
   - Si timeout: DETENER e INFORMAR

4. **Documentación**
   - TODO stub debe estar documentado en PHASE2_BRIDGE.md
   - TODO error debe estar documentado en EXECUTION_REPORT.md
   - TODO comentario no atendido debe estar en informe

### Reglas de Stubs (Fase 1)

```go
// Template de stub
// TODO PHASE2: [Descripción clara de lo que falta]
// Location: [archivo:línea]
// Reason: [Docker|PostgreSQL|Redis|External Service]
// Required: [Descripción de implementación requerida]

func ExampleStub() {
    // Implementación mock/stub
}
```

### Reglas de Commit

**Fase 1**:
```
feat(sprint-XX): Fase 1 - [descripción breve]

- Completado: [lista]
- Stubs: [lista]
- Ver: PHASE2_BRIDGE.md
```

**Fase 2**:
```
feat(sprint-XX): Fase 2 - Implementación real completa

- Stubs reemplazados: [lista]
- Tests: Pasando
- CI/CD: OK
```

**Correcciones en PR**:
```
fix: [descripción del error corregido]

[Descripción detallada de la corrección]
Intento: X/3
```

### Reglas de PR

**Título**:
```
feat: [Nombre del Sprint sin prefijo Sprint-XX]
```

**Body** (generado automáticamente):
```markdown
## Sprint: Sprint-XX-Nombre

### Cambios Implementados
[Lista de cambios]

### Tests
- ✅ Tests unitarios: X passed
- ✅ Tests integración: Y passed
- ✅ Race detector: 0 races
- ✅ Build: Exitoso

### Métricas
- Coverage: X% → Y% (+Z%)
- Complejidad reducida: N funciones

---
Generado automáticamente por Workflow Orchestration (Fase 2)
Sprint completado en: [duración]
```

### Reglas de Monitoreo CI/CD

```bash
# Pseudocódigo
MAX_WAIT = 300 segundos (5 min)
CHECK_INTERVAL = 60 segundos (1 min)
ERROR_TRACKING = {}

while elapsed < MAX_WAIT:
  checks = get_pr_checks()
  
  if all_completed(checks):
    if all_passed(checks):
      return SUCCESS
    else:
      errors = get_failed_checks()
      for error in errors:
        error_hash = md5(error.message)
        
        if ERROR_TRACKING[error_hash] >= 3:
          STOP_AND_INFORM("Error repetido 3 veces", error)
        
        attempt_fix(error)
        ERROR_TRACKING[error_hash] += 1
        
        push_fix()
        elapsed = 0  # Reset timer
        break
  
  else:
    if elapsed >= MAX_WAIT:
      STOP_AND_INFORM("Timeout CI/CD", get_checks_status())
    
    wait(CHECK_INTERVAL)
    elapsed += CHECK_INTERVAL

STOP_AND_INFORM("Error inesperado en monitoreo")
```

### Reglas de Comentarios Copilot

**Procedentes** (CORREGIR):
- Mejoras de seguridad
- Corrección de bugs
- Mejoras de performance
- Reducción de complejidad (si Sprint-05 completado)
- Mejoras de error handling
- Agregar validaciones

**No Procedentes** (DOCUMENTAR, NO CORREGIR):
- Cambios cosméticos (renombrar variables sin razón funcional)
- Mover código sin beneficio claro
- Agregar abstracciones innecesarias
- Cambios que rompen convenciones del proyecto
- Cambios que crean deuda técnica mayor

**Informe de Comentarios No Atendidos**:
```markdown
## Comentarios de Code Review NO Atendidos

### 1. Renombrar variable `msg` a `message`
- **Archivo**: internal/queue/consumer.go:45
- **Razón**: Cambio cosmético, `msg` es convención del proyecto
- **Impacto**: Ninguno
- **Recomendación**: Mantener como está

### 2. Mover función X a nuevo paquete
- **Archivo**: internal/session/service.go:123
- **Razón**: Crearía dependencia circular
- **Impacto**: Alto (deuda técnica)
- **Recomendación**: Considerar en refactoring arquitectónico futuro
```

---

**Última Actualización**: 2025-11-16  
**Versión del Workflow**: 1.0.0  
**Estado**: Listo para ejecución
