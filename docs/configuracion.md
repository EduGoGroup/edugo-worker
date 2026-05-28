# Configuración y Ambientes — EduGo Worker

> Documentación generada desde análisis directo del código fuente (marzo 2026).

---

## 1. Sistema de Configuración

El worker usa **Viper** para cargar configuración con múltiples fuentes y merge automático.

**Archivo:** `internal/config/config.go`

### Struct Config principal

```go
type Config struct {
    Database        DatabaseConfig        `mapstructure:"database"`
    Messaging       MessagingConfig       `mapstructure:"messaging"`
    NLP             NLPConfig             `mapstructure:"nlp"`
    Storage         StorageConfig         `mapstructure:"storage"`
    PDF             PDFConfig             `mapstructure:"pdf"`
    Logging         LoggingConfig         `mapstructure:"logging"`
    APIAdmin        APIAdminConfig        `mapstructure:"api_admin"`
    Metrics         MetricsConfig         `mapstructure:"metrics"`
    Health          HealthConfig          `mapstructure:"health"`
    CircuitBreakers CircuitBreakersConfig `mapstructure:"circuit_breakers"`
    RateLimiter     RateLimiterConfig     `mapstructure:"rate_limiter"`
    Shutdown        ShutdownConfig        `mapstructure:"shutdown"`
}
```

---

## 2. Archivos YAML

| Archivo | Propósito |
|---------|-----------|
| `config/config.yaml` | Configuración base (defaults para todos los ambientes) |
| `config/config-local.yaml` | Override para desarrollo local |
| `config/config-dev.yaml` | Override para ambiente DEV |
| `config/config-qa.yaml` | Override para QA |
| `config/config-prod.yaml` | Override para producción |

---

## 3. Precedencia de Configuración

```
Variables de entorno     ← Mayor prioridad
       ↓
config-{APP_ENV}.yaml    ← Override por ambiente
       ↓
config.yaml              ← Valores base (defaults)
```

La variable `APP_ENV` determina qué archivo de override se carga (default: `local`).

---

## 4. Variables de Entorno

| Variable | Requerida | Default | Descripción |
|----------|-----------|---------|-------------|
| `APP_ENV` | No | `local` | Ambiente: `local`, `dev`, `qa`, `prod` |
| `POSTGRES_PASSWORD` | **Sí** | — | Contraseña de PostgreSQL |
| `MONGODB_URI` | **Sí** | — | URI de conexión MongoDB |
| `RABBITMQ_URL` | **Sí** | — | URL de conexión RabbitMQ |
| `OPENAI_API_KEY` | No | — | API key de OpenAI (si no hay, usa fallback) |
| `ANTHROPIC_API_KEY` | No | — | API key de Anthropic Claude (alternativa) |

### Validación

El método `Config.Validate()` verifica las 3 variables requeridas al inicio:

```go
func (c *Config) Validate() error {
    if c.Database.Postgres.Password == "" {
        return fmt.Errorf("POSTGRES_PASSWORD is required")
    }
    if c.Database.MongoDB.URI == "" {
        return fmt.Errorf("MONGODB_URI is required")
    }
    if c.Messaging.RabbitMQ.URL == "" {
        return fmt.Errorf("RABBITMQ_URL is required")
    }
    // NLP.APIKey es opcional - si no está, usamos SmartFallback
    return nil
}
```

---

## 5. Configuración Base Completa (`config.yaml`)

```yaml
# Base de datos
database:
  postgres:
    host: "localhost"
    port: 5432
    database: "edugo"
    user: "edugo_user"
    max_connections: 10
    ssl_mode: "disable"
  mongodb:
    database: "edugo"
    timeout: 10s

# Mensajería
messaging:
  rabbitmq:
    queues:
      material_uploaded: "edugo.material.uploaded"
      assessment_attempt: "edugo.assessment.attempt"
    exchanges:
      materials: "edugo.materials"
    prefetch_count: 5

# NLP (IA para generación de contenido)
nlp:
  provider: "openai"          # "openai", "anthropic", "mock"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4-turbo-preview"
    max_tokens: 4096
    temperature: 0.7
    timeout: "30s"
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-3-sonnet-20240229"
    max_tokens: 4096
    temperature: 0.7
    timeout: "30s"

# Logging
logging:
  level: "info"
  format: "json"

# Autenticación centralizada con api-admin
api_admin:
  base_url: "http://api-admin:8081"
  timeout: "5s"
  cache_ttl: "60s"
  cache_enabled: true
  max_bulk_size: 50

# Health Checks
health:
  timeouts:
    mongodb: "5s"
    postgres: "3s"
    rabbitmq: "3s"

# Circuit Breakers
circuit_breakers:
  nlp:
    max_failures: 5
    timeout: "60s"
    max_requests: 1
    success_threshold: 2
  storage:
    max_failures: 5
    timeout: "60s"
    max_requests: 1
    success_threshold: 2

# Rate Limiter
rate_limiter:
  enabled: true
  by_event_type:
    material.uploaded:
      requests_per_second: 5.0    # PDF es costoso
      burst_size: 10.0
    material.updated:
      requests_per_second: 10.0
      burst_size: 20.0
    assessment.attempt:
      requests_per_second: 15.0   # Mayor throughput
      burst_size: 30.0
  default:
    requests_per_second: 10.0
    burst_size: 20.0

# Graceful Shutdown
shutdown:
  timeout: "30s"
  wait_for_messages: true
```

---

## 6. Ambientes

### Local (desarrollo)

```
APP_ENV=local
POSTGRES_PASSWORD=edugo_pass
MONGODB_URI=mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
RABBITMQ_URL=amqp://edugo_user:edugo_pass@localhost:5672/
OPENAI_API_KEY=sk-test-key
```

### Dev / QA / Prod

Cada ambiente tiene su propio `config-{env}.yaml` con overrides específicos (URLs de producción, credenciales vía secrets, etc.).

---

## 7. Docker

### Dockerfile

El Dockerfile usa una imagen **minimal de runtime** — la compilación ocurre en el CI:

```dockerfile
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Binario pre-compilado por el CI
COPY main .
COPY config/ ./config/

RUN chmod +x ./main

CMD ["./main"]
```

**Nota:** No hay multi-stage build con compilación Go — el binario se compila en el CI (`manual-release.yml`) y se copia al contenedor.

### docker-compose.yml

```yaml
version: '3.8'

services:
  worker:
    build: .
    container_name: edugo-worker
    environment:
      APP_ENV: ${APP_ENV:-local}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-edugo_pass}
      MONGODB_URI: ${MONGODB_URI:-mongodb://...}
      RABBITMQ_URL: ${RABBITMQ_URL:-amqp://...}
      OPENAI_API_KEY: ${OPENAI_API_KEY}
    volumes:
      - ./config:/app/config:ro
      - ./logs:/app/logs
    networks:
      - edugo-network
    restart: unless-stopped

networks:
  edugo-network:
    external: true
```

El worker se conecta a la red `edugo-network` donde corren PostgreSQL, MongoDB y RabbitMQ.

---

## 8. Makefile — Targets Principales

| Target | Descripción |
|--------|-------------|
| `make help` | Mostrar todos los targets disponibles |
| `make build` | Compilar binario en `bin/worker` |
| `make run` | Ejecutar en modo desarrollo (`go run`) |
| `make test` | Ejecutar todos los tests con race detector |
| `make test-coverage` | Tests + reporte HTML de cobertura |
| `make test-unit` | Solo tests unitarios (short mode) |
| `make test-integration` | Solo tests de integración (tag: integration) |
| `make fmt` | Formatear código con gofmt |
| `make vet` | Análisis estático con go vet |
| `make lint` | Linter completo con golangci-lint |
| `make audit` | Auditoría: verify + format + vet + tests |
| `make deps` | Descargar dependencias |
| `make tidy` | Limpiar go.mod |
| `make docker-build` | Build imagen Docker |
| `make docker-run` | Run con docker-compose |
| `make docker-stop` | Stop docker-compose |
| `make clean` | Limpiar binarios, cache, coverage |
| `make ci` | Pipeline CI: audit + coverage + swagger |
| `make pre-commit` | Pre-commit hook: fmt + vet + test |
| `make all` | Build completo: clean → deps → fmt → vet → test → build |
| `make info` | Información del proyecto (versión, ambiente, Go version) |

### Variables de ambiente en Makefile

```makefile
export APP_ENV ?= local
export POSTGRES_PASSWORD ?= edugo_pass
export MONGODB_URI ?= mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
export RABBITMQ_URL ?= amqp://edugo_user:edugo_pass@localhost:5672/
export OPENAI_API_KEY ?= sk-test-key
```

---

## 9. Cómo Agregar una Nueva Variable de Configuración

### Paso 1: Agregar al struct en `config.go`

```go
type Config struct {
    // ... existentes ...
    MiNuevaConfig MiNuevaConfig `mapstructure:"mi_nueva_config"`
}

type MiNuevaConfig struct {
    Habilitado bool   `mapstructure:"habilitado"`
    Valor      string `mapstructure:"valor"`
}
```

### Paso 2: Agregar defaults en `config.yaml`

```yaml
mi_nueva_config:
  habilitado: false
  valor: "default"
```

### Paso 3: (Opcional) Agregar método con defaults

```go
func (c *Config) GetMiNuevaConfigWithDefaults() MiNuevaConfig {
    cfg := c.MiNuevaConfig
    if cfg.Valor == "" {
        cfg.Valor = "valor-por-defecto"
    }
    return cfg
}
```

### Paso 4: Agregar override por ambiente

En `config-dev.yaml`, `config-prod.yaml`, etc.:

```yaml
mi_nueva_config:
  habilitado: true
  valor: "valor-produccion"
```

### Paso 5: (Opcional) Variable de entorno

Viper automáticamente mapea env vars con el formato `MI_NUEVA_CONFIG_VALOR` (uppercase, underscores).

---

## 10. Métodos de Configuración con Defaults

El patrón `Get*WithDefaults()` asegura que siempre hay valores razonables:

| Método | Defaults |
|--------|----------|
| `GetActiveNLPConfig()` | model: gpt-4-turbo-preview, maxTokens: 4096, temp: 0.7, timeout: 30s |
| `GetAPIAdminConfigWithDefaults()` | baseURL: localhost:8081, timeout: 5s, cacheTTL: 60s |
| `GetMetricsConfigWithDefaults()` | port: 9090 |
| `GetHealthConfigWithDefaults()` | MongoDB: 5s, Postgres: 3s, RabbitMQ: 3s |
| `GetCircuitBreakerConfigWithDefaults()` | maxFailures: 5, timeout: 60s, maxRequests: 1 |
| `GetRateLimiterConfigWithDefaults()` | rps: 10, burst: 20 |
| `GetShutdownConfigWithDefaults()` | timeout: 30s |

---

*Generado: marzo 2026 | Basado en análisis directo del código fuente*
