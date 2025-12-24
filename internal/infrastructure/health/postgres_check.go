package health

import (
	"context"
	"database/sql"
	"time"
)

// PostgreSQLCheck implementa HealthCheck para PostgreSQL
type PostgreSQLCheck struct {
	db      *sql.DB
	timeout time.Duration
}

// NewPostgreSQLCheck crea un nuevo PostgreSQL health check
func NewPostgreSQLCheck(db *sql.DB, timeout time.Duration) *PostgreSQLCheck {
	return &PostgreSQLCheck{
		db:      db,
		timeout: timeout,
	}
}

// Name retorna el nombre del health check
func (c *PostgreSQLCheck) Name() string {
	return "postgresql"
}

// Check ejecuta el health check de PostgreSQL
func (c *PostgreSQLCheck) Check(ctx context.Context) CheckResult {
	result := CheckResult{
		Component: c.Name(),
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Crear contexto con timeout
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Intentar hacer ping a la base de datos
	start := time.Now()
	err := c.db.PingContext(checkCtx)
	duration := time.Since(start)

	result.Metadata["response_time_ms"] = duration.Milliseconds()

	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = "failed to ping PostgreSQL: " + err.Error()
		return result
	}

	// Obtener estadísticas de conexión
	stats := c.db.Stats()
	result.Metadata["open_connections"] = stats.OpenConnections
	result.Metadata["in_use"] = stats.InUse
	result.Metadata["idle"] = stats.Idle
	result.Metadata["wait_count"] = stats.WaitCount
	result.Metadata["max_open_connections"] = stats.MaxOpenConnections

	// Verificar si hay demasiadas conexiones en uso
	if stats.OpenConnections >= stats.MaxOpenConnections {
		result.Status = StatusDegraded
		result.Message = "connection pool at maximum capacity"
		return result
	}

	result.Status = StatusHealthy
	result.Message = "PostgreSQL is healthy"
	return result
}
