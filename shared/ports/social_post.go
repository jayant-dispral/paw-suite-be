package ports

import (
	"context"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SocialPostFilterOptions defines filtering and pagination options
type SocialPostFilterOptions struct {
	Limit     int64
	Offset    int64
	Sentiment *domain.Sentiment
	Platform  *string
	StartDate *time.Time
	EndDate   *time.Time
}

// CreateSocialPostRequest represents a request to create a social post
type CreateSocialPostRequest struct {
	Platform        string            `json:"platform" validate:"required,oneof=twitter reddit linkedin facebook"`
	ExternalID      string            `json:"external_id" validate:"required"`
	Content         string            `json:"content" validate:"required"`
	URL             string            `json:"url" validate:"required,url"`
	PostedAt        time.Time         `json:"posted_at" validate:"required"`
	Author          domain.Author     `json:"author" validate:"required"`
	Engagement      domain.Engagement `json:"engagement" validate:"required"`
	Sentiment       domain.Sentiment  `json:"sentiment" validate:"required,oneof=positive neutral negative"`
	SentimentScore  float64           `json:"sentiment_score" validate:"min=-1,max=1"`
	Entities        domain.Entities   `json:"entities"`
	MatchedKeywords []string          `json:"matched_keywords"`
	IsViral         bool              `json:"is_viral"`
}

// PostStats holds statistics about social posts
type PostStats struct {
	TotalPosts    int64 `json:"total_posts"`
	PositivePosts int64 `json:"positive_posts"`
	NeutralPosts  int64 `json:"neutral_posts"`
	NegativePosts int64 `json:"negative_posts"`
	ViralPosts    int64 `json:"viral_posts"`
}

// PaginationMeta holds pagination metadata
type PaginationMeta struct {
	TotalCount int64 `json:"total_count"`
	Limit      int64 `json:"limit"`
	Offset     int64 `json:"offset"`
	HasMore    bool  `json:"has_more"`
}

// PaginatedResponse wraps a list of posts with pagination metadata
type PaginatedResponse struct {
	Data []domain.SocialPost `json:"data"`
	Meta PaginationMeta      `json:"meta"`
}

// SocialPostRepository interface for social post data access
type SocialPostRepository interface {
	// Create operations
	Create(ctx context.Context, post *domain.SocialPost) error
	BulkCreate(ctx context.Context, posts []domain.SocialPost) error

	// Read operations
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.SocialPost, error)
	FindByExternalID(ctx context.Context, platform, externalID string) (*domain.SocialPost, error)
	ExistsByExternalID(ctx context.Context, platform, externalID string) (bool, error)
	FindByProjectID(ctx context.Context, projectID primitive.ObjectID, opts *SocialPostFilterOptions) ([]domain.SocialPost, error)
	FindByProjectIDAndSentiment(ctx context.Context, projectID primitive.ObjectID, sentiment domain.Sentiment, limit int64) ([]domain.SocialPost, error)
	FindViralPostsByProjectID(ctx context.Context, projectID primitive.ObjectID, limit int64) ([]domain.SocialPost, error)
	FindByMatchedKeyword(ctx context.Context, projectID primitive.ObjectID, keyword string, limit int64) ([]domain.SocialPost, error)
	FindRecentPostsByProjectID(ctx context.Context, projectID primitive.ObjectID, since time.Time, limit int64) ([]domain.SocialPost, error)
	FindExpiredPosts(ctx context.Context, limit int64) ([]domain.SocialPost, error)

	// Count operations
	CountByProjectID(ctx context.Context, projectID primitive.ObjectID) (int64, error)
	CountBySentiment(ctx context.Context, projectID primitive.ObjectID, sentiment domain.Sentiment) (int64, error)

	// Update operations
	Update(ctx context.Context, post *domain.SocialPost) error

	// Delete operations
	Delete(ctx context.Context, id primitive.ObjectID) error
	DeleteExpiredPosts(ctx context.Context) (int64, error)
}

// SocialPostService interface for social post business logic
type SocialPostService interface {
	// Read operations
	GetPostsByProjectID(ctx context.Context, projectIDStr, userIDStr string, opts *SocialPostFilterOptions) (*PaginatedResponse, error)
	GetPostByID(ctx context.Context, postIDStr, projectIDStr, userIDStr string) (*domain.SocialPost, error)
	GetPostsBySentiment(ctx context.Context, projectIDStr, userIDStr string, sentiment domain.Sentiment, limit int64) ([]domain.SocialPost, error)
	GetViralPosts(ctx context.Context, projectIDStr, userIDStr string, limit int64) ([]domain.SocialPost, error)
	GetPostsByKeyword(ctx context.Context, projectIDStr, userIDStr string, keyword string, limit int64) ([]domain.SocialPost, error)
	GetPostStats(ctx context.Context, projectIDStr, userIDStr string) (*PostStats, error)

	// Write operations
	IngestPost(ctx context.Context, projectIDStr string, req *CreateSocialPostRequest) (*domain.SocialPost, error)
	BulkIngestPosts(ctx context.Context, projectIDStr string, posts []CreateSocialPostRequest) (int64, error)
}
