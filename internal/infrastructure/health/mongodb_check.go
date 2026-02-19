package health

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoClient define la interfaz para operaciones de MongoDB necesarias para health checks
type MongoClient interface {
	Ping(ctx context.Context, rp *readpref.ReadPref) error
}

// MongoDBCheck implementa HealthCheck para MongoDB
type MongoDBCheck struct {
	client  MongoClient
	timeout time.Duration
}

// NewMongoDBCheck crea un nuevo MongoDB health check
func NewMongoDBCheck(client *mongo.Client, timeout time.Duration) *MongoDBCheck {
	return &MongoDBCheck{
		client:  client,
		timeout: timeout,
	}
}

// NewMongoDBCheckWithClient crea un health check con una interfaz MongoClient
// Útil para testing con mocks
func NewMongoDBCheckWithClient(client MongoClient, timeout time.Duration) *MongoDBCheck {
	return &MongoDBCheck{
		client:  client,
		timeout: timeout,
	}
}

// Name retorna el nombre del health check
func (c *MongoDBCheck) Name() string {
	return "mongodb"
}

// Check ejecuta el health check de MongoDB
func (c *MongoDBCheck) Check(ctx context.Context) CheckResult {
	result := CheckResult{
		Component: c.Name(),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Crear contexto con timeout
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Intentar hacer ping a MongoDB
	start := time.Now()
	err := c.client.Ping(checkCtx, nil)
	duration := time.Since(start)

	result.Metadata["response_time_ms"] = duration.Milliseconds()

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = "failed to ping MongoDB: " + err.Error()
		return result
	}

	result.Status = StatusHealthy
	result.Message = "MongoDB is healthy"
	return result
}
