package processor

import (
	"context"
	"database/sql"

	"github.com/edugo/worker/internal/application/dto"
	"github.com/edugo/worker/internal/domain/valueobject"
	"github.com/edugo/shared/pkg/database/postgres"
	"github.com/edugo/shared/pkg/errors"
	"github.com/edugo/shared/pkg/logger"
	"github.com/edugo/shared/pkg/types/enum"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MaterialUploadedProcessor procesa eventos de material subido
type MaterialUploadedProcessor struct {
	db      *sql.DB
	mongodb *mongo.Database
	logger  logger.Logger
}

func NewMaterialUploadedProcessor(db *sql.DB, mongodb *mongo.Database, logger logger.Logger) *MaterialUploadedProcessor {
	return &MaterialUploadedProcessor{
		db:      db,
		mongodb: mongodb,
		logger:  logger,
	}
}

func (p *MaterialUploadedProcessor) Process(ctx context.Context, event dto.MaterialUploadedEvent) error {
	p.logger.Info("processing material uploaded",
		"material_id", event.MaterialID,
		"s3_key", event.S3Key,
	)

	materialID, err := valueobject.MaterialIDFromString(event.MaterialID)
	if err != nil {
		return errors.NewValidationError("invalid material_id")
	}

	// Usar transacción de shared/database/postgres
	err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
		// 1. Actualizar estado a processing
		_, err := tx.ExecContext(ctx,
			"UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
			enum.ProcessingStatusProcessing.String(),
			materialID.String(),
		)
		if err != nil {
			return err
		}

		// 2. Simular extracción de texto PDF (en prod usar PDF library)
		p.logger.Debug("extracting PDF text", "s3_key", event.S3Key)

		// 3. Simular generación de resumen con OpenAI (en prod usar OpenAI API)
		p.logger.Debug("generating summary with AI")

		// 4. Guardar resumen en MongoDB
		summaryCollection := p.mongodb.Collection("material_summaries")
		summary := bson.M{
			"material_id":  event.MaterialID,
			"main_ideas":   []string{"Idea 1", "Idea 2", "Idea 3"},
			"key_concepts": bson.M{"concept1": "definition1"},
			"sections":     []bson.M{},
			"glossary":     bson.M{},
			"created_at":   event.Timestamp,
		}

		_, err = summaryCollection.InsertOne(ctx, summary)
		if err != nil {
			return err
		}

		// 5. Simular generación de quiz con IA
		p.logger.Debug("generating quiz with AI")

		// 6. Guardar quiz en MongoDB
		assessmentCollection := p.mongodb.Collection("material_assessments")
		assessment := bson.M{
			"material_id": event.MaterialID,
			"questions": []bson.M{
				{
					"id":            "q1",
					"question_text": "Pregunta de ejemplo",
					"question_type": "multiple_choice",
					"options":       []string{"A", "B", "C", "D"},
					"correct_answer": "A",
					"explanation":   "Explicación",
				},
			},
			"created_at": event.Timestamp,
		}

		_, err = assessmentCollection.InsertOne(ctx, assessment)
		if err != nil {
			return err
		}

		// 7. Actualizar estado a completed
		_, err = tx.ExecContext(ctx,
			"UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
			enum.ProcessingStatusCompleted.String(),
			materialID.String(),
		)

		return err
	})

	if err != nil {
		p.logger.Error("processing failed", "error", err, "material_id", event.MaterialID)
		return errors.NewInternalError("processing failed", err)
	}

	p.logger.Info("material processing completed", "material_id", event.MaterialID)
	return nil
}
