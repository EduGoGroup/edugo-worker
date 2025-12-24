package repository

import (
	"context"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaterialAssessmentRepository define la interfaz para la persistencia de evaluaciones
type MaterialAssessmentRepository interface {
	// Create crea una nueva evaluación en la base de datos
	Create(ctx context.Context, assessment *entities.MaterialAssessment) error

	// FindByMaterialID busca una evaluación por material_id
	FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialAssessment, error)

	// FindByID busca una evaluación por su ObjectID
	FindByID(ctx context.Context, id primitive.ObjectID) (*entities.MaterialAssessment, error)

	// Update actualiza una evaluación existente
	Update(ctx context.Context, assessment *entities.MaterialAssessment) error

	// Delete elimina una evaluación por material_id
	Delete(ctx context.Context, materialID string) error

	// FindByDifficulty busca evaluaciones por dificultad de preguntas
	FindByDifficulty(ctx context.Context, difficulty string, limit int64) ([]*entities.MaterialAssessment, error)

	// FindByTotalQuestions busca evaluaciones por número total de preguntas
	FindByTotalQuestions(ctx context.Context, minQuestions, maxQuestions int, limit int64) ([]*entities.MaterialAssessment, error)

	// FindRecent busca las evaluaciones más recientes
	FindRecent(ctx context.Context, limit int64) ([]*entities.MaterialAssessment, error)

	// CountByTotalPoints cuenta evaluaciones en un rango de puntos totales
	CountByTotalPoints(ctx context.Context, minPoints, maxPoints int) (int64, error)

	// Exists verifica si existe una evaluación para un material
	Exists(ctx context.Context, materialID string) (bool, error)

	// GetAverageQuestionCount obtiene el promedio de preguntas por evaluación
	GetAverageQuestionCount(ctx context.Context) (float64, error)
}
