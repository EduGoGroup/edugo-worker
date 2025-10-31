# Dockerfile para Worker de Procesamiento
# Procesa materiales usando RabbitMQ

FROM golang:1.23-alpine AS builder

# Argumento para GitHub token (acceso a repos privados)
ARG GITHUB_TOKEN

# Instalar dependencias del sistema
RUN apk add --no-cache git ca-certificates

# Configurar git para usar token con repos privados
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Establecer directorio de trabajo
WORKDIR /app

# Variables de entorno para Go
ENV GOPRIVATE=github.com/EduGoGroup/*

# Copiar go.mod y go.sum
COPY go.mod go.sum ./

# Descargar dependencias (incluido edugo-shared privado)
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Etapa final - imagen ligera
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copiar binario compilado desde builder
COPY --from=builder /app/main .

# Comando de inicio
CMD ["./main"]
