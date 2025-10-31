package consumer

import (
	"context"
	"encoding/json"

	"github.com/edugo/worker/internal/application/dto"
	"github.com/edugo/worker/internal/application/processor"
	"github.com/edugo/shared/pkg/logger"
	"github.com/edugo/shared/pkg/types/enum"
)

// EventConsumer consume eventos de RabbitMQ y los enruta a processors
type EventConsumer struct {
	materialUploadedProc   *processor.MaterialUploadedProcessor
	materialReprocessProc  *processor.MaterialReprocessProcessor
	materialDeletedProc    *processor.MaterialDeletedProcessor
	assessmentAttemptProc  *processor.AssessmentAttemptProcessor
	studentEnrolledProc    *processor.StudentEnrolledProcessor
	logger                 logger.Logger
}

func NewEventConsumer(
	materialUploadedProc *processor.MaterialUploadedProcessor,
	materialReprocessProc *processor.MaterialReprocessProcessor,
	materialDeletedProc *processor.MaterialDeletedProcessor,
	assessmentAttemptProc *processor.AssessmentAttemptProcessor,
	studentEnrolledProc *processor.StudentEnrolledProcessor,
	logger logger.Logger,
) *EventConsumer {
	return &EventConsumer{
		materialUploadedProc:   materialUploadedProc,
		materialReprocessProc:  materialReprocessProc,
		materialDeletedProc:    materialDeletedProc,
		assessmentAttemptProc:  assessmentAttemptProc,
		studentEnrolledProc:    studentEnrolledProc,
		logger:                 logger,
	}
}

// RouteEvent enruta el evento al processor correcto
func (c *EventConsumer) RouteEvent(ctx context.Context, body []byte) error {
	// Parsear event type
	var baseEvent struct {
		EventType string `json:"event_type"`
	}

	if err := json.Unmarshal(body, &baseEvent); err != nil {
		c.logger.Error("failed to parse event", "error", err)
		return err
	}

	c.logger.Debug("routing event", "event_type", baseEvent.EventType)

	// Enrutar según tipo de evento
	switch enum.EventType(baseEvent.EventType) {
	case enum.EventMaterialUploaded:
		var event dto.MaterialUploadedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return c.materialUploadedProc.Process(ctx, event)

	case enum.EventMaterialReprocess:
		var event dto.MaterialUploadedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return c.materialReprocessProc.Process(ctx, event)

	case enum.EventMaterialDeleted:
		var event dto.MaterialDeletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return c.materialDeletedProc.Process(ctx, event)

	case enum.EventAssessmentAttemptRecorded:
		var event dto.AssessmentAttemptEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return c.assessmentAttemptProc.Process(ctx, event)

	case enum.EventStudentEnrolled:
		var event dto.StudentEnrolledEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}
		return c.studentEnrolledProc.Process(ctx, event)

	default:
		c.logger.Warn("unknown event type", "event_type", baseEvent.EventType)
		return nil
	}
}
