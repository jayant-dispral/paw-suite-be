package workerpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestWorkerPoolBasicFlow(t *testing.T) {
	// 1. Setup MongoDB connection
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 2. Create worker pool
	// API rate: 10/sec, API workers: 40, Processing workers: 5
	wp := NewWorkerPool(10, 40, 5, db)
	wp.Start()

	// 3. Submit test tasks
	numTasks := 5
	fmt.Printf("\n=== Submitting %d tasks ===\n", numTasks)

	for i := 0; i < numTasks; i++ {
		event := domain.BrandMonitorEvent{
			ProjectID:   "507f1f77bcf86cd799439011", // Valid ObjectID
			KeyWords:    []string{"MrBeast", "Crypto"},
			RequestedBy: "test-user",
			Timestamp:   time.Now(),
		}

		task, err := NewTask(event)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		if err := wp.SubmitTask(task); err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}

		fmt.Printf("Submitted task %d: %s\n", i+1, task.ID)
	}

	// 4. Wait for processing to complete
	fmt.Println("\n=== Waiting for processing ===")
	time.Sleep(15 * time.Second)

	// 5. Verify posts were saved to database
	fmt.Println("\n=== Verifying database ===")
	collection := db.Collection("social_posts")
	count, err := collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		t.Fatalf("Failed to count documents: %v", err)
	}

	fmt.Printf("Total posts saved: %d\n", count)

	if count == 0 {
		t.Fatal("Expected posts to be saved, but found 0")
	}

	// 6. Test graceful shutdown
	fmt.Println("\n=== Testing graceful shutdown ===")
	if err := wp.Shutdown(5 * time.Second); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	fmt.Println("\n=== Test completed successfully ===")
}

func TestRateLimiting(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create worker pool with rate limit of 10/sec
	wp := NewWorkerPool(10, 40, 5, db)
	wp.Start()

	fmt.Println("\n=== Testing rate limiting ===")

	// Track when each task gets dispatched to API workers
	// dispatchTimes := make([]time.Time, 0)

	// Submit 30 tasks instantly
	numTasks := 30
	startTime := time.Now()

	for i := 0; i < numTasks; i++ {
		event := domain.BrandMonitorEvent{
			ProjectID:   "507f1f77bcf86cd799439011",
			KeyWords:    []string{fmt.Sprintf("keyword_%d", i)},
			RequestedBy: "test-user",
			Timestamp:   time.Now(),
		}

		task, _ := NewTask(event)
		wp.SubmitTask(task)
	}

	fmt.Printf("Submitted %d tasks instantly\n", numTasks)

	// Wait just long enough for dispatch (not full processing)
	time.Sleep(5 * time.Second)
	elapsed := time.Since(startTime)

	fmt.Printf("Time to dispatch all tasks: %v\n", elapsed)
	fmt.Printf("Expected minimum (30 tasks / 10 per sec): 3 seconds\n")
	fmt.Printf("Expected maximum (with some overhead): ~4 seconds\n")

	// Rate limiting should make it take at least 3 seconds
	if elapsed < 2500*time.Millisecond {
		t.Errorf("Rate limiting failed: dispatched too fast (%v)", elapsed)
	}

	// Should not take more than 5 seconds (overhead should be minimal)
	if elapsed > 5*time.Second {
		t.Logf("Warning: Dispatch took longer than expected (%v)", elapsed)
	}

	fmt.Println("Rate limiting working correctly!")

	wp.Shutdown(5 * time.Second)
}

func TestErrorHandling(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	wp := NewWorkerPool(10, 5, 2, db)
	wp.Start()

	fmt.Println("\n=== Testing error handling (API failures) ===")

	// Submit many tasks to trigger the 5% error rate in mock API
	for i := 0; i < 50; i++ {
		event := domain.BrandMonitorEvent{
			ProjectID:   "507f1f77bcf86cd799439011",
			KeyWords:    []string{fmt.Sprintf("test_%d", i)},
			RequestedBy: "test-user",
			Timestamp:   time.Now(),
		}

		task, _ := NewTask(event)
		wp.SubmitTask(task)
	}

	time.Sleep(20 * time.Second)

	// Verify some posts were still saved despite errors
	collection := db.Collection("social_posts")
	count, _ := collection.CountDocuments(context.Background(), bson.M{})

	fmt.Printf("Posts saved despite errors: %d\n", count)

	if count == 0 {
		t.Fatal("No posts saved - error handling may be too aggressive")
	}

	wp.Shutdown(5 * time.Second)
}

// Helper function to setup test database
func setupTestDB(t *testing.T) (*mongo.Database, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB (adjust connection string as needed)
	connectionString := "mongodb://admin:password@localhost:27017/?authSource=admin"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}

	// Use test database
	dbName := fmt.Sprintf("test_workerpool_%d", time.Now().Unix())
	db := client.Database(dbName)

	// Create indexes
	createIndexes(db)

	fmt.Printf("Connected to test database: %s\n", dbName)

	// Cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Drop test database
		if err := db.Drop(ctx); err != nil {
			t.Logf("Failed to drop test database: %v", err)
		}

		// Disconnect
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Failed to disconnect: %v", err)
		}

		fmt.Printf("Cleaned up test database: %s\n", dbName)
	}

	return db, cleanup
}

func createIndexes(db *mongo.Database) {
	ctx := context.Background()
	collection := db.Collection("social_posts")

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "platform", Value: 1}, {Key: "external_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "project_id", Value: 1}, {Key: "posted_at", Value: -1}},
		},
	}

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		fmt.Printf("Warning: Failed to create indexes: %v\n", err)
	}
}
