package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	"github.com/jayant-dispral/brand-threat-be/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/pkg/response"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectHandler struct {
	projectService ports.ProjectService
}

func NewProjectHandler(projectService ports.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		//this should never happen
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	//parse request body
	var req domain.CreateProjectRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	// service layer call
	project, err := h.projectService.CreateProject(r.Context(), userID, &req)
	if err != nil {
		if err.Error() == "project limit reached" {
			response.WithError(w, err)
			return
		}

		response.WithError(w, err)
		return
	}

	//4. Return success response
	response.JSONWithMessage(w, http.StatusCreated, "Create new project success fully", map[string]interface{}{
		"success": true,
		"data":    project,
	})
}

func (h *ProjectHandler) GetMyProjects(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		// this should never happen cause AuthMiddleware will reject if not present
		response.WithError(w, domain.ErrUnauthorized)
		return
	}
	userID, err := utils.HexToObjectID(userIDStr)
	if err != nil {
		response.WithError(w, err)
	}
	projects, err := h.projectService.GetProjectsByUserId(r.Context(), userID)

	if err != nil {
		response.WithError(w, err)
	}

	response.JSONWithMessage(w, http.StatusOK, "Projects fetched successfully", projects)
}

func (h *ProjectHandler) GetProjectDetails(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	project, err := h.projectService.GetProjectDetails(r.Context(), projectIDStr, userIDStr)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Project details fetched successfully", project)
}
func (h *ProjectHandler) UpdateProjectDetails(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var req domain.UpdateProjectRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	project, err := h.projectService.UpdateProjectDetails(r.Context(), projectIDStr, userIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Project updated successfully", project)
}

func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	err := h.projectService.DeleteProject(r.Context(), projectIDStr, userIDStr)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Project deleted successfully", nil)
}

func (h *ProjectHandler) UpdateProjectStatus(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var req domain.UpdateProjectStatusRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	err := h.projectService.UpdateProjectStatus(r.Context(), projectIDStr, userIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Project status updated successfully", nil)
}

func (h *ProjectHandler) UpdateMonitoringConfig(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var req domain.MonitoringConfig
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	err := h.projectService.UpdateMonitoringConfig(r.Context(), projectIDStr, userIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Monitoring config updated successfully", nil)
}

func (h *ProjectHandler) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var req domain.AlertConfig
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	err := h.projectService.UpdateAlertConfig(r.Context(), projectIDStr, userIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Alert config updated successfully", nil)
}

func (h *ProjectHandler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var req domain.AddTeamMemberRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	err := h.projectService.AddTeamMember(r.Context(), projectIDStr, userIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusCreated, "Team member added successfully", nil)
}

func (h *ProjectHandler) UpdateTeamMemberRole(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	memberUserIDStr := chi.URLParam(r, "userId")
	if memberUserIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	var req domain.UpdateTeamMemberRoleRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	err := h.projectService.UpdateTeamMemberRole(r.Context(), projectIDStr, userIDStr, memberUserIDStr, &req)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Team member role updated successfully", nil)
}

func (h *ProjectHandler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	memberUserIDStr := chi.URLParam(r, "userId")
	if memberUserIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	err := h.projectService.RemoveTeamMember(r.Context(), projectIDStr, userIDStr, memberUserIDStr)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Team member removed successfully", nil)
}

func (h *ProjectHandler) GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		response.WithError(w, domain.ErrInvalidInput)
		return
	}

	userIDStr, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userIDStr == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	teamMembers, err := h.projectService.GetTeamMembers(r.Context(), projectIDStr, userIDStr)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Team members fetched successfully", teamMembers)
}
