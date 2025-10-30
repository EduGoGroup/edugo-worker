# Dockerfile para Worker de Procesamiento
# Procesa materiales usando RabbitMQ

FROM golang:1.21-alpine AS builder

# Instalar dependencias del sistema
RUN apk add --no-cache git

# Establecer directorio de trabajo
WORKDIR /app

# Copiar go.mod y go.sum
COPY go.mod go.sum ./

# Descargar dependencias
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
