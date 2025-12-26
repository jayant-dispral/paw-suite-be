package http

import (
	"net/http"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	"github.com/jayant-dispral/brand-threat-be/pkg/response"
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
