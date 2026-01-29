// FILE PATH: services/data-service/cmd/main.go
// This is the main entry point for the Data Service with RabbitMQ event consumer integration.
//
// INTEGRATION INSTRUCTIONS:
// 1. If you already have a main.go, MERGE this code into your existing file
// 2. Add the business logic service initialization (lines 33-35)
// 3. Add the event consumer initialization (lines 37-50)
// 4. Add the consumer start in background (lines 52-58)
// 5. Update your graceful shutdown to call cancel() and eventConsumer.Close()
//
// Place this file in: <project-root>/services/data-service/cmd/main.go

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
	"github.com/jayant-dispral/brand-threat-be/services/data-service/internal/service"
	apiHandler "github.com/jayant-dispral/brand-threat-be/services/data-service/internal/adapters/handler/http"
)

func main() {
	log.Println("🚀 Starting Data Service...")

	// ============================================================
	// CONFIGURATION
	// ============================================================
	rabbitmqURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	exchangeName := getEnv("RABBITMQ_EXCHANGE", "brand_events")
	queueName := getEnv("RABBITMQ_QUEUE", "data_service_queue")

	// Routing keys determine which events this service consumes
	// "brand.#" = all brand events (brand.monitor.scan, brand.alert.created, etc.)
	routingKeys := []string{"brand.#"}

	// ============================================================
	// INITIALIZE BUSINESS LOGIC SERVICE
	// ============================================================
	brandScanService := service.NewBrandScanService()
	// TODO: Add your dependencies to brandScanService
	// brandScanService := service.NewBrandScanService(mongoRepo, scannerAPI)

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

	// ============================================================
	// START EVENT CONSUMER
	// ============================================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Println("👂 Starting event consumer...")
		if err := eventConsumer.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("⚠️  Event consumer stopped: %v", err)
		}
	}()

	// ============================================================
	// START HTTP SERVER (Optional - for health checks, etc.)
	// ============================================================
	router := apiHandler.NewRouter()
	server := &http.Server{
	    Addr:    ":8081",
	    Handler: router,
	}
	
	go func() {
	    log.Println("🌐 HTTP server starting on :8081")
	    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	        log.Fatalf("❌ HTTP server failed: %v", err)
	    }
	}()

	// ============================================================
	// GRACEFUL SHUTDOWN
	// ============================================================
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Println("✅ Data Service started successfully")
	log.Println("🔄 Waiting for events... (Press Ctrl+C to shutdown)")

	// Block until signal received
	<-sigCh
	log.Println("\n🛑 Shutdown signal received, initiating graceful shutdown...")

	// Cancel context to stop consumer
	cancel()

	// Shutdown timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
	    log.Printf("⚠️  HTTP server shutdown error: %v", err)
	}

	// Give consumer time to finish processing in-flight messages
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
