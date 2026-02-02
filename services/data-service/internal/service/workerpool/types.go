package workerpool

import (
	"fmt"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Task represents work to be done by API workers
type Task struct {
	ID          string // Format: {projectID}_{timestamp}
	ProjectID   primitive.ObjectID
	Keywords    []string
	RequestedBy string
	Timestamp   time.Time
}

// NewTask creates a task from BrandMonitorEvent
func NewTask(event domain.BrandMonitorEvent) (Task, error) {
	projectID, err := primitive.ObjectIDFromHex(event.ProjectID)
	if err != nil {
		return Task{}, fmt.Errorf("invalid project ID: %w", err)
	}

	taskID := fmt.Sprintf("%s_%d", event.ProjectID, event.TimeStamp.Unix())

	return Task{
		ID:          taskID,
		ProjectID:   projectID,
		Keywords:    event.KeyWords,
		RequestedBy: event.RequestedBy,
		Timestamp:   event.TimeStamp,
	}, nil
}

// Result represents cleaned data from the scraping API
type Result struct {
	TaskID      string
	SocialPosts []domain.SocialPost // ~20 posts per result
	FetchedAt   time.Time
}
