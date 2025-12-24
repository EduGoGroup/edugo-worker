package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MaterialUploadedProcessor procesa eventos de material subido
type MaterialUploadedProcessor struct {
	db            *sql.DB
	mongodb       *mongo.Database
	logger        logger.Logger
	storageClient storage.Client
	pdfExtractor  pdf.Extractor
	nlpClient     nlp.Client
}

// MaterialUploadedProcessorConfig contiene las dependencias del processor
type MaterialUploadedProcessorConfig struct {
	DB            *sql.DB
	MongoDB       *mongo.Database
	Logger        logger.Logger
	StorageClient storage.Client
	PDFExtractor  pdf.Extractor
	NLPClient     nlp.Client
}

func NewMaterialUploadedProcessor(cfg MaterialUploadedProcessorConfig) *MaterialUploadedProcessor {
	return &MaterialUploadedProcessor{
		db:            cfg.DB,
		mongodb:       cfg.MongoDB,
		logger:        cfg.Logger,
		storageClient: cfg.StorageClient,
		pdfExtractor:  cfg.PDFExtractor,
		nlpClient:     cfg.NLPClient,
	}
}

func (p *MaterialUploadedProcessor) processEvent(ctx context.Context, event dto.MaterialUploadedEvent) error {
	startTime := time.Now()

	p.logger.Info("processing material uploaded",
		"material_id", event.MaterialID,
		"s3_key", event.S3Key,
	)

	materialID, err := valueobject.MaterialIDFromString(event.MaterialID)
	if err != nil {
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
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

	// 2. Descargar PDF desde S3
	p.logger.Debug("descargando PDF de S3", "s3_key", event.S3Key)
	s3Start := time.Now()
	pdfReader, err := p.storageClient.Download(ctx, event.S3Key)
	s3Status := "success"
	if err != nil {
		s3Status = "error"
	}
	metrics.RecordS3Operation("download", s3Status, time.Since(s3Start).Seconds())
	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "s3_error")
		return errors.NewInternalError("failed to download PDF", err)
	}
	defer func() {
		if err := pdfReader.Close(); err != nil {
			p.logger.Error("error cerrando PDF reader", "error", err)
		}
	}()

	// 3. Extraer texto del PDF
	p.logger.Debug("extrayendo texto del PDF")
	pdfStart := time.Now()
	extractionResult, err := p.pdfExtractor.ExtractWithMetadata(ctx, pdfReader)
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
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "pdf_error")
		return errors.NewInternalError("failed to extract PDF text", err)
	}

	extractedText := extractionResult.Text
	p.logger.Info("texto extraído", "pages", extractionResult.PageCount, "words", extractionResult.WordCount)

	// 4. Generar resumen con NLP
	p.logger.Debug("generando resumen con NLP")
	nlpSummaryStart := time.Now()
	summary, err := p.nlpClient.GenerateSummary(ctx, extractedText)
	nlpSummaryStatus := "success"
	if err != nil {
		nlpSummaryStatus = "error"
	}
	// Estimar tokens: aproximadamente 1 palabra = 1.33 tokens (para inglés/español)
	estimatedTokens := estimateTokens(extractedText)
	metrics.RecordOpenAIRequest(nlpSummaryStatus, time.Since(nlpSummaryStart).Seconds(), estimatedTokens)
	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "nlp_summary_error")
		return errors.NewInternalError("failed to generate summary", err)
	}

	// 5. Generar quiz con NLP
	p.logger.Debug("generando quiz con NLP")
	nlpQuizStart := time.Now()
	quiz, err := p.nlpClient.GenerateQuiz(ctx, extractedText, 10)
	nlpQuizStatus := "success"
	if err != nil {
		nlpQuizStatus = "error"
	}
	// Estimar tokens para la generación del quiz
	estimatedQuizTokens := estimateTokens(extractedText)
	metrics.RecordOpenAIRequest(nlpQuizStatus, time.Since(nlpQuizStart).Seconds(), estimatedQuizTokens)
	if err != nil {
		p.updateStatusToFailed(ctx, materialID.String())
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "nlp_quiz_error")
		return errors.NewInternalError("failed to generate quiz", err)
	}

	// 6. Guardar en MongoDB dentro de transacción
	dbStart := time.Now()
	err = postgres.WithTransaction(ctx, p.db, func(tx *sql.Tx) error {
		// Guardar resumen en MongoDB
		mongoStart := time.Now()
		summaryCollection := p.mongodb.Collection("material_summaries")
		summaryDoc := bson.M{
			"material_id":  event.MaterialID,
			"main_ideas":   summary.MainIdeas,
			"key_concepts": summary.KeyConcepts,
			"sections":     p.sectionsToSlice(summary.Sections),
			"glossary":     summary.Glossary,
			"word_count":   summary.WordCount,
			"created_at":   summary.GeneratedAt,
		}

		_, err = summaryCollection.InsertOne(ctx, summaryDoc)
		metrics.RecordDatabaseOperation("mongodb", "insert", time.Since(mongoStart).Seconds(), err == nil)
		if err != nil {
			return err
		}

		// Guardar quiz en MongoDB
		mongoStart = time.Now()
		assessmentCollection := p.mongodb.Collection("material_assessment_worker")
		assessmentDoc := bson.M{
			"material_id": event.MaterialID,
			"questions":   p.questionsToSlice(quiz.Questions),
			"created_at":  quiz.GeneratedAt,
		}

		_, err = assessmentCollection.InsertOne(ctx, assessmentDoc)
		metrics.RecordDatabaseOperation("mongodb", "insert", time.Since(mongoStart).Seconds(), err == nil)
		if err != nil {
			return err
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
		p.logger.Error("processing failed", "error", err, "material_id", event.MaterialID)
		//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
		metrics.RecordEventProcessed("material_uploaded", "database_error")
		return errors.NewInternalError("processing failed", err)
	}

	// Registrar evento completado exitosamente
	//nolint:staticcheck // Deprecated intencional, se mantiene por compatibilidad
	metrics.RecordEventProcessed("material_uploaded", "success")
	metrics.RecordProcessingDuration("material_uploaded", time.Since(startTime).Seconds())

	p.logger.Info("material processing completed", "material_id", event.MaterialID)
	return nil
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

// sectionsToSlice convierte las secciones a formato BSON
func (p *MaterialUploadedProcessor) sectionsToSlice(sections []nlp.Section) []bson.M {
	result := make([]bson.M, len(sections))
	for i, s := range sections {
		result[i] = bson.M{
			"title":   s.Title,
			"content": s.Content,
		}
	}
	return result
}

// questionsToSlice convierte las preguntas a formato BSON
func (p *MaterialUploadedProcessor) questionsToSlice(questions []nlp.Question) []bson.M {
	result := make([]bson.M, len(questions))
	for i, q := range questions {
		result[i] = bson.M{
			"id":             q.ID,
			"question_text":  q.QuestionText,
			"question_type":  q.QuestionType,
			"options":        q.Options,
			"correct_answer": q.CorrectAnswer,
			"explanation":    q.Explanation,
			"difficulty":     q.Difficulty,
			"points":         q.Points,
		}
	}
	return result
}

// EventType implementa la interfaz Processor
func (p *MaterialUploadedProcessor) EventType() string {
	return "material_uploaded"
}

// Process implementa la interfaz Processor
// Deserializa el payload JSON y llama al método interno processEvent
func (p *MaterialUploadedProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.MaterialUploadedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}
	return p.processEvent(ctx, event)
}

// estimateTokens estima la cantidad de tokens en un texto
// Aproximación: 1 palabra ≈ 1.33 tokens para inglés/español
// Esta es una estimación conservadora basada en el modelo de OpenAI
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Contar palabras (split por espacios en blanco)
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

	// Aplicar factor de conversión: 1 palabra ≈ 1.33 tokens
	// Usamos multiplicación por 4/3 para evitar decimales
	return (words * 4) / 3
}
