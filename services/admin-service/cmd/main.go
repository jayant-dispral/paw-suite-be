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
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/social_post"
	"github.com/jayant-dispral/brand-threat-be/services/admin-service/internal/service/threat"
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
	socialPostRepo := mongo.NewSocialPostRepository(db)
	threatRepo := mongo.NewThreatRepository(db)
	threatScanStateRepo := mongo.NewThreatScanStateRepository(db)

	authService := auth.NewService(userRepo, cfg.JWTSecret)
	userService := user.NewService(userRepo)
	projectService := project.NewProjectService(projectRepo, userRepo)
	socialPostService := social_post.NewSocialPostService(socialPostRepo, projectRepo)
	threatService := threat.NewThreatService(threatRepo, threatScanStateRepo, projectRepo)

	// ------------------------------------------------
	// 7. HTTP Handlers + Router
	// ------------------------------------------------
	authHandler := apiHandler.NewAuthHandler(authService)
	userHandler := apiHandler.NewUserHandler(userService)
	projectHandler := apiHandler.NewProjectHandler(projectService)
	socialPostHandler := apiHandler.NewSocialPostHandler(socialPostService)
	threatHandler := apiHandler.NewThreatHandler(threatService)

	router := apiHandler.NewRouter(authHandler, userHandler, projectHandler, socialPostHandler, threatHandler)

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
		Addr:        ":" + cfg.ServerPort,
		Handler:     router,
		ReadTimeout: 15 * time.Second,

		// FIX: WriteTimeout was 10 seconds. The threat scan pipeline runs for
		// up to scanTimeout (10 minutes) and GET /threats polls scan progress
		// during that window. A 10s WriteTimeout kills the connection before
		// the first scan result arrives, returning a broken response to the
		// client. Set to 0 to disable the per-connection write deadline and
		// rely instead on the per-request middleware.Timeout(60s) for normal
		// endpoints. Long-running scan poll requests manage their own deadline
		// via the scan context inside analyzeProjectThreats.
		//
		// If a hard server-level cap is required, set this to slightly above
		// scanTimeout, e.g. 11 * time.Minute.
		WriteTimeout: 0,

		IdleTimeout: 120 * time.Second,
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

// FIX: getEnvDuration was declared but never called in the original main.go.
// Removed to eliminate dead code. If duration-based env vars are needed in
// future (e.g. configurable scan timeout), add them at that point.