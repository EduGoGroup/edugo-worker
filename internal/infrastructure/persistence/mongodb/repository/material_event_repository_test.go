//go:build integration

package repository_test

import (
	"context"
	"testing"

	"github.com/EduGoGroup/edugo-infrastructure/mongodb/entities"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/persistence/mongodb/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMaterialEventRepository_Create(t *testing.T) {
	// Setup
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewMaterialEventRepository(db)
	ctx := context.Background()

	// Test
	event := entities.NewMaterialEventWithMaterialID(
		entities.EventTypeMaterialUploaded,
		uuid.New().String(),
		primitive.M{"file": "test.pdf", "size": 1024},
	)

	err := repo.Create(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	if event.ID.IsZero() {
		t.Error("Expected ID to be set after creation")
	}

	// Verify
	found, err := repo.FindByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to find event: %v", err)
	}

	if found.EventType != entities.EventTypeMaterialUploaded {
		t.Errorf("Expected event_type %s, got %s", entities.EventTypeMaterialUploaded, found.EventType)
	}
	if found.Status != entities.EventStatusPending {
		t.Errorf("Expected status %s, got %s", entities.EventStatusPending, found.Status)
	}
}

func TestMaterialEventRepository_FindByStatus(t *testing.T) {
	// Setup
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewMaterialEventRepository(db)
	ctx := context.Background()

	// Create test events
	for i := 0; i < 2; i++ {
		event := entities.NewMaterialEvent(
			entities.EventTypeAssessmentAttempt,
			primitive.M{"student_id": uuid.New().String()},
		)
		_ = repo.Create(ctx, event)
	}

	// Test
	events, err := repo.FindByStatus(ctx, entities.EventStatusPending, 10)
	if err != nil {
		t.Fatalf("Failed to find events: %v", err)
	}

	if len(events) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(events))
	}

	for _, e := range events {
		if e.Status != entities.EventStatusPending {
			t.Errorf("Expected status %s, got %s", entities.EventStatusPending, e.Status)
		}
	}
}

func TestMaterialEventRepository_MarkAsCompleted(t *testing.T) {
	// Setup
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewMaterialEventRepository(db)
	ctx := context.Background()

	event := entities.NewMaterialEvent(
		entities.EventTypeMaterialUploaded,
		primitive.M{"test": "data"},
	)
	_ = repo.Create(ctx, event)

	// Test
	event.MarkAsCompleted()
	err := repo.Update(ctx, event)
	if err != nil {
		t.Fatalf("Failed to update event: %v", err)
	}

	// Verify
	found, _ := repo.FindByID(ctx, event.ID)
	if found.Status != entities.EventStatusCompleted {
		t.Errorf("Expected status %s, got %s", entities.EventStatusCompleted, found.Status)
	}
	if found.ProcessedAt == nil {
		t.Error("Expected ProcessedAt to be set")
	}
}

func TestMaterialEventRepository_GetEventStatistics(t *testing.T) {
	// Setup
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := repository.NewMaterialEventRepository(db)
	ctx := context.Background()

	// Create events with different statuses
	event1 := entities.NewMaterialEvent(entities.EventTypeMaterialUploaded, primitive.M{})
	_ = repo.Create(ctx, event1)

	event2 := entities.NewMaterialEvent(entities.EventTypeMaterialUploaded, primitive.M{})
	event2.MarkAsCompleted()
	_ = repo.Create(ctx, event2)

	// Test
	stats, err := repo.GetEventStatistics(ctx)
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	if stats[entities.EventStatusPending] < 1 {
		t.Errorf("Expected at least 1 pending event, got %d", stats[entities.EventStatusPending])
	}
	if stats[entities.EventStatusCompleted] < 1 {
		t.Errorf("Expected at least 1 completed event, got %d", stats[entities.EventStatusCompleted])
	}
}
