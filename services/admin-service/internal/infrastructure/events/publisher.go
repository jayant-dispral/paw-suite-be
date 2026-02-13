// FILE PATH: services/admin-service/internal/infrastructure/events/publisher.go
// This is the RabbitMQ adapter for publishing events (Admin Service).

package events

import (
	"context"
	"fmt"
	"log"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/rabbitmq"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// RabbitMQEventPublisher is the RabbitMQ implementation of EventPublisher
type RabbitMQEventPublisher struct {
	client    *rabbitmq.Client
	publisher *rabbitmq.Publisher
}

// Config holds the configuration for the event publisher
type Config struct {
	RabbitMQURL  string
	ExchangeName string
}

// NewRabbitMQEventPublisher creates a new RabbitMQ event publisher
func NewRabbitMQEventPublisher(cfg Config) (ports.EventPublisher, error) {
	// Create RabbitMQ client with production-ready defaults
	clientConfig := rabbitmq.DefaultConfig(cfg.RabbitMQURL)
	clientConfig.Name = "admin-service-publisher" // Shows in RabbitMQ management UI

	client, err := rabbitmq.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rabbitmq client: %w", err)
	}

	// Create publisher with Topic Exchange
	publisher, err := rabbitmq.NewPublisher(client, cfg.ExchangeName, "topic")
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	log.Printf("[EventPublisher] Initialized (exchange: %s)", cfg.ExchangeName)

	return &RabbitMQEventPublisher{
		client:    client,
		publisher: publisher,
	}, nil
}

// PublishBrandMonitorEvent publishes a brand monitoring event
func (p *RabbitMQEventPublisher) PublishBrandMonitorEvent(ctx context.Context, event *domain.BrandMonitorEvent) error {
	// Routing key follows pattern: brand.monitor.scan
	// This allows consumers to subscribe using wildcards:
	// - "brand.#" = all brand events
	// - "brand.monitor.*" = only brand monitoring events
	routingKey := "brand.monitor.scan"

	if err := p.publisher.Publish(ctx, routingKey, event); err != nil {
		return fmt.Errorf("failed to publish brand monitor event: %w", err)
	}

	log.Printf("[EventPublisher] Published BrandMonitorEvent (project: %s, keyword: %s)",
		event.ProjectID, event.KeyWords)

	return nil
}

// Close gracefully shuts down the publisher
func (p *RabbitMQEventPublisher) Close() error {
	log.Printf("[EventPublisher] Shutting down...")

	if err := p.publisher.Close(); err != nil {
		return fmt.Errorf("failed to close publisher: %w", err)
	}

	if err := p.client.Close(); err != nil {
		return fmt.Errorf("failed to close rabbitmq client: %w", err)
	}

	return nil
}

// Compile-time check that RabbitMQEventPublisher implements EventPublisher
var _ ports.EventPublisher = (*RabbitMQEventPublisher)(nil)