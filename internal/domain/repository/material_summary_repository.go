package repository

import (
	"context"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MaterialSummaryRepository define la interfaz para la persistencia de resúmenes de materiales
type MaterialSummaryRepository interface {
	// Create crea un nuevo resumen en la base de datos
	Create(ctx context.Context, summary *entities.MaterialSummary) error

	// FindByMaterialID busca un resumen por material_id
	FindByMaterialID(ctx context.Context, materialID string) (*entities.MaterialSummary, error)

	// FindByID busca un resumen por su ObjectID
	FindByID(ctx context.Context, id bson.ObjectID) (*entities.MaterialSummary, error)

	// Update actualiza un resumen existente
	Update(ctx context.Context, summary *entities.MaterialSummary) error

	// Delete elimina un resumen por material_id
	Delete(ctx context.Context, materialID string) error

	// FindByLanguage busca resúmenes por idioma
	FindByLanguage(ctx context.Context, language string, limit int64) ([]*entities.MaterialSummary, error)

	// FindRecent busca los resúmenes más recientes
	FindRecent(ctx context.Context, limit int64) ([]*entities.MaterialSummary, error)

	// CountByLanguage cuenta los resúmenes por idioma
	CountByLanguage(ctx context.Context, language string) (int64, error)

	// Exists verifica si existe un resumen para un material
	Exists(ctx context.Context, materialID string) (bool, error)
}
