// social_post.go   -> SocialPost, Sentiment
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Sentiment string

const (
	SentimentPositive Sentiment = "positive"
	SentimentNeutral  Sentiment = "neutral"
	SentimentNegative Sentiment = "negative"
)

type SocialPost struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`

	// External Data
	Platform   string `bson:"platform" json:"platform"`       // "twitter", "reddit", "linkedin"
	ExternalID string `bson:"external_id" json:"external_id"` // Tweet ID, Reddit post ID
	Author     string `bson:"author" json:"author"`           // Username/handle
	AuthorID   string `bson:"author_id" json:"author_id"`     // Platform-specific user ID
	Content    string `bson:"content" json:"content"`         // Post text
	URL        string `bson:"url" json:"url"`                 // Direct link to post

	// Analysis
	Sentiment      Sentiment `bson:"sentiment" json:"sentiment"`
	SentimentScore float64   `bson:"sentiment_score" json:"sentiment_score"` // -1.0 to 1.0

	// Engagement Metrics (updated via idempotent upserts)
	Likes    int  `bson:"likes" json:"likes"`
	Shares   int  `bson:"shares" json:"shares"`
	Comments int  `bson:"comments" json:"comments"`
	Views    int  `bson:"views" json:"views"`
	IsViral  bool `bson:"is_viral" json:"is_viral"` // Computed based on AlertConfig.ViralThreshold

	// Metadata
	PostedAt  time.Time `bson:"posted_at" json:"posted_at"`   // When the post was created on the platform
	FetchedAt time.Time `bson:"fetched_at" json:"fetched_at"` // When we scraped it
	ExpiresAt time.Time `bson:"expires_at" json:"expires_at"` // TTL index for data retention
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// Indexes for SocialPost collection:
// 1. {"platform": 1, "external_id": 1} - unique compound index (prevents duplicates)
// 2. {"project_id": 1, "posted_at": -1} - for timeline queries
// 3. {"project_id": 1, "sentiment": 1, "posted_at": -1} - for sentiment filtering
// 4. {"project_id": 1, "is_viral": 1, "posted_at": -1} - for viral content queries
// 5. {"expires_at": 1} - TTL index for automatic deletion based on data retention
// 6. {"fetched_at": 1} - for tracking scraping freshness
