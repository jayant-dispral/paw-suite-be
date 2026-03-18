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

type MongoThreatScanStateRepository struct {
	coll *mongo.Collection
}

func NewThreatScanStateRepository(db *mongo.Database) ports.ThreatScanStateRepository {
	coll := db.Collection("threat_scan_states")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "project_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		})
		if err != nil {
			log.Printf("failed to create project_id unique index on threat_scan_states: %v", err)
		}
	}()

	return &MongoThreatScanStateRepository{coll: coll}
}

func (r *MongoThreatScanStateRepository) GetByProjectID(ctx context.Context, projectID primitive.ObjectID) (*domain.ThreatScanState, error) {
	var state domain.ThreatScanState
	err := r.coll.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&state)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("threat scan state not found"))
		}
		return nil, fmt.Errorf("failed to find threat scan state: %w", err)
	}
	return &state, nil
}

func (r *MongoThreatScanStateRepository) Upsert(ctx context.Context, state *domain.ThreatScanState) error {
	now := time.Now().UTC()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	update := bson.M{
		"$set": bson.M{
			"status":             state.Status,
			"source_domain":      state.SourceDomain,
			"source_fingerprint": state.SourceFingerprint,
			"current_candidate":  state.CurrentCandidate,
			"current_algorithm":  state.CurrentAlgorithm,
			"last_requested_at":  state.LastRequestedAt,
			"last_started_at":    state.LastStartedAt,
			"last_completed_at":  state.LastCompletedAt,
			"last_error":         state.LastError,
			"metrics":            state.Metrics,
			"updated_at":         state.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"project_id": state.ProjectID,
			"created_at": state.CreatedAt,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx, bson.M{"project_id": state.ProjectID}, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert threat scan state: %w", err)
	}

	return nil
}
