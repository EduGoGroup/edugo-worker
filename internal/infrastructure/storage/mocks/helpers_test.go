package mocks

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSuccessfulMockClient(t *testing.T) {
	mockClient := NewSuccessfulMockClient(t)
	ctx := context.Background()

	// Test Download
	reader, err := mockClient.Download(ctx, "test.pdf")
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "PDF content")

	// Test Upload
	err = mockClient.Upload(ctx, "test.pdf", io.NopCloser(nil))
	assert.NoError(t, err)

	// Test Exists
	exists, err := mockClient.Exists(ctx, "test.pdf")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test Delete
	err = mockClient.Delete(ctx, "test.pdf")
	assert.NoError(t, err)
}

func TestNewFailingMockClient(t *testing.T) {
	testErr := ErrMockS3Network
	mockClient := NewFailingMockClient(t, testErr)
	ctx := context.Background()

	// Test Download
	reader, err := mockClient.Download(ctx, "test.pdf")
	assert.Error(t, err)
	assert.ErrorIs(t, err, testErr)
	assert.Nil(t, reader)

	// Test Upload
	err = mockClient.Upload(ctx, "test.pdf", io.NopCloser(nil))
	assert.Error(t, err)
	assert.ErrorIs(t, err, testErr)

	// Test Exists
	exists, err := mockClient.Exists(ctx, "test.pdf")
	assert.Error(t, err)
	assert.False(t, exists)

	// Test Delete
	err = mockClient.Delete(ctx, "test.pdf")
	assert.Error(t, err)
}

func TestNewTimeoutMockClient(t *testing.T) {
	mockClient := NewTimeoutMockClient(t)
	ctx := context.Background()

	reader, err := mockClient.Download(ctx, "test.pdf")
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, reader)
}

func TestNewNotFoundMockClient(t *testing.T) {
	mockClient := NewNotFoundMockClient(t)
	ctx := context.Background()

	reader, err := mockClient.Download(ctx, "missing.pdf")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMockS3NotFound)
	assert.Nil(t, reader)
}

func TestWithDownloadResponse(t *testing.T) {
	mockClient := NewMockClient(t)
	customContent := "Custom PDF content for specific test"

	WithDownloadResponse(mockClient, "specific.pdf", customContent, nil)

	ctx := context.Background()
	reader, err := mockClient.Download(ctx, "specific.pdf")

	require.NoError(t, err)
	require.NotNil(t, reader)

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, customContent, string(content))
}

func TestWithUploadResponse(t *testing.T) {
	mockClient := NewMockClient(t)

	WithUploadResponse(mockClient, "upload.pdf", nil)

	ctx := context.Background()
	err := mockClient.Upload(ctx, "upload.pdf", io.NopCloser(nil))

	assert.NoError(t, err)
}

func TestWithExistsResponse(t *testing.T) {
	mockClient := NewMockClient(t)

	WithExistsResponse(mockClient, "existing.pdf", true, nil)
	WithExistsResponse(mockClient, "missing.pdf", false, ErrMockS3NotFound)

	ctx := context.Background()

	// Test archivo existente
	exists, err := mockClient.Exists(ctx, "existing.pdf")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test archivo no existente
	exists, err = mockClient.Exists(ctx, "missing.pdf")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestNewFlakeyMockClient(t *testing.T) {
	failCount := 2
	mockClient := NewFlakeyMockClient(t, failCount, ErrMockS3Network)
	ctx := context.Background()

	// Primeras 2 llamadas deben fallar
	for i := 0; i < failCount; i++ {
		reader, err := mockClient.Download(ctx, "test.pdf")
		assert.Error(t, err, "intento %d debería fallar", i+1)
		assert.ErrorIs(t, err, ErrMockS3Network)
		assert.Nil(t, reader)
	}

	// Tercera llamada debe tener éxito
	reader, err := mockClient.Download(ctx, "test.pdf")
	assert.NoError(t, err, "intento después de fallos debería tener éxito")
	assert.NotNil(t, reader)

	content, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "after retries")
}

func TestFlakeyMockClient_SimulatesRetryScenario(t *testing.T) {
	// Simular un escenario donde el cliente falla 2 veces y luego tiene éxito
	mockClient := NewFlakeyMockClient(t, 2, ErrMockS3Timeout)
	ctx := context.Background()

	attempts := 0
	maxAttempts := 3
	var lastErr error
	var reader io.ReadCloser

	// Simular retry logic
	for attempts < maxAttempts {
		attempts++
		reader, lastErr = mockClient.Download(ctx, "flaky.pdf")

		if lastErr == nil {
			break
		}

		t.Logf("Intento %d falló: %v", attempts, lastErr)
	}

	// Verificar que eventualmente tuvo éxito
	assert.Equal(t, 3, attempts, "debería intentar 3 veces")
	assert.NoError(t, lastErr, "debería tener éxito después de 3 intentos")
	assert.NotNil(t, reader)
}
