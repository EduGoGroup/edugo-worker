package bootstrap

import (
	"context"
	"database/sql"

	"github.com/EduGoGroup/edugo-shared/bootstrap"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

// customFactoriesWrapper envuelve las factories de shared y retiene referencias a los tipos concretos
// que necesitamos para los adapters de api-mobile
type customFactoriesWrapper struct {
	// Factories de shared (usamos delegación)
	sharedFactories *bootstrap.Factories

	// Referencias a tipos concretos que necesitamos retener
	sqlDB         *sql.DB
	mongoClient   *mongo.Client
	rabbitChannel *amqp.Channel
	s3Client      *awsS3.Client
	logrusLogger  *logrus.Logger
}

// newCustomFactoriesWrapper crea un wrapper de factories que retiene los tipos concretos
func newCustomFactoriesWrapper(sharedFactories *bootstrap.Factories) *customFactoriesWrapper {
	return &customFactoriesWrapper{
		sharedFactories: sharedFactories,
	}
}

// PostgreSQLFactory wrapper - usa CreateRawConnection para obtener *sql.DB
type customPostgreSQLFactory struct {
	shared bootstrap.PostgreSQLFactory
	sqlDB  **sql.DB // puntero al puntero para poder guardar la referencia
}

func (f *customPostgreSQLFactory) CreateConnection(ctx context.Context, config bootstrap.PostgreSQLConfig) (*gorm.DB, error) {
	return f.shared.CreateConnection(ctx, config)
}

func (f *customPostgreSQLFactory) CreateRawConnection(ctx context.Context, config bootstrap.PostgreSQLConfig) (*sql.DB, error) {
	db, err := f.shared.CreateRawConnection(ctx, config)
	if err != nil {
		return nil, err
	}

	// Guardar referencia para uso posterior
	*f.sqlDB = db

	return db, nil
}

func (f *customPostgreSQLFactory) Ping(ctx context.Context, db *gorm.DB) error {
	return f.shared.Ping(ctx, db)
}

func (f *customPostgreSQLFactory) Close(db *gorm.DB) error {
	return f.shared.Close(db)
}

// MongoDBFactory wrapper - retiene el client
type customMongoDBFactory struct {
	shared      bootstrap.MongoDBFactory
	mongoClient **mongo.Client // puntero al puntero para poder guardar la referencia
}

func (f *customMongoDBFactory) CreateConnection(ctx context.Context, config bootstrap.MongoDBConfig) (*mongo.Client, error) {
	client, err := f.shared.CreateConnection(ctx, config)
	if err != nil {
		return nil, err
	}

	// Guardar referencia para uso posterior
	*f.mongoClient = client

	return client, nil
}

func (f *customMongoDBFactory) GetDatabase(client *mongo.Client, dbName string) *mongo.Database {
	return f.shared.GetDatabase(client, dbName)
}

func (f *customMongoDBFactory) Ping(ctx context.Context, client *mongo.Client) error {
	return f.shared.Ping(ctx, client)
}

func (f *customMongoDBFactory) Close(ctx context.Context, client *mongo.Client) error {
	return f.shared.Close(ctx, client)
}

// RabbitMQFactory wrapper - retiene el channel
type customRabbitMQFactory struct {
	shared  bootstrap.RabbitMQFactory
	channel **amqp.Channel // puntero al puntero para poder guardar la referencia
}

func (f *customRabbitMQFactory) CreateConnection(ctx context.Context, config bootstrap.RabbitMQConfig) (*amqp.Connection, error) {
	return f.shared.CreateConnection(ctx, config)
}

func (f *customRabbitMQFactory) CreateChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := f.shared.CreateChannel(conn)
	if err != nil {
		return nil, err
	}

	// Guardar referencia para uso posterior
	*f.channel = ch

	return ch, nil
}

func (f *customRabbitMQFactory) DeclareQueue(channel *amqp.Channel, queueName string) (amqp.Queue, error) {
	return f.shared.DeclareQueue(channel, queueName)
}

func (f *customRabbitMQFactory) Close(channel *amqp.Channel, conn *amqp.Connection) error {
	return f.shared.Close(channel, conn)
}

// S3Factory wrapper - retiene el cliente
type customS3Factory struct {
	shared   bootstrap.S3Factory
	s3Client **awsS3.Client // puntero al puntero para poder guardar la referencia
}

func (f *customS3Factory) CreateClient(ctx context.Context, config bootstrap.S3Config) (*awsS3.Client, error) {
	client, err := f.shared.CreateClient(ctx, config)
	if err != nil {
		return nil, err
	}

	// Guardar referencia para uso posterior
	*f.s3Client = client

	return client, nil
}

func (f *customS3Factory) CreatePresignClient(client *awsS3.Client) interface{} {
	return f.shared.CreatePresignClient(client)
}

func (f *customS3Factory) ValidateBucket(ctx context.Context, client *awsS3.Client, bucket string) error {
	return f.shared.ValidateBucket(ctx, client, bucket)
}

// LoggerFactory wrapper - retiene el logger
type customLoggerFactory struct {
	shared bootstrap.LoggerFactory
	logger **logrus.Logger // puntero al puntero para poder guardar la referencia
}

func (f *customLoggerFactory) CreateLogger(ctx context.Context, env string, version string) (*logrus.Logger, error) {
	logger, err := f.shared.CreateLogger(ctx, env, version)
	if err != nil {
		return nil, err
	}

	// Guardar referencia para uso posterior
	*f.logger = logger

	return logger, nil
}

// createCustomFactories crea factories personalizadas que retienen los tipos concretos
func createCustomFactories(wrapper *customFactoriesWrapper) *bootstrap.Factories {
	return &bootstrap.Factories{
		Logger: &customLoggerFactory{
			shared: wrapper.sharedFactories.Logger,
			logger: &wrapper.logrusLogger,
		},
		PostgreSQL: &customPostgreSQLFactory{
			shared: wrapper.sharedFactories.PostgreSQL,
			sqlDB:  &wrapper.sqlDB,
		},
		MongoDB: &customMongoDBFactory{
			shared:      wrapper.sharedFactories.MongoDB,
			mongoClient: &wrapper.mongoClient,
		},
		RabbitMQ: &customRabbitMQFactory{
			shared:  wrapper.sharedFactories.RabbitMQ,
			channel: &wrapper.rabbitChannel,
		},
		S3: &customS3Factory{
			shared:   wrapper.sharedFactories.S3,
			s3Client: &wrapper.s3Client,
		},
	}
}
