# Configuración por Ambientes - Worker

## Archivos

- `config.yaml` - Configuración base (común a todos los ambientes)
- `config-local.yaml` - Desarrollo local
- `config-dev.yaml` - Servidor de desarrollo
- `config-qa.yaml` - QA/Staging
- `config-prod.yaml` - Producción

## Cómo Funciona

La aplicación carga configuración con esta precedencia (mayor a menor):

1. **Variables de ambiente** (ej: `EDUGO_WORKER_SERVER_PORT=9090`)
2. **Archivo específico** (ej: `config-dev.yaml`)
3. **Archivo base** (`config.yaml`)
4. **Defaults** (valores por defecto en código)

## Uso

### Cambiar Ambiente

```bash
# Local (default)
go run cmd/main.go

# Development
APP_ENV=dev go run cmd/main.go

# QA
APP_ENV=qa go run cmd/main.go

# Production
APP_ENV=prod go run cmd/main.go
```

### Sobrescribir con ENV Variables

```bash
# Cambiar puerto
APP_ENV=dev EDUGO_WORKER_SERVER_PORT=9090 go run cmd/main.go

# Cambiar log level
EDUGO_WORKER_LOGGING_LEVEL=debug go run cmd/main.go
```

## Variables de Ambiente

Prefijo: `EDUGO_WORKER_`

Formato: Reemplazar `.` con `_` y convertir a MAYÚSCULAS

Ejemplos:
- `server.port` → `EDUGO_WORKER_SERVER_PORT`
- `database.postgres.host` → `EDUGO_WORKER_DATABASE_POSTGRES_HOST`
- `logging.level` → `EDUGO_WORKER_LOGGING_LEVEL`

## Secretos

**NUNCA** commitear secretos en archivos YAML.

Usar variables de ambiente:
- `POSTGRES_PASSWORD` - Password de PostgreSQL
- `MONGODB_URI` - URI completa de MongoDB
- `RABBITMQ_URL` - URL completa de RabbitMQ

## Agregar Nueva Configuración

1. Agregar campo en `internal/config/config.go`
2. Agregar valor en `config.yaml` (base)
3. Sobrescribir en archivos específicos si es necesario
4. Usar en código: `cfg.NuevoCampo`
