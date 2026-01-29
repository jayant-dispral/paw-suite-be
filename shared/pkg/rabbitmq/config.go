package rabbitmq

import (
	"fmt"
	"time"
)

// Config holds RabbitMQ connection configuration
type Config struct {
	// Connection
	URL  string // amqp://user:pass@rabbitmq:5672/
	Name string // Connection name for debugging (shown in RabbitMQ management UI)

	// Reconnection strategy
	MaxReconnectAttempts int           // 0 = infinite retries
	InitialRetryDelay    time.Duration // Start at 1s
	MaxRetryDelay        time.Duration // Cap at 30s
	RetryMultiplier      float64       // Exponential backoff multiplier (2.0 = double each time)

	// Consumer settings
	PrefetchCount int  // How many unacked messages per consumer (1-10 for safety, 50+ for throughput)
	AutoAck       bool // NEVER true in production
}

// DefaultConfig returns production-ready defaults
func DefaultConfig(url string) *Config {
	return &Config{
		URL:                  url,
		Name:                 "paw-suite-service", // Override this per service
		MaxReconnectAttempts: 0,                   // Infinite retries
		InitialRetryDelay:    1 * time.Second,
		MaxRetryDelay:        30 * time.Second,
		RetryMultiplier:      2.0, // 1s, 2s, 4s, 8s, 16s, 30s, 30s...
		PrefetchCount:        5,   // Balance: not too aggressive, not too slow
		AutoAck:              false,
	}
}

// Validate checks configuration
func (c *Config) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("rabbitmq URL cannot be empty")
	}
	if c.PrefetchCount < 1 {
		return fmt.Errorf("prefetch count must be >= 1")
	}
	if c.AutoAck {
		return fmt.Errorf("auto-ack is disabled for production safety")
	}
	return nil
}