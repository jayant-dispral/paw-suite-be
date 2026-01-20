package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// NewRouter initializes the main Chi router and mounts all sub-routers
func NewRouter(authHandler *AuthHandler, userHandler *UserHandler, projectHanlder *ProjectHandler) *chi.Mux {
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

	r.Use(SecureHeaders)
	r.Use(LimitBodySize(1024 * 1024))
	// 2. Base Health Check
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Sentinel API is running 🛡️"))
	})

	// 3. Mount API Versions
	r.Route("/api/v1", func(r chi.Router) {
		// Mount the Auth sub-router
		r.Mount("/auth", authRoutes(authHandler))

		r.Mount("/users", userRoutes(userHandler, &authHandler.service))

		r.Mount("/projects", projectRoutes(projectHanlder, &authHandler.service))
	})

	return r
}

// authRoutes defines the sub-routes for Authentication
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

// userRoutes defienes the user routes
func userRoutes(h *UserHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()

	//auth middleware
	r.Use(AuthMiddleware(*AuthSVC))
	r.Get("/me", h.GetMe)
	r.Post("/me", h.UpdateUser)
	return r
}

// projectRoutes defines the project routes
func projectRoutes(h *ProjectHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()

	//auth middleware
	r.Use(AuthMiddleware(*AuthSVC))
	r.Post("/", h.CreateProject)
	r.Get("/", h.GetMyProjects)
	r.Get("/{projectID}", h.GetProjectDetails)
	r.Put("/{projectID}", h.UpdateProjectDetails)
	r.Delete("/{projectID}", h.DeleteProject)
	r.Put("/{projectID}/status", h.UpdateProjectStatus)
	r.Put("/{projectID}/monitoring", h.UpdateMonitoringConfig)
	r.Put("/{projectID}/alerts", h.UpdateAlertConfig)
	r.Post("/{projectID}/team", h.AddTeamMember)
	r.Put("/{projectID}/team/{userId}", h.UpdateTeamMemberRole)
	r.Delete("/{projectID}/team/{userId}", h.RemoveTeamMember)
	r.Get("/{projectID}/team", h.GetTeamMembers)
	return r
}
