package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectRepository interface {
	Create(ctx context.Context, project *domain.Project) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*domain.Project, error)
	FindByOwnerID(ctx context.Context, ownerID primitive.ObjectID) ([]domain.Project, error)
	Update(ctx context.Context, project *domain.Project) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	CountByOwnerID(ctx context.Context, ownerID primitive.ObjectID) (int64, error)
}

type ProjectService interface {
	CreateProject(ctx context.Context, ownerID primitive.ObjectID, req *domain.CreateProjectRequest) (*domain.Project, error)
	GetProjectsByUserId(ctx context.Context, ownerId primitive.ObjectID) ([]domain.Project, error)
	GetProjectDetails(ctx context.Context, projectIDStr, userIDStr string) (*domain.Project, error)
	UpdateProjectDetails(ctx context.Context, projectIDStr, userIDStr string, req *domain.UpdateProjectRequest) (*domain.Project, error)
	DeleteProject(ctx context.Context, projectIDStr, userIDStr string) error
	UpdateProjectStatus(ctx context.Context, projectIDStr, userIDStr string, req *domain.UpdateProjectStatusRequest) error
	UpdateMonitoringConfig(ctx context.Context, projectIDStr, userIDStr string, req *domain.MonitoringConfig) error
	UpdateAlertConfig(ctx context.Context, projectIDStr, userIDStr string, req *domain.AlertConfig) error
	AddTeamMember(ctx context.Context, projectIDStr, userIDStr string, req *domain.AddTeamMemberRequest) error
	UpdateTeamMemberRole(ctx context.Context, projectIDStr, userIDStr, memberUserIDStr string, req *domain.UpdateTeamMemberRoleRequest) error
	RemoveTeamMember(ctx context.Context, projectIDStr, userIDStr, memberUserIDStr string) error
	GetTeamMembers(ctx context.Context, projectIDStr, userIDStr string) ([]domain.TeamMember, error)
}
