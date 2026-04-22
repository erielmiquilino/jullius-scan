package domain

import (
	"encoding/json"
	"time"
)

// JobStatus represents the lifecycle state of a scraping job.
type JobStatus string

const (
	JobStatusQueued          JobStatus = "queued"
	JobStatusProcessing      JobStatus = "processing"
	JobStatusCompleted       JobStatus = "completed"
	JobStatusFailed          JobStatus = "failed"
	JobStatusAwaitingCaptcha JobStatus = "awaiting_captcha"
)

// FailureReason categorizes why a scraping job failed.
type FailureReason string

const (
	FailureTimeout        FailureReason = "timeout"
	FailureCaptcha        FailureReason = "captcha"
	FailureNavigation     FailureReason = "navigation"
	FailureParsing        FailureReason = "parsing"
	FailureUnknown        FailureReason = "unknown"
	FailureCaptchaExpired FailureReason = "captcha_expired"
	FailureCaptchaTimeout FailureReason = "captcha_timeout"
)

// User represents an authenticated user linked to Firebase Auth.
type User struct {
	ID         int64     `json:"id"`
	FirebaseID string    `json:"firebase_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

// House represents a shared household that owns receipts.
type House struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// HouseMember links a user to a house.
type HouseMember struct {
	ID       int64     `json:"id"`
	UserID   int64     `json:"user_id"`
	HouseID  int64     `json:"house_id"`
	Role     string    `json:"role"` // "owner" or "member"
	JoinedAt time.Time `json:"joined_at"`
}

// Store represents a commercial establishment extracted from a receipt.
type Store struct {
	ID        int64     `json:"id"`
	CNPJ      string    `json:"cnpj"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Receipt represents a parsed NFC-e fiscal receipt.
type Receipt struct {
	ID          int64     `json:"id"`
	HouseID     int64     `json:"house_id"`
	StoreID     int64     `json:"store_id"`
	FiscalKey   string    `json:"fiscal_key"`
	FiscalURL   string    `json:"fiscal_url"`
	IssuedAt    time.Time `json:"issued_at"`
	TotalAmount float64   `json:"total_amount"`
	CreatedAt   time.Time `json:"created_at"`
}

// CaptchaPhase identifies where in the scraping pipeline a job was paused for captcha.
type CaptchaPhase string

const (
	CaptchaPhaseSummary CaptchaPhase = "summary"
	CaptchaPhaseDetail  CaptchaPhase = "detail"
)

// Item represents a line item in a receipt.
type Item struct {
	ID          int64   `json:"id"`
	ReceiptID   int64   `json:"receipt_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	TotalPrice  float64 `json:"total_price"`
	Barcode     *string `json:"barcode,omitempty"`
}

// ScrapingJob represents an asynchronous scraping task.
type ScrapingJob struct {
	ID            int64          `json:"id"`
	HouseID       int64          `json:"house_id"`
	SubmittedBy   int64          `json:"submitted_by"`
	FiscalURL     string         `json:"fiscal_url"`
	Status        JobStatus      `json:"status"`
	Attempts      int            `json:"attempts"`
	FailureReason *FailureReason `json:"failure_reason,omitempty"`
	ErrorDetail   string         `json:"error_detail,omitempty"`
	ReceiptID     *int64         `json:"receipt_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`

	// Captcha pause/resume fields — populated only when status is awaiting_captcha.
	CaptchaCurrentURL     *string          `json:"captcha_current_url,omitempty"`
	CaptchaSessionCookies *json.RawMessage `json:"captcha_session_cookies,omitempty"`
	CaptchaPendingAt      *time.Time       `json:"captcha_pending_at,omitempty"`
	CaptchaResumedAt      *time.Time       `json:"captcha_resumed_at,omitempty"`
	CaptchaRetryCount     int              `json:"captcha_retry_count,omitempty"`
	// UA sent by the mobile app when submitting captcha cookies; used by the worker
	// to replay the session with the same user-agent that Cloudflare issued the cookie for.
	CaptchaUserAgent *string `json:"captcha_user_agent,omitempty"`

	// CaptchaPhase records whether the job was paused at the summary page or the
	// detail page transition. NULL / empty means summary (legacy behavior).
	CaptchaPhase *CaptchaPhase `json:"captcha_phase,omitempty"`
	// ParsedSummary holds the already-parsed summary payload (store + receipt + items
	// without barcode) as JSON, so on a detail-phase resume the worker can skip
	// re-scraping the summary page.
	ParsedSummary *json.RawMessage `json:"parsed_summary,omitempty"`
}
