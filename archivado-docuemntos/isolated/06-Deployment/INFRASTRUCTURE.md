# Infraestructura - Worker

## Arquitectura

```
[RabbitMQ] → [Worker Instance 1] → [MongoDB]
              [Worker Instance 2]    [PostgreSQL]
              [Worker Instance 3]    [S3]
```

## Docker Compose

```yaml
version: '3.8'
services:
  worker:
    image: edugo-worker:latest
    environment:
      - RABBITMQ_URL=amqp://rabbitmq:5672
      - MONGO_URI=mongodb://mongo:27017
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    deploy:
      replicas: 3
    depends_on:
      - rabbitmq
      - mongodb
```

## Escalado
- Horizontal: 2-5 workers según carga
- RabbitMQ distribuye mensajes automáticamente

## Recursos por Worker
- CPU: 1 core
- RAM: 2GB
- Disk: 5GB (temporal para PDFs)
