package http

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/response"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// contextKey is a custom type to prevent key collisions in context
type contextKey string

const UserIDKey contextKey = "user_id"

// AuthMiddleware protects routes by validating the Authorization header
func AuthMiddleware(authSvc ports.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//extract token from the header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Printf("Auth middleware error: missing authorization header")
				response.WithError(w, domain.ErrUnauthorized)
				return
			}

			//Format check
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Printf("Auth middleware error: invalid header format")
				response.WithError(w, domain.ErrInvalidInput)
				return
			}
			token := parts[1]

			// 2. validate token (call the auth service)
			userID, err := authSvc.ValidateToken(r.Context(), token)
			if err != nil {
				log.Printf("Auth middleware error: token validation failed: %v", err)
				response.WithError(w, domain.ErrUnauthorized)
				return
			}

			//inject user id into context
			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			//pas to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
