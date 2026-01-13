package project

import (
	"context"
	"fmt"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectService struct {
	projectRepo ports.ProjectRepository
	userRepo    ports.UserRepository
}

func NewProjectService(projectRepo ports.ProjectRepository, userRepo ports.UserRepository) *ProjectService {
	return &ProjectService{
		projectRepo: projectRepo,
		userRepo:    userRepo,
	}
}

func (s *ProjectService) GetProjectsByUserId(ctx context.Context, ownerId primitive.ObjectID) ([]domain.Project, error) {
	projects, err := s.projectRepo.FindByOwnerID(ctx, ownerId)
	if err != nil {

	}
	return projects, err
}

// CreateProject validated subscription limits and creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, ownerID primitive.ObjectID, req *domain.CreateProjectRequest) (*domain.Project, error) {

	// 1. get the user to check for the subscription limits
	user, err := s.userRepo.GetUserById(ctx, ownerID.Hex())
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// check if the user had reached project limit
	curreProjectCount, err := s.projectRepo.CountByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to count projects: %w", err)
	}

	if curreProjectCount >= int64(user.Subscription.MaxProjects) {
		return nil, fmt.Errorf("project limit reached: %d/%d (updgrage your plan)", curreProjectCount, user.Subscription.MaxProjects)
	}

	//validate keywords against subscription
	if len(req.MonitoringConfig.Keywords) > user.Subscription.MaxKeywords {
		return nil, fmt.Errorf("keyword limit exceeded: %d/%d allowed", len(req.MonitoringConfig.Keywords), user.Subscription.MaxProjects)
	}

	// 4. build the project domain object
	project := &domain.Project{
		OwnerID:          ownerID,
		Name:             req.Name,
		Description:      req.Description,
		Status:           domain.ProjectActive,
		BrandName:        req.BrandName,
		PrimaryDomain:    req.PrimaryDomain,
		OfficialHandles:  req.OfficialHandles,
		MonitoringConfig: req.MonitoringConfig,
		AlertConfig:      req.AlertConfig,
		TeamMembers:      []domain.TeamMember{}, //Empty for now; user can add later
	}

	//create in databse
	err = s.projectRepo.Create(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to create a project: %w", err)
	}

	return project, nil
}
