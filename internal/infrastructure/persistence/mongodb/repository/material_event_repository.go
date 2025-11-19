package repository

import (
	"context"
	"errors"
	"time"

	"github.com/EduGoGroup/edugo-worker/internal/domain/entity"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrMaterialEventNotFound = errors.New("material event not found")
)

// MaterialEventRepository maneja la persistencia de eventos de auditoría en MongoDB
type MaterialEventRepository struct {
	collection *mongo.Collection
}

// NewMaterialEventRepository crea una nueva instancia del repositorio
func NewMaterialEventRepository(db *mongo.Database) *MaterialEventRepository {
	return &MaterialEventRepository{
		collection: db.Collection("material_event"),
	}
}

// Create crea un nuevo evento en MongoDB
func (r *MaterialEventRepository) Create(ctx context.Context, event *entity.MaterialEvent) error {
	if !event.IsValid() {
		return errors.New("invalid material event")
	}

	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, event)
	if err != nil {
		return err
	}

	event.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// FindByID busca un evento por su ObjectID
func (r *MaterialEventRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*entity.MaterialEvent, error) {
	var event entity.MaterialEvent

	filter := bson.M{"_id": id}
	err := r.collection.FindOne(ctx, filter).Decode(&event)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrMaterialEventNotFound
		}
		return nil, err
	}

	return &event, nil
}

// Update actualiza un evento existente
func (r *MaterialEventRepository) Update(ctx context.Context, event *entity.MaterialEvent) error {
	if !event.IsValid() {
		return errors.New("invalid material event")
	}

	event.UpdatedAt = time.Now()

	filter := bson.M{"_id": event.ID}
	update := bson.M{"$set": event}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return ErrMaterialEventNotFound
	}

	return nil
}

// FindByMaterialID busca eventos por material_id
func (r *MaterialEventRepository) FindByMaterialID(ctx context.Context, materialID string, limit int64) ([]*entity.MaterialEvent, error) {
	filter := bson.M{"material_id": materialID}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// FindByEventType busca eventos por tipo
func (r *MaterialEventRepository) FindByEventType(ctx context.Context, eventType string, limit int64) ([]*entity.MaterialEvent, error) {
	filter := bson.M{"event_type": eventType}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// FindByStatus busca eventos por estado
func (r *MaterialEventRepository) FindByStatus(ctx context.Context, status string, limit int64) ([]*entity.MaterialEvent, error) {
	filter := bson.M{"status": status}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// FindFailedEvents busca eventos fallidos que pueden reintentarse
func (r *MaterialEventRepository) FindFailedEvents(ctx context.Context, maxRetries int, limit int64) ([]*entity.MaterialEvent, error) {
	filter := bson.M{
		"status":      entity.EventStatusFailed,
		"retry_count": bson.M{"$lt": maxRetries},
	}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// FindPendingEvents busca eventos pendientes de procesar
func (r *MaterialEventRepository) FindPendingEvents(ctx context.Context, limit int64) ([]*entity.MaterialEvent, error) {
	filter := bson.M{"status": entity.EventStatusPending}
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: 1}}) // FIFO: más antiguos primero

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// FindRecent busca los eventos más recientes
func (r *MaterialEventRepository) FindRecent(ctx context.Context, limit int64) ([]*entity.MaterialEvent, error) {
	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*entity.MaterialEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// CountByStatus cuenta eventos por estado
func (r *MaterialEventRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	filter := bson.M{"status": status}
	return r.collection.CountDocuments(ctx, filter)
}

// CountByEventType cuenta eventos por tipo
func (r *MaterialEventRepository) CountByEventType(ctx context.Context, eventType string) (int64, error) {
	filter := bson.M{"event_type": eventType}
	return r.collection.CountDocuments(ctx, filter)
}

// GetEventStatistics obtiene estadísticas de eventos
func (r *MaterialEventRepository) GetEventStatistics(ctx context.Context) (map[string]int64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$status"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stats := make(map[string]int64)
	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		stats[result.ID] = result.Count
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// DeleteOldEvents elimina eventos antiguos (útil para pruebas, el TTL index hace esto automáticamente)
func (r *MaterialEventRepository) DeleteOldEvents(ctx context.Context, olderThan time.Time) (int64, error) {
	filter := bson.M{
		"created_at": bson.M{"$lt": olderThan},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}
