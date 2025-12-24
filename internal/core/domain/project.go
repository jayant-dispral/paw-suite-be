package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// project.go       -> Project, ProjectStatus, Handle, MonitoringConfig, AlertConfig, TeamMember, TeamMemberRole

type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectPaused   ProjectStatus = "paused"
	ProjectArchived ProjectStatus = "archived"
)

type Project struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID     primitive.ObjectID `bson:"owner_id" json:"owner_id"` //referneces to the user
	Name        string             `bson:"name" json:"name" validate:"required,min1,max=100"`
	Description string             `bson:"description" json:"description"`
	Status      ProjectStatus      `bson:"status" json:"status"`

	//Brand indentity
	BrandName       string   `bson:"brand_name" json:"brand_name"`
	PrimaryDomain   string   `bson:"primary_domain" json:"primary_domain"`
	OfficialHandles []Handle `bson:"official_handles" json:"official_handles"`
	BrandLogoURL    string   `bson:"brand_logo_url,omitempty" json:"brand_logo_url,omitempty"`

	//Monitoring configuration
	MonitoringConfig MonitoringConfig `bson:"monitoring_config" json:"monitoring_config"`
	AlertConfig      AlertConfig      `bson:"alert_config" json:"alert_config"`

	//Team Management
	TeamMembers []TeamMember `bson:"team_members" json:"team_members"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type Handle struct {
	Platform string `bson:"platform" json:"platform"` //twitter linkedin
	Handle   string `bson:"handle" json:"handle"`     //@sentinelSupport
	Verified bool   `bson:"verified" json:"verified"`
}

type MonitoringConfig struct {
	Keywords     []string `bson:"keywords" json:"keywords"`
	Hashtags     []string `bson:"hashtags" json:"hashtags"`
	Handles      []string `bson:"handles" json:"handles"`
	Platforms    []string `bson:"platforms" json:"platforms"`
	ScanFrequncy int      `bson:"scan_frequncy" json:"scan_frequncy"` //in minitues (5,10,15..)

	//Threat Radar settings
	EnableTyposquatting   bool `bson:"enable_typosquatting" json:"enable_typosquatting"`
	EnableImpersonating   bool `bson:"enable_impersonating" json:"enable_impersonating"`
	TyposquattingScanFreq int  `bson:"typosquatting_scan_freq" json:"typosquatting_scan_freq"` // in hours
}

type AlertConfig struct {
	// Sentiments Thresholds
	NegativeSpikeThreshold int `bson:"negative_spike_threshold" json:"negative_spike_threshold"` //10 post in 5 min
	SpikeWindowMinutes     int `bson:"spike_window_minutes" json:"spike_window_minutes"`

	//Engagement Thresholds
	ViralThreshold int `bson:"viral_threshold" json:"viral_threshold"`

	//Notification Channels
	EmailRecipients []string `bson:"email_recipients" json:"email_recipients"`
	SlackWebHookURL string   `bson:"slack_web_hook_url" json:"slack_web_hook_url"`

	//Alert prefrences
	QuietHoursStart *time.Time `bson:"quiet_hours_start" json:"quiet_hours_start"`
	QuiteHoursEnd   *time.Time `bson:"quite_hours_end" json:"quite_hours_end"`
}

type TeamMemberRole string

const (
	RoleAdmin  TeamMemberRole = "admin"  //Full control
	RoleEditor TeamMemberRole = "editor" //can modify settings, acknowledge alerts
	RoleViewer TeamMemberRole = "viewer" // read-only access
)

type TeamMember struct {
	UserID  primitive.ObjectID `bson:"user_id" json:"user_id"`
	Role    TeamMemberRole     `bson:"role" json:"role"`
	AddedAt time.Time          `bson:"added_at" json:"added_at"`
	AddedBy primitive.ObjectID `bson:"added_by" json:"added_by"`
}

// Indexes for Project collection:
// 1. {"owner_id": 1, "status": 1, "created_at": -1}
// 2. {"team_members.user_id": 1} - for finding projects where user is a member
// 3. {"status": 1, "monitoring_config.platforms": 1} - for active monitoring queries
