package mongo

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoProjectRepository struct {
	coll *mongo.Collection
}

func NewProjectRepository(db *mongo.Database) ports.ProjectRepository {
	coll := db.Collection("projects")

	//create indexes asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		//Index 1: Query user's project efficiently
		_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "owner_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "created_at", Value: -1},
			},
		})
		if err != nil {
			log.Printf("failed to create owner_id compound index on projects: %v", err)
		}

		//Index 2:Find projects where user is a team member
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{{
				Key: "team_members.user_id", Value: 1,
			}},
		})
		if err != nil {
			log.Printf("failed to create team_members index on projects: %v", err)
		}

		//Index 3: Query active projexts by platform (for monitoring jobs)
		_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "monitoring_config.platforms", Value: 1},
			},
		})
		if err != nil {
			log.Printf("failed to create status-platforms index on projects: %v", err)
		}

		log.Println("✅ Project indexes created successfully")
	}()

	return &MongoProjectRepository{
		coll: coll,
	}
}

func (r *MongoProjectRepository) Create(ctx context.Context, project *domain.Project) error {
	project.CreatedAt = time.Now()
	project.UpdatedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	// set the generated id back to the struct
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		project.ID = oid
	}

	return nil
}

func (r *MongoProjectRepository) FindByOwnerID(ctx context.Context, ownerID primitive.ObjectID) ([]domain.Project, error) {
	filter := bson.M{"owner_id": ownerID}

	opts := options.Find().SetSort(bson.D{
		{Key: "created_at", Value: -1},
	})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find projects: %w", err)
	}
	defer cursor.Close(ctx)

	var projects []domain.Project
	if err = cursor.All(ctx, &projects); err != nil {
		return nil, fmt.Errorf("failed to decode projectsL %w", err)
	}

	return projects, nil
}

// CountByID counts how many projects does the user have
func (r *MongoProjectRepository) CountByOwnerID(ctx context.Context, ownerID primitive.ObjectID) (int64, error) {
	filter := bson.M{"owner_id": ownerID}
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count projects: %w", err)
	}

	return count, nil
}

func (r *MongoProjectRepository) FindByID(ctx context.Context, projectID primitive.ObjectID) (*domain.Project, error) {
	filter := bson.M{"_id": projectID}

	var project domain.Project

	err := r.coll.FindOne(ctx, filter).Decode(&project)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
		}
		return nil, fmt.Errorf("failed to find project: %w", err)
	}

	return &project, nil
}

func (r *MongoProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	project.UpdatedAt = time.Now()

	filter := bson.M{"_id": project.ID}
	update := bson.M{"$set": project}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

func (r *MongoProjectRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}

	result, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}
