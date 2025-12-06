# Tareas Sprint 06

## TASK-06-001: GitHub Actions Workflow
**Estimación:** 3h

Archivo: `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/.github/workflows/ci.yml`

```yaml
name: CI Worker

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      rabbitmq:
        image: rabbitmq:3.12-alpine
        ports: [5672:5672]
      mongodb:
        image: mongo:7-alpine
        ports: [27017:27017]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test ./... -v -cover
```

## TASK-06-002: Dockerfile
**Estimación:** 2h

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk add --no-cache poppler-utils tesseract-ocr
COPY --from=builder /app/worker .
CMD ["./worker"]
```

**Tiempo:** 5h
