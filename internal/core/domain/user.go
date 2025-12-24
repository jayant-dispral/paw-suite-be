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
	FullName     string             `bson:"full_name" json:"full_name"`
	Subscription Subscription       `bson:"subscription" json:"subscription"`
	Preferences  UserPreferences    `bson:"preferences" json:"preferences"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}

type Subscription struct {
	Tier              SubscriptionTier `bson:"tier" json:"tier"`
	StartDate         time.Time        `bson:"start_date" json:"start_date"`
	EndDate           *time.Time       `bson:"end_date,omitempty" json:"end_date,omitempty"` //nil for active subscriptions
	MaxProjects       int              `bson:"max_projects" json:"max_projects"`
	MaxKeywords       int              `bson:"max_keywords" json:"max_keywords"`
	DataRetentionDays int              `bson:"data_retention_days" json:"data_retention_days"`
	APICallsPerMonth  int              `bson:"api_calls_per_month" json:"api_calls_per_month"`
}

type UserPreferences struct {
	EmailNotifications bool   `bson:"email_notification" json:"email_notification"`
	Timezone           string `bson:"timezone" json:"timezone"`
}

// Indexes for User collection:
// 1. {"email": 1} - unique
// 2. {"subscription.tier": 1, "created_at": -1} - analytics queries
