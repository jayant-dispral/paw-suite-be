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
func NewRouter(authHandler *AuthHandler, userHandler *UserHandler, projectHandler *ProjectHandler, socialPostHandler *SocialPostHandler, threatHandler *ThreatHandler) *chi.Mux {
	r := chi.NewRouter()

	// 1. Global Middleware (Applied to ALL requests)
	r.Use(middleware.RequestID) // Generates unique ID for every request (tracing)
	r.Use(middleware.RealIP)    // Gets true IP (useful behind Nginx/Cloudflare)
	r.Use(middleware.Logger)    // Logs request details
	r.Use(middleware.Recoverer) // Prevents server crash on panic
	r.Use(middleware.Timeout(60 * time.Second))

	// Basic CORS Setup (Crucial for Frontend integration)
	r.Use(cors.Handler(cors.Options{
		// Explicit dev origins avoid browsers rejecting wildcard when credentials are allowed
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:3000", "http://127.0.0.1:3000"},
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

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// 3. Mount API Versions
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/auth", authRoutes(authHandler))
		r.Mount("/users", userRoutes(userHandler, &authHandler.service))
		r.Mount("/projects", projectRoutes(projectHandler, &authHandler.service))
		r.Mount("/projects/{projectID}/posts", socialPostRoutes(socialPostHandler, &authHandler.service))
		r.Mount("/projects/{projectID}/threats", threatRoutes(threatHandler, &authHandler.service))
	})

	return r
}

func authRoutes(h *AuthHandler) http.Handler {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	return r
}

func userRoutes(h *UserHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(*AuthSVC))
	r.Get("/search", h.SearchByEmail)
	r.Get("/me", h.GetMe)
	r.Post("/me", h.UpdateUser)
	return r
}

func projectRoutes(h *ProjectHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()
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

func socialPostRoutes(h *SocialPostHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(*AuthSVC))
	r.Get("/", h.GetPosts)
	r.Get("/{postID}", h.GetPost)
	r.Get("/sentiment/{sentiment}", h.GetPostsBySentiment)
	r.Get("/viral", h.GetViralPosts)
	r.Get("/keyword/{keyword}", h.GetPostsByKeyword)
	r.Get("/stats", h.GetPostStats)
	r.Post("/", h.IngestPost)
	r.Post("/bulk", h.BulkIngestPosts)
	return r
}

func threatRoutes(h *ThreatHandler, AuthSVC *ports.AuthService) http.Handler {
	r := chi.NewRouter()
	r.Use(AuthMiddleware(*AuthSVC))

	r.Get("/", h.GetThreatIntel)
	r.Post("/refresh", h.RefreshThreatIntel)
	r.Put("/{threatID}/resolve", h.ResolveThreat)
	r.Put("/{threatID}/ignore", h.IgnoreThreat)
	// FIX: WhitelistThreat and AcknowledgeThreat handlers were added to
	// http/threat.go when ThreatWhitelisted and ThreatAcknowledged were
	// added to the domain and service, but these two routes were never
	// registered here, making both endpoints unreachable (404).
	r.Put("/{threatID}/whitelist", h.WhitelistThreat)
	r.Put("/{threatID}/acknowledge", h.AcknowledgeThreat)

	return r
}
