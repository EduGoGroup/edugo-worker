# Validación - Fase 4: Observabilidad y Resiliencia

---

## ✅ Checklist de Validación

### 1. Métricas Prometheus

**Verificar endpoint:**
```bash
curl http://localhost:8080/metrics

# Debe retornar métricas en formato Prometheus
```

**Criterios:**
- [ ] Endpoint `/metrics` accesible
- [ ] Métricas de eventos presentes
- [ ] Métricas de OpenAI presentes
- [ ] Métricas de DBs presentes
- [ ] Formato Prometheus válido

---

### 2. Health Checks

**Verificar endpoints:**
```bash
# Health general
curl http://localhost:8080/health

# Liveness
curl http://localhost:8080/health/live

# Readiness
curl http://localhost:8080/health/ready
```

**Criterios:**
- [ ] `/health` retorna JSON con status
- [ ] `/health/live` retorna 200 cuando worker vivo
- [ ] `/health/ready` retorna 200 cuando listo
- [ ] Componentes individuales reportan estado
- [ ] Retorna 503 cuando unhealthy

---

### 3. Circuit Breakers

**Test manual:**
```bash
# Simular fallo de OpenAI
# (configurar API key inválida temporalmente)

# Ver logs, debe mostrar:
# - Circuit breaker abierto
# - Requests rechazados
# - Half-open después de timeout
```

**Criterios:**
- [ ] Se abre después de N fallos consecutivos
- [ ] Rechaza requests cuando abierto
- [ ] Pasa a half-open después de timeout
- [ ] Se cierra cuando requests exitosos
- [ ] Logs claros de cambios de estado

---

### 4. Rate Limiting

**Test:**
```bash
# Enviar múltiples eventos rápidamente
# Ver que OpenAI no excede límite configurado
```

**Criterios:**
- [ ] Respeta límite de requests/segundo
- [ ] No excede límites de OpenAI API
- [ ] Backoff funciona ante rate limit (429)
- [ ] Métricas reflejan throttling

---

### 5. Graceful Shutdown

**Test:**
```bash
# Iniciar worker
./bin/worker &
WORKER_PID=$!

# Enviar mensaje
# Inmediatamente enviar SIGTERM
kill -TERM $WORKER_PID

# Ver logs
```

**Criterios:**
- [ ] Completa procesamiento de mensaje actual
- [ ] No acepta nuevos mensajes
- [ ] Cierra conexiones limpiamente
- [ ] Exit code 0
- [ ] Logs muestran shutdown ordenado

---

### 6. Dashboards Grafana

**Verificar:**
- [ ] Dashboard importa correctamente
- [ ] Métricas se visualizan
- [ ] Paneles muestran datos reales
- [ ] Rangos de tiempo funcionan

---

### 7. Integración con Kubernetes

**Probes:**
```yaml
# Verificar en pod de Kubernetes
kubectl describe pod worker-xxx

# Debe mostrar:
# Liveness: ... (healthy)
# Readiness: ... (ready)
```

**Criterios:**
- [ ] Liveness probe funciona
- [ ] Readiness probe funciona
- [ ] Pod se marca como Ready
- [ ] Pod no se reinicia inesperadamente

---

## 🎯 Criterios de Aceptación

✅ **FASE 4 EXITOSA** si:

1. Métricas Prometheus funcionando
2. Health checks responden correctamente
3. Circuit breakers funcionan
4. Rate limiting implementado
5. Graceful shutdown funciona
6. Dashboards muestran datos
7. CI/CD pasa
8. PR aprobado y mergeado

---

## 🎉 Plan de Mejoras Completado

Con la Fase 4 completada, el worker está **100% listo para producción**.
