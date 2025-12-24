package health

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockPostgresDB es un mock simple de la interfaz PostgresDB
type MockPostgresDB struct {
	pingError error
	pingDelay time.Duration
	stats     sql.DBStats
}

func (m *MockPostgresDB) PingContext(ctx context.Context) error {
	if m.pingDelay > 0 {
		select {
		case <-time.After(m.pingDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.pingError
}

func (m *MockPostgresDB) Stats() sql.DBStats {
	return m.stats
}

func TestPostgreSQLCheck_Name(t *testing.T) {
	mockDB := &MockPostgresDB{}
	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)

	assert.Equal(t, "postgresql", check.Name())
}

func TestPostgreSQLCheck_Check_Success(t *testing.T) {
	mockDB := &MockPostgresDB{
		pingError: nil,
		stats: sql.DBStats{
			OpenConnections:    5,
			InUse:              2,
			Idle:               3,
			WaitCount:          0,
			MaxOpenConnections: 10,
		},
	}

	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, 5, result.Metadata["open_connections"])
	assert.Equal(t, 2, result.Metadata["in_use"])
	assert.Equal(t, 3, result.Metadata["idle"])
}

func TestPostgreSQLCheck_Check_Failure(t *testing.T) {
	expectedError := errors.New("connection refused")
	mockDB := &MockPostgresDB{
		pingError: expectedError,
	}

	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.Contains(t, result.Message, "connection refused")
	assert.NotNil(t, result.Timestamp)
}

func TestPostgreSQLCheck_Check_Timeout(t *testing.T) {
	mockDB := &MockPostgresDB{
		pingError: nil,
		pingDelay: 200 * time.Millisecond,
	}

	check := NewPostgreSQLCheckWithDB(mockDB, 100*time.Millisecond)
	result := check.Check(context.Background())

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
}

func TestPostgreSQLCheck_Check_ContextCanceled(t *testing.T) {
	mockDB := &MockPostgresDB{
		pingError: nil,
		pingDelay: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
}

func TestPostgreSQLCheck_Check_ConnectionPoolAtCapacity(t *testing.T) {
	mockDB := &MockPostgresDB{
		pingError: nil,
		stats: sql.DBStats{
			OpenConnections:    10,
			InUse:              8,
			Idle:               2,
			WaitCount:          5,
			MaxOpenConnections: 10,
		},
	}

	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusDegraded, result.Status)
	assert.Contains(t, result.Message, "maximum capacity")
	assert.Equal(t, 10, result.Metadata["open_connections"])
	assert.Equal(t, 10, result.Metadata["max_open_connections"])
}

func TestPostgreSQLCheck_Check_ResponseTime(t *testing.T) {
	mockDB := &MockPostgresDB{
		pingError: nil,
		pingDelay: 50 * time.Millisecond,
		stats: sql.DBStats{
			OpenConnections:    5,
			MaxOpenConnections: 10,
		},
	}

	check := NewPostgreSQLCheckWithDB(mockDB, 5*time.Second)
	result := check.Check(context.Background())

	assert.Equal(t, StatusHealthy, result.Status)
	responseTime := result.Metadata["response_time_ms"].(int64)
	assert.GreaterOrEqual(t, responseTime, int64(50))
}
