package storage

import (
	"context"
	"io"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/circuitbreaker"
)

// ClientWithCircuitBreaker envuelve un Client con protección de circuit breaker
type ClientWithCircuitBreaker struct {
	client         Client
	circuitBreaker *circuitbreaker.CircuitBreaker
}

// NewClientWithCircuitBreaker crea un nuevo cliente Storage con circuit breaker
func NewClientWithCircuitBreaker(client Client, cb *circuitbreaker.CircuitBreaker) Client {
	return &ClientWithCircuitBreaker{
		client:         client,
		circuitBreaker: cb,
	}
}

// Download descarga un archivo con protección de circuit breaker
func (c *ClientWithCircuitBreaker) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	var reader io.ReadCloser
	var err error

	executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		reader, err = c.client.Download(ctx, key)
		return err
	})

	if executeErr != nil {
		return nil, executeErr
	}

	return reader, nil
}

// Upload sube un archivo con protección de circuit breaker
func (c *ClientWithCircuitBreaker) Upload(ctx context.Context, key string, content io.Reader) error {
	return c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return c.client.Upload(ctx, key, content)
	})
}

// Delete elimina un archivo con protección de circuit breaker
func (c *ClientWithCircuitBreaker) Delete(ctx context.Context, key string) error {
	return c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		return c.client.Delete(ctx, key)
	})
}

// Exists verifica si un archivo existe con protección de circuit breaker
func (c *ClientWithCircuitBreaker) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	var err error

	executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		exists, err = c.client.Exists(ctx, key)
		return err
	})

	if executeErr != nil {
		return false, executeErr
	}

	return exists, nil
}

// GetMetadata obtiene metadatos con protección de circuit breaker
func (c *ClientWithCircuitBreaker) GetMetadata(ctx context.Context, key string) (*FileMetadata, error) {
	var metadata *FileMetadata
	var err error

	executeErr := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		metadata, err = c.client.GetMetadata(ctx, key)
		return err
	})

	if executeErr != nil {
		return nil, executeErr
	}

	return metadata, nil
}

// Verificar que implementa la interfaz
var _ Client = (*ClientWithCircuitBreaker)(nil)
