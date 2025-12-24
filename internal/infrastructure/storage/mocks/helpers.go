package mocks

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	mock "github.com/stretchr/testify/mock"
)

// Errores comunes para testing
var (
	ErrMockS3NotFound     = errors.New("mock: archivo no encontrado en S3")
	ErrMockS3Timeout      = errors.New("mock: timeout conectando a S3")
	ErrMockS3Unauthorized = errors.New("mock: acceso no autorizado a S3")
	ErrMockS3Network      = errors.New("mock: error de red con S3")
)

// NewSuccessfulMockClient crea un mock que siempre tiene éxito
func NewSuccessfulMockClient(t *testing.T) *MockClient {
	mockClient := NewMockClient(t)

	// Mock para Download que retorna contenido de prueba
	mockClient.On("Download", mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader("PDF content for testing")), nil).
		Maybe()

	// Mock para Upload que siempre tiene éxito
	mockClient.On("Upload", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()

	// Mock para Delete que siempre tiene éxito
	mockClient.On("Delete", mock.Anything, mock.Anything).
		Return(nil).
		Maybe()

	// Mock para Exists que siempre retorna true
	mockClient.On("Exists", mock.Anything, mock.Anything).
		Return(true, nil).
		Maybe()

	return mockClient
}

// NewFailingMockClient crea un mock que siempre falla con el error especificado
func NewFailingMockClient(t *testing.T, err error) *MockClient {
	mockClient := NewMockClient(t)

	// Todos los métodos fallan con el error especificado
	mockClient.On("Download", mock.Anything, mock.Anything).
		Return(nil, err).
		Maybe()

	mockClient.On("Upload", mock.Anything, mock.Anything, mock.Anything).
		Return(err).
		Maybe()

	mockClient.On("Delete", mock.Anything, mock.Anything).
		Return(err).
		Maybe()

	mockClient.On("Exists", mock.Anything, mock.Anything).
		Return(false, err).
		Maybe()

	return mockClient
}

// NewTimeoutMockClient crea un mock que simula timeouts
func NewTimeoutMockClient(t *testing.T) *MockClient {
	return NewFailingMockClient(t, context.DeadlineExceeded)
}

// NewNotFoundMockClient crea un mock que simula archivos no encontrados
func NewNotFoundMockClient(t *testing.T) *MockClient {
	return NewFailingMockClient(t, ErrMockS3NotFound)
}

// NewNetworkErrorMockClient crea un mock que simula errores de red
func NewNetworkErrorMockClient(t *testing.T) *MockClient {
	return NewFailingMockClient(t, ErrMockS3Network)
}

// WithDownloadResponse configura un mock para retornar contenido específico
func WithDownloadResponse(mockClient *MockClient, key string, content string, err error) *MockClient {
	if err != nil {
		mockClient.On("Download", mock.Anything, key).
			Return(nil, err).
			Once()
	} else {
		mockClient.On("Download", mock.Anything, key).
			Return(io.NopCloser(strings.NewReader(content)), nil).
			Once()
	}
	return mockClient
}

// WithUploadResponse configura un mock para el método Upload
func WithUploadResponse(mockClient *MockClient, key string, err error) *MockClient {
	mockClient.On("Upload", mock.Anything, key, mock.Anything).
		Return(err).
		Once()
	return mockClient
}

// WithExistsResponse configura un mock para el método Exists
func WithExistsResponse(mockClient *MockClient, key string, exists bool, err error) *MockClient {
	mockClient.On("Exists", mock.Anything, key).
		Return(exists, err).
		Once()
	return mockClient
}

// NewFlakeyMockClient crea un mock que falla las primeras N veces y luego tiene éxito
// Útil para testing de retry logic
func NewFlakeyMockClient(t *testing.T, failCount int, failErr error) *MockClient {
	mockClient := NewMockClient(t)

	// Configurar Download para fallar N veces y luego tener éxito
	for i := 0; i < failCount; i++ {
		mockClient.On("Download", mock.Anything, mock.Anything).
			Return(nil, failErr).
			Once()
	}
	mockClient.On("Download", mock.Anything, mock.Anything).
		Return(io.NopCloser(strings.NewReader("PDF content after retries")), nil).
		Maybe()

	return mockClient
}
