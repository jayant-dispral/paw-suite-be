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
    
    // Common Core Fields
    Platform   string    `bson:"platform" json:"platform" validate:"required,oneof=twitter reddit linkedin facebook"`
    ExternalID string    `bson:"external_id" json:"external_id" validate:"required"`
    Content    string    `bson:"content" json:"content" validate:"required"`
    URL        string    `bson:"url" json:"url" validate:"required,url"`
    PostedAt   time.Time `bson:"posted_at" json:"posted_at"`
    
    // Author Info
    Author Author `bson:"author" json:"author" validate:"required"`
    
    // Engagement
    Engagement Engagement `bson:"engagement" json:"engagement" validate:"required"`
    
    // Content Analysis
    Sentiment      Sentiment `bson:"sentiment" json:"sentiment" validate:"required,oneof=positive neutral negative"`
    SentimentScore float64   `bson:"sentiment_score" json:"sentiment_score" validate:"min=-1,max=1"`
    
    // Metadata
    Entities        Entities `bson:"entities" json:"entities"`
    MatchedKeywords []string `bson:"matched_keywords" json:"matched_keywords"`
    IsViral         bool     `bson:"is_viral" json:"is_viral"`
    
    // Timestamps
    FetchedAt  time.Time `bson:"fetched_at" json:"fetched_at"`
    IngestedAt time.Time `bson:"ingested_at" json:"ingested_at"`
    ExpiresAt  time.Time `bson:"expires_at" json:"expires_at"`
    CreatedAt  time.Time `bson:"created_at" json:"created_at"`
    UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
}

type Author struct {
    ID        string `bson:"id" json:"id" validate:"required"`
    Username  string `bson:"username" json:"username" validate:"required"`
    Name      string `bson:"name" json:"name"`
    Verified  bool   `bson:"verified" json:"verified"`
    Followers int    `bson:"followers" json:"followers" validate:"min=0"`
}

type Engagement struct {
    Likes    int `bson:"likes" json:"likes" validate:"min=0"`
    Shares   int `bson:"shares" json:"shares" validate:"min=0"`     // Retweets, Shares, Crosspost
    Comments int `bson:"comments" json:"comments" validate:"min=0"` // Replies, Comments
    Views    int `bson:"views" json:"views" validate:"min=0"`
}

type Entities struct {
    Hashtags []string `bson:"hashtags,omitempty" json:"hashtags,omitempty"`
    URLs     []string `bson:"urls,omitempty" json:"urls,omitempty"`
    Mentions []string `bson:"mentions,omitempty" json:"mentions,omitempty"`
}

// Indexes for SocialPost collection:
// 1. {"platform": 1, "external_id": 1} - unique compound index (prevents duplicates)
// 2. {"project_id": 1, "posted_at": -1} - for timeline queries
// 3. {"project_id": 1, "sentiment": 1, "posted_at": -1} - for sentiment filtering
// 4. {"project_id": 1, "is_viral": 1, "posted_at": -1} - for viral content queries
// 5. {"expires_at": 1} - TTL index for automatic deletion
// 6. {"fetched_at": 1} - for tracking scraping freshness
// 7. {"matched_keywords": 1, "project_id": 1} - for keyword analysis