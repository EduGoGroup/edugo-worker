# Imagen mínima de runtime — la compilación ocurre en el CI (manual-release.yml
# Job 2) y el binario se pasa como contexto de build, eliminando toda compilación
# Go dentro del contenedor.
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Binario pre-compilado por el CI
COPY main .

# Archivos de configuración YAML
COPY config/ ./config/

RUN chmod +x ./main

CMD ["./main"]
