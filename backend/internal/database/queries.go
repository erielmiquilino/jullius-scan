package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/erielfranco/jullius-scan/backend/internal/domain"
)

// UserQueries provides database operations for users.
type UserQueries struct {
	db *DB
}

// NewUserQueries creates a new UserQueries instance.
func NewUserQueries(db *DB) *UserQueries {
	return &UserQueries{db: db}
}

// FindByFirebaseID retrieves a user by their Firebase UID.
func (q *UserQueries) FindByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	var u domain.User
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, firebase_id, email, name, created_at
		 FROM users
		 WHERE firebase_id = $1`,
		firebaseID,
	).Scan(&u.ID, &u.FirebaseID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user by firebase_id: %w", err)
	}
	return &u, nil
}

// HouseQueries provides database operations for houses and memberships.
type HouseQueries struct {
	db *DB
}

// NewHouseQueries creates a new HouseQueries instance.
func NewHouseQueries(db *DB) *HouseQueries {
	return &HouseQueries{db: db}
}

// FindActiveHouseForUser retrieves the active House for a given user.
// In MVP, a user belongs to exactly one house.
func (q *HouseQueries) FindActiveHouseForUser(ctx context.Context, userID int64) (*domain.House, error) {
	var h domain.House
	err := q.db.Pool.QueryRow(ctx,
		`SELECT h.id, h.name, h.created_at
		 FROM houses h
		 INNER JOIN house_members hm ON hm.house_id = h.id
		 WHERE hm.user_id = $1
		 LIMIT 1`,
		userID,
	).Scan(&h.ID, &h.Name, &h.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find active house for user: %w", err)
	}
	return &h, nil
}

// IsUserMemberOfHouse checks whether a user belongs to a specific house.
func (q *HouseQueries) IsUserMemberOfHouse(ctx context.Context, userID, houseID int64) (bool, error) {
	var exists bool
	err := q.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM house_members
			WHERE user_id = $1 AND house_id = $2
		)`,
		userID, houseID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check house membership: %w", err)
	}
	return exists, nil
}

// JobQueries provides database operations for scraping jobs.
type JobQueries struct {
	db *DB
}

// NewJobQueries creates a new JobQueries instance.
func NewJobQueries(db *DB) *JobQueries {
	return &JobQueries{db: db}
}

// FindActiveJobByURL checks if there is an active (queued/processing/awaiting_captcha) job for a fiscal URL in a house.
func (q *JobQueries) FindActiveJobByURL(ctx context.Context, houseID int64, fiscalURL string) (*domain.ScrapingJob, error) {
	var j domain.ScrapingJob
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, house_id, submitted_by, fiscal_url, status, attempts,
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at,
		        captcha_current_url, captcha_session_cookies, captcha_pending_at, captcha_resumed_at, captcha_retry_count, captcha_user_agent,
		        captcha_phase, parsed_summary, captcha_resume_page_html
		 FROM scraping_jobs
		 WHERE house_id = $1 AND fiscal_url = $2 AND status IN ('queued', 'processing', 'awaiting_captcha')
		 LIMIT 1`,
		houseID, fiscalURL,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt,
		&j.CaptchaCurrentURL, &j.CaptchaSessionCookies, &j.CaptchaPendingAt, &j.CaptchaResumedAt, &j.CaptchaRetryCount, &j.CaptchaUserAgent,
		&j.CaptchaPhase, &j.ParsedSummary, &j.CaptchaResumePageHTML)
	if err != nil {
		return nil, fmt.Errorf("find active job by url: %w", err)
	}
	return &j, nil
}

// FindCompletedReceiptByURL checks if a receipt already exists for this fiscal URL in a house.
func (q *JobQueries) FindCompletedReceiptByURL(ctx context.Context, houseID int64, fiscalURL string) (*domain.Receipt, error) {
	var r domain.Receipt
	err := q.db.Pool.QueryRow(ctx,
		`SELECT r.id, r.house_id, r.store_id, r.fiscal_key, r.fiscal_url,
		        r.issued_at, r.total_amount, r.created_at
		 FROM receipts r
		 WHERE r.house_id = $1 AND r.fiscal_url = $2
		 LIMIT 1`,
		houseID, fiscalURL,
	).Scan(&r.ID, &r.HouseID, &r.StoreID, &r.FiscalKey, &r.FiscalURL,
		&r.IssuedAt, &r.TotalAmount, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find completed receipt by url: %w", err)
	}
	return &r, nil
}

// FindJobByReceiptID finds the job that produced a given receipt.
func (q *JobQueries) FindJobByReceiptID(ctx context.Context, receiptID int64) (*domain.ScrapingJob, error) {
	var j domain.ScrapingJob
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, house_id, submitted_by, fiscal_url, status, attempts,
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at,
		        captcha_current_url, captcha_session_cookies, captcha_pending_at, captcha_resumed_at, captcha_retry_count, captcha_user_agent,
		        captcha_phase, parsed_summary, captcha_resume_page_html
		 FROM scraping_jobs
		 WHERE receipt_id = $1
		 LIMIT 1`,
		receiptID,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt,
		&j.CaptchaCurrentURL, &j.CaptchaSessionCookies, &j.CaptchaPendingAt, &j.CaptchaResumedAt, &j.CaptchaRetryCount, &j.CaptchaUserAgent,
		&j.CaptchaPhase, &j.ParsedSummary, &j.CaptchaResumePageHTML)
	if err != nil {
		return nil, fmt.Errorf("find job by receipt_id: %w", err)
	}
	return &j, nil
}

// GetByID retrieves a scraping job by its ID.
func (q *JobQueries) GetByID(ctx context.Context, jobID int64) (*domain.ScrapingJob, error) {
	var j domain.ScrapingJob
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, house_id, submitted_by, fiscal_url, status, attempts,
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at,
		        captcha_current_url, captcha_session_cookies, captcha_pending_at, captcha_resumed_at, captcha_retry_count, captcha_user_agent,
		        captcha_phase, parsed_summary, captcha_resume_page_html
		 FROM scraping_jobs
		 WHERE id = $1`,
		jobID,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt,
		&j.CaptchaCurrentURL, &j.CaptchaSessionCookies, &j.CaptchaPendingAt, &j.CaptchaResumedAt, &j.CaptchaRetryCount, &j.CaptchaUserAgent,
		&j.CaptchaPhase, &j.ParsedSummary, &j.CaptchaResumePageHTML)
	if err != nil {
		return nil, fmt.Errorf("get job by id: %w", err)
	}
	return &j, nil
}

// PauseJobForCaptcha atomically transitions a job from processing to awaiting_captcha,
// persisting the current URL, browser session cookies, captcha phase, and optionally
// the already-parsed summary payload (used when pausing during the detail page transition).
func (q *JobQueries) PauseJobForCaptcha(ctx context.Context, jobID int64, currentURL string, cookies json.RawMessage, phase domain.CaptchaPhase, parsedSummary json.RawMessage) error {
	var summaryArg interface{}
	if len(parsedSummary) > 0 {
		summaryArg = parsedSummary
	}
	_, err := q.db.Pool.Exec(ctx,
		`UPDATE scraping_jobs
		 SET status                  = 'awaiting_captcha',
		     captcha_current_url     = $2,
		     captcha_session_cookies = $3,
		     captcha_phase           = $4,
		     parsed_summary          = COALESCE($5, parsed_summary),
		     captcha_resume_page_html = NULL,
		     captcha_pending_at      = NOW()
		 WHERE id = $1 AND status = 'processing'`,
		jobID, currentURL, cookies, string(phase), summaryArg,
	)
	if err != nil {
		return fmt.Errorf("pause job for captcha: %w", err)
	}
	slog.Info("job paused for captcha", "job_id", jobID, "captcha_url", currentURL, "phase", phase)
	return nil
}

// DeleteByIDAndHouse removes a receipt owned by the given house inside a transaction.
// Associated items are removed by ON DELETE CASCADE and scraping job references are
// cleared by the receipt_id foreign key ON DELETE SET NULL.
func (q *ReceiptQueries) DeleteByIDAndHouse(ctx context.Context, receiptID, houseID int64) error {
	tx, err := q.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete receipt transaction: %w", err)
	}
	defer func() {
		// Ignore rollback error; commit makes this a no-op on the success path.
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(ctx,
		`DELETE FROM receipts
		 WHERE id = $1 AND house_id = $2`,
		receiptID, houseID,
	)
	if err != nil {
		return fmt.Errorf("delete receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete receipt transaction: %w", err)
	}

	slog.Info("receipt deleted", "receipt_id", receiptID, "house_id", houseID)
	return nil
}

// ResumeJobFromCaptcha validates the job is in awaiting_captcha, updates the
// cookies, user agent, optional current URL, optional rendered page HTML,
// increments retry count, and transitions back to processing for re-dispatch.
func (q *JobQueries) ResumeJobFromCaptcha(ctx context.Context, jobID int64, cookies json.RawMessage, userAgent, currentURL, pageHTML string) error {
	var uaArg interface{}
	if userAgent != "" {
		uaArg = userAgent
	}
	var urlArg interface{}
	if currentURL != "" {
		urlArg = currentURL
	}
	var htmlArg interface{}
	if pageHTML != "" {
		htmlArg = pageHTML
	}
	tag, err := q.db.Pool.Exec(ctx,
		`UPDATE scraping_jobs
		 SET status                  = 'processing',
		     captcha_session_cookies = $2,
		     captcha_user_agent      = COALESCE($3, captcha_user_agent),
		     captcha_current_url     = COALESCE($4, captcha_current_url),
		     captcha_resume_page_html = $5,
		     captcha_resumed_at      = NOW(),
		     captcha_retry_count     = captcha_retry_count + 1
		 WHERE id = $1 AND status = 'awaiting_captcha'`,
		jobID, cookies, uaArg, urlArg, htmlArg,
	)
	if err != nil {
		return fmt.Errorf("resume job from captcha: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resume job from captcha: job %d is not in awaiting_captcha state", jobID)
	}
	slog.Info("job resumed from captcha",
		"job_id", jobID,
		"has_user_agent", userAgent != "",
		"has_current_url", currentURL != "",
		"has_page_html", pageHTML != "",
		"page_html_len", len(pageHTML),
	)
	return nil
}

// GetCaptchaContext returns the persisted SEFAZ URL needed to show the WebView to the user.
func (q *JobQueries) GetCaptchaContext(ctx context.Context, jobID int64) (captchaURL string, retryCount int, err error) {
	err = q.db.Pool.QueryRow(ctx,
		`SELECT captcha_current_url, captcha_retry_count
		 FROM scraping_jobs
		 WHERE id = $1 AND status = 'awaiting_captcha'`,
		jobID,
	).Scan(&captchaURL, &retryCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", 0, fmt.Errorf("get captcha context: job %d is not in awaiting_captcha state", jobID)
		}
		return "", 0, fmt.Errorf("get captcha context: %w", err)
	}
	return captchaURL, retryCount, nil
}

// ExpireAwaitingCaptchaJobs marks as failed any jobs that have been in awaiting_captcha
// longer than the given duration.
func (q *JobQueries) ExpireAwaitingCaptchaJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	tag, err := q.db.Pool.Exec(ctx,
		`UPDATE scraping_jobs
		 SET status         = 'failed',
		     failure_reason = 'captcha_timeout',
		     error_detail   = 'captcha resolution not submitted within timeout',
		     completed_at   = NOW()
		 WHERE status = 'awaiting_captcha' AND captcha_pending_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("expire awaiting captcha jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CreateJob inserts a new scraping job.
func (q *JobQueries) CreateJob(ctx context.Context, job *domain.ScrapingJob) error {
	err := q.db.Pool.QueryRow(ctx,
		`INSERT INTO scraping_jobs (house_id, submitted_by, fiscal_url, status, attempts, created_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 RETURNING id, created_at`,
		job.HouseID, job.SubmittedBy, job.FiscalURL, domain.JobStatusQueued, 0,
	).Scan(&job.ID, &job.CreatedAt)
	if err != nil {
		return fmt.Errorf("create scraping job: %w", err)
	}
	job.Status = domain.JobStatusQueued
	slog.Info("scraping job created", "job_id", job.ID, "house_id", job.HouseID, "fiscal_url", job.FiscalURL)
	return nil
}

// UpdateJobStatus updates the status and related fields of a scraping job.
func (q *JobQueries) UpdateJobStatus(ctx context.Context, jobID int64, status domain.JobStatus, failureReason *domain.FailureReason, errorDetail string, receiptID *int64) error {
	statusText := string(status)
	_, err := q.db.Pool.Exec(ctx,
		`UPDATE scraping_jobs
		 SET status = $2,
		     failure_reason = $3,
		     error_detail = $4,
		     receipt_id = $5,
		     captcha_resume_page_html = CASE WHEN $6 IN ('completed', 'failed') THEN NULL ELSE captcha_resume_page_html END,
		     started_at = CASE WHEN $6 = 'processing' AND started_at IS NULL THEN NOW() ELSE started_at END,
		     completed_at = CASE WHEN $6 IN ('completed', 'failed') THEN NOW() ELSE completed_at END,
		     attempts = CASE WHEN $6 = 'processing' THEN attempts + 1 ELSE attempts END
		 WHERE id = $1`,
		jobID, status, failureReason, errorDetail, receiptID, statusText,
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

// ReceiptQueries provides database operations for receipts, stores, and items.
type ReceiptQueries struct {
	db *DB
}

// NewReceiptQueries creates a new ReceiptQueries instance.
func NewReceiptQueries(db *DB) *ReceiptQueries {
	return &ReceiptQueries{db: db}
}

// ListByHouse returns all receipts belonging to a house, ordered by creation date desc.
func (q *ReceiptQueries) ListByHouse(ctx context.Context, houseID int64) ([]domain.Receipt, error) {
	rows, err := q.db.Pool.Query(ctx,
		`SELECT id, house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount, created_at
		 FROM receipts
		 WHERE house_id = $1
		 ORDER BY created_at DESC`,
		houseID,
	)
	if err != nil {
		return nil, fmt.Errorf("list receipts by house: %w", err)
	}
	defer rows.Close()

	var receipts []domain.Receipt
	for rows.Next() {
		var r domain.Receipt
		if err := rows.Scan(&r.ID, &r.HouseID, &r.StoreID, &r.FiscalKey, &r.FiscalURL,
			&r.IssuedAt, &r.TotalAmount, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan receipt row: %w", err)
		}
		receipts = append(receipts, r)
	}
	return receipts, rows.Err()
}

// GetByID returns a single receipt by its ID.
func (q *ReceiptQueries) GetByID(ctx context.Context, receiptID int64) (*domain.Receipt, error) {
	var r domain.Receipt
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount, created_by, created_at
		 FROM receipts
		 WHERE id = $1`,
		receiptID,
	).Scan(&r.ID, &r.HouseID, &r.StoreID, &r.FiscalKey, &r.FiscalURL,
		&r.IssuedAt, &r.TotalAmount, &r.CreatedBy, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get receipt by id: %w", err)
	}
	return &r, nil
}

// GetByIDWithSubmitter returns a receipt joined with its submitter (when known).
// The second return value is nil when the receipt has no recorded created_by.
func (q *ReceiptQueries) GetByIDWithSubmitter(ctx context.Context, receiptID int64) (*domain.Receipt, *domain.User, error) {
	var r domain.Receipt
	var (
		uID    *int64
		uFID   *string
		uEmail *string
		uName  *string
	)
	err := q.db.Pool.QueryRow(ctx,
		`SELECT r.id, r.house_id, r.store_id, r.fiscal_key, r.fiscal_url,
		        r.issued_at, r.total_amount, r.created_by, r.created_at,
		        u.id, u.firebase_id, u.email, u.name
		 FROM receipts r
		 LEFT JOIN users u ON u.id = r.created_by
		 WHERE r.id = $1`,
		receiptID,
	).Scan(&r.ID, &r.HouseID, &r.StoreID, &r.FiscalKey, &r.FiscalURL,
		&r.IssuedAt, &r.TotalAmount, &r.CreatedBy, &r.CreatedAt,
		&uID, &uFID, &uEmail, &uName)
	if err != nil {
		return nil, nil, fmt.Errorf("get receipt with submitter by id: %w", err)
	}
	if uID == nil {
		return &r, nil, nil
	}
	return &r, &domain.User{
		ID:         *uID,
		FirebaseID: derefString(uFID),
		Email:      derefString(uEmail),
		Name:       derefString(uName),
	}, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetStoreByID returns a store by its ID.
func (q *ReceiptQueries) GetStoreByID(ctx context.Context, storeID int64) (*domain.Store, error) {
	var s domain.Store
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, cnpj, name, address, created_at
		 FROM stores
		 WHERE id = $1`,
		storeID,
	).Scan(&s.ID, &s.CNPJ, &s.Name, &s.Address, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get store by id: %w", err)
	}
	return &s, nil
}

// GetItemsByReceiptID returns all items for a receipt.
func (q *ReceiptQueries) GetItemsByReceiptID(ctx context.Context, receiptID int64) ([]domain.Item, error) {
	rows, err := q.db.Pool.Query(ctx,
		`SELECT id, receipt_id, description, quantity, unit, unit_price, total_price, barcode
		 FROM items
		 WHERE receipt_id = $1
		 ORDER BY id`,
		receiptID,
	)
	if err != nil {
		return nil, fmt.Errorf("get items by receipt_id: %w", err)
	}
	defer rows.Close()

	var items []domain.Item
	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.ID, &it.ReceiptID, &it.Description, &it.Quantity,
			&it.Unit, &it.UnitPrice, &it.TotalPrice, &it.Barcode); err != nil {
			return nil, fmt.Errorf("scan item row: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertStore inserts a store or returns the existing one by CNPJ.
func (q *ReceiptQueries) UpsertStore(ctx context.Context, store *domain.Store) error {
	err := q.db.Pool.QueryRow(ctx,
		`INSERT INTO stores (cnpj, name, address, created_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (cnpj) DO UPDATE SET name = EXCLUDED.name, address = EXCLUDED.address
		 RETURNING id, created_at`,
		store.CNPJ, store.Name, store.Address,
	).Scan(&store.ID, &store.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert store: %w", err)
	}
	return nil
}

// CreateReceipt inserts a new receipt. The CreatedBy field, when non-nil,
// records which House member submitted the originating job.
func (q *ReceiptQueries) CreateReceipt(ctx context.Context, receipt *domain.Receipt) error {
	err := q.db.Pool.QueryRow(ctx,
		`INSERT INTO receipts (house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 RETURNING id, created_at`,
		receipt.HouseID, receipt.StoreID, receipt.FiscalKey, receipt.FiscalURL, receipt.IssuedAt, receipt.TotalAmount, receipt.CreatedBy,
	).Scan(&receipt.ID, &receipt.CreatedAt)
	if err != nil {
		return fmt.Errorf("create receipt: %w", err)
	}
	return nil
}

// CreateItems inserts multiple items for a receipt using a batch.
func (q *ReceiptQueries) CreateItems(ctx context.Context, items []domain.Item) error {
	if len(items) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, it := range items {
		batch.Queue(
			`INSERT INTO items (receipt_id, description, quantity, unit, unit_price, total_price, barcode)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			it.ReceiptID, it.Description, it.Quantity, it.Unit, it.UnitPrice, it.TotalPrice, it.Barcode,
		)
	}

	br := q.db.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for range items {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("create item: %w", err)
		}
	}
	return nil
}

// ItemQueries provides item-search operations spanning items, receipts, and stores.
type ItemQueries struct {
	db *DB
}

// NewItemQueries creates a new ItemQueries instance.
func NewItemQueries(db *DB) *ItemQueries {
	return &ItemQueries{db: db}
}

// ItemSearchAggregate is one grouped row in the item search result.
// A "group" is identified by barcode when present, otherwise by exact description.
type ItemSearchAggregate struct {
	Description       string
	Barcode           *string
	LastPurchasedAt   time.Time
	LastUnitPrice     float64
	LastTotalPrice    float64
	PreviousUnitPrice *float64
	AverageUnitPrice  float64
	PurchaseCount     int
	StoreID           int64
	StoreName         string
	StoreCNPJ         string
	ReceiptID         int64
}

// ItemSearchLimit is the maximum number of grouped items returned by SearchItemsByHouse.
const ItemSearchLimit = 50

// barcodeQueryPattern matches inputs that are 8 or more contiguous digits.
var barcodeQueryPattern = regexp.MustCompile(`^\d{8,}$`)

// IsBarcodeQuery reports whether a query string should be treated as an exact
// barcode lookup (8+ contiguous digits) instead of a description search.
func IsBarcodeQuery(query string) bool {
	return barcodeQueryPattern.MatchString(query)
}

// SearchItemsByHouse runs the aggregated item search for a given house.
//   - query: already trimmed; routed to barcode-exact when matching ^\d{8,}$,
//     otherwise to a substring ILIKE on description with diacritic-insensitive
//     normalization (immutable_unaccent + lower).
//   - periodDays: nil → no temporal filter; ≥0 → issued_at within last N days.
//
// Returns the aggregated list (≤ItemSearchLimit), a truncated flag (true when a
// 51st group existed and was discarded), and "barcode" or "description" so the
// caller can echo which strategy was used.
func (q *ItemQueries) SearchItemsByHouse(ctx context.Context, houseID int64, query string, periodDays *int) ([]ItemSearchAggregate, bool, string, error) {
	matchedBy := "description"
	var pattern string
	matchExpr := "immutable_unaccent(lower(items.description)) ILIKE immutable_unaccent(lower($2))"
	if IsBarcodeQuery(query) {
		matchedBy = "barcode"
		matchExpr = "items.barcode = $2"
		pattern = query
	} else {
		pattern = "%" + query + "%"
	}

	sql := `
WITH eligible AS (
    SELECT items.id, items.barcode, items.description, items.unit_price,
           items.quantity, items.total_price,
           r.id AS receipt_id, r.issued_at,
           s.id AS store_id, s.name AS store_name, s.cnpj AS store_cnpj
    FROM items
    JOIN receipts r ON r.id = items.receipt_id
    JOIN stores s   ON s.id = r.store_id
    WHERE r.house_id = $1
      AND ` + matchExpr + `
      AND ($3::int IS NULL OR r.issued_at >= NOW() - make_interval(days => $3::int))
),
ranked AS (
    SELECT eligible.*,
           COALESCE(NULLIF(barcode, ''), 'desc:' || description) AS group_key,
           ROW_NUMBER() OVER (
               PARTITION BY COALESCE(NULLIF(barcode, ''), 'desc:' || description)
               ORDER BY issued_at DESC, id DESC
           ) AS rn
    FROM eligible
),
aggregates AS (
    SELECT group_key,
           COUNT(*)::int AS purchase_count,
           (SUM(total_price) / NULLIF(SUM(quantity), 0))::float8 AS average_unit_price
    FROM ranked
    GROUP BY group_key
),
last_purchase AS (
    SELECT group_key, description, barcode,
           unit_price::float8  AS last_unit_price,
           total_price::float8 AS last_total_price,
           issued_at AS last_purchased_at,
           receipt_id, store_id, store_name, store_cnpj
    FROM ranked
    WHERE rn = 1
),
prev_purchase AS (
    SELECT group_key, unit_price::float8 AS previous_unit_price
    FROM ranked
    WHERE rn = 2
)
SELECT lp.description, lp.barcode, lp.last_purchased_at,
       lp.last_unit_price, lp.last_total_price,
       pp.previous_unit_price, ag.average_unit_price, ag.purchase_count,
       lp.store_id, lp.store_name, lp.store_cnpj, lp.receipt_id
FROM last_purchase lp
JOIN aggregates ag USING (group_key)
LEFT JOIN prev_purchase pp USING (group_key)
ORDER BY lp.last_purchased_at DESC, lp.receipt_id DESC
LIMIT 51
`

	rows, err := q.db.Pool.Query(ctx, sql, houseID, pattern, periodDays)
	if err != nil {
		return nil, false, matchedBy, fmt.Errorf("search items by house: %w", err)
	}
	defer rows.Close()

	results := make([]ItemSearchAggregate, 0, ItemSearchLimit)
	for rows.Next() {
		var a ItemSearchAggregate
		if err := rows.Scan(
			&a.Description, &a.Barcode, &a.LastPurchasedAt,
			&a.LastUnitPrice, &a.LastTotalPrice,
			&a.PreviousUnitPrice, &a.AverageUnitPrice, &a.PurchaseCount,
			&a.StoreID, &a.StoreName, &a.StoreCNPJ, &a.ReceiptID,
		); err != nil {
			return nil, false, matchedBy, fmt.Errorf("scan item search row: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, matchedBy, fmt.Errorf("iterate item search rows: %w", err)
	}

	truncated := len(results) > ItemSearchLimit
	if truncated {
		results = results[:ItemSearchLimit]
	}
	return results, truncated, matchedBy, nil
}
