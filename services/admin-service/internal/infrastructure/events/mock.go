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
		"507f1f77bcf86cd799439011",  // Valid 24-char hex ObjectID
		"507f191e810c19729de860ea",
		"507f191e810c19729de860eb",
		"507f191e810c19729de860ec",
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

	keyWordsSize := 1 + rand.Intn(3)
	randomKeyWords := getRandomKeywords(keywords, keyWordsSize)

	return &domain.BrandMonitorEvent{
		ProjectID:   projects[rand.Intn(len(projects))],
		KeyWords:    randomKeyWords,
		RequestedBy: users[rand.Intn(len(users))],
		Timestamp:   time.Now(),
	}
}

func getRandomKeywords(keywords []string, count int) []string {
    if count <= 0 || len(keywords) == 0 {
        return []string{}
    }
    
    // Limit count to keywords length
    if count > len(keywords) {
        count = len(keywords)
    }
    
    randomKeywords := make([]string, 0, count) // Pre-allocate capacity
    
    for i := 0; i < count; i++ {
        randomKeywords = append(randomKeywords, keywords[rand.Intn(len(keywords))])
    }
    
    return randomKeywords
}