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
	ThreatResolved      ThreatStatus = "resolved"       // Threat neutralized
	// FIX: ThreatWhitelisted was missing but referenced in service.go expiryForStatus().
	// Added here to match the canonical expiryForStatus() logic which gives whitelisted
	// threats a 10-year expiry so they are never accidentally re-surfaced.
	ThreatWhitelisted ThreatStatus = "whitelisted" // Permanently suppressed by brand owner
)

type Threat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`

	Type     ThreatType   `bson:"type" json:"type" validate:"required,oneof=typosquatting impersonation phishing"`
	// FIX: validate tag extended to include "whitelisted" alongside existing statuses.
	Status   ThreatStatus `bson:"status" json:"status" validate:"required,oneof=active acknowledged false_positive resolved whitelisted"`
	Severity string       `bson:"severity" json:"severity" validate:"required,oneof=low medium high critical"`
	Score    int          `bson:"score" json:"score" validate:"min=0,max=100"`
	Summary  string       `bson:"summary,omitempty" json:"summary,omitempty"`

	// Threat Details (polymorphic based on Type)
	Details ThreatDetails `bson:"details" json:"details" validate:"required"`

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

type ThreatSignal struct {
	Signal string `bson:"signal" json:"signal"`
	Weight int    `bson:"weight" json:"weight"`
	Detail string `bson:"detail" json:"detail"`
}

type ThreatRecommendedAction struct {
	Key         string `bson:"key" json:"key"`
	Label       string `bson:"label" json:"label"`
	Description string `bson:"description" json:"description"`
	URL         string `bson:"url,omitempty" json:"url,omitempty"`
	Priority    int    `bson:"priority" json:"priority"`
}

type ThreatDNSProfile struct {
	ARecords   []string `bson:"a_records,omitempty" json:"a_records,omitempty"`
	MXRecords  []string `bson:"mx_records,omitempty" json:"mx_records,omitempty"`
	NSRecords  []string `bson:"ns_records,omitempty" json:"ns_records,omitempty"`
	TXTRecords []string `bson:"txt_records,omitempty" json:"txt_records,omitempty"`
	GeoLocation string  `bson:"geo_location,omitempty" json:"geo_location,omitempty"`
	HasMX      bool     `bson:"has_mx" json:"has_mx"`
	Resolves   bool     `bson:"resolves" json:"resolves"`
}

type ThreatSSLProfile struct {
	HasCertificate bool       `bson:"has_certificate" json:"has_certificate"`
	Issuer         string     `bson:"issuer,omitempty" json:"issuer,omitempty"`
	CommonName     string     `bson:"common_name,omitempty" json:"common_name,omitempty"`
	ValidFrom      *time.Time `bson:"valid_from,omitempty" json:"valid_from,omitempty"`
	ValidTo        *time.Time `bson:"valid_to,omitempty" json:"valid_to,omitempty"`
}

type ThreatWebProfile struct {
	IsLive            bool     `bson:"is_live" json:"is_live"`
	HTTPStatus        int      `bson:"http_status" json:"http_status"`
	HTTPBanner        string   `bson:"http_banner,omitempty" json:"http_banner,omitempty"`
	Title             string   `bson:"title,omitempty" json:"title,omitempty"`
	HasLoginForm      bool     `bson:"has_login_form" json:"has_login_form"`
	LooksLikeBrand    bool     `bson:"looks_like_brand" json:"looks_like_brand"`
	IsParkingPage     bool     `bson:"is_parking_page" json:"is_parking_page"`
	Technologies      []string `bson:"technologies,omitempty" json:"technologies,omitempty"`
	VisualSimilarity  float64  `bson:"visual_similarity" json:"visual_similarity"`
	ContentSimilarity float64  `bson:"content_similarity" json:"content_similarity"`
	FuzzyMatchScore   float64  `bson:"fuzzy_match_score,omitempty" json:"fuzzy_match_score,omitempty"`
}

type ThreatBusinessProfile struct {
	TargetCategory    string  `bson:"target_category,omitempty" json:"target_category,omitempty"`
	CandidateCategory string  `bson:"candidate_category,omitempty" json:"candidate_category,omitempty"`
	Confidence        float64 `bson:"confidence" json:"confidence"`
	Similarity        float64 `bson:"similarity" json:"similarity"`
	MatchesTarget     bool    `bson:"matches_target" json:"matches_target"`
}

type ThreatDetails struct {
	// Typosquatting
	SuspiciousDomain string            `bson:"suspicious_domain,omitempty" json:"suspicious_domain,omitempty" validate:"omitempty,fqdn"`
	Registrar        string            `bson:"registrar,omitempty" json:"registrar,omitempty"`
	RegistrationDate *time.Time        `bson:"registration_date,omitempty" json:"registration_date,omitempty"`
	DNSRecords       map[string]string `bson:"dns_records,omitempty" json:"dns_records,omitempty"`

	// Impersonation
	Platform      string `bson:"platform,omitempty" json:"platform,omitempty"`
	FakeHandle    string `bson:"fake_handle,omitempty" json:"fake_handle,omitempty"`
	ProfileURL    string `bson:"profile_url,omitempty" json:"profile_url,omitempty" validate:"omitempty,url"`
	ScreenshotURL string `bson:"screenshot_url,omitempty" json:"screenshot_url,omitempty" validate:"omitempty,url"`

	// Common
	SimilarityScore float64 `bson:"similarity_score,omitempty" json:"similarity_score,omitempty" validate:"min=0,max=100"`
	Description     string  `bson:"description,omitempty" json:"description,omitempty"`

	// Rich threat intel
	PrimaryDomain      string                    `bson:"primary_domain,omitempty" json:"primary_domain,omitempty"`
	PermutationType    string                    `bson:"permutation_type,omitempty" json:"permutation_type,omitempty"`
	RDAPURL            string                    `bson:"rdap_url,omitempty" json:"rdap_url,omitempty"`
	AgeDeltaDays       int                       `bson:"age_delta_days,omitempty" json:"age_delta_days,omitempty"`
	OlderThanPrimary   bool                      `bson:"older_than_primary,omitempty" json:"older_than_primary,omitempty"`
	DNS                ThreatDNSProfile          `bson:"dns,omitempty" json:"dns,omitempty"`
	SSL                ThreatSSLProfile          `bson:"ssl,omitempty" json:"ssl,omitempty"`
	Web                ThreatWebProfile          `bson:"web,omitempty" json:"web,omitempty"`
	Business           ThreatBusinessProfile     `bson:"business,omitempty" json:"business,omitempty"`
	Signals            []ThreatSignal            `bson:"signals,omitempty" json:"signals,omitempty"`
	RecommendedActions []ThreatRecommendedAction `bson:"recommended_actions,omitempty" json:"recommended_actions,omitempty"`
}

// Indexes for Threat collection:
// 1. {"project_id": 1, "status": 1, "detected_at": -1}
// 2. {"project_id": 1, "type": 1, "status": 1}
// 3. {"status": 1, "severity": 1} - for prioritizing active threats
// 4. {"expires_at": 1} - TTL index
// 5. {"details.suspicious_domain": 1} - unique sparse index (prevent duplicate domain alerts)