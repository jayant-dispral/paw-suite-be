package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jayant-dispral/brand-threat-be/internal/adapters/repo/mongo"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserRepositoryLifecycle covers the entire CRUD journey for the user repository.
func TestUserRepositoryLifecycle(t *testing.T) {
	// The setup is now handled by TestMain in main_test.go
	// We can directly use the testDB variable.
	repo := mongo.NewUserRepository(testDB)
	collection := testDB.Collection("users")

	// Clean slate for this specific test
	err := collection.Drop(context.Background())
	require.NoError(t, err, "Failed to drop users collection before test")

	// Re-create indexes since we dropped the collection

	//TODO:P1:fix below code
	// repo.CreateIndexes(context.Background())
	time.Sleep(100 * time.Millisecond) // Give indexes time to build

	testEmail := "lifecycle@example.com"
	var createdUserID string

	t.Run("1. Save user successfully", func(t *testing.T) {
		user := domain.User{
			Email:    testEmail,
			Password: "hashed_password",
			FullName: "Test User",
		}
		id, err := repo.Save(context.Background(), user)
		require.NoError(t, err)
		require.NotEmpty(t, id)
		createdUserID = id
	})

	t.Run("2. Fail to save duplicate user", func(t *testing.T) {
		user := domain.User{Email: testEmail}
		_, err := repo.Save(context.Background(), user)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrConflict))
	})

	t.Run("3. Get user by email", func(t *testing.T) {
		user, err := repo.GetUserByEmail(context.Background(), testEmail)
		require.NoError(t, err)
		assert.Equal(t, testEmail, user.Email)
		assert.Equal(t, "Test User", user.FullName)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())
	})

	t.Run("4. Get user by ID", func(t *testing.T) {
		require.NotEmpty(t, createdUserID, "createdUserID should not be empty")
		user, err := repo.GetUserById(context.Background(), createdUserID)
		require.NoError(t, err)
		assert.Equal(t, testEmail, user.Email)
	})

	t.Run("5. Fail to get non-existent user", func(t *testing.T) {
		_, err := repo.GetUserById(context.Background(), "507f1f77bcf86cd799439011") // valid but non-existent ID
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})

	t.Run("6. Update user", func(t *testing.T) {
		require.NotEmpty(t, createdUserID, "createdUserID should not be empty")
		newName := "Updated Name"
		update := domain.UpdateUserStruct{FullName: &newName}

		err := repo.UpdateUser(context.Background(), createdUserID, update)
		require.NoError(t, err)

		// Verify
		user, err := repo.GetUserById(context.Background(), createdUserID)
		require.NoError(t, err)
		assert.Equal(t, newName, user.FullName)
		assert.True(t, user.UpdatedAt.After(user.CreatedAt))
	})

	t.Run("7. Fail to update non-existent user", func(t *testing.T) {
		newName := "Ghost Name"
		update := domain.UpdateUserStruct{FullName: &newName}
		err := repo.UpdateUser(context.Background(), "507f1f77bcf86cd799439011", update)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound))
	})
}
