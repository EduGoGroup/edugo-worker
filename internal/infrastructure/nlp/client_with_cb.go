package nlp

import (
	"context"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/circuitbreaker"
)

// ClientWithCircuitBreaker envuelve un Client con protección de circuit breaker
type ClientWithCircuitBreaker struct {
	client         Client
	circuitBreaker *circuitbreaker.CircuitBreaker
}

// NewClientWithCircuitBreaker crea un nuevo cliente NLP con circuit breaker
func NewClientWithCircuitBreaker(client Client, cb *circuitbreaker.CircuitBreaker) Client {
	return &ClientWithCircuitBreaker{
		client:         client,
		circuitBreaker: cb,
	}
}

// GenerateSummary genera un resumen con protección de circuit breaker
func (c *ClientWithCircuitBreaker) GenerateSummary(ctx context.Context, text string) (*Summary, error) {
	var summary *Summary
	var err error

	executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		summary, err = c.client.GenerateSummary(ctx, text)
		return err
	})

	if executeErr != nil {
		return nil, executeErr
	}

	return summary, nil
}

// GenerateQuiz genera un quiz con protección de circuit breaker
func (c *ClientWithCircuitBreaker) GenerateQuiz(ctx context.Context, text string, questionCount int) (*Quiz, error) {
	var quiz *Quiz
	var err error

	executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		quiz, err = c.client.GenerateQuiz(ctx, text, questionCount)
		return err
	})

	if executeErr != nil {
		return nil, executeErr
	}

	return quiz, nil
}

// HealthCheck verifica la salud del servicio con protección de circuit breaker
func (c *ClientWithCircuitBreaker) HealthCheck(ctx context.Context) error {
	return c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return c.client.HealthCheck(ctx)
	})
}

// Verificar que implementa la interfaz
var _ Client = (*ClientWithCircuitBreaker)(nil)
