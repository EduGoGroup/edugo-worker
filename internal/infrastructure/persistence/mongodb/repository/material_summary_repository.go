package repository

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/domain/entity"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrMaterialSummaryNotFound      = errors.New("material summary not found")
	ErrMaterialSummaryAlreadyExists = errors.New("material summary already exists")
)

// MaterialSummaryRepository maneja la persistencia de resúmenes de materiales en MongoDB
type MaterialSummaryRepository struct {
	collection *mongo.Collection
}

// NewMaterialSummaryRepository crea una nueva instancia del repositorio
func NewMaterialSummaryRepository(db *mongo.Database) *MaterialSummaryRepository {
	return &MaterialSummaryRepository{
		collection: db.Collection("material_summary"),
	}
}

// Create crea un nuevo resumen en MongoDB
func (r *MaterialSummaryRepository) Create(ctx context.Context, summary *entity.MaterialSummary) error {
	if !summary.IsValid() {
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

	summary.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByMaterialID busca un resumen por material_id
func (r *MaterialSummaryRepository) FindByMaterialID(ctx context.Context, materialID string) (*entity.MaterialSummary, error) {
	var summary entity.MaterialSummary

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
func (r *MaterialSummaryRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*entity.MaterialSummary, error) {
	var summary entity.MaterialSummary

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
func (r *MaterialSummaryRepository) Update(ctx context.Context, summary *entity.MaterialSummary) error {
	if !summary.IsValid() {
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
func (r *MaterialSummaryRepository) Delete(ctx context.Context, materialID string) error {
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
func (r *MaterialSummaryRepository) FindByLanguage(ctx context.Context, language string, limit int64) ([]*entity.MaterialSummary, error) {
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

	var summaries []*entity.MaterialSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

// FindRecent busca los resúmenes más recientes
func (r *MaterialSummaryRepository) FindRecent(ctx context.Context, limit int64) ([]*entity.MaterialSummary, error) {
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

	var summaries []*entity.MaterialSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, err
	}

	return summaries, nil
}

// CountByLanguage cuenta los resúmenes por idioma
func (r *MaterialSummaryRepository) CountByLanguage(ctx context.Context, language string) (int64, error) {
	filter := bson.M{"language": language}
	return r.collection.CountDocuments(ctx, filter)
}

// Exists verifica si existe un resumen para un material
func (r *MaterialSummaryRepository) Exists(ctx context.Context, materialID string) (bool, error) {
	filter := bson.M{"material_id": materialID}
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
