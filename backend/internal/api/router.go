package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/erielfranco/jullius-scan/backend/internal/api/middleware"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
)

// NewRouter creates the HTTP router with all API routes.
func NewRouter(db *database.DB, q *queue.Client, auth *middleware.FirebaseAuth) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.StructuredLogger)

	// Health check (public)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Authenticated API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Use(middleware.ResolveHouse(db))

		// Receipt/job endpoints (stubs - implemented in task group 2)
		r.Post("/receipts", notImplemented)
		r.Get("/receipts", notImplemented)
		r.Get("/receipts/{id}", notImplemented)
		r.Get("/jobs/{id}", notImplemented)
	})

	return r
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}
