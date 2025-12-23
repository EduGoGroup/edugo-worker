package container

import (
	"database/sql"

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

	// Processor Registry
	ProcessorRegistry *processor.Registry

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

	// Crear processors individuales
	materialUploadedProc := processor.NewMaterialUploadedProcessor(cfg.DB, cfg.MongoDB, cfg.Logger)
	materialDeletedProc := processor.NewMaterialDeletedProcessor(cfg.MongoDB, cfg.Logger)
	assessmentAttemptProc := processor.NewAssessmentAttemptProcessor(cfg.Logger)
	studentEnrolledProc := processor.NewStudentEnrolledProcessor(cfg.Logger)

	// Crear ProcessorRegistry y registrar todos los processors
	c.ProcessorRegistry = processor.NewRegistry(cfg.Logger)
	c.ProcessorRegistry.Register(materialUploadedProc)
	c.ProcessorRegistry.Register(processor.NewMaterialReprocessProcessor(materialUploadedProc, cfg.Logger))
	c.ProcessorRegistry.Register(materialDeletedProc)
	c.ProcessorRegistry.Register(assessmentAttemptProc)
	c.ProcessorRegistry.Register(studentEnrolledProc)

	// Inicializar consumer con registry
	c.EventConsumer = consumer.NewEventConsumer(
		c.ProcessorRegistry,
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
			if c.Logger != nil {
				c.Logger.Error("Error cerrando DB", "error", err.Error())
			}
		}
	}
	return nil
}
