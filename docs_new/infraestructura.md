# Infraestructura - edugo-worker

Documentacion tecnica de los componentes de infraestructura del worker de EduGo.

---

## 1. Vista General de Componentes

```
                         +-----------------------+
                         |    MetricsServer      |
                         |  :port/metrics        |
                         |  :port/health         |
                         |  :port/health/live    |
                         |  :port/health/ready   |
                         +----------+------------+
                                    |
                    +---------------+---------------+
                    |                               |
            +-------+-------+             +---------+---------+
            | HealthHandler |             | promhttp.Handler  |
            +-------+-------+             +-------------------+
                    |
            +-------+-------+
            |    Checker     |
            +-------+-------+
                    |
       +------------+------------+
       |            |            |
  +----+----+  +----+----+  +---+------+
  |Postgres |  |MongoDB  |  |RabbitMQ  |
  | Check   |  | Check   |  | Check    |
  +---------+  +---------+  +----------+

  +-------------------+     +-------------------+
  |  CircuitBreaker   |     |  MultiRateLimiter |
  |  (NLP, Storage)   |     |  (por event_type) |
  +-------------------+     +-------------------+

  +-------------------+     +-------------------+
  |  NLP Client       |     |  Storage Client   |
  | +-- OpenAI        |     | +-- S3 Client     |
  | +-- Fallback      |     | +-- CB Wrapper    |
  | +-- CB Wrapper    |     +-------------------+
  +-------------------+

  +-------------------+     +-------------------+
  |  PDF Extractor    |     |  EventConsumer    |
  | +-- pdfcpu        |     |  (RabbitMQ)       |
  | +-- TextCleaner   |     +-------------------+
  +-------------------+

  +-------------------+     +-------------------+
  | GracefulShutdown  |     | MongoDB Repos     |
  | (LIFO ordering)   |     | +-- MaterialEvent |
  +-------------------+     | +-- MaterialSum.  |
                            | +-- MaterialAssm. |
                            +-------------------+
```

---

## 2. Health Checks

### Interfaz base

Archivo: `internal/infrastructure/health/health.go`

```go
// Status representa el estado de salud de un componente
type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusUnhealthy Status = "unhealthy"
    StatusDegraded  Status = "degraded"
)

// CheckResult representa el resultado de un health check
type CheckResult struct {
    Status    Status                 `json:"status"`
    Component string                 `json:"component"`
    Message   string                 `json:"message,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthCheck representa un health check individual
type HealthCheck interface {
    Name() string
    Check(ctx context.Context) CheckResult
}
```

El `Checker` gestiona multiples health checks. Permite registrar checks individuales y ejecutarlos todos con `CheckAll()`. Ademas expone `IsHealthy()`, `IsReady()` y `IsLive()` para las probes de Kubernetes.

### Checks implementados

| Check | Archivo | Componente | Interfaz para testing | Que verifica |
|-------|---------|------------|----------------------|--------------|
| PostgreSQL | `postgres_check.go` | `"postgresql"` | `PostgresDB` | `PingContext()` + `Stats()` del pool de conexiones |
| MongoDB | `mongodb_check.go` | `"mongodb"` | `MongoClient` | `Ping()` con `readpref.ReadPref` |
| RabbitMQ | `rabbitmq_check.go` | `"rabbitmq"` | `RabbitMQChannel` | `IsClosed()` del canal AMQP |

#### PostgreSQL Check

Realiza `PingContext()` con timeout configurable. Ademas recopila estadisticas del pool de conexiones (`OpenConnections`, `InUse`, `Idle`, `WaitCount`, `MaxOpenConnections`). Si `OpenConnections >= MaxOpenConnections`, retorna `StatusDegraded`. Incluye `response_time_ms` en metadata.

```go
check := health.NewPostgreSQLCheck(db, 5*time.Second)
// O para testing con mock:
check := health.NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
```

#### MongoDB Check

Ejecuta `Ping()` contra MongoDB con timeout configurable. Incluye `response_time_ms` en metadata.

```go
check := health.NewMongoDBCheck(mongoClient, 5*time.Second)
```

#### RabbitMQ Check

Verifica si el canal AMQP esta cerrado con `IsClosed()`. Es un check sincronico sin timeout (la verificacion es local al objeto channel).

```go
check := health.NewRabbitMQCheck(amqpChannel, 5*time.Second)
```

### Endpoint HTTP

Archivo: `internal/infrastructure/http/health_handler.go`

| Endpoint | Metodo | Descripcion | Status codes |
|----------|--------|-------------|-------------|
| `GET /health` | `Health()` | Health check completo de todos los componentes | 200 healthy / 503 unhealthy |
| `GET /health/live` | `Liveness()` | Liveness probe (siempre retorna alive si la app responde) | 200 alive |
| `GET /health/ready` | `Readiness()` | Readiness probe con logica granular de componentes criticos | 200 ready / 200 ready_degraded / 503 not_ready |

#### Logica de Readiness

Componentes **criticos** (afectan el estado ready): `mongodb`, `postgresql`.
Componentes **opcionales** (no afectan ready): `rabbitmq`.

- Si algun componente critico falla: `not_ready` (503)
- Si solo fallan componentes opcionales: `ready_degraded` (200)
- Si todo esta bien: `ready` (200)

#### Respuesta JSON de ejemplo

```json
{
  "status": "healthy",
  "checks": {
    "postgresql": {
      "status": "healthy",
      "component": "postgresql",
      "message": "PostgreSQL is healthy",
      "timestamp": "2026-03-08T10:00:00Z",
      "metadata": {
        "response_time_ms": 5,
        "open_connections": 5,
        "in_use": 2,
        "idle": 3,
        "wait_count": 0,
        "max_open_connections": 10
      }
    },
    "mongodb": {
      "status": "healthy",
      "component": "mongodb",
      "message": "MongoDB is healthy",
      "timestamp": "2026-03-08T10:00:00Z",
      "metadata": {
        "response_time_ms": 3
      }
    },
    "rabbitmq": {
      "status": "healthy",
      "component": "rabbitmq",
      "message": "RabbitMQ is healthy",
      "timestamp": "2026-03-08T10:00:00Z",
      "metadata": {
        "response_time_ms": 0
      }
    }
  }
}
```

---

## 3. Metricas Prometheus

Archivo: `internal/infrastructure/metrics/metrics.go`

Todas las metricas se registran con `promauto` (registro automatico en el registry global de Prometheus). Se exponen en `GET /metrics` a traves del `MetricsServer`.

### Tabla completa de metricas

| Nombre | Tipo | Labels | Descripcion |
|--------|------|--------|-------------|
| `worker_events_processed_total` | Counter | `event_type`, `status` | Total de eventos procesados por tipo y estado |
| `worker_processing_duration_seconds` | Histogram | `event_type` | Duracion del procesamiento de eventos (buckets: DefBuckets) |
| `worker_events_in_queue` | Gauge | -- | Numero de eventos actualmente en cola |
| `worker_openai_requests_total` | Counter | `status` | Solicitudes a OpenAI API (success, error, rate_limited, timeout) |
| `worker_openai_latency_seconds` | Histogram | -- | Latencia de solicitudes a OpenAI (buckets: 0.1, 0.5, 1, 2, 5, 10, 30, 60) |
| `worker_openai_tokens_used_total` | Counter | -- | Total de tokens consumidos en OpenAI |
| `worker_openai_errors_total` | Counter | `error_type` | Errores de OpenAI (rate_limit, timeout, server_error, invalid_request) |
| `worker_s3_operations_total` | Counter | `operation`, `status` | Operaciones S3 por tipo (download, upload, delete) y estado |
| `worker_s3_operation_duration_seconds` | Histogram | `operation` | Duracion de operaciones S3 (buckets: 0.1, 0.5, 1, 2, 5, 10, 30) |
| `worker_pdf_extraction_total` | Counter | `status` | Extracciones de PDF (success, error) |
| `worker_pdf_extraction_duration_seconds` | Histogram | -- | Duracion de extraccion de texto PDF (buckets: 0.1, 0.5, 1, 2, 5, 10, 30) |
| `worker_pdf_pages_processed` | Histogram | -- | Paginas procesadas por PDF (buckets: 1, 5, 10, 20, 50, 100, 200, 500) |
| `worker_db_operations_total` | Counter | `db_type`, `operation`, `status` | Operaciones de BD por tipo (postgres, mongodb), operacion y estado |
| `worker_db_operation_duration_seconds` | Histogram | `db_type`, `operation` | Duracion de operaciones de BD (buckets: DefBuckets) |
| `worker_circuit_breaker_state` | Gauge | `service` | Estado actual del circuit breaker (0=closed, 1=half-open, 2=open) |
| `worker_circuit_breaker_transitions_total` | Counter | `service`, `from_state`, `to_state` | Transiciones de estado del circuit breaker |
| `worker_rate_limiter_waits_total` | Counter | `event_type` | Veces que el rate limiter causo espera |
| `worker_rate_limiter_wait_duration_seconds` | Histogram | `event_type` | Duracion de esperas por rate limiter (buckets: 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1) |
| `worker_rate_limiter_tokens_available` | Gauge | `event_type` | Tokens disponibles actualmente por event type |
| `worker_rate_limiter_allowed_total` | Counter | `event_type` | Requests permitidos por el rate limiter |

### Funciones helper de registro

```go
// Registro combinado de evento procesado
metrics.RecordEventProcessing(eventType, status, durationSeconds)

// Registro de solicitud a OpenAI
metrics.RecordOpenAIRequest(status, durationSeconds, tokensUsed)

// Registro de operacion S3
metrics.RecordS3Operation(operation, status, durationSeconds)

// Registro de extraccion PDF
metrics.RecordPDFExtraction(status, durationSeconds, pageCount)

// Registro de operacion de BD
metrics.RecordDBOperation(dbType, operation, status, durationSeconds)
// O con bool:
metrics.RecordDatabaseOperation(dbType, operation, durationSeconds, success)

// Circuit breaker
metrics.SetCircuitBreakerState(service, stateInt)
metrics.RecordCircuitBreakerTransition(service, fromState, toState)

// Rate limiter
metrics.RecordRateLimiterWait(eventType, durationSeconds)
metrics.RecordRateLimiterAllowed(eventType)
metrics.UpdateRateLimiterTokens(eventType, tokens)
```

**Nota importante**: `RecordEventProcessed()` esta deprecado. Usar `RecordEventProcessing()` cuando se disponga de la duracion. No llamar ambas funciones para el mismo evento (produciria doble conteo en `worker_events_processed_total`).

---

## 4. Circuit Breaker

Archivo: `internal/infrastructure/circuitbreaker/circuit_breaker.go`

Implementacion del patron circuit breaker con tres estados:

```
  +---------+     MaxFailures      +--------+
  | CLOSED  | ------------------> |  OPEN   |
  | (allow  |                     | (reject |
  |  all)   | <-+                 |  all)   |
  +---------+   |                 +----+----+
                |                      |
                |  SuccessThreshold    |  Timeout expira
                |  met                 |
                |                 +----+------+
                +-----------------| HALF-OPEN |
                                  | (limited  |
                                  |  requests)|
                                  +-----------+
```

### Configuracion

```go
type Config struct {
    Name              string        // Nombre para metricas (ej: "nlp", "storage")
    MaxFailures       uint32        // Fallos antes de abrir (default: 5)
    Timeout           time.Duration // Tiempo en open antes de half-open (default: 60s)
    MaxRequests       uint32        // Requests permitidos en half-open (default: 1)
    SuccessThreshold  uint32        // Exitos en half-open para cerrar (default: 2)
    FailureRateWindow time.Duration // Ventana para calcular tasa de fallos (default: 30s)
}
```

Configuracion por defecto:

```go
cfg := circuitbreaker.DefaultConfig("nlp")
// MaxFailures=5, Timeout=60s, MaxRequests=1, SuccessThreshold=2, FailureRateWindow=30s
```

### Uso

```go
cb := circuitbreaker.New(cfg)

err := cb.Execute(ctx, func(ctx context.Context) error {
    return externalService.Call(ctx)
})

// Errores especificos del circuit breaker:
// - circuitbreaker.ErrCircuitOpen: el circuito esta abierto, request rechazado
// - circuitbreaker.ErrTooManyRequests: demasiados requests en half-open
```

### Transiciones de estado

- **Closed -> Open**: Se alcanza `MaxFailures` fallos consecutivos.
- **Open -> Half-Open**: Transcurre `Timeout` desde la ultima transicion.
- **Half-Open -> Closed**: Se alcanzan `SuccessThreshold` exitos (con maximo `MaxRequests` requests).
- **Half-Open -> Open**: Cualquier fallo en half-open reabre el circuito.
- **Closed (reset de fallos)**: Si en estado closed, transcurre `FailureRateWindow` desde el ultimo fallo, el contador de fallos se resetea a 0.

### Uso con NLP y S3

Los wrappers `ClientWithCircuitBreaker` envuelven los clientes reales con circuit breaker:

```go
// NLP con circuit breaker
nlpCB := circuitbreaker.New(circuitbreaker.DefaultConfig("nlp"))
nlpClient := nlp.NewClientWithCircuitBreaker(openaiClient, nlpCB)

// Storage con circuit breaker
storageCB := circuitbreaker.New(circuitbreaker.DefaultConfig("storage"))
storageClient := storage.NewClientWithCircuitBreaker(s3Client, storageCB)
```

Cada operacion del cliente (Download, Upload, GenerateSummary, etc.) se ejecuta dentro de `cb.Execute()`, registrando automaticamente exitos y fallos en las metricas de Prometheus.

---

## 5. Rate Limiter

Archivos: `internal/infrastructure/ratelimiter/rate_limiter.go`, `multi_rate_limiter.go`

### Token Bucket (RateLimiter individual)

Implementa el algoritmo Token Bucket:

- El bucket inicia lleno (`burstSize` tokens).
- Se recarga a razon de `requestsPerSecond` tokens por segundo.
- Cada `Allow()` consume 1 token. Retorna `false` si no hay tokens.
- `Wait(ctx)` espera hasta que haya un token disponible (polling cada 10ms) o el contexto se cancele.

```go
rl := ratelimiter.New(
    10,  // requestsPerSecond: 10 req/s sostenido
    20,  // burstSize: 20 requests en rafaga
)

if rl.Allow() {
    // Procesar request
}

// O esperar:
err := rl.Wait(ctx)
```

### MultiRateLimiter

Gestiona multiples rate limiters por `event_type`. Permite configurar limites diferentes para cada tipo de evento.

```go
configs := map[string]ratelimiter.Config{
    "material.uploaded":  {RequestsPerSecond: 5, BurstSize: 10},
    "assessment.attempt": {RequestsPerSecond: 15, BurstSize: 30},
}
defaultCfg := &ratelimiter.Config{RequestsPerSecond: 10, BurstSize: 20}

limiter := ratelimiter.NewMulti(configs, defaultCfg)
```

Comportamiento para event types no configurados:

- Si hay `defaultConfig`: crea un nuevo rate limiter con la configuracion por defecto (lazy initialization con double-check locking).
- Si no hay `defaultConfig`: permite el request sin limite.

Metodos disponibles:

| Metodo | Descripcion |
|--------|-------------|
| `Allow(eventType)` | Intenta consumir un token. Retorna true/false. |
| `Wait(ctx, eventType)` | Espera hasta que haya token o se cancele el contexto. |
| `Tokens(eventType)` | Tokens disponibles (-1 si no hay limiter). |
| `Reset(eventType)` | Reinicia un limiter especifico. |
| `ResetAll()` | Reinicia todos los limiters. |
| `EventTypes()` | Lista de event types configurados. |
| `HasLimiter(eventType)` | Verifica si existe un limiter para el event type. |

### Thread safety

Tanto `RateLimiter` como `MultiRateLimiter` son thread-safe:

- `RateLimiter` usa `sync.Mutex` para proteger `tokens` y `lastRefillTime`.
- `MultiRateLimiter` usa `sync.RWMutex` para proteger el mapa de limiters (read lock para lectura, write lock para creacion lazy).

---

## 6. Graceful Shutdown

Archivo: `internal/infrastructure/shutdown/graceful_shutdown.go`

### Funcionamiento

Las funciones de limpieza se ejecutan en orden **LIFO** (ultimo registrado, primero ejecutado). Esto garantiza que las dependencias se cierren en el orden correcto (por ejemplo, cerrar el consumer antes de cerrar la conexion a la base de datos).

### Senales capturadas

- `SIGINT` (Ctrl+C)
- `SIGTERM` (signal de terminacion de proceso, Kubernetes, Docker)

### Timeout

Configurable al crear el `GracefulShutdown`. Default: **30 segundos**. Si alguna tarea excede el timeout, se propaga el error pero se continuan ejecutando las demas tareas.

### API

```go
type ShutdownFunc func(ctx context.Context) error

gs := shutdown.NewGracefulShutdown(30*time.Second, logger)

// Registrar tareas de limpieza (se ejecutan en orden LIFO)
gs.Register("metrics-server", func(ctx context.Context) error {
    return metricsServer.Shutdown(ctx)
})
gs.Register("rabbitmq", func(ctx context.Context) error {
    return channel.Close()
})
gs.Register("mongodb", func(ctx context.Context) error {
    return mongoClient.Disconnect(ctx)
})
gs.Register("postgresql", func(ctx context.Context) error {
    return db.Close()
})

// Esperar senal y ejecutar shutdown
err := gs.WaitForSignal() // Bloquea hasta recibir SIGINT/SIGTERM
```

Orden de ejecucion: `postgresql` -> `mongodb` -> `rabbitmq` -> `metrics-server` (LIFO).

### Logger

Usa una interfaz `Logger` propia (no depende de implementacion especifica):

```go
type Logger interface {
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

---

## 7. PDF Extraction

Archivos: `internal/infrastructure/pdf/interface.go`, `extractor.go`, `cleaner.go`

### Interfaz Extractor

```go
type Extractor interface {
    Extract(ctx context.Context, reader io.Reader) (string, error)
    ExtractWithMetadata(ctx context.Context, reader io.Reader) (*ExtractionResult, error)
}

type ExtractionResult struct {
    Text      string            // Texto extraido y limpiado
    RawText   string            // Texto sin procesar
    PageCount int               // Numero de paginas
    WordCount int               // Numero de palabras
    Metadata  map[string]string // Metadatos del PDF (autor, titulo, etc.)
    HasImages bool              // Si el PDF contiene imagenes
    IsScanned bool              // Si es un PDF escaneado (sin texto)
}
```

### Interfaz Cleaner

```go
type Cleaner interface {
    Clean(text string) string
    RemoveHeaders(text string) string
    NormalizeSpaces(text string) string
}
```

### Implementacion: PDFExtractor (pdfcpu)

Usa la libreria `github.com/pdfcpu/pdfcpu` para extraer texto pagina por pagina.

**Validaciones previas**:
- Tamano maximo: 100 MB (`maxPDFSize`)
- Reader nil o datos vacios: retorna `ErrPDFEmpty`
- Datos que exceden el maximo: retorna `ErrPDFTooLarge`

**Deteccion de PDF escaneado** (3 heuristicas):
1. Menos de 50 palabras en total (`scannedThreshold`) -> escaneado.
2. Menos del 25% de las paginas tienen texto significativo (>= 10 palabras/pagina) -> escaneado.
3. Promedio de palabras por pagina menor a 10 (`minWordsPerPage`) -> escaneado.

Si se detecta como escaneado, retorna `ErrPDFScanned`.

**Errores definidos**:

| Error | Descripcion |
|-------|-------------|
| `ErrPDFTooLarge` | PDF excede 100 MB |
| `ErrPDFEmpty` | PDF vacio o corrupto (0 bytes o 0 paginas) |
| `ErrPDFScanned` | PDF escaneado sin texto extraible (requiere OCR) |
| `ErrPDFCorrupt` | PDF corrupto o invalido (no es un archivo PDF) |

**Respeta cancelacion de contexto**: verifica `ctx.Done()` entre paginas.

### Implementacion: TextCleaner

- `RemoveHeaders()`: elimina lineas que coinciden con patron `^(Pagina|Page|Pag\.?)\s*\d+` (case insensitive). Tambien elimina lineas vacias.
- `NormalizeSpaces()`: colapsa espacios y tabs multiples a uno solo. Reduce 3+ saltos de linea a 2.
- `Clean()`: ejecuta `RemoveHeaders()` + `NormalizeSpaces()` + `TrimSpace()`.

---

## 8. NLP Client

Archivos: `internal/infrastructure/nlp/interface.go`, `openai/client.go`, `fallback/client.go`, `client_with_cb.go`

### Interfaz Client

```go
type Client interface {
    GenerateSummary(ctx context.Context, text string) (*Summary, error)
    GenerateQuiz(ctx context.Context, text string, questionCount int) (*Quiz, error)
    HealthCheck(ctx context.Context) error
}
```

### Modelos de datos

**Summary**: contiene `MainIdeas` ([]string), `KeyConcepts` (map[string]string), `Sections` ([]Section con Title/Content/Points), `Glossary` (map[string]string), `WordCount`, `GeneratedAt`.

**Quiz**: contiene `Questions` ([]Question) y `GeneratedAt`. Cada `Question` tiene: `ID`, `QuestionText`, `QuestionType` ("multiple_choice", "true_false", "open"), `Options`, `CorrectAnswer`, `Explanation`, `Difficulty` ("easy", "medium", "hard"), `Points`.

### OpenAI Client

Archivo: `internal/infrastructure/nlp/openai/client.go`

**Modelos soportados**: `gpt-4-turbo-preview`, `gpt-4`, `gpt-3.5-turbo`, `gpt-3.5-turbo-16k`.

**Limites de texto**: minimo 50 caracteres, maximo 50,000 caracteres. Preguntas: minimo 1, maximo 50.

**Defaults**: maxTokens=2000, temperature=0.7, timeout=60s.

**Estado actual**: La llamada real a la API (`callOpenAIAPI`) es un placeholder preparado para integracion futura. Retorna un error indicando que requiere API key real. La arquitectura esta lista para conectar con `github.com/sashabaranov/go-openai`.

**Prompts**: construidos internamente para generar JSON estructurado (resumenes con `main_ideas`, `key_concepts`, `sections`, `glossary`; quizzes con `questions`).

**Errores especificos**:

| Error | Descripcion |
|-------|-------------|
| `ErrInvalidAPIKey` | API key vacia o invalida |
| `ErrInvalidModel` | Modelo no soportado |
| `ErrEmptyText` | Texto vacio o menor a 50 caracteres |
| `ErrTextTooLong` | Texto mayor a 50,000 caracteres |
| `ErrInvalidQuestionCount` | Numero de preguntas fuera de rango 1-50 |
| `ErrAPITimeout` | Timeout esperando respuesta |
| `ErrAPIRateLimit` | Limite de tasa excedido |
| `ErrAPIQuotaExceeded` | Cuota excedida |
| `ErrAPIUnauthorized` | Autenticacion fallida |

### Fallback Client (SmartClient)

Archivo: `internal/infrastructure/nlp/fallback/client.go`

Genera resumenes y quizzes **sin depender de API externas**. Usa heuristicas basicas:

- **GenerateSummary**: divide texto en oraciones, extrae las 3 primeras como `MainIdeas`, cuenta frecuencia de palabras para `KeyConcepts`, divide el contenido en 3 secciones (Introduccion, Desarrollo, Conclusion). Simula latencia de 500-1500ms.
- **GenerateQuiz**: genera preguntas de opcion multiple basadas en oraciones del texto. Asigna dificultad ciclicamente (easy, medium, hard). Simula latencia de 800-2000ms.
- **HealthCheck**: siempre retorna `nil`.

### Circuit Breaker Wrapper

Archivo: `internal/infrastructure/nlp/client_with_cb.go`

Envuelve cualquier `nlp.Client` con proteccion de circuit breaker. Cada metodo (`GenerateSummary`, `GenerateQuiz`, `HealthCheck`) se ejecuta dentro de `cb.Execute()`.

```go
nlpClient := nlp.NewClientWithCircuitBreaker(openaiClient, cb)
```

---

## 9. S3 Storage

Archivos: `internal/infrastructure/storage/interface.go`, `s3/client.go`, `client_with_cb.go`

### Interfaz Client

```go
type Client interface {
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Upload(ctx context.Context, key string, content io.Reader) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    GetMetadata(ctx context.Context, key string) (*FileMetadata, error)
}

type FileMetadata struct {
    Key          string
    Size         int64
    ContentType  string
    LastModified string
    ETag         string
}
```

### Implementacion S3

Archivo: `internal/infrastructure/storage/s3/client.go`

**SDK**: `github.com/aws/aws-sdk-go-v2`.

**Constructor**:

```go
client, err := s3.NewClient(ctx,
    region,       // AWS region
    bucket,       // Nombre del bucket
    endpoint,     // Endpoint personalizado (MinIO, localstack)
    accessKey,    // AWS access key
    secretKey,    // AWS secret key
    usePathStyle, // Path style para MinIO
    logger,
)
```

**Validaciones en Download**:

1. Validacion de extension: solo `.pdf` permitido.
2. Validacion de metadata (HeadObject antes de descargar):
   - Tamano minimo: 1 KB (`minFileSize`)
   - Tamano maximo: 100 MB (`maxFileSize`)
   - Content-type: solo `application/pdf` (permite `application/octet-stream` o vacio si la extension es correcta)
3. Timeout de descarga: 30 segundos.

**Retry con backoff exponencial**:

- Maximo 3 reintentos (`maxRetries`).
- Backoff base: 100ms, con duplicacion exponencial (`100ms * 2^attempt`).
- Aplica a `Download` y `Upload`.

**Operaciones**:

| Operacion | Retry | Validaciones previas |
|-----------|-------|---------------------|
| `Download` | Si (3 intentos) | Extension, tamano, content-type |
| `Upload` | Si (3 intentos) | Ninguna |
| `Delete` | No | Ninguna |
| `Exists` | No | Ninguna (HeadObject) |
| `GetMetadata` | No | Ninguna (HeadObject) |

### Circuit Breaker Wrapper

Archivo: `internal/infrastructure/storage/client_with_cb.go`

Envuelve el `storage.Client` con circuit breaker. Todas las operaciones pasan por `cb.Execute()`.

```go
storageClient := storage.NewClientWithCircuitBreaker(s3Client, cb)
```

---

## 10. RabbitMQ Consumer

Archivo: `internal/infrastructure/messaging/consumer/event_consumer.go`

### Arquitectura

El `EventConsumer` es un componente ligero que recibe mensajes crudos ([]byte) y los enruta al `processor.Registry` correspondiente.

```go
type EventConsumer struct {
    registry *processor.Registry
    logger   logger.Logger
}

func (c *EventConsumer) RouteEvent(ctx context.Context, body []byte) error {
    return c.registry.Process(ctx, body)
}
```

El `Registry` deserializa el JSON del mensaje, extrae el campo `event_type`, y despacha al `Processor` registrado para ese tipo de evento.

### Flujo de un mensaje

```
RabbitMQ Queue
    |
    v
EventConsumer.RouteEvent(ctx, body)
    |
    v
Registry.Process(ctx, body)
    |
    +-- Deserializar JSON
    +-- Extraer event_type
    +-- Buscar Processor registrado
    +-- Ejecutar Processor.Process(ctx, body)
```

**Nota**: La configuracion de prefetch, ACK/NACK y la conexion real a RabbitMQ se manejan en la capa de bootstrap (fuera de este paquete). El `EventConsumer` solo se encarga del routing.

---

## 11. MongoDB Repositories

Archivo: `internal/infrastructure/persistence/mongodb/repository/`

### Collections usadas

| Collection | Repository | Entidad |
|------------|-----------|---------|
| `material_event` | `MongoMaterialEventRepository` | `entities.MaterialEvent` |
| `material_summary` | `MongoMaterialSummaryRepository` | `entities.MaterialSummary` |
| `material_assessment_worker` | `MongoMaterialAssessmentRepository` | `entities.MaterialAssessment` |

### MongoMaterialEventRepository

Operaciones sobre la collection `material_event`:

| Operacion | Descripcion |
|-----------|-------------|
| `Create(ctx, event)` | Crea un nuevo evento. Valida con `EventStateMachine`. |
| `FindByID(ctx, id)` | Busca por ObjectID. |
| `Update(ctx, event)` | Actualiza un evento existente. Valida con `EventStateMachine`. |
| `FindByMaterialID(ctx, materialID, limit)` | Busca por material_id (orden: createdAt DESC). |
| `FindByEventType(ctx, eventType, limit)` | Busca por event_type (orden: createdAt DESC). |
| `FindByStatus(ctx, status, limit)` | Busca por status (orden: createdAt DESC). |
| `FindFailedEvents(ctx, maxRetries, limit)` | Busca eventos fallidos con retry_count < maxRetries. |
| `FindPendingEvents(ctx, limit)` | Busca eventos pendientes (FIFO: createdAt ASC). |
| `FindRecent(ctx, limit)` | Busca eventos recientes (createdAt DESC). |
| `CountByStatus(ctx, status)` | Cuenta eventos por status. |
| `CountByEventType(ctx, eventType)` | Cuenta eventos por tipo. |
| `GetEventStatistics(ctx)` | Agregacion: cuenta por status (pipeline $group). |
| `DeleteOldEvents(ctx, olderThan)` | Elimina eventos anteriores a la fecha dada. |

### MongoMaterialSummaryRepository

Operaciones sobre la collection `material_summary`:

| Operacion | Descripcion |
|-----------|-------------|
| `Create(ctx, summary)` | Crea un resumen. Valida con `SummaryValidator`. Detecta duplicados. |
| `FindByMaterialID(ctx, materialID)` | Busca por material_id (FindOne). |
| `FindByID(ctx, id)` | Busca por ObjectID. |
| `Update(ctx, summary)` | Actualiza un resumen. Valida con `SummaryValidator`. |
| `Delete(ctx, materialID)` | Elimina por material_id. |
| `FindByLanguage(ctx, language, limit)` | Busca por idioma. |
| `FindRecent(ctx, limit)` | Busca resumenes recientes. |
| `CountByLanguage(ctx, language)` | Cuenta por idioma. |
| `Exists(ctx, materialID)` | Verifica existencia por material_id. |

### MongoMaterialAssessmentRepository

Operaciones sobre la collection `material_assessment_worker`:

| Operacion | Descripcion |
|-----------|-------------|
| `Create(ctx, assessment)` | Crea una evaluacion. Valida con `AssessmentValidator`. |
| `FindByMaterialID(ctx, materialID)` | Busca por material_id (FindOne). |
| `FindByID(ctx, id)` | Busca por ObjectID. |
| `Update(ctx, assessment)` | Actualiza una evaluacion. Valida con `AssessmentValidator`. |
| `Delete(ctx, materialID)` | Elimina por material_id. |
| `FindByDifficulty(ctx, difficulty, limit)` | Busca por dificultad de preguntas (nested query). |
| `FindByTotalQuestions(ctx, min, max, limit)` | Busca por rango de total_questions. |
| `FindRecent(ctx, limit)` | Busca evaluaciones recientes. |
| `CountByTotalPoints(ctx, min, max)` | Cuenta por rango de total_points. |
| `Exists(ctx, materialID)` | Verifica existencia por material_id. |
| `GetAverageQuestionCount(ctx)` | Agregacion: promedio de total_questions ($group + $avg). |

### Patrones comunes

- Todos los repositorios setean `CreatedAt` y `UpdatedAt` automaticamente.
- Validacion de entidad antes de `Create` y `Update` (via `StateMachine` o `Validator`).
- Manejo de `mongo.ErrNoDocuments` -> error especifico del repositorio (ej: `ErrMaterialEventNotFound`).
- Manejo de `mongo.IsDuplicateKeyError` -> error `AlreadyExists` (en summary y assessment).
- Cursores siempre se cierran con `defer cursor.Close(ctx)`.

---

## 12. Como agregar un nuevo componente de infraestructura

### Paso 1: Definir la interfaz

Crear un archivo `interface.go` en el paquete correspondiente (`internal/infrastructure/{componente}/`):

```go
package mycomponent

import "context"

type Client interface {
    DoSomething(ctx context.Context, input string) (string, error)
}
```

### Paso 2: Implementar

Crear la implementacion concreta:

```go
package myimpl

type RealClient struct {
    // dependencias
}

func NewClient(/* config */) (*RealClient, error) {
    // ...
}

func (c *RealClient) DoSomething(ctx context.Context, input string) (string, error) {
    // implementacion real
}

// Verificar que implementa la interfaz en tiempo de compilacion
var _ mycomponent.Client = (*RealClient)(nil)
```

### Paso 3: Agregar circuit breaker wrapper (si es servicio externo)

```go
package mycomponent

type ClientWithCircuitBreaker struct {
    client         Client
    circuitBreaker *circuitbreaker.CircuitBreaker
}

func NewClientWithCircuitBreaker(client Client, cb *circuitbreaker.CircuitBreaker) Client {
    return &ClientWithCircuitBreaker{client: client, circuitBreaker: cb}
}

func (c *ClientWithCircuitBreaker) DoSomething(ctx context.Context, input string) (string, error) {
    var result string
    var err error
    executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
        result, err = c.client.DoSomething(ctx, input)
        return err
    })
    if executeErr != nil {
        return "", executeErr
    }
    return result, nil
}
```

### Paso 4: Agregar metricas

En `internal/infrastructure/metrics/metrics.go`, agregar las metricas:

```go
var (
    MyComponentOperationsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "worker_mycomponent_operations_total",
            Help: "Total number of mycomponent operations",
        },
        []string{"operation", "status"},
    )
)

func RecordMyComponentOperation(operation, status string, duration float64) {
    MyComponentOperationsTotal.WithLabelValues(operation, status).Inc()
}
```

### Paso 5: Configurar en `.mockery.yaml`

Agregar la interfaz para generacion automatica de mocks:

```yaml
packages:
  github.com/EduGoGroup/edugo-worker/internal/infrastructure/mycomponent:
    interfaces:
      Client:
```

Ejecutar `mockery` para generar los mocks.

### Paso 6: Agregar health check (si aplica)

```go
type MyComponentCheck struct {
    client  MyComponentClient
    timeout time.Duration
}

func (c *MyComponentCheck) Name() string { return "mycomponent" }
func (c *MyComponentCheck) Check(ctx context.Context) health.CheckResult {
    // implementar verificacion de salud
}
```

Registrar en el `Checker`:

```go
checker.Register(NewMyComponentCheck(client, 5*time.Second))
```

### Paso 7: Registrar en graceful shutdown

```go
gs.Register("mycomponent", func(ctx context.Context) error {
    return myClient.Close(ctx)
})
```
