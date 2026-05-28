# Tareas - Fase 4: Observabilidad y Resiliencia

---

## 📋 Resumen de Tareas

### Semana 1: Métricas y Health

**T4.1: Implementar métricas Prometheus** (8h)
- Definir métricas necesarias
- Instrumentar código
- Endpoint `/metrics`
- Tests

**T4.2: Implementar health checks** (6h)
- Checker para cada dependencia
- Endpoints HTTP (/health, /health/live, /health/ready)
- Tests

### Semana 2: Resiliencia

**T4.3: Implementar circuit breakers** (10h)
- Circuit breaker genérico
- Aplicar a OpenAI
- Aplicar a DBs
- Tests

**T4.4: Implementar rate limiting** (6h)
- Rate limiter para OpenAI
- Configuración flexible
- Tests

**T4.5: Graceful shutdown** (4h)
- Completar mensajes en proceso
- Cerrar conexiones ordenadamente
- Tests

### Semana 3: Dashboards y Documentación

**T4.6: Dashboards Grafana** (8h)
- Dashboard principal del worker
- Dashboard de errores
- Dashboard de OpenAI

**T4.7: Alertas básicas** (6h)
- Alertas de errores críticos
- Alertas de latencia alta
- Alertas de circuit breaker abierto

**T4.8: Documentación operacional** (4h)
- Runbook de troubleshooting
- Guía de métricas
- README actualizado

---

## ✅ Total Estimado: 52 horas (~2 semanas)
