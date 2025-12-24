package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/stretchr/testify/assert"
)

// mockLogger implementa logger.Logger para testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...interface{})  {}
func (m *mockLogger) Info(msg string, fields ...interface{})   {}
func (m *mockLogger) Warn(msg string, fields ...interface{})   {}
func (m *mockLogger) Error(msg string, fields ...interface{})  {}
func (m *mockLogger) Fatal(msg string, fields ...interface{})  {}
func (m *mockLogger) With(fields ...interface{}) logger.Logger { return m }
func (m *mockLogger) Sync() error                              { return nil }

func createTestLogger() logger.Logger {
	return &mockLogger{}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "PDF corrupto es permanente",
			err:      pdfErrors.ErrPDFCorrupt,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF escaneado es permanente",
			err:      pdfErrors.ErrPDFScanned,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF demasiado grande es permanente",
			err:      pdfErrors.ErrPDFTooLarge,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF vacío es permanente",
			err:      pdfErrors.ErrPDFEmpty,
			expected: ErrorTypePermanent,
		},
		{
			name:     "error genérico es transitorio por defecto",
			err:      errors.New("error genérico de red"),
			expected: ErrorTypeTransient,
		},
		{
			name:     "nil error es permanente (no debería pasar)",
			err:      nil,
			expected: ErrorTypePermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithRetry_Success(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callCount := 0

	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return nil // éxito en el primer intento
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "debería llamarse solo una vez")
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callCount := 0
	transientErr := errors.New("error transitorio de red")

	err := WithRetry(ctx, cfg, func() error {
		callCount++
		if callCount < 3 {
			return transientErr // fallar las primeras 2 veces
		}
		return nil // éxito en el tercer intento
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "debería haber 3 intentos")
}

func TestWithRetry_PermanentError(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callCount := 0

	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return pdfErrors.ErrPDFCorrupt // error permanente
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, pdfErrors.ErrPDFCorrupt)
	assert.Equal(t, 1, callCount, "no debería reintentar errores permanentes")
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      100 * time.Millisecond,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callCount := 0
	transientErr := errors.New("error transitorio persistente")

	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return transientErr // siempre falla
	})

	assert.Error(t, err)
	assert.Equal(t, transientErr, err)
	assert.Equal(t, 4, callCount, "debería intentar 4 veces (1 inicial + 3 reintentos)")
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      1 * time.Second,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	transientErr := errors.New("error transitorio")

	// Cancelar después del primer intento
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(ctx, cfg, func() error {
		callCount++
		return transientErr
	})

	assert.Error(t, err)
	assert.True(t, isContextError(err), "debería ser un error de contexto")
	assert.LessOrEqual(t, callCount, 2, "no debería completar todos los reintentos")
}

func TestWithRetry_ExponentialBackoff(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      3,
		InitialBackoff:  50 * time.Millisecond,
		MaxBackoff:      500 * time.Millisecond,
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callTimes := []time.Time{}
	transientErr := errors.New("error transitorio")

	err := WithRetry(ctx, cfg, func() error {
		callTimes = append(callTimes, time.Now())
		return transientErr
	})

	assert.Error(t, err)
	assert.Equal(t, 4, len(callTimes), "debería haber 4 intentos")

	// Verificar que los tiempos entre llamadas aumentan exponencialmente
	if len(callTimes) >= 2 {
		diff1 := callTimes[1].Sub(callTimes[0])
		assert.GreaterOrEqual(t, diff1, 40*time.Millisecond, "primer backoff debería ser ~50ms")
	}

	if len(callTimes) >= 3 {
		diff2 := callTimes[2].Sub(callTimes[1])
		assert.GreaterOrEqual(t, diff2, 90*time.Millisecond, "segundo backoff debería ser ~100ms")
	}
}

func TestWithRetry_BackoffCap(t *testing.T) {
	log := createTestLogger()
	cfg := RetryConfig{
		MaxRetries:      5,
		InitialBackoff:  10 * time.Millisecond,
		MaxBackoff:      50 * time.Millisecond, // cap bajo para test
		BackoffMultiple: 2.0,
		Logger:          log,
	}

	ctx := context.Background()
	callTimes := []time.Time{}
	transientErr := errors.New("error transitorio")

	err := WithRetry(ctx, cfg, func() error {
		callTimes = append(callTimes, time.Now())
		return transientErr
	})

	assert.Error(t, err)
	assert.Equal(t, 6, len(callTimes), "debería haber 6 intentos")

	// Verificar que el backoff no excede el máximo
	// Después de varios intentos, el backoff debería estabilizarse en maxBackoff
	if len(callTimes) >= 5 {
		diff := callTimes[4].Sub(callTimes[3])
		assert.LessOrEqual(t, diff, 70*time.Millisecond, "backoff no debería exceder max + margen")
	}
}

func TestIsContextError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "context.Canceled es context error",
			err:      context.Canceled,
			expected: true,
		},
		{
			name:     "context.DeadlineExceeded es context error",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "error genérico no es context error",
			err:      errors.New("error genérico"),
			expected: false,
		},
		{
			name:     "nil no es context error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContextError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	log := createTestLogger()
	cfg := DefaultRetryConfig(log)

	assert.Equal(t, maxRetries, cfg.MaxRetries)
	assert.Equal(t, initialBackoff, cfg.InitialBackoff)
	assert.Equal(t, maxBackoff, cfg.MaxBackoff)
	assert.Equal(t, backoffMultiple, cfg.BackoffMultiple)
	assert.NotNil(t, cfg.Logger)
}
