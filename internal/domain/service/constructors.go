package service

import (
	"time"

	mongoentities "github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AssessmentConstructor proporciona constructores para MaterialAssessment y sus componentes
type AssessmentConstructor struct {
	validator *AssessmentValidator
}

// NewAssessmentConstructor crea una nueva instancia
func NewAssessmentConstructor() *AssessmentConstructor {
	return &AssessmentConstructor{
		validator: NewAssessmentValidator(),
	}
}

// NewMaterialAssessment crea una nueva instancia de MaterialAssessment
func (c *AssessmentConstructor) NewMaterialAssessment(materialID string, questions []mongoentities.Question, aiModel string) *mongoentities.MaterialAssessment {
	now := time.Now()
	totalPoints := c.validator.CalculateTotalPoints(questions)

	return &mongoentities.MaterialAssessment{
		MaterialID:     materialID,
		Questions:      questions,
		TotalQuestions: len(questions),
		TotalPoints:    totalPoints,
		Version:        1,
		AIModel:        aiModel,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewQuestion crea una nueva pregunta
func (c *AssessmentConstructor) NewQuestion(questionID, questionText, questionType, correctAnswer, explanation string, points int, difficulty string) mongoentities.Question {
	return mongoentities.Question{
		QuestionID:    questionID,
		QuestionText:  questionText,
		QuestionType:  questionType,
		CorrectAnswer: correctAnswer,
		Explanation:   explanation,
		Points:        points,
		Difficulty:    difficulty,
		Options:       []mongoentities.Option{},
		Tags:          []string{},
	}
}

// AddOption agrega una opción a una pregunta
func (c *AssessmentConstructor) AddOption(question *mongoentities.Question, optionID, optionText string) {
	question.Options = append(question.Options, mongoentities.Option{
		OptionID:   optionID,
		OptionText: optionText,
	})
}

// SummaryConstructor proporciona constructores para MaterialSummary
type SummaryConstructor struct {
	validator *SummaryValidator
}

// NewSummaryConstructor crea una nueva instancia
func NewSummaryConstructor() *SummaryConstructor {
	return &SummaryConstructor{
		validator: NewSummaryValidator(),
	}
}

// NewMaterialSummary crea una nueva instancia de MaterialSummary con valores por defecto
func (c *SummaryConstructor) NewMaterialSummary(materialID, summary string, keyPoints []string, language, aiModel string) *mongoentities.MaterialSummary {
	now := time.Now()
	return &mongoentities.MaterialSummary{
		MaterialID: materialID,
		Summary:    summary,
		KeyPoints:  keyPoints,
		Language:   language,
		WordCount:  c.validator.CountWords(summary),
		Version:    1,
		AIModel:    aiModel,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// EventConstructor proporciona constructores para MaterialEvent
type EventConstructor struct{}

// NewEventConstructor crea una nueva instancia
func NewEventConstructor() *EventConstructor {
	return &EventConstructor{}
}

// NewMaterialEvent crea una nueva instancia de MaterialEvent
func (c *EventConstructor) NewMaterialEvent(eventType string, payload bson.M) *mongoentities.MaterialEvent {
	now := time.Now()
	return &mongoentities.MaterialEvent{
		EventType:  eventType,
		Payload:    payload,
		Status:     "pending",
		RetryCount: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewMaterialEventWithMaterialID crea un evento con material_id
func (c *EventConstructor) NewMaterialEventWithMaterialID(eventType, materialID string, payload bson.M) *mongoentities.MaterialEvent {
	event := c.NewMaterialEvent(eventType, payload)
	event.MaterialID = materialID
	return event
}
