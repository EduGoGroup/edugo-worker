# Servicios y Dependencias - EduGo Worker

## 📋 Visión General

El worker depende de varios servicios externos para funcionar correctamente. Este documento detalla cada dependencia, cómo se conecta y qué se necesita para configurarla.

---

## 🔗 Mapa de Dependencias

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         DEPENDENCY MAP                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│                        ┌─────────────────┐                                   │
│                        │   EDUGO WORKER  │                                   │
│                        └────────┬────────┘                                   │
│                                 │                                            │
│         ┌───────────┬───────────┼───────────┬───────────┐                   │
│         │           │           │           │           │                   │
│         ▼           ▼           ▼           ▼           ▼                   │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐│
│  │ PostgreSQL │ │  MongoDB   │ │  RabbitMQ  │ │  OpenAI    │ │  AWS S3    ││
│  │            │ │            │ │            │ │  (GPT-4)   │ │            ││
│  │ Estado de  │ │ Contenido  │ │ Mensajería │ │ Generación │ │ PDFs       ││
│  │ materiales │ │ generado   │ │ eventos    │ │ IA         │ │ archivos   ││
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘ └────────────┘│
│                                                                              │
│         ┌───────────────────────────────────────────────────────────────┐   │
│         │                     SERVICIOS INTERNOS                         │   │
│         │                                                                │   │
│         │  ┌─────────────┐         ┌─────────────────────────────────┐  │   │
│         │  │  api-admin  │         │  edugo-shared (librería)        │  │   │
│         │  │             │         │                                  │  │   │
│         │  │ Validación  │         │ • bootstrap                      │  │   │
│         │  │ de tokens   │         │ • database/postgres              │  │   │
│         │  │ JWT         │         │ • logger                         │  │   │
│         │  └─────────────┘         │ • common/errors                  │  │   │
│         │                          │ • common/types                   │  │   │
│         │                          └─────────────────────────────────┘  │   │
│         └───────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 🗄️ PostgreSQL

### Propósito
Base de datos relacional para almacenar el **estado** de los materiales (tabla `materials` compartida con APIs).

### Conexión

| Parámetro | Valor Default | Variable de Entorno |
|-----------|---------------|---------------------|
| Host | `localhost` | `POSTGRES_HOST` |
| Port | `5432` | `POSTGRES_PORT` |
| Database | `edugo` | `POSTGRES_DATABASE` |
| User | `edugo_user` | `POSTGRES_USER` |
| Password | - | `POSTGRES_PASSWORD` ⚠️ |
| SSL Mode | `disable` | - |
| Max Connections | `10` | - |

### Verificación de Conexión

```bash
# Usando psql
psql -h localhost -U edugo_user -d edugo -c "SELECT 1;"

# O desde el worker (logs)
# ✅ Worker iniciado correctamente → conexión OK
```

### Tablas Utilizadas

- `materials` - Lectura/escritura del campo `processing_status`

### Docker Compose (ejemplo)

```yaml
postgres:
  image: postgres:15-alpine
  environment:
    POSTGRES_USER: edugo_user
    POSTGRES_PASSWORD: edugo_pass
    POSTGRES_DB: edugo
  ports:
    - "5432:5432"
  networks:
    - edugo-network
```

---

## 🍃 MongoDB

### Propósito
Base de datos documental para almacenar **contenido generado** (resúmenes, evaluaciones, eventos).

### Conexión

| Parámetro | Variable de Entorno |
|-----------|---------------------|
| URI Completa | `MONGODB_URI` ⚠️ |
| Database | `edugo` (en config.yaml) |
| Timeout | `10s` (configurable) |

### Formato URI

```
mongodb://[user]:[password]@[host]:[port]/[database]?authSource=admin
```

### Ejemplo

```bash
MONGODB_URI=mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin
```

### Colecciones Utilizadas

| Colección | Uso |
|-----------|-----|
| `material_summary` | Resúmenes generados por IA |
| `material_assessment_worker` | Evaluaciones/quizzes |
| `material_events` | Log de eventos procesados |

### Verificación de Conexión

```bash
# Usando mongosh
mongosh "mongodb://edugo_admin:edugo_pass@localhost:27017/edugo?authSource=admin"
> db.runCommand({ ping: 1 })
```

### Docker Compose (ejemplo)

```yaml
mongodb:
  image: mongo:7.0
  environment:
    MONGO_INITDB_ROOT_USERNAME: edugo_admin
    MONGO_INITDB_ROOT_PASSWORD: edugo_pass
  ports:
    - "27017:27017"
  networks:
    - edugo-network
```

---

## 🐰 RabbitMQ

### Propósito
Message broker para recibir eventos de otros servicios.

### Conexión

| Parámetro | Variable de Entorno |
|-----------|---------------------|
| URL Completa | `RABBITMQ_URL` ⚠️ |

### Formato URL

```
amqp://[user]:[password]@[host]:[port]/[vhost]
```

### Ejemplo

```bash
RABBITMQ_URL=amqp://edugo_user:edugo_pass@localhost:5672/
```

### Recursos Utilizados

| Recurso | Nombre | Tipo |
|---------|--------|------|
| Exchange | `edugo.materials` | topic |
| Queue | `edugo.material.uploaded` | durable |
| DLQ Exchange | `edugo_dlq` | direct |

### Verificación de Conexión

```bash
# Management UI (si está habilitado)
http://localhost:15672
# User: edugo_user / Pass: edugo_pass

# O usando rabbitmqctl
rabbitmqctl list_queues
```

### Docker Compose (ejemplo)

```yaml
rabbitmq:
  image: rabbitmq:3.12-management-alpine
  environment:
    RABBITMQ_DEFAULT_USER: edugo_user
    RABBITMQ_DEFAULT_PASS: edugo_pass
  ports:
    - "5672:5672"
    - "15672:15672"  # Management UI
  networks:
    - edugo-network
```

---

## 🤖 OpenAI API

### Propósito
Generación de resúmenes y evaluaciones usando GPT-4.

### Conexión

| Parámetro | Valor | Variable de Entorno |
|-----------|-------|---------------------|
| API Key | - | `OPENAI_API_KEY` ⚠️ |
| Model | `gpt-4` | config.yaml |
| Max Tokens | `4000` | config.yaml |
| Temperature | `0.7` | config.yaml |

### Uso

```go
// El worker actualmente SIMULA las llamadas a OpenAI
// TODO: Implementar integración real

// Configuración en config.yaml
nlp:
  provider: "openai"
  model: "gpt-4"
  max_tokens: 4000
  temperature: 0.7
```

### Verificación

```bash
# Probar API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Consideraciones

- **Rate Limits**: OpenAI tiene límites de requests/minuto
- **Costos**: Cada llamada a GPT-4 tiene costo
- **Tokens**: El contenido del PDF puede exceder límites
- **Fallback**: Considerar modelos alternativos (gpt-3.5-turbo)

---

## 📦 AWS S3

### Propósito
Almacenamiento de archivos PDF subidos por los docentes.

### Conexión (TODO - No implementado aún)

| Parámetro | Variable de Entorno |
|-----------|---------------------|
| Region | `AWS_REGION` |
| Access Key | `AWS_ACCESS_KEY_ID` |
| Secret Key | `AWS_SECRET_ACCESS_KEY` |
| Bucket | `S3_BUCKET` |

### Uso Esperado

```go
// El worker recibe s3_key en el evento
// y debe descargar el archivo para procesar

// Ejemplo de s3_key:
// "materials/courses/unit-123/document.pdf"
```

### Dependencias Go

```go
// go.mod
github.com/aws/aws-sdk-go-v2/service/s3 v1.68.0
```

---

## 🔐 api-admin (Servicio Interno)

### Propósito
Validación centralizada de tokens JWT.

### Conexión

| Parámetro | Valor Default | Config |
|-----------|---------------|--------|
| Base URL | `http://api-admin:8081` | `api_admin.base_url` |
| Timeout | `5s` | `api_admin.timeout` |
| Cache TTL | `60s` | `api_admin.cache_ttl` |
| Cache Enabled | `true` | `api_admin.cache_enabled` |

### Endpoints Consumidos

| Endpoint | Método | Propósito |
|----------|--------|-----------|
| `/v1/auth/verify` | POST | Validar token individual |
| `/v1/auth/verify-bulk` | POST | Validar múltiples tokens |

### Request/Response

```json
// POST /v1/auth/verify
// Request
{ "token": "eyJhbG..." }

// Response (éxito)
{
  "valid": true,
  "user_id": "uuid",
  "email": "user@example.com",
  "role": "teacher",
  "expires_at": "2024-01-15T12:00:00Z"
}

// Response (error)
{
  "valid": false,
  "error": "token expired"
}
```

### Features del AuthClient

- **Cache**: Evita llamadas repetidas para mismo token
- **Circuit Breaker**: Protección ante fallos de api-admin
- **Bulk Validation**: Optimizado para procesar batches

### Implementación

**Archivo:** `internal/client/auth_client.go`

```go
type AuthClient struct {
    baseURL        string
    httpClient     *http.Client
    cache          *tokenCache
    circuitBreaker *gobreaker.CircuitBreaker
    config         AuthClientConfig
}

// Métodos principales
func (c *AuthClient) ValidateToken(ctx, token) (*TokenInfo, error)
func (c *AuthClient) ValidateTokensBulk(ctx, tokens) ([]BulkTokenResult, error)
```

---

## 📚 edugo-shared (Librería)

### Propósito
Librería interna compartida entre todos los servicios de EduGo.

### Módulos Utilizados

| Módulo | Import | Uso |
|--------|--------|-----|
| bootstrap | `github.com/EduGoGroup/edugo-shared/bootstrap` | Inicialización de recursos |
| database/postgres | `github.com/EduGoGroup/edugo-shared/database/postgres` | Transacciones SQL |
| logger | `github.com/EduGoGroup/edugo-shared/logger` | Logging estructurado |
| common/errors | `github.com/EduGoGroup/edugo-shared/common/errors` | Tipos de error estándar |
| common/types | `github.com/EduGoGroup/edugo-shared/common/types` | UUID, enums |
| lifecycle | `github.com/EduGoGroup/edugo-shared/lifecycle` | Cleanup de recursos |
| testing | `github.com/EduGoGroup/edugo-shared/testing` | Helpers para tests |

### Versión Actual

```go
// go.mod
github.com/EduGoGroup/edugo-shared/bootstrap v0.9.0
github.com/EduGoGroup/edugo-shared/common v0.7.0
github.com/EduGoGroup/edugo-shared/database/postgres v0.7.0
github.com/EduGoGroup/edugo-shared/lifecycle v0.7.0
github.com/EduGoGroup/edugo-shared/logger v0.7.0
github.com/EduGoGroup/edugo-shared/testing v0.7.0
```

### Acceso a Repositorio Privado

```bash
# Configurar Go para repos privados
export GOPRIVATE=github.com/EduGoGroup/*
export GONOSUMDB=github.com/EduGoGroup/*

# Configurar git con token
git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
```

---

## 📚 edugo-infrastructure (Librería)

### Propósito
Entidades MongoDB compartidas entre servicios.

### Módulos Utilizados

| Módulo | Import | Uso |
|--------|--------|-----|
| mongodb/entities | `github.com/EduGoGroup/edugo-infrastructure/mongodb/entities` | Entidades de MongoDB |

### Versión Actual

```go
// go.mod
github.com/EduGoGroup/edugo-infrastructure/mongodb v0.10.1
```

### Entidades Usadas

- `MaterialSummary` - Resúmenes de materiales
- `MaterialAssessment` - Evaluaciones
- `MaterialEvent` - Eventos procesados

---

## ✅ Checklist de Servicios

Para que el worker funcione correctamente, verificar:

```
□ PostgreSQL
  □ Servidor corriendo en el puerto configurado
  □ Base de datos 'edugo' creada
  □ Usuario con permisos de lectura/escritura
  □ Tabla 'materials' existe

□ MongoDB
  □ Servidor corriendo en el puerto configurado
  □ Base de datos 'edugo' accesible
  □ Usuario autenticado correctamente

□ RabbitMQ
  □ Servidor corriendo en el puerto configurado
  □ Usuario con permisos de crear queues/exchanges
  □ Exchange 'edugo.materials' puede crearse

□ OpenAI (opcional para desarrollo)
  □ API Key válida
  □ Créditos disponibles
  □ Rate limits no excedidos

□ api-admin (opcional)
  □ Servicio corriendo
  □ Endpoint /v1/auth/verify disponible

□ Red Docker
  □ Red 'edugo-network' creada
  □ Worker puede resolver nombres de servicios
```

---

## 🐳 Stack Completo con Docker Compose

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: edugo_user
      POSTGRES_PASSWORD: edugo_pass
      POSTGRES_DB: edugo
    ports:
      - "5432:5432"
    networks:
      - edugo-network

  mongodb:
    image: mongo:7.0
    environment:
      MONGO_INITDB_ROOT_USERNAME: edugo_admin
      MONGO_INITDB_ROOT_PASSWORD: edugo_pass
    ports:
      - "27017:27017"
    networks:
      - edugo-network

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: edugo_user
      RABBITMQ_DEFAULT_PASS: edugo_pass
    ports:
      - "5672:5672"
      - "15672:15672"
    networks:
      - edugo-network

  worker:
    build: .
    environment:
      APP_ENV: local
      POSTGRES_PASSWORD: edugo_pass
      MONGODB_URI: mongodb://edugo_admin:edugo_pass@mongodb:27017/edugo?authSource=admin
      RABBITMQ_URL: amqp://edugo_user:edugo_pass@rabbitmq:5672/
      OPENAI_API_KEY: ${OPENAI_API_KEY}
    depends_on:
      - postgres
      - mongodb
      - rabbitmq
    networks:
      - edugo-network

networks:
  edugo-network:
    driver: bridge
```

---

## 🔧 Troubleshooting

### Error: "Error cargando configuración"
- Verificar que `APP_ENV` está definido
- Verificar que el archivo config-{APP_ENV}.yaml existe

### Error: "POSTGRES_PASSWORD is required"
- Definir variable de entorno `POSTGRES_PASSWORD`

### Error: "Error inicializando infraestructura"
- Verificar conexión a PostgreSQL, MongoDB, RabbitMQ
- Revisar URIs y credenciales

### Error: "Error configurando RabbitMQ"
- Verificar que el usuario tiene permisos para crear exchanges
- Verificar que el vhost existe

### Error: Circuit Breaker Open
- api-admin no está respondiendo
- Esperar timeout del circuit breaker (30s default)
