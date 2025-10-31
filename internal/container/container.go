package container

import (
	"database/sql"

	"github.com/edugo/worker/internal/application/processor"
	"github.com/edugo/worker/internal/infrastructure/messaging/consumer"
	"github.com/edugo/shared/pkg/logger"
	"go.mongodb.org/mongo-driver/mongo"
)

// Container es el contenedor de dependencias del Worker
type Container struct {
	// Infrastructure
	DB      *sql.DB
	MongoDB *mongo.Database
	Logger  logger.Logger

	// Processors
	MaterialUploadedProc  *processor.MaterialUploadedProcessor
	MaterialReprocessProc *processor.MaterialReprocessProcessor
	MaterialDeletedProc   *processor.MaterialDeletedProcessor
	AssessmentAttemptProc *processor.AssessmentAttemptProcessor
	StudentEnrolledProc   *processor.StudentEnrolledProcessor

	// Consumer
	EventConsumer *consumer.EventConsumer
}

func NewContainer(db *sql.DB, mongodb *mongo.Database, logger logger.Logger) *Container {
	c := &Container{
		DB:      db,
		MongoDB: mongodb,
		Logger:  logger,
	}

	// Inicializar processors
	c.MaterialUploadedProc = processor.NewMaterialUploadedProcessor(db, mongodb, logger)
	c.MaterialDeletedProc = processor.NewMaterialDeletedProcessor(mongodb, logger)
	c.AssessmentAttemptProc = processor.NewAssessmentAttemptProcessor(logger)
	c.StudentEnrolledProc = processor.NewStudentEnrolledProcessor(logger)
	c.MaterialReprocessProc = processor.NewMaterialReprocessProcessor(c.MaterialUploadedProc, logger)

	// Inicializar consumer con routing
	c.EventConsumer = consumer.NewEventConsumer(
		c.MaterialUploadedProc,
		c.MaterialReprocessProc,
		c.MaterialDeletedProc,
		c.AssessmentAttemptProc,
		c.StudentEnrolledProc,
		logger,
	)

	return c
}

func (c *Container) Close() error {
	if c.DB != nil {
		c.DB.Close()
	}
	return nil
}
