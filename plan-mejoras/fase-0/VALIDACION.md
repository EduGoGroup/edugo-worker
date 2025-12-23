# Validación - Fase 0: Actualización de Dependencias

---

## 🎯 Objetivo de Validación

Asegurar que la actualización de dependencias no introduce regresiones y que el sistema mantiene su funcionalidad básica.

---

## ✅ Checklist de Validación Pre-PR

### 1. Compilación

```bash
# Limpiar builds anteriores
make clean || rm -rf bin/

# Compilar
make build
```

**Criterios:**
- [ ] Compilación exitosa sin errores
- [ ] Binario generado en directorio esperado
- [ ] Sin warnings críticos de compilación
- [ ] Tamaño del binario similar al anterior (±10%)

**Salida esperada:**
```
go build -o bin/worker ./cmd/
Build successful
```

---

### 2. Tests Unitarios

```bash
# Ejecutar todos los tests
go test ./... -v

# Con cobertura
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Criterios:**
- [ ] Todos los tests existentes pasan (100%)
- [ ] Tiempo de ejecución similar al anterior (±20%)
- [ ] Cobertura se mantiene o mejora
- [ ] Sin tests omitidos (skipped) inesperadamente

**Salida esperada:**
```
ok      github.com/EduGoGroup/edugo-worker/internal/...    0.XXXs
PASS
coverage: XX.X% of statements
```

**Registro de Tests:**
```markdown
| Paquete | Tests | Pass | Fail | Skip | Tiempo |
|---------|-------|------|------|------|--------|
| internal/application/processor | X | X | 0 | 0 | XXXms |
| internal/domain/service | X | X | 0 | 0 | XXXms |
| internal/infrastructure/... | X | X | 0 | 0 | XXXms |
| **TOTAL** | **X** | **X** | **0** | **0** | **XXXms** |
```

---

### 3. Tests de Integración

```bash
# Si existen
make test-integration

# O con Docker
docker-compose -f docker-compose.test.yml up --abort-on-container-exit

# O con testcontainers
TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration ./...
```

**Criterios:**
- [ ] Tests de integración pasan (si existen)
- [ ] Conexiones a DB (PostgreSQL/MongoDB) funcionan
- [ ] Conexión a RabbitMQ funciona
- [ ] No hay fugas de recursos (conexiones, goroutines)

**Si no hay tests de integración:**
- [x] Marcar como N/A
- [ ] Considerar agregar en Fase 3

---

### 4. Linters y Análisis Estático

```bash
# go vet
go vet ./...

# golangci-lint (si está disponible)
golangci-lint run ./...

# staticcheck (si está disponible)
staticcheck ./...
```

**Criterios:**
- [ ] Sin errores de `go vet`
- [ ] Sin errores críticos de linters
- [ ] Warnings documentados si existen

**Warnings aceptables:**
- Comentarios sin formato específico
- Variables no usadas en código de test
- Importaciones agrupadas de forma no estándar

**Warnings NO aceptables:**
- Posibles nil pointer dereference
- Variables declaradas pero no usadas
- Errores ignorados sin justificación

---

### 5. Dependencias

```bash
# Verificar integridad
go mod verify

# Buscar dependencias deprecadas
grep -r "streadway/amqp" --include="*.go" .

# Ver árbol de dependencias
go mod graph | head -20

# Listar todas las dependencias
go list -m all
```

**Criterios:**
- [ ] `go mod verify` exitoso
- [ ] No hay uso directo de `streadway/amqp` en código
- [ ] `go.mod` y `go.sum` consistentes
- [ ] Sin dependencias con vulnerabilidades conocidas

**Verificar vulnerabilidades:**
```bash
# Si está disponible govulncheck
govulncheck ./...
```

---

### 6. Compilación en Diferentes Plataformas

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/worker-linux ./cmd/

# macOS (si aplica)
GOOS=darwin GOARCH=amd64 go build -o bin/worker-darwin ./cmd/

# Windows (opcional)
GOOS=windows GOARCH=amd64 go build -o bin/worker-windows.exe ./cmd/
```

**Criterios:**
- [ ] Compilación exitosa en Linux
- [ ] Compilación exitosa en macOS (si aplica)
- [ ] Sin errores de cross-compilation

---

### 7. Verificación de Cambios en go.mod

```bash
# Ver diferencias
git diff dev go.mod
git diff dev go.sum

# Ver qué cambió
go mod graph | diff - <(git show dev:go.mod | go mod graph 2>/dev/null)
```

**Criterios:**
- [ ] Solo cambiaron versiones de dependencias esperadas
- [ ] No se agregaron dependencias inesperadas
- [ ] No se eliminaron dependencias necesarias

**Documentar:**
```markdown
## Cambios en go.mod

### Actualizadas
- github.com/EduGoGroup/edugo-infrastructure/mongodb: v0.10.1 → vX.Y.Z
- github.com/EduGoGroup/edugo-shared/bootstrap: v0.9.0 → vX.Y.Z
...

### Agregadas
- Ninguna (o listar si hay)

### Eliminadas
- Ninguna (o listar si hay)
```

---

## 🧪 Tests Manuales (Opcional pero Recomendado)

### Test 1: Inicialización del Worker

**Objetivo:** Verificar que el worker inicia correctamente.

**Pasos:**
```bash
# Configurar variables de entorno mínimas
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=test
export POSTGRES_PASSWORD=test
export POSTGRES_DB=edugo_test
export MONGODB_URI=mongodb://localhost:27017
export RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Ejecutar worker
./bin/worker
```

**Resultado esperado:**
- Worker inicia sin panic
- Logs muestran conexión a servicios
- Worker queda esperando mensajes

**Criterio de éxito:**
- [ ] Worker inicia sin errores
- [ ] Logs muestran inicialización correcta
- [ ] Conexiones a DBs establecidas

---

### Test 2: Shutdown Graceful

**Objetivo:** Verificar que el worker se cierra correctamente.

**Pasos:**
1. Iniciar worker
2. Enviar señal SIGTERM: `kill -TERM <pid>`
3. Observar logs

**Resultado esperado:**
- Worker recibe señal
- Completa procesamiento en curso
- Cierra conexiones limpiamente
- Termina con exit code 0

**Criterio de éxito:**
- [ ] Shutdown sin panic
- [ ] Logs muestran cierre ordenado
- [ ] Sin conexiones huérfanas

---

## 📋 Checklist Final Pre-PR

Antes de crear el Pull Request, verificar:

### Compilación y Tests
- [ ] `make build` exitoso
- [ ] `make test` todos los tests pasan
- [ ] `make lint` sin errores críticos
- [ ] Tests de integración pasan (si existen)

### Dependencias
- [ ] `go mod verify` exitoso
- [ ] `go mod tidy` ejecutado
- [ ] Sin dependencias deprecadas directas
- [ ] Vulnerabilidades verificadas (si tool disponible)

### Código
- [ ] Sin cambios de código (solo go.mod/go.sum)
- [ ] Sin warnings de compilación críticos
- [ ] Cross-compilation funciona

### Documentación
- [ ] Archivo `CAMBIOS.md` creado (si aplica)
- [ ] Versiones documentadas
- [ ] Breaking changes documentados (si hay)

### Git
- [ ] Commit con mensaje descriptivo
- [ ] Solo archivos necesarios en commit
- [ ] Rama pusheada a origin

---

## 📝 Checklist Post-PR

Después de crear el Pull Request:

### CI/CD
- [ ] Todos los checks de CI/CD pasan
- [ ] Tests automatizados exitosos
- [ ] Build en CI exitoso
- [ ] Linters en CI sin errores

### Review
- [ ] PR tiene descripción completa
- [ ] Labels correctos agregados
- [ ] Reviewer asignado (si aplica)
- [ ] Comentarios respondidos

### Merge
- [ ] Aprobación recibida
- [ ] Sin conflictos con `dev`
- [ ] Merge completado
- [ ] CI/CD en `dev` pasa después del merge

---

## 🚨 Validación Post-Merge

Después del merge a `dev`:

```bash
# Actualizar dev local
git checkout dev
git pull origin dev

# Verificar que todo compila
make build

# Verificar tests
make test

# Crear tag
git tag -a fase-0-complete -m "Fase 0: Dependencias actualizadas"
git push origin fase-0-complete
```

**Criterios:**
- [ ] `dev` compila después del merge
- [ ] Tests pasan en `dev`
- [ ] Tag `fase-0-complete` creado
- [ ] Documentación actualizada

---

## 📊 Reporte de Validación

**Template para documentar resultados:**

```markdown
# Reporte de Validación - Fase 0

**Fecha:** YYYY-MM-DD
**Ejecutado por:** [Nombre]
**Rama:** chore/fase-0-actualizar-dependencias
**Commit:** [hash]

## Resultados

### Compilación
- ✅ Build exitoso
- ✅ Sin warnings críticos
- Tiempo: XXs

### Tests Unitarios
- ✅ Todos pasan (X/X)
- Cobertura: XX.X%
- Tiempo: XXXms

### Tests Integración
- ✅ Todos pasan (X/X) / ⚠️ N/A
- Tiempo: XXXms

### Linters
- ✅ go vet: sin errores
- ✅ golangci-lint: sin errores / ⚠️ N/A
- Warnings: X

### Dependencias
- ✅ go mod verify: OK
- ✅ Sin deprecadas directas
- ✅ Sin vulnerabilidades conocidas / ⚠️ No verificado

### Tests Manuales
- ✅ Worker inicia correctamente
- ✅ Shutdown graceful funciona

## Versiones Actualizadas

| Dependencia | Antes | Después |
|-------------|-------|---------|
| edugo-infrastructure/mongodb | v0.10.1 | vX.Y.Z |
| edugo-shared/bootstrap | v0.9.0 | vX.Y.Z |
| ... | ... | ... |

## Problemas Encontrados

Ninguno / [Descripción de problemas y soluciones]

## Conclusión

✅ Validación exitosa - Listo para PR
⚠️ Requiere ajustes - [Detallar]
❌ Bloqueado - [Detallar]

## Próximos Pasos

1. Crear PR
2. Esperar CI/CD
3. Solicitar review
4. Merge a dev
```

---

## 🔧 Troubleshooting

### Problema: Tests fallan después de actualización

**Diagnóstico:**
```bash
# Ver qué tests fallan
go test ./... -v | grep FAIL

# Ejecutar test específico con más detalle
go test -v ./internal/path/to/package -run TestName
```

**Posibles causas:**
1. Breaking change en dependencia
2. Cambio en comportamiento de librería
3. Test flaky que ahora falla consistentemente

**Solución:**
1. Revisar CHANGELOG de dependencia actualizada
2. Adaptar test si es breaking change esperado
3. Reportar bug en dependencia si es inesperado

---

### Problema: Compilación falla

**Diagnóstico:**
```bash
# Ver error completo
go build -v ./... 2>&1 | tee build-error.log

# Ver dependencias del paquete que falla
go list -f '{{.Deps}}' ./path/to/failing/package
```

**Posibles causas:**
1. API cambió en dependencia
2. Tipo removido o renombrado
3. Firma de función cambió

**Solución:**
1. Identificar breaking change exacto
2. Adaptar código (crear issue si es extenso)
3. Considerar downgrade temporal si no es crítico

---

### Problema: Linters reportan nuevos errores

**Diagnóstico:**
```bash
# Ver errores específicos
golangci-lint run --no-config --disable-all --enable=X

# Ver qué cambió en reglas
golangci-lint linters
```

**Solución:**
1. Si son errores reales: corregir
2. Si son false positives: documentar y deshabilitar regla específica
3. Si son warnings menores: crear issue para fase posterior

---

## ✅ Criterio de Aceptación Global

La Fase 0 se considera **EXITOSA** si:

1. ✅ Todas las dependencias actualizadas
2. ✅ Proyecto compila sin errores
3. ✅ Todos los tests existentes pasan
4. ✅ Linters sin errores críticos
5. ✅ CI/CD pasa en PR
6. ✅ PR mergeado a `dev`
7. ✅ Tag `fase-0-complete` creado
8. ✅ Sin regresiones evidentes

Si **alguno falla**, documentar y decidir:
- Adaptar código (si es menor)
- Crear issue y continuar (si no bloquea)
- Revertir actualización (si bloquea completamente)
