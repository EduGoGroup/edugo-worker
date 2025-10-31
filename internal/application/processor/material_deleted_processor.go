package processor

import (
	"context"

	"github.com/edugo/worker/internal/application/dto"
	"github.com/edugo/worker/internal/domain/valueobject"
	"github.com/edugo/shared/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

func (p *MaterialDeletedProcessor) Process(ctx context.Context, event dto.MaterialDeletedEvent) error {
	p.logger.Info("processing material deleted", "material_id", event.MaterialID)

	materialID, _ := valueobject.MaterialIDFromString(event.MaterialID)

	// Eliminar summary
	summaryCol := p.mongodb.Collection("material_summaries")
	_, err := summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
	if err != nil {
		p.logger.Error("failed to delete summary", "error", err)
	}

	// Eliminar assessment
	assessmentCol := p.mongodb.Collection("material_assessments")
	_, err = assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()})
	if err != nil {
		p.logger.Error("failed to delete assessment", "error", err)
	}

	p.logger.Info("material cleanup completed", "material_id", event.MaterialID)
	return nil
}
