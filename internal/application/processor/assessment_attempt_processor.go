package processor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/common/errors"
	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/application/dto"
	"github.com/google/uuid"
)

// AssessmentAttemptProcessor handles "assessment.attempt_recorded" events.
// It records analytics data in PostgreSQL and delegates notification creation
// to the notification sub-processor.
type AssessmentAttemptProcessor struct {
	db        *sql.DB
	notifProc *AssessmentAttemptNotifProcessor
	logger    logger.Logger
}

// NewAssessmentAttemptProcessor creates a new processor with DB access for analytics
// and an optional notification sub-processor for teacher notifications.
func NewAssessmentAttemptProcessor(db *sql.DB, notifProc *AssessmentAttemptNotifProcessor, logger logger.Logger) *AssessmentAttemptProcessor {
	return &AssessmentAttemptProcessor{db: db, notifProc: notifProc, logger: logger}
}

func (p *AssessmentAttemptProcessor) EventType() string {
	return "assessment.attempt_recorded"
}

func (p *AssessmentAttemptProcessor) Process(ctx context.Context, payload []byte) error {
	var event dto.AssessmentAttemptEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.NewValidationError("invalid event payload")
	}

	if err := p.validatePayload(event.Payload); err != nil {
		return err
	}

	return p.processEvent(ctx, event)
}

// validatePayload checks that required fields are present.
func (p *AssessmentAttemptProcessor) validatePayload(pl dto.AssessmentAttemptPayload) error {
	if pl.AttemptID == "" {
		return errors.NewValidationError("attempt_id is required")
	}
	if pl.AssessmentID == "" {
		return errors.NewValidationError("assessment_id is required")
	}
	if pl.StudentID == "" {
		return errors.NewValidationError("student_id is required")
	}
	if pl.SchoolID == "" {
		return errors.NewValidationError("school_id is required")
	}
	return nil
}

func (p *AssessmentAttemptProcessor) processEvent(ctx context.Context, event dto.AssessmentAttemptEvent) error {
	pl := event.Payload

	p.logger.Info("processing assessment.attempt_recorded",
		"attempt_id", pl.AttemptID,
		"assessment_id", pl.AssessmentID,
		"student_id", pl.StudentID,
		"score", pl.Score,
		"total_points", pl.TotalPoints,
	)

	// 1. Insert analytics record
	if err := p.insertAnalytics(ctx, event); err != nil {
		return fmt.Errorf("inserting analytics: %w", err)
	}

	// 2. Update cumulative assessment stats
	if err := p.updateAssessmentStats(ctx, event); err != nil {
		p.logger.Error("failed to update assessment stats (non-fatal)",
			"assessment_id", pl.AssessmentID,
			"error", err.Error(),
		)
		// Non-fatal: analytics already recorded
	}

	// 3. Detect low score (< 50%)
	if pl.TotalPoints > 0 && pl.Score/pl.TotalPoints < 0.5 {
		p.logger.Warn("low score detected",
			"attempt_id", pl.AttemptID,
			"student_id", pl.StudentID,
			"score", pl.Score,
			"total_points", pl.TotalPoints,
			"percentage", pl.Score/pl.TotalPoints*100,
		)
	}

	// 4. Delegate notification creation to sub-processor
	if p.notifProc != nil {
		if err := p.notifProc.processEvent(ctx, event); err != nil {
			p.logger.Error("failed to create attempt notification (non-fatal)",
				"attempt_id", pl.AttemptID,
				"error", err.Error(),
			)
			// Non-fatal: analytics already recorded
		}
	}

	p.logger.Info("assessment.attempt_recorded processed successfully",
		"attempt_id", pl.AttemptID,
	)
	return nil
}

// insertAnalytics inserts a row into assessment.attempt_analytics with SHA1 idempotency.
func (p *AssessmentAttemptProcessor) insertAnalytics(ctx context.Context, event dto.AssessmentAttemptEvent) error {
	pl := event.Payload

	// Deterministic ID for idempotency
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(pl.AttemptID+"analytics"))

	attemptID, err := uuid.Parse(pl.AttemptID)
	if err != nil {
		return fmt.Errorf("invalid attempt_id: %w", err)
	}
	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}
	studentID, err := uuid.Parse(pl.StudentID)
	if err != nil {
		return fmt.Errorf("invalid student_id: %w", err)
	}
	schoolID, err := uuid.Parse(pl.SchoolID)
	if err != nil {
		return fmt.Errorf("invalid school_id: %w", err)
	}

	// Calculate percentage if not provided
	percentage := pl.Percentage
	if percentage == 0 && pl.TotalPoints > 0 {
		percentage = (pl.Score / pl.TotalPoints) * 100
	}

	// Parse submitted_at or use event timestamp
	submittedAt := event.Timestamp
	if pl.SubmittedAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, pl.SubmittedAt); parseErr == nil {
			submittedAt = parsed
		}
	}

	query := `INSERT INTO assessment.attempt_analytics
		(id, attempt_id, assessment_id, student_id, school_id, score, total_points, percentage, duration_seconds, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING`

	_, err = p.db.ExecContext(ctx, query,
		id, attemptID, assessmentID, studentID, schoolID,
		pl.Score, pl.TotalPoints, percentage,
		nilIfZero(pl.DurationSeconds), submittedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert analytics: %w", err)
	}

	p.logger.Info("analytics record inserted",
		"analytics_id", id.String(),
		"attempt_id", pl.AttemptID,
	)
	return nil
}

// updateAssessmentStats updates cumulative statistics for the assessment using UPSERT.
func (p *AssessmentAttemptProcessor) updateAssessmentStats(ctx context.Context, event dto.AssessmentAttemptEvent) error {
	pl := event.Payload

	assessmentID, err := uuid.Parse(pl.AssessmentID)
	if err != nil {
		return fmt.Errorf("invalid assessment_id: %w", err)
	}

	percentage := pl.Percentage
	if percentage == 0 && pl.TotalPoints > 0 {
		percentage = (pl.Score / pl.TotalPoints) * 100
	}

	query := `INSERT INTO assessment.assessment_stats (id, assessment_id, attempt_count, average_score, average_percentage, updated_at)
		VALUES ($1, $2, 1, $3, $4, NOW())
		ON CONFLICT (assessment_id) DO UPDATE SET
			attempt_count = assessment.assessment_stats.attempt_count + 1,
			average_score = (assessment.assessment_stats.average_score * assessment.assessment_stats.attempt_count + $3) / (assessment.assessment_stats.attempt_count + 1),
			average_percentage = (assessment.assessment_stats.average_percentage * assessment.assessment_stats.attempt_count + $4) / (assessment.assessment_stats.attempt_count + 1),
			updated_at = NOW()`

	statsID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(pl.AssessmentID+"stats"))

	_, err = p.db.ExecContext(ctx, query, statsID, assessmentID, pl.Score, percentage)
	if err != nil {
		return fmt.Errorf("failed to upsert assessment stats: %w", err)
	}

	p.logger.Info("assessment stats updated",
		"assessment_id", pl.AssessmentID,
	)
	return nil
}

// nilIfZero returns nil for zero values, allowing NULL in the DB.
func nilIfZero(v int) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
