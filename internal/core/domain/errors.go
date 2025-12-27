package domain

import "errors"

// Sentinel Errors
// The Service layer returns these. The Handler layer checks for them.
var (
	// 400 Bad Request
	ErrInvalidInput = errors.New("invalid input provided")

	// 401 Unauthorized
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token has expired")

	// 403 Forbidden
	ErrForbidden = errors.New("access forbidden")

	// 404 Not Found
	ErrNotFound = errors.New("resource not found")

	// 409 Conflict
	ErrConflict = errors.New("resource already exists")

	// 500 Internal
	ErrInternal = errors.New("internal server error")
)
