package processor

import (
	"context"
	"encoding/json"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/EduGoGroup/edugo-worker/internal/domain/valueobject"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MaterialDeletedProcessor struct {
	mongodb *mongo.Database
	logger  logger.Logger
}

func NewMaterialDeletedProcessor(mongodb *mongo.Database, logger logger.Logger) *MaterialDeletedProcessor {
	return &MaterialDeletedProcessor{
		mongodb: mongodb,
		logger:  logger,
	}
}

func (p *MaterialDeletedProcessor) EventType() string {
	return "material_deleted"
}

func (p *MaterialDeletedProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.MaterialDeletedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

func (p *MaterialDeletedProcessor) processEvent(ctx context.Context, event dto.MaterialDeletedEvent) error {
	p.logger.Info("processing material deleted", "material_id", event.MaterialID)

	materialID, _ := valueobject.MaterialIDFromString(event.MaterialID)

	// Eliminar summary
	summaryCol := p.mongodb.Collection("material_summaries")
	_, err := summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
	if err != nil {
		p.logger.Error("failed to delete summary", "error", err)
	}

	// Eliminar assessment
	assessmentCol := p.mongodb.Collection("material_assessment_worker")
	_, err = assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
	if err != nil {
		p.logger.Error("failed to delete assessment", "error", err)
	}

	p.logger.Info("material cleanup completed", "material_id", event.MaterialID)
	return nil
}
