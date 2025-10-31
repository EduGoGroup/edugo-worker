package processor

import (
	"context"

	"github.com/edugo/worker/internal/application/dto"
	"github.com/edugo/shared/pkg/logger"
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

func (p *MaterialReprocessProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
	p.logger.Info("reprocessing material", "material_id", event.MaterialID)

	// Reprocesar es lo mismo que procesar por primera vez
	return p.uploadedProcessor.Process(ctx, event)
}
