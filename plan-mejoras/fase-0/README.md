# Fase 0: Actualización de Dependencias

> **Objetivo:** Actualizar todas las dependencias de `edugo-infrastructure` y `edugo-shared` a sus últimas versiones y validar que el proyecto compila y todos los tests pasan.
>
> **Duración estimada:** 1-2 días
> **Complejidad:** Baja
> **Riesgo:** Bajo

---

## 🎯 Objetivos

1. ✅ Actualizar `edugo-infrastructure` a la última versión disponible
2. ✅ Actualizar todos los módulos de `edugo-shared` a la última versión
3. ✅ Resolver conflictos de dependencias si existen
4. ✅ Validar que el proyecto compila sin errores
5. ✅ Validar que todos los tests existentes pasan
6. ✅ Eliminar warnings de deprecación

---

## 📦 Dependencias a Actualizar

### Estado Actual (desde go.mod)

```go
github.com/EduGoGroup/edugo-infrastructure/mongodb v0.10.1
github.com/EduGoGroup/edugo-shared/bootstrap v0.9.0
github.com/EduGoGroup/edugo-shared/common v0.7.0
github.com/EduGoGroup/edugo-shared/database/postgres v0.7.0
github.com/EduGoGroup/edugo-shared/lifecycle v0.7.0
github.com/EduGoGroup/edugo-shared/logger v0.7.0
github.com/EduGoGroup/edugo-shared/testing v0.7.0
```

### Versiones Objetivo

Se actualizarán a las últimas versiones disponibles en los repositorios respectivos.

---

## 🔄 Proceso de Actualización

### Paso 1: Crear Rama

```bash
git checkout dev
git pull origin dev
git checkout -b chore/fase-0-actualizar-dependencias
```

### Paso 2: Verificar Últimas Versiones

```bash
# Para infraestructura
cd ../edugo-infrastructure
git fetch --tags
git tag -l | sort -V | tail -5

# Para shared
cd ../edugo-shared
git fetch --tags
git tag -l | sort -V | tail -5

# Volver a worker
cd ../edugo-worker
```

### Paso 3: Actualizar go.mod

```bash
# Actualizar infraestructura
go get github.com/EduGoGroup/edugo-infrastructure/mongodb@latest

# Actualizar shared modules
go get github.com/EduGoGroup/edugo-shared/bootstrap@latest
go get github.com/EduGoGroup/edugo-shared/common@latest
go get github.com/EduGoGroup/edugo-shared/database/postgres@latest
go get github.com/EduGoGroup/edugo-shared/lifecycle@latest
go get github.com/EduGoGroup/edugo-shared/logger@latest
go get github.com/EduGoGroup/edugo-shared/testing@latest

# Limpiar dependencias
go mod tidy
```

### Paso 4: Validar Compilación

```bash
# Limpiar build anterior
make clean || rm -rf bin/

# Compilar
make build
```

**Criterio de éxito:** Compilación exitosa sin errores.

### Paso 5: Ejecutar Tests

```bash
# Tests unitarios
make test

# Ver cobertura
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Criterio de éxito:** Todos los tests pasan (100% de los existentes).

### Paso 6: Validar Linters

```bash
# Si existe make lint
make lint

# O manualmente
go vet ./...
golangci-lint run ./...
```

### Paso 7: Verificar No Hay Warnings de Deprecación

```bash
# Buscar warnings en compilación
go build -v ./... 2>&1 | grep -i "deprecat"

# Buscar uso de streadway/amqp (deprecada)
grep -r "streadway/amqp" --include="*.go" .

# Si se encuentra, debe eliminarse
go mod why github.com/streadway/amqp
```

---

## 📋 Checklist de Validación

Antes de hacer commit y PR, verificar:

- [ ] `go.mod` actualizado con nuevas versiones
- [ ] `go sum` regenerado correctamente
- [ ] `make build` exitoso sin errores ni warnings
- [ ] `make test` pasa todos los tests
- [ ] `make lint` (o `go vet`) sin errores
- [ ] No hay dependencias deprecadas (verificar `streadway/amqp`)
- [ ] Código compila en modo release: `go build -ldflags="-s -w" ./cmd/`
- [ ] Tests de integración pasan (si existen): `make test-integration`

---

## 🐛 Troubleshooting

### Error: Dependencia no encontrada

**Problema:**
```
go: github.com/EduGoGroup/edugo-shared/bootstrap@v0.10.0: invalid version: unknown revision v0.10.0
```

**Solución:**
```bash
# Verificar versiones disponibles
go list -m -versions github.com/EduGoGroup/edugo-shared/bootstrap

# Usar la última versión disponible
go get github.com/EduGoGroup/edugo-shared/bootstrap@vX.Y.Z
```

### Error: Conflicto de dependencias

**Problema:**
```
go: github.com/EduGoGroup/edugo-shared/common@v0.8.0 requires
    github.com/some/dependency@v2.0.0 but
    github.com/another/module requires v1.0.0
```

**Solución:**
```bash
# Ver árbol de dependencias
go mod graph | grep conflicting-package

# Forzar versión compatible
go get github.com/some/dependency@v2.0.0
go mod tidy
```

### Error: Tests fallan después de actualización

**Problema:**
Algunos tests fallan después de actualizar dependencias.

**Solución:**
1. Revisar CHANGELOG de la dependencia actualizada
2. Identificar breaking changes
3. Adaptar código según cambios
4. Si es complejo, crear issue y revertir actualización temporalmente

---

## 📝 Commit y PR

### Formato de Commit

```bash
git add go.mod go.sum
git commit -m "chore(fase-0): actualizar dependencias edugo-infrastructure y edugo-shared

- Actualizar edugo-infrastructure/mongodb a vX.Y.Z
- Actualizar edugo-shared/bootstrap a vX.Y.Z
- Actualizar edugo-shared/common a vX.Y.Z
- Actualizar edugo-shared/database/postgres a vX.Y.Z
- Actualizar edugo-shared/lifecycle a vX.Y.Z
- Actualizar edugo-shared/logger a vX.Y.Z
- Actualizar edugo-shared/testing a vX.Y.Z
- Ejecutar go mod tidy

Validación:
- ✅ Compilación exitosa
- ✅ Todos los tests pasan (X/X)
- ✅ Linters sin errores
- ✅ Sin dependencias deprecadas

Fase: 0
Refs: plan-mejoras/fase-0/README.md"
```

### Crear Pull Request

```bash
git push origin chore/fase-0-actualizar-dependencias
```

**Título del PR:**
```
chore: Fase 0 - Actualizar dependencias edugo-infrastructure y edugo-shared
```

**Descripción del PR:**
```markdown
## 🎯 Objetivo

Actualizar todas las dependencias de `edugo-infrastructure` y `edugo-shared` como prerequisito para las siguientes fases de mejoras.

## 📦 Dependencias Actualizadas

| Dependencia | Versión Anterior | Versión Nueva |
|-------------|------------------|---------------|
| edugo-infrastructure/mongodb | v0.10.1 | vX.Y.Z |
| edugo-shared/bootstrap | v0.9.0 | vX.Y.Z |
| edugo-shared/common | v0.7.0 | vX.Y.Z |
| edugo-shared/database/postgres | v0.7.0 | vX.Y.Z |
| edugo-shared/lifecycle | v0.7.0 | vX.Y.Z |
| edugo-shared/logger | v0.7.0 | vX.Y.Z |
| edugo-shared/testing | v0.7.0 | vX.Y.Z |

## ✅ Validación

- [x] Compilación exitosa
- [x] Tests pasan (X/X)
- [x] Linters sin errores
- [x] Sin warnings de deprecación
- [x] Sin dependencias deprecadas

## 🔗 Referencias

- Plan: `plan-mejoras/fase-0/README.md`
- Documentación: `documents/mejoras/`

## 🏷️ Labels

`fase-0` `dependencies` `chore`
```

---

## 📊 Criterios de Aceptación

Para considerar la Fase 0 como **completada**:

1. ✅ PR mergeado a `dev`
2. ✅ CI/CD pasa en `dev` después del merge
3. ✅ Tag `fase-0-complete` creado
4. ✅ Documentación de progreso actualizada en `plan-mejoras/README.md`

---

## ⏭️ Siguiente Fase

Una vez completada la Fase 0, proceder con:

**Fase 1: Funcionalidad Crítica**
- Implementar routing real de eventos
- Eliminar código deprecado
- Refactorizar bootstrap

Ver: `plan-mejoras/fase-1/README.md`
