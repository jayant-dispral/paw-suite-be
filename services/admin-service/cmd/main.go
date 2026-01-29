package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apiHandler "github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/adapters/handler/http"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/adapters/repository/mongo"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/config"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/infrastructure/events"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/auth"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/project"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/user"
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


	//====================================================
	// 		RabbitMQ 
	//====================================================
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	exchangeName := getEnv("RABBITMQ_EXCHANGE", "brand_events")
	mockEventsEnabled := getEnv("MOCK_EVENTS_ENABLED", "true") == "true"
	mockEventInterval := getEnvDuration("MOCK_EVENT_INTERVAL", 5*time.Second)

	//Initilise event publisher
	eventPublisher, err := events.NewRabbitMQEventPublisher(events.Config{
		RabbitMQURL:  rabbitmqURL,
		ExchangeName: exchangeName,
	})
	if err != nil {
		log.Fatalf("❌ Failed to initialize event publisher: %v", err)
	}
	defer func() {
		log.Println("📤 Closing event publisher...")
		if err := eventPublisher.Close(); err != nil {
			log.Printf("⚠️  Error closing event publisher: %v", err)
		}
	}()

	//start mock event generator
	var mockGenerator *events.MockEventGenerator
	if mockEventsEnabled {
		log.Printf("🧪 Mock event generator enabled (interval: %v)", mockEventInterval)
		mockGenerator = events.NewMockEventGenerator(eventPublisher, mockEventInterval)

		// Start generator in background
		go func() {
			if err := mockGenerator.Start(context.Background()); err != nil {
				log.Printf("⚠️  Mock event generator stopped: %v", err)
			}
		}()

		defer mockGenerator.Stop()
	}

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


	//mock event generate
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	

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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Println("✅ Admin Service started successfully")
	log.Println("🔄 Press Ctrl+C to shutdown...")

	// Block until signal received
	<-sigCh
	log.Println("\n🛑 Shutdown signal received, initiating graceful shutdown...")

	// Shutdown timeout context
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop mock generator (if running)
	if mockGenerator != nil {
		log.Println("🧪 Stopping mock event generator...")
		mockGenerator.Stop()
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
	    log.Printf("⚠️  HTTP server shutdown error: %v", err)
	}

	log.Println("👋 Admin Service stopped gracefully")
}


func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getEnvDuration retrieves duration from environment variable with fallback
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}