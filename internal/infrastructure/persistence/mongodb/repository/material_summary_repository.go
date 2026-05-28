package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"github.com/EduGoGroup/edugo-worker/internal/domain/repository"
	"github.com/EduGoGroup/edugo-worker/internal/domain/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrMaterialSummaryNotFound      = errors.New("material summary not found")
	ErrMaterialSummaryAlreadyExists = errors.New("material summary already exists")
)

// MongoMaterialSummaryRepository implementa la interfaz MaterialSummaryRepository usando MongoDB
type MongoMaterialSummaryRepository struct {
	collection *mongo.Collection
	validator  *service.SummaryValidator
}

// Verificar que MongoMaterialSummaryRepository implementa repository.MaterialSummaryRepository
var _ repository.MaterialSummaryRepository = (*MongoMaterialSummaryRepository)(nil)

// NewMaterialSummaryRepository crea una nueva instancia del repositorio
func NewMaterialSummaryRepository(db *mongo.Database) repository.MaterialSummaryRepository {
	return &MongoMaterialSummaryRepository{
		collection: db.Collection("material_summary"),
		validator:  service.NewSummaryValidator(),
	}
}

// Create crea un nuevo resumen en MongoDB
func (r *MongoMaterialSummaryRepository) Create(ctx context.Context, summary *entities.MaterialSummary) error {
	if !r.validator.IsValid(summary) {
		return errors.New("invalid material summary")
	}

	summary.CreatedAt = time.Now()
	summary.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, summary)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrMaterialSummaryAlreadyExists
		}
		return err
	}

	summary.ID = result.InsertedID.(bson.ObjectID)
	return nil
}

// FindByMaterialID busca un resumen por material_id
func (r *MongoMaterialSummaryRepository) FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialSummary, error) {
	var summary entities.MaterialSummary

	filter := bson.M{"material_id": materialID}
	err := r.collection.FindOne(ctx, filter).Decode(&summary)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMaterialSummaryNotFound
		}
		return nil, err
	}

	return &summary, nil
}

// FindByID busca un resumen por su ObjectID
func (r *MongoMaterialSummaryRepository) FindByID(ctx context.Context, id bson.ObjectID) (*entities.MaterialSummary, error) {
	var summary entities.MaterialSummary

	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&summary)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMaterialSummaryNotFound
		}
		return nil, err
	}

	return &summary, nil
}

// Update actualiza un resumen existente
func (r *MongoMaterialSummaryRepository) Update(ctx context.Context, summary *entities.MaterialSummary) error {
	if !r.validator.IsValid(summary) {
		return errors.New("invalid material summary")
	}

	summary.UpdatedAt = time.Now()

	filter := bson.M{"_id": summary.ID}
	update := bson.M{"$set": summary}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrMaterialSummaryNotFound
	}

	return nil
}

// Delete elimina un resumen por material_id
func (r *MongoMaterialSummaryRepository) Delete(ctx context.Context, materialID string) error {
	filter := bson.M{"material_id": materialID}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return ErrMaterialSummaryNotFound
	}

	return nil
}

// FindByLanguage busca resúmenes por idioma
func (r *MongoMaterialSummaryRepository) FindByLanguage(ctx context.Context, language string, limit int64) ([]*entities.MaterialSummary, error) {
	filter := bson.M{"language": language}
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

	var summaries []*entities.MaterialSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

// FindRecent busca los resúmenes más recientes
func (r *MongoMaterialSummaryRepository) FindRecent(ctx context.Context, limit int64) ([]*entities.MaterialSummary, error) {
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

	var summaries []*entities.MaterialSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

// CountByLanguage cuenta los resúmenes por idioma
func (r *MongoMaterialSummaryRepository) CountByLanguage(ctx context.Context, language string) (int64, error) {
	filter := bson.M{"language": language}
	return r.collection.CountDocuments(ctx, filter)
}

// Exists verifica si existe un resumen para un material
func (r *MongoMaterialSummaryRepository) Exists(ctx context.Context, materialID string) (bool, error) {
	filter := bson.M{"material_id": materialID}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
