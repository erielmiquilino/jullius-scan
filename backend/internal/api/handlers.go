package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/erielfranco/jullius-scan/backend/internal/api/middleware"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/domain"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
	"github.com/erielfranco/jullius-scan/backend/internal/scraper"
)

// Handlers holds the dependencies for API request handlers.
type Handlers struct {
	db    *database.DB
	queue *queue.Client
	jobs  *database.JobQueries
	rcpt  *database.ReceiptQueries
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db *database.DB, q *queue.Client) *Handlers {
	return &Handlers{
		db:    db,
		queue: q,
		jobs:  database.NewJobQueries(db),
		rcpt:  database.NewReceiptQueries(db),
	}
}

// --- Request / Response types ---

// SubmitReceiptRequest is the request body for POST /receipts.
type SubmitReceiptRequest struct {
	FiscalURL string `json:"fiscal_url"`
}

// SubmitReceiptResponse is the response for POST /receipts.
type SubmitReceiptResponse struct {
	JobID     int64            `json:"job_id"`
	Status    domain.JobStatus `json:"status"`
	FiscalURL string           `json:"fiscal_url"`
	ReceiptID *int64           `json:"receipt_id,omitempty"`
	Message   string           `json:"message,omitempty"`
}

// JobResponse is the response for GET /jobs/{id}.
type JobResponse struct {
	ID               int64                 `json:"id"`
	HouseID          int64                 `json:"house_id"`
	FiscalURL        string                `json:"fiscal_url"`
	Status           domain.JobStatus      `json:"status"`
	Attempts         int                   `json:"attempts"`
	FailureReason    *domain.FailureReason `json:"failure_reason,omitempty"`
	ErrorDetail      string                `json:"error_detail,omitempty"`
	ReceiptID        *int64                `json:"receipt_id,omitempty"`
	CreatedAt        string                `json:"created_at"`
	StartedAt        *string               `json:"started_at,omitempty"`
	CompletedAt      *string               `json:"completed_at,omitempty"`
	CaptchaPendingAt *string               `json:"captcha_pending_at,omitempty"`
}

// CaptchaContextResponse is the response for GET /jobs/{id}/captcha.
type CaptchaContextResponse struct {
	SefazURL  string `json:"sefaz_url"`
	UserAgent string `json:"user_agent"`
}

// CaptchaResumeRequest is the request body for POST /jobs/{id}/captcha/resume.
type CaptchaResumeRequest struct {
	Cookies   json.RawMessage `json:"cookies"`
	UserAgent string          `json:"user_agent,omitempty"`
}

// SessionCookie represents a browser cookie for serialisation.
type SessionCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"http_only"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"same_site,omitempty"`
}

// ReceiptResponse is the response for GET /receipts/{id}.
type ReceiptResponse struct {
	ID          int64                `json:"id"`
	HouseID     int64                `json:"house_id"`
	FiscalKey   string               `json:"fiscal_key"`
	FiscalURL   string               `json:"fiscal_url"`
	IssuedAt    string               `json:"issued_at"`
	TotalAmount float64              `json:"total_amount"`
	Store       *StoreResponse       `json:"store,omitempty"`
	Items       []ItemResponse       `json:"items,omitempty"`
	SubmittedBy *SubmittedByResponse `json:"submitted_by,omitempty"`
	CreatedAt   string               `json:"created_at"`
}

// SubmittedByResponse identifies the House member who originated a receipt.
// Populated only on the receipt detail endpoint when the receipt has a known
// submitter; omitted from listing responses by design.
type SubmittedByResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// StoreResponse is the store data in a receipt response.
type StoreResponse struct {
	ID      int64  `json:"id"`
	CNPJ    string `json:"cnpj"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

// ItemResponse is a line item in a receipt response.
type ItemResponse struct {
	ID          int64   `json:"id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	Barcode     *string `json:"barcode,omitempty"`
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// --- Handlers ---

// SubmitReceipt handles POST /api/v1/receipts.
// Creates an idempotent scraping job scoped to the authenticated user's House.
func (h *Handlers) SubmitReceipt(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}
	userID, _ := middleware.GetUserID(r.Context())

	// Parse request body
	var req SubmitReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	// Validate fiscal URL
	if err := validateFiscalURL(req.FiscalURL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error(), "INVALID_URL")
		return
	}

	// Idempotency check 1: completed receipt already exists for this URL in this House
	existingReceipt, err := h.jobs.FindCompletedReceiptByURL(r.Context(), houseID, req.FiscalURL)
	if err == nil && existingReceipt != nil {
		// Return the existing receipt's associated job
		existingJob, jobErr := h.jobs.FindJobByReceiptID(r.Context(), existingReceipt.ID)
		if jobErr == nil && existingJob != nil {
			respondJSON(w, http.StatusOK, SubmitReceiptResponse{
				JobID:     existingJob.ID,
				Status:    existingJob.Status,
				FiscalURL: req.FiscalURL,
				ReceiptID: &existingReceipt.ID,
				Message:   "receipt already exists for this URL",
			})
			return
		}
		// Receipt exists but no job found — still return the receipt info
		respondJSON(w, http.StatusOK, SubmitReceiptResponse{
			JobID:     0,
			Status:    domain.JobStatusCompleted,
			FiscalURL: req.FiscalURL,
			ReceiptID: &existingReceipt.ID,
			Message:   "receipt already exists for this URL",
		})
		return
	}

	// Idempotency check 2: active job already in progress for this URL in this House
	activeJob, err := h.jobs.FindActiveJobByURL(r.Context(), houseID, req.FiscalURL)
	if err == nil && activeJob != nil {
		respondJSON(w, http.StatusOK, SubmitReceiptResponse{
			JobID:     activeJob.ID,
			Status:    activeJob.Status,
			FiscalURL: req.FiscalURL,
			Message:   "scraping job already in progress",
		})
		return
	}

	// Create new scraping job
	job := &domain.ScrapingJob{
		HouseID:     houseID,
		SubmittedBy: userID,
		FiscalURL:   req.FiscalURL,
	}

	if err := h.jobs.CreateJob(r.Context(), job); err != nil {
		slog.Error("failed to create scraping job",
			"error", err,
			"house_id", houseID,
			"fiscal_url", req.FiscalURL,
		)
		respondError(w, http.StatusInternalServerError, "failed to create scraping job", "INTERNAL")
		return
	}

	// Enqueue to Redis
	msg := queue.JobMessage{
		JobID:     job.ID,
		FiscalURL: job.FiscalURL,
		HouseID:   job.HouseID,
		Attempt:   0,
	}
	if err := h.queue.Enqueue(r.Context(), msg); err != nil {
		slog.Error("failed to enqueue job to redis",
			"error", err,
			"job_id", job.ID,
		)
		// Job is created in DB but not enqueued — mark as failed
		reason := domain.FailureUnknown
		_ = h.jobs.UpdateJobStatus(r.Context(), job.ID, domain.JobStatusFailed, &reason, "failed to enqueue to redis", nil)
		respondError(w, http.StatusInternalServerError, "failed to enqueue scraping job", "QUEUE_ERROR")
		return
	}

	slog.Info("receipt submission accepted",
		"job_id", job.ID,
		"house_id", houseID,
		"fiscal_url", req.FiscalURL,
	)

	respondJSON(w, http.StatusAccepted, SubmitReceiptResponse{
		JobID:     job.ID,
		Status:    job.Status,
		FiscalURL: job.FiscalURL,
	})
}

// ListReceipts handles GET /api/v1/receipts.
// Returns all receipts for the authenticated user's House.
func (h *Handlers) ListReceipts(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	receipts, err := h.rcpt.ListByHouse(r.Context(), houseID)
	if err != nil {
		slog.Error("failed to list receipts", "error", err, "house_id", houseID)
		respondError(w, http.StatusInternalServerError, "failed to list receipts", "INTERNAL")
		return
	}

	result := make([]ReceiptResponse, 0, len(receipts))
	for _, rc := range receipts {
		result = append(result, toReceiptResponse(&rc, nil, nil))
	}

	respondJSON(w, http.StatusOK, result)
}

// GetReceipt handles GET /api/v1/receipts/{id}.
// Returns a single receipt with store, items, and submitter (when known) for the
// authenticated user's House.
func (h *Handlers) GetReceipt(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	receiptID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id", "INVALID_ID")
		return
	}

	receipt, submitter, err := h.rcpt.GetByIDWithSubmitter(r.Context(), receiptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "receipt not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to get receipt", "error", err, "receipt_id", receiptID)
		respondError(w, http.StatusInternalServerError, "failed to get receipt", "INTERNAL")
		return
	}

	// House access enforcement
	if receipt.HouseID != houseID {
		respondError(w, http.StatusForbidden, "access denied", "ACCESS_DENIED")
		return
	}

	store, _ := h.rcpt.GetStoreByID(r.Context(), receipt.StoreID)
	items, _ := h.rcpt.GetItemsByReceiptID(r.Context(), receiptID)

	respondJSON(w, http.StatusOK, toReceiptDetailResponse(receipt, store, items, submitter))
}

// DeleteReceipt handles DELETE /api/v1/receipts/{id}.
// Removes a single receipt for the authenticated user's House.
func (h *Handlers) DeleteReceipt(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	receiptID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id", "INVALID_ID")
		return
	}

	receipt, err := h.rcpt.GetByID(r.Context(), receiptID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "receipt not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to get receipt before delete", "error", err, "receipt_id", receiptID)
		respondError(w, http.StatusInternalServerError, "failed to get receipt", "INTERNAL")
		return
	}

	if receipt.HouseID != houseID {
		respondError(w, http.StatusForbidden, "access denied", "ACCESS_DENIED")
		return
	}

	if err := h.rcpt.DeleteByIDAndHouse(r.Context(), receiptID, houseID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receipt not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to delete receipt", "error", err, "receipt_id", receiptID, "house_id", houseID)
		respondError(w, http.StatusInternalServerError, "failed to delete receipt", "INTERNAL")
		return
	}

	respondNoContent(w)
}

// GetJob handles GET /api/v1/jobs/{id}.
// Returns a scraping job status for the authenticated user's House.
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	jobID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job id", "INVALID_ID")
		return
	}

	job, err := h.jobs.GetByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "job not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to get job", "error", err, "job_id", jobID)
		respondError(w, http.StatusInternalServerError, "failed to get job", "INTERNAL")
		return
	}

	// House access enforcement
	if job.HouseID != houseID {
		respondError(w, http.StatusForbidden, "access denied", "ACCESS_DENIED")
		return
	}

	respondJSON(w, http.StatusOK, toJobResponse(job))
}

// GetCaptchaContext handles GET /api/v1/jobs/{id}/captcha.
// Returns the SEFAZ URL and user-agent needed to open the captcha WebView in the mobile app.
func (h *Handlers) GetCaptchaContext(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	jobID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job id", "INVALID_ID")
		return
	}

	job, err := h.jobs.GetByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "job not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to get job for captcha context", "error", err, "job_id", jobID)
		respondError(w, http.StatusInternalServerError, "failed to get job", "INTERNAL")
		return
	}

	if job.HouseID != houseID {
		respondError(w, http.StatusForbidden, "access denied", "ACCESS_DENIED")
		return
	}

	if job.Status != domain.JobStatusAwaitingCaptcha {
		respondError(w, http.StatusConflict, "job is not awaiting captcha resolution", "NOT_AWAITING_CAPTCHA")
		return
	}

	sefazURL := ""
	if job.CaptchaCurrentURL != nil {
		sefazURL = *job.CaptchaCurrentURL
	}
	if sefazURL == "" {
		sefazURL = job.FiscalURL
	}

	respondJSON(w, http.StatusOK, CaptchaContextResponse{
		SefazURL:  sefazURL,
		UserAgent: scraper.UserAgent,
	})
}

// ResumeCaptcha handles POST /api/v1/jobs/{id}/captcha/resume.
// Accepts validated session cookies from the mobile app and re-enqueues the job.
func (h *Handlers) ResumeCaptcha(w http.ResponseWriter, r *http.Request) {
	houseID, ok := middleware.GetHouseID(r.Context())
	if !ok {
		respondError(w, http.StatusForbidden, "no house context", "NO_HOUSE")
		return
	}

	jobID, err := parseIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job id", "INVALID_ID")
		return
	}

	job, err := h.jobs.GetByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "job not found", "NOT_FOUND")
			return
		}
		slog.Error("failed to get job for captcha resume", "error", err, "job_id", jobID)
		respondError(w, http.StatusInternalServerError, "failed to get job", "INTERNAL")
		return
	}

	if job.HouseID != houseID {
		respondError(w, http.StatusForbidden, "access denied", "ACCESS_DENIED")
		return
	}

	if job.Status != domain.JobStatusAwaitingCaptcha {
		respondError(w, http.StatusConflict, "job is not awaiting captcha resolution", "NOT_AWAITING_CAPTCHA")
		return
	}

	var req CaptchaResumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if err := h.jobs.ResumeJobFromCaptcha(r.Context(), jobID, req.Cookies, req.UserAgent); err != nil {
		slog.Error("failed to resume job from captcha", "error", err, "job_id", jobID)
		respondError(w, http.StatusInternalServerError, "failed to resume job", "INTERNAL")
		return
	}

	resumeMsg := queue.JobMessage{
		JobID:     job.ID,
		FiscalURL: job.FiscalURL,
		HouseID:   job.HouseID,
		Attempt:   job.Attempts,
	}
	if err := h.queue.EnqueueResume(r.Context(), resumeMsg); err != nil {
		slog.Error("failed to enqueue captcha resume", "error", err, "job_id", jobID)
		respondError(w, http.StatusInternalServerError, "failed to enqueue resume", "QUEUE_ERROR")
		return
	}

	slog.Info("captcha resume accepted", "job_id", jobID, "house_id", houseID)
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

// --- Validation ---

// validateFiscalURL validates that a fiscal URL is a well-formed HTTP(S) URL
// pointing to a known SEFAZ domain pattern.
func validateFiscalURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("fiscal_url is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("fiscal_url is not a valid URL")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("fiscal_url must use http or https scheme")
	}

	if parsed.Host == "" {
		return errors.New("fiscal_url must have a valid host")
	}

	// Accept any URL in MVP — SEFAZ domains vary by state.
	// Future: add allowlist of known SEFAZ hosts.
	return nil
}

// --- Response helpers ---

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message, code string) {
	respondJSON(w, status, ErrorResponse{Error: message, Code: code})
}

func respondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func parseIDParam(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
	return strconv.ParseInt(raw, 10, 64)
}

func toJobResponse(j *domain.ScrapingJob) JobResponse {
	resp := JobResponse{
		ID:            j.ID,
		HouseID:       j.HouseID,
		FiscalURL:     j.FiscalURL,
		Status:        j.Status,
		Attempts:      j.Attempts,
		FailureReason: j.FailureReason,
		ErrorDetail:   j.ErrorDetail,
		ReceiptID:     j.ReceiptID,
		CreatedAt:     j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if j.StartedAt != nil {
		s := j.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.StartedAt = &s
	}
	if j.CompletedAt != nil {
		s := j.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CompletedAt = &s
	}
	if j.CaptchaPendingAt != nil {
		s := j.CaptchaPendingAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CaptchaPendingAt = &s
	}
	return resp
}

func toReceiptResponse(rc *domain.Receipt, store *domain.Store, items []domain.Item) ReceiptResponse {
	resp := ReceiptResponse{
		ID:          rc.ID,
		HouseID:     rc.HouseID,
		FiscalKey:   rc.FiscalKey,
		FiscalURL:   rc.FiscalURL,
		IssuedAt:    rc.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
		TotalAmount: rc.TotalAmount,
		CreatedAt:   rc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if store != nil {
		resp.Store = &StoreResponse{
			ID:      store.ID,
			CNPJ:    store.CNPJ,
			Name:    store.Name,
			Address: store.Address,
		}
	}
	if items != nil {
		resp.Items = make([]ItemResponse, 0, len(items))
		for _, it := range items {
			resp.Items = append(resp.Items, ItemResponse{
				ID:          it.ID,
				Description: it.Description,
				Quantity:    it.Quantity,
				Unit:        it.Unit,
				UnitPrice:   it.UnitPrice,
				TotalPrice:  it.TotalPrice,
				Barcode:     it.Barcode,
			})
		}
	}
	return resp
}

// toReceiptDetailResponse adds the submitter to the base receipt response.
// Used by GET /api/v1/receipts/{id} only; the listing endpoint keeps the
// submitter-less shape via toReceiptResponse.
func toReceiptDetailResponse(rc *domain.Receipt, store *domain.Store, items []domain.Item, submitter *domain.User) ReceiptResponse {
	resp := toReceiptResponse(rc, store, items)
	if submitter != nil {
		resp.SubmittedBy = &SubmittedByResponse{
			ID:    submitter.ID,
			Name:  submitter.Name,
			Email: submitter.Email,
		}
	}
	return resp
}
