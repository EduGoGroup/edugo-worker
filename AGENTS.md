# AGENTS.md — edugo-worker

> Detalle local. Reglas globales del ecosistema en `../../AGENTS.md` (no las repitas).
> Norte actual del proyecto en `docs/ACTIVE.md`. Doc técnica detallada en `docs/` (arquitectura,
> eventos, configuracion, procesadores, infraestructura, testing) y en el `README.md`.

## Propósito

**Worker de procesamiento de eventos** de EduGo. No expone API HTTP de negocio: consume mensajes de
**RabbitMQ** y ejecuta el trabajo pesado/asíncrono detrás de `edugo-api-learning` —
extracción de texto de PDFs, embeddings/NLP para búsqueda, scoring de intentos, creación de
notificaciones, inscripción de estudiantes. Persiste en **PostgreSQL** y **MongoDB** y usa **S3**.

## Arquitectura (clean / DDD por capas — distinta a las APIs)

```
cmd/main.go                  punto de entrada (no hay cmd/api ni builder/)
internal/
  config/                    configuración (env / config.yaml)
  bootstrap/                 ResourceBuilder (API fluida, cleanup LIFO, validación de deps)
  container/                 contenedor de dependencias (DI)
  client/                    clientes externos (AuthClient hacia identity)
  application/
    processor/               procesadores de eventos + registry.go (enrutamiento por event_type) + retry
    service/                 servicios de aplicación
    dto/
  domain/
    service/ valueobject/ repository/ constants/   lógica y contratos de dominio
  infrastructure/
    messaging/   consumidor RabbitMQ
    persistence/ repos Postgres/Mongo
    storage/     S3 · pdf/ extracción · nlp/ embeddings
    http/ health/ metrics/ ratelimiter/ circuitbreaker/ shutdown/
```

**Patrón clave**: `ProcessorRegistry` — en vez de un switch gigante, cada procesador se registra y el
consumer enruta por `event_type`. Consumer y processors quedan desacoplados.

## Cómo correr y testear

`Makefile` (set propio del worker, distinto al de las APIs):
- `make build` — binario en `bin/`. `make run` o `go run cmd/main.go`.
- `make test` / `make test-coverage` / `make lint` / `make format`.
- Docker: `docker-compose up -d`. Config vía env (`POSTGRES_*`, `MONGODB_*`, `RABBITMQ_URL`,
  `API_IDENTITY_*`, `LOG_*`).

## Eventos procesados (registrados en el registry)

`material_uploaded`, `material_deleted`, `material_reprocess`, `assessment_attempt`,
`student_enrolled`, y notificaciones de assessment (`assigned`, `attempt`, `reviewed`).
Para agregar uno nuevo: implementa un `*_processor.go` que satisfaga la interfaz de `processor.go`,
regístralo en `registry.go`, y mapea el `event_type`/cola en `config`.

## Convenciones y gotchas locales

- **Sin servidor HTTP de negocio**: el `infrastructure/http` y `health` son para liveness/metrics, no API.
- **Schemas de eventos**: el contrato de los mensajes es compartido (`edugo-shared/messaging/events`)
  y validado contra JSON Schema en `edugo-infrastructure/schemas`. No inventes campos ad-hoc.
- **Resiliencia**: hay `retry`, `circuitbreaker`, `ratelimiter` y `shutdown` (graceful) ya implementados;
  reúsalos en procesadores nuevos en lugar de reinventar.
- **AuthClient** (`client/`) llama a la API de identity para validar/contexto; tiene caché con TTL.
- Reglas globales: código en inglés, logs/docs en español, fechas UTC.
