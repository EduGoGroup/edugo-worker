# Configuración - EduGo Worker

## 📋 Visión General

El worker utiliza un sistema de configuración basado en **YAML + Variables de Entorno**, con soporte para múltiples ambientes (local, dev, qa, prod).

---

## 🗂️ Archivos de Configuración

```
config/
├── config.yaml          # Configuración base (defaults)
├── config-local.yaml    # Override para desarrollo local
├── config-dev.yaml      # Override para ambiente de desarrollo
├── config-qa.yaml       # Override para QA
└── config-prod.yaml     # Override para producción
```

### Precedencia de Configuración

```
1. Variables de entorno (mayor prioridad)
2. config-{APP_ENV}.yaml
3. config.yaml (base/defaults)
```

---

## 🔧 Variables de Entorno

### Variables Requeridas

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `APP_ENV` | Ambiente de ejecución | `local`, `dev`, `qa`, `prod` |
| `POSTGRES_PASSWORD` | Contraseña PostgreSQL | `edugo_pass` |
| `MONGODB_URI` | URI completa de MongoDB | `mongodb://user:pass@host:27017/db?authSource=admin` |
| `RABBITMQ_URL` | URL de conexión RabbitMQ | `amqp://user:pass@host:5672/` |
| `OPENAI_API_KEY` | API Key de OpenAI | `sk-...` |

### Variables Opcionales

| Variable | Descripción | Default |
|----------|-------------|---------|
| `POSTGRES_HOST` | Host PostgreSQL | `localhost` |
| `POSTGRES_PORT` | Puerto PostgreSQL | `5432` |
| `POSTGRES_USER` | Usuario PostgreSQL | `edugo_user` |
| `POSTGRES_DATABASE` | Nombre BD PostgreSQL | `edugo` |
| `LOG_LEVEL` | Nivel de log | `info` |
| `LOG_FORMAT` | Formato de log | `json` |

---

## 📄 Configuración Base (config.yaml)

```yaml
# Configuración Base - Worker

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

messaging:
  rabbitmq:
    queues:
      material_uploaded: "edugo.material.uploaded"
      assessment_attempt: "edugo.assessment.attempt"
    exchanges:
      materials: "edugo.materials"
    prefetch_count: 5

nlp:
  provider: "openai"
  model: "gpt-4"
  max_tokens: 4000
  temperature: 0.7

logging:
  level: "info"
  format: "json"

api_admin:
  base_url: "http://api-admin:8081"
  timeout: "5s"
  cache_ttl: "60s"
  cache_enabled: true
  max_bulk_size: 50
```

---

## 🏠 Configuración Local (config-local.yaml)

```yaml
# Override para desarrollo local

database:
  postgres:
    host: "localhost"
    port: 5432
    ssl_mode: "disable"

  mongodb:
    # URI se toma de variable de entorno MONGODB_URI
    database: "edugo"

messaging:
  rabbitmq:
    prefetch_count: 1  # Menos prefetch para debugging

logging:
  level: "debug"  # Más verbose en local
  format: "text"  # Más legible que JSON

api_admin:
  base_url: "http://localhost:8081"
```

---

## 🚀 Configuración Producción (config-prod.yaml)

```yaml
# Override para producción

database:
  postgres:
    host: "postgres.internal"
    port: 5432
    ssl_mode: "require"
    max_connections: 25

  mongodb:
    timeout: 30s

messaging:
  rabbitmq:
    prefetch_count: 10  # Mayor throughput

nlp:
  model: "gpt-4-turbo"  # Modelo más rápido
  max_tokens: 8000

logging:
  level: "info"
  format: "json"

api_admin:
  base_url: "http://api-admin.internal:8081"
  timeout: "10s"
  cache_ttl: "120s"
```

---

## 🔨 Estructura de Config en Go

```go
// internal/config/config.go

type Config struct {
    Database  DatabaseConfig  `mapstructure:"database"`
    Messaging MessagingConfig `mapstructure:"messaging"`
    NLP       NLPConfig       `mapstructure:"nlp"`
    Logging   LoggingConfig   `mapstructure:"logging"`
    APIAdmin  APIAdminConfig  `mapstructure:"api_admin"`
}

type DatabaseConfig struct {
    Postgres PostgresConfig `mapstructure:"postgres"`
    MongoDB  MongoDBConfig  `mapstructure:"mongodb"`
}

type PostgresConfig struct {
    Host           string `mapstructure:"host"`
    Port           int    `mapstructure:"port"`
    Database       string `mapstructure:"database"`
    User           string `mapstructure:"user"`
    Password       string `mapstructure:"password"`      // Desde env
    MaxConnections int    `mapstructure:"max_connections"`
    SSLMode        string `mapstructure:"ssl_mode"`
}

type MongoDBConfig struct {
    URI      string        `mapstructure:"uri"`          // Desde env
    Database string        `mapstructure:"database"`
    Timeout  time.Duration `mapstructure:"timeout"`
}

type MessagingConfig struct {
    RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
}

type RabbitMQConfig struct {
    URL           string         `mapstructure:"url"`    // Desde env
    Queues        QueuesConfig   `mapstructure:"queues"`
    Exchanges     ExchangeConfig `mapstructure:"exchanges"`
    PrefetchCount int            `mapstructure:"prefetch_count"`
}

type QueuesConfig struct {
    MaterialUploaded  string `mapstructure:"material_uploaded"`
    AssessmentAttempt string `mapstructure:"assessment_attempt"`
}

type ExchangeConfig struct {
    Materials string `mapstructure:"materials"`
}

type NLPConfig struct {
    Provider    string  `mapstructure:"provider"`
    APIKey      string  `mapstructure:"api_key"`         // Desde env
    Model       string  `mapstructure:"model"`
    MaxTokens   int     `mapstructure:"max_tokens"`
    Temperature float64 `mapstructure:"temperature"`
}

type LoggingConfig struct {
    Level  string `mapstructure:"level"`
    Format string `mapstructure:"format"`
}

type APIAdminConfig struct {
    BaseURL      string        `mapstructure:"base_url"`
    Timeout      time.Duration `mapstructure:"timeout"`
    CacheTTL     time.Duration `mapstructure:"cache_ttl"`
    CacheEnabled bool          `mapstructure:"cache_enabled"`
    MaxBulkSize  int           `mapstructure:"max_bulk_size"`
}
```

---

## ✅ Validación de Configuración

```go
// internal/config/config.go

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
    if c.NLP.APIKey == "" {
        return fmt.Errorf("OPENAI_API_KEY is required")
    }
    return nil
}
```

---

## 🐳 Docker Environment

### docker-compose.yml

```yaml
version: '3.8'

services:
  worker:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: edugo-worker
    environment:
      APP_ENV: ${APP_ENV:-local}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-edugo_pass}
      MONGODB_URI: ${MONGODB_URI:-mongodb://edugo_admin:edugo_pass@mongodb:27017/edugo?authSource=admin}
      RABBITMQ_URL: ${RABBITMQ_URL:-amqp://edugo_user:edugo_pass@rabbitmq:5672/}
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
    name: edugo-network
```

### Dockerfile Build Args

```dockerfile
# Argumento para GitHub token (acceso a repos privados)
ARG GITHUB_TOKEN

# Variables de entorno para Go
ENV GOPRIVATE=github.com/EduGoGroup/*
ENV GONOSUMDB=github.com/EduGoGroup/*
```

---

## 🔐 Manejo de Secretos

### Recomendaciones

1. **Nunca** commitear secretos en archivos YAML
2. Usar variables de entorno para credenciales
3. En producción usar:
   - AWS Secrets Manager
   - HashiCorp Vault
   - Kubernetes Secrets

### Ejemplo con .envrc (direnv)

```bash
# .envrc (NO commitear este archivo)
export APP_ENV=local
export POSTGRES_PASSWORD=edugo_pass
export MONGODB_URI=mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
export RABBITMQ_URL=amqp://edugo_user:edugo_pass@localhost:5672/
export OPENAI_API_KEY=sk-your-key-here
```

---

## 📊 Configuración por Ambiente

### Comparativa de Ambientes

| Configuración | Local | Dev | QA | Prod |
|--------------|-------|-----|-----|------|
| Log Level | debug | debug | info | info |
| Log Format | text | json | json | json |
| PostgreSQL SSL | disable | disable | require | require |
| PostgreSQL Max Conn | 5 | 10 | 15 | 25 |
| MongoDB Timeout | 10s | 10s | 20s | 30s |
| RabbitMQ Prefetch | 1 | 5 | 5 | 10 |
| OpenAI Model | gpt-4 | gpt-4 | gpt-4 | gpt-4-turbo |
| Cache TTL | 60s | 60s | 60s | 120s |

---

## 🔄 Recarga de Configuración

Actualmente el worker **no soporta** recarga en caliente de configuración. Para aplicar cambios:

1. Modificar archivos YAML o variables de entorno
2. Reiniciar el worker

```bash
# Con Docker
docker-compose restart worker

# Desarrollo local
# Ctrl+C y volver a ejecutar
make run
```

---

## 📝 Makefile Defaults

El Makefile define valores por defecto para desarrollo:

```makefile
# Environment variables con defaults
export APP_ENV ?= local
export POSTGRES_PASSWORD ?= edugo_pass
export MONGODB_URI ?= mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
export RABBITMQ_URL ?= amqp://edugo_user:edugo_pass@localhost:5672/
export OPENAI_API_KEY ?= sk-test-key
```

---

## 🛠️ Comandos Útiles

```bash
# Ver configuración actual
make info

# Ejecutar con ambiente específico
APP_ENV=qa make run

# Ejecutar con variables personalizadas
POSTGRES_PASSWORD=mypass MONGODB_URI=mongodb://... make run

# Validar configuración (logs al inicio)
make run  # Ver logs de inicialización
```
