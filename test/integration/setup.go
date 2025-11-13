//go:build integration

package integration

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/EduGoGroup/edugo-shared/testing/containers"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
)

// setupAllContainers inicia todos los contenedores necesarios (PostgreSQL + MongoDB + RabbitMQ)
func setupAllContainers(t *testing.T) (*containers.Manager, func()) {
	cfg := containers.NewConfig().
		WithPostgreSQL(&containers.PostgresConfig{
			Database: "edugo",
			Username: "edugo_user",
			Password: "edugo_pass",
		}).
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo",
			Username: "edugo_admin",
			Password: "edugo_pass",
		}).
		WithRabbitMQ(&containers.RabbitConfig{
			Username: "edugo_user",
			Password: "edugo_pass",
		}).
		Build()
	
	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}
	
	cleanup := func() {
		// Cleanup es manejado por el manager
	}
	
	return manager, cleanup
}

// setupPostgres inicia solo PostgreSQL
func setupPostgres(t *testing.T) (*sql.DB, func()) {
	ctx := context.Background()
	
	cfg := containers.NewConfig().
		WithPostgreSQL(&containers.PostgresConfig{
			Database: "edugo",
			Username: "edugo_user",
			Password: "edugo_pass",
		}).
		Build()
	
	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}
	
	pg := manager.PostgreSQL()
	if pg == nil {
		t.Fatal("Failed to get PostgreSQL container")
	}
	
	connString, err := pg.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}
	
	db, err := sql.Open("postgres", connString)
	if err != nil {
		t.Fatalf("Failed to connect to Postgres: %v", err)
	}
	
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Failed to ping Postgres: %v", err)
	}
	
	cleanup := func() {
		db.Close()
	}
	
	return db, cleanup
}

// setupMongoDB inicia solo MongoDB
func setupMongoDB(t *testing.T) (*mongo.Database, func()) {
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo",
			Username: "edugo_admin",
			Password: "edugo_pass",
		}).
		Build()
	
	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}
	
	mongoDB := manager.MongoDB()
	if mongoDB == nil {
		t.Fatal("Failed to get MongoDB container")
	}
	
	db := mongoDB.Database()
	
	cleanup := func() {
		// Cleanup es manejado por el manager
	}
	
	return db, cleanup
}

// setupRabbitMQ inicia solo RabbitMQ
func setupRabbitMQ(t *testing.T) (*amqp.Channel, func()) {
	ctx := context.Background()
	
	cfg := containers.NewConfig().
		WithRabbitMQ(&containers.RabbitConfig{
			Username: "edugo_user",
			Password: "edugo_pass",
		}).
		Build()
	
	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}
	
	rabbitMQ := manager.RabbitMQ()
	if rabbitMQ == nil {
		t.Fatal("Failed to get RabbitMQ container")
	}
	
	connURL, err := rabbitMQ.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("Failed to get RabbitMQ connection string: %v", err)
	}
	
	conn, err := amqp.Dial(connURL)
	if err != nil {
		t.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		t.Fatalf("Failed to create channel: %v", err)
	}
	
	cleanup := func() {
		channel.Close()
		conn.Close()
	}
	
	return channel, cleanup
}
