package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	sharedBootstrap "github.com/EduGoGroup/edugo-shared/bootstrap"
	"github.com/EduGoGroup/edugo-shared/lifecycle"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/bootstrap/adapter"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	gormLogger "gorm.io/gorm/logger"
)

// Resources contiene todos los recursos inicializados para el worker
type Resources struct {
	Logger            logger.Logger
	PostgreSQL        *sql.DB
	MongoDB           *mongo.Database
	RabbitMQChannel   *amqp.Channel
	AuthClient        *client.AuthClient
	LifecycleManager  *lifecycle.Manager
	ProcessorRegistry *processor.Registry
}

// ResourceBuilder construye Resources de forma incremental usando el patrón Builder
type ResourceBuilder struct {
	config *config.Config
	ctx    context.Context

	// Recursos de infraestructura
	logger        logger.Logger
	logrusLogger  *logrus.Logger
	sqlDB         *sql.DB
	mongoClient   *mongo.Client
	mongodb       *mongo.Database
	rabbitConn    *amqp.Connection
	rabbitChannel *amqp.Channel

	// Recursos de aplicación
	authClient        *client.AuthClient
	processorRegistry *processor.Registry

	// Lifecycle
	lifecycleManager *lifecycle.Manager
	cleanupFuncs     []func() error

	// Control de errores
	err error
}

// NewResourceBuilder crea un nuevo builder de recursos
func NewResourceBuilder(ctx context.Context, cfg *config.Config) *ResourceBuilder {
	return &ResourceBuilder{
		config:       cfg,
		ctx:          ctx,
		cleanupFuncs: make([]func() error, 0),
	}
}

// WithLogger configura el logger (debe ser llamado primero)
func (b *ResourceBuilder) WithLogger() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	// Crear logger usando shared/bootstrap
	loggerFactory := sharedBootstrap.NewDefaultLoggerFactory()
	logrusLogger, err := loggerFactory.CreateLogger(
		b.ctx,
		"production",
		"v1.0.0",
	)
	if err != nil {
		b.err = fmt.Errorf("failed to create logger: %w", err)
		return b
	}

	// Guardar referencias
	b.logrusLogger = logrusLogger
	b.logger = adapter.NewLoggerAdapter(logrusLogger)

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("syncing logger")
		return b.logger.Sync()
	})

	b.logger.Info("✅ Logger initialized successfully")
	return b
}

// WithPostgreSQL configura la conexión a PostgreSQL
func (b *ResourceBuilder) WithPostgreSQL() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before PostgreSQL")
		return b
	}

	b.logger.Info("connecting to PostgreSQL",
		"host", b.config.Database.Postgres.Host,
		"database", b.config.Database.Postgres.Database,
	)

	// Configurar GORM logger
	gormLogLevel := gormLogger.Silent
	if b.config.Logging.Level == "debug" {
		gormLogLevel = gormLogger.Info
	}
	gormLog := gormLogger.Default.LogMode(gormLogLevel)

	// Crear factory y conexión
	pgFactory := sharedBootstrap.NewDefaultPostgreSQLFactory(gormLog)
	sqlDB, err := pgFactory.CreateRawConnection(b.ctx, sharedBootstrap.PostgreSQLConfig{
		Host:     b.config.Database.Postgres.Host,
		Port:     b.config.Database.Postgres.Port,
		User:     b.config.Database.Postgres.User,
		Password: b.config.Database.Postgres.Password,
		Database: b.config.Database.Postgres.Database,
		SSLMode:  b.config.Database.Postgres.SSLMode,
	})
	if err != nil {
		b.err = fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		return b
	}

	// Guardar referencia
	b.sqlDB = sqlDB

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("closing PostgreSQL connection")
		return b.sqlDB.Close()
	})

	b.logger.Info("✅ PostgreSQL connected successfully")
	return b
}

// WithMongoDB configura la conexión a MongoDB
func (b *ResourceBuilder) WithMongoDB() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before MongoDB")
		return b
	}

	b.logger.Info("connecting to MongoDB",
		"database", b.config.Database.MongoDB.Database,
	)

	// Crear factory y cliente
	mongoFactory := sharedBootstrap.NewDefaultMongoDBFactory()
	mongoClient, err := mongoFactory.CreateConnection(b.ctx, sharedBootstrap.MongoDBConfig{
		URI:      b.config.Database.MongoDB.URI,
		Database: b.config.Database.MongoDB.Database,
	})
	if err != nil {
		b.err = fmt.Errorf("failed to connect to MongoDB: %w", err)
		return b
	}

	// Obtener database
	mongoDB := mongoFactory.GetDatabase(mongoClient, b.config.Database.MongoDB.Database)

	// Guardar referencias
	b.mongoClient = mongoClient
	b.mongodb = mongoDB

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("closing MongoDB connection")
		return b.mongoClient.Disconnect(context.Background())
	})

	b.logger.Info("✅ MongoDB connected successfully")
	return b
}

// WithRabbitMQ configura la conexión a RabbitMQ
func (b *ResourceBuilder) WithRabbitMQ() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before RabbitMQ")
		return b
	}

	b.logger.Info("connecting to RabbitMQ")

	// Crear factory
	rabbitFactory := sharedBootstrap.NewDefaultRabbitMQFactory()

	// Crear conexión
	conn, err := rabbitFactory.CreateConnection(b.ctx, sharedBootstrap.RabbitMQConfig{
		URL: b.config.Messaging.RabbitMQ.URL,
	})
	if err != nil {
		b.err = fmt.Errorf("failed to connect to RabbitMQ: %w", err)
		return b
	}

	// Crear channel
	channel, err := rabbitFactory.CreateChannel(conn)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			b.logger.Error("failed to close RabbitMQ connection after channel error", "error", closeErr.Error())
		}
		b.err = fmt.Errorf("failed to create RabbitMQ channel: %w", err)
		return b
	}

	// Guardar referencias
	b.rabbitConn = conn
	b.rabbitChannel = channel

	// Registrar cleanup (orden inverso: channel antes que connection)
	b.addCleanup(func() error {
		b.logger.Info("closing RabbitMQ channel")
		if err := b.rabbitChannel.Close(); err != nil {
			return fmt.Errorf("failed to close RabbitMQ channel: %w", err)
		}
		b.logger.Info("closing RabbitMQ connection")
		if err := b.rabbitConn.Close(); err != nil {
			return fmt.Errorf("failed to close RabbitMQ connection: %w", err)
		}
		return nil
	})

	b.logger.Info("✅ RabbitMQ connected successfully")
	return b
}

// WithAuthClient configura el cliente de autenticación
func (b *ResourceBuilder) WithAuthClient() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	apiAdminCfg := b.config.GetAPIAdminConfigWithDefaults()
	b.authClient = client.NewAuthClient(client.AuthClientConfig{
		BaseURL:      apiAdminCfg.BaseURL,
		Timeout:      apiAdminCfg.Timeout,
		CacheTTL:     apiAdminCfg.CacheTTL,
		CacheEnabled: apiAdminCfg.CacheEnabled,
		MaxBulkSize:  apiAdminCfg.MaxBulkSize,
	})

	if b.logger != nil {
		b.logger.Info("✅ AuthClient initialized successfully",
			"base_url", apiAdminCfg.BaseURL,
			"cache_enabled", apiAdminCfg.CacheEnabled,
		)
	}

	return b
}

// WithProcessors configura el registry de processors
func (b *ResourceBuilder) WithProcessors() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	// Verificar dependencias
	if b.logger == nil || b.sqlDB == nil || b.mongodb == nil {
		b.err = fmt.Errorf("logger, PostgreSQL and MongoDB required before processors")
		return b
	}

	b.logger.Info("initializing processors")

	// Crear processors individuales
	materialUploadedProc := processor.NewMaterialUploadedProcessor(
		b.sqlDB,
		b.mongodb,
		b.logger,
	)
	materialDeletedProc := processor.NewMaterialDeletedProcessor(
		b.mongodb,
		b.logger,
	)
	assessmentAttemptProc := processor.NewAssessmentAttemptProcessor(b.logger)
	studentEnrolledProc := processor.NewStudentEnrolledProcessor(b.logger)

	// Crear registry
	registry := processor.NewRegistry(b.logger)

	// Registrar processors
	registry.Register(materialUploadedProc)
	registry.Register(processor.NewMaterialReprocessProcessor(materialUploadedProc, b.logger))
	registry.Register(materialDeletedProc)
	registry.Register(assessmentAttemptProc)
	registry.Register(studentEnrolledProc)

	// Guardar referencia
	b.processorRegistry = registry

	b.logger.Info("✅ Processors registered successfully", "count", registry.Count())
	return b
}

// Build construye y retorna Resources con su función de cleanup
func (b *ResourceBuilder) Build() (*Resources, func() error, error) {
	// Verificar si hubo errores durante la construcción
	if b.err != nil {
		// Ejecutar cleanup de recursos parcialmente inicializados
		if cleanupErr := b.cleanup(); cleanupErr != nil {
			// Log cleanup error pero retornar el error original
			return nil, nil, fmt.Errorf("%w (cleanup also failed: %v)", b.err, cleanupErr)
		}
		return nil, nil, b.err
	}

	// Verificar que todos los recursos requeridos están inicializados
	if b.logger == nil {
		return nil, nil, fmt.Errorf("logger is required")
	}

	// Crear lifecycle manager con logger
	b.lifecycleManager = lifecycle.NewManager(b.logger)

	// Construir Resources
	resources := &Resources{
		Logger:            b.logger,
		PostgreSQL:        b.sqlDB,
		MongoDB:           b.mongodb,
		RabbitMQChannel:   b.rabbitChannel,
		AuthClient:        b.authClient,
		LifecycleManager:  b.lifecycleManager,
		ProcessorRegistry: b.processorRegistry,
	}

	// Crear función de cleanup
	cleanup := func() error {
		return b.cleanup()
	}

	b.logger.Info("✅ All resources initialized successfully")
	return resources, cleanup, nil
}

// addCleanup registra una función de cleanup
// Los cleanups se agregan al inicio para ejecutar en orden inverso (LIFO)
func (b *ResourceBuilder) addCleanup(fn func() error) {
	b.cleanupFuncs = append([]func() error{fn}, b.cleanupFuncs...)
}

// cleanup ejecuta todas las funciones de cleanup en orden LIFO
func (b *ResourceBuilder) cleanup() error {
	if b.logger != nil {
		b.logger.Info("starting resource cleanup")
	}

	var errors []error

	// Ejecutar cleanups en orden (LIFO - último creado, primero cerrado)
	for i, cleanupFn := range b.cleanupFuncs {
		if err := cleanupFn(); err != nil {
			errors = append(errors, fmt.Errorf("cleanup %d failed: %w", i, err))
		}
	}

	if len(errors) > 0 {
		errMsg := fmt.Sprintf("cleanup had %d errors", len(errors))
		if b.logger != nil {
			for _, err := range errors {
				b.logger.Error("cleanup error", "error", err.Error())
			}
			b.logger.Error(errMsg)
		}
		return fmt.Errorf("%s: %v", errMsg, errors)
	}

	if b.logger != nil {
		b.logger.Info("✅ Resource cleanup completed successfully")
	}

	return nil
}
