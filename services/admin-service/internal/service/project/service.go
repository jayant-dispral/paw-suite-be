package project

import (
	"context"
	"fmt"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	pkgerrors "github.com/jayant-dispral/brand-threat-be/shared/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
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
		return nil, err
	}
	for i := range projects {
		s.hydrateTeamMemberEmails(ctx, projects[i].TeamMembers)
	}
	return projects, nil
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
	project.NextScanAt = time.Now().Add(
		time.Duration(project.MonitoringConfig.ScanFrequency) * time.Minute,
	)
	//create in databse
	err = s.projectRepo.Create(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to create a project: %w", err)
	}

	return project, nil
}

func (s *ProjectService) GetProjectDetails(ctx context.Context, projectIDStr, userIDStr string) (*domain.Project, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, fmt.Errorf("Project is empty")
	}

	var userExistsInTeam = false
	for _, teamMember := range project.TeamMembers {
		if teamMember.UserID.Hex() == userIDStr {
			userExistsInTeam = true
			break
		}
	}

	if project.OwnerID.Hex() != userIDStr && !userExistsInTeam {
		return nil, pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: you must be the project owner or a team member to view this project"))
	}

	s.hydrateTeamMemberEmails(ctx, project.TeamMembers)
	return project, nil
}

func (s *ProjectService) UpdateProjectDetails(ctx context.Context, projectIDStr, userIDStr string, req *domain.UpdateProjectRequest) (*domain.Project, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or admin team member
	if project.OwnerID.Hex() != userIDStr {
		isAdmin := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && tm.Role == domain.RoleAdmin {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			return nil, pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	// Apply updates
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.BrandName != nil {
		project.BrandName = *req.BrandName
	}
	if req.PrimaryDomain != nil {
		project.PrimaryDomain = *req.PrimaryDomain
	}
	if req.OfficialHandles != nil {
		project.OfficialHandles = *req.OfficialHandles
	}
	if req.MonitoringConfig != nil {
		project.MonitoringConfig = *req.MonitoringConfig
	}
	if req.AlertConfig != nil {
		project.AlertConfig = *req.AlertConfig
	}

	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	s.hydrateTeamMemberEmails(ctx, project.TeamMembers)
	return project, nil
}

func (s *ProjectService) DeleteProject(ctx context.Context, projectIDStr, userIDStr string) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Only owner can delete
	if project.OwnerID.Hex() != userIDStr {
		return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: only owner can delete project"))
	}

	// Soft delete: set status to archived
	project.Status = domain.ProjectArchived
	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to archive project: %w", err)
	}

	return nil
}

func (s *ProjectService) UpdateProjectStatus(ctx context.Context, projectIDStr, userIDStr string, req *domain.UpdateProjectStatusRequest) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or admin
	if project.OwnerID.Hex() != userIDStr {
		isAdmin := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && tm.Role == domain.RoleAdmin {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	project.Status = req.Status
	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}

func (s *ProjectService) UpdateMonitoringConfig(ctx context.Context, projectIDStr, userIDStr string, req *domain.MonitoringConfig) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or editor/admin
	if project.OwnerID.Hex() != userIDStr {
		hasPermission := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && (tm.Role == domain.RoleAdmin || tm.Role == domain.RoleEditor) {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	project.MonitoringConfig = *req
	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to update monitoring config: %w", err)
	}

	return nil
}

func (s *ProjectService) UpdateAlertConfig(ctx context.Context, projectIDStr, userIDStr string, req *domain.AlertConfig) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or editor/admin
	if project.OwnerID.Hex() != userIDStr {
		hasPermission := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && (tm.Role == domain.RoleAdmin || tm.Role == domain.RoleEditor) {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	project.AlertConfig = *req
	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to update alert config: %w", err)
	}

	return nil
}

func (s *ProjectService) AddTeamMember(ctx context.Context, projectIDStr, userIDStr string, req *domain.AddTeamMemberRequest) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or admin
	if project.OwnerID.Hex() != userIDStr {
		isAdmin := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && tm.Role == domain.RoleAdmin {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	// Check if user is already a member
	inviteeUser, err := s.userRepo.GetUserByEmail(ctx, req.UserEmail)
	if err != nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("user with email %s not found", req.UserEmail))
	}

	for _, tm := range project.TeamMembers {
		if tm.UserID == inviteeUser.ID {
			return pkgerrors.NewError(domain.ErrConflict, fmt.Errorf("user is already a team member"))
		}
	}

	// Add member
	addedBy, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return pkgerrors.NewError(domain.ErrInvalidInput, fmt.Errorf("invalid user_id for added_by"))
	}
	newMember := domain.TeamMember{
		UserID:  inviteeUser.ID,
		Email:   inviteeUser.Email,
		Role:    req.Role,
		AddedAt: time.Now(),
		AddedBy: addedBy,
	}
	project.TeamMembers = append(project.TeamMembers, newMember)

	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}

	return nil
}

func (s *ProjectService) UpdateTeamMemberRole(ctx context.Context, projectIDStr, userIDStr, memberUserIDStr string, req *domain.UpdateTeamMemberRoleRequest) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or admin
	if project.OwnerID.Hex() != userIDStr {
		isAdmin := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && tm.Role == domain.RoleAdmin {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	// Find and update member
	memberUserID, err := primitive.ObjectIDFromHex(memberUserIDStr)
	if err != nil {
		return pkgerrors.NewError(domain.ErrInvalidInput, fmt.Errorf("invalid member user_id"))
	}

	found := false
	for i, tm := range project.TeamMembers {
		if tm.UserID == memberUserID {
			project.TeamMembers[i].Role = req.Role
			found = true
			break
		}
	}

	if !found {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("team member not found"))
	}

	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to update team member role: %w", err)
	}

	return nil
}

func (s *ProjectService) RemoveTeamMember(ctx context.Context, projectIDStr, userIDStr, memberUserIDStr string) error {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}

	if project == nil {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check permissions: owner or admin
	if project.OwnerID.Hex() != userIDStr {
		isAdmin := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr && tm.Role == domain.RoleAdmin {
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			return pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: insufficient permissions"))
		}
	}

	// Find and remove member
	memberUserID, err := primitive.ObjectIDFromHex(memberUserIDStr)
	if err != nil {
		return pkgerrors.NewError(domain.ErrInvalidInput, fmt.Errorf("invalid member user_id"))
	}

	index := -1
	for i, tm := range project.TeamMembers {
		if tm.UserID == memberUserID {
			index = i
			break
		}
	}

	if index == -1 {
		return pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("team member not found"))
	}

	// Remove from slice
	project.TeamMembers = append(project.TeamMembers[:index], project.TeamMembers[index+1:]...)

	err = s.projectRepo.Update(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	return nil
}

func (s *ProjectService) GetTeamMembers(ctx context.Context, projectIDStr, userIDStr string) ([]domain.TeamMember, error) {
	projectID, err := utils.HexToObjectID(projectIDStr)
	if err != nil {
		return nil, err
	}

	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project == nil {
		return nil, pkgerrors.NewError(domain.ErrNotFound, fmt.Errorf("project not found"))
	}

	// Check access: owner or team member
	if project.OwnerID.Hex() != userIDStr {
		isMember := false
		for _, tm := range project.TeamMembers {
			if tm.UserID.Hex() == userIDStr {
				isMember = true
				break
			}
		}
		if !isMember {
			return nil, pkgerrors.NewError(domain.ErrForbidden, fmt.Errorf("access denied: you must be the project owner or a team member"))
		}
	}

	s.hydrateTeamMemberEmails(ctx, project.TeamMembers)
	return project.TeamMembers, nil
}

func (s *ProjectService) hydrateTeamMemberEmails(ctx context.Context, members []domain.TeamMember) {
	for i := range members {
		if members[i].Email != "" {
			continue
		}
		user, err := s.userRepo.GetUserById(ctx, members[i].UserID.Hex())
		if err != nil {
			continue
		}
		members[i].Email = user.Email
	}
}
