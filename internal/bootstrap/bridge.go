package bootstrap

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/EduGoGroup/edugo-worker/internal/bootstrap/adapter"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	sharedBootstrap "github.com/EduGoGroup/edugo-shared/bootstrap"
	"github.com/EduGoGroup/edugo-shared/lifecycle"
	"github.com/EduGoGroup/edugo-shared/logger"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	gormLogger "gorm.io/gorm/logger"
)

// Resources contiene todos los recursos inicializados para el worker
type Resources struct {
	Logger           logger.Logger
	PostgreSQL       *sql.DB
	MongoDB          *mongo.Database
	RabbitMQChannel  *amqp.Channel
	LifecycleManager *lifecycle.Manager
}

// bridgeToSharedBootstrap conecta shared/bootstrap con worker
func bridgeToSharedBootstrap(
	ctx context.Context,
	cfg *config.Config,
) (*Resources, func() error, error) {
	// 1. Configurar logger GORM
	gormLogLevel := gormLogger.Silent
	if cfg.Logging.Level == "debug" {
		gormLogLevel = gormLogger.Info
	}
	gormLog := gormLogger.Default.LogMode(gormLogLevel)

	// 2. Crear factories de shared
	sharedFactories := &sharedBootstrap.Factories{
		Logger:     sharedBootstrap.NewDefaultLoggerFactory(),
		PostgreSQL: sharedBootstrap.NewDefaultPostgreSQLFactory(gormLog),
		MongoDB:    sharedBootstrap.NewDefaultMongoDBFactory(),
		RabbitMQ:   sharedBootstrap.NewDefaultRabbitMQFactory(),
	}

	// 3. Crear wrapper para retener tipos concretos
	wrapper := newCustomFactoriesWrapper(sharedFactories)
	customFactories := createCustomFactories(wrapper)

	// 4. Crear lifecycle manager temporal
	lifecycleManager := lifecycle.NewManager(nil)

	// 5. Config para bootstrap
	bootstrapConfig := struct {
		Environment string
		PostgreSQL  sharedBootstrap.PostgreSQLConfig
		MongoDB     sharedBootstrap.MongoDBConfig
		RabbitMQ    sharedBootstrap.RabbitMQConfig
	}{
		Environment: "production",
		PostgreSQL: sharedBootstrap.PostgreSQLConfig{
			Host:     cfg.Database.Postgres.Host,
			Port:     cfg.Database.Postgres.Port,
			User:     cfg.Database.Postgres.User,
			Password: cfg.Database.Postgres.Password,
			Database: cfg.Database.Postgres.Database,
			SSLMode:  cfg.Database.Postgres.SSLMode,
		},
		MongoDB: sharedBootstrap.MongoDBConfig{
			URI:      cfg.Database.MongoDB.URI,
			Database: cfg.Database.MongoDB.Database,
		},
		RabbitMQ: sharedBootstrap.RabbitMQConfig{
			URL: cfg.Messaging.RabbitMQ.URL,
		},
	}

	// 6. Llamar shared/bootstrap
	_, err := sharedBootstrap.Bootstrap(
		ctx,
		bootstrapConfig,
		customFactories,
		lifecycleManager,
		sharedBootstrap.WithRequiredResources("logger", "postgresql", "mongodb", "rabbitmq"),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to bootstrap: %w", err)
	}

	// 7. Crear logger adapter
	if wrapper.logrusLogger == nil {
		return nil, nil, fmt.Errorf("logger not initialized")
	}
	loggerAdapter := adapter.NewLoggerAdapter(wrapper.logrusLogger)

	// 8. Crear lifecycle con logger
	lifecycleWithLogger := lifecycle.NewManager(loggerAdapter)

	// 9. Construir Resources
	resources := &Resources{
		Logger:           loggerAdapter,
		PostgreSQL:       wrapper.sqlDB,
		MongoDB:          wrapper.mongoClient.Database(cfg.Database.MongoDB.Database),
		RabbitMQChannel:  wrapper.rabbitChannel,
		LifecycleManager: lifecycleWithLogger,
	}

	// 10. Cleanup
	cleanup := func() error {
		resources.Logger.Info("starting worker cleanup")
		err := lifecycleWithLogger.Cleanup()
		if err != nil {
			resources.Logger.Error("cleanup with errors", "error", err.Error())
			return err
		}
		resources.Logger.Info("worker cleanup completed")
		return nil
	}

	return resources, cleanup, nil
}
