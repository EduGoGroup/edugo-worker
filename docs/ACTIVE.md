---
plan: ninguno
estado: estable
actualizado: 2026-05-22
---
# Norte actual — edugo-worker

Sin plan activo. El worker está implementado y estable: consume RabbitMQ y procesa materiales,
intentos, inscripciones y notificaciones vía ProcessorRegistry. La Fase 1 (refactor Bootstrap +
ProcessorRegistry + ResourceBuilder) está cerrada (ver `README.md`).

Deuda viva (ecosistema): el flujo `assessment.published`/announcements → notificaciones no está
cerrado end-to-end; los procesadores de notificación existen pero falta el disparo completo desde
platform. Se coordina en `../../../docs/ACTIVE.md`.
