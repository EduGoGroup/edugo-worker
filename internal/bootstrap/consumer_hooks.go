package bootstrap

import (
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/rabbit"

	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
)

// Mensajes de log de un fallo del handler. Los guiones de scripts/e2e los buscan tal cual.
const (
	RetryLogMessage      = "Mensaje fallido: se reintenta"
	DeadLetterLogMessage = "Mensaje enviado a la cola muerta"
)

// WithFailureHooks devuelve la configuración del consumer con el clasificador del worker
// (lo permanente va a la cola muerta sin reintentos) y con una línea de log por cada
// reintento y por cada envío a la cola muerta.
//
// Los ganchos se llaman desde varias goroutines; solo usan el logger, que es seguro en
// concurrencia.
func WithFailureHooks(cfg rabbit.ConsumerConfig, log logger.Logger) rabbit.ConsumerConfig {
	cfg.IsPermanent = processor.IsPermanent
	cfg.OnRetry = func(ev rabbit.FailureEvent) {
		log.Warn(RetryLogMessage, failureFields(ev, "transient")...)
	}
	cfg.OnDeadLetter = func(ev rabbit.FailureEvent) {
		log.Error(DeadLetterLogMessage, failureFields(ev, ev.Reason)...)
	}
	return cfg
}

// failureFields arma los campos comunes de las dos líneas. El hash y el tamaño del cuerpo
// identifican un mensaje que no trae nada legible (un JSON roto).
func failureFields(ev rabbit.FailureEvent, reason string) []any {
	errText := ""
	if ev.Err != nil {
		errText = ev.Err.Error()
	}
	return []any{
		"queue", ev.Queue,
		"retry", ev.Retry,
		"max_retries", ev.MaxRetries,
		"backoff", ev.Backoff.String(),
		"reason", reason,
		"error", errText,
		"body_size", ev.BodySize,
		"body_hash", ev.BodyHash,
	}
}
