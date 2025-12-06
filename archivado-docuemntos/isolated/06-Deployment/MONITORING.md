# Monitoreo - Worker

## Métricas Clave

| Métrica | Objetivo | Alerta Si |
|---------|----------|-----------|
| **Processing Time** | <3min promedio | >5min |
| **Success Rate** | >95% | <90% |
| **Queue Depth** | <100 mensajes | >500 |
| **OpenAI Cost/Material** | <$0.20 | >$0.30 |

## Prometheus Metrics

```go
var (
    MaterialsProcessed = prometheus.NewCounter(...)
    ProcessingDuration = prometheus.NewHistogram(...)
    OpenAICost = prometheus.NewGauge(...)
    QueueDepth = prometheus.NewGauge(...)
)
```

## Logs Estructurados

```go
logger.Info("Material processed",
    "material_id", materialID,
    "duration_seconds", duration,
    "cost_usd", cost,
    "tokens_used", tokens,
)
```

## Alertas Críticas
- Queue depth >500 → Escalar workers
- Success rate <90% → Investigar
- Costo promedio >$0.30 → Optimizar prompts
