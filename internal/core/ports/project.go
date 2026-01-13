package ports

import (
	"context"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
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
}
