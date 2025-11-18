# Tareas Sprint 05

## TASK-05-001: Tests Integración RabbitMQ
**Estimación:** 3h

```go
// tests/integration/rabbitmq_test.go
func TestWorker_ConsumeMessage(t *testing.T) {
    // Testcontainers con RabbitMQ
    rabbitmqContainer := testcontainers.RunContainer(...)
    
    // Publicar mensaje
    // Verificar que worker lo procesa
    // Verificar ACK
}
```

## TASK-05-002: Tests E2E Flow
**Estimación:** 3h

```go
func TestE2E_MaterialProcessing(t *testing.T) {
    // 1. Publicar evento RabbitMQ
    // 2. Worker procesa
    // 3. Verificar MongoDB tiene resumen
    // 4. Verificar PostgreSQL actualizado
}
```

**Tiempo:** 6h
