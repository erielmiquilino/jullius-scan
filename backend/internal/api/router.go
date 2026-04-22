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

	// Create handlers with all dependencies
	h := NewHandlers(db, q)

	// Authenticated API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate)
		r.Use(middleware.ResolveHouse(db))

		// Receipt endpoints
		r.Post("/receipts", h.SubmitReceipt)
		r.Get("/receipts", h.ListReceipts)
		r.Get("/receipts/{id}", h.GetReceipt)
		r.Delete("/receipts/{id}", h.DeleteReceipt)

		// Job endpoints
		r.Get("/jobs/{id}", h.GetJob)
		r.Get("/jobs/{id}/captcha", h.GetCaptchaContext)
		r.Post("/jobs/{id}/captcha/resume", h.ResumeCaptcha)
	})

	return r
}
