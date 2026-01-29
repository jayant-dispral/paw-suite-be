package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

// EventPublisher publishes domain events to a message broker
// This is a port (interface) - the concrete implementation is in the adapters layer
type EventPublisher interface {
	// PublishBrandMonitorEvent publishes a brand monitoring scan event
	PublishBrandMonitorEvent(ctx context.Context, event *domain.BrandMonitorEvent) error

	// Close gracefully shuts down the publisher
	Close() error
}

// EventConsumer consumes domain events from a message broker
// This is a port (interface) - the concrete implementation is in the adapters layer
type EventConsumer interface {
	// Start begins consuming events (blocking call)
	Start(ctx context.Context) error

	// Close gracefully shuts down the consumer
	Close() error
}

// BrandMonitorEventHandler handles incoming brand monitor events
// Business logic services implement this interface
type BrandMonitorEventHandler interface {
	// HandleBrandMonitorEvent processes a brand monitoring event
	HandleBrandMonitorEvent(ctx context.Context, event *domain.BrandMonitorEvent) error
}