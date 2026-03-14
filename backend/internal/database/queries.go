package database

import (
	"context"
	"fmt"
	"log/slog"

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

// FindActiveJobByURL checks if there is an active (queued/processing) job for a fiscal URL in a house.
func (q *JobQueries) FindActiveJobByURL(ctx context.Context, houseID int64, fiscalURL string) (*domain.ScrapingJob, error) {
	var j domain.ScrapingJob
	err := q.db.Pool.QueryRow(ctx,
		`SELECT id, house_id, submitted_by, fiscal_url, status, attempts,
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at
		 FROM scraping_jobs
		 WHERE house_id = $1 AND fiscal_url = $2 AND status IN ('queued', 'processing')
		 LIMIT 1`,
		houseID, fiscalURL,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt)
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
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at
		 FROM scraping_jobs
		 WHERE receipt_id = $1
		 LIMIT 1`,
		receiptID,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt)
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
		        failure_reason, error_detail, receipt_id, created_at, started_at, completed_at
		 FROM scraping_jobs
		 WHERE id = $1`,
		jobID,
	).Scan(&j.ID, &j.HouseID, &j.SubmittedBy, &j.FiscalURL, &j.Status, &j.Attempts,
		&j.FailureReason, &j.ErrorDetail, &j.ReceiptID, &j.CreatedAt, &j.StartedAt, &j.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("get job by id: %w", err)
	}
	return &j, nil
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
	_, err := q.db.Pool.Exec(ctx,
		`UPDATE scraping_jobs
		 SET status = $2,
		     failure_reason = $3,
		     error_detail = $4,
		     receipt_id = $5,
		     started_at = CASE WHEN $2 = 'processing' AND started_at IS NULL THEN NOW() ELSE started_at END,
		     completed_at = CASE WHEN $2 IN ('completed', 'failed') THEN NOW() ELSE completed_at END,
		     attempts = CASE WHEN $2 = 'processing' THEN attempts + 1 ELSE attempts END
		 WHERE id = $1`,
		jobID, status, failureReason, errorDetail, receiptID,
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
		`SELECT id, house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount, created_at
		 FROM receipts
		 WHERE id = $1`,
		receiptID,
	).Scan(&r.ID, &r.HouseID, &r.StoreID, &r.FiscalKey, &r.FiscalURL,
		&r.IssuedAt, &r.TotalAmount, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get receipt by id: %w", err)
	}
	return &r, nil
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
		`SELECT id, receipt_id, description, quantity, unit, unit_price, total_price
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
			&it.Unit, &it.UnitPrice, &it.TotalPrice); err != nil {
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

// CreateReceipt inserts a new receipt.
func (q *ReceiptQueries) CreateReceipt(ctx context.Context, receipt *domain.Receipt) error {
	err := q.db.Pool.QueryRow(ctx,
		`INSERT INTO receipts (house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 RETURNING id, created_at`,
		receipt.HouseID, receipt.StoreID, receipt.FiscalKey, receipt.FiscalURL, receipt.IssuedAt, receipt.TotalAmount,
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
			`INSERT INTO items (receipt_id, description, quantity, unit, unit_price, total_price)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			it.ReceiptID, it.Description, it.Quantity, it.Unit, it.UnitPrice, it.TotalPrice,
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
