package main

import (
	"context"
	"log"
	"net/http"
	"time"

	apiHandler "github.com/jayant-dispral/brand-threat-be/internal/adapters/handler/http"
	"github.com/jayant-dispral/brand-threat-be/internal/adapters/repo/mongo"
	"github.com/jayant-dispral/brand-threat-be/internal/config"
	"github.com/jayant-dispral/brand-threat-be/internal/service/auth"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Database Connection
	dbClient, err := mongo.NewConnection(cfg.MongoDBDatabaseURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(context.Background())

	db := dbClient.Database("sentinel_prod")

	// 3. Dependency Injection
	userRepo := mongo.NewUserRepository(db)
	authService := auth.NewService(userRepo, cfg.JWTSecret)

	// Create the Handler (The Waiter)
	authHandler := apiHandler.NewAuthHandler(authService)

	// 4. Setup Router (The Traffic Controller)
	// Main.go no longer knows about "/auth/login". It just asks for a Router.
	r := apiHandler.NewRouter(authHandler)

	// 5. Start Server
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("🚀 Server starting on port %s", cfg.ServerPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
