package database

import (
	"context"
	"fmt"
	"log/slog"

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
