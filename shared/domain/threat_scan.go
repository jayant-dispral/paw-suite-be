package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ThreatScanStatus string

const (
	ThreatScanIdle    ThreatScanStatus = "idle"
	ThreatScanRunning ThreatScanStatus = "running"
	ThreatScanReady   ThreatScanStatus = "ready"
	ThreatScanFailed  ThreatScanStatus = "failed"
)

type ThreatScanMetrics struct {
	TotalGenerated      int `bson:"total_generated" json:"total_generated"`
	ValidCandidates     int `bson:"valid_candidates" json:"valid_candidates"`
	ProcessedCandidates int `bson:"processed_candidates" json:"processed_candidates"`
	EnrichedCandidates  int `bson:"enriched_candidates" json:"enriched_candidates"`
	SurfacedFindings    int `bson:"surfaced_findings" json:"surfaced_findings"`
	Suppressed          int `bson:"suppressed_candidates" json:"suppressed_candidates"`
}

type ThreatScanState struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID         primitive.ObjectID `bson:"project_id" json:"project_id"`
	Status            ThreatScanStatus   `bson:"status" json:"status"`
	SourceDomain      string             `bson:"source_domain,omitempty" json:"source_domain,omitempty"`
	SourceFingerprint string             `bson:"source_fingerprint,omitempty" json:"source_fingerprint,omitempty"`
	CurrentCandidate  string             `bson:"current_candidate,omitempty" json:"current_candidate,omitempty"`
	CurrentAlgorithm  string             `bson:"current_algorithm,omitempty" json:"current_algorithm,omitempty"`
	LastRequestedAt   *time.Time         `bson:"last_requested_at,omitempty" json:"last_requested_at,omitempty"`
	LastStartedAt     *time.Time         `bson:"last_started_at,omitempty" json:"last_started_at,omitempty"`
	LastCompletedAt   *time.Time         `bson:"last_completed_at,omitempty" json:"last_completed_at,omitempty"`
	LastError         string             `bson:"last_error,omitempty" json:"last_error,omitempty"`
	Metrics           ThreatScanMetrics  `bson:"metrics,omitempty" json:"metrics,omitempty"`
	UpdatedAt         time.Time          `bson:"updated_at" json:"updated_at"`
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
}
