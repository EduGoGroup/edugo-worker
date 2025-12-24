package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	nlpMocks "github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp/mocks"
	pdfMocks "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf/mocks"
	storageMocks "github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMaterialUploadedProcessor_EventType(t *testing.T) {
	processor := &MaterialUploadedProcessor{}

	eventType := processor.EventType()

	assert.Equal(t, "material_uploaded", eventType)
}

func TestMaterialUploadedProcessor_Process_InvalidJSON(t *testing.T) {
	// Arrange
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	processor := &MaterialUploadedProcessor{
		db:     db,
		logger: newTestLogger(),
	}

	invalidPayload := []byte("invalid json {")

	// Act
	err = processor.Process(context.Background(), invalidPayload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event payload")
}

func TestMaterialUploadedProcessor_Process_InvalidMaterialID(t *testing.T) {
	// Arrange
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	processor := &MaterialUploadedProcessor{
		db:     db,
		logger: newTestLogger(),
	}

	event := dto.MaterialUploadedEvent{
		MaterialID: "invalid-uuid",
		S3Key:      "test.pdf",
	}
	payload, _ := json.Marshal(event)

	// Act
	err = processor.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid material_id")
}

func TestMaterialUploadedProcessor_Process_StorageDownloadError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Mock para actualizar estado a processing
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock para actualizar estado a failed (cuando falla el download)
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	storageClient := storageMocks.NewMockClient(t)
	storageClient.EXPECT().
		Download(mock.Anything, "test.pdf").
		Return(nil, assert.AnError)

	processor := &MaterialUploadedProcessor{
		db:            db,
		logger:        newTestLogger(),
		storageClient: storageClient,
	}

	event := dto.MaterialUploadedEvent{
		MaterialID: "550e8400-e29b-41d4-a716-446655440000",
		S3Key:      "test.pdf",
	}
	payload, _ := json.Marshal(event)

	// Act
	err = processor.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download PDF")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestMaterialUploadedProcessor_Process_PDFExtractionError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Mock para actualizar estado a processing
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock para actualizar estado a failed
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock storage client que retorna un reader
	mockReader := io.NopCloser(bytes.NewReader([]byte("pdf content")))
	storageClient := storageMocks.NewMockClient(t)
	storageClient.EXPECT().
		Download(mock.Anything, "test.pdf").
		Return(mockReader, nil)

	// Mock PDF extractor que falla
	pdfExtractor := pdfMocks.NewMockExtractor(t)
	pdfExtractor.EXPECT().
		Extract(mock.Anything, mock.Anything).
		Return("", assert.AnError)

	processor := &MaterialUploadedProcessor{
		db:            db,
		logger:        newTestLogger(),
		storageClient: storageClient,
		pdfExtractor:  pdfExtractor,
	}

	event := dto.MaterialUploadedEvent{
		MaterialID: "550e8400-e29b-41d4-a716-446655440000",
		S3Key:      "test.pdf",
	}
	payload, _ := json.Marshal(event)

	// Act
	err = processor.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract PDF text")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestMaterialUploadedProcessor_Process_NLPSummaryError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Mock para actualizar estado a processing
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock para actualizar estado a failed
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock storage client
	mockReader := io.NopCloser(bytes.NewReader([]byte("pdf content")))
	storageClient := storageMocks.NewMockClient(t)
	storageClient.EXPECT().
		Download(mock.Anything, "test.pdf").
		Return(mockReader, nil)

	// Mock PDF extractor exitoso
	pdfExtractor := pdfMocks.NewMockExtractor(t)
	pdfExtractor.EXPECT().
		Extract(mock.Anything, mock.Anything).
		Return("Extracted text from PDF", nil)

	// Mock NLP client que falla en GenerateSummary
	nlpClient := nlpMocks.NewMockClient(t)
	nlpClient.EXPECT().
		GenerateSummary(mock.Anything, "Extracted text from PDF").
		Return(nil, assert.AnError)

	processor := &MaterialUploadedProcessor{
		db:            db,
		logger:        newTestLogger(),
		storageClient: storageClient,
		pdfExtractor:  pdfExtractor,
		nlpClient:     nlpClient,
	}

	event := dto.MaterialUploadedEvent{
		MaterialID: "550e8400-e29b-41d4-a716-446655440000",
		S3Key:      "test.pdf",
	}
	payload, _ := json.Marshal(event)

	// Act
	err = processor.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate summary")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestMaterialUploadedProcessor_Process_NLPQuizError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Mock para actualizar estado a processing
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("processing", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock para actualizar estado a failed
	dbMock.ExpectExec("UPDATE materials SET processing_status").
		WithArgs("failed", "550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock storage client
	mockReader := io.NopCloser(bytes.NewReader([]byte("pdf content")))
	storageClient := storageMocks.NewMockClient(t)
	storageClient.EXPECT().
		Download(mock.Anything, "test.pdf").
		Return(mockReader, nil)

	// Mock PDF extractor exitoso
	pdfExtractor := pdfMocks.NewMockExtractor(t)
	pdfExtractor.EXPECT().
		Extract(mock.Anything, mock.Anything).
		Return("Extracted text from PDF", nil)

	// Mock NLP client - GenerateSummary exitoso
	summary := &nlp.Summary{
		MainIdeas:   []string{"Idea 1", "Idea 2"},
		KeyConcepts: map[string]string{"concept1": "description1"},
		Sections:    []nlp.Section{{Title: "Section 1", Content: "Content 1"}},
		Glossary:    map[string]string{"term1": "definition1"},
		WordCount:   100,
		GeneratedAt: time.Now(),
	}
	nlpClient := nlpMocks.NewMockClient(t)
	nlpClient.EXPECT().
		GenerateSummary(mock.Anything, "Extracted text from PDF").
		Return(summary, nil)

	// Mock NLP client - GenerateQuiz falla
	nlpClient.EXPECT().
		GenerateQuiz(mock.Anything, "Extracted text from PDF", 10).
		Return(nil, assert.AnError)

	processor := &MaterialUploadedProcessor{
		db:            db,
		logger:        newTestLogger(),
		storageClient: storageClient,
		pdfExtractor:  pdfExtractor,
		nlpClient:     nlpClient,
	}

	event := dto.MaterialUploadedEvent{
		MaterialID: "550e8400-e29b-41d4-a716-446655440000",
		S3Key:      "test.pdf",
	}
	payload, _ := json.Marshal(event)

	// Act
	err = processor.Process(context.Background(), payload)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate quiz")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

// TestMaterialUploadedProcessor_Process_Success requiere MongoDB real o un mock complejo
// Lo dejaremos para tests de integración
func TestMaterialUploadedProcessor_NewMaterialUploadedProcessor(t *testing.T) {
	// Arrange
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Crear un cliente MongoDB de prueba (puede ser nil para este test)
	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017")
	mongoClient, _ := mongo.Connect(context.Background(), clientOpts)
	mongodb := mongoClient.Database("test")

	storageClient := storageMocks.NewMockClient(t)
	pdfExtractor := pdfMocks.NewMockExtractor(t)
	nlpClient := nlpMocks.NewMockClient(t)

	cfg := MaterialUploadedProcessorConfig{
		DB:            db,
		MongoDB:       mongodb,
		Logger:        newTestLogger(),
		StorageClient: storageClient,
		PDFExtractor:  pdfExtractor,
		NLPClient:     nlpClient,
	}

	// Act
	processor := NewMaterialUploadedProcessor(cfg)

	// Assert
	assert.NotNil(t, processor)
	assert.Equal(t, db, processor.db)
	assert.Equal(t, mongodb, processor.mongodb)
	assert.Equal(t, storageClient, processor.storageClient)
	assert.Equal(t, pdfExtractor, processor.pdfExtractor)
	assert.Equal(t, nlpClient, processor.nlpClient)
}
