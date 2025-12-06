# Roadmap de Mejoras Técnicas - EduGo Worker

> **Propósito:** Plan de mejoras técnicas a corto, mediano y largo plazo.  
> **Última actualización:** Diciembre 2024

---

## 🎯 Visión

Transformar el worker de un **prototipo no funcional** a un **servicio de producción robusto** que procese materiales educativos de forma confiable, escalable y observable.

---

## 📅 Timeline

```
Q4 2024                    Q1 2025                    Q2 2025
┌─────────────────────────┬─────────────────────────┬─────────────────────────┐
│      FASE 1             │       FASE 2            │       FASE 3            │
│   Funcionalidad Base    │   Producción Ready      │   Optimización          │
│                         │                         │                         │
│ • Routing processors    │ • Métricas Prometheus   │ • Horizontal scaling    │
│ • Integración OpenAI    │ • Alertas               │ • Caching Redis         │
│ • Extracción PDF        │ • Circuit breakers      │ • Async processing      │
│ • Tests unitarios       │ • Rate limiting         │ • Multi-tenant          │
│                         │ • Health checks         │ • A/B testing prompts   │
└─────────────────────────┴─────────────────────────┴─────────────────────────┘
        ▲                          ▲                          ▲
        │                          │                          │
    Dic 2024                   Mar 2025                   Jun 2025
```

---

## 🚀 Fase 1: Funcionalidad Base (Q4 2024)

### Objetivos
- [ ] Worker procesa eventos realmente
- [ ] Integración funcional con OpenAI
- [ ] Extracción de texto de PDFs
- [ ] Tests con >70% cobertura

### Épicas

#### EP-001: Routing de Eventos
**Prioridad:** 🔴 Crítica  
**Duración:** 1 sprint (2 semanas)

| Tarea | Descripción | Estimación |
|-------|-------------|------------|
| Crear `ProcessorRegistry` | Patrón registry para processors | 4h |
| Modificar `processMessage()` | Conectar con registry | 4h |
| Tests de routing | Verificar todos los event types | 4h |
| Documentación | Actualizar docs con nuevo flujo | 2h |

**Criterio de Aceptación:**
- Todos los event_type tienen processor asociado
- Eventos desconocidos se loguean pero no fallan
- Tests pasan con >90% cobertura en routing

---

#### EP-002: Integración OpenAI
**Prioridad:** 🔴 Crítica  
**Duración:** 2 sprints (4 semanas)

| Tarea | Descripción | Estimación |
|-------|-------------|------------|
| Cliente OpenAI | `internal/infrastructure/nlp/openai/client.go` | 8h |
| Prompt engineering | Diseñar prompts para resumen y quiz | 8h |
| Parseo de respuestas | Extraer estructura de respuestas GPT | 6h |
| Manejo de errores | Rate limits, timeouts, retries | 6h |
| Tests con mocks | Tests sin llamadas reales a OpenAI | 8h |
| Integración E2E | Test completo con OpenAI real | 4h |

**Estructura de Archivos:**
```
internal/infrastructure/nlp/
├── openai/
│   ├── client.go           # Cliente HTTP para OpenAI
│   ├── client_test.go
│   ├── prompts.go          # Templates de prompts
│   ├── prompts_test.go
│   ├── parser.go           # Parseo de respuestas
│   └── parser_test.go
├── interface.go            # Interfaz común NLP
└── mock/
    └── mock_client.go      # Mock para tests
```

**Prompts a Desarrollar:**
```go
// prompts.go
const SummaryPrompt = `
Analiza el siguiente texto educativo y genera un resumen estructurado en JSON:

{
  "main_ideas": ["idea1", "idea2", "idea3"],
  "key_concepts": {
    "concepto": "definición"
  },
  "sections": [
    {"title": "título", "summary": "resumen de sección"}
  ],
  "glossary": {
    "término": "explicación simple"
  }
}

TEXTO:
{{.Content}}

IDIOMA DE SALIDA: {{.Language}}
`

const QuizPrompt = `
Genera un quiz educativo basado en el siguiente contenido.
Incluye preguntas de diferentes dificultades y tipos.

Formato JSON requerido:
{
  "questions": [
    {
      "id": "q1",
      "question_text": "pregunta",
      "question_type": "multiple_choice|true_false|open",
      "difficulty": "easy|medium|hard",
      "options": [{"id": "a", "text": "opción"}],
      "correct_answer": "a",
      "explanation": "por qué es correcta"
    }
  ]
}

CONTENIDO:
{{.Content}}

RESUMEN:
{{.Summary}}

Genera {{.QuestionCount}} preguntas.
`
```

---

#### EP-003: Extracción de PDF
**Prioridad:** 🔴 Crítica  
**Duración:** 1.5 sprints (3 semanas)

| Tarea | Descripción | Estimación |
|-------|-------------|------------|
| Cliente S3 | Descargar archivos de S3 | 6h |
| Extractor PDF | Usar pdfcpu o similar | 12h |
| Limpieza de texto | Normalizar texto extraído | 4h |
| Manejo de errores | PDFs corruptos, sin texto | 4h |
| Tests | Diferentes tipos de PDF | 8h |

**Estructura:**
```
internal/infrastructure/
├── storage/
│   ├── s3/
│   │   ├── client.go       # Cliente AWS S3
│   │   ├── client_test.go
│   │   └── downloader.go   # Descarga con retry
│   └── interface.go
└── pdf/
    ├── extractor.go        # Extracción de texto
    ├── extractor_test.go
    ├── cleaner.go          # Limpieza de texto
    └── testdata/           # PDFs de prueba
        ├── simple.pdf
        ├── complex.pdf
        └── scanned.pdf     # PDF sin texto (OCR needed)
```

---

#### EP-004: Tests Unitarios
**Prioridad:** 🟡 Media  
**Duración:** 1 sprint (2 semanas)

| Tarea | Descripción | Estimación |
|-------|-------------|------------|
| Mocks de repositories | Test doubles para MongoDB | 6h |
| Mocks de servicios externos | OpenAI, S3 | 6h |
| Tests de processors | Unit tests para cada processor | 12h |
| Tests de domain services | Validators, state machine | 6h |
| CI/CD para tests | GitHub Actions | 4h |

**Meta de Cobertura:**
- `internal/application/processor/`: >80%
- `internal/domain/service/`: >90%
- `internal/infrastructure/persistence/`: >70%
- Global: >70%

---

## 🏭 Fase 2: Producción Ready (Q1 2025)

### Objetivos
- [ ] Observabilidad completa
- [ ] Resiliencia ante fallos
- [ ] Documentación operacional
- [ ] Procesos de deploy automatizados

### Épicas

#### EP-005: Observabilidad
**Duración:** 2 sprints

| Componente | Descripción |
|------------|-------------|
| **Métricas Prometheus** | `worker_events_processed_total`, `worker_processing_duration_seconds`, `worker_errors_total` |
| **Logging estructurado** | JSON con correlation IDs |
| **Tracing** | OpenTelemetry para requests distribuidos |
| **Dashboards** | Grafana dashboards predefinidos |
| **Alertas** | AlertManager rules para errores y latencia |

**Métricas a Implementar:**
```go
// internal/infrastructure/metrics/prometheus.go

var (
    EventsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "worker_events_processed_total",
            Help: "Total de eventos procesados",
        },
        []string{"event_type", "status"},
    )
    
    ProcessingDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "worker_processing_duration_seconds",
            Help:    "Duración del procesamiento de eventos",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
        },
        []string{"event_type"},
    )
    
    OpenAILatency = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "worker_openai_latency_seconds",
            Help:    "Latencia de llamadas a OpenAI",
            Buckets: []float64{1, 2, 5, 10, 20, 30, 60},
        },
    )
    
    OpenAITokensUsed = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "worker_openai_tokens_total",
            Help: "Total de tokens consumidos en OpenAI",
        },
    )
)
```

---

#### EP-006: Resiliencia
**Duración:** 1.5 sprints

| Componente | Descripción |
|------------|-------------|
| **Circuit Breakers** | Para OpenAI, MongoDB, PostgreSQL |
| **Rate Limiting** | Límites de requests a OpenAI |
| **Retry con Backoff** | Exponential backoff para fallos transitorios |
| **Dead Letter Queue** | Manejo de mensajes que fallan |
| **Graceful Shutdown** | Completar procesamiento antes de cerrar |

**Configuración de Circuit Breaker:**
```go
// internal/infrastructure/resilience/circuit_breaker.go

type CircuitBreakerConfig struct {
    Name              string
    MaxRequests       uint32        // Max requests en half-open
    Interval          time.Duration // Intervalo de reset
    Timeout           time.Duration // Tiempo antes de half-open
    FailureThreshold  float64       // % de fallos para abrir
    MinRequests       int           // Min requests antes de evaluar
}

var DefaultOpenAIBreaker = CircuitBreakerConfig{
    Name:             "openai",
    MaxRequests:      3,
    Interval:         10 * time.Second,
    Timeout:          30 * time.Second,
    FailureThreshold: 0.5,
    MinRequests:      5,
}
```

---

#### EP-007: Health Checks
**Duración:** 0.5 sprint

| Endpoint | Descripción |
|----------|-------------|
| `/health` | Estado general del worker |
| `/health/live` | Kubernetes liveness probe |
| `/health/ready` | Kubernetes readiness probe |
| `/metrics` | Métricas Prometheus |

```go
// internal/infrastructure/http/health.go

type HealthStatus struct {
    Status     string            `json:"status"`
    Timestamp  time.Time         `json:"timestamp"`
    Components map[string]string `json:"components"`
}

func (h *HealthHandler) Check() *HealthStatus {
    status := &HealthStatus{
        Timestamp:  time.Now(),
        Components: make(map[string]string),
    }
    
    // Check PostgreSQL
    if err := h.db.PingContext(ctx); err != nil {
        status.Components["postgresql"] = "unhealthy"
    } else {
        status.Components["postgresql"] = "healthy"
    }
    
    // Check MongoDB
    if err := h.mongo.Ping(ctx, nil); err != nil {
        status.Components["mongodb"] = "unhealthy"
    } else {
        status.Components["mongodb"] = "healthy"
    }
    
    // Check RabbitMQ
    // ... similar
    
    // Determinar status global
    for _, s := range status.Components {
        if s == "unhealthy" {
            status.Status = "unhealthy"
            return status
        }
    }
    status.Status = "healthy"
    return status
}
```

---

## 🚀 Fase 3: Optimización (Q2 2025)

### Objetivos
- [ ] Escalabilidad horizontal
- [ ] Optimización de costos OpenAI
- [ ] Procesamiento paralelo
- [ ] Multi-tenancy

### Épicas

#### EP-008: Horizontal Scaling
| Componente | Descripción |
|------------|-------------|
| **Kubernetes HPA** | Auto-scaling basado en queue depth |
| **Consumer Groups** | Múltiples workers consumiendo |
| **Idempotencia** | Procesamiento seguro ante duplicados |

#### EP-009: Caching
| Componente | Descripción |
|------------|-------------|
| **Redis Cache** | Cache de resúmenes generados |
| **Deduplicación** | Evitar reprocesar materiales |
| **Cache de prompts** | Optimizar tokens usados |

#### EP-010: Procesamiento Inteligente
| Componente | Descripción |
|------------|-------------|
| **Batch Processing** | Agrupar documentos similares |
| **Priority Queue** | Procesar primero lo importante |
| **A/B Testing** | Probar diferentes prompts |

---

## 📊 KPIs de Éxito

### Fase 1
| KPI | Meta | Medición |
|-----|------|----------|
| Eventos procesados | >95% | Prometheus |
| Cobertura tests | >70% | CI/CD |
| Tiempo promedio procesamiento | <60s | Prometheus |

### Fase 2
| KPI | Meta | Medición |
|-----|------|----------|
| Uptime | >99.5% | Monitoring |
| Latencia P99 | <120s | Prometheus |
| Error rate | <1% | Prometheus |

### Fase 3
| KPI | Meta | Medición |
|-----|------|----------|
| Costo por material | -30% | AWS Cost Explorer |
| Throughput | 100 mat/hora | Prometheus |
| Latencia P95 | <30s | Prometheus |

---

## 🔄 Proceso de Revisión

1. **Semanal:** Revisión de progreso en épicas activas
2. **Quincenal:** Sprint review con stakeholders
3. **Mensual:** Revisión de roadmap y re-priorización
4. **Trimestral:** Retrospectiva de fase y planificación siguiente

---

## 📝 Changelog del Roadmap

| Fecha | Cambio | Razón |
|-------|--------|-------|
| 2024-12 | Documento inicial | Primera versión |
