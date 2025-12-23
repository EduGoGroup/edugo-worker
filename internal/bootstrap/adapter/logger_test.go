package adapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewLoggerAdapter(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	adapter := NewLoggerAdapter(logrusLogger)

	if adapter == nil {
		t.Fatal("expected adapter to be created")
	}

	// Verificar que es del tipo correcto
	_, ok := adapter.(*LoggerAdapter)
	if !ok {
		t.Error("expected adapter to be of type *LoggerAdapter")
	}
}

func TestLoggerAdapter_Debug(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})
	logrusLogger.SetLevel(logrus.DebugLevel)

	adapter := NewLoggerAdapter(logrusLogger)

	// Test sin fields
	adapter.Debug("test debug message")
	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"level":"debug"`) {
		t.Errorf("expected debug level, got: %s", output)
	}

	// Test con fields
	buf.Reset()
	adapter.Debug("test with fields", "key1", "value1", "key2", 42)
	output = buf.String()
	if !strings.Contains(output, "test with fields") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"key1":"value1"`) {
		t.Errorf("expected key1 field, got: %s", output)
	}
	if !strings.Contains(output, `"key2":42`) {
		t.Errorf("expected key2 field, got: %s", output)
	}
}

func TestLoggerAdapter_Info(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Test sin fields
	adapter.Info("test info message")
	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("expected info level, got: %s", output)
	}

	// Test con fields
	buf.Reset()
	adapter.Info("info with fields", "user", "john", "age", 30)
	output = buf.String()
	if !strings.Contains(output, "info with fields") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"user":"john"`) {
		t.Errorf("expected user field, got: %s", output)
	}
}

func TestLoggerAdapter_Warn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Test sin fields
	adapter.Warn("test warning")
	output := buf.String()
	if !strings.Contains(output, "test warning") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"level":"warning"`) {
		t.Errorf("expected warning level, got: %s", output)
	}

	// Test con fields
	buf.Reset()
	adapter.Warn("warning with fields", "issue", "deprecated")
	output = buf.String()
	if !strings.Contains(output, `"issue":"deprecated"`) {
		t.Errorf("expected issue field, got: %s", output)
	}
}

func TestLoggerAdapter_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Test sin fields
	adapter.Error("test error")
	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"level":"error"`) {
		t.Errorf("expected error level, got: %s", output)
	}

	// Test con fields
	buf.Reset()
	adapter.Error("error with fields", "code", 500, "error_msg", "internal error")
	output = buf.String()
	if !strings.Contains(output, `"code":500`) {
		t.Errorf("expected code field, got: %s", output)
	}
	if !strings.Contains(output, `"error_msg":"internal error"`) {
		t.Errorf("expected error_msg field, got: %s", output)
	}
}

func TestLoggerAdapter_With(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Crear logger con contexto
	contextLogger := adapter.With("request_id", "123", "user", "john")

	// Verificar que retorna un logger
	if contextLogger == nil {
		t.Fatal("expected contextLogger to be created")
	}

	// Verificar que es del tipo correcto
	_, ok := contextLogger.(*loggerEntryAdapter)
	if !ok {
		t.Error("expected contextLogger to be of type *loggerEntryAdapter")
	}

	// Usar el logger con contexto
	contextLogger.Info("test message")
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"request_id":"123"`) {
		t.Errorf("expected request_id field, got: %s", output)
	}
	if !strings.Contains(output, `"user":"john"`) {
		t.Errorf("expected user field, got: %s", output)
	}
}

func TestLoggerAdapter_Sync(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	adapter := NewLoggerAdapter(logrusLogger)

	// Sync debería retornar nil sin error
	err := adapter.Sync()
	if err != nil {
		t.Errorf("expected no error from Sync, got: %v", err)
	}
}

func TestLoggerEntryAdapter_Debug(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})
	logrusLogger.SetLevel(logrus.DebugLevel)

	adapter := NewLoggerAdapter(logrusLogger)
	entryAdapter := adapter.With("base", "context")

	// Test sin fields adicionales
	entryAdapter.Debug("entry debug")
	output := buf.String()
	if !strings.Contains(output, "entry debug") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
	if !strings.Contains(output, `"base":"context"`) {
		t.Errorf("expected base context field, got: %s", output)
	}

	// Test con fields adicionales
	buf.Reset()
	entryAdapter.Debug("entry with more fields", "extra", "data")
	output = buf.String()
	if !strings.Contains(output, `"extra":"data"`) {
		t.Errorf("expected extra field, got: %s", output)
	}
}

func TestLoggerEntryAdapter_Info(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)
	entryAdapter := adapter.With("service", "api")

	entryAdapter.Info("entry info", "status", "ok")
	output := buf.String()

	if !strings.Contains(output, `"service":"api"`) {
		t.Errorf("expected service field, got: %s", output)
	}
	if !strings.Contains(output, `"status":"ok"`) {
		t.Errorf("expected status field, got: %s", output)
	}
}

func TestLoggerEntryAdapter_Warn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)
	entryAdapter := adapter.With("module", "auth")

	entryAdapter.Warn("entry warning")
	output := buf.String()

	if !strings.Contains(output, `"module":"auth"`) {
		t.Errorf("expected module field, got: %s", output)
	}
	if !strings.Contains(output, `"level":"warning"`) {
		t.Errorf("expected warning level, got: %s", output)
	}
}

func TestLoggerEntryAdapter_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)
	entryAdapter := adapter.With("component", "db")

	entryAdapter.Error("entry error", "code", 500)
	output := buf.String()

	if !strings.Contains(output, `"component":"db"`) {
		t.Errorf("expected component field, got: %s", output)
	}
	if !strings.Contains(output, `"code":500`) {
		t.Errorf("expected code field, got: %s", output)
	}
}

func TestLoggerEntryAdapter_With(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Crear contexto en capas
	layer1 := adapter.With("layer", "1")
	layer2 := layer1.With("layer", "2")

	layer2.Info("nested context")
	output := buf.String()

	// El último valor de "layer" debería ser "2"
	if !strings.Contains(output, "nested context") {
		t.Errorf("expected output to contain message, got: %s", output)
	}
}

func TestLoggerEntryAdapter_Sync(t *testing.T) {
	t.Parallel()

	logrusLogger := logrus.New()
	adapter := NewLoggerAdapter(logrusLogger)
	entryAdapter := adapter.With("test", "sync")

	err := entryAdapter.Sync()
	if err != nil {
		t.Errorf("expected no error from Sync, got: %v", err)
	}
}

func TestConvertToLogrusFields(t *testing.T) {
	t.Parallel()

	// Test con pares normales
	fields := convertToLogrusFields("key1", "value1", "key2", 42, "key3", true)

	if len(fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(fields))
	}

	if fields["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", fields["key1"])
	}

	if fields["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", fields["key2"])
	}

	if fields["key3"] != true {
		t.Errorf("expected key3=true, got %v", fields["key3"])
	}

	// Test con número impar de argumentos (último se ignora)
	fields = convertToLogrusFields("key1", "value1", "orphan")
	if len(fields) != 1 {
		t.Errorf("expected 1 field (orphan ignored), got %d", len(fields))
	}

	// Test con key no-string
	fields = convertToLogrusFields(123, "value")
	if _, exists := fields["unknown"]; !exists {
		t.Error("expected 'unknown' key for non-string key")
	}

	// Test sin argumentos
	fields = convertToLogrusFields()
	if len(fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(fields))
	}
}

func TestLoggerAdapter_ComplexFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logrusLogger := logrus.New()
	logrusLogger.SetOutput(&buf)
	logrusLogger.SetFormatter(&logrus.JSONFormatter{})

	adapter := NewLoggerAdapter(logrusLogger)

	// Test con diferentes tipos de valores
	type customStruct struct {
		Name string
		Age  int
	}

	adapter.Info("complex fields",
		"string", "text",
		"int", 123,
		"float", 45.67,
		"bool", true,
		"struct", customStruct{Name: "John", Age: 30},
		"slice", []string{"a", "b", "c"},
		"map", map[string]int{"x": 1, "y": 2},
	)

	output := buf.String()

	// Verificar que el JSON es válido
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v", err)
	}

	// Verificar algunos campos
	if logEntry["string"] != "text" {
		t.Errorf("expected string field, got: %v", logEntry["string"])
	}

	if logEntry["bool"] != true {
		t.Errorf("expected bool field, got: %v", logEntry["bool"])
	}
}
