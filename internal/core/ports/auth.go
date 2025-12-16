package ports

import "context"

// AuthService defines the business logic for user authentication.
// This is the "Input Port" that your HTTP Handlers will call.
type AuthService interface {
	// Register creates a new user and returns their ID
	Register(ctx context.Context, email, password string) (string, error)

	// Login verifies credentials and returns a JWT token
	Login(ctx context.Context, email, password string) (string, error)

	// ValidateToken checks if a token is valid and returns the user ID
	ValidateToken(ctx context.Context, token string) (string, error)
}
