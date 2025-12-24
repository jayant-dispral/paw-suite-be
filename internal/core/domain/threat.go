// threat.go        -> Threat, ThreatType, ThreatStatus, ThreatDetails
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ThreatType string

const (
	ThreatTyposquatting ThreatType = "typosquatting"
	ThreatImpersonation ThreatType = "impersonation"
	ThreatPhishing      ThreatType = "phishing"
)

type ThreatStatus string

const (
	ThreatActive        ThreatStatus = "active"         // Newly detected
	ThreatAcknowledged  ThreatStatus = "acknowledged"   // Team is aware
	ThreatFalsePositive ThreatStatus = "false_positive" // Dismissed
	ThreatResolved      ThreatStatus = "resolved"       // Threat neutralized (domain taken down, etc.)
)

type Threat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`

	Type     ThreatType   `bson:"type" json:"type"`
	Status   ThreatStatus `bson:"status" json:"status"`
	Severity string       `bson:"severity" json:"severity"` // "low", "medium", "high", "critical"

	// Threat Details (polymorphic based on Type)
	Details ThreatDetails `bson:"details" json:"details"`

	// Resolution
	AcknowledgedBy primitive.ObjectID `bson:"acknowledged_by,omitempty" json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time         `bson:"acknowledged_at,omitempty" json:"acknowledged_at,omitempty"`
	ResolvedBy     primitive.ObjectID `bson:"resolved_by,omitempty" json:"resolved_by,omitempty"`
	ResolvedAt     *time.Time         `bson:"resolved_at,omitempty" json:"resolved_at,omitempty"`
	Notes          string             `bson:"notes,omitempty" json:"notes,omitempty"` // Team notes

	DetectedAt time.Time `bson:"detected_at" json:"detected_at"`
	ExpiresAt  time.Time `bson:"expires_at" json:"expires_at"` // TTL for resolved threats
	CreatedAt  time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at" json:"updated_at"`
}

type ThreatDetails struct {
	// Typosquatting
	SuspiciousDomain string            `bson:"suspicious_domain,omitempty" json:"suspicious_domain,omitempty"`
	Registrar        string            `bson:"registrar,omitempty" json:"registrar,omitempty"`
	RegistrationDate *time.Time        `bson:"registration_date,omitempty" json:"registration_date,omitempty"`
	DNSRecords       map[string]string `bson:"dns_records,omitempty" json:"dns_records,omitempty"`

	// Impersonation
	Platform      string `bson:"platform,omitempty" json:"platform,omitempty"`
	FakeHandle    string `bson:"fake_handle,omitempty" json:"fake_handle,omitempty"`
	ProfileURL    string `bson:"profile_url,omitempty" json:"profile_url,omitempty"`
	ScreenshotURL string `bson:"screenshot_url,omitempty" json:"screenshot_url,omitempty"`

	// Common
	SimilarityScore float64 `bson:"similarity_score,omitempty" json:"similarity_score,omitempty"` // 0-100%
	Description     string  `bson:"description,omitempty" json:"description,omitempty"`
}

// Indexes for Threat collection:
// 1. {"project_id": 1, "status": 1, "detected_at": -1}
// 2. {"project_id": 1, "type": 1, "status": 1}
// 3. {"status": 1, "severity": 1} - for prioritizing active threats
// 4. {"expires_at": 1} - TTL index
// 5. {"details.suspicious_domain": 1} - unique sparse index (prevent duplicate domain alerts)
