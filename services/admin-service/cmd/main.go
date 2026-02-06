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
	log.Println("🚀 Starting admin-service")

	// ------------------------------------------------
	// 1. Root context (controls EVERYTHING)
	// ------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ------------------------------------------------
	// 2. OS signal handling (Kubernetes / Ctrl+C safe)
	// ------------------------------------------------
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("🛑 Received signal: %v", sig)
		cancel()
	}()

	// ------------------------------------------------
	// 3. Load configuration
	// ------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}
	log.Printf("✅ Config loaded")

	// ------------------------------------------------
	// 4. MongoDB
	// ------------------------------------------------
	dbClient, err := mongo.NewConnection(cfg.MongoDBDatabaseURI)
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(context.Background())

	db := dbClient.Database(cfg.MongoDBDatabaseName)

	// ------------------------------------------------
	// 5. RabbitMQ publisher
	// ------------------------------------------------
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	exchangeName := getEnv("RABBITMQ_EXCHANGE", "brand_events")

	eventPublisher, err := events.NewRabbitMQEventPublisher(events.Config{
		RabbitMQURL:  rabbitmqURL,
		ExchangeName: exchangeName,
	})
	if err != nil {
		log.Fatalf("❌ Failed to initialize event publisher: %v", err)
	}
	defer func() {
		log.Println("📤 Closing event publisher")
		_ = eventPublisher.Close()
	}()

	// ------------------------------------------------
	// 6. Repositories + Services
	// ------------------------------------------------
	userRepo := mongo.NewUserRepository(db)
	projectRepo := mongo.NewProjectRepository(db)

	authService := auth.NewService(userRepo, cfg.JWTSecret)
	userService := user.NewService(userRepo)
	projectService := project.NewProjectService(projectRepo, userRepo)

	// ------------------------------------------------
	// 7. HTTP Handlers + Router
	// ------------------------------------------------
	authHandler := apiHandler.NewAuthHandler(authService)
	userHandler := apiHandler.NewUserHandler(userService)
	projectHandler := apiHandler.NewProjectHandler(projectService)

	router := apiHandler.NewRouter(authHandler, userHandler, projectHandler)

	// ------------------------------------------------
	// 8. Scheduler (background worker)
	// ------------------------------------------------
	scanner := project.NewBrandEventScanner(
		projectRepo,
		eventPublisher,
		10*time.Second, // scheduler tick
		20,             // batch size
	)

	go func() {
		if err := scanner.Start(ctx); err != nil {
			log.Printf("⚠️ Scheduler stopped: %v", err)
		}
	}()

	// ------------------------------------------------
	// 9. HTTP Server
	// ------------------------------------------------
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Shutdown HTTP server when context is cancelled
	go func() {
		<-ctx.Done()
		log.Println("🧹 Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("🌐 HTTP server listening on :%s", cfg.ServerPort)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ HTTP server failed: %v", err)
	}

	log.Println("👋 Admin service exited cleanly")
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
