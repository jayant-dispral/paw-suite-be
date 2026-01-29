// This is the RabbitMQ adapter for consuming events (Data Service).
 

package events

import (
	"context"
	"fmt"
	"log"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/rabbitmq"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// RabbitMQEventConsumer is the RabbitMQ implementation of EventConsumer
type RabbitMQEventConsumer struct {
	client   *rabbitmq.Client
	consumer *rabbitmq.Consumer
	handler  ports.BrandMonitorEventHandler
}

// Config holds the configuration for the event consumer
type Config struct {
	RabbitMQURL  string
	ExchangeName string
	QueueName    string
	RoutingKeys  []string
}

// NewRabbitMQEventConsumer creates a new RabbitMQ event consumer
func NewRabbitMQEventConsumer(cfg Config, handler ports.BrandMonitorEventHandler) (ports.EventConsumer, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}

	// Create RabbitMQ client
	clientConfig := rabbitmq.DefaultConfig(cfg.RabbitMQURL)
	clientConfig.Name = "data-service-consumer" // Shows in RabbitMQ management UI

	client, err := rabbitmq.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create rabbitmq client: %w", err)
	}

	ec := &RabbitMQEventConsumer{
		client:  client,
		handler: handler,
	}

	// Create consumer with handler wrapper
	consumer, err := rabbitmq.NewConsumer(client, rabbitmq.ConsumerConfig{
		ExchangeName: cfg.ExchangeName,
		ExchangeType: "topic",
		QueueName:    cfg.QueueName,
		RoutingKeys:  cfg.RoutingKeys,
		Handler:      ec.handleMessage,
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	ec.consumer = consumer

	log.Printf("[EventConsumer] Initialized (queue: %s, routing keys: %v)", cfg.QueueName, cfg.RoutingKeys)

	return ec, nil
}

// handleMessage is the internal handler that unmarshals messages and delegates to business logic
func (c *RabbitMQEventConsumer) handleMessage(ctx context.Context, body []byte) error {
	// Unmarshal the event
	var event domain.BrandMonitorEvent
	if err := rabbitmq.UnmarshalMessage(body, &event); err != nil {
		// Don't requeue malformed messages - they'll fail forever
		log.Printf("[EventConsumer] ERROR: Failed to unmarshal message (discarding): %v", err)
		return nil // ACK to remove bad message
	}

	log.Printf("[EventConsumer] 📨 Received BrandMonitorEvent: Project=%s, Keyword=%s, RequestedBy=%s",
		event.ProjectID, event.KeyWord, event.RequestedBy)

	// Delegate to business logic handler
	if err := c.handler.HandleBrandMonitorEvent(ctx, &event); err != nil {
		// Business logic failed - NACK and requeue for retry
		log.Printf("[EventConsumer] ❌ Handler failed (will requeue): %v", err)
		return err // NACK
	}

	log.Printf("[EventConsumer] ✓ Successfully processed event: Project=%s, Keyword=%s",
		event.ProjectID, event.KeyWord)

	return nil // ACK
}

// Start is a no-op - the consumer is already running in background
func (c *RabbitMQEventConsumer) Start(ctx context.Context) error {
	// The consumer automatically starts consuming when created
	// This method exists to satisfy the EventConsumer interface
	log.Printf("[EventConsumer] Consumer is running (waiting for messages...)")

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// Close gracefully shuts down the consumer
func (c *RabbitMQEventConsumer) Close() error {
	log.Printf("[EventConsumer] Shutting down...")

	if err := c.consumer.Close(); err != nil {
		return fmt.Errorf("failed to close consumer: %w", err)
	}

	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close rabbitmq client: %w", err)
	}

	return nil
}

// Compile-time check that RabbitMQEventConsumer implements EventConsumer
var _ ports.EventConsumer = (*RabbitMQEventConsumer)(nil)