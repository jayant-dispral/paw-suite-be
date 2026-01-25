// alert.go         -> Alert, AlertTrigger, AlertStatus
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AlertTrigger string

const (
	AlertNegativeSpike AlertTrigger = "negative_spike"
	AlertViralContent  AlertTrigger = "viral_content"
	AlertNewThreat     AlertTrigger = "new_threat"
	AlertManual        AlertTrigger = "manual" // User-created alert
)

type AlertStatus string

const (
	AlertPending      AlertStatus = "pending"      // Waiting to be sent
	AlertSent         AlertStatus = "sent"         // Successfully delivered
	AlertFailed       AlertStatus = "failed"       // Delivery failed
	AlertAcknowledged AlertStatus = "acknowledged" // User marked as read
	AlertDismissed    AlertStatus = "dismissed"    // User dismissed
)

type Alert struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`

	Trigger  AlertTrigger `bson:"trigger" json:"trigger" validate:"required,oneof=negative_spike viral_content new_threat manual"`
	Status   AlertStatus  `bson:"status" json:"status" validate:"required,oneof=pending sent failed acknowledged dismissed"`
	Severity string       `bson:"severity" json:"severity" validate:"required,oneof=info warning critical"` // "info", "warning", "critical"

	// Alert Content
	Title     string `bson:"title" json:"title" validate:"required,max=255"`
	Message   string `bson:"message" json:"message" validate:"required"`
	ActionURL string `bson:"action_url,omitempty" json:"action_url,omitempty" validate:"omitempty,url"` // Link to relevant dashboard view

	// Related Entities
	RelatedPostIDs  []primitive.ObjectID `bson:"related_post_ids,omitempty" json:"related_post_ids,omitempty"`
	RelatedThreatID *primitive.ObjectID  `bson:"related_threat_id,omitempty" json:"related_threat_id,omitempty"`

	// Delivery
	EmailSent bool       `bson:"email_sent" json:"email_sent"`
	SlackSent bool       `bson:"slack_sent" json:"slack_sent"`
	SentAt    *time.Time `bson:"sent_at,omitempty" json:"sent_at,omitempty"`

	// User Interaction
	AcknowledgedBy primitive.ObjectID `bson:"acknowledged_by,omitempty" json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time         `bson:"acknowledged_at,omitempty" json:"acknowledged_at,omitempty"`
	DismissedBy    primitive.ObjectID `bson:"dismissed_by,omitempty" json:"dismissed_by,omitempty"`
	DismissedAt    *time.Time         `bson:"dismissed_at,omitempty" json:"dismissed_at,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	ExpiresAt time.Time `bson:"expires_at" json:"expires_at"` // TTL for old alerts
}

// Indexes for Alert collection:
// 1. {"project_id": 1, "status": 1, "created_at": -1}
// 2. {"project_id": 1, "trigger": 1, "created_at": -1}
// 3. {"status": 1, "created_at": 1} - for processing pending alerts (FIFO)
// 4. {"expires_at": 1} - TTL index
