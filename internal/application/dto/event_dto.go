package dto

import "time"

// MaterialUploadedEvent evento cuando se sube un material
type MaterialUploadedEvent struct {
	EventType         string    `json:"event_type"`
	MaterialID        string    `json:"material_id"`
	AuthorID          string    `json:"author_id"`
	S3Key             string    `json:"s3_key"`
	PreferredLanguage string    `json:"preferred_language"`
	Timestamp         time.Time `json:"timestamp"`
}

// AssessmentAttemptEvent evento cuando se intenta un quiz
type AssessmentAttemptEvent struct {
	EventType  string                 `json:"event_type"`
	MaterialID string                 `json:"material_id"`
	UserID     string                 `json:"user_id"`
	Answers    map[string]interface{} `json:"answers"`
	Score      float64                `json:"score"`
	Timestamp  time.Time              `json:"timestamp"`
}

// MaterialDeletedEvent evento cuando se elimina un material
type MaterialDeletedEvent struct {
	EventType  string    `json:"event_type"`
	MaterialID string    `json:"material_id"`
	Timestamp  time.Time `json:"timestamp"`
}

// StudentEnrolledEvent evento cuando un estudiante se inscribe
type StudentEnrolledEvent struct {
	EventType string    `json:"event_type"`
	StudentID string    `json:"student_id"`
	UnitID    string    `json:"unit_id"`
	Timestamp time.Time `json:"timestamp"`
}
