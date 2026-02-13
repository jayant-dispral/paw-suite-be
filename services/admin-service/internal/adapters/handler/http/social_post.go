package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/response"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// SocialPostHandler handles HTTP requests for social posts
type SocialPostHandler struct {
	service ports.SocialPostService
}

func NewSocialPostHandler(service ports.SocialPostService) *SocialPostHandler {
	return &SocialPostHandler{
		service: service,
	}
}

// GetPosts retrieves all posts for a project
func (h *SocialPostHandler) GetPosts(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	// Parse query parameters
	opts := &ports.SocialPostFilterOptions{}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.ParseInt(limit, 10, 64); err == nil {
			opts.Limit = l
		}
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.ParseInt(offset, 10, 64); err == nil {
			opts.Offset = o
		}
	}

	if sentiment := r.URL.Query().Get("sentiment"); sentiment != "" {
		s := domain.Sentiment(sentiment)
		if s == domain.SentimentPositive || s == domain.SentimentNeutral || s == domain.SentimentNegative {
			opts.Sentiment = &s
		}
	}

	posts, err := h.service.GetPostsByProjectID(r.Context(), projectID, userID, opts)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Posts fetched successfully", posts)
}

// GetPost retrieves a single post by ID
func (h *SocialPostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	postID := chi.URLParam(r, "postID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	post, err := h.service.GetPostByID(r.Context(), postID, projectID, userID)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Post fetched successfully", post)
}

// GetPostsBySentiment retrieves posts filtered by sentiment
func (h *SocialPostHandler) GetPostsBySentiment(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	sentiment := chi.URLParam(r, "sentiment")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	limit := int64(100) // Default limit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil {
			limit = parsed
		}
	}

	posts, err := h.service.GetPostsBySentiment(r.Context(), projectID, userID, domain.Sentiment(sentiment), limit)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Posts fetched successfully", posts)
}

// GetViralPosts retrieves viral posts for a project
func (h *SocialPostHandler) GetViralPosts(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	limit := int64(50) // Default limit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil {
			limit = parsed
		}
	}

	posts, err := h.service.GetViralPosts(r.Context(), projectID, userID, limit)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Viral posts fetched successfully", posts)
}

// GetPostsByKeyword retrieves posts containing a specific keyword
func (h *SocialPostHandler) GetPostsByKeyword(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	keyword := chi.URLParam(r, "keyword")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	limit := int64(100) // Default limit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 64); err == nil {
			limit = parsed
		}
	}

	posts, err := h.service.GetPostsByKeyword(r.Context(), projectID, userID, keyword, limit)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Posts fetched successfully", posts)
}

// GetPostStats retrieves post statistics for a project
func (h *SocialPostHandler) GetPostStats(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	stats, err := h.service.GetPostStats(r.Context(), projectID, userID)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Post statistics fetched successfully", stats)
}

// IngestPostRequest represents the request body for ingesting a single post
type IngestPostRequest struct {
	Platform        string   `json:"platform"`
	ExternalID      string   `json:"external_id"`
	Content         string   `json:"content"`
	URL             string   `json:"url"`
	PostedAt        string   `json:"posted_at"` // RFC3339 format
	AuthorID        string   `json:"author_id"`
	AuthorUsername  string   `json:"author_username"`
	AuthorName      string   `json:"author_name"`
	AuthorVerified  bool     `json:"author_verified"`
	AuthorFollowers int      `json:"author_followers"`
	Likes           int      `json:"likes"`
	Shares          int      `json:"shares"`
	Comments        int      `json:"comments"`
	Views           int      `json:"views"`
	Sentiment       string   `json:"sentiment"`
	SentimentScore  float64  `json:"sentiment_score"`
	Hashtags        []string `json:"hashtags"`
	Mentions        []string `json:"mentions"`
	MatchedKeywords []string `json:"matched_keywords"`
	IsViral         bool     `json:"is_viral"`
}

// IngestPost ingests a single social post
func (h *SocialPostHandler) IngestPost(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")

	var req IngestPostRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	// Parse posted_at time
	postedAt, err := time.Parse(time.RFC3339, req.PostedAt)
	if err != nil {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	// Build the service request
	serviceReq := &ports.CreateSocialPostRequest{
		Platform:       req.Platform,
		ExternalID:     req.ExternalID,
		Content:        req.Content,
		URL:            req.URL,
		PostedAt:       postedAt,
		Author: domain.Author{
			ID:        req.AuthorID,
			Username:  req.AuthorUsername,
			Name:      req.AuthorName,
			Verified:  req.AuthorVerified,
			Followers: req.AuthorFollowers,
		},
		Engagement: domain.Engagement{
			Likes:    req.Likes,
			Shares:   req.Shares,
			Comments: req.Comments,
			Views:    req.Views,
		},
		Sentiment:       domain.Sentiment(req.Sentiment),
		SentimentScore:  req.SentimentScore,
		Entities: domain.Entities{
			Hashtags: req.Hashtags,
			Mentions: req.Mentions,
		},
		MatchedKeywords: req.MatchedKeywords,
		IsViral:         req.IsViral,
	}

	post, err := h.service.IngestPost(r.Context(), projectID, serviceReq)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusCreated, "Post ingested successfully", post)
}

// BulkIngestRequest represents the request body for bulk ingestion
type BulkIngestRequest struct {
	Posts []IngestPostRequest `json:"posts"`
}

// BulkIngestPosts ingests multiple social posts
func (h *SocialPostHandler) BulkIngestPosts(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")

	var req BulkIngestRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	// Convert to service requests
	serviceReqs := make([]ports.CreateSocialPostRequest, 0, len(req.Posts))
	for _, p := range req.Posts {
		postedAt, err := time.Parse(time.RFC3339, p.PostedAt)
		if err != nil {
			response.WithError(w, domain.ErrInvalidInput)
			return
		}

		serviceReqs = append(serviceReqs, ports.CreateSocialPostRequest{
			Platform:        p.Platform,
			ExternalID:      p.ExternalID,
			Content:         p.Content,
			URL:             p.URL,
			PostedAt:        postedAt,
			Author: domain.Author{
				ID:        p.AuthorID,
				Username:  p.AuthorUsername,
				Name:      p.AuthorName,
				Verified:  p.AuthorVerified,
				Followers: p.AuthorFollowers,
			},
			Engagement: domain.Engagement{
				Likes:    p.Likes,
				Shares:   p.Shares,
				Comments: p.Comments,
				Views:    p.Views,
			},
			Sentiment:       domain.Sentiment(p.Sentiment),
			SentimentScore:  p.SentimentScore,
			Entities: domain.Entities{
				Hashtags: p.Hashtags,
				Mentions: p.Mentions,
			},
			MatchedKeywords: p.MatchedKeywords,
			IsViral:         p.IsViral,
		})
	}

	count, err := h.service.BulkIngestPosts(r.Context(), projectID, serviceReqs)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusCreated, "Posts ingested successfully", map[string]interface{}{
		"ingested_count": count,
	})
}
