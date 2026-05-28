package health

import (
	"database/sql"
	"time"

	sharedh "github.com/EduGoGroup/edugo-shared/health"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Status = sharedh.Status
type CheckResult = sharedh.CheckResult
type HealthCheck = sharedh.HealthCheck
type Checker = sharedh.Checker

type PostgresDB = sharedh.PostgresDB
type PostgreSQLCheck = sharedh.PostgreSQLCheck
type MongoClient = sharedh.MongoClient
type MongoDBCheck = sharedh.MongoDBCheck
type RabbitMQChannel = sharedh.RabbitMQChannel
type RabbitMQCheck = sharedh.RabbitMQCheck

const (
	StatusHealthy   = sharedh.StatusHealthy
	StatusUnhealthy = sharedh.StatusUnhealthy
	StatusDegraded  = sharedh.StatusDegraded
)

// NewChecker creates a new health Checker.
func NewChecker() *Checker {
	return sharedh.NewChecker()
}

// NewPostgreSQLCheck creates a new PostgreSQL health check.
func NewPostgreSQLCheck(db *sql.DB, timeout time.Duration) *PostgreSQLCheck {
	return sharedh.NewPostgreSQLCheck(db, timeout)
}

// NewPostgreSQLCheckWithDB creates a PostgreSQL health check using the PostgresDB interface.
func NewPostgreSQLCheckWithDB(db PostgresDB, timeout time.Duration) *PostgreSQLCheck {
	return sharedh.NewPostgreSQLCheckWithDB(db, timeout)
}

// NewMongoDBCheck creates a new MongoDB health check.
func NewMongoDBCheck(client *mongo.Client, timeout time.Duration) *MongoDBCheck {
	return sharedh.NewMongoDBCheck(client, timeout)
}

// NewMongoDBCheckWithClient creates a MongoDB health check using the MongoClient interface.
func NewMongoDBCheckWithClient(client MongoClient, timeout time.Duration) *MongoDBCheck {
	return sharedh.NewMongoDBCheckWithClient(client, timeout)
}

// NewRabbitMQCheck creates a new RabbitMQ health check.
func NewRabbitMQCheck(channel *amqp.Channel, timeout time.Duration) *RabbitMQCheck {
	return sharedh.NewRabbitMQCheck(channel, timeout)
}

// NewRabbitMQCheckWithChannel creates a RabbitMQ health check using the RabbitMQChannel interface.
func NewRabbitMQCheckWithChannel(channel RabbitMQChannel, timeout time.Duration) *RabbitMQCheck {
	return sharedh.NewRabbitMQCheckWithChannel(channel, timeout)
}
