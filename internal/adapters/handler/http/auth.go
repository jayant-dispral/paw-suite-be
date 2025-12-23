package http

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token  string `json:"token",omitempty`
	UserId string `json:"user_id",omitempty`
}

// isValidEmail checks if the email format is valid
func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// RegisterHandler
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	//Basic validation
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if !isValidEmail(req.Email) {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	//calling the service
	userId, err := h.service.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if err.Error() == "user already exists" { // Simple string check for PoC
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(authResponse{UserId: userId})
}

// Login Handler
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, userId, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: token, UserId: userId})
}
