# Guía de Deployment - Worker

## Pre-requisitos
- [ ] RabbitMQ 3.12+ configurado
- [ ] MongoDB 7.0+ accesible
- [ ] PostgreSQL 15+ accesible
- [ ] OpenAI API key válida
- [ ] S3 bucket con PDFs

## Pasos

### 1. Configurar Variables de Entorno
```bash
export RABBITMQ_URL="amqp://user:pass@rabbitmq:5672/"
export MONGO_URI="mongodb://mongo:27017"
export DB_URL="postgres://user:pass@postgres:5432/edugo"
export OPENAI_API_KEY="sk-..."
export S3_BUCKET="edugo-materials"
```

### 2. Deploy con Docker
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Build
docker build -t edugo-worker:latest .

# Run
docker run -d \
  -e RABBITMQ_URL=$RABBITMQ_URL \
  -e MONGO_URI=$MONGO_URI \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  --name edugo-worker \
  edugo-worker:latest

# Logs
docker logs -f edugo-worker
```

### 3. Verificar Funcionamiento
```bash
# Verificar que consume mensajes
docker logs edugo-worker | grep "Consumed message"

# Verificar conexión a RabbitMQ
docker exec edugo-worker curl http://rabbitmq:15672/api/queues
```

### 4. Escalado Horizontal
```bash
# Levantar 3 workers
docker-compose up -d --scale worker=3
```

## Rollback
```bash
docker stop edugo-worker
docker rm edugo-worker
docker run edugo-worker:previous-version
```
