package health

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQChannel define la interfaz para operaciones de RabbitMQ necesarias para health checks
type RabbitMQChannel interface {
	IsClosed() bool
}

// RabbitMQCheck implementa HealthCheck para RabbitMQ
type RabbitMQCheck struct {
	channel RabbitMQChannel
	timeout time.Duration
}

// NewRabbitMQCheck crea un nuevo RabbitMQ health check
func NewRabbitMQCheck(channel *amqp.Channel, timeout time.Duration) *RabbitMQCheck {
	return &RabbitMQCheck{
		channel: channel,
		timeout: timeout,
	}
}

// NewRabbitMQCheckWithChannel crea un health check con una interfaz RabbitMQChannel
// Útil para testing con mocks
func NewRabbitMQCheckWithChannel(channel RabbitMQChannel, timeout time.Duration) *RabbitMQCheck {
	return &RabbitMQCheck{
		channel: channel,
		timeout: timeout,
	}
}

// Name retorna el nombre del health check
func (c *RabbitMQCheck) Name() string {
	return "rabbitmq"
}

// Check ejecuta el health check de RabbitMQ
func (c *RabbitMQCheck) Check(ctx context.Context) CheckResult {
	result := CheckResult{
		Component: c.Name(),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	start := time.Now()

	// Verificar si el canal está cerrado
	if c.channel.IsClosed() {
		result.Status = StatusUnhealthy
		result.Message = "RabbitMQ channel is closed"
		return result
	}

	duration := time.Since(start)
	result.Metadata["response_time_ms"] = duration.Milliseconds()

	result.Status = StatusHealthy
	result.Message = "RabbitMQ is healthy"
	return result
}
