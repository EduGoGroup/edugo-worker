package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"github.com/EduGoGroup/edugo-worker/internal/domain/service"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrMaterialAssessmentNotFound      = errors.New("material assessment not found")
	ErrMaterialAssessmentAlreadyExists = errors.New("material assessment already exists")
)

// MaterialAssessmentRepository maneja la persistencia de evaluaciones en MongoDB
type MaterialAssessmentRepository struct {
	collection *mongo.Collection
	validator  *service.AssessmentValidator
}

// NewMaterialAssessmentRepository crea una nueva instancia del repositorio
func NewMaterialAssessmentRepository(db *mongo.Database) *MaterialAssessmentRepository {
	return &MaterialAssessmentRepository{
		collection: db.Collection("material_assessment_worker"),
		validator:  service.NewAssessmentValidator(),
	}
}

// Create crea una nueva evaluación en MongoDB
func (r *MaterialAssessmentRepository) Create(ctx context.Context, assessment *entities.MaterialAssessment) error {
	if !r.validator.IsValid(assessment) {
		return errors.New("invalid material assessment")
	}

	assessment.CreatedAt = time.Now()
	assessment.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, assessment)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrMaterialAssessmentAlreadyExists
		}
		return err
	}

	assessment.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByMaterialID busca una evaluación por material_id
func (r *MaterialAssessmentRepository) FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialAssessment, error) {
	var assessment entities.MaterialAssessment

	filter := bson.M{"material_id": materialID}
	err := r.collection.FindOne(ctx, filter).Decode(&assessment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMaterialAssessmentNotFound
		}
		return nil, err
	}

	return &assessment, nil
}

// FindByID busca una evaluación por su ObjectID
func (r *MaterialAssessmentRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*entities.MaterialAssessment, error) {
	var assessment entities.MaterialAssessment

	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&assessment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMaterialAssessmentNotFound
		}
		return nil, err
	}

	return &assessment, nil
}

// Update actualiza una evaluación existente
func (r *MaterialAssessmentRepository) Update(ctx context.Context, assessment *entities.MaterialAssessment) error {
	if !r.validator.IsValid(assessment) {
		return errors.New("invalid material assessment")
	}

	assessment.UpdatedAt = time.Now()

	filter := bson.M{"_id": assessment.ID}
	update := bson.M{"$set": assessment}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrMaterialAssessmentNotFound
	}

	return nil
}

// Delete elimina una evaluación por material_id
func (r *MaterialAssessmentRepository) Delete(ctx context.Context, materialID string) error {
	filter := bson.M{"material_id": materialID}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return ErrMaterialAssessmentNotFound
	}

	return nil
}

// FindByDifficulty busca evaluaciones por dificultad de preguntas
func (r *MaterialAssessmentRepository) FindByDifficulty(ctx context.Context, difficulty string, limit int64) ([]*entities.MaterialAssessment, error) {
	filter := bson.M{"questions.difficulty": difficulty}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error cerrando cursor: %v", err)
		}
	}()

	var assessments []*entities.MaterialAssessment
	if err := cursor.All(ctx, &assessments); err != nil {
		return nil, err
	}

	return assessments, nil
}

// FindByTotalQuestions busca evaluaciones por número total de preguntas
func (r *MaterialAssessmentRepository) FindByTotalQuestions(ctx context.Context, minQuestions, maxQuestions int, limit int64) ([]*entities.MaterialAssessment, error) {
	filter := bson.M{
		"total_questions": bson.M{
			"$gte": minQuestions,
			"$lte": maxQuestions,
		},
	}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error cerrando cursor: %v", err)
		}
	}()

	var assessments []*entities.MaterialAssessment
	if err := cursor.All(ctx, &assessments); err != nil {
		return nil, err
	}

	return assessments, nil
}

// FindRecent busca las evaluaciones más recientes
func (r *MaterialAssessmentRepository) FindRecent(ctx context.Context, limit int64) ([]*entities.MaterialAssessment, error) {
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error cerrando cursor: %v", err)
		}
	}()

	var assessments []*entities.MaterialAssessment
	if err := cursor.All(ctx, &assessments); err != nil {
		return nil, err
	}

	return assessments, nil
}

// CountByTotalPoints cuenta evaluaciones en un rango de puntos totales
func (r *MaterialAssessmentRepository) CountByTotalPoints(ctx context.Context, minPoints, maxPoints int) (int64, error) {
	filter := bson.M{
		"total_points": bson.M{
			"$gte": minPoints,
			"$lte": maxPoints,
		},
	}
	return r.collection.CountDocuments(ctx, filter)
}

// Exists verifica si existe una evaluación para un material
func (r *MaterialAssessmentRepository) Exists(ctx context.Context, materialID string) (bool, error) {
	filter := bson.M{"material_id": materialID}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAverageQuestionCount obtiene el promedio de preguntas por evaluación
func (r *MaterialAssessmentRepository) GetAverageQuestionCount(ctx context.Context) (float64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "avg_questions", Value: bson.D{{Key: "$avg", Value: "$total_questions"}}},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Printf("Error cerrando cursor: %v", err)
		}
	}()

	var result []bson.M
	if err := cursor.All(ctx, &result); err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	avgQuestions, ok := result[0]["avg_questions"].(float64)
	if !ok {
		return 0, errors.New("unexpected average format")
	}

	return avgQuestions, nil
}
