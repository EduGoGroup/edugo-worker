package bootstrap

import (
	"context"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/config"
)

func TestNewResourceBuilder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	builder := NewResourceBuilder(ctx, cfg)

	if builder == nil {
		t.Fatal("expected builder to be created")
	}

	if builder.ctx != ctx {
		t.Error("expected context to be set")
	}

	if builder.config != cfg {
		t.Error("expected config to be set")
	}

	if builder.cleanupFuncs == nil {
		t.Error("expected cleanupFuncs to be initialized")
	}
}

func TestResourceBuilder_ErrorHandling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	// Configurar config inválida para PostgreSQL
	cfg.Database.Postgres.Host = ""

	builder := NewResourceBuilder(ctx, cfg).
		WithLogger().
		WithPostgreSQL()

	if builder.err == nil {
		t.Error("expected error when PostgreSQL config is invalid")
	}

	// Build should return the error
	_, _, err := builder.Build()
	if err == nil {
		t.Error("expected Build to return error")
	}
}

func TestResourceBuilder_DependencyValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	// Intentar crear PostgreSQL sin logger
	builder := NewResourceBuilder(ctx, cfg).
		WithPostgreSQL()

	if builder.err == nil {
		t.Error("expected error when PostgreSQL is called before logger")
	}

	if builder.err.Error() != "logger required before PostgreSQL" {
		t.Errorf("unexpected error message: %v", builder.err)
	}
}

func TestResourceBuilder_Chaining(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	// Test que el chaining funciona
	builder := NewResourceBuilder(ctx, cfg)

	result := builder.
		WithLogger().
		WithAuthClient()

	if result != builder {
		t.Error("expected builder to return itself for chaining")
	}
}

func TestResourceBuilder_CleanupRegistration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	builder := NewResourceBuilder(ctx, cfg).
		WithLogger()

	// Verificar que se registró un cleanup para el logger
	if len(builder.cleanupFuncs) == 0 {
		t.Error("expected cleanup function to be registered")
	}
}

func TestResourceBuilder_BuildRequiresLogger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	builder := NewResourceBuilder(ctx, cfg)

	_, _, err := builder.Build()

	if err == nil {
		t.Error("expected error when building without logger")
	}

	if err.Error() != "logger is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResourceBuilder_CleanupOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := &config.Config{}

	var order []string

	builder := NewResourceBuilder(ctx, cfg)

	// Registrar cleanups en orden específico
	builder.addCleanup(func() error {
		order = append(order, "first")
		return nil
	})

	builder.addCleanup(func() error {
		order = append(order, "second")
		return nil
	})

	builder.addCleanup(func() error {
		order = append(order, "third")
		return nil
	})

	// Ejecutar cleanup
	err := builder.cleanup()
	if err != nil {
		t.Fatalf("unexpected error during cleanup: %v", err)
	}

	// Verificar orden LIFO (último agregado, primero ejecutado)
	expected := []string{"third", "second", "first"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d cleanups, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("cleanup order[%d]: expected %s, got %s", i, v, order[i])
		}
	}
}
