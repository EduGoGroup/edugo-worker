package health

import (
	"time"

	sharedh "github.com/EduGoGroup/edugo-shared/health"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Status = sharedh.Status
type CheckResult = sharedh.CheckResult
type HealthCheck = sharedh.HealthCheck
type Checker = sharedh.Checker

type RabbitMQChannel = sharedh.RabbitMQChannel
type RabbitMQCheck = sharedh.RabbitMQCheck

const (
	StatusHealthy   = sharedh.StatusHealthy
	StatusUnhealthy = sharedh.StatusUnhealthy
	StatusDegraded  = sharedh.StatusDegraded
)

// NewChecker creates a new health Checker.
func NewChecker() *Checker {
	return sharedh.NewChecker()
}

// NewRabbitMQCheck creates a new RabbitMQ health check.
func NewRabbitMQCheck(channel *amqp.Channel, timeout time.Duration) *RabbitMQCheck {
	return sharedh.NewRabbitMQCheck(channel, timeout)
}

// NewRabbitMQCheckWithChannel creates a RabbitMQ health check using the RabbitMQChannel interface.
func NewRabbitMQCheckWithChannel(channel RabbitMQChannel, timeout time.Duration) *RabbitMQCheck {
	return sharedh.NewRabbitMQCheckWithChannel(channel, timeout)
}
