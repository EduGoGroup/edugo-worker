//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/testing/containers"
	"github.com/EduGoGroup/edugo-worker/internal/domain/entity"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/persistence/mongodb/repository"
	"github.com/google/uuid"
)

func TestMaterialSummaryRepository_Create(t *testing.T) {
	// Setup
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo_test",
			Username: "test_user",
			Password: "test_pass",
		}).
		Build()

	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}

	mongoDB := manager.MongoDB()
	if mongoDB == nil {
		t.Fatal("MongoDB container is nil")
	}

	db := mongoDB.Database()

	repo := repository.NewMaterialSummaryRepository(db)
	ctx := context.Background()

	// Test
	summary := entity.NewMaterialSummary(
		uuid.New().String(),
		"Este es un resumen de prueba del material educativo",
		[]string{"Punto 1", "Punto 2", "Punto 3"},
		"es",
		"gpt-4",
	)
	summary.ProcessingTimeMs = 1500

	err = repo.Create(ctx, summary)
	if err != nil {
		t.Fatalf("Failed to create summary: %v", err)
	}

	if summary.ID.IsZero() {
		t.Error("Expected ID to be set after creation")
	}

	// Verify
	found, err := repo.FindByMaterialID(ctx, summary.MaterialID)
	if err != nil {
		t.Fatalf("Failed to find summary: %v", err)
	}

	if found.MaterialID != summary.MaterialID {
		t.Errorf("Expected material_id %s, got %s", summary.MaterialID, found.MaterialID)
	}
	if found.Summary != summary.Summary {
		t.Errorf("Expected summary %s, got %s", summary.Summary, found.Summary)
	}
	if len(found.KeyPoints) != len(summary.KeyPoints) {
		t.Errorf("Expected %d key points, got %d", len(summary.KeyPoints), len(found.KeyPoints))
	}
}

func TestMaterialSummaryRepository_FindByMaterialID(t *testing.T) {
	// Setup
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo_test",
			Username: "test_user",
			Password: "test_pass",
		}).
		Build()

	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}

	mongoDB := manager.MongoDB()
	db := mongoDB.Database()
	repo := repository.NewMaterialSummaryRepository(db)
	ctx := context.Background()

	materialID := uuid.New().String()
	summary := entity.NewMaterialSummary(
		materialID,
		"Resumen de prueba",
		[]string{"Punto A", "Punto B"},
		"en",
		"gpt-4-turbo",
	)
	summary.ProcessingTimeMs = 2000

	_ = repo.Create(ctx, summary)

	// Test
	found, err := repo.FindByMaterialID(ctx, materialID)
	if err != nil {
		t.Fatalf("Failed to find summary: %v", err)
	}

	if found.MaterialID != materialID {
		t.Errorf("Expected material_id %s, got %s", materialID, found.MaterialID)
	}
}

func TestMaterialSummaryRepository_Update(t *testing.T) {
	// Setup
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo_test",
			Username: "test_user",
			Password: "test_pass",
		}).
		Build()

	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}

	mongoDB := manager.MongoDB()
	db := mongoDB.Database()
	repo := repository.NewMaterialSummaryRepository(db)
	ctx := context.Background()

	summary := entity.NewMaterialSummary(
		uuid.New().String(),
		"Resumen original",
		[]string{"Original 1"},
		"es",
		"gpt-4",
	)
	summary.ProcessingTimeMs = 1000
	_ = repo.Create(ctx, summary)

	// Test
	summary.Summary = "Resumen actualizado"
	summary.IncrementVersion()

	err = repo.Update(ctx, summary)
	if err != nil {
		t.Fatalf("Failed to update summary: %v", err)
	}

	// Verify
	found, _ := repo.FindByMaterialID(ctx, summary.MaterialID)
	if found.Summary != "Resumen actualizado" {
		t.Errorf("Expected updated summary, got %s", found.Summary)
	}
	if found.Version != 2 {
		t.Errorf("Expected version 2, got %d", found.Version)
	}
}

func TestMaterialSummaryRepository_Delete(t *testing.T) {
	// Setup
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo_test",
			Username: "test_user",
			Password: "test_pass",
		}).
		Build()

	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}

	mongoDB := manager.MongoDB()
	db := mongoDB.Database()
	repo := repository.NewMaterialSummaryRepository(db)
	ctx := context.Background()

	materialID := uuid.New().String()
	summary := entity.NewMaterialSummary(
		materialID,
		"Resumen a eliminar",
		[]string{"Delete 1"},
		"pt",
		"gpt-4",
	)
	summary.ProcessingTimeMs = 1000
	_ = repo.Create(ctx, summary)

	// Test
	err = repo.Delete(ctx, materialID)
	if err != nil {
		t.Fatalf("Failed to delete summary: %v", err)
	}

	// Verify
	_, err = repo.FindByMaterialID(ctx, materialID)
	if err != repository.ErrMaterialSummaryNotFound {
		t.Errorf("Expected ErrMaterialSummaryNotFound, got %v", err)
	}
}

func TestMaterialSummaryRepository_FindByLanguage(t *testing.T) {
	// Setup
	cfg := containers.NewConfig().
		WithMongoDB(&containers.MongoConfig{
			Database: "edugo_test",
			Username: "test_user",
			Password: "test_pass",
		}).
		Build()

	manager, err := containers.GetManager(t, cfg)
	if err != nil {
		t.Fatalf("Failed to get manager: %v", err)
	}

	mongoDB := manager.MongoDB()
	db := mongoDB.Database()
	repo := repository.NewMaterialSummaryRepository(db)
	ctx := context.Background()

	// Create test data
	for i := 0; i < 3; i++ {
		summary := entity.NewMaterialSummary(
			uuid.New().String(),
			"Resumen en español",
			[]string{"Punto"},
			"es",
			"gpt-4",
		)
		summary.ProcessingTimeMs = 1000
		_ = repo.Create(ctx, summary)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Test
	summaries, err := repo.FindByLanguage(ctx, "es", 10)
	if err != nil {
		t.Fatalf("Failed to find summaries: %v", err)
	}

	if len(summaries) < 3 {
		t.Errorf("Expected at least 3 summaries, got %d", len(summaries))
	}

	for _, s := range summaries {
		if s.Language != "es" {
			t.Errorf("Expected language 'es', got '%s'", s.Language)
		}
	}
}
