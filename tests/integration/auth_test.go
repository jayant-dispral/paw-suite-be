package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jayant-dispral/brand-threat-be/internal/adapters/repo/mongo"
	"github.com/jayant-dispral/brand-threat-be/internal/config"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
)

// TestUserRepository_Lifecycle covers the entire CRUD journey and edge cases.
// Requirement: Run 'make docker-up' before running this test.
func TestUserRepository_Lifecycle(t *testing.T) {
	// --- SETUP ---
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Fallback for testing if .env isn't set
	uri := cfg.MongoDBDatabaseURI
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	client, err := mongo.NewConnection(uri)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}

	// Use a dedicated test database
	db := client.Database("sentinel_integration_test")

	// Clean slate: Drop collection before testing
	// This ensures previous failed tests don't pollute this run
	_ = db.Collection("users").Drop(context.Background())

	// Initialize Repo (This triggers the Index Creation!)
	repo := mongo.NewUserRepository(db)

	// Wait a moment for index creation goroutine to finish (Integration test hack)
	time.Sleep(100 * time.Millisecond)

	// Shared State
	testEmail := "test@sentinel.com"
	var createdUserID string

	// --- TESTS ---

	t.Run("1. Create User - Happy Path", func(t *testing.T) {
		ctx := context.Background()
		user := domain.User{
			Email:    testEmail,
			Password: "hashed_secret_password",
		}

		id, err := repo.Save(ctx, user)
		if err != nil {
			t.Fatalf("Failed to save user: %v", err)
		}
		if id == "" {
			t.Error("Expected valid ID, got empty string")
		}
		createdUserID = id // Save for later tests
	})

	t.Run("2. Create Duplicate User - Should Fail", func(t *testing.T) {
		// Attempt to save the SAME email again
		ctx := context.Background()
		user := domain.User{
			Email:    testEmail, // Same email
			Password: "different_password",
		}

		_, err := repo.Save(ctx, user)
		if err == nil {
			t.Fatal("Expected error for duplicate email, got nil")
		}

		// Check for either the clean error OR the raw Mongo error (E11000)
		errMsg := err.Error()
		if errMsg != "email already exists" && !strings.Contains(errMsg, "E11000") && !strings.Contains(errMsg, "duplicate") {
			t.Errorf("Expected duplicate/exists error, got: %v", err)
		}
	})

	t.Run("3. Get User By Email - Happy Path", func(t *testing.T) {
		ctx := context.Background()
		fetchedUser, err := repo.GetUserByEmail(ctx, testEmail)
		if err != nil {
			t.Fatalf("Failed to fetch user: %v", err)
		}

		if fetchedUser.Email != testEmail {
			t.Errorf("Expected email %s, got %s", testEmail, fetchedUser.Email)
		}
		// Check if timestamps were set
		if fetchedUser.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set, got Zero time")
		}

		// Update the createdUserID with the definitive ID from the DB
		// This ensures Test 4 uses the correct ID even if Save() returned something slightly different
		createdUserID = fetchedUser.ID.Hex()
	})

	t.Run("4. Get User By ID - Happy Path", func(t *testing.T) {
		ctx := context.Background()
		// Ensure we have an ID to test with
		if createdUserID == "" {
			t.Fatal("Skipping test: No createdUserID available from previous steps")
		}

		fetchedUser, err := repo.GetUserById(ctx, createdUserID)
		if err != nil {
			t.Fatalf("Failed to fetch user by ID: %v", err)
		}
		if fetchedUser.Email != testEmail {
			t.Errorf("Expected email %s, got %s", testEmail, fetchedUser.Email)
		}
	})

	t.Run("5. Get Non-Existent User - Should Fail Gracefully", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.GetUserByEmail(ctx, "ghost@sentinel.com")

		if err == nil {
			t.Fatal("Expected error for non-existent user, got nil")
		}
		if err.Error() != "user not found" {
			t.Errorf("Expected 'user not found' error, got: %v", err)
		}
	})

	t.Run("6. Context Timeout - Resiliency Check", func(t *testing.T) {
		// Create a context that dies INSTANTLY
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		_, err := repo.GetUserByEmail(ctx, testEmail)
		if err == nil {
			t.Fatal("Expected timeout error, got success")
		}
		// MongoDB driver usually returns "context deadline exceeded"
	})
}
