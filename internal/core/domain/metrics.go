// metrics.go       -> DailyMetrics

package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Daily aggregated metrics for historical trend analysis
type DailyMetrics struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`
	Date      time.Time          `bson:"date" json:"date"` // Truncated to day (00:00:00)

	// Social Sentiment
	TotalPosts    int     `bson:"total_posts" json:"total_posts" validate:"min=0"`
	PositivePosts int     `bson:"positive_posts" json:"positive_posts" validate:"min=0"`
	NeutralPosts  int     `bson:"neutral_posts" json:"neutral_posts" validate:"min=0"`
	NegativePosts int     `bson:"negative_posts" json:"negative_posts" validate:"min=0"`
	AvgSentiment  float64 `bson:"avg_sentiment" json:"avg_sentiment" validate:"min=-1,max=1"`

	// Engagement
	TotalEngagement int `bson:"total_engagement" json:"total_engagement" validate:"min=0"` // Sum of likes, shares, comments
	ViralPosts      int `bson:"viral_posts" json:"viral_posts" validate:"min=0"`

	// Threats
	NewThreats      int `bson:"new_threats" json:"new_threats" validate:"min=0"`
	ResolvedThreats int `bson:"resolved_threats" json:"resolved_threats" validate:"min=0"`

	// Alerts
	AlertsTriggered int `bson:"alerts_triggered" json:"alerts_triggered" validate:"min=0"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

// Indexes for DailyMetrics collection:
// 1. {"project_id": 1, "date": -1} - unique compound index
// 2. {"date": 1} - for date-range queries
