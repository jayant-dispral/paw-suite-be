package integration

import (
	"context"
	"testing"

	"github.com/jayant-dispral/brand-threat-be/internal/adapters/repo/mongo"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
)

// Requirement: Run 'make docker-up' before running this test
func TestUserRegistrationFlow(t *testing.T) {
	// 1. Connect to Real DB (Localhost from Docker)
	client, err := mongo.NewConnection("mongodb://localhost:27017")
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}

	// Clean up: Drop collection to start fresh
	db := client.Database("sentinel_test")
	db.Collection("users").Drop(context.Background())

	// 2. Initialize Repo
	repo := mongo.NewUserRepository(db)

	// 3. Test: Save User
	ctx := context.Background()
	user := domain.User{
		Email:    "test@sentinel.com",
		Password: "hashed_secret",
	}

	id, err := repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}
	if id == "" {
		t.Error("Expected valid ID, got empty string")
	}

	// 4. Test: Retrieve User
	fetchedUser, err := repo.GetUserByEmail(ctx, "test@sentinel.com")
	if err != nil {
		t.Fatalf("Failed to fetch user: %v", err)
	}

	if fetchedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, fetchedUser.Email)
	}
}
