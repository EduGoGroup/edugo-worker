package processor

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/google/uuid"
)

// NotificationCreator inserts in-app notifications into PostgreSQL.
type NotificationCreator struct {
	db     *sql.DB
	logger logger.Logger
}

// NewNotificationCreator creates a new NotificationCreator.
func NewNotificationCreator(db *sql.DB, logger logger.Logger) *NotificationCreator {
	return &NotificationCreator{db: db, logger: logger}
}

// Create inserts a single notification row into notifications.notifications.
func (nc *NotificationCreator) Create(ctx context.Context, userID uuid.UUID, notifType, title, body, resourceType string, resourceID uuid.UUID) error {
	id := uuid.New()
	query := `INSERT INTO notifications.notifications (id, user_id, type, title, body, resource_type, resource_id, is_read, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, false, NOW())`

	_, err := nc.db.ExecContext(ctx, query, id, userID, notifType, title, body, resourceType, resourceID)
	if err != nil {
		return fmt.Errorf("failed to insert notification: %w", err)
	}

	nc.logger.Info("notification created",
		"notification_id", id.String(),
		"user_id", userID.String(),
		"type", notifType,
	)
	return nil
}
