package http

import (
	"log"
	"net/http"

	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/response"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// Auth handler wraps the logic so that the http requests can talk to it
type AuthHandler struct {
	service ports.AuthService
}

// NewAuthHandler is the constructor
func NewAuthHandler(svc ports.AuthService) *AuthHandler {
	return &AuthHandler{
		service: svc,
	}
}

// Data transfer objects DTOs.

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	FullName string `json:"full_name" validate:"required,min=3,max=24"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type authResponse struct {
	Token  string `json:"token,omitempty"`
	UserId string `json:"user_id,omitempty"`
}

// RegisterHandler
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	//calling the service
	userId, err := h.service.Register(r.Context(), req.Email, req.Password, req.FullName)
	if err != nil {
		log.Printf("Auth register error: %v", err)
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusCreated, "the user was created successfully", authResponse{UserId: userId})
}

// Login Handler
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := utils.DecodeJSON(w, r, &req); err != nil {
		if vErrs := utils.ParseValidationError(err); vErrs != nil {
			response.JSONValidation(w, vErrs)
			return
		}
		response.WithError(w, err)
		return
	}

	token, userId, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("Auth login error: %v", err)
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusCreated, "the user was created successfully", authResponse{UserId: userId, Token: token})
}
