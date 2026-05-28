# Tareas - Fase 0: Actualización de Dependencias

---

## 📋 Lista de Tareas

### T0.1: Preparación del Entorno

**Estimación:** 15 minutos

- [ ] Verificar que estás en rama `dev` actualizada
- [ ] Crear rama `chore/fase-0-actualizar-dependencias`
- [ ] Verificar acceso a repositorios de dependencias

**Comandos:**
```bash
git checkout dev
git pull origin dev
git checkout -b chore/fase-0-actualizar-dependencias
```

---

### T0.2: Consultar Últimas Versiones Disponibles

**Estimación:** 10 minutos

- [ ] Consultar últimas versiones de `edugo-infrastructure`
- [ ] Consultar últimas versiones de `edugo-shared`
- [ ] Documentar versiones objetivo en un archivo temporal

**Comandos:**
```bash
# Opción 1: Desde los repositorios locales
cd ../edugo-infrastructure
git fetch --tags
git tag -l | sort -V | tail -n 5

cd ../edugo-shared
git fetch --tags
git tag -l | sort -V | tail -n 5

cd ../edugo-worker

# Opción 2: Consultar directamente con go
go list -m -versions github.com/EduGoGroup/edugo-infrastructure/mongodb
go list -m -versions github.com/EduGoGroup/edugo-shared/bootstrap
```

**Resultado esperado:**
Conocer las versiones más recientes de cada dependencia.

---

### T0.3: Actualizar edugo-infrastructure

**Estimación:** 5 minutos

- [ ] Actualizar `edugo-infrastructure/mongodb` a la última versión
- [ ] Ejecutar `go mod tidy`
- [ ] Verificar que no hay errores en `go.mod`

**Comandos:**
```bash
go get github.com/EduGoGroup/edugo-infrastructure/mongodb@latest
go mod tidy
```

**Validación:**
```bash
grep "edugo-infrastructure" go.mod
```

---

### T0.4: Actualizar edugo-shared (todos los módulos)

**Estimación:** 10 minutos

- [ ] Actualizar `edugo-shared/bootstrap`
- [ ] Actualizar `edugo-shared/common`
- [ ] Actualizar `edugo-shared/database/postgres`
- [ ] Actualizar `edugo-shared/lifecycle`
- [ ] Actualizar `edugo-shared/logger`
- [ ] Actualizar `edugo-shared/testing`
- [ ] Ejecutar `go mod tidy`

**Comandos:**
```bash
go get github.com/EduGoGroup/edugo-shared/bootstrap@latest
go get github.com/EduGoGroup/edugo-shared/common@latest
go get github.com/EduGoGroup/edugo-shared/database/postgres@latest
go get github.com/EduGoGroup/edugo-shared/lifecycle@latest
go get github.com/EduGoGroup/edugo-shared/logger@latest
go get github.com/EduGoGroup/edugo-shared/testing@latest
go mod tidy
```

**Validación:**
```bash
grep "edugo-shared" go.mod
```

---

### T0.5: Verificar y Limpiar Dependencias Indirectas

**Estimación:** 5 minutos

- [ ] Ejecutar `go mod tidy` nuevamente
- [ ] Verificar que no hay dependencias duplicadas
- [ ] Verificar que `go.sum` está actualizado

**Comandos:**
```bash
go mod tidy
go mod verify
```

---

### T0.6: Compilar Proyecto

**Estimación:** 2 minutos

- [ ] Limpiar builds anteriores
- [ ] Compilar con `make build`
- [ ] Verificar que no hay errores de compilación

**Comandos:**
```bash
# Limpiar
make clean 2>/dev/null || rm -rf bin/

# Compilar
make build
```

**Criterio de éxito:**
- Salida: `Build successful` o similar
- Sin errores de compilación
- Binario generado en `bin/`

**Si falla:**
- Revisar mensajes de error
- Identificar breaking changes en dependencias
- Consultar CHANGELOG de la dependencia
- Adaptar código si es necesario

---

### T0.7: Ejecutar Tests Unitarios

**Estimación:** 5 minutos

- [ ] Ejecutar todos los tests unitarios
- [ ] Verificar que todos pasan
- [ ] Documentar cobertura de tests

**Comandos:**
```bash
# Ejecutar tests
make test

# O manualmente
go test ./... -v

# Ver cobertura
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

**Criterio de éxito:**
- Todos los tests pasan
- Cobertura se mantiene o mejora

**Si fallan tests:**
1. Identificar qué tests fallan
2. Revisar si son por breaking changes
3. Adaptar tests si es necesario
4. Documentar cambios

---

### T0.8: Ejecutar Tests de Integración (si existen)

**Estimación:** 10 minutos

- [ ] Verificar si existen tests de integración
- [ ] Configurar ambiente si es necesario (Docker, etc.)
- [ ] Ejecutar tests de integración
- [ ] Verificar que todos pasan

**Comandos:**
```bash
# Verificar si existe make target
make test-integration 2>/dev/null

# O ejecutar con Docker
TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./...
```

**Si no existen tests de integración:**
- Marcar como N/A
- Continuar con siguiente tarea

---

### T0.9: Ejecutar Linters

**Estimación:** 3 minutos

- [ ] Ejecutar `make lint` (si existe)
- [ ] Ejecutar `go vet`
- [ ] Ejecutar `golangci-lint` (si está instalado)
- [ ] Verificar que no hay errores críticos

**Comandos:**
```bash
# Opción 1: Make target
make lint 2>/dev/null

# Opción 2: Manual
go vet ./...

# Opción 3: golangci-lint (si está instalado)
golangci-lint run ./...
```

**Criterio de éxito:**
- Sin errores de linter
- Warnings aceptables pueden quedar (documentar)

---

### T0.10: Verificar Dependencias Deprecadas

**Estimación:** 5 minutos

- [ ] Buscar uso de `streadway/amqp` (deprecada)
- [ ] Verificar que solo se usa `rabbitmq/amqp091-go`
- [ ] Eliminar `streadway/amqp` si no se usa

**Comandos:**
```bash
# Buscar imports de streadway/amqp
grep -r "streadway/amqp" --include="*.go" .

# Ver por qué está en dependencias
go mod why github.com/streadway/amqp

# Si no se usa directamente, verificar dependencias transitivas
go mod graph | grep streadway
```

**Acción:**
- Si aparece en código: crear issue para eliminarlo en Fase 1
- Si es dependencia transitiva: documentar y dejar para después

---

### T0.11: Crear Archivo de Resumen de Cambios

**Estimación:** 10 minutos

- [ ] Crear archivo `fase-0/CAMBIOS.md` con resumen
- [ ] Documentar versiones anteriores y nuevas
- [ ] Documentar breaking changes encontrados (si hay)
- [ ] Documentar adaptaciones realizadas (si hay)

**Plantilla de CAMBIOS.md:**
```markdown
# Cambios - Fase 0

## Versiones Actualizadas

| Dependencia | Versión Anterior | Versión Nueva | Breaking Changes |
|-------------|------------------|---------------|------------------|
| edugo-infrastructure/mongodb | v0.10.1 | vX.Y.Z | No |
| edugo-shared/bootstrap | v0.9.0 | vX.Y.Z | No |
| ... | ... | ... | ... |

## Adaptaciones Realizadas

Ninguna / Listar aquí si hubo cambios de código

## Tests

- Tests unitarios: X/X pasan ✅
- Tests integración: Y/Y pasan ✅
- Cobertura: Z%

## Notas

- streadway/amqp aún en dependencias transitivas
```

---

### T0.12: Commit de Cambios

**Estimación:** 5 minutos

- [ ] Revisar cambios con `git status`
- [ ] Agregar solo `go.mod` y `go.sum` (inicialmente)
- [ ] Crear commit descriptivo
- [ ] Agregar archivo `CAMBIOS.md` si fue creado

**Comandos:**
```bash
# Revisar cambios
git status
git diff go.mod

# Commit
git add go.mod go.sum
git add plan-mejoras/fase-0/CAMBIOS.md  # Si existe

git commit -m "chore(fase-0): actualizar dependencias edugo-infrastructure y edugo-shared

- Actualizar edugo-infrastructure/mongodb a vX.Y.Z
- Actualizar edugo-shared/bootstrap a vX.Y.Z
- Actualizar edugo-shared/common a vX.Y.Z
- Actualizar edugo-shared/database/postgres a vX.Y.Z
- Actualizar edugo-shared/lifecycle a vX.Y.Z
- Actualizar edugo-shared/logger a vX.Y.Z
- Actualizar edugo-shared/testing a vX.Y.Z

Validación:
- ✅ Compilación exitosa
- ✅ Tests pasan (X/X)
- ✅ Linters sin errores
- ✅ Sin warnings críticos

Fase: 0
Refs: plan-mejoras/fase-0/README.md"
```

---

### T0.13: Push y Crear Pull Request

**Estimación:** 10 minutos

- [ ] Push de la rama a origin
- [ ] Crear Pull Request en GitHub
- [ ] Agregar descripción completa
- [ ] Agregar labels: `fase-0`, `dependencies`, `chore`
- [ ] Solicitar revisión (si aplica)

**Comandos:**
```bash
git push origin chore/fase-0-actualizar-dependencias
```

**Luego en GitHub:**
1. Ir a repositorio
2. Crear nuevo PR
3. Usar template (ver `README.md` de fase-0)
4. Agregar labels
5. Asignar reviewers

---

### T0.14: Esperar CI/CD y Review

**Estimación:** Variable (depende del equipo)

- [ ] Verificar que CI/CD pasa todos los checks
- [ ] Responder a comentarios de review (si hay)
- [ ] Hacer ajustes si son solicitados
- [ ] Obtener aprobación

**Criterio de éxito:**
- ✅ CI/CD verde
- ✅ Aprobación de al menos 1 reviewer (si aplica)
- ✅ Sin conflictos con `dev`

---

### T0.15: Merge y Limpieza

**Estimación:** 5 minutos

- [ ] Hacer merge del PR a `dev`
- [ ] Eliminar rama remota (opcional)
- [ ] Checkout a `dev` y pull
- [ ] Crear tag `fase-0-complete`
- [ ] Push del tag

**Comandos:**
```bash
# Después del merge en GitHub
git checkout dev
git pull origin dev

# Crear tag
git tag -a fase-0-complete -m "Fase 0 completada: Dependencias actualizadas"
git push origin fase-0-complete

# Eliminar rama local
git branch -d chore/fase-0-actualizar-dependencias
```

---

### T0.16: Actualizar Documentación de Progreso

**Estimación:** 5 minutos

- [ ] Actualizar tabla de progreso en `plan-mejoras/README.md`
- [ ] Marcar Fase 0 como completada
- [ ] Documentar fecha de inicio y fin
- [ ] Documentar link al PR

**Cambios en `plan-mejoras/README.md`:**
```markdown
| Fase | Estado | Inicio | Fin | PR |
|------|--------|--------|-----|-----|
| Fase 0 | ✅ Completada | 2024-XX-XX | 2024-XX-XX | #123 |
| Fase 1 | ⏳ Pendiente | - | - | - |
```

---

## 📊 Resumen de Estimaciones

| Tarea | Estimación |
|-------|------------|
| T0.1 - T0.5 | 45 min |
| T0.6 - T0.10 | 25 min |
| T0.11 - T0.13 | 25 min |
| T0.14 | Variable |
| T0.15 - T0.16 | 10 min |
| **Total (sin review)** | **~1.75 horas** |

**Con review y CI/CD:** 2-4 horas (mismo día) o 1-2 días (si hay demoras en review)

---

## ✅ Checklist Final

Antes de considerar la Fase 0 como completa:

- [ ] Todas las tareas T0.1 - T0.16 completadas
- [ ] PR mergeado a `dev`
- [ ] CI/CD pasa en `dev`
- [ ] Tag `fase-0-complete` creado y pusheado
- [ ] Documentación actualizada
- [ ] Sin errores de compilación
- [ ] Todos los tests pasan
