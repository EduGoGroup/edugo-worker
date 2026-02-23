package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/common/types/enum"
	"github.com/EduGoGroup/edugo-shared/database/postgres"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/EduGoGroup/edugo-worker/internal/domain/valueobject"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/metrics"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/storage"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MaterialUploadedProcessor procesa eventos de material subido
type MaterialUploadedProcessor struct {
	db            *sql.DB
	mongodb       *mongo.Database
	logger        logger.Logger
	storageClient storage.Client
	pdfExtractor  pdf.Extractor
	nlpClient     nlp.Client
	aiModel       string
}

// MaterialUploadedProcessorConfig contiene las dependencias del processor
type MaterialUploadedProcessorConfig struct {
	DB            *sql.DB
	MongoDB       *mongo.Database
	Logger        logger.Logger
	StorageClient storage.Client
	PDFExtractor  pdf.Extractor
	NLPClient     nlp.Client
	AIModel       string // Nombre del modelo IA activo (ej: "gpt-4-turbo-preview")
}

func NewMaterialUploadedProcessor(cfg MaterialUploadedProcessorConfig) *MaterialUploadedProcessor {
	aiModel := cfg.AIModel
	if aiModel == "" {
		aiModel = "unknown"
	}
	return &MaterialUploadedProcessor{
		db:            cfg.DB,
		mongodb:       cfg.MongoDB,
		logger:        cfg.Logger,
		storageClient: cfg.StorageClient,
		pdfExtractor:  cfg.PDFExtractor,
		nlpClient:     cfg.NLPClient,
		aiModel:       aiModel,
	}
}

func (p *MaterialUploadedProcessor) processEvent(ctx context.Context, event dto.MaterialUploadedEvent) error {
	startTime := time.Now()

	p.logger.Info("processing material uploaded",
		"material_id", event.GetMaterialID(),
		"s3_key", event.GetS3Key(),
	)

	materialID, err := valueobject.MaterialIDFromString(event.GetMaterialID())
	if err != nil {
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "validation_error")
		return errors.NewValidationError("invalid material_id")
	}

	// 1. Actualizar estado a processing
	_, err = p.db.ExecContext(ctx,
		"UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
		enum.ProcessingStatusProcessing.String(),
		materialID.String(),
	)
	if err != nil {
		p.logger.Error("error actualizando estado a processing", "error", err)
		return errors.NewInternalError("failed to update status", err)
	}

	// 2. Descargar PDF desde S3 con retry logic
	p.logger.Debug("descargando PDF de S3", "s3_key", event.GetS3Key())
	s3Start := time.Now()

	var pdfReader io.ReadCloser
	retryCfg := DefaultRetryConfig(p.logger)
	err = WithRetry(ctx, retryCfg, func() error {
		var downloadErr error
		pdfReader, downloadErr = p.storageClient.Download(ctx, event.GetS3Key())
		return downloadErr
	})

	s3Status := "success"
	if err != nil {
		s3Status = "error"
	}
	metrics.RecordS3Operation("download", s3Status, time.Since(s3Start).Seconds())

	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		p.logger.Error("error descargando PDF de S3 después de reintentos",
			"error", err,
			"s3_key", event.GetS3Key(),
			"errorType", classifyError(err),
		)
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "s3_error")
		return errors.NewInternalError("failed to download PDF", err)
	}
	defer func() {
		if err := pdfReader.Close(); err != nil {
			p.logger.Error("error cerrando PDF reader", "error", err)
		}
	}()

	// 3. Extraer texto del PDF con retry logic
	p.logger.Debug("extrayendo texto del PDF")
	pdfStart := time.Now()

	var extractionResult *pdf.ExtractionResult
	err = WithRetry(ctx, retryCfg, func() error {
		var extractErr error
		extractionResult, extractErr = p.pdfExtractor.ExtractWithMetadata(ctx, pdfReader)
		return extractErr
	})

	pdfStatus := "success"
	if err != nil {
		pdfStatus = "error"
	}
	pageCount := 0
	if extractionResult != nil {
		pageCount = extractionResult.PageCount
	}
	metrics.RecordPDFExtraction(pdfStatus, time.Since(pdfStart).Seconds(), pageCount)

	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		p.logger.Error("error extrayendo texto del PDF",
			"error", err,
			"errorType", classifyError(err),
		)
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "pdf_error")
		return errors.NewInternalError("failed to extract PDF text", err)
	}

	extractedText := extractionResult.Text
	p.logger.Info("texto extraído", "pages", extractionResult.PageCount, "words", extractionResult.WordCount)

	// 4. Generar resumen con NLP con retry logic
	p.logger.Debug("generando resumen con NLP")
	nlpSummaryStart := time.Now()

	var summary *nlp.Summary
	err = WithRetry(ctx, retryCfg, func() error {
		var summaryErr error
		summary, summaryErr = p.nlpClient.GenerateSummary(ctx, extractedText)
		return summaryErr
	})

	nlpSummaryStatus := "success"
	if err != nil {
		nlpSummaryStatus = "error"
	}
	estimatedTokens := estimateTokens(extractedText)
	metrics.RecordOpenAIRequest(nlpSummaryStatus, time.Since(nlpSummaryStart).Seconds(), estimatedTokens)
	summaryProcessingMs := int(time.Since(nlpSummaryStart).Milliseconds())

	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		p.logger.Error("error generando resumen con NLP",
			"error", err,
			"errorType", classifyError(err),
			"textLength", len(extractedText),
		)
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "nlp_summary_error")
		return errors.NewInternalError("failed to generate summary", err)
	}

	// 5. Generar quiz con NLP con retry logic
	p.logger.Debug("generando quiz con NLP")
	nlpQuizStart := time.Now()

	var quiz *nlp.Quiz
	err = WithRetry(ctx, retryCfg, func() error {
		var quizErr error
		quiz, quizErr = p.nlpClient.GenerateQuiz(ctx, extractedText, 10)
		return quizErr
	})

	nlpQuizStatus := "success"
	if err != nil {
		nlpQuizStatus = "error"
	}
	estimatedQuizTokens := estimateTokens(extractedText)
	metrics.RecordOpenAIRequest(nlpQuizStatus, time.Since(nlpQuizStart).Seconds(), estimatedQuizTokens)
	quizProcessingMs := int(time.Since(nlpQuizStart).Milliseconds())

	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		p.logger.Error("error generando quiz con NLP",
			"error", err,
			"errorType", classifyError(err),
			"textLength", len(extractedText),
		)
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "nlp_quiz_error")
		return errors.NewInternalError("failed to generate quiz", err)
	}

	// 6. Guardar en MongoDB usando entidades canónicas
	dbStart := time.Now()
	err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
		now := time.Now()

		// Guardar resumen en MongoDB usando la entidad canónica MaterialSummary
		mongoStart := time.Now()
		summaryDoc := p.buildSummaryDoc(event.GetMaterialID(), summary, extractedText, summaryProcessingMs, now)
		summaryCol := p.mongodb.Collection(mongoentities.MaterialSummary{}.CollectionName())
		_, err = summaryCol.InsertOne(ctx, summaryDoc)
		metrics.RecordDatabaseOperation("mongodb", "insert", time.Since(mongoStart).Seconds(), err == nil)
		if err != nil {
			return fmt.Errorf("inserting summary: %w", err)
		}

		// Guardar quiz en MongoDB usando la entidad canónica MaterialAssessment
		mongoStart = time.Now()
		assessmentDoc := p.buildAssessmentDoc(event.GetMaterialID(), quiz, extractedText, quizProcessingMs, now)
		assessmentCol := p.mongodb.Collection(mongoentities.MaterialAssessment{}.CollectionName())
		_, err = assessmentCol.InsertOne(ctx, assessmentDoc)
		metrics.RecordDatabaseOperation("mongodb", "insert", time.Since(mongoStart).Seconds(), err == nil)
		if err != nil {
			return fmt.Errorf("inserting assessment: %w", err)
		}

		// Actualizar estado a completed
		pgStart := time.Now()
		_, err = tx.ExecContext(ctx,
			"UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
			enum.ProcessingStatusCompleted.String(),
			materialID.String(),
		)
		metrics.RecordDatabaseOperation("postgres", "update", time.Since(pgStart).Seconds(), err == nil)
		return err
	})
	metrics.RecordDatabaseOperation("postgres", "transaction", time.Since(dbStart).Seconds(), err == nil)

	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		p.logger.Error("processing failed", "error", err, "material_id", event.GetMaterialID())
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "database_error")
		return errors.NewInternalError("processing failed", err)
	}

	//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
	metrics.RecordEventProcessed("material_uploaded", "success")
	metrics.RecordProcessingDuration("material_uploaded", time.Since(startTime).Seconds())

	p.logger.Info("material processing completed", "material_id", event.GetMaterialID())
	return nil
}

// buildSummaryDoc construye el documento MaterialSummary a partir del output del NLP.
// El NLP devuelve MainIdeas []string y KeyConcepts map[string]string;
// se mapean al schema canónico (Summary string, KeyPoints []string).
func (p *MaterialUploadedProcessor) buildSummaryDoc(
	materialID string,
	summary *nlp.Summary,
	sourceText string,
	processingMs int,
	now time.Time,
) mongoentities.MaterialSummary {
	// Convertir MainIdeas a un párrafo de resumen principal
	summaryText := strings.Join(summary.MainIdeas, ". ")

	// Convertir KeyConcepts (map) a lista de puntos clave
	keyPoints := make([]string, 0, len(summary.KeyConcepts))
	for concept := range summary.KeyConcepts {
		keyPoints = append(keyPoints, concept)
	}

	return mongoentities.MaterialSummary{
		MaterialID:       materialID,
		Summary:          summaryText,
		KeyPoints:        keyPoints,
		Language:         "es",
		WordCount:        summary.WordCount,
		Version:          1,
		AIModel:          p.aiModel,
		ProcessingTimeMs: processingMs,
		Metadata: &mongoentities.SummaryMetadata{
			SourceLength: len(sourceText),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// buildAssessmentDoc construye el documento MaterialAssessment a partir del output del NLP.
func (p *MaterialUploadedProcessor) buildAssessmentDoc(
	materialID string,
	quiz *nlp.Quiz,
	sourceText string,
	processingMs int,
	now time.Time,
) mongoentities.MaterialAssessment {
	questions := make([]mongoentities.Question, len(quiz.Questions))
	totalPoints := 0

	for i, q := range quiz.Questions {
		options := make([]mongoentities.Option, len(q.Options))
		for j, opt := range q.Options {
			options[j] = mongoentities.Option{
				OptionID:   fmt.Sprintf("opt_%d", j+1),
				OptionText: opt,
			}
		}
		questions[i] = mongoentities.Question{
			QuestionID:    q.ID,
			QuestionText:  q.QuestionText,
			QuestionType:  q.QuestionType,
			Options:       options,
			CorrectAnswer: q.CorrectAnswer,
			Explanation:   q.Explanation,
			Points:        q.Points,
			Difficulty:    q.Difficulty,
		}
		totalPoints += q.Points
	}

	return mongoentities.MaterialAssessment{
		MaterialID:       materialID,
		Questions:        questions,
		TotalQuestions:   len(questions),
		TotalPoints:      totalPoints,
		Version:          1,
		AIModel:          p.aiModel,
		ProcessingTimeMs: processingMs,
		Metadata: &mongoentities.AssessmentMetadata{
			SourceLength:     len(sourceText),
			EstimatedTimeMin: len(questions) * 2,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// updateStatusToFailed actualiza el estado del material a failed
func (p *MaterialUploadedProcessor) updateStatusToFailed(ctx context.Context, materialID string) {
	_, err := p.db.ExecContext(ctx,
		"UPDATE materials SET processing_status = $1, updated_at = NOW() WHERE id = $2",
		enum.ProcessingStatusFailed.String(),
		materialID,
	)
	if err != nil {
		p.logger.Error("error actualizando estado a failed", "error", err)
	}
}

// EventType implementa la interfaz Processor
func (p *MaterialUploadedProcessor) EventType() string {
	return "material_uploaded"
}

// Process implementa la interfaz Processor
func (p *MaterialUploadedProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.MaterialUploadedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

// estimateTokens estima la cantidad de tokens en un texto
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	words := 0
	inWord := false
	for _, char := range text {
		if char == ' ' || char == '\n' || char == '\t' || char == '\r' {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return (words * 4) / 3
}
