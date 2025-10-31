package processor

import (
	"context"

	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/EduGoGroup/edugo-shared/pkg/logger"
)

type StudentEnrolledProcessor struct {
	logger logger.Logger
}

func NewStudentEnrolledProcessor(logger logger.Logger) *StudentEnrolledProcessor {
	return &StudentEnrolledProcessor{logger: logger}
}

func (p *StudentEnrolledProcessor) Process(ctx context.Context, event dto.StudentEnrolledEvent) error {
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
