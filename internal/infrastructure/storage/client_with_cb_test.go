package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/circuitbreaker"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage/mocks"
)

func TestClientWithCircuitBreaker_Download_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 2
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	expectedContent := "test content"
	mockReader := io.NopCloser(strings.NewReader(expectedContent))

	mockClient.On("Download", ctx, "test-key").Return(mockReader, nil)

	// Act
	reader, err := client.Download(ctx, "test-key")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	content, _ := io.ReadAll(reader)
	assert.Equal(t, expectedContent, string(content))
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_Download_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("storage service error")

	// Abrir el circuit con un fallo
	mockClient.On("Download", ctx, "test-key").Return(nil, testError).Once()
	_, _ = client.Download(ctx, "test-key")

	// Act - Circuit debe estar abierto
	reader, err := client.Download(ctx, "test-key")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestClientWithCircuitBreaker_Upload_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	content := bytes.NewReader([]byte("test content"))

	mockClient.On("Upload", ctx, "test-key", content).Return(nil)

	// Act
	err := client.Upload(ctx, "test-key", content)

	// Assert
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_Upload_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	content := bytes.NewReader([]byte("test"))
	testError := errors.New("upload failed")

	// Abrir el circuit
	mockClient.On("Upload", ctx, "test-key", content).Return(testError).Once()
	_ = client.Upload(ctx, "test-key", content)

	// Act
	err := client.Upload(ctx, "test-key", content)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
}

func TestClientWithCircuitBreaker_Delete_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	mockClient.On("Delete", ctx, "test-key").Return(nil)

	// Act
	err := client.Delete(ctx, "test-key")

	// Assert
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_Exists_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	mockClient.On("Exists", ctx, "test-key").Return(true, nil)

	// Act
	exists, err := client.Exists(ctx, "test-key")

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_Exists_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("exists check failed")

	// Abrir el circuit
	mockClient.On("Exists", ctx, "test-key").Return(false, testError).Once()
	_, _ = client.Exists(ctx, "test-key")

	// Act
	exists, err := client.Exists(ctx, "test-key")

	// Assert
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
}

func TestClientWithCircuitBreaker_GetMetadata_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	expectedMetadata := &storage.FileMetadata{
		Key:         "test-key",
		Size:        1024,
		ContentType: "application/pdf",
	}

	mockClient.On("GetMetadata", ctx, "test-key").Return(expectedMetadata, nil)

	// Act
	metadata, err := client.GetMetadata(ctx, "test-key")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedMetadata, metadata)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_GetMetadata_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("metadata fetch failed")

	// Abrir el circuit
	mockClient.On("GetMetadata", ctx, "test-key").Return(nil, testError).Once()
	_, _ = client.GetMetadata(ctx, "test-key")

	// Act
	metadata, err := client.GetMetadata(ctx, "test-key")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
}

func TestClientWithCircuitBreaker_Recovery(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("storage-test")
	cbConfig.MaxFailures = 1
	cbConfig.Timeout = 100 * time.Millisecond
	cbConfig.SuccessThreshold = 1
	cb := circuitbreaker.New(cbConfig)
	client := storage.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("temporary error")

	// Abrir el circuit
	mockClient.On("Exists", ctx, "test").Return(false, testError).Once()
	_, _ = client.Exists(ctx, "test")
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// Esperar timeout
	time.Sleep(150 * time.Millisecond)

	// Recuperación exitosa
	mockClient.On("Exists", ctx, "test").Return(true, nil).Once()

	// Act
	exists, err := client.Exists(ctx, "test")

	// Assert
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	mockClient.AssertExpectations(t)
}
