package service

import (
	"context"
	"fmt"
	"log"

	"github.com/jayant-dispral/brand-threat-be/services/data-service/internal/service/workerpool"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BrandScanService handles brand monitoring events
type BrandScanService struct {
	workerPool *workerpool.WorkerPool
}

// NewBrandScanService creates a new brand scan service with worker pool
func NewBrandScanService(wp *workerpool.WorkerPool) *BrandScanService {
	return &BrandScanService{
		workerPool: wp,
	}
}

// HandleBrandMonitorEvent processes incoming brand monitor events

func (s *BrandScanService) HandleBrandMonitorEvent(ctx context.Context, event *domain.BrandMonitorEvent) error {
	log.Printf("[BrandScanService] 📨 Received event: Project=%s, Keywords=%v, RequestedBy=%s",
		event.ProjectID, event.KeyWords, event.RequestedBy)

	// validate event
	if err := s.validateEvent(event); err != nil {
		log.Printf("[BrandScanService] ❌ Invalid event: %v", err)
		return nil // ACK invalid events (don't requeue)
	}

	//create one task per keyword
	tasksSubmitted := 0
	for _, keyword := range event.KeyWords {
		task, err := s.createTask(event, keyword)
		if err != nil {
			log.Printf("[BrandScanService] ⚠️  Failed to create task for keyword '%s': %v", keyword, err)
			continue
		}

		//submit to worker pool
		if err := s.workerPool.SubmitTask(task); err != nil {
			log.Printf("[BrandScanService] ⚠️  Failed to submit task for keyword '%s': %v", keyword, err)
			// Don't fail entire event - continue with other keywords
			continue
		}
		tasksSubmitted++

		if tasksSubmitted == 0 {
			return fmt.Errorf("failed to submit any tasks")
		}

		log.Printf("[BrandScanService] ✓ Submitted task for keyword '%s' (task ID: %s)", keyword, task.ID)
	}
	return  nil
}

func (s *BrandScanService) createTask(event *domain.BrandMonitorEvent, keyword string) (workerpool.Task, error) {
	projectID, err := primitive.ObjectIDFromHex(event.ProjectID)
	if err != nil {
		return workerpool.Task{}, fmt.Errorf("invalid project ID: %w", err)
	}

	task := workerpool.Task{
		ID:          fmt.Sprintf("%s_%s_%d", event.ProjectID, keyword, event.Timestamp.Unix()),
		ProjectID:   projectID,
		Keywords:    []string{keyword}, // One keyword per task
		RequestedBy: event.RequestedBy,
		Timestamp:   event.Timestamp,
	}

	return task, nil
}

// validateEvent validates the incoming event
func (s *BrandScanService) validateEvent(event *domain.BrandMonitorEvent) error {
	if event.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if len(event.KeyWords) == 0 {
		return fmt.Errorf("at least one keyword is required")
	}
	if event.Timestamp.IsZero() {
		return fmt.Errorf("Timestamp is required")
	}
	return nil
}
