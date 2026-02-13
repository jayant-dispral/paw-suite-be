package mongo

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/shared/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoSocialPostRepository struct {
	coll *mongo.Collection
}

func NewSocialPostRepository(db *mongo.Database) ports.SocialPostRepository {
	coll := db.Collection("social_posts")

	// Create indexes asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Index 1: Unique compound index to prevent duplicates (platform + external_id)
		_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "platform", Value: 1},
				{Key: "external_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		})
		if err != nil {
			log.Printf("failed to create platform-external_id unique index on social_posts: %v", err)
		}

		// Index 2: Timeline queries by project
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "posted_at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("failed to create project-posted_at index on social_posts: %v", err)
		}

		// Index 3: Sentiment filtering
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "sentiment", Value: 1},
				{Key: "posted_at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("failed to create project-sentiment index on social_posts: %v", err)
		}

		// Index 4: Viral content queries
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "project_id", Value: 1},
				{Key: "is_viral", Value: 1},
				{Key: "posted_at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("failed to create project-is_viral index on social_posts: %v", err)
		}

		// Index 5: TTL index for automatic deletion
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		})
		if err != nil {
			log.Printf("failed to create expires_at TTL index on social_posts: %v", err)
		}

		// Index 6: Track scraping freshness
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{Key: "fetched_at", Value: 1}},
		})
		if err != nil {
			log.Printf("failed to create fetched_at index on social_posts: %v", err)
		}

		// Index 7: Keyword analysis
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "matched_keywords", Value: 1},
				{Key: "project_id", Value: 1},
			},
		})
		if err != nil {
			log.Printf("failed to create matched_keywords index on social_posts: %v", err)
		}

		log.Println("✅ SocialPost indexes created successfully")
	}()

	return &MongoSocialPostRepository{
		coll: coll,
	}
}

// Create inserts a new social post
func (r *MongoSocialPostRepository) Create(ctx context.Context, post *domain.SocialPost) error {
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	post.IngestedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, post)
	if err != nil {
		return fmt.Errorf("failed to create social post: %w", err)
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		post.ID = oid
	}

	return nil
}

// BulkCreate inserts multiple social posts at once
func (r *MongoSocialPostRepository) BulkCreate(ctx context.Context, posts []domain.SocialPost) error {
	if len(posts) == 0 {
		return nil
	}

	now := time.Now()
	for i := range posts {
		posts[i].CreatedAt = now
		posts[i].UpdatedAt = now
		posts[i].IngestedAt = now
	}

	docs := make([]interface{}, len(posts))
	for i, post := range posts {
		docs[i] = post
	}

	opts := options.InsertMany().SetOrdered(false) // Continue on error for duplicates
	result, err := r.coll.InsertMany(ctx, docs, opts)
	if err != nil {
		// Check if it's a bulk write exception (some may have failed due to duplicates)
		if _, ok := err.(mongo.BulkWriteException); ok {
			log.Printf("some posts were skipped due to duplicates: %v", err)
			return nil
		}
		return fmt.Errorf("failed to bulk create social posts: %w", err)
	}

	log.Printf("✅ Inserted %d social posts", len(result.InsertedIDs))
	return nil
}

// FindByID retrieves a social post by its ObjectID
func (r *MongoSocialPostRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.SocialPost, error) {
	filter := bson.M{"_id": id}

	var post domain.SocialPost
	err := r.coll.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("social post not found"))
		}
		return nil, fmt.Errorf("failed to find social post: %w", err)
	}

	return &post, nil
}

// FindByExternalID retrieves a social post by platform and external_id
// This is used for duplicate detection during ingestion
func (r *MongoSocialPostRepository) FindByExternalID(ctx context.Context, platform, externalID string) (*domain.SocialPost, error) {
	filter := bson.M{
		"platform":    platform,
		"external_id": externalID,
	}

	var post domain.SocialPost
	err := r.coll.FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("social post not found"))
		}
		return nil, fmt.Errorf("failed to find social post by external ID: %w", err)
	}

	return &post, nil
}

// ExistsByExternalID checks if a post exists by platform and external_id
func (r *MongoSocialPostRepository) ExistsByExternalID(ctx context.Context, platform, externalID string) (bool, error) {
	filter := bson.M{
		"platform":    platform,
		"external_id": externalID,
	}

	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("failed to check social post existence: %w", err)
	}

	return count > 0, nil
}

// FindByProjectID retrieves all posts for a project with pagination
func (r *MongoSocialPostRepository) FindByProjectID(ctx context.Context, projectID primitive.ObjectID, opts *ports.SocialPostFilterOptions) ([]domain.SocialPost, error) {
	filter := bson.M{"project_id": projectID}

	findOpts := options.Find().
		SetSort(bson.D{{Key: "posted_at", Value: -1}})

	if opts != nil {
		if opts.Limit > 0 {
			findOpts.SetLimit(opts.Limit)
		}
		if opts.Offset > 0 {
			findOpts.SetSkip(opts.Offset)
		}
	}

	cursor, err := r.coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find social posts: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode social posts: %w", err)
	}

	return posts, nil
}

// FindByProjectIDAndSentiment retrieves posts filtered by sentiment
func (r *MongoSocialPostRepository) FindByProjectIDAndSentiment(ctx context.Context, projectID primitive.ObjectID, sentiment domain.Sentiment, limit int64) ([]domain.SocialPost, error) {
	filter := bson.M{
		"project_id": projectID,
		"sentiment":   sentiment,
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "posted_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find social posts by sentiment: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode social posts: %w", err)
	}

	return posts, nil
}

// FindViralPostsByProjectID retrieves viral posts for a project
func (r *MongoSocialPostRepository) FindViralPostsByProjectID(ctx context.Context, projectID primitive.ObjectID, limit int64) ([]domain.SocialPost, error) {
	filter := bson.M{
		"project_id": projectID,
		"is_viral":   true,
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "posted_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find viral posts: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode viral posts: %w", err)
	}

	return posts, nil
}

// FindByMatchedKeyword finds posts containing a specific keyword
func (r *MongoSocialPostRepository) FindByMatchedKeyword(ctx context.Context, projectID primitive.ObjectID, keyword string, limit int64) ([]domain.SocialPost, error) {
	filter := bson.M{
		"project_id":       projectID,
		"matched_keywords": keyword,
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "posted_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find posts by keyword: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode posts: %w", err)
	}

	return posts, nil
}

// Update updates a social post's analysis data
func (r *MongoSocialPostRepository) Update(ctx context.Context, post *domain.SocialPost) error {
	post.UpdatedAt = time.Now()

	filter := bson.M{"_id": post.ID}
	update := bson.M{"$set": post}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update social post: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("social post not found")
	}

	return nil
}

// Delete removes a social post by ID
func (r *MongoSocialPostRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}

	result, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete social post: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("social post not found")
	}

	return nil
}

// CountByProjectID counts all posts for a project
func (r *MongoSocialPostRepository) CountByProjectID(ctx context.Context, projectID primitive.ObjectID) (int64, error) {
	filter := bson.M{"project_id": projectID}

	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count social posts: %w", err)
	}

	return count, nil
}

// FindExpiredPosts retrieves posts that have passed their expiration date
// Used by cleanup jobs for manual deletion of expired content
func (r *MongoSocialPostRepository) FindExpiredPosts(ctx context.Context, limit int64) ([]domain.SocialPost, error) {
	filter := bson.M{
		"expires_at": bson.M{"$lte": time.Now()},
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "expires_at", Value: 1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find expired posts: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode expired posts: %w", err)
	}

	return posts, nil
}

// DeleteExpiredPosts deletes all posts that have expired
// Returns the count of deleted posts
func (r *MongoSocialPostRepository) DeleteExpiredPosts(ctx context.Context) (int64, error) {
	filter := bson.M{
		"expires_at": bson.M{"$lte": time.Now()},
	}

	result, err := r.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired posts: %w", err)
	}

	return result.DeletedCount, nil
}

// FindRecentPostsByProjectID finds posts fetched within a time window
func (r *MongoSocialPostRepository) FindRecentPostsByProjectID(ctx context.Context, projectID primitive.ObjectID, since time.Time, limit int64) ([]domain.SocialPost, error) {
	filter := bson.M{
		"project_id": projectID,
		"fetched_at":  bson.M{"$gte": since},
	}

	opts := options.Find().
		SetLimit(limit).
		SetSort(bson.D{{Key: "posted_at", Value: -1}})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find recent posts: %w", err)
	}
	defer cursor.Close(ctx)

	var posts []domain.SocialPost
	if err = cursor.All(ctx, &posts); err != nil {
		return nil, fmt.Errorf("failed to decode posts: %w", err)
	}

	return posts, nil
}

// CountBySentiment counts posts by sentiment for analytics
func (r *MongoSocialPostRepository) CountBySentiment(ctx context.Context, projectID primitive.ObjectID, sentiment domain.Sentiment) (int64, error) {
	filter := bson.M{
		"project_id": projectID,
		"sentiment":   sentiment,
	}

	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count posts by sentiment: %w", err)
	}

	return count, nil
}
