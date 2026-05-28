# Manifiesto de limpieza — edugo-worker

Fecha: 2026-05-22. Triage de documentación interna del worker.

## Resultado

`docs_new/` se renombró a `docs/` (única carpeta de documentación). Todos sus
documentos eran referencia técnica derivada del código y vigente → se conservaron.

## KEEP

| Documento | Motivo |
|---|---|
| `docs/arquitectura.md` | Referencia de arquitectura, stack, ResourceBuilder, flujo de arranque. Atada al código actual (Go 1.25.0, processors, builder). |
| `docs/configuracion.md` | Sistema Viper, variables de entorno, YAML, Makefile, ambientes. Vigente. |
| `docs/eventos.md` | Mensajería RabbitMQ: exchange, routing keys, schemas de eventos, rate limiting, DLQ. Vigente. |
| `docs/infraestructura.md` | Componentes de infra: health, circuit breaker, rate limiter, métricas. Vigente. |
| `docs/procesadores.md` | Interfaz Processor, registry, guía para agregar processors. Vigente. |
| `docs/testing.md` | Framework de testing, mocks, testcontainers, comandos. Vigente. |
| `../README.md` | README canónico (se queda en raíz). Se corrigieron referencias obsoletas. |
| `../CHANGELOG.md` | Changelog estándar Keep-a-Changelog (se queda en raíz). |

## Cambios menores al README (raíz)

- Go 1.23 → Go 1.25 (alineado con `go.mod`).
- Eliminada referencia a directorio inexistente `improvements/`.
- Eliminada referencia a `DESIGN_PROCESSOR_REGISTRY.md` (ya no existe; solo
  queda `internal/bootstrap/DESIGN_RESOURCE_BUILDER.md`).
- Añadido apuntador a `docs/` para la documentación técnica.

## RESCUE / DELETE

Ninguno. No había reportes efímeros ni planes en este proyecto.
