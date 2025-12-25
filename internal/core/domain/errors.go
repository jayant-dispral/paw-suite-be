package domain

import "errors"

// Sentinel Errors
// The Service layer returns these. The Handler layer checks for them.
var (
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource already exists")
	ErrInternal           = errors.New("internal server error")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized access")
)
