package main

import (
	"context"
	"log"
	"net/http"
	"time"

	apiHandler "github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/adapters/handler/http"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/adapters/repository/mongo"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/config"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/infrastructure/events"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/auth"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/project"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/user"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/pkg/mocks"
)

func main() {
	log.Printf("Starting app")
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded config: %+v", cfg)

	// 2. Database Connection
	log.Printf("MongoDB URI: %s", cfg.MongoDBDatabaseURI)
	dbClient, err := mongo.NewConnection(cfg.MongoDBDatabaseURI)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(context.Background())

	db := dbClient.Database(cfg.MongoDBDatabaseName)

	// 3. Dependency Injection "REPO"
	userRepo := mongo.NewUserRepository(db)
	projectRepo := mongo.NewProjectRepository(db)

	authService := auth.NewService(userRepo, cfg.JWTSecret)
	userService := user.NewService(userRepo)
	projectService := project.NewProjectService(projectRepo, userRepo)

	// Create the Handlers (The Waiter)
	authHandler := apiHandler.NewAuthHandler(authService)
	userHandler := apiHandler.NewUserHandler(userService)
	projectHandler := apiHandler.NewProjectHandler(projectService)

	// 4. Setup Router (The Traffic Controller)
	// Main.go no longer knows about "/auth/login". It just asks for a Router.
	r := apiHandler.NewRouter(authHandler, userHandler, projectHandler)

	//5. setup kafka producers
	kafkaProducer := events.NewProducer([]string{"kafka.paw-suite.svc.cluster.local:9092"}, "brand-moniter")
	if kafkaProducer == nil {
		log.Fatal("Kafka producer failed to start")
	}

	//mock event generate
	ticker := time.NewTicker(5 *time.Second)
	defer ticker.Stop()
	go func() {
		for {
			<- ticker.C
			event := mocks.GenerateMockBrandMoniterEvent()
			kafkaProducer.SendBrandSearch(context.Background(), *event)
		}
	}()
	

	// 5. Start Server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
		// ReadTimeout: Max time to read the request body.
		// Protects against "Slowloris" attacks (clients sending 1 byte every 30s)
		ReadTimeout: 5 * time.Second,

		// WriteTimeout: Max time to write the response.
		// If your DB takes 20s, this cuts the connection at 10s to free resources.
		WriteTimeout: 10 * time.Second,

		// IdleTimeout: Max time to keep a Keep-Alive connection open.
		IdleTimeout: 120 * time.Second,
	}

	log.Printf("🚀 Server starting on port %s", cfg.ServerPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
