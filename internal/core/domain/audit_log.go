// audit_log.go     -> AuditLog, AuditAction
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditAction string

const (
	ActionProjectCreated     AuditAction = "project_created"
	ActionProjectUpdated     AuditAction = "project_updated"
	ActionThreatAcknowledged AuditAction = "threat_acknowledged"
	ActionThreatResolved     AuditAction = "threat_resolved"
	ActionAlertDismissed     AuditAction = "alert_dismissed"
	ActionTeamMemberAdded    AuditAction = "team_member_added"
	ActionTeamMemberRemoved  AuditAction = "team_member_removed"
)

type AuditLog struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID primitive.ObjectID `bson:"project_id" json:"project_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"` // Who performed the action

	Action     AuditAction            `bson:"action" json:"action"`
	EntityType string                 `bson:"entity_type" json:"entity_type"` // "threat", "alert", "project"
	EntityID   primitive.ObjectID     `bson:"entity_id,omitempty" json:"entity_id,omitempty"`
	Changes    map[string]interface{} `bson:"changes,omitempty" json:"changes,omitempty"` // Before/after values
	IPAddress  string                 `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	UserAgent  string                 `bson:"user_agent,omitempty" json:"user_agent,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	ExpiresAt time.Time `bson:"expires_at" json:"expires_at"` // TTL for compliance
}

// Indexes for AuditLog collection:
// 1. {"project_id": 1, "created_at": -1}
// 2. {"user_id": 1, "created_at": -1}
// 3. {"expires_at": 1} - TTL index
