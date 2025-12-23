package container

import (
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/bootstrap/adapter"
	"github.com/sirupsen/logrus"
)

func TestNewContainer(t *testing.T) {
	t.Parallel()

	// Crear logger mock
	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	cfg := ContainerConfig{
		DB:         nil, // En test unitario no necesitamos DB real
		MongoDB:    nil,
		Logger:     logger,
		AuthClient: nil,
	}

	container := NewContainer(cfg)

	if container == nil {
		t.Fatal("expected container to be created")
	}

	if container.Logger == nil {
		t.Error("expected logger to be set")
	}

	if container.ProcessorRegistry == nil {
		t.Error("expected ProcessorRegistry to be created")
	}

	if container.EventConsumer == nil {
		t.Error("expected EventConsumer to be created")
	}

	// Verificar que hay processors registrados
	registeredTypes := container.ProcessorRegistry.RegisteredTypes()
	if len(registeredTypes) == 0 {
		t.Error("expected processors to be registered")
	}

	// Verificar eventos específicos esperados
	expectedEvents := []string{
		"material_uploaded",
		"material_deleted",
		"material_reprocess",
		"assessment_attempt",
		"student_enrolled",
	}

	for _, expectedEvent := range expectedEvents {
		found := false
		for _, event := range registeredTypes {
			if event == expectedEvent {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event %s to be registered", expectedEvent)
		}
	}
}

func TestNewContainer_WithAllDependencies(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	cfg := ContainerConfig{
		DB:         nil,
		MongoDB:    nil,
		Logger:     logger,
		AuthClient: nil,
	}

	container := NewContainer(cfg)

	// Verificar que todas las propiedades están inicializadas correctamente
	if container.DB != cfg.DB {
		t.Error("expected DB to be set from config")
	}

	if container.MongoDB != cfg.MongoDB {
		t.Error("expected MongoDB to be set from config")
	}

	if container.Logger != cfg.Logger {
		t.Error("expected Logger to be set from config")
	}

	if container.AuthClient != cfg.AuthClient {
		t.Error("expected AuthClient to be set from config")
	}
}

func TestNewContainerLegacy(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	container := NewContainerLegacy(nil, nil, logger)

	if container == nil {
		t.Fatal("expected container to be created")
	}

	if container.Logger == nil {
		t.Error("expected logger to be set")
	}

	if container.ProcessorRegistry == nil {
		t.Error("expected ProcessorRegistry to be created")
	}

	if container.EventConsumer == nil {
		t.Error("expected EventConsumer to be created")
	}
}

func TestNewContainerLegacy_ParametersMapping(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	container := NewContainerLegacy(nil, nil, logger)

	// Verificar que los parámetros se mapean correctamente
	if container.DB != nil {
		t.Error("expected DB to be nil")
	}

	if container.MongoDB != nil {
		t.Error("expected MongoDB to be nil")
	}

	if container.Logger != logger {
		t.Error("expected Logger to match input")
	}
}

func TestContainer_Close(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	container := NewContainer(ContainerConfig{
		Logger: logger,
	})

	// Close no debería fallar incluso con DB nil
	err := container.Close()
	if err != nil {
		t.Errorf("expected no error from Close with nil DB, got: %v", err)
	}
}

func TestContainer_Close_WithNilLogger(t *testing.T) {
	t.Parallel()

	container := &Container{
		DB:     nil,
		Logger: nil,
	}

	// Close debería funcionar incluso sin logger
	err := container.Close()
	if err != nil {
		t.Errorf("expected no error from Close, got: %v", err)
	}
}

func TestNewContainer_ProcessorRegistryIntegration(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	cfg := ContainerConfig{
		Logger: logger,
	}

	container := NewContainer(cfg)

	// Verificar que el ProcessorRegistry fue creado con el logger correcto
	if container.ProcessorRegistry == nil {
		t.Fatal("expected ProcessorRegistry to be created")
	}

	// Verificar que los processors se pueden obtener
	registeredTypes := container.ProcessorRegistry.RegisteredTypes()
	if len(registeredTypes) < 5 {
		t.Errorf("expected at least 5 processors, got %d", len(registeredTypes))
	}
}

func TestNewContainer_EventConsumerIntegration(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	logger := adapter.NewLoggerAdapter(logrusLogger)

	cfg := ContainerConfig{
		Logger: logger,
	}

	container := NewContainer(cfg)

	// Verificar que EventConsumer fue creado correctamente
	if container.EventConsumer == nil {
		t.Fatal("expected EventConsumer to be created")
	}

	// El EventConsumer debería tener acceso al ProcessorRegistry
	// Esto se verifica indirectamente ya que EventConsumer requiere registry
	if container.ProcessorRegistry == nil {
		t.Error("expected ProcessorRegistry to exist for EventConsumer")
	}
}
