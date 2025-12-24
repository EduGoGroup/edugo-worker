package repository

import (
	"context"
	"time"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaterialEventRepository define la interfaz para la persistencia de eventos de auditoría
type MaterialEventRepository interface {
	// Create crea un nuevo evento en la base de datos
	Create(ctx context.Context, event *entities.MaterialEvent) error

	// FindByID busca un evento por su ObjectID
	FindByID(ctx context.Context, id primitive.ObjectID) (*entities.MaterialEvent, error)

	// Update actualiza un evento existente
	Update(ctx context.Context, event *entities.MaterialEvent) error

	// FindByMaterialID busca eventos por material_id
	FindByMaterialID(ctx context.Context, materialID string, limit int64) ([]*entities.MaterialEvent, error)

	// FindByEventType busca eventos por tipo
	FindByEventType(ctx context.Context, eventType string, limit int64) ([]*entities.MaterialEvent, error)

	// FindByStatus busca eventos por estado
	FindByStatus(ctx context.Context, status string, limit int64) ([]*entities.MaterialEvent, error)

	// FindFailedEvents busca eventos fallidos que pueden reintentarse
	FindFailedEvents(ctx context.Context, maxRetries int, limit int64) ([]*entities.MaterialEvent, error)

	// FindPendingEvents busca eventos pendientes de procesar
	FindPendingEvents(ctx context.Context, limit int64) ([]*entities.MaterialEvent, error)

	// FindRecent busca los eventos más recientes
	FindRecent(ctx context.Context, limit int64) ([]*entities.MaterialEvent, error)

	// CountByStatus cuenta eventos por estado
	CountByStatus(ctx context.Context, status string) (int64, error)

	// CountByEventType cuenta eventos por tipo
	CountByEventType(ctx context.Context, eventType string) (int64, error)

	// GetEventStatistics obtiene estadísticas de eventos
	GetEventStatistics(ctx context.Context) (map[string]int64, error)

	// DeleteOldEvents elimina eventos antiguos
	DeleteOldEvents(ctx context.Context, olderThan time.Time) (int64, error)
}
