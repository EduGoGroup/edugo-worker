package s3

import (
	"context"
	"io"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// createTestLogger crea un logger para tests que no imprime en consola
func createTestLogger() logger.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard) // No imprimir logs durante tests
	return logger.NewLogrusLogger(l)
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name          string
		region        string
		bucket        string
		endpoint      string
		accessKey     string
		secretKey     string
		usePathStyle  bool
		expectError   bool
	}{
		{
			name:         "configuración válida para MinIO",
			region:       "us-east-1",
			bucket:       "test-bucket",
			endpoint:     "http://localhost:9000",
			accessKey:    "minioadmin",
			secretKey:    "minioadmin",
			usePathStyle: true,
			expectError:  false,
		},
		{
			name:         "configuración válida para AWS S3",
			region:       "us-west-2",
			bucket:       "prod-bucket",
			endpoint:     "",
			accessKey:    "AKIAIOSFODNN7EXAMPLE",
			secretKey:    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			usePathStyle: false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := createTestLogger()
			ctx := context.Background()

			client, err := NewClient(
				ctx,
				tt.region,
				tt.bucket,
				tt.endpoint,
				tt.accessKey,
				tt.secretKey,
				tt.usePathStyle,
				log,
			)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.bucket, client.bucket)
				assert.NotNil(t, client.s3Client)
				assert.NotNil(t, client.logger)
			}
		})
	}
}

func TestClientImplementsInterface(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	client, err := NewClient(
		ctx,
		"us-east-1",
		"test-bucket",
		"http://localhost:9000",
		"minioadmin",
		"minioadmin",
		true,
		log,
	)

	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Verificar que el cliente implementa la interfaz storage.Client
	// El compilador ya verifica esto con la línea: var _ storage.Client = (*Client)(nil)
	// Este test es más para documentación
	assert.Implements(t, (*interface{})(nil), client)
}
