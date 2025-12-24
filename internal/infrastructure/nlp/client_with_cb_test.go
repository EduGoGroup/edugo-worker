package nlp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/circuitbreaker"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp/mocks"
)

func TestClientWithCircuitBreaker_GenerateSummary_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cbConfig.MaxFailures = 2
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	expectedSummary := &nlp.Summary{
		MainIdeas:   []string{"Idea 1", "Idea 2"},
		WordCount:   100,
		GeneratedAt: time.Now(),
	}

	mockClient.On("GenerateSummary", ctx, "test text").Return(expectedSummary, nil)

	// Act
	summary, err := client.GenerateSummary(ctx, "test text")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_GenerateSummary_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cbConfig.MaxFailures = 1
	cbConfig.Timeout = 100 * time.Millisecond
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("nlp service error")

	// Abrir el circuit con un fallo
	mockClient.On("GenerateSummary", ctx, "test text").Return(nil, testError).Once()
	_, _ = client.GenerateSummary(ctx, "test text")

	// Act - Circuit debe estar abierto
	summary, err := client.GenerateSummary(ctx, "test text")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, summary)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())
}

func TestClientWithCircuitBreaker_GenerateQuiz_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	expectedQuiz := &nlp.Quiz{
		Questions: []nlp.Question{
			{
				ID:           "q1",
				QuestionText: "Test question?",
				QuestionType: "multiple_choice",
			},
		},
		GeneratedAt: time.Now(),
	}

	mockClient.On("GenerateQuiz", ctx, "test text", 5).Return(expectedQuiz, nil)

	// Act
	quiz, err := client.GenerateQuiz(ctx, "test text", 5)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedQuiz, quiz)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_GenerateQuiz_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("nlp service error")

	// Abrir el circuit con un fallo
	mockClient.On("GenerateQuiz", ctx, "test text", 5).Return(nil, testError).Once()
	_, _ = client.GenerateQuiz(ctx, "test text", 5)

	// Act - Circuit debe estar abierto
	quiz, err := client.GenerateQuiz(ctx, "test text", 5)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, quiz)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
}

func TestClientWithCircuitBreaker_HealthCheck_Success(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	mockClient.On("HealthCheck", ctx).Return(nil)

	// Act
	err := client.HealthCheck(ctx)

	// Assert
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClientWithCircuitBreaker_HealthCheck_CircuitOpen(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cbConfig.MaxFailures = 1
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("health check failed")

	// Abrir el circuit
	mockClient.On("HealthCheck", ctx).Return(testError).Once()
	_ = client.HealthCheck(ctx)

	// Act - Circuit debe estar abierto
	err := client.HealthCheck(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, circuitbreaker.ErrCircuitOpen, err)
}

func TestClientWithCircuitBreaker_Recovery(t *testing.T) {
	// Arrange
	mockClient := mocks.NewMockClient(t)
	cbConfig := circuitbreaker.DefaultConfig("nlp-test")
	cbConfig.MaxFailures = 1
	cbConfig.Timeout = 100 * time.Millisecond
	cbConfig.SuccessThreshold = 1
	cb := circuitbreaker.New(cbConfig)
	client := nlp.NewClientWithCircuitBreaker(mockClient, cb)

	ctx := context.Background()
	testError := errors.New("temporary error")

	// Abrir el circuit
	mockClient.On("GenerateSummary", ctx, "test").Return(nil, testError).Once()
	_, _ = client.GenerateSummary(ctx, "test")
	assert.Equal(t, circuitbreaker.StateOpen, cb.State())

	// Esperar timeout
	time.Sleep(150 * time.Millisecond)

	// Recuperación exitosa
	expectedSummary := &nlp.Summary{MainIdeas: []string{"Recovered"}}
	mockClient.On("GenerateSummary", ctx, "test").Return(expectedSummary, nil).Once()

	// Act
	summary, err := client.GenerateSummary(ctx, "test")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	assert.Equal(t, circuitbreaker.StateClosed, cb.State())
	mockClient.AssertExpectations(t)
}
