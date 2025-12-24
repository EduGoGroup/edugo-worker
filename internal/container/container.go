package container

import (
	"database/sql"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/messaging/consumer"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	"go.mongodb.org/mongo-driver/mongo"
)

// Container es el contenedor de dependencias del Worker
type Container struct {
	// Infrastructure
	DB         *sql.DB
	MongoDB    *mongo.Database
	Logger     logger.Logger
	AuthClient *client.AuthClient

	// Infrastructure Services
	StorageClient storage.Client
	PDFExtractor  pdf.Extractor
	NLPClient     nlp.Client

	// Processor Registry
	ProcessorRegistry *processor.Registry

	// Consumer
	EventConsumer *consumer.EventConsumer
}

// ContainerConfig configuración para crear el container
type ContainerConfig struct {
	DB            *sql.DB
	MongoDB       *mongo.Database
	Logger        logger.Logger
	AuthClient    *client.AuthClient
	StorageClient storage.Client
	PDFExtractor  pdf.Extractor
	NLPClient     nlp.Client
}

// NewContainer crea un nuevo container con todas las dependencias
func NewContainer(cfg ContainerConfig) *Container {
	c := &Container{
		DB:            cfg.DB,
		MongoDB:       cfg.MongoDB,
		Logger:        cfg.Logger,
		AuthClient:    cfg.AuthClient,
		StorageClient: cfg.StorageClient,
		PDFExtractor:  cfg.PDFExtractor,
		NLPClient:     cfg.NLPClient,
	}

	// Crear processors individuales
	materialUploadedProc := processor.NewMaterialUploadedProcessor(processor.MaterialUploadedProcessorConfig{
		DB:            cfg.DB,
		MongoDB:       cfg.MongoDB,
		Logger:        cfg.Logger,
		StorageClient: cfg.StorageClient,
		PDFExtractor:  cfg.PDFExtractor,
		NLPClient:     cfg.NLPClient,
	})
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
