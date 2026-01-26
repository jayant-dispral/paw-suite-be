package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// 1. Global Middleware (Applied to ALL requests)
	r.Use(middleware.RequestID) // Generates unique ID for every request (tracing)
	r.Use(middleware.RealIP)    // Gets true IP (useful behind Nginx/Cloudflare)
	r.Use(middleware.Logger)    // Logs request details
	r.Use(middleware.Recoverer) // Prevents server crash on panic
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data service end point health"))
	})

	return  r
}