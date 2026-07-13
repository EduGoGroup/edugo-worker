//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/EduGoGroup/edugo-shared/testing/containers"
	amqp "github.com/rabbitmq/amqp091-go"
)

// setupRabbitMQ inicia solo RabbitMQ.
//
// Plan 037 (D-037.11): el worker quedó como esqueleto sin Postgres ni Mongo; su
// única dependencia de infraestructura de mensajería es RabbitMQ.
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
