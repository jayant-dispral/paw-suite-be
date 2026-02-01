// FILE PATH: services/admin-service/internal/infrastructure/events/mock_generator.go
// This generates mock brand monitoring events every 5 seconds for testing the pipeline.
// NOTE: This is ONLY for testing. Disable in production by setting MOCK_EVENTS_ENABLED=false

package events

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// MockEventGenerator generates mock brand monitor events for testing
type MockEventGenerator struct {
	publisher ports.EventPublisher
	interval  time.Duration
	stopCh    chan struct{}
}

// NewMockEventGenerator creates a new mock event generator
func NewMockEventGenerator(publisher ports.EventPublisher, interval time.Duration) *MockEventGenerator {
	return &MockEventGenerator{
		publisher: publisher,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Start begins generating mock events (blocking call)
func (g *MockEventGenerator) Start(ctx context.Context) error {
	log.Printf("[MockEventGenerator] Starting (interval: %v)", g.interval)

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	// Generate first event immediately
	g.generateAndPublish(ctx)

	for {
		select {
		case <-ticker.C:
			g.generateAndPublish(ctx)

		case <-g.stopCh:
			log.Printf("[MockEventGenerator] Stopped")
			return nil

		case <-ctx.Done():
			log.Printf("[MockEventGenerator] Context cancelled")
			return ctx.Err()
		}
	}
}

// Stop gracefully stops the generator
func (g *MockEventGenerator) Stop() {
	close(g.stopCh)
}

// generateAndPublish creates and publishes a mock event
func (g *MockEventGenerator) generateAndPublish(ctx context.Context) {
	event := g.createMockEvent()

	if err := g.publisher.PublishBrandMonitorEvent(ctx, event); err != nil {
		log.Printf("[MockEventGenerator] ERROR: Failed to publish event: %v", err)
		return
	}

	log.Printf("[MockEventGenerator] ✓ Published mock event: Project=%s, Keyword=%s",
		event.ProjectID, event.KeyWords)
}

// createMockEvent generates a realistic mock BrandMonitorEvent
func (g *MockEventGenerator) createMockEvent() *domain.BrandMonitorEvent {
	// Mock data pools
	projects := []string{
		"proj-ecommerce-001",
		"proj-saas-platform-002",
		"proj-fintech-app-003",
		"proj-healthcare-portal-004",
	}

	keywords := []string{
		"tesla",
		"apple-iphone",
		"nike-shoes",
		"microsoft-azure",
		"amazon-prime",
		"google-pixel",
		"netflix-streaming",
		"spotify-premium",
	}

	users := []string{
		"user-alice-789",
		"user-bob-456",
		"user-charlie-123",
		"user-diana-321",
	}

	return &domain.BrandMonitorEvent{
		ProjectID:   projects[rand.Intn(len(projects))],
		KeyWords:     keywords[0:rand.Intn(len(keywords))],
		RequestedBy: users[rand.Intn(len(users))],
		TimeStamp:   time.Now(),
	}
}