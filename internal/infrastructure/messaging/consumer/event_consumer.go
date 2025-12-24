package consumer

import (
	"context"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
)

// EventConsumer consume eventos de RabbitMQ y los enruta a processors
type EventConsumer struct {
	registry *processor.Registry
	logger   logger.Logger
}

func NewEventConsumer(
	registry *processor.Registry,
	logger logger.Logger,
) *EventConsumer {
	return &EventConsumer{
		registry: registry,
		logger:   logger,
	}
}

// RouteEvent enruta el evento al processor correcto usando el registry
func (c *EventConsumer) RouteEvent(ctx context.Context, body []byte) error {
	c.logger.Debug("routing event to processor")
	return c.registry.Process(ctx, body)
}
