package health

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestRabbitMQCheck_Name(t *testing.T) {
	// Crear un channel mock (será nil pero suficiente para el test del nombre)
	var channel *amqp.Channel
	check := NewRabbitMQCheck(channel, 3*time.Second)

	assert.Equal(t, "rabbitmq", check.Name())
}

func TestRabbitMQCheck_Check_Success(t *testing.T) {
	// Este test requiere una instancia real de RabbitMQ
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Skipf("RabbitMQ no disponible: %v", err)
	}
	defer func() { _ = conn.Close() }()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Error creando canal: %v", err)
	}
	defer func() { _ = channel.Close() }()

	check := NewRabbitMQCheck(channel, 3*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.NotNil(t, result.Metadata["response_time_ms"])
}

func TestRabbitMQCheck_Check_ChannelClosed(t *testing.T) {
	// Este test verifica el comportamiento cuando el canal está cerrado
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Skipf("RabbitMQ no disponible: %v", err)
	}
	defer func() { _ = conn.Close() }()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Error creando canal: %v", err)
	}

	// Cerrar el canal antes del check
	_ = channel.Close()

	check := NewRabbitMQCheck(channel, 3*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "channel is closed")
	assert.NotNil(t, result.Timestamp)
}

func TestRabbitMQCheck_Check_ConnectionLost(t *testing.T) {
	// Este test verifica el comportamiento cuando la conexión se pierde
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Skipf("RabbitMQ no disponible: %v", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Error creando canal: %v", err)
	}

	// Cerrar la conexión, lo que cerrará el canal
	_ = conn.Close()

	check := NewRabbitMQCheck(channel, 3*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "channel is closed")
	assert.NotNil(t, result.Timestamp)
}

func TestRabbitMQCheck_Check_QuickResponse(t *testing.T) {
	// Test que verifica que el check es rápido cuando el canal está sano
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Skipf("RabbitMQ no disponible: %v", err)
	}
	defer func() { _ = conn.Close() }()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Error creando canal: %v", err)
	}
	defer func() { _ = channel.Close() }()

	check := NewRabbitMQCheck(channel, 3*time.Second)

	start := time.Now()
	result := check.Check(ctx)
	duration := time.Since(start)

	assert.Equal(t, StatusHealthy, result.Status)
	assert.Less(t, duration, 100*time.Millisecond, "El check debería ser muy rápido")

	responseTime, ok := result.Metadata["response_time_ms"].(int64)
	assert.True(t, ok, "response_time_ms debe ser int64")
	assert.Less(t, responseTime, int64(100), "response_time_ms debe ser menor a 100ms")
}
