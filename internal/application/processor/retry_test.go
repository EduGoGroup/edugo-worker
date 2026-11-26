package processor

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
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
			name:     "prep invalido tras su reintento es permanente (HU-IA.4)",
			err:      fmt.Errorf("prep: %w", ErrInvalidPrep),
			expected: ErrorTypePermanent,
		},
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
			name:     "mensaje no enrutable es permanente",
			err:      ErrUnroutableMessage,
			expected: ErrorTypePermanent,
		},
		{
			name:     "mensaje no enrutable envuelto es permanente",
			err:      fmt.Errorf("%w: invalid message format", ErrUnroutableMessage),
			expected: ErrorTypePermanent,
		},
		{
			name:     "evento malformado es permanente",
			err:      fmt.Errorf("%w: sin attempt_id", ErrMalformedEvent),
			expected: ErrorTypePermanent,
		},
		{
			name:     "4xx permanente de learning es permanente",
			err:      fmt.Errorf("post review: %w", m2m.ErrLearningPermanent),
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

func TestIsPermanent(t *testing.T) {
	assert.False(t, IsPermanent(nil), "nil no es un fallo")
	assert.False(t, IsPermanent(errors.New("error generico de red")))
	assert.True(t, IsPermanent(ErrInvalidPrep), "el artefacto invalido tras el reintento es fallo de dato (HU-IA.4)")
	assert.True(t, IsPermanent(fmt.Errorf("prep: %w", ErrInvalidPrep)))
	assert.False(t, IsPermanent(llm.ErrLLMQuality), "la calidad sola no cambia de clase fuera del prep")

	assert.True(t, IsPermanent(ErrUnroutableMessage))
	assert.True(t, IsPermanent(fmt.Errorf("%w: missing event_type", ErrUnroutableMessage)))
	assert.True(t, IsPermanent(ErrMalformedEvent))
	assert.True(t, IsPermanent(fmt.Errorf("finalize: %w", m2m.ErrLearningPermanent)))
	assert.True(t, IsPermanent(pdfErrors.ErrPDFCorrupt))
}
