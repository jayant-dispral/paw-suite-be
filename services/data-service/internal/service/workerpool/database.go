package workerpool

import (
	"context"
	"fmt"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (wp *WorkerPool) saveToDB(post domain.SocialPost) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    collection := wp.db.Collection("social_posts")
    
    // Upsert based on platform + external_id
    filter := bson.M{
        "platform":    post.Platform,
        "external_id": post.ExternalID,
    }
    
    // Update timestamps
    now := time.Now()
    post.UpdatedAt = now
    if post.CreatedAt.IsZero() {
        post.CreatedAt = now
    }
    
    update := bson.M{
        "$set": post,
        "$setOnInsert": bson.M{
            "created_at": now,
        },
    }
    
    opts := options.Update().SetUpsert(true)
    
    result, err := collection.UpdateOne(ctx, filter, update, opts)
    if err != nil {
        return fmt.Errorf("failed to upsert post: %w", err)
    }
    
    if result.UpsertedCount > 0 {
        fmt.Printf("  [DB] Inserted new post: %s\n", post.ExternalID)
    } else if result.ModifiedCount > 0 {
        fmt.Printf("  [DB] Updated existing post: %s\n", post.ExternalID)
    } else {
        fmt.Printf("  [DB] No changes for post: %s\n", post.ExternalID)
    }
    
    return nil
}