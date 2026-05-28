//go:build integration

package integration

import (
	"context"
	"testing"
)

// TestSetupContainers verifica que todos los containers se inicialicen correctamente
func TestSetupContainers(t *testing.T) {
	SkipIfIntegrationTestsDisabled(t)

	manager, cleanup := setupAllContainers(t)
	defer cleanup()

	// Verificar PostgreSQL
	if manager.PostgreSQL() == nil {
		t.Fatal("PostgreSQL container not initialized")
	}
	t.Log("✅ PostgreSQL container inicializado")

	// Verificar MongoDB
	if manager.MongoDB() == nil {
		t.Fatal("MongoDB container not initialized")
	}
	t.Log("✅ MongoDB container inicializado")

	// Verificar RabbitMQ
	if manager.RabbitMQ() == nil {
		t.Fatal("RabbitMQ container not initialized")
	}
	t.Log("✅ RabbitMQ container inicializado")

	t.Log("🎉 Todos los containers inicializados correctamente")
}

// TestSetupPostgres verifica que PostgreSQL se inicialice correctamente
func TestSetupPostgres(t *testing.T) {
	SkipIfIntegrationTestsDisabled(t)

	db, cleanup := setupPostgres(t)
	defer cleanup()

	// Verificar conexión
	var result int
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Failed to query Postgres: %v", err)
	}

	if result != 1 {
		t.Fatalf("Expected 1, got %d", result)
	}

	t.Log("✅ PostgreSQL testcontainer funcionando correctamente")
}

// TestSetupMongoDB verifica que MongoDB se inicialice correctamente
func TestSetupMongoDB(t *testing.T) {
	SkipIfIntegrationTestsDisabled(t)

	db, cleanup := setupMongoDB(t)
	defer cleanup()

	// Verificar conexión listando colecciones
	ctx := context.Background()
	collections, err := db.ListCollectionNames(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to list MongoDB collections: %v", err)
	}

	t.Logf("✅ MongoDB testcontainer funcionando correctamente (collections: %d)", len(collections))
}

// TestSetupRabbitMQ verifica que RabbitMQ se inicialice correctamente
func TestSetupRabbitMQ(t *testing.T) {
	SkipIfIntegrationTestsDisabled(t)

	channel, cleanup := setupRabbitMQ(t)
	defer cleanup()

	// Declarar una cola de prueba
	_, err := channel.QueueDeclare(
		"test_queue", // name
		false,        // durable
		true,         // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	t.Log("✅ RabbitMQ testcontainer funcionando correctamente")
}
