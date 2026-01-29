package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes messages to a RabbitMQ exchange
type Publisher struct {
	client       *Client
	exchangeName string
	exchangeType string // "topic", "direct", "fanout", "headers"

	mu      sync.RWMutex
	channel *amqp.Channel
	closed  bool
}

// NewPublisher creates a new publisher
func NewPublisher(client *Client, exchangeName, exchangeType string) (*Publisher, error) {
	pub := &Publisher{
		client:       client,
		exchangeName: exchangeName,
		exchangeType: exchangeType,
	}

	// Create initial channel and declare exchange
	if err := pub.setup(); err != nil {
		return nil, fmt.Errorf("publisher setup failed: %w", err)
	}

	// Start monitoring for reconnections
	go pub.maintainChannel()

	return pub, nil
}

// setup creates channel and declares exchange (idempotent)
func (p *Publisher) setup() error {
	conn, err := p.client.GetConnection()
	if err != nil {
		return err
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Declare exchange (idempotent - safe to call multiple times)
	if err := ch.ExchangeDeclare(
		p.exchangeName, // name
		p.exchangeType, // type
		true,           // durable (survives broker restart)
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	); err != nil {
		ch.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	p.mu.Lock()
	p.channel = ch
	p.mu.Unlock()

	log.Printf("[Publisher] Exchange '%s' (type: %s) declared", p.exchangeName, p.exchangeType)
	return nil
}

// maintainChannel monitors for reconnections and recreates channel
func (p *Publisher) maintainChannel() {
	for {
		select {
		case <-p.client.reconnectedCh:
			log.Printf("[Publisher] Detected reconnection, recreating channel...")
			if err := p.setup(); err != nil {
				log.Printf("[Publisher] Failed to recreate channel: %v", err)
				// Will retry on next reconnection
			}
		case <-p.client.ctx.Done():
			return
		}
	}
}

// Publish sends a message to the exchange with a routing key
func (p *Publisher) Publish(ctx context.Context, routingKey string, message interface{}) error {
	// Serialize message to JSON
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return p.PublishRaw(ctx, routingKey, body)
}

// PublishRaw sends raw bytes to the exchange
func (p *Publisher) PublishRaw(ctx context.Context, routingKey string, body []byte) error {
	p.mu.RLock()
	ch := p.channel
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		return fmt.Errorf("publisher is closed")
	}

	if ch == nil {
		return fmt.Errorf("channel not ready")
	}

	// Publish with context
	return ch.PublishWithContext(
		ctx,
		p.exchangeName, // exchange
		routingKey,     // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // Survives broker restart if queue is durable
			Timestamp:    time.Now(),
		},
	)
}

// Close gracefully closes the publisher
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	log.Printf("[Publisher] Closed")
	return nil
}