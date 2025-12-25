package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
)

// starndard response ensures all the apis return the same structure
type StandardResponse struct {
	Status  string      `json:"status"` //sucess or error
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON sends a standard success response with just Data
// Example: { "status": "success", "data": { ... } }
func JSON(w http.ResponseWriter, status int, data interface{}) {
	respond(w, status, StandardResponse{
		Status: "success",
		Data:   data,
	})
}

// WithMessage sends a success response with just a Message
// Example: { "status": "success", "message": "Operation successful" }
func WithMessage(w http.ResponseWriter, status int, message string) {
	respond(w, status, StandardResponse{
		Status:  "success",
		Message: message,
	})
}

func JSONWithMessage(w http.ResponseWriter, status int, message string, data interface{}) {
	respond(w, status, StandardResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// WithError determines the status code based on the error type
func WithError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := "An unexpected error occurred"

	// Smart Error Mapping
	// This replaces "if err.Error() == ..."
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		msg = err.Error()
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		msg = err.Error()
	case errors.Is(err, domain.ErrInvalidCredentials):
		status = http.StatusUnauthorized
		msg = err.Error()
	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusForbidden
		msg = err.Error()
		// Add more custom errors here as needed
	default:
		// Use the error message directly if it doesn't match a sentinel
		msg = err.Error()
	}

	respond(w, status, StandardResponse{
		Status:  "error",
		Message: msg,
	})
}

func respond(w http.ResponseWriter, status int, payload StandardResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
