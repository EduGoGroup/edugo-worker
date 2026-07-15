# AGENTS.md — edugo-worker

> Detalle local. Reglas globales del ecosistema en `../../AGENTS.md` (no las repitas).
> Norte actual del proyecto en `docs/ACTIVE.md`. Doc técnica detallada en `docs/` (arquitectura,
> eventos, configuracion, procesadores, infraestructura, testing) y en el `README.md`.

## Propósito

**Worker de procesamiento de eventos** de EduGo. No expone API HTTP de negocio: consume mensajes de
**RabbitMQ** y está pensado para el trabajo pesado/asíncrono detrás de `edugo-api-learning`.

> **Estado: ESQUELETO (plan 037 D-037.11).** El `ProcessorRegistry` arranca **vacío**: no hay processors
> de negocio. El único carril que le quedaba (`material.uploaded`/`material.reprocess`) persistía en
> **Mongo** y no tenía consumidor (el worker nunca se desplegó); al retirarse Mongo del ecosistema esos
> processors se eliminaron, junto con **Postgres** y **Mongo** (conexiones, config, health, métricas de BD).
> Sigue vivo como cáscara: conecta a RabbitMQ (declara exchange/cola + consumer con DLQ), healthcheck,
> métricas Prometheus, AuthClient (identity), rate limiter, circuit breakers, shutdown, y la infra
> PDF/NLP lista para reusar (el storage S3/MinIO se retiró — sin consumidores post-dieta 037; los
> materiales viven en Cloudflare R2 y los sirve learning; ver bug 0040). Los processors del carril
> **LLM** llegan en **037-F3** (store y orquestación nuevos).

## Arquitectura (clean / DDD por capas — distinta a las APIs)

```
cmd/main.go                  punto de entrada (no hay cmd/api ni builder/)
internal/
  config/                    configuración (env / config.yaml)
  bootstrap/                 ResourceBuilder (API fluida, cleanup LIFO, validación de deps)
  container/                 contenedor de dependencias (DI)
  client/                    clientes externos (AuthClient hacia identity)
  application/
    processor/               registry.go (enrutamiento por event_type) + processor.go (interfaz) + retry
                             — SIN implementaciones de processor (registry vacío, F3 los añade)
    dto/
  domain/
    valueobject/             lógica y contratos de dominio
  infrastructure/
    messaging/   consumidor RabbitMQ
    pdf/         extracción · nlp/ embeddings (listos para reusar en F3)
    http/ health/ metrics/ ratelimiter/ circuitbreaker/ shutdown/
```

**Patrón clave**: `ProcessorRegistry` — cada procesador se registra y el consumer enruta por
`event_type`. En el estado esqueleto el registry arranca vacío; el consumer tolera 0 processors.
Ya **no hay** `persistence/` (Postgres/Mongo retirados).

## Cómo correr y testear

`Makefile` (set propio del worker, distinto al de las APIs):
- `make build` — binario en `bin/`. `make run` o `go run cmd/main.go`.
- `make test` / `make test-coverage` / `make lint` / `make format`.
- Docker: `docker-compose up -d`. Config vía env (`RABBITMQ_URL`, `API_IDENTITY_*`, `LOG_*`,
  `OPENAI_API_KEY`). Ya **no** usa `POSTGRES_*` ni `MONGODB_*`.

## Eventos procesados (registrados en el registry)

**Ninguno hoy** — el registry arranca vacío (esqueleto, plan 037 D-037.11). Los processors del carril
**LLM** llegan en **037-F3**. Para agregar uno (cuando llegue F3): implementa un `*_processor.go` que
satisfaga la interfaz de `processor.go`, regístralo en el `ProcessorRegistry` (bootstrap
`WithProcessors`), y mapea el `event_type`/binding de RabbitMQ.

## Convenciones y gotchas locales

- **Sin servidor HTTP de negocio**: el `infrastructure/http` y `health` son para liveness/metrics, no API.
- **Schemas de eventos**: el contrato de los mensajes es compartido (`edugo-shared/messaging/events`)
  y validado contra JSON Schema en `edugo-infrastructure/schemas`. No inventes campos ad-hoc.
- **Resiliencia**: hay `retry`, `circuitbreaker`, `ratelimiter` y `shutdown` (graceful) ya implementados;
  reúsalos en procesadores nuevos en lugar de reinventar.
- **AuthClient** (`client/`) llama a la API de identity para validar/contexto; tiene caché con TTL.
- Reglas globales: código en inglés, logs/docs en español, fechas UTC.
