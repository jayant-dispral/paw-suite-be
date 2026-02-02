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
    
    now := time.Now()
    post.UpdatedAt = now
    
    // Prepare update document
    update := bson.M{
        "$set": post,
        "$setOnInsert": bson.M{
            "created_at": now,  // Only set on insert
        },
    }
    
    // Remove created_at from $set to avoid conflict
    postBSON, err := bson.Marshal(post)
    if err != nil {
        return fmt.Errorf("failed to marshal post: %w", err)
    }
    
    var postMap bson.M
    if err := bson.Unmarshal(postBSON, &postMap); err != nil {
        return fmt.Errorf("failed to unmarshal post: %w", err)
    }
    
    // Remove created_at from the map that goes into $set
    delete(postMap, "created_at")
    
    update = bson.M{
        "$set":         postMap,
        "$setOnInsert": bson.M{"created_at": now},
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