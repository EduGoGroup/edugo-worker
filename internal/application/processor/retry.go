package processor

import (
	"context"
	"errors"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

const (
	// Configuración de retry
	maxRetries      = 3
	initialBackoff  = 500 * time.Millisecond
	maxBackoff      = 10 * time.Second
	backoffMultiple = 2.0
)

// ErrorType clasifica el tipo de error
type ErrorType int

const (
	// ErrorTypePermanent indica un error que no se puede resolver con reintentos
	ErrorTypePermanent ErrorType = iota
	// ErrorTypeTransient indica un error temporal que puede resolverse con reintentos
	ErrorTypeTransient
)

// RetryConfig configura el comportamiento de retry
type RetryConfig struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffMultiple float64
	Logger          logger.Logger
}

// DefaultRetryConfig retorna la configuración por defecto
func DefaultRetryConfig(log logger.Logger) RetryConfig {
	return RetryConfig{
		MaxRetries:      maxRetries,
		InitialBackoff:  initialBackoff,
		MaxBackoff:      maxBackoff,
		BackoffMultiple: backoffMultiple,
		Logger:          log,
	}
}

// classifyError determina si un error es transitorio o permanente
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypePermanent // No debería pasar, pero por seguridad
	}

	// Errores permanentes de PDF
	if errors.Is(err, pdfErrors.ErrPDFCorrupt) ||
		errors.Is(err, pdfErrors.ErrPDFScanned) ||
		errors.Is(err, pdfErrors.ErrPDFTooLarge) ||
		errors.Is(err, pdfErrors.ErrPDFEmpty) {
		return ErrorTypePermanent
	}

	// Errores permanentes de S3
	// Los errores de validación de S3 son permanentes
	// Los timeouts de S3 son transitorios

	// TODO: Clasificar errores de NLP cuando estén disponibles
	// - API key inválida → permanente
	// - Rate limit → transitorio
	// - Timeout → transitorio
	// - Quota excedida → permanente (o transitorio con backoff largo)

	// Por defecto, considerar transitorios los errores de red/timeout
	// que podrían resolverse con reintentos
	return ErrorTypeTransient
}

// WithRetry ejecuta una operación con lógica de reintento
func WithRetry(ctx context.Context, cfg RetryConfig, operation func() error) error {
	var lastErr error
	backoff := cfg.InitialBackoff

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Verificar cancelación de contexto antes de cada intento
		select {
		case <-ctx.Done():
			cfg.Logger.Warn("operación cancelada por contexto",
				"attempt", attempt,
				"lastError", lastErr,
			)
			return ctx.Err()
		default:
		}

		// Ejecutar operación
		err := operation()
		if err == nil {
			// Éxito
			if attempt > 0 {
				cfg.Logger.Info("operación exitosa después de reintentos", "attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// Clasificar error
		errorType := classifyError(err)
		if errorType == ErrorTypePermanent {
			cfg.Logger.Warn("error permanente detectado, no se reintentará",
				"error", err,
				"attempt", attempt+1,
			)
			return err
		}

		// Si es el último intento, no hacer backoff
		if attempt == cfg.MaxRetries {
			cfg.Logger.Error("máximo de reintentos alcanzado",
				"error", err,
				"attempts", attempt+1,
				"maxRetries", cfg.MaxRetries,
			)
			return err
		}

		// Error transitorio: aplicar backoff
		cfg.Logger.Warn("error transitorio, reintentando",
			"error", err,
			"attempt", attempt+1,
			"backoff", backoff,
		)

		// Esperar con backoff exponencial
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Calcular siguiente backoff (exponencial con límite)
		backoff = time.Duration(float64(backoff) * cfg.BackoffMultiple)
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return lastErr
}

// isContextError verifica si un error es por cancelación de contexto
func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
