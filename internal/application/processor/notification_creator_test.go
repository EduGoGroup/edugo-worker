package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationCreator_Create_Success(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)

	userID := uuid.New()
	resourceID := uuid.New()

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(), // id (generated)
			userID,
			"assessment_assigned",
			"Test Title",
			"Test Body",
			"assessment",
			resourceID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Act
	err = nc.Create(context.Background(), userID, "assessment_assigned", "Test Title", "Test Body", "assessment", resourceID)

	// Assert
	assert.NoError(t, err)
	assert.NoError(t, dbMock.ExpectationsWereMet())
}

func TestNotificationCreator_Create_DeterministicID(t *testing.T) {
	// Verify that calling Create twice with the same params produces the same deterministic UUID
	// (idempotency via uuid.NewSHA1).
	userID := uuid.New()
	resourceID := uuid.New()
	notifType := "assessment_assigned"
	resourceType := "assessment"

	id1 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(userID.String()+notifType+resourceType+resourceID.String()))
	id2 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(userID.String()+notifType+resourceType+resourceID.String()))

	assert.Equal(t, id1, id2, "deterministic IDs should match for the same input")

	// Different inputs should produce different IDs
	otherUserID := uuid.New()
	id3 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(otherUserID.String()+notifType+resourceType+resourceID.String()))
	assert.NotEqual(t, id1, id3, "different inputs should produce different IDs")
}

func TestNotificationCreator_Create_DBError(t *testing.T) {
	// Arrange
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	logger := newTestLogger()
	nc := NewNotificationCreator(db, logger)

	userID := uuid.New()
	resourceID := uuid.New()

	dbMock.ExpectExec("INSERT INTO notifications.notifications").
		WithArgs(
			sqlmock.AnyArg(),
			userID,
			"assessment_assigned",
			"Test Title",
			"Test Body",
			"assessment",
			resourceID,
		).
		WillReturnError(fmt.Errorf("connection refused"))

	// Act
	err = nc.Create(context.Background(), userID, "assessment_assigned", "Test Title", "Test Body", "assessment", resourceID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to insert notification")
	assert.Contains(t, err.Error(), "connection refused")
	assert.NoError(t, dbMock.ExpectationsWereMet())
}
