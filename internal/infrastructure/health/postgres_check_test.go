package health

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestPostgreSQLCheck_Name(t *testing.T) {
	// Crear una DB mock (no conectada)
	db := &sql.DB{}
	check := NewPostgreSQLCheck(db, 3*time.Second)

	assert.Equal(t, "postgresql", check.Name())
}

func TestPostgreSQLCheck_Check_Success(t *testing.T) {
	// Este test requiere una instancia real de PostgreSQL
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", "host=localhost port=5432 user=edugo_user password=edugo_password dbname=edugo sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL no disponible: %v", err)
	}
	defer db.Close()

	// Verificar que la conexión funciona
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("No se puede conectar a PostgreSQL: %v", err)
	}

	check := NewPostgreSQLCheck(db, 3*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Contains(t, result.Message, "healthy")
	assert.NotNil(t, result.Timestamp)
	assert.NotNil(t, result.Metadata)
	assert.NotNil(t, result.Metadata["response_time_ms"])
	assert.NotNil(t, result.Metadata["open_connections"])
	assert.NotNil(t, result.Metadata["in_use"])
	assert.NotNil(t, result.Metadata["idle"])
}

func TestPostgreSQLCheck_Check_ConnectionPoolDegraded(t *testing.T) {
	// Este test verifica el estado degradado cuando el pool está al máximo
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", "host=localhost port=5432 user=edugo_user password=edugo_password dbname=edugo sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL no disponible: %v", err)
	}
	defer db.Close()

	// Verificar que la conexión funciona
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("No se puede conectar a PostgreSQL: %v", err)
	}

	// Configurar un pool muy pequeño
	db.SetMaxOpenConns(1)

	// Abrir una conexión para saturar el pool
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("Error abriendo conexión: %v", err)
	}
	defer conn.Close()

	check := NewPostgreSQLCheck(db, 3*time.Second)

	// Crear un contexto con timeout corto para evitar espera larga
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result := check.Check(checkCtx)

	assert.Equal(t, "postgresql", result.Component)
	// Puede ser degradado si alcanza el máximo, o unhealthy si falla el ping
	assert.True(t, result.Status == StatusDegraded || result.Status == StatusUnhealthy)
	assert.NotNil(t, result.Timestamp)
}

func TestPostgreSQLCheck_Check_Timeout(t *testing.T) {
	// Este test verifica que el timeout funciona correctamente
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	// Intentar conectar a un host que no existe para forzar timeout
	db, err := sql.Open("postgres", "host=invalid-host port=5432 user=test dbname=test sslmode=disable connect_timeout=1")
	if err != nil {
		t.Fatalf("Error creando conexión: %v", err)
	}
	defer db.Close()

	check := NewPostgreSQLCheck(db, 1*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "failed to ping")
	assert.NotNil(t, result.Timestamp)
}

func TestPostgreSQLCheck_Check_ContextCanceled(t *testing.T) {
	// Test que verifica el comportamiento cuando el contexto es cancelado
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	db, _ := sql.Open("postgres", "host=localhost port=5432 user=test dbname=test sslmode=disable")
	defer db.Close()

	check := NewPostgreSQLCheck(db, 3*time.Second)
	result := check.Check(ctx)

	assert.Equal(t, "postgresql", result.Component)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.NotNil(t, result.Timestamp)
}
