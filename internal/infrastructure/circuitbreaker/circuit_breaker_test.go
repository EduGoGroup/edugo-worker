package circuitbreaker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")

	// Act
	cb := New(config)

	// Assert
	assert.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(0), cb.Failures())
	assert.Equal(t, uint32(0), cb.Successes())
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	// Arrange
	cb := New(DefaultConfig("test"))
	ctx := context.Background()

	// Act
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(0), cb.Failures())
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	// Arrange
	cb := New(DefaultConfig("test"))
	ctx := context.Background()
	expectedErr := errors.New("test error")

	// Act
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return expectedErr
	})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(1), cb.Failures())
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 3
	cb := New(config)
	ctx := context.Background()

	// Act - Generar 3 fallos
	for i := 0; i < 3; i++ {
		_ = cb.Execute(ctx, func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	// Assert
	assert.Equal(t, StateOpen, cb.State())
	assert.Equal(t, uint32(3), cb.Failures())
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 1
	cb := New(config)
	ctx := context.Background()

	// Abrir el circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	// Act - Intentar ejecutar con circuit abierto
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	// Assert
	assert.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 1
	config.Timeout = 100 * time.Millisecond
	cb := New(config)
	ctx := context.Background()

	// Abrir el circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	assert.Equal(t, StateOpen, cb.State())

	// Esperar a que pase el timeout
	time.Sleep(150 * time.Millisecond)

	// Act - Ejecutar después del timeout
	err := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_ClosesAfterSuccessesInHalfOpen(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 1
	config.Timeout = 100 * time.Millisecond
	config.SuccessThreshold = 2
	config.MaxRequests = 5
	cb := New(config)
	ctx := context.Background()

	// Abrir el circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	// Esperar a que pase el timeout
	time.Sleep(150 * time.Millisecond)

	// Act - Ejecutar con éxito en half-open
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}

	// Assert
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(0), cb.Failures())
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 1
	config.Timeout = 100 * time.Millisecond
	config.MaxRequests = 2 // Permitir 2 peticiones en half-open para este test
	cb := New(config)
	ctx := context.Background()

	// Abrir el circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	// Esperar a que pase el timeout
	time.Sleep(150 * time.Millisecond)

	// Transicionar a half-open con una ejecución exitosa
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.Equal(t, StateHalfOpen, cb.State())

	// Act - Fallar en half-open
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	// Assert
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_LimitsRequestsInHalfOpen(t *testing.T) {
	// Arrange
	config := DefaultConfig("test")
	config.MaxFailures = 1
	config.Timeout = 100 * time.Millisecond
	config.MaxRequests = 1
	config.SuccessThreshold = 10 // Alto para que no se cierre inmediatamente
	cb := New(config)
	ctx := context.Background()

	// Abrir el circuit
	_ = cb.Execute(ctx, func(ctx context.Context) error {
		return errors.New("test error")
	})

	// Esperar a que pase el timeout
	time.Sleep(150 * time.Millisecond)

	// Transicionar a half-open con primera petición
	err1 := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err1)
	assert.Equal(t, StateHalfOpen, cb.State())

	// Act - La segunda petición debe ser rechazada porque MaxRequests=1
	err2 := cb.Execute(ctx, func(ctx context.Context) error {
		return nil
	})

	// Assert
	assert.Error(t, err2)
	assert.Equal(t, ErrTooManyRequests, err2)
	assert.Equal(t, StateHalfOpen, cb.State())
}
