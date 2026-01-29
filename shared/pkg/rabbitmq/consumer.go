package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// HandlerFunc processes a message. Return error to NACK, nil to ACK
type HandlerFunc func(ctx context.Context, body []byte) error

// Consumer consumes messages from a queue
type Consumer struct {
	client       *Client
	exchangeName string
	exchangeType string
	queueName    string
	routingKeys  []string // Binding keys (supports wildcards for topic exchange)
	handler      HandlerFunc

	mu      sync.RWMutex
	channel *amqp.Channel
	closed  bool
	wg      sync.WaitGroup // Tracks active message handlers
}

// ConsumerConfig holds consumer configuration
type ConsumerConfig struct {
	ExchangeName string
	ExchangeType string
	QueueName    string
	RoutingKeys  []string // For topic: ["brand.#"], for direct: ["brand.monitor.scan"]
	Handler      HandlerFunc
}

// NewConsumer creates a new consumer
func NewConsumer(client *Client, config ConsumerConfig) (*Consumer, error) {
	if config.Handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}
	if len(config.RoutingKeys) == 0 {
		return nil, fmt.Errorf("at least one routing key required")
	}

	c := &Consumer{
		client:       client,
		exchangeName: config.ExchangeName,
		exchangeType: config.ExchangeType,
		queueName:    config.QueueName,
		routingKeys:  config.RoutingKeys,
		handler:      config.Handler,
	}

	// Setup topology and start consuming
	if err := c.setup(); err != nil {
		return nil, fmt.Errorf("consumer setup failed: %w", err)
	}

	// Start monitoring for reconnections
	go c.maintainChannel()

	return c, nil
}

// setup creates channel, declares topology, and starts consuming
func (c *Consumer) setup() error {
	conn, err := c.client.GetConnection()
	if err != nil {
		return err
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Set QoS (prefetch count from config)
	if err := ch.Qos(
		c.client.config.PrefetchCount, // prefetch count
		0,     // prefetch size
		false, // global
	); err != nil {
		ch.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare exchange (idempotent)
	if err := ch.ExchangeDeclare(
		c.exchangeName,
		c.exchangeType,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		ch.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue (idempotent)
	queue, err := ch.QueueDeclare(
		c.queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange with routing keys
	for _, routingKey := range c.routingKeys {
		if err := ch.QueueBind(
			queue.Name,
			routingKey,
			c.exchangeName,
			false, // no-wait
			nil,
		); err != nil {
			ch.Close()
			return fmt.Errorf("failed to bind queue to exchange (key: %s): %w", routingKey, err)
		}
		log.Printf("[Consumer] Queue '%s' bound to exchange '%s' with routing key '%s'",
			queue.Name, c.exchangeName, routingKey)
	}

	// Start consuming
	msgs, err := ch.Consume(
		queue.Name,
		"",    // consumer tag (auto-generated)
		false, // auto-ack (NEVER true in production)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.mu.Lock()
	c.channel = ch
	c.mu.Unlock()

	// Process messages in background
	c.wg.Add(1)
	go c.processMessages(msgs)

	log.Printf("[Consumer] Started consuming from queue '%s'", queue.Name)
	return nil
}

// maintainChannel monitors for reconnections and recreates channel
func (c *Consumer) maintainChannel() {
	for {
		select {
		case <-c.client.reconnectedCh:
			c.mu.RLock()
			closed := c.closed
			c.mu.RUnlock()

			if closed {
				return
			}

			log.Printf("[Consumer] Detected reconnection, recreating channel...")
			if err := c.setup(); err != nil {
				log.Printf("[Consumer] Failed to recreate channel: %v", err)
				// Will retry on next reconnection
			}
		case <-c.client.ctx.Done():
			return
		}
	}
}

// processMessages handles incoming messages
func (c *Consumer) processMessages(msgs <-chan amqp.Delivery) {
	defer c.wg.Done()

	for msg := range msgs {
		c.handleMessage(msg)
	}

	log.Printf("[Consumer] Message channel closed")
}

// handleMessage processes a single message with manual ack/nack
func (c *Consumer) handleMessage(msg amqp.Delivery) {
	ctx := context.Background()

	log.Printf("[Consumer] Received message (routing key: %s, delivery tag: %d)",
		msg.RoutingKey, msg.DeliveryTag)

	// Call user handler
	if err := c.handler(ctx, msg.Body); err != nil {
		log.Printf("[Consumer] Handler failed: %v (NACK with requeue)", err)

		// NACK and requeue (message will be redelivered)
		if nackErr := msg.Nack(false, true); nackErr != nil {
			log.Printf("[Consumer] Failed to NACK message: %v", nackErr)
		}
		return
	}

	// ACK (remove message from queue)
	if err := msg.Ack(false); err != nil {
		log.Printf("[Consumer] Failed to ACK message: %v", err)
	} else {
		log.Printf("[Consumer] Message processed successfully (delivery tag: %d)", msg.DeliveryTag)
	}
}

// Close gracefully closes the consumer
func (c *Consumer) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true

	ch := c.channel
	c.mu.Unlock()

	// Close channel (stops message delivery)
	if ch != nil {
		if err := ch.Close(); err != nil {
			log.Printf("[Consumer] Error closing channel: %v", err)
		}
	}

	// Wait for in-flight messages to complete
	log.Printf("[Consumer] Waiting for in-flight messages to complete...")
	c.wg.Wait()

	log.Printf("[Consumer] Closed")
	return nil
}

// UnmarshalMessage is a helper to unmarshal JSON messages
func UnmarshalMessage(body []byte, v interface{}) error {
	return json.Unmarshal(body, v)
}