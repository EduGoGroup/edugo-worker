package infrastructure

import (
	"context"
	"io"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/bootstrap/adapter"
	"github.com/EduGoGroup/edugo-worker/internal/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestLogger crea un logger silencioso para tests
func createTestLogger() logger.Logger {
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(io.Discard)
	return adapter.NewLoggerAdapter(logrusLogger)
}

func TestFactory_CreatePDFExtractor(t *testing.T) {
	// Arrange
	cfg := config.Config{
		// No requiere configuración específica para PDF
	}
	logger := createTestLogger()
	factory := NewFactory(cfg, logger)

	// Act
	extractor, err := factory.CreatePDFExtractor()

	// Assert
	require.NoError(t, err, "no debería haber error al crear extractor PDF")
	assert.NotNil(t, extractor, "el extractor PDF no debería ser nil")
}

func TestFactory_CreateNLPClient(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		apiKey       string
		expectClient bool
		description  string
	}{
		{
			name:         "SmartFallback sin API key",
			provider:     "openai",
			apiKey:       "",
			expectClient: true,
			description:  "debería crear SmartFallback cuando no hay API key",
		},
		{
			name:         "SmartFallback con API key (OpenAI no implementado)",
			provider:     "openai",
			apiKey:       "sk-test-key",
			expectClient: true,
			description:  "debería crear SmartFallback porque OpenAI no está implementado",
		},
		{
			name:         "SmartFallback por defecto",
			provider:     "",
			apiKey:       "",
			expectClient: true,
			description:  "debería crear SmartFallback cuando no hay provider configurado",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := config.Config{
				NLP: config.NLPConfig{
					Provider: tt.provider,
					APIKey:   tt.apiKey,
				},
			}
			logger := createTestLogger()
			factory := NewFactory(cfg, logger)

			// Act
			client, err := factory.CreateNLPClient()

			// Assert
			require.NoError(t, err, "no debería haber error al crear cliente NLP")
			if tt.expectClient {
				assert.NotNil(t, client, tt.description)
			}
		})
	}
}

func TestFactory_CreateStorageClient_UnsupportedProvider(t *testing.T) {
	// Arrange
	cfg := config.Config{
		Storage: config.StorageConfig{
			Provider: "azure", // Provider no soportado
		},
	}
	logger := createTestLogger()
	factory := NewFactory(cfg, logger)
	ctx := context.Background()

	// Act
	client, err := factory.CreateStorageClient(ctx)

	// Assert
	require.Error(t, err, "debería haber error con provider no soportado")
	assert.Nil(t, client, "el cliente debería ser nil cuando hay error")
	assert.Contains(t, err.Error(), "proveedor de almacenamiento no soportado", "el error debería indicar que el proveedor no está soportado")
	assert.Contains(t, err.Error(), "azure", "el error debería mencionar el proveedor específico")
}
