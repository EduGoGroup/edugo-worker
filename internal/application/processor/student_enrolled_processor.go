package processor

import (
	"context"
	"encoding/json"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
)

type StudentEnrolledProcessor struct {
	logger logger.Logger
}

func NewStudentEnrolledProcessor(logger logger.Logger) *StudentEnrolledProcessor {
	return &StudentEnrolledProcessor{logger: logger}
}

func (p *StudentEnrolledProcessor) EventType() string {
	return "student_enrolled"
}

func (p *StudentEnrolledProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.StudentEnrolledEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *StudentEnrolledProcessor) processEvent(ctx context.Context, event dto.StudentEnrolledEvent) error {
	p.logger.Info("processing student enrolled",
		"student_id", event.StudentID,
		"unit_id", event.UnitID,
	)

	// Aquí se podría:
	// - Enviar email de bienvenida
	// - Crear registro de onboarding
	// - Notificar al teacher

	p.logger.Info("student enrollment processed successfully")
	return nil
}
