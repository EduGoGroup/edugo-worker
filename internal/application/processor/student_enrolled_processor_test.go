package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contextKey es un tipo personalizado para las keys del contexto
// para evitar colisiones con otras librerías (SA1029)
type contextKey string

const testContextKey contextKey = "test_key"

func TestStudentEnrolledProcessor_EventType(t *testing.T) {
	logger := createTestLogger()
	processor := NewStudentEnrolledProcessor(logger)

	eventType := processor.EventType()
	assert.Equal(t, "student_enrolled", eventType)
}

func TestStudentEnrolledProcessor_Process_Success(t *testing.T) {
	// Arrange
	logger := createTestLogger()
	processor := NewStudentEnrolledProcessor(logger)

	event := dto.StudentEnrolledEvent{
		StudentID: "student-123",
		UnitID:    "unit-456",
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	err = processor.Process(ctx, payload)

	// Assert
	assert.NoError(t, err)
}

func TestStudentEnrolledProcessor_Process_InvalidJSON(t *testing.T) {
	// Arrange
	logger := createTestLogger()
	processor := NewStudentEnrolledProcessor(logger)

	invalidPayload := []byte(`{"invalid json}`)
	ctx := context.Background()

	// Act
	err := processor.Process(ctx, invalidPayload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestStudentEnrolledProcessor_Process_EmptyPayload(t *testing.T) {
	// Arrange
	logger := createTestLogger()
	processor := NewStudentEnrolledProcessor(logger)

	emptyPayload := []byte(`{}`)
	ctx := context.Background()

	// Act
	err := processor.Process(ctx, emptyPayload)

	// Assert
	// Debería procesarse sin error aunque los campos estén vacíos
	assert.NoError(t, err)
}

func TestStudentEnrolledProcessor_Process_WithContext(t *testing.T) {
	// Arrange
	logger := createTestLogger()
	processor := NewStudentEnrolledProcessor(logger)

	event := dto.StudentEnrolledEvent{
		StudentID: "student-789",
		UnitID:    "unit-012",
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	// Crear contexto con valores
	ctx := context.WithValue(context.Background(), testContextKey, "test_value")

	// Act
	err = processor.Process(ctx, payload)

	// Assert
	assert.NoError(t, err)
}

func TestNewStudentEnrolledProcessor(t *testing.T) {
	// Arrange
	logger := createTestLogger()

	// Act
	processor := NewStudentEnrolledProcessor(logger)

	// Assert
	assert.NotNil(t, processor)
	assert.NotNil(t, processor.logger)
	assert.Equal(t, "student_enrolled", processor.EventType())
}
