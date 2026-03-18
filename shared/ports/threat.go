package ports

import (
	"context"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ThreatIntelSummary struct {
	BrandName            string `json:"brand_name"`
	ProtectedDomain      string `json:"protected_domain"`
	TotalGenerated       int    `json:"total_generated"`
	ValidCandidates      int    `json:"valid_candidates"`
	EnrichedCandidates   int    `json:"enriched_candidates"`
	SuppressedCandidates int    `json:"suppressed_candidates"`
	SurfacedFindings     int    `json:"surfaced_findings"`
	ActiveFindings       int    `json:"active_findings"`
	CriticalFindings     int    `json:"critical_findings"`
}

type ThreatIntelResponse struct {
	Summary  ThreatIntelSummary `json:"summary"`
	Scan     ThreatScanInfo     `json:"scan"`
	Findings []domain.Threat    `json:"findings"`
}

type ThreatScanInfo struct {
	Status              domain.ThreatScanStatus `json:"status"`
	SourceDomain        string                  `json:"source_domain,omitempty"`
	// FIX: removed omitempty — omitempty on int omits the field when value is 0.
	// During Stage 1 (DNS fan-out, before any enrichment completes) the value is
	// genuinely 0 and was disappearing from the JSON response entirely.
	// The frontend handled it via ?? 0, so the progress bar showed 0% instead of
	// a loading indicator. Without omitempty the field is always present so the
	// frontend can distinguish "not started" (0) from "missing field" (undefined).
	ProcessedCandidates int                     `json:"processed_candidates"`
	CurrentCandidate    string                  `json:"current_candidate,omitempty"`
	CurrentAlgorithm    string                  `json:"current_algorithm,omitempty"`
	LastRequestedAt     *time.Time              `json:"last_requested_at,omitempty"`
	LastStartedAt       *time.Time              `json:"last_started_at,omitempty"`
	LastCompletedAt     *time.Time              `json:"last_completed_at,omitempty"`
	LastError           string                  `json:"last_error,omitempty"`
}

type ThreatRepository interface {
	CreateMany(ctx context.Context, threats []domain.Threat) error
	CountByProjectID(ctx context.Context, projectID primitive.ObjectID) (int64, error)
	DeleteByProjectID(ctx context.Context, projectID primitive.ObjectID) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Threat, error)
	FindByProjectID(ctx context.Context, projectID primitive.ObjectID) ([]domain.Threat, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status domain.ThreatStatus, actorID primitive.ObjectID) error
}

type ThreatScanStateRepository interface {
	GetByProjectID(ctx context.Context, projectID primitive.ObjectID) (*domain.ThreatScanState, error)
	Upsert(ctx context.Context, state *domain.ThreatScanState) error
}

// ThreatService defines the application-layer contract.
//
// FIX: Added WhitelistThreat and AcknowledgeThreat to match:
//   - domain.ThreatWhitelisted (newly added to domain/threat.go)
//   - domain.ThreatAcknowledged (already in domain/threat.go but had no service method)
//   - mongo UpdateStatus which now handles all four non-active statuses
//
// The HTTP handler (http/threat.go) must expose routes for these two new
// methods. See http/threat.go for the updated handler.
type ThreatService interface {
	GetThreatIntel(ctx context.Context, projectIDStr, userIDStr string) (*ThreatIntelResponse, error)
	// RefreshThreatIntel forces a new scan to start immediately (if not already running).
	// Intended for manual refresh actions from the UI.
	RefreshThreatIntel(ctx context.Context, projectIDStr, userIDStr string) (*ThreatIntelResponse, error)
	ResolveThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error
	IgnoreThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error
	// WhitelistThreat permanently suppresses a threat (10-year TTL).
	// Use when the domain is legitimately owned by the brand or a partner.
	WhitelistThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error
	// AcknowledgeThreat marks the team as aware without dismissing.
	// The threat remains active and visible; expiry stays at 1 month.
	AcknowledgeThreat(ctx context.Context, projectIDStr, threatIDStr, userIDStr string) error
}
