package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter initializes the main Chi router and mounts all sub-routers
func NewRouter(authHandler *AuthHandler) *chi.Mux {
	r := chi.NewRouter()

	// 1. Global Middleware (Applied to ALL requests)
	r.Use(middleware.RequestID) // Generates unique ID for every request (tracing)
	r.Use(middleware.RealIP)    // Gets true IP (useful behind Nginx/Cloudflare)
	r.Use(middleware.Logger)    // Logs request details
	r.Use(middleware.Recoverer) // Prevents server crash on panic
	r.Use(middleware.Timeout(60 * time.Second))

	// Basic CORS Setup (Crucial for Frontend integration)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Change to your frontend URL in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 2. Base Health Check
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Sentinel API is running 🛡️"))
	})

	// 3. Mount API Versions
	r.Route("/api/v1", func(r chi.Router) {
		// Mount the Auth sub-router
		r.Mount("/auth", authRoutes(authHandler))

		// Future: r.Mount("/users", userRoutes(userHandler))
		// Future: r.Mount("/payments", paymentRoutes(paymentHandler))
	})

	return r
}

// authRoutes defines the sub-routes for Authentication
// This keeps the main router clean.
func authRoutes(h *AuthHandler) http.Handler {
	r := chi.NewRouter()

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)

	// Example of grouped middleware specific to Auth
	// r.Group(func(r chi.Router) {
	//     r.Use(MyCustomAuthMiddleware)
	//     r.Post("/refresh", h.RefreshToken)
	// })

	return r
}
