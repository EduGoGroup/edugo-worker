package container

import (
	"database/sql"
	"log"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/messaging/consumer"
	"go.mongodb.org/mongo-driver/mongo"
)

// Container es el contenedor de dependencias del Worker
type Container struct {
	// Infrastructure
	DB         *sql.DB
	MongoDB    *mongo.Database
	Logger     logger.Logger
	AuthClient *client.AuthClient

	// Processors
	MaterialUploadedProc  *processor.MaterialUploadedProcessor
	MaterialReprocessProc *processor.MaterialReprocessProcessor
	MaterialDeletedProc   *processor.MaterialDeletedProcessor
	AssessmentAttemptProc *processor.AssessmentAttemptProcessor
	StudentEnrolledProc   *processor.StudentEnrolledProcessor

	// Consumer
	EventConsumer *consumer.EventConsumer
}

// ContainerConfig configuración para crear el container
type ContainerConfig struct {
	DB         *sql.DB
	MongoDB    *mongo.Database
	Logger     logger.Logger
	AuthClient *client.AuthClient
}

// NewContainer crea un nuevo container con todas las dependencias
func NewContainer(cfg ContainerConfig) *Container {
	c := &Container{
		DB:         cfg.DB,
		MongoDB:    cfg.MongoDB,
		Logger:     cfg.Logger,
		AuthClient: cfg.AuthClient,
	}

	// Inicializar processors
	c.MaterialUploadedProc = processor.NewMaterialUploadedProcessor(cfg.DB, cfg.MongoDB, cfg.Logger)
	c.MaterialDeletedProc = processor.NewMaterialDeletedProcessor(cfg.MongoDB, cfg.Logger)
	c.AssessmentAttemptProc = processor.NewAssessmentAttemptProcessor(cfg.Logger)
	c.StudentEnrolledProc = processor.NewStudentEnrolledProcessor(cfg.Logger)
	c.MaterialReprocessProc = processor.NewMaterialReprocessProcessor(c.MaterialUploadedProc, cfg.Logger)

	// Inicializar consumer con routing
	c.EventConsumer = consumer.NewEventConsumer(
		c.MaterialUploadedProc,
		c.MaterialReprocessProc,
		c.MaterialDeletedProc,
		c.AssessmentAttemptProc,
		c.StudentEnrolledProc,
		cfg.Logger,
	)

	return c
}

// NewContainerLegacy crea container con firma legacy (compatibilidad hacia atrás)
func NewContainerLegacy(db *sql.DB, mongodb *mongo.Database, logger logger.Logger) *Container {
	return NewContainer(ContainerConfig{
		DB:      db,
		MongoDB: mongodb,
		Logger:  logger,
	})
}

func (c *Container) Close() error {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			log.Printf("Error cerrando DB: %v", err)
		}
	}
	return nil
}
