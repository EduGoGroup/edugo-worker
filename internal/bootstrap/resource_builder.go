package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sharedBootstrap "github.com/EduGoGroup/edugo-shared/bootstrap"
	"github.com/EduGoGroup/edugo-shared/lifecycle"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
	"github.com/EduGoGroup/edugo-worker/internal/client"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/health"
	httpInfra "github.com/EduGoGroup/edugo-worker/internal/infrastructure/http"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/mongo"
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
	MetricsServer     *httpInfra.MetricsServer
	HealthChecker     *health.Checker
}

// ResourceBuilder construye Resources de forma incremental usando el patrón Builder
type ResourceBuilder struct {
	config *config.Config
	ctx    context.Context

	// Recursos de infraestructura base
	logger      logger.Logger
	sqlDB       *sql.DB
	mongoClient *mongo.Client
	mongodb     *mongo.Database
	rabbitConn    *amqp.Connection
	rabbitChannel *amqp.Channel

	// Servicios de infraestructura externa
	storageClient storage.Client
	pdfExtractor  pdf.Extractor
	nlpClient     nlp.Client

	// Recursos de aplicación
	authClient        *client.AuthClient
	processorRegistry *processor.Registry
	metricsServer     *httpInfra.MetricsServer
	healthChecker     *health.Checker

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

	// Guardar referencia
	b.logger = logrusLogger

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
		return b.mongoClient.Disconnect(b.ctx)
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

// WithInfrastructure configura los servicios de infraestructura externa (S3, PDF, NLP)
func (b *ResourceBuilder) WithInfrastructure() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before infrastructure")
		return b
	}

	b.logger.Info("initializing infrastructure services")

	// Crear factory
	factory := infrastructure.NewFactory(*b.config, b.logger)

	// Crear cliente de almacenamiento (S3/MinIO)
	storageClient, err := factory.CreateStorageClient(b.ctx)
	if err != nil {
		b.err = fmt.Errorf("failed to create storage client: %w", err)
		return b
	}
	b.storageClient = storageClient

	// Crear extractor PDF
	pdfExtractor, err := factory.CreatePDFExtractor()
	if err != nil {
		b.err = fmt.Errorf("failed to create PDF extractor: %w", err)
		return b
	}
	b.pdfExtractor = pdfExtractor

	// Crear cliente NLP
	nlpClient, err := factory.CreateNLPClient()
	if err != nil {
		b.err = fmt.Errorf("failed to create NLP client: %w", err)
		return b
	}
	b.nlpClient = nlpClient

	b.logger.Info("✅ Infrastructure services initialized successfully")
	return b
}

// WithProcessors configura el registry de processors
func (b *ResourceBuilder) WithProcessors() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	// Verificar dependencias base
	if b.logger == nil || b.sqlDB == nil || b.mongodb == nil {
		b.err = fmt.Errorf("logger, PostgreSQL and MongoDB required before processors")
		return b
	}

	// Verificar dependencias de infraestructura
	if b.storageClient == nil || b.pdfExtractor == nil || b.nlpClient == nil {
		b.err = fmt.Errorf("infrastructure services required before processors (call WithInfrastructure first)")
		return b
	}

	b.logger.Info("initializing processors")

	// Crear processors individuales
	materialUploadedProc := processor.NewMaterialUploadedProcessor(processor.MaterialUploadedProcessorConfig{
		DB:            b.sqlDB,
		MongoDB:       b.mongodb,
		Logger:        b.logger,
		StorageClient: b.storageClient,
		PDFExtractor:  b.pdfExtractor,
		NLPClient:     b.nlpClient,
	})
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

// WithHealthChecks configura los health checks para las dependencias
func (b *ResourceBuilder) WithHealthChecks() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before health checks")
		return b
	}

	b.logger.Info("initializing health checks")

	// Crear checker
	checker := health.NewChecker()

	// Obtener configuración de health checks con valores por defecto
	healthConfig := b.config.GetHealthConfigWithDefaults()

	// Registrar health check de PostgreSQL si está disponible
	if b.sqlDB != nil {
		pgCheck := health.NewPostgreSQLCheck(b.sqlDB, healthConfig.Timeouts.Postgres)
		checker.Register(pgCheck)
		b.logger.Info("registered PostgreSQL health check")
	}

	// Registrar health check de MongoDB si está disponible
	if b.mongoClient != nil {
		mongoCheck := health.NewMongoDBCheck(b.mongoClient, healthConfig.Timeouts.MongoDB)
		checker.Register(mongoCheck)
		b.logger.Info("registered MongoDB health check")
	}

	// Registrar health check de RabbitMQ si está disponible
	if b.rabbitChannel != nil {
		rabbitCheck := health.NewRabbitMQCheck(b.rabbitChannel, healthConfig.Timeouts.RabbitMQ)
		checker.Register(rabbitCheck)
		b.logger.Info("registered RabbitMQ health check")
	}

	// Guardar referencia
	b.healthChecker = checker

	b.logger.Info("✅ Health checks initialized successfully")
	return b
}

// WithMetricsServer configura el servidor HTTP de métricas Prometheus
func (b *ResourceBuilder) WithMetricsServer() *ResourceBuilder {
	if b.err != nil {
		return b
	}

	if b.logger == nil {
		b.err = fmt.Errorf("logger required before metrics server")
		return b
	}

	// Obtener configuración con valores por defecto
	metricsCfg := b.config.GetMetricsConfigWithDefaults()

	// Si las métricas no están habilitadas, retornar sin hacer nada
	if !metricsCfg.Enabled {
		b.logger.Info("metrics server disabled")
		return b
	}

	b.logger.Info("initializing metrics server", "port", metricsCfg.Port)

	// Crear servidor de métricas con health checker si está disponible
	var metricsServer *httpInfra.MetricsServer
	if b.healthChecker != nil {
		metricsServer = httpInfra.NewMetricsServerWithConfig(httpInfra.MetricsServerConfig{
			Port:          metricsCfg.Port,
			HealthChecker: b.healthChecker,
		})
		b.logger.Info("metrics server configured with health endpoints")
	} else {
		metricsServer = httpInfra.NewMetricsServer(metricsCfg.Port)
	}

	// Iniciar servidor en goroutine
	go func() {
		b.logger.Info("starting metrics server", "port", metricsCfg.Port)
		if err := metricsServer.Start(); err != nil {
			b.logger.Error("metrics server error", "error", err.Error())
		}
	}()

	// Guardar referencia
	b.metricsServer = metricsServer

	// Registrar cleanup
	b.addCleanup(func() error {
		b.logger.Info("shutting down metrics server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return b.metricsServer.Shutdown(ctx)
	})

	b.logger.Info("✅ Metrics server initialized successfully", "endpoint", fmt.Sprintf("http://localhost:%d/metrics", metricsCfg.Port))
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
		MetricsServer:     b.metricsServer,
		HealthChecker:     b.healthChecker,
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
