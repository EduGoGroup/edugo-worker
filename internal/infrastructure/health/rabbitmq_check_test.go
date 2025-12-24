package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockRabbitMQChannel es un mock simple de la interfaz RabbitMQChannel
type MockRabbitMQChannel struct {
	closed     bool
	checkDelay time.Duration
}

func (m *MockRabbitMQChannel) IsClosed() bool {
	if m.checkDelay > 0 {
		time.Sleep(m.checkDelay)
	}
	return m.closed
}

func TestRabbitMQCheck_Name(t *testing.T) {
	mockChannel := &MockRabbitMQChannel{}
	check := NewRabbitMQCheckWithChannel(mockChannel, 5*time.Second)

	assert.Equal(t, "rabbitmq", check.Name())
}

func TestRabbitMQCheck_Check_Success(t *testing.T) {
	mockChannel := &MockRabbitMQChannel{
		closed: false,
	}

	check := NewRabbitMQCheckWithChannel(mockChannel, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.NotNil(t, result.Metadata["response_time_ms"])
}

func TestRabbitMQCheck_Check_ChannelClosed(t *testing.T) {
	mockChannel := &MockRabbitMQChannel{
		closed: true,
	}

	check := NewRabbitMQCheckWithChannel(mockChannel, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "channel is closed")
	assert.NotNil(t, result.Timestamp)
}

func TestRabbitMQCheck_Check_WithContext(t *testing.T) {
	mockChannel := &MockRabbitMQChannel{
		closed: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	check := NewRabbitMQCheckWithChannel(mockChannel, 5*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "rabbitmq", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
}

func TestRabbitMQCheck_Check_ResponseTime(t *testing.T) {
	mockChannel := &MockRabbitMQChannel{
		closed:     false,
		checkDelay: 10 * time.Millisecond,
	}

	check := NewRabbitMQCheckWithChannel(mockChannel, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, StatusHealthy, result.Status)
	responseTime := result.Metadata["response_time_ms"].(int64)
	assert.GreaterOrEqual(t, responseTime, int64(10))
}
