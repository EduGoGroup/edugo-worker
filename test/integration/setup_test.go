//go:build integration

package integration

import (
	"testing"
)

// TestSetupRabbitMQ verifica que RabbitMQ se inicialice correctamente.
//
// Plan 037 (D-037.11): el worker esqueleto solo depende de RabbitMQ; los
// containers de PostgreSQL y MongoDB se retiraron junto con sus processors.
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
