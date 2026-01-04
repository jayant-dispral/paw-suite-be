package domain

// user.go -> User, Subscription, SubscriptionTier, UserPreferences
import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubscriptionTier string

const (
	TierFree       SubscriptionTier = "free"
	TierPro        SubscriptionTier = "pro"
	TierEnterprise SubscriptionTier = "enterprise"
)

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email" validate:"required,email"`
	Password     string             `bson:"password" json:"-"` // never return password
	FullName     string             `bson:"full_name" json:"full_name" validate:"required,min=2,max=100"`
	Subscription Subscription       `bson:"subscription" json:"subscription" validate:"required"`
	Preferences  UserPreferences    `bson:"preferences" json:"preferences" validate:"required"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}

type Subscription struct {
	Tier              SubscriptionTier `bson:"tier" json:"tier" validate:"required,oneof=free pro enterprise"`
	StartDate         time.Time        `bson:"start_date" json:"start_date"`
	EndDate           *time.Time       `bson:"end_date,omitempty" json:"end_date,omitempty"` //nil for active subscriptions
	MaxProjects       int              `bson:"max_projects" json:"max_projects" validate:"min=1"`
	MaxKeywords       int              `bson:"max_keywords" json:"max_keywords" validate:"min=1"`
	DataRetentionDays int              `bson:"data_retention_days" json:"data_retention_days" validate:"min=1"`
	APICallsPerMonth  int              `bson:"api_calls_per_month" json:"api_calls_per_month" validate:"min=1"`
}

type UserPreferences struct {
	EmailNotifications bool   `bson:"email_notification" json:"email_notification"`
	Timezone           string `bson:"timezone" json:"timezone" validate:"required,timezone"`
}

// Indexes for User collection:
// 1. {"email": 1} - unique
// 2. {"subscription.tier": 1, "created_at": -1} - analytics queries

type UpdateUserStruct struct {
	FullName    *string          `bson:"full_name" json:"full_name,omitempty" validate:"omitempty,min=2,max=100"`
	Preferences *UserPreferences `bson:"preferences" json:"preferences,omitempty" validate:"omitempty"`
}
