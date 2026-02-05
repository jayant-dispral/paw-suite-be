// services/data-service/cmd/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jayant-dispral/brand-threat-be/services/data-service/infrastrucutre/events"
	apiHandler "github.com/jayant-dispral/brand-threat-be/services/data-service/internal/adapters/handler/http"
	"github.com/jayant-dispral/brand-threat-be/services/data-service/internal/service"
	"github.com/jayant-dispral/brand-threat-be/services/data-service/internal/service/workerpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	log.Println("🚀 Starting Data Service with Worker Pool...")

	// ============================================================
	// CONFIGURATION
	// ============================================================
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	mongoURL := getEnv("MONGODB_DATABASE_URI", "mongodb://admin:password@mongo:27017/brand_threat?authSource=admin")
	exchangeName := getEnv("RABBITMQ_EXCHANGE", "brand_events")
	queueName := getEnv("RABBITMQ_QUEUE", "data_service_queue")
	routingKeys := []string{"brand.#"}

	// Worker pool configuration
	apiRateLimit := 10 // requests per second to external API
	apiWorkers := 40   // concurrent API workers (adjust based on API latency)
	procWorkers := 5   // concurrent processing workers

	// ============================================================
	// CONNECT TO MONGODB
	// ============================================================
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("⚠️  Error disconnecting MongoDB: %v", err)
		}
	}()

	// Ping MongoDB
	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("❌ Failed to ping MongoDB: %v", err)
	}

	db := mongoClient.Database("brand_threat")
	log.Println("✅ Connected to MongoDB")

	// ============================================================
	// INITIALIZE WORKER POOL
	// ============================================================
	wp := workerpool.NewWorkerPool(apiRateLimit, apiWorkers, procWorkers, db)
	wp.Start()
	log.Printf("✅ Worker pool started: %d API workers, %d processing workers, rate limit: %d/sec",
		apiWorkers, procWorkers, apiRateLimit)

	defer func() {
		log.Println("🛑 Shutting down worker pool...")
		if err := wp.Shutdown(10 * time.Second); err != nil {
			log.Printf("⚠️  Worker pool shutdown error: %v", err)
		}
	}()

	// ============================================================
	// INITIALIZE BUSINESS LOGIC SERVICE
	// ============================================================
	brandScanService := service.NewBrandScanService(wp)

	// ============================================================
	// INITIALIZE EVENT CONSUMER
	// ============================================================
	eventConsumer, err := events.NewRabbitMQEventConsumer(events.Config{
		RabbitMQURL:  rabbitmqURL,
		ExchangeName: exchangeName,
		QueueName:    queueName,
		RoutingKeys:  routingKeys,
	}, brandScanService)
	if err != nil {
		log.Fatalf("❌ Failed to initialize event consumer: %v", err)
	}
	defer func() {
		log.Println("📥 Closing event consumer...")
		if err := eventConsumer.Close(); err != nil {
			log.Printf("⚠️  Error closing event consumer: %v", err)
		}
	}()

	// ===========================================================
	// HTTP SERVER
	// ===========================================================
	router := apiHandler.NewRouter()
	httpServer := http.Server{
		Addr:    ":8081",
		Handler: router,
	}
	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if !wp.IsHealthy() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Worker pool not ready\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready\n"))
	})

	go func() {
		log.Println("🌐 HTTP server starting on :8081")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️  HTTP server error: %v", err)
		}
	}()

	// ============================================================
	// START EVENT CONSUMER
	// ============================================================
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	go func() {
		log.Println("👂 Starting event consumer...")
		if err := eventConsumer.Start(consumerCtx); err != nil && err != context.Canceled {
			log.Printf("⚠️  Event consumer stopped: %v", err)
		}
	}()

	// ============================================================
	// GRACEFUL SHUTDOWN
	// ============================================================
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Println("✅ Data Service started successfully")
	log.Println("🔄 Listening for events... (Press Ctrl+C to shutdown)")

	// Block until signal received
	<-sigCh
	log.Println("\n🛑 Shutdown signal received, initiating graceful shutdown...")

	// Cancel consumer context
	consumerCancel()

	// Wait a bit for in-flight messages
	time.Sleep(2 * time.Second)

	log.Println("👋 Data Service stopped gracefully")
}

// getEnv retrieves environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
