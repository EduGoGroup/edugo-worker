package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDBCheck_Name(t *testing.T) {
	client, _ := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	check := NewMongoDBCheck(client, 5*time.Second)

	assert.Equal(t, "mongodb", check.Name())
}

func TestMongoDBCheck_Check_Success(t *testing.T) {
	// Este test requiere una instancia real de MongoDB
	// Se ejecutará solo si la variable de entorno INTEGRATION_TESTS está configurada
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Skipf("MongoDB no disponible: %v", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Error desconectando MongoDB: %v", err)
		}
	}()

	check := NewMongoDBCheck(client, 5*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.NotNil(t, result.Metadata["response_time_ms"])
}

func TestMongoDBCheck_Check_Timeout(t *testing.T) {
	// Este test verifica que el timeout funciona correctamente
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	// Intentar conectar a un host que no existe para forzar timeout
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://invalid-host:27017"))
	if err != nil {
		t.Fatalf("Error creando cliente: %v", err)
	}

	check := NewMongoDBCheck(client, 1*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.NotNil(t, result.Timestamp)
}

func TestMongoDBCheck_Check_ContextCanceled(t *testing.T) {
	// Test que verifica el comportamiento cuando el contexto es cancelado
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	client, _ := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	check := NewMongoDBCheck(client, 5*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.NotNil(t, result.Timestamp)
}
