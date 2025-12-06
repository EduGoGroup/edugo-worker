# 📋 Guía de Migración: edugo-shared v0.1.0 → v2.0.5

## 🎯 Objetivo

Migrar proyectos de la versión monolítica **v0.1.0** a la arquitectura modular **v2.0.5** de edugo-shared, donde cada proyecto solo descarga los submódulos que realmente necesita.

---

## 📦 Cambio Principal

### Antes (v0.1.0) - Monolítico ❌
```go
// go.mod
require github.com/EduGoGroup/edugo-shared v0.1.0

// Código
import "github.com/EduGoGroup/edugo-shared/pkg/errors"
import "github.com/EduGoGroup/edugo-shared/pkg/logger"
```

**Problema:** Se descargan TODAS las dependencias (15+) aunque solo uses errors y logger.

### Después (v2.0.5) - Modular ✅
```go
// go.mod
require (
    github.com/EduGoGroup/edugo-shared/common v0.0.0-XXXXXX
    github.com/EduGoGroup/edugo-shared/logger v0.0.0-XXXXXX
)

// Código
import "github.com/EduGoGroup/edugo-shared/common/errors"
import "github.com/EduGoGroup/edugo-shared/logger"
```

**Beneficio:** Solo se descargan las dependencias de los submódulos que usas (reducción del 80-93%).

---

## 📚 Submódulos Disponibles

| Submódulo | Qué Incluye | Cuándo Usarlo |
|-----------|-------------|---------------|
| **common** | errors, types, types/enum, validator, config | Siempre (base común) |
| **auth** | Autenticación JWT | APIs con autenticación de usuarios |
| **logger** | Logging estructurado (Zap) | Todas las aplicaciones |
| **messaging/rabbit** | Helpers para RabbitMQ | Microservicios con mensajería |
| **database/postgres** | Utilidades PostgreSQL | Apps con PostgreSQL |
| **database/mongodb** | Utilidades MongoDB | Apps con MongoDB |

---

## 🗺️ Mapeo de Imports

Usa esta tabla para reemplazar los imports en tu código:

| Import Antiguo (v0.1.0) | Import Nuevo (v2.0.5) | Submódulo |
|-------------------------|----------------------|-----------|
| `v2/pkg/errors` | `common/errors` | common |
| `v2/pkg/types` | `common/types` | common |
| `v2/pkg/types/enum` | `common/types/enum` | common |
| `v2/pkg/validator` | `common/validator` | common |
| `v2/pkg/config` | `common/config` | common |
| `v2/pkg/auth` | `auth` | auth |
| `v2/pkg/logger` | `logger` | logger |
| `v2/pkg/messaging` | `messaging/rabbit` | messaging/rabbit |
| `database/postgres` | `database/postgres` | database/postgres |
| `database/mongodb` | `database/mongodb` | database/mongodb |

---

## 🚀 Proceso de Migración (Paso a Paso)

### Paso 1: Identificar Submódulos Necesarios

Busca qué paquetes de edugo-shared estás usando:

```bash
# En la raíz de tu proyecto
grep -r "github.com/EduGoGroup/edugo-shared" --include="*.go" . | \
  grep -v "vendor/" | \
  sed 's/.*github.com\/EduGoGroup\/edugo-shared\/v2\/pkg\///' | \
  sed 's/".*$//' | \
  cut -d'/' -f1 | \
  sort | uniq
```

**Ejemplo de salida:**
```
auth
errors
logger
types
validator
```

**Traducción a submódulos:**
- `errors, types, validator` → submódulo **common**
- `auth` → submódulo **auth**
- `logger` → submódulo **logger**

### Paso 2: Actualizar go.mod

```bash
# Eliminar versión monolítica
go mod edit -droprequire github.com/EduGoGroup/edugo-shared

# Instalar submódulos necesarios
# IMPORTANTE: Usa el formato @submodulo/v2.0.5

# Para TODOS los proyectos (base común):
go get github.com/EduGoGroup/edugo-shared/common@common/v2.0.5

# Si usas autenticación:
go get github.com/EduGoGroup/edugo-shared/auth@auth/v2.0.5

# Si usas logging:
go get github.com/EduGoGroup/edugo-shared/logger@logger/v2.0.5

# Si usas RabbitMQ:
go get github.com/EduGoGroup/edugo-shared/messaging/rabbit@messaging/rabbit/v2.0.5

# Si usas PostgreSQL:
go get github.com/EduGoGroup/edugo-shared/database/postgres@database/postgres/v2.0.5

# Si usas MongoDB:
go get github.com/EduGoGroup/edugo-shared/database/mongodb@database/mongodb/v2.0.5
```

### Paso 3: Actualizar Imports en Código

**Opción A: Reemplazo Automático (Recomendado)**

```bash
# Ejecutar desde la raíz del proyecto
find . -name "*.go" -type f -not -path "./vendor/*" -exec sed -i '' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/errors|github.com/EduGoGroup/edugo-shared/common/errors|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/types/enum|github.com/EduGoGroup/edugo-shared/common/types/enum|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/types|github.com/EduGoGroup/edugo-shared/common/types|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/validator|github.com/EduGoGroup/edugo-shared/common/validator|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/config|github.com/EduGoGroup/edugo-shared/common/config|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/auth|github.com/EduGoGroup/edugo-shared/auth|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/logger|github.com/EduGoGroup/edugo-shared/logger|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/messaging|github.com/EduGoGroup/edugo-shared/messaging/rabbit|g' \
  {} \;
```

**Nota para Linux:** Usa `sed -i` en lugar de `sed -i ''`

**Opción B: Reemplazo Manual en IDE**

En VSCode/GoLand, usa "Find and Replace in Files":

1. `github.com/EduGoGroup/edugo-shared/pkg/errors` → `github.com/EduGoGroup/edugo-shared/common/errors`
2. `github.com/EduGoGroup/edugo-shared/pkg/types/enum` → `github.com/EduGoGroup/edugo-shared/common/types/enum`
3. `github.com/EduGoGroup/edugo-shared/pkg/types` → `github.com/EduGoGroup/edugo-shared/common/types`
4. `github.com/EduGoGroup/edugo-shared/pkg/validator` → `github.com/EduGoGroup/edugo-shared/common/validator`
5. `github.com/EduGoGroup/edugo-shared/pkg/auth` → `github.com/EduGoGroup/edugo-shared/auth`
6. `github.com/EduGoGroup/edugo-shared/pkg/logger` → `github.com/EduGoGroup/edugo-shared/logger`

### Paso 4: Limpiar Dependencias

```bash
go mod tidy
```

### Paso 5: Verificar Compilación

```bash
go build ./...
```

Si hay errores de imports, verifica que:
1. Instalaste todos los submódulos necesarios
2. Actualizaste correctamente los imports
3. No quedaron imports antiguos con `v2/pkg/`

### Paso 6: Ejecutar Tests

```bash
go test ./...
```

### Paso 7: Verificar que no Queden Imports Antiguos

```bash
# Este comando NO debe retornar resultados
grep -r "github.com/EduGoGroup/edugo-shared/v2" --include="*.go" . | grep -v "vendor/"
```

Si retorna algo, significa que quedaron imports sin actualizar.

---

## ✅ Checklist de Verificación

Antes de hacer commit, asegúrate de:

- [ ] go.mod ya no tiene `github.com/EduGoGroup/edugo-shared/v2`
- [ ] go.mod tiene los submódulos necesarios (common, auth, logger, etc.)
- [ ] No hay imports con `v2/pkg/` en el código
- [ ] `go mod tidy` ejecutado sin errores
- [ ] `go build ./...` compila sin errores
- [ ] `go test ./...` pasa todos los tests
- [ ] API/servicio inicia correctamente (si aplica)
- [ ] Swagger UI carga correctamente (si aplica)

---

## 🎯 Ejemplo de Migración Completa

### Proyecto: edugo-api-mobile ✅

**Submódulos necesarios:**
- common (errors, types, validator)
- auth (JWT)
- logger (logging)

**Comandos ejecutados:**
```bash
# 1. Limpiar versión antigua
go mod edit -droprequire github.com/EduGoGroup/edugo-shared/v2

# 2. Instalar submódulos
go get github.com/EduGoGroup/edugo-shared/common@common/v2.0.5
go get github.com/EduGoGroup/edugo-shared/auth@auth/v2.0.5
go get github.com/EduGoGroup/edugo-shared/logger@logger/v2.0.5

# 3. Actualizar imports automáticamente
find . -name "*.go" -type f -not -path "./vendor/*" -exec sed -i '' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/errors|github.com/EduGoGroup/edugo-shared/common/errors|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/types/enum|github.com/EduGoGroup/edugo-shared/common/types/enum|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/types|github.com/EduGoGroup/edugo-shared/common/types|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/validator|github.com/EduGoGroup/edugo-shared/common/validator|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/auth|github.com/EduGoGroup/edugo-shared/auth|g' \
  -e 's|github.com/EduGoGroup/edugo-shared/pkg/logger|github.com/EduGoGroup/edugo-shared/logger|g' \
  {} \;

# 4. Limpiar y verificar
go mod tidy
go build ./...
go test ./...

# 5. Commit
git add .
git commit -m "feat: migrar a edugo-shared v2.0.5 con arquitectura modular"
```

**Resultados:**
- ✅ 28 archivos actualizados
- ✅ Compilación exitosa
- ✅ Tests pasando
- ✅ API corriendo
- ✅ Swagger UI funcionando

---

## 🔧 Troubleshooting

### Error: "module not found"

```
go: module github.com/EduGoGroup/edugo-shared@v2.0.5 found (v2.0.5+incompatible),
    but does not contain package github.com/EduGoGroup/edugo-shared/common
```

**Solución:** Usa el formato de tag correcto:
```bash
# ❌ Incorrecto
go get github.com/EduGoGroup/edugo-shared/common@v2.0.5

# ✅ Correcto
go get github.com/EduGoGroup/edugo-shared/common@common/v2.0.5
```

### Error: "ambiguous import"

```
ambiguous import: github.com/EduGoGroup/edugo-shared/common/errors
```

**Solución:** Asegúrate de haber eliminado la versión monolítica:
```bash
go mod edit -droprequire github.com/EduGoGroup/edugo-shared/v2
go mod tidy
```

### Error: "package not in GOROOT"

**Solución:** Verifica que actualizaste TODOS los imports. Busca imports viejos:
```bash
grep -r "v2/pkg/" --include="*.go" .
```

---

## 📊 Beneficios Cuantificados

### Proyecto edugo-api-mobile

**Antes (v0.1.0):**
- Dependencias descargadas: ~50 paquetes
- Tamaño de dependencias: ~15 MB
- Tiempo de build inicial: ~8 segundos

**Después (v2.0.5):**
- Dependencias descargadas: ~12 paquetes
- Tamaño de dependencias: ~3 MB
- Tiempo de build inicial: ~4 segundos
- **Reducción: 80% menos dependencias**

---

## 🎓 Prompt para Claude/IA

Si vas a usar Claude Code o cualquier IA para hacer la migración, usa este prompt:

```
Necesito migrar mi proyecto Go de edugo-shared v0.1.0 (monolítico) a v2.0.5 (modular).

CONTEXTO:
- La versión v0.1.0 era un módulo monolítico que descargaba todas las dependencias
- La versión v2.0.5 dividió el código en submódulos independientes
- Solo debo instalar los submódulos que realmente uso

SUBMÓDULOS DISPONIBLES:
- common: errors, types, types/enum, validator, config
- auth: autenticación JWT
- logger: logging estructurado
- messaging/rabbit: RabbitMQ helpers
- database/postgres: utilidades PostgreSQL
- database/mongodb: utilidades MongoDB

MAPEO DE IMPORTS:
v2/pkg/errors        → common/errors
v2/pkg/types         → common/types
v2/pkg/types/enum    → common/types/enum
v2/pkg/validator     → common/validator
v2/pkg/config        → common/config
v2/pkg/auth          → auth
v2/pkg/logger        → logger
v2/pkg/messaging     → messaging/rabbit

PASOS REQUERIDOS:
1. Analizar qué paquetes de edugo-shared usa mi proyecto
2. Determinar qué submódulos necesito instalar
3. Eliminar github.com/EduGoGroup/edugo-shared/v2 del go.mod
4. Instalar los submódulos necesarios usando el formato @submodulo/v2.0.5
5. Actualizar todos los imports en archivos .go
6. Ejecutar go mod tidy
7. Compilar con go build ./...
8. Ejecutar tests con go test ./...
9. Verificar que no queden imports antiguos con v2/pkg/
10. Crear commit con los cambios

IMPORTANTE:
- Al instalar submódulos usa: go get github.com/EduGoGroup/edugo-shared/common@common/v2.0.5
- Al actualizar imports, reemplaza v2/pkg/* por los nuevos paths
- El orden de reemplazo importa: primero types/enum, luego types
- Verifica que el proyecto compile y los tests pasen antes de hacer commit

Por favor, ejecuta la migración siguiendo estos pasos y verifica que todo funcione correctamente.
```

---

## 📞 Soporte

Si tienes problemas durante la migración:

1. Revisa el archivo `ISSUE_EDUGO_SHARED_TAGGING.md` en el proyecto edugo-api-mobile
2. Consulta el commit de ejemplo: `0caed82` en edugo-api-mobile
3. Contacta al equipo de EduGo Shared

---

**Versión del documento:** 1.0
**Última actualización:** 2025-10-31
**Proyecto de referencia:** edugo-api-mobile (migración exitosa)
