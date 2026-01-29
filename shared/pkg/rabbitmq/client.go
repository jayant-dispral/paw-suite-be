package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client manages RabbitMQ connection with automatic reconnection
type Client struct {
	config *Config
	conn   *amqp.Connection
	
	mu            sync.RWMutex
	connected     bool
	reconnecting  bool
	shutdownCh    chan struct{}
	reconnectedCh chan struct{} // Notifies when reconnection succeeds
	
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new RabbitMQ client with retry logic for initial connection
func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &Client{
		config:        config,
		shutdownCh:    make(chan struct{}),
		reconnectedCh: make(chan struct{}, 1), // Buffered to prevent blocking
		ctx:           ctx,
		cancel:        cancel,
	}
	
	// Initial connection with retry logic
	if err := client.connectWithRetry(); err != nil {
		cancel()
		return nil, fmt.Errorf("initial connection failed: %w", err)
	}
	
	// Start monitoring connection health
	go client.maintainConnection()
	
	return client, nil
}

// connectWithRetry attempts to connect with exponential backoff
func (c *Client) connectWithRetry() error {
	retryDelay := c.config.InitialRetryDelay
	attempts := 0
	
	for {
		attempts++
		
		log.Printf("[RabbitMQ] Initial connection attempt %d...", attempts)
		if err := c.connect(); err != nil {
			log.Printf("[RabbitMQ] Connection attempt %d failed: %v", attempts, err)
			
			// Check if we've exceeded max attempts (0 = infinite)
			if c.config.MaxReconnectAttempts > 0 && attempts >= c.config.MaxReconnectAttempts {
				return fmt.Errorf("max connection attempts (%d) reached", c.config.MaxReconnectAttempts)
			}
			
			log.Printf("[RabbitMQ] Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
			
			// Exponential backoff with cap
			retryDelay = time.Duration(float64(retryDelay) * c.config.RetryMultiplier)
			if retryDelay > c.config.MaxRetryDelay {
				retryDelay = c.config.MaxRetryDelay
			}
			
			continue
		}
		
		log.Printf("[RabbitMQ] Connected successfully after %d attempts", attempts)
		return nil
	}
}

// connect establishes the initial connection
func (c *Client) connect() error {
	log.Printf("[RabbitMQ] Connecting to %s (name: %s)...", maskPassword(c.config.URL), c.config.Name)
	
	conn, err := amqp.DialConfig(c.config.URL, amqp.Config{
		Properties: amqp.Table{
			"connection_name": c.config.Name,
		},
	})
	if err != nil {
		return err
	}
	
	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()
	
	log.Printf("[RabbitMQ] Connected successfully")
	return nil
}

// maintainConnection monitors connection health and reconnects on failure
func (c *Client) maintainConnection() {
	for {
		select {
		case <-c.shutdownCh:
			log.Printf("[RabbitMQ] Shutdown signal received, stopping connection monitor")
			return
		case <-c.ctx.Done():
			return
		default:
		}
		
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		
		if conn == nil {
			c.reconnect()
			continue
		}
		
		// Wait for connection to close (blocks until error)
		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))
		
		select {
		case err := <-notifyClose:
			if err != nil {
				log.Printf("[RabbitMQ] Connection lost: %v", err)
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()
				c.reconnect()
			}
		case <-c.shutdownCh:
			return
		case <-c.ctx.Done():
			return
		}
	}
}

// reconnect attempts to reconnect with exponential backoff
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return // Already reconnecting
	}
	c.reconnecting = true
	c.mu.Unlock()
	
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()
	
	retryDelay := c.config.InitialRetryDelay
	attempts := 0
	
	for {
		select {
		case <-c.shutdownCh:
			return
		case <-c.ctx.Done():
			return
		default:
		}
		
		attempts++
		if c.config.MaxReconnectAttempts > 0 && attempts > c.config.MaxReconnectAttempts {
			log.Printf("[RabbitMQ] Max reconnection attempts (%d) reached. Giving up.", c.config.MaxReconnectAttempts)
			return
		}
		
		log.Printf("[RabbitMQ] Reconnection attempt %d (waiting %v)...", attempts, retryDelay)
		time.Sleep(retryDelay)
		
		if err := c.connect(); err != nil {
			log.Printf("[RabbitMQ] Reconnection failed: %v", err)
			
			// Exponential backoff with jitter
			retryDelay = time.Duration(float64(retryDelay) * c.config.RetryMultiplier)
			if retryDelay > c.config.MaxRetryDelay {
				retryDelay = c.config.MaxRetryDelay
			}
			
			// Add jitter (±20% randomness)
			jitter := time.Duration(rand.Float64()*0.4-0.2) * retryDelay
			retryDelay += jitter
			
			continue
		}
		
		// Success!
		log.Printf("[RabbitMQ] Reconnected successfully after %d attempts", attempts)
		
		// Notify channels/publishers to recreate themselves
		select {
		case c.reconnectedCh <- struct{}{}:
		default: // Don't block if no one is listening
		}
		
		return
	}
}

// IsConnected returns current connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetConnection returns the underlying connection (use carefully)
func (c *Client) GetConnection() (*amqp.Connection, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.connected || c.conn == nil {
		return nil, fmt.Errorf("not connected to RabbitMQ")
	}
	
	return c.conn, nil
}

// Close gracefully shuts down the client
func (c *Client) Close() error {
	log.Printf("[RabbitMQ] Initiating graceful shutdown...")
	
	// Signal shutdown
	close(c.shutdownCh)
	c.cancel()
	
	// Close connection
	c.mu.Lock()
	conn := c.conn
	c.connected = false
	c.mu.Unlock()
	
	if conn != nil {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}
	
	log.Printf("[RabbitMQ] Shutdown complete")
	return nil
}

// maskPassword hides password in logs
func maskPassword(url string) string {
	// Simple masking for amqp://user:pass@host:port/vhost
	// Production: Use proper URL parsing
	return "amqp://***:***@" + url[strings.LastIndex(url, "@")+1:]
}