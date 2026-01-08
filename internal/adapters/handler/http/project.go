package http

import (
	"net/http"

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
	w.Header().Set("Content-Type", "application/json")
	response.JSONWithMessage(w, http.StatusCreated, "Create new project success fully", map[string]interface{}{
		"success": true,
		"data":    project,
	})
}

// func (h *ProjectHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
// 	userId, ok := r.Context().Value(UserIDKey).(primitive.ObjectID)

// 	projects, err := h.projectService.
// }
