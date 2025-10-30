//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

// TestContainers contiene todos los contenedores de prueba
type TestContainers struct {
	Postgres *postgres.PostgresContainer
	MongoDB  *mongodb.MongoDBContainer
	RabbitMQ *rabbitmq.RabbitMQContainer
}

// SetupContainers inicia todos los contenedores necesarios para testing
func SetupContainers(t *testing.T) (*TestContainers, func()) {
	ctx := context.Background()

	// PostgreSQL con scripts de inicialización
	t.Log("🐘 Iniciando PostgreSQL testcontainer...")
	pgContainer, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("edugo"),
		postgres.WithUsername("edugo_user"),
		postgres.WithPassword("edugo_pass"),
		postgres.WithInitScripts(
			"../../../scripts/postgresql/01_schema.sql",
			"../../../scripts/postgresql/02_indexes.sql",
		),
	)
	if err != nil {
		t.Fatalf("Failed to start Postgres: %v", err)
	}
	t.Log("✅ PostgreSQL ready")

	// MongoDB
	t.Log("🍃 Iniciando MongoDB testcontainer...")
	mongoContainer, err := mongodb.Run(ctx, "mongo:7.0",
		mongodb.WithUsername("edugo_admin"),
		mongodb.WithPassword("edugo_pass"),
	)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	t.Log("✅ MongoDB ready")

	// RabbitMQ
	t.Log("🐰 Iniciando RabbitMQ testcontainer...")
	rabbitContainer, err := rabbitmq.Run(ctx, "rabbitmq:3.12-management-alpine",
		rabbitmq.WithAdminUsername("edugo_user"),
		rabbitmq.WithAdminPassword("edugo_pass"),
	)
	if err != nil {
		pgContainer.Terminate(ctx)
		mongoContainer.Terminate(ctx)
		t.Fatalf("Failed to start RabbitMQ: %v", err)
	}
	t.Log("✅ RabbitMQ ready")

	containers := &TestContainers{
		Postgres: pgContainer,
		MongoDB:  mongoContainer,
		RabbitMQ: rabbitContainer,
	}

	// Cleanup function
	cleanup := func() {
		t.Log("🧹 Cleaning up testcontainers...")
		pgContainer.Terminate(ctx)
		mongoContainer.Terminate(ctx)
		rabbitContainer.Terminate(ctx)
		t.Log("✅ Testcontainers terminated")
	}

	return containers, cleanup
}

// SetupPostgres inicia solo PostgreSQL
func SetupPostgres(t *testing.T) (*postgres.PostgresContainer, func()) {
	ctx := context.Background()

	t.Log("🐘 Iniciando PostgreSQL testcontainer...")
	pgContainer, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("edugo"),
		postgres.WithUsername("edugo_user"),
		postgres.WithPassword("edugo_pass"),
		postgres.WithInitScripts(
			"../../../scripts/postgresql/01_schema.sql",
			"../../../scripts/postgresql/02_indexes.sql",
		),
	)
	if err != nil {
		t.Fatalf("Failed to start Postgres: %v", err)
	}
	t.Log("✅ PostgreSQL ready")

	cleanup := func() {
		pgContainer.Terminate(ctx)
	}

	return pgContainer, cleanup
}

// SetupMongoDB inicia solo MongoDB
func SetupMongoDB(t *testing.T) (*mongodb.MongoDBContainer, func()) {
	ctx := context.Background()

	t.Log("🍃 Iniciando MongoDB testcontainer...")
	mongoContainer, err := mongodb.Run(ctx, "mongo:7.0",
		mongodb.WithUsername("edugo_admin"),
		mongodb.WithPassword("edugo_pass"),
	)
	if err != nil {
		t.Fatalf("Failed to start MongoDB: %v", err)
	}
	t.Log("✅ MongoDB ready")

	cleanup := func() {
		mongoContainer.Terminate(ctx)
	}

	return mongoContainer, cleanup
}

// SetupRabbitMQ inicia solo RabbitMQ
func SetupRabbitMQ(t *testing.T) (*rabbitmq.RabbitMQContainer, func()) {
	ctx := context.Background()

	t.Log("🐰 Iniciando RabbitMQ testcontainer...")
	rabbitContainer, err := rabbitmq.Run(ctx, "rabbitmq:3.12-management-alpine",
		rabbitmq.WithAdminUsername("edugo_user"),
		rabbitmq.WithAdminPassword("edugo_pass"),
	)
	if err != nil {
		t.Fatalf("Failed to start RabbitMQ: %v", err)
	}
	t.Log("✅ RabbitMQ ready")

	cleanup := func() {
		rabbitContainer.Terminate(ctx)
	}

	return rabbitContainer, cleanup
}
