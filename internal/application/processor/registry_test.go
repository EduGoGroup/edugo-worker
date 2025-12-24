package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
)

// nopLogger es un logger que no hace nada (para tests)
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Info(msg string, keysAndValues ...interface{})   {}
func (l *nopLogger) Warn(msg string, keysAndValues ...interface{})   {}
func (l *nopLogger) Error(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Fatal(msg string, keysAndValues ...interface{})  {}
func (l *nopLogger) Sync() error                                     { return nil }
func (l *nopLogger) With(keysAndValues ...interface{}) logger.Logger { return l }

func newTestLogger() logger.Logger {
	return &nopLogger{}
}

// mockProcessor es un processor de prueba
type mockProcessor struct {
	eventType     string
	processCalled bool
	processError  error
	lastPayload   []byte
}

func (m *mockProcessor) EventType() string {
	return m.eventType
}

func (m *mockProcessor) Process(ctx context.Context, payload []byte) error {
	m.processCalled = true
	m.lastPayload = payload
	return m.processError
}

func TestNewRegistry(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.Count() != 0 {
		t.Errorf("expected empty registry, got %d processors", registry.Count())
	}
}

func TestRegistry_Register(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Registrar un processor
	mock1 := &mockProcessor{eventType: "test_event"}
	registry.Register(mock1)

	if registry.Count() != 1 {
		t.Errorf("expected 1 processor, got %d", registry.Count())
	}

	types := registry.RegisteredTypes()
	if len(types) != 1 || types[0] != "test_event" {
		t.Errorf("expected [test_event], got %v", types)
	}
}

func TestRegistry_Register_Multiple(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Registrar múltiples processors
	mock1 := &mockProcessor{eventType: "event_1"}
	mock2 := &mockProcessor{eventType: "event_2"}
	mock3 := &mockProcessor{eventType: "event_3"}

	registry.Register(mock1)
	registry.Register(mock2)
	registry.Register(mock3)

	if registry.Count() != 3 {
		t.Errorf("expected 3 processors, got %d", registry.Count())
	}

	types := registry.RegisteredTypes()
	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d: %v", len(types), types)
	}
}

func TestRegistry_Register_Overwrite(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Registrar un processor
	mock1 := &mockProcessor{eventType: "test_event"}
	registry.Register(mock1)

	// Registrar otro con el mismo event_type (debe sobrescribir)
	mock2 := &mockProcessor{eventType: "test_event"}
	registry.Register(mock2)

	if registry.Count() != 1 {
		t.Errorf("expected 1 processor after overwrite, got %d", registry.Count())
	}
}

func TestRegistry_Process_ValidMessage(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Registrar mock processor
	mock := &mockProcessor{eventType: "material_uploaded"}
	registry.Register(mock)

	// Crear mensaje de prueba
	message := map[string]interface{}{
		"event_type":  "material_uploaded",
		"material_id": "123",
	}
	payload, _ := json.Marshal(message)

	// Procesar
	ctx := context.Background()
	err := registry.Process(ctx, payload)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mock.processCalled {
		t.Error("processor.Process was not called")
	}

	if string(mock.lastPayload) != string(payload) {
		t.Error("processor received different payload")
	}
}

func TestRegistry_Process_UnknownEventType(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// NO registrar ningún processor

	// Crear mensaje con event_type desconocido
	message := map[string]interface{}{
		"event_type": "unknown_event",
		"data":       "test",
	}
	payload, _ := json.Marshal(message)

	// Procesar - NO debe retornar error
	ctx := context.Background()
	err := registry.Process(ctx, payload)

	if err != nil {
		t.Errorf("expected nil error for unknown event_type, got: %v", err)
	}
}

func TestRegistry_Process_InvalidJSON(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Payload inválido
	payload := []byte("invalid json {")

	ctx := context.Background()
	err := registry.Process(ctx, payload)

	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestRegistry_Process_MissingEventType(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Mensaje sin event_type
	message := map[string]interface{}{
		"data": "test",
	}
	payload, _ := json.Marshal(message)

	ctx := context.Background()
	err := registry.Process(ctx, payload)

	if err == nil {
		t.Error("expected error for missing event_type, got nil")
	}
}

func TestRegistry_Process_ProcessorError(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	// Mock processor que retorna error
	expectedError := errors.New("processing failed")
	mock := &mockProcessor{
		eventType:    "test_event",
		processError: expectedError,
	}
	registry.Register(mock)

	// Crear mensaje
	message := map[string]interface{}{
		"event_type": "test_event",
	}
	payload, _ := json.Marshal(message)

	// Procesar - debe propagar el error
	ctx := context.Background()
	err := registry.Process(ctx, payload)

	if err == nil {
		t.Error("expected error from processor, got nil")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("expected error %v, got %v", expectedError, err)
	}
}

func TestRegistry_RegisteredTypes_Empty(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	types := registry.RegisteredTypes()

	if len(types) != 0 {
		t.Errorf("expected empty slice, got %v", types)
	}
}

func TestRegistry_Count_Empty(t *testing.T) {
	logger := newTestLogger()
	registry := NewRegistry(logger)

	if registry.Count() != 0 {
		t.Errorf("expected count 0, got %d", registry.Count())
	}
}
