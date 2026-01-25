package integration

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jayant-dispral/brand-threat-be/internal/adapters/handler/http"
	mongorepo "github.com/jayant-dispral/brand-threat-be/internal/adapters/repo/mongo"
	"github.com/jayant-dispral/brand-threat-be/internal/config"
	"github.com/jayant-dispral/brand-threat-be/internal/service/auth"
	"github.com/jayant-dispral/brand-threat-be/internal/service/user"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	testRouter *chi.Mux
	testDB     *mongo.Database
)

func TestMain(m *testing.M) {
	// --- SETUP ---
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	uri := cfg.MongoDBDatabaseURI
	if uri == "" {
		uri = "mongodb://localhost:27017" // Fallback for local testing
	}

	dbName := "sentinel_integration_test"
	client, err := mongorepo.NewConnection(uri)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	db := client.Database(dbName)

	// Clean the DB before running tests
	err = db.Drop(context.Background())
	if err != nil {
		log.Fatalf("Failed to drop database: %v", err)
	}

	// Initialize dependencies (Repo -> Service -> Handler)
	userRepo := mongorepo.NewUserRepository(db)
	userSvc := user.NewService(userRepo)
	authSvc := auth.NewService(userRepo, cfg.JWTSecret)

	userHandler := http.NewUserHandler(userSvc)
	authHandler := http.NewAuthHandler(authSvc)

	// Initialize router
	testRouter = http.NewRouter(authHandler, userHandler)

	// Store DB connection for test-specific cleanup
	testDB = db

	// Run tests
	exitCode := m.Run()

	// --- TEARDOWN ---
	// Optional: You could drop the DB after tests if needed
	// db.Drop(context.Background())

	os.Exit(exitCode)
}
