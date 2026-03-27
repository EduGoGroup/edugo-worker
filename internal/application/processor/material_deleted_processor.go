package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	sharedMetrics "github.com/EduGoGroup/edugo-shared/metrics"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/EduGoGroup/edugo-worker/internal/domain/valueobject"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MaterialDeletedProcessor struct {
	mongodb       *mongo.Database
	logger        logger.Logger
	sharedMetrics *sharedMetrics.Metrics
}

func NewMaterialDeletedProcessor(mongodb *mongo.Database, logger logger.Logger, sm *sharedMetrics.Metrics) *MaterialDeletedProcessor {
	return &MaterialDeletedProcessor{
		mongodb:       mongodb,
		logger:        logger,
		sharedMetrics: sm,
	}
}

func (p *MaterialDeletedProcessor) EventType() string {
	return "material_deleted"
}

func (p *MaterialDeletedProcessor) Process(ctx context.Context, payload []byte) error {
	start := time.Now()

	var event dto.MaterialDeletedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		p.recordSharedMetrics(start, err)
		return errors.NewValidationError("invalid event payload")
	}

	err := p.processEvent(ctx, event)
	p.recordSharedMetrics(start, err)
	return err
}

// recordSharedMetrics registra métricas en el facade centralizado (shared/metrics).
func (p *MaterialDeletedProcessor) recordSharedMetrics(start time.Time, err error) {
	if p.sharedMetrics == nil {
		return
	}
	duration := time.Since(start)
	p.sharedMetrics.RecordMessageProcessed("material_deleted", duration, err)
	p.sharedMetrics.RecordBusinessOperation("material", "delete", duration, err)
}

func (p *MaterialDeletedProcessor) processEvent(ctx context.Context, event dto.MaterialDeletedEvent) error {
	p.logger.Info("processing material deleted", "material_id", event.MaterialID)

	materialID, err := valueobject.MaterialIDFromString(event.MaterialID)
	if err != nil {
		return fmt.Errorf("invalid material_id %q: %w", event.MaterialID, err)
	}

	var errs []error

	// Eliminar summary
	summaryCol := p.mongodb.Collection(mongoentities.MaterialSummary{}.CollectionName())
	if _, err := summaryCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()}); err != nil {
		p.logger.Error("failed to delete summary", "material_id", event.MaterialID, "error", err)
		errs = append(errs, fmt.Errorf("delete summary: %w", err))
	}

	// Eliminar assessment
	assessmentCol := p.mongodb.Collection(mongoentities.MaterialAssessment{}.CollectionName())
	if _, err := assessmentCol.DeleteOne(ctx, bson.M{"material_id": materialID.String()}); err != nil {
		p.logger.Error("failed to delete assessment", "material_id", event.MaterialID, "error", err)
		errs = append(errs, fmt.Errorf("delete assessment: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("material cleanup partial failure: %v", errs)
	}

	p.logger.Info("material cleanup completed", "material_id", event.MaterialID)
	return nil
}
