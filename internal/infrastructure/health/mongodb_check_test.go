package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MockMongoClient es un mock simple de la interfaz MongoClient
type MockMongoClient struct {
	pingError error
	pingDelay time.Duration
}

func (m *MockMongoClient) Ping(ctx context.Context, rp *readpref.ReadPref) error {
	if m.pingDelay > 0 {
		select {
		case <-time.After(m.pingDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.pingError
}

func TestMongoDBCheck_Name(t *testing.T) {
	mockClient := &MockMongoClient{}
	check := NewMongoDBCheckWithClient(mockClient, 5*time.Second)

	assert.Equal(t, "mongodb", check.Name())
}

func TestMongoDBCheck_Check_Success(t *testing.T) {
	mockClient := &MockMongoClient{
		pingError: nil,
	}

	check := NewMongoDBCheckWithClient(mockClient, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.NotNil(t, result.Metadata["response_time_ms"])
}

func TestMongoDBCheck_Check_Failure(t *testing.T) {
	expectedError := errors.New("connection refused")
	mockClient := &MockMongoClient{
		pingError: expectedError,
	}

	check := NewMongoDBCheckWithClient(mockClient, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.Contains(t, result.Message, "connection refused")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
}

func TestMongoDBCheck_Check_Timeout(t *testing.T) {
	mockClient := &MockMongoClient{
		pingError: nil,
		pingDelay: 200 * time.Millisecond, // Delay mayor que el timeout
	}

	check := NewMongoDBCheckWithClient(mockClient, 100*time.Millisecond)
	result := check.Check(context.Background())

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.NotNil(t, result.Timestamp)
}

func TestMongoDBCheck_Check_ContextCanceled(t *testing.T) {
	mockClient := &MockMongoClient{
		pingError: nil,
		pingDelay: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	check := NewMongoDBCheckWithClient(mockClient, 5*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "mongodb", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.NotNil(t, result.Timestamp)
}

func TestMongoDBCheck_Check_ResponseTime(t *testing.T) {
	mockClient := &MockMongoClient{
		pingError: nil,
		pingDelay: 50 * time.Millisecond,
	}

	check := NewMongoDBCheckWithClient(mockClient, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, StatusHealthy, result.Status)
	responseTime := result.Metadata["response_time_ms"].(int64)
	assert.GreaterOrEqual(t, responseTime, int64(50))
}
