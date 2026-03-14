package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/erielfranco/jullius-scan/backend/internal/config"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/domain"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
)

// Worker processes scraping jobs from the Redis queue.
type Worker struct {
	db       *database.DB
	queue    *queue.Client
	config   *config.Config
	executor *Executor
	jobs     *database.JobQueries
	receipts *database.ReceiptQueries
}

// NewWorker creates a new scraping worker.
func NewWorker(db *database.DB, q *queue.Client, cfg *config.Config) *Worker {
	return &Worker{
		db:       db,
		queue:    q,
		config:   cfg,
		executor: NewExecutor(cfg.ScrapeTimeout),
		jobs:     database.NewJobQueries(db),
		receipts: database.NewReceiptQueries(db),
	}
}

// Run starts the worker loop, consuming jobs until the context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("worker started, waiting for jobs",
		"timeout", w.config.ScrapeTimeout,
		"max_retries", w.config.MaxRetries,
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping due to context cancellation")
			return
		default:
		}

		msg, err := w.queue.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			slog.Error("failed to dequeue job", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if msg == nil {
			continue // no job available, loop again
		}

		slog.Info("processing job",
			"job_id", msg.JobID,
			"fiscal_url", msg.FiscalURL,
			"house_id", msg.HouseID,
			"attempt", msg.Attempt,
		)

		w.processJob(ctx, msg)
	}
}

// processJob handles a single scraping job with full lifecycle management.
// It transitions the job through: queued -> processing -> completed/failed,
// includes retry logic for transient failures, and detects captcha blocks.
func (w *Worker) processJob(ctx context.Context, msg *queue.JobMessage) {
	// Mark job as processing and increment attempt counter.
	if err := w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusProcessing, nil, "", nil); err != nil {
		slog.Error("failed to mark job as processing",
			"job_id", msg.JobID,
			"error", err,
		)
		return
	}

	// Create a timeout context for this specific job execution.
	jobCtx, jobCancel := context.WithTimeout(ctx, w.config.ScrapeTimeout)
	defer jobCancel()

	// Execute browser automation.
	result, err := w.executor.FetchPage(jobCtx, msg.FiscalURL)
	if err != nil {
		w.handleFailure(ctx, msg, err)
		return
	}

	// Check for captcha/anti-bot block — terminal failure, no retry.
	if DetectCaptcha(result) {
		reason := domain.FailureCaptcha
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			"SEFAZ page presented a captcha or anti-bot challenge (accepted MVP limitation)", nil)
		slog.Warn("job failed: captcha detected",
			"job_id", msg.JobID,
			"fiscal_url", msg.FiscalURL,
		)
		return
	}

	// Parse the extracted HTML into structured receipt data.
	parsed, err := ParseSEFAZHTML(result.HTML)
	if err != nil {
		w.handleParsingFailure(ctx, msg, err)
		return
	}

	// Persist the extracted data.
	if err := w.persistReceipt(ctx, msg, parsed); err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to persist receipt: %v", err), nil)
		slog.Error("job failed: persistence error",
			"job_id", msg.JobID,
			"error", err,
		)
		return
	}

	slog.Info("job completed successfully",
		"job_id", msg.JobID,
		"fiscal_url", msg.FiscalURL,
	)
}

// persistReceipt saves the parsed receipt data (store, receipt, items) to PostgreSQL
// and marks the job as completed.
func (w *Worker) persistReceipt(ctx context.Context, msg *queue.JobMessage, parsed *ParsedReceipt) error {
	// Upsert store (by CNPJ — avoids duplicates across receipts).
	store := &parsed.Store
	if err := w.receipts.UpsertStore(ctx, store); err != nil {
		return fmt.Errorf("upsert store: %w", err)
	}

	// Create receipt linked to the House that submitted the job.
	receipt := &domain.Receipt{
		HouseID:     msg.HouseID,
		StoreID:     store.ID,
		FiscalKey:   parsed.Receipt.FiscalKey,
		FiscalURL:   msg.FiscalURL,
		IssuedAt:    parsed.Receipt.IssuedAt,
		TotalAmount: parsed.Receipt.TotalAmount,
	}
	if err := w.receipts.CreateReceipt(ctx, receipt); err != nil {
		return fmt.Errorf("create receipt: %w", err)
	}

	// Create line items linked to the receipt.
	if len(parsed.Items) > 0 {
		for i := range parsed.Items {
			parsed.Items[i].ReceiptID = receipt.ID
		}
		if err := w.receipts.CreateItems(ctx, parsed.Items); err != nil {
			return fmt.Errorf("create items: %w", err)
		}
	}

	// Mark job as completed with reference to the receipt.
	if err := w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusCompleted, nil, "", &receipt.ID); err != nil {
		return fmt.Errorf("update job status to completed: %w", err)
	}

	slog.Info("receipt persisted",
		"job_id", msg.JobID,
		"receipt_id", receipt.ID,
		"store_cnpj", store.CNPJ,
		"total_amount", receipt.TotalAmount,
		"items_count", len(parsed.Items),
	)

	return nil
}

// handleFailure processes a browser/navigation failure and decides whether to retry.
func (w *Worker) handleFailure(ctx context.Context, msg *queue.JobMessage, err error) {
	nextAttempt := msg.Attempt + 1

	// Classify the failure. The error string from chromedp will contain
	// "context deadline exceeded" if the job-level timeout fired.
	isTimeout := isTimeoutError(err)

	// Determine failure reason.
	var reason domain.FailureReason
	if isTimeout {
		reason = domain.FailureTimeout
	} else {
		reason = domain.FailureNavigation
	}

	// If we haven't exhausted retries, re-enqueue for another attempt.
	if nextAttempt < w.config.MaxRetries {
		retryMsg := queue.JobMessage{
			JobID:     msg.JobID,
			FiscalURL: msg.FiscalURL,
			HouseID:   msg.HouseID,
			Attempt:   nextAttempt,
		}

		// Reset job to queued for the retry.
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusQueued, nil,
			fmt.Sprintf("attempt %d failed: %v — retrying", nextAttempt, err), nil)

		if enqErr := w.queue.Enqueue(ctx, retryMsg); enqErr != nil {
			// Can't re-enqueue — mark as terminal failure.
			_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
				fmt.Sprintf("attempt %d failed: %v — re-enqueue also failed: %v", nextAttempt, err, enqErr), nil)
			slog.Error("job failed: could not re-enqueue for retry",
				"job_id", msg.JobID,
				"attempt", nextAttempt,
				"original_error", err,
				"enqueue_error", enqErr,
			)
			return
		}

		slog.Warn("job attempt failed, retrying",
			"job_id", msg.JobID,
			"attempt", nextAttempt,
			"max_retries", w.config.MaxRetries,
			"error", err,
		)
		return
	}

	// Max retries exhausted — terminal failure.
	_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
		fmt.Sprintf("all %d attempts exhausted: %v", w.config.MaxRetries, err), nil)
	slog.Error("job failed: max retries exhausted",
		"job_id", msg.JobID,
		"attempts", nextAttempt,
		"max_retries", w.config.MaxRetries,
		"error", err,
	)
}

// handleParsingFailure processes a parsing error — parsing failures are not retried
// since the same HTML would produce the same error.
func (w *Worker) handleParsingFailure(ctx context.Context, msg *queue.JobMessage, err error) {
	reason := domain.FailureParsing
	_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
		fmt.Sprintf("HTML parsing failed: %v", err), nil)
	slog.Error("job failed: parsing error",
		"job_id", msg.JobID,
		"fiscal_url", msg.FiscalURL,
		"error", err,
	)
}

// isTimeoutError checks if an error is related to context timeout/deadline.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "timeout")
}
