package processor

import (
	"context"
	"encoding/json"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
)

type AssessmentAttemptProcessor struct {
	logger logger.Logger
}

func NewAssessmentAttemptProcessor(logger logger.Logger) *AssessmentAttemptProcessor {
	return &AssessmentAttemptProcessor{logger: logger}
}

func (p *AssessmentAttemptProcessor) EventType() string {
	return "assessment_attempt"
}

func (p *AssessmentAttemptProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.AssessmentAttemptEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *AssessmentAttemptProcessor) processEvent(ctx context.Context, event dto.AssessmentAttemptEvent) error {
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
