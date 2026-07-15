package processor

import (
	"errors"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/resilience/retry"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// ErrorType re-exporta el tipo del shared retry.
type ErrorType = retry.ErrorType

const (
	ErrorTypePermanent = retry.ErrorTypePermanent
	ErrorTypeTransient = retry.ErrorTypeTransient
)

// RetryConfig re-exporta la configuracion del shared retry.
type RetryConfig = retry.Config

// DefaultRetryConfig retorna la configuracion por defecto con el clasificador de errores del worker.
func DefaultRetryConfig(log logger.Logger) RetryConfig {
	cfg := retry.DefaultConfig()
	cfg.Logger = log
	cfg.Classifier = classifyError
	return cfg
}

// WithRetry delega al shared retry.WithRetry.
var WithRetry = retry.WithRetry

// classifyError determina si un error es transitorio o permanente.
// Contiene logica especifica del worker (errores de PDF).
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypePermanent
	}

	// Errores permanentes de PDF
	if errors.Is(err, pdfErrors.ErrPDFCorrupt) ||
		errors.Is(err, pdfErrors.ErrPDFScanned) ||
		errors.Is(err, pdfErrors.ErrPDFTooLarge) ||
		errors.Is(err, pdfErrors.ErrPDFEmpty) {
		return ErrorTypePermanent
	}

	// Evento malformado: reintentar no lo arregla, debe ir al DLQ sin reprocesar.
	if errors.Is(err, ErrMalformedEvent) {
		return ErrorTypePermanent
	}

	// 4xx permanente de learning (request malformada, answer inexistente, scope
	// insuficiente): reintentar no lo arregla.
	if errors.Is(err, m2m.ErrLearningPermanent) {
		return ErrorTypePermanent
	}

	return ErrorTypeTransient
}
