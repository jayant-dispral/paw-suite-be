package social_post

import (
	"context"
	"fmt"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/shared/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

type SocialPostService struct {
	socialPostRepo ports.SocialPostRepository
	projectRepo    ports.ProjectRepository
}

func NewSocialPostService(socialPostRepo ports.SocialPostRepository, projectRepo ports.ProjectRepository) ports.SocialPostService {
	return &SocialPostService{
		socialPostRepo: socialPostRepo,
		projectRepo:    projectRepo,
	}
}

// GetPostsByProjectID retrieves all posts for a project with pagination
func (s *SocialPostService) GetPostsByProjectID(ctx context.Context, projectIDStr, userIDStr string, opts *ports.SocialPostFilterOptions) (*ports.PaginatedResponse, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	// Check if user has access to this project
	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Set default limit if not provided
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	// Fetch posts
	posts, err := s.socialPostRepo.FindByProjectID(ctx, projectID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}

	// Get total count for pagination
	totalCount, err := s.socialPostRepo.CountByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to count posts: %w", err)
	}

	// Calculate has_more
	currentOffset := opts.Offset
	if currentOffset < 0 {
		currentOffset = 0
	}
	hasMore := (currentOffset + int64(len(posts))) < totalCount

	return &ports.PaginatedResponse{
		Data: posts,
		Meta: ports.PaginationMeta{
			TotalCount: totalCount,
			Limit:      opts.Limit,
			Offset:     currentOffset,
			HasMore:    hasMore,
		},
	}, nil
}

// GetPostByID retrieves a single post by ID
func (s *SocialPostService) GetPostByID(ctx context.Context, postIDStr, projectIDStr, userIDStr string) (*domain.SocialPost, error) {
	postID, err := utils.HexToObjectID(postIDStr)
	if err != nil {
		return nil, err
	}

	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	// Check if user has access to the project
	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	post, err := s.socialPostRepo.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}

	// Verify post belongs to the project
	if post.ProjectID != projectID {
		return nil, pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: post does not belong to this project"))
	}

	return post, nil
}

// GetPostsBySentiment retrieves posts filtered by sentiment
func (s *SocialPostService) GetPostsBySentiment(ctx context.Context, projectIDStr, userIDStr string, sentiment domain.Sentiment, limit int64) ([]domain.SocialPost, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	posts, err := s.socialPostRepo.FindByProjectIDAndSentiment(ctx, projectID, sentiment, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts by sentiment: %w", err)
	}

	return posts, nil
}

// GetViralPosts retrieves viral posts for a project
func (s *SocialPostService) GetViralPosts(ctx context.Context, projectIDStr, userIDStr string, limit int64) ([]domain.SocialPost, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	posts, err := s.socialPostRepo.FindViralPostsByProjectID(ctx, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch viral posts: %w", err)
	}

	return posts, nil
}

// GetPostsByKeyword retrieves posts containing a specific keyword
func (s *SocialPostService) GetPostsByKeyword(ctx context.Context, projectIDStr, userIDStr string, keyword string, limit int64) ([]domain.SocialPost, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	posts, err := s.socialPostRepo.FindByMatchedKeyword(ctx, projectID, keyword, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch posts by keyword: %w", err)
	}

	return posts, nil
}

// GetPostStats retrieves post statistics for a project
func (s *SocialPostService) GetPostStats(ctx context.Context, projectIDStr, userIDStr string) (*ports.PostStats, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	totalPosts, err := s.socialPostRepo.CountByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to count total posts: %w", err)
	}

	positivePosts, err := s.socialPostRepo.CountBySentiment(ctx, projectID, domain.SentimentPositive)
	if err != nil {
		return nil, fmt.Errorf("failed to count positive posts: %w", err)
	}

	neutralPosts, err := s.socialPostRepo.CountBySentiment(ctx, projectID, domain.SentimentNeutral)
	if err != nil {
		return nil, fmt.Errorf("failed to count neutral posts: %w", err)
	}

	negativePosts, err := s.socialPostRepo.CountBySentiment(ctx, projectID, domain.SentimentNegative)
	if err != nil {
		return nil, fmt.Errorf("failed to count negative posts: %w", err)
	}

	return &ports.PostStats{
		TotalPosts:    totalPosts,
		PositivePosts: positivePosts,
		NeutralPosts:  neutralPosts,
		NegativePosts: negativePosts,
		ViralPosts:    0, // Could be added as a separate count method
	}, nil
}

// IngestPost ingests a single social post (called by data-service typically)
func (s *SocialPostService) IngestPost(ctx context.Context, projectIDStr string, req *ports.CreateSocialPostRequest) (*domain.SocialPost, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	// Check if post already exists (deduplication)
	exists, err := s.socialPostRepo.ExistsByExternalID(ctx, req.Platform, req.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("failed to check post existence: %w", err)
	}
	if exists {
		return nil, pkgerrors.NewError(domain.ErrConflict, fmt.Errorf("post already exists"))
	}

	// Verify project exists
	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	now := time.Now()
	post := &domain.SocialPost{
		ProjectID:       projectID,
		Platform:        req.Platform,
		ExternalID:      req.ExternalID,
		Content:         req.Content,
		URL:             req.URL,
		PostedAt:        req.PostedAt,
		Author:          req.Author,
		Engagement:      req.Engagement,
		Sentiment:       req.Sentiment,
		SentimentScore:  req.SentimentScore,
		Entities:        req.Entities,
		MatchedKeywords: req.MatchedKeywords,
		IsViral:         req.IsViral,
		FetchedAt:       now,
		ExpiresAt:       now.AddDate(0, 0, 30), // Default 30 days retention
	}

	if err := s.socialPostRepo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return post, nil
}

// BulkIngestPosts ingests multiple social posts
func (s *SocialPostService) BulkIngestPosts(ctx context.Context, projectIDStr string, posts []ports.CreateSocialPostRequest) (int64, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return 0, err
	}

	// Verify project exists
	_, err = s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return 0, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	now := time.Now()
	domainPosts := make([]domain.SocialPost, 0, len(posts))

	for _, req := range posts {
		domainPosts = append(domainPosts, domain.SocialPost{
			ProjectID:       projectID,
			Platform:        req.Platform,
			ExternalID:      req.ExternalID,
			Content:         req.Content,
			URL:             req.URL,
			PostedAt:        req.PostedAt,
			Author:          req.Author,
			Engagement:      req.Engagement,
			Sentiment:       req.Sentiment,
			SentimentScore:  req.SentimentScore,
			Entities:        req.Entities,
			MatchedKeywords: req.MatchedKeywords,
			IsViral:         req.IsViral,
			FetchedAt:       now,
			ExpiresAt:       now.AddDate(0, 0, 30), // Default 30 days retention
		})
	}

	if err := s.socialPostRepo.BulkCreate(ctx, domainPosts); err != nil {
		return 0, fmt.Errorf("failed to bulk create posts: %w", err)
	}

	return int64(len(domainPosts)), nil
}
