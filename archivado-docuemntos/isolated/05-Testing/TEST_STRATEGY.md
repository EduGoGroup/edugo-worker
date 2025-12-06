# Estrategia de Testing - Worker

## Pirámide de Testing
```
     /E2E\      10% - Flow completo (RabbitMQ → Worker → MongoDB)
    /INT  \     20% - Integración con RabbitMQ, MongoDB
   /  UNIT  \   70% - Unitarios (services, validators)
```

## Coverage Objetivos
- **Global:** >75%
- **Services:** >80%
- **Consumer:** >70%

## Herramientas
- Testify (assertions)
- Testcontainers (RabbitMQ, MongoDB)
- Mocks para OpenAI

## Tests Críticos
- Procesamiento end-to-end
- Retry logic funciona
- Dead Letter Queue
- Rate limiting OpenAI

```bash
# Ejecutar todos
go test ./... -v -cover
```
