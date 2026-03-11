# Arquitectura — EduGo Worker

> Documentación generada desde análisis directo del código fuente (marzo 2026).

---

## 1. Descripción del Servicio

**EduGo Worker** es un servicio de procesamiento de eventos que consume mensajes de RabbitMQ y ejecuta tareas asíncronas para la plataforma educativa EduGo. Su principal responsabilidad es procesar materiales educativos (PDFs), generando resúmenes y quizzes automáticos mediante IA.

**Responsabilidades:**
- Procesar materiales subidos (extracción de texto PDF + generación de resumen/quiz con OpenAI)
- Limpiar datos cuando se elimina un material
- Reprocesar materiales existentes
- Procesar intentos de evaluación (stub)
- Procesar inscripciones de estudiantes (stub)

---

## 2. Stack Tecnológico

| Tecnología | Versión | Uso |
|-----------|---------|-----|
| Go | 1.25.3 | Lenguaje principal |
| RabbitMQ (amqp091-go) | v1.10.0 | Mensajería — consumo de eventos |
| PostgreSQL (lib/pq + GORM) | v1.11.2 / v1.31.1 | Metadatos de materiales |
| MongoDB (mongo-driver) | v2.5.0 | Contenido generado (resúmenes, quizzes) |
| AWS SDK v2 (S3) | v1.41.2 | Almacenamiento de archivos |
| pdfcpu | v0.11.1 | Extracción de texto de PDFs |
| Prometheus | v1.23.2 | Métricas y observabilidad |
| Viper | v1.21.0 | Configuración (YAML + env vars) |
| gobreaker | v1.0.0 | Circuit breaker |
| testify | v1.11.1 | Testing |
| testcontainers | v0.40.0 | Tests de integración |
| edugo-shared/bootstrap | v0.51.0 | Factories para DB/MQ connections |
| edugo-shared/logger | v0.50.1 | Logger estructurado |
| edugo-shared/lifecycle | v0.50.3 | Lifecycle management |

---

## 3. Estructura de Carpetas

```
edugo-worker/
├── cmd/
│   └── main.go                         # Punto de entrada
├── internal/
│   ├── application/
│   │   ├── dto/
│   │   │   └── event_dto.go            # Definición de eventos
│   │   └── processor/                  # Lógica de procesamiento
│   │       ├── processor.go            # Interfaz Processor
│   │       ├── registry.go             # ProcessorRegistry (routing)
│   │       ├── retry.go                # Retry con backoff exponencial
│   │       ├── material_uploaded_processor.go    # ✅ Completo
│   │       ├── material_deleted_processor.go     # ✅ Completo
│   │       ├── material_reprocess_processor.go   # ✅ Wrapper
│   │       ├── assessment_attempt_processor.go   # ⚠️ Stub
│   │       └── student_enrolled_processor.go     # ⚠️ Stub
│   ├── bootstrap/
│   │   └── resource_builder.go         # Patrón Builder fluido
│   ├── client/
│   │   └── auth_client.go              # Cliente HTTP para API Admin
│   ├── config/
│   │   └── config.go                   # Structs de configuración
│   ├── container/
│   │   └── container.go                # DI container
│   ├── domain/
│   │   ├── constants/                  # Constantes de eventos
│   │   ├── repository/                 # Interfaces de repositorio
│   │   ├── service/                    # Servicios de dominio
│   │   └── valueobject/                # Value objects (MaterialID)
│   └── infrastructure/
│       ├── circuitbreaker/             # Circuit breaker
│       ├── health/                     # Health checks (PG, Mongo, RMQ)
│       ├── http/                       # Servidor métricas + health endpoint
│       ├── messaging/consumer/         # RabbitMQ consumer
│       ├── metrics/                    # Definiciones Prometheus
│       ├── nlp/                        # Cliente NLP (OpenAI + fallback)
│       ├── pdf/                        # Extracción PDF (pdfcpu)
│       ├── persistence/mongodb/        # Repositorios MongoDB
│       ├── ratelimiter/                # Token bucket rate limiter
│       ├── shutdown/                   # Graceful shutdown LIFO
│       └── storage/                    # Cliente S3 (AWS SDK v2)
├── config/                             # Archivos YAML de configuración
│   ├── config.yaml                     # Base
│   ├── config-local.yaml
│   ├── config-dev.yaml
│   ├── config-qa.yaml
│   └── config-prod.yaml
├── test/integration/                   # Tests de integración
├── Dockerfile                          # Imagen de runtime (Alpine)
├── docker-compose.yml                  # Worker + dependencias
├── Makefile                            # Targets de build/test
├── go.mod / go.sum                     # Dependencias Go
└── README.md                           # Documentación raíz
```

---

## 4. Diagrama de Capas

```
┌─────────────────────────────────────────────┐
│                  cmd/main.go                 │  ← Punto de entrada
│    Config → ResourceBuilder → Consumer loop  │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│           application/processor/             │  ← Lógica de negocio
│  ProcessorRegistry → Processor → EventDTO   │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│              domain/                         │  ← Interfaces y reglas
│  Repository interfaces, ValueObjects,        │
│  Services, Constants                         │
└─────────────────┬───────────────────────────┘
                  │
┌─────────────────▼───────────────────────────┐
│           infrastructure/                    │  ← Implementaciones
│  MongoDB repos, S3 client, NLP client,      │
│  PDF extractor, Health, Metrics, RateLimiter │
└─────────────────────────────────────────────┘
```

**Regla clave:** Las capas superiores dependen de las inferiores a través de interfaces definidas en `domain/`. La capa de `infrastructure/` implementa esas interfaces.

---

## 5. Patrón ResourceBuilder

El bootstrap del worker usa un **Builder fluido** que inicializa recursos en orden con validación de dependencias y cleanup automático en orden LIFO.

**Archivo:** `internal/bootstrap/resource_builder.go`

### Uso en main.go

```go
resources, cleanup, err := bootstrap.NewResourceBuilder(ctx, cfg).
    WithLogger().          // 1. Logger (requerido primero)
    WithPostgreSQL().      // 2. Conexión PostgreSQL
    WithMongoDB().         // 3. Conexión MongoDB
    WithRabbitMQ().        // 4. Conexión RabbitMQ + channel
    WithAuthClient().      // 5. Cliente HTTP para API Admin
    WithInfrastructure().  // 6. S3, PDF extractor, NLP client
    WithProcessors().      // 7. Registra los 5 processors
    WithHealthChecks().    // 8. Health checks para PG/Mongo/RMQ
    WithMetricsServer().   // 9. Servidor Prometheus en puerto 9090
    Build()
```

### Struct Resources (resultado del Build)

```go
type Resources struct {
    Logger            logger.Logger
    PostgreSQL        *sql.DB
    MongoDB           *mongo.Database
    RabbitMQChannel   *amqp.Channel
    AuthClient        *client.AuthClient
    LifecycleManager  *lifecycle.Manager
    ProcessorRegistry *processor.Registry
    MetricsServer     *httpInfra.MetricsServer
    HealthChecker     *health.Checker
}
```

### Características del Builder

| Característica | Descripción |
|---------------|-------------|
| **Validación de dependencias** | Cada `With*()` verifica que sus dependencias existan (ej: `WithPostgreSQL()` requiere `WithLogger()`) |
| **Early error return** | Si un paso falla, los siguientes son no-ops (chequean `b.err != nil`) |
| **LIFO Cleanup** | Los cleanups se insertan al inicio del slice: `append([]func(){fn}, b.cleanupFuncs...)` |
| **Partial cleanup** | Si `Build()` falla, ejecuta cleanup de recursos parcialmente inicializados |
| **Factories compartidas** | Usa `edugo-shared/bootstrap` para crear conexiones (PostgreSQL, MongoDB, RabbitMQ) |

### Orden de Cleanup (LIFO)

```
Inicialización:                 Cleanup (inverso):
1. Logger                       9. MetricsServer.Shutdown()
2. PostgreSQL                   8. HealthChecker (no cleanup)
3. MongoDB                      7. Processors (no cleanup)
4. RabbitMQ                     6. Infrastructure (no cleanup)
5. AuthClient                   5. AuthClient (no cleanup)
6. Infrastructure (S3/PDF/NLP)  4. RabbitMQ Channel+Connection.Close()
7. Processors                   3. MongoDB.Disconnect()
8. HealthChecks                 2. PostgreSQL.Close()
9. MetricsServer                1. Logger.Sync()
```

---

## 6. Inyección de Dependencias

El worker usa **inyección explícita sin framework**. No hay Koin ni Wire — las dependencias se pasan como parámetros en los constructores.

### Ejemplo: MaterialUploadedProcessor

```go
materialUploadedProc := processor.NewMaterialUploadedProcessor(
    processor.MaterialUploadedProcessorConfig{
        DB:            b.sqlDB,          // *sql.DB
        MongoDB:       b.mongodb,        // *mongo.Database
        Logger:        b.logger,         // logger.Logger
        StorageClient: b.storageClient,  // storage.Client (interfaz)
        PDFExtractor:  b.pdfExtractor,   // pdf.Extractor (interfaz)
        NLPClient:     b.nlpClient,      // nlp.Client (interfaz)
        AIModel:       aiModel,          // string
    },
)
```

### Interfaces principales (para testing)

| Interfaz | Paquete | Implementaciones |
|----------|---------|------------------|
| `storage.Client` | `infrastructure/storage` | S3 client, mock |
| `pdf.Extractor` | `infrastructure/pdf` | pdfcpu extractor, mock |
| `nlp.Client` | `infrastructure/nlp` | OpenAI client, fallback client, mock |
| `health.Check` | `infrastructure/health` | PostgreSQL, MongoDB, RabbitMQ checks |

---

## 7. Dependencias Externas

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   RabbitMQ       │     │   PostgreSQL      │     │    MongoDB       │
│   (eventos)      │     │   (metadatos)     │     │   (contenido)    │
│                  │     │                   │     │                  │
│ Exchange: topic  │     │ Tabla: materials  │     │ Collections:     │
│ Queue: durable   │     │ Estado: pending/  │     │ material_summary │
│ Prefetch: 5      │     │ processing/done   │     │ material_assess. │
└────────┬─────────┘     └────────┬──────────┘     └────────┬─────────┘
         │                        │                          │
         └────────────┬───────────┴──────────────────────────┘
                      │
              ┌───────▼───────┐
              │  EduGo Worker │
              └───────┬───────┘
                      │
         ┌────────────┼────────────┐
         │            │            │
┌────────▼────┐ ┌─────▼─────┐ ┌───▼──────────┐
│   AWS S3    │ │  OpenAI   │ │  API Admin   │
│ (archivos)  │ │  (IA/NLP) │ │ (auth cache) │
│             │ │           │ │              │
│ Download    │ │ Summary   │ │ GetUser      │
│ Upload      │ │ Quiz gen  │ │ GetRoles     │
│ Delete      │ │ Fallback  │ │ Permissions  │
└─────────────┘ └───────────┘ └──────────────┘
```

| Servicio | Propósito | Config Key |
|----------|-----------|------------|
| **PostgreSQL** | Metadatos de materiales (estado, timestamps) | `database.postgres.*` |
| **MongoDB** | Contenido generado: resúmenes y assessments | `database.mongodb.*` |
| **RabbitMQ** | Cola de eventos para procesamiento asíncrono | `messaging.rabbitmq.*` |
| **AWS S3** | Almacenamiento de archivos PDF | `storage.s3.*` |
| **OpenAI** | Generación de resúmenes y quizzes con IA | `nlp.openai.*` |
| **API Admin** | Consulta de usuarios y permisos (con cache) | `api_admin.*` |

---

## 8. Flujo de Arranque

```
main()
│
├─ 1. config.Load()                    # Viper: YAML + env vars
│
├─ 2. ResourceBuilder.Build()          # Inicializa todo en orden
│     ├─ Logger (logrus via shared)
│     ├─ PostgreSQL (GORM factory)
│     ├─ MongoDB (shared factory)
│     ├─ RabbitMQ (shared factory)
│     ├─ AuthClient (HTTP client)
│     ├─ Infrastructure (S3, PDF, NLP)
│     ├─ Processors (5 registrados)
│     ├─ HealthChecks (PG, Mongo, RMQ)
│     └─ MetricsServer (Prometheus)
│
├─ 3. setupRabbitMQ()                  # Exchange + queue + binding
│     ├─ ExchangeDeclare("edugo.materials", "topic")
│     ├─ QueueDeclare("edugo.material.uploaded", durable)
│     │   └─ x-max-priority: 10
│     │   └─ x-dead-letter-exchange: "edugo_dlq"
│     └─ QueueBind("material.uploaded")
│
├─ 4. MultiRateLimiter                 # Token bucket por event_type
│
├─ 5. Channel.Consume()               # auto-ack: false, prefetch via config
│
├─ 6. goroutine: for msg := range msgs
│     ├─ Rate limit wait
│     ├─ ProcessorRegistry.Process(payload)
│     ├─ ACK (éxito) / NACK+requeue (error)
│     └─ Métricas Prometheus
│
├─ 7. GracefulShutdown.Register()      # LIFO: consumer → metrics → cleanup
│
└─ 8. WaitForSignal()                  # SIGINT/SIGTERM → shutdown ordenado
```

---

## 9. Puertos y Endpoints

| Puerto | Endpoint | Descripción |
|--------|----------|-------------|
| 9090 | `GET /metrics` | Métricas Prometheus |
| 9090 | `GET /health` | Health check agregado (PG + Mongo + RMQ) |
| — | RabbitMQ consumer | No expone puerto, consume de la cola |

**Health response:**
```json
{
  "status": "healthy",
  "checks": [
    {"component": "postgres", "status": "healthy"},
    {"component": "mongodb", "status": "healthy"},
    {"component": "rabbitmq", "status": "healthy"}
  ]
}
```

---

*Generado: marzo 2026 | Basado en análisis directo del código fuente*
