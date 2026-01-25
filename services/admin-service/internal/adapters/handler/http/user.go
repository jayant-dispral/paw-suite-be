package http

import (
	"net/http"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/response"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

type UserHandler struct {
	service ports.UserService
}

// NewUserHandler is the constructer
func NewUserHandler(service ports.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	//extract the userId from the context (set by auth middleware)
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		//this should never happen
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	user, err := h.service.GetUserProfile(r.Context(), userID)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, user)

}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	//Parse the request body
	var update domain.UpdateUserStruct

	if err := utils.DecodeJSON(w, r, &update); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	if err := h.service.UpdateUser(r.Context(), userID, update); err != nil {
		response.WithError(w, err)
		return
	}

	user, err := h.service.GetUserProfile(r.Context(), userID)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, user)
}
