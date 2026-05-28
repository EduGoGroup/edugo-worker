package processor

import (
	"errors"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/stretchr/testify/assert"
)

// mockLogger implementa logger.Logger para testing
type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...any)    {}
func (m *mockLogger) Info(_ string, _ ...any)     {}
func (m *mockLogger) Warn(_ string, _ ...any)     {}
func (m *mockLogger) Error(_ string, _ ...any)    {}
func (m *mockLogger) Fatal(_ string, _ ...any)    {}
func (m *mockLogger) With(_ ...any) logger.Logger { return m }
func (m *mockLogger) Sync() error                 { return nil }

func createTestLogger() logger.Logger {
	return &mockLogger{}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{
			name:     "PDF corrupto es permanente",
			err:      pdfErrors.ErrPDFCorrupt,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF escaneado es permanente",
			err:      pdfErrors.ErrPDFScanned,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF demasiado grande es permanente",
			err:      pdfErrors.ErrPDFTooLarge,
			expected: ErrorTypePermanent,
		},
		{
			name:     "PDF vacio es permanente",
			err:      pdfErrors.ErrPDFEmpty,
			expected: ErrorTypePermanent,
		},
		{
			name:     "error generico es transitorio por defecto",
			err:      errors.New("error generico de red"),
			expected: ErrorTypeTransient,
		},
		{
			name:     "nil error es permanente",
			err:      nil,
			expected: ErrorTypePermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	log := createTestLogger()
	cfg := DefaultRetryConfig(log)

	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, cfg.InitialBackoff)
	assert.Equal(t, 10*time.Second, cfg.MaxBackoff)
	assert.Equal(t, 2.0, cfg.BackoffMultiple)
	assert.NotNil(t, cfg.Logger)
	assert.NotNil(t, cfg.Classifier, "should inject classifyError as Classifier")
}
