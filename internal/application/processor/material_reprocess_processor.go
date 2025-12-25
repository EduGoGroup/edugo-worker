package processor

import (
	"context"
	"encoding/json"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
)

type MaterialReprocessProcessor struct {
	uploadedProcessor *MaterialUploadedProcessor
	logger            logger.Logger
}

func NewMaterialReprocessProcessor(uploadedProcessor *MaterialUploadedProcessor, logger logger.Logger) *MaterialReprocessProcessor {
	return &MaterialReprocessProcessor{
		uploadedProcessor: uploadedProcessor,
		logger:            logger,
	}
}

func (p *MaterialReprocessProcessor) processEvent(ctx context.Context, event dto.MaterialUploadedEvent) error {
	p.logger.Info("reprocessing material", "material_id", event.GetMaterialID())

	// Reprocesar es lo mismo que procesar por primera vez
	return p.uploadedProcessor.processEvent(ctx, event)
}

// EventType implementa la interfaz Processor
func (p *MaterialReprocessProcessor) EventType() string {
	return "material_reprocess"
}

// Process implementa la interfaz Processor
func (p *MaterialReprocessProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.MaterialUploadedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}
