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
		name         string
		region       string
		bucket       string
		endpoint     string
		accessKey    string
		secretKey    string
		usePathStyle bool
		expectError  bool
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

func TestClient_validateFileExtension(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	client, err := NewClient(ctx, "us-east-1", "test-bucket", "", "key", "secret", false, log)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		key         string
		expectError bool
	}{
		{
			name:        "extensión .pdf válida",
			key:         "documents/material.pdf",
			expectError: false,
		},
		{
			name:        "extensión .PDF mayúsculas válida",
			key:         "documents/MATERIAL.PDF",
			expectError: false,
		},
		{
			name:        "extensión .Pdf mixta válida",
			key:         "documents/Material.Pdf",
			expectError: false,
		},
		{
			name:        "extensión .docx inválida",
			key:         "documents/material.docx",
			expectError: true,
		},
		{
			name:        "extensión .txt inválida",
			key:         "documents/material.txt",
			expectError: true,
		},
		{
			name:        "sin extensión inválida",
			key:         "documents/material",
			expectError: true,
		},
		{
			name:        "ruta completa con .pdf válida",
			key:         "uploads/2025/12/24/abc123-material.pdf",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateFileExtension(tt.key)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "extensión de archivo no permitida")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_validateFileSize(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	client, err := NewClient(ctx, "us-east-1", "test-bucket", "", "key", "secret", false, log)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		size        int64
		key         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "tamaño válido 1MB",
			size:        1024 * 1024,
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "tamaño válido 50MB",
			size:        50 * 1024 * 1024,
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "tamaño válido máximo 100MB",
			size:        100 * 1024 * 1024,
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "tamaño mínimo válido 1KB",
			size:        1024,
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "archivo demasiado pequeño 512 bytes",
			size:        512,
			key:         "file.pdf",
			expectError: true,
			errorMsg:    "demasiado pequeño",
		},
		{
			name:        "archivo vacío 0 bytes",
			size:        0,
			key:         "file.pdf",
			expectError: true,
			errorMsg:    "demasiado pequeño",
		},
		{
			name:        "archivo demasiado grande 101MB",
			size:        101 * 1024 * 1024,
			key:         "file.pdf",
			expectError: true,
			errorMsg:    "demasiado grande",
		},
		{
			name:        "archivo demasiado grande 200MB",
			size:        200 * 1024 * 1024,
			key:         "file.pdf",
			expectError: true,
			errorMsg:    "demasiado grande",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateFileSize(tt.size, tt.key)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_validateContentType(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	client, err := NewClient(ctx, "us-east-1", "test-bucket", "", "key", "secret", false, log)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		contentType string
		key         string
		expectError bool
	}{
		{
			name:        "content type application/pdf válido",
			contentType: "application/pdf",
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "content type con charset válido",
			contentType: "application/pdf; charset=utf-8",
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "content type con mayúsculas válido",
			contentType: "APPLICATION/PDF",
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "content type vacío permitido",
			contentType: "",
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "content type octet-stream permitido",
			contentType: "application/octet-stream",
			key:         "file.pdf",
			expectError: false,
		},
		{
			name:        "content type text/plain inválido",
			contentType: "text/plain",
			key:         "file.pdf",
			expectError: true,
		},
		{
			name:        "content type image/jpeg inválido",
			contentType: "image/jpeg",
			key:         "file.pdf",
			expectError: true,
		},
		{
			name:        "content type application/json inválido",
			contentType: "application/json",
			key:         "file.pdf",
			expectError: true,
		},
		{
			name:        "content type con espacios válido",
			contentType: "  application/pdf  ",
			key:         "file.pdf",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateContentType(tt.contentType, tt.key)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "tipo de contenido no permitido")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_DefaultValues(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	client, err := NewClient(ctx, "us-east-1", "test-bucket", "", "key", "secret", false, log)
	assert.NoError(t, err)

	// Verificar valores por defecto
	assert.Equal(t, int64(100*1024*1024), client.maxFileSize, "maxFileSize debe ser 100MB")
	assert.Equal(t, int64(1024), client.minFileSize, "minFileSize debe ser 1KB")
	assert.Equal(t, []string{"application/pdf"}, client.allowedTypes, "allowedTypes debe contener application/pdf")
	assert.NotZero(t, client.downloadTimeout, "downloadTimeout debe estar configurado")
}
