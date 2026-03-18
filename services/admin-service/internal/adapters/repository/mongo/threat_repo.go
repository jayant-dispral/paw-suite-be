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

type MongoThreatRepository struct {
	coll *mongo.Collection
}

func NewThreatRepository(db *mongo.Database) ports.ThreatRepository {
	coll := db.Collection("threats")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		indexes := []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: "project_id", Value: 1},
					{Key: "details.suspicious_domain", Value: 1},
				},
				Options: options.Index().SetUnique(true).SetSparse(true),
			},
			{
				Keys: bson.D{
					{Key: "project_id", Value: 1},
					{Key: "status", Value: 1},
					{Key: "score", Value: -1},
					{Key: "detected_at", Value: -1},
				},
			},
			{
				Keys: bson.D{
					{Key: "project_id", Value: 1},
					{Key: "type", Value: 1},
					{Key: "severity", Value: 1},
				},
			},
			{
				Keys:    bson.D{{Key: "expires_at", Value: 1}},
				Options: options.Index().SetExpireAfterSeconds(0),
			},
		}

		if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
			log.Printf("failed to create indexes on threats: %v", err)
			return
		}

		log.Println("✅ Threat indexes created successfully")
	}()

	return &MongoThreatRepository{coll: coll}
}

func (r *MongoThreatRepository) CreateMany(ctx context.Context, threats []domain.Threat) error {
	if len(threats) == 0 {
		return nil
	}

	now := time.Now().UTC()
	docs := make([]interface{}, 0, len(threats))
	for i := range threats {
		if threats[i].CreatedAt.IsZero() {
			threats[i].CreatedAt = now
		}
		if threats[i].UpdatedAt.IsZero() {
			threats[i].UpdatedAt = now
		}
		if threats[i].DetectedAt.IsZero() {
			threats[i].DetectedAt = now
		}
		if threats[i].ExpiresAt.IsZero() {
			threats[i].ExpiresAt = now.AddDate(0, 1, 0)
		}
		docs = append(docs, threats[i])
	}

	_, err := r.coll.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("failed to create threats: %w", err)
	}

	return nil
}

func (r *MongoThreatRepository) CountByProjectID(ctx context.Context, projectID primitive.ObjectID) (int64, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{"project_id": projectID})
	if err != nil {
		return 0, fmt.Errorf("failed to count threats: %w", err)
	}
	return count, nil
}

func (r *MongoThreatRepository) DeleteByProjectID(ctx context.Context, projectID primitive.ObjectID) error {
	if _, err := r.coll.DeleteMany(ctx, bson.M{"project_id": projectID}); err != nil {
		return fmt.Errorf("failed to delete threats for project: %w", err)
	}
	return nil
}

func (r *MongoThreatRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Threat, error) {
	var threat domain.Threat
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&threat); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("threat not found"))
		}
		return nil, fmt.Errorf("failed to find threat: %w", err)
	}
	return &threat, nil
}

func (r *MongoThreatRepository) FindByProjectID(ctx context.Context, projectID primitive.ObjectID) ([]domain.Threat, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "score", Value: -1},
		{Key: "detected_at", Value: -1},
	})

	cursor, err := r.coll.Find(ctx, bson.M{"project_id": projectID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list threats: %w", err)
	}
	defer cursor.Close(ctx)

	var threats []domain.Threat
	if err := cursor.All(ctx, &threats); err != nil {
		return nil, fmt.Errorf("failed to decode threats: %w", err)
	}

	return threats, nil
}

// UpdateStatus persists a user-initiated status transition and sets ExpiresAt
// using the same canonical logic as service.go's expiryForStatus(), keeping
// the two in sync.
//
// FIX 1: The original ThreatResolved case set expires_at = now+1month, but
// service.go expiryForStatus() sets it to now+2months. Aligned to 2 months.
//
// FIX 2: The original default case set expires_at = now+10years, but
// service.go expiryForStatus() defaults to now+1month. Aligned to 1 month.
//
// FIX 3: Added ThreatWhitelisted case (now defined in domain/threat.go) which
// sets expires_at = now+10years, matching service.go expiryForStatus().
//
// Single source of truth: whenever expiry durations need changing, update
// service.go expiryForStatus() and mirror the same values here.
func (r *MongoThreatRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status domain.ThreatStatus, actorID primitive.ObjectID) error {
	now := time.Now().UTC()
	setFields := bson.M{
		"status":     status,
		"updated_at": now,
	}

	switch status {
	case domain.ThreatResolved:
		// FIX: was now+1month; canonical value (service.go) is now+2months.
		setFields["resolved_by"] = actorID
		setFields["resolved_at"] = now
		setFields["expires_at"] = now.AddDate(0, 2, 0)

	case domain.ThreatFalsePositive:
		setFields["acknowledged_by"] = actorID
		setFields["acknowledged_at"] = now
		setFields["expires_at"] = now.AddDate(0, 2, 0)

	case domain.ThreatWhitelisted:
		// FIX: new case — permanently suppressed; 10-year TTL prevents re-surfacing.
		setFields["acknowledged_by"] = actorID
		setFields["acknowledged_at"] = now
		setFields["expires_at"] = now.AddDate(10, 0, 0)

	case domain.ThreatAcknowledged:
		setFields["acknowledged_by"] = actorID
		setFields["acknowledged_at"] = now
		// Acknowledged threats stay in the active TTL bucket.
		setFields["expires_at"] = now.AddDate(0, 1, 0)

	default:
		// FIX: was now+10years; canonical value (service.go) is now+1month.
		setFields["expires_at"] = now.AddDate(0, 1, 0)
	}

	result, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": setFields})
	if err != nil {
		return fmt.Errorf("failed to update threat status: %w", err)
	}
	if result.MatchedCount == 0 {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("threat not found"))
	}

	return nil
}