package processor

import (
	"context"

	"github.com/edugo/worker/internal/application/dto"
	"github.com/edugo/shared/pkg/logger"
)

type AssessmentAttemptProcessor struct {
	logger logger.Logger
}

func NewAssessmentAttemptProcessor(logger logger.Logger) *AssessmentAttemptProcessor {
	return &AssessmentAttemptProcessor{logger: logger}
}

func (p *AssessmentAttemptProcessor) Process(ctx context.Context, event dto.AssessmentAttemptEvent) error {
	p.logger.Info("processing assessment attempt",
		"material_id", event.MaterialID,
		"user_id", event.UserID,
		"score", event.Score,
	)

	// Aquí se podría:
	// - Enviar notificación al docente si score bajo
	// - Actualizar estadísticas
	// - Registrar en tabla de analytics

	p.logger.Info("assessment attempt processed successfully")
	return nil
}
