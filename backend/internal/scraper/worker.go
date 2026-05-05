package scraper

import (
	"context"
	"encoding/json"
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
func (w *Worker) processJob(ctx context.Context, msg *queue.JobMessage) {
	if err := w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusProcessing, nil, "", nil); err != nil {
		slog.Error("failed to mark job as processing", "job_id", msg.JobID, "error", err)
		return
	}

	jobCtx, jobCancel := context.WithTimeout(ctx, w.config.ScrapeTimeout)
	defer jobCancel()

	if msg.IsResume {
		w.processResume(ctx, jobCtx, msg)
		return
	}

	// Fresh job: two-phase fetch (summary → detail in one browser session).
	twoPhase, err := w.executor.FetchWithDetailPhase(jobCtx, msg.FiscalURL, nil, "")
	if err != nil {
		w.handleFailure(ctx, msg, err)
		return
	}

	// Captcha on summary page.
	if DetectCaptcha(twoPhase.Summary) {
		w.handleCaptchaPause(ctx, msg, twoPhase.Summary, domain.CaptchaPhaseSummary, nil)
		return
	}

	parsed, err := ParseSEFAZHTML(twoPhase.Summary.HTML)
	if err != nil {
		w.handleParsingFailure(ctx, msg, err)
		return
	}

	// Captcha on detail page transition.
	if twoPhase.DetailCaptcha {
		parsedJSON, _ := json.Marshal(parsed)
		w.handleCaptchaPause(ctx, msg, &ExecutorResult{FinalURL: twoPhase.DetailURL, SessionCookies: twoPhase.DetailCookies}, domain.CaptchaPhaseDetail, parsedJSON)
		return
	}

	// Detail page available — enrich items with EAN.
	if twoPhase.Detail != nil {
		parsed.Items = enrichWithBarcodes(parsed.Items, twoPhase.Detail.HTML)
	}

	job, err := w.jobs.GetByID(ctx, msg.JobID)
	if err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to load job before persist: %v", err), nil)
		slog.Error("job failed: cannot load job for persistence", "job_id", msg.JobID, "error", err)
		return
	}

	if err := w.persistReceipt(ctx, msg, parsed, job.SubmittedBy); err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to persist receipt: %v", err), nil)
		slog.Error("job failed: persistence error", "job_id", msg.JobID, "error", err)
		return
	}

	slog.Info("job completed successfully", "job_id", msg.JobID, "fiscal_url", msg.FiscalURL)
}

// processResume handles the resume path after a captcha pause, branching on captcha_phase.
func (w *Worker) processResume(ctx context.Context, jobCtx context.Context, msg *queue.JobMessage) {
	job, err := w.jobs.GetByID(ctx, msg.JobID)
	if err != nil {
		slog.Error("resume: failed to load job", "job_id", msg.JobID, "error", err)
		return
	}

	var cookies []BrowserCookie
	if job.CaptchaSessionCookies != nil {
		if err := json.Unmarshal(*job.CaptchaSessionCookies, &cookies); err != nil {
			w.handleFailure(ctx, msg, fmt.Errorf("unmarshal stored cookies: %w", err))
			return
		}
	}

	ua := ""
	if job.CaptchaUserAgent != nil {
		ua = *job.CaptchaUserAgent
	}

	// Route by captcha phase.
	if job.CaptchaPhase != nil && *job.CaptchaPhase == domain.CaptchaPhaseDetail {
		w.resumeDetailPhase(ctx, jobCtx, msg, job, cookies, ua)
		return
	}

	// Summary phase (or legacy NULL phase): resume from fiscal URL with injected cookies.
	twoPhase, err := w.executor.FetchWithDetailPhase(jobCtx, msg.FiscalURL, cookies, ua)
	if err != nil {
		w.handleFailure(ctx, msg, err)
		return
	}

	if DetectCaptcha(twoPhase.Summary) {
		if job.CaptchaRetryCount >= 2 {
			reason := domain.FailureCaptchaExpired
			_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
				"captcha challenge persisted after human resolution; session may have expired", nil)
			slog.Warn("job failed: captcha expired on resume", "job_id", msg.JobID)
		} else {
			w.handleCaptchaPause(ctx, msg, twoPhase.Summary, domain.CaptchaPhaseSummary, nil)
		}
		return
	}

	parsed, err := ParseSEFAZHTML(twoPhase.Summary.HTML)
	if err != nil {
		w.handleParsingFailure(ctx, msg, err)
		return
	}

	if twoPhase.DetailCaptcha {
		parsedJSON, _ := json.Marshal(parsed)
		w.handleCaptchaPause(ctx, msg, &ExecutorResult{FinalURL: twoPhase.DetailURL, SessionCookies: twoPhase.DetailCookies}, domain.CaptchaPhaseDetail, parsedJSON)
		return
	}

	if twoPhase.Detail != nil {
		parsed.Items = enrichWithBarcodes(parsed.Items, twoPhase.Detail.HTML)
	}

	if err := w.persistReceipt(ctx, msg, parsed, job.SubmittedBy); err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to persist receipt: %v", err), nil)
		slog.Error("job failed: persistence error", "job_id", msg.JobID, "error", err)
	}
}

// resumeDetailPhase resumes a job that was paused during the summary→detail transition.
// It navigates only to the detail page (using captcha_current_url), merges EANs onto
// the persisted parsed_summary, and persists the final receipt.
func (w *Worker) resumeDetailPhase(ctx context.Context, jobCtx context.Context, msg *queue.JobMessage, job *domain.ScrapingJob, cookies []BrowserCookie, ua string) {
	if w.completeDetailPhaseFromSubmittedHTML(ctx, msg, job) {
		return
	}

	if job.CaptchaCurrentURL == nil || *job.CaptchaCurrentURL == "" {
		slog.Error("resume detail phase: captcha_current_url is empty", "job_id", msg.JobID)
		w.handleFailure(ctx, msg, fmt.Errorf("detail-phase resume: captcha_current_url is empty"))
		return
	}

	detailURL := *job.CaptchaCurrentURL
	detailResult, err := w.executor.FetchPageWithCookies(jobCtx, detailURL, cookies, ua)
	if err != nil {
		w.handleFailure(ctx, msg, err)
		return
	}

	if DetectCaptcha(detailResult) {
		if job.CaptchaRetryCount >= 2 {
			reason := domain.FailureCaptchaExpired
			_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
				"captcha persisted on detail page after human resolution", nil)
			slog.Warn("job failed: detail captcha expired on resume", "job_id", msg.JobID)
		} else {
			w.handleCaptchaPause(ctx, msg, detailResult, domain.CaptchaPhaseDetail, nil)
		}
		return
	}

	// Load the persisted summary payload.
	if job.ParsedSummary == nil {
		slog.Error("resume detail phase: parsed_summary is nil", "job_id", msg.JobID)
		w.handleFailure(ctx, msg, fmt.Errorf("detail-phase resume: parsed_summary is missing"))
		return
	}

	var parsed ParsedReceipt
	if err := json.Unmarshal(*job.ParsedSummary, &parsed); err != nil {
		w.handleFailure(ctx, msg, fmt.Errorf("detail-phase resume: unmarshal parsed_summary: %w", err))
		return
	}

	parsed.Items = enrichWithBarcodes(parsed.Items, detailResult.HTML)

	if err := w.persistReceipt(ctx, msg, &parsed, job.SubmittedBy); err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to persist receipt: %v", err), nil)
		slog.Error("job failed: persistence error on detail resume", "job_id", msg.JobID, "error", err)
		return
	}

	slog.Info("job completed via detail-phase resume", "job_id", msg.JobID)
}

// completeDetailPhaseFromSubmittedHTML completes a detail-phase captcha resume
// from the rendered WebView HTML submitted by the mobile app. It returns false
// when no usable HTML was submitted so the caller can fall back to browser resume.
func (w *Worker) completeDetailPhaseFromSubmittedHTML(ctx context.Context, msg *queue.JobMessage, job *domain.ScrapingJob) bool {
	if job.CaptchaResumePageHTML == nil || strings.TrimSpace(*job.CaptchaResumePageHTML) == "" {
		return false
	}

	barcodes, err := ParseDetailPage(*job.CaptchaResumePageHTML)
	if err != nil {
		slog.Warn("resume detail phase: submitted HTML is not a usable detail page, falling back to browser resume",
			"job_id", msg.JobID,
			"html_length", len(*job.CaptchaResumePageHTML),
			"error", err,
		)
		return false
	}

	if job.ParsedSummary == nil {
		slog.Error("resume detail phase: parsed_summary is nil for submitted HTML resume", "job_id", msg.JobID)
		w.handleFailure(ctx, msg, fmt.Errorf("detail-phase submitted HTML resume: parsed_summary is missing"))
		return true
	}

	var parsed ParsedReceipt
	if err := json.Unmarshal(*job.ParsedSummary, &parsed); err != nil {
		w.handleFailure(ctx, msg, fmt.Errorf("detail-phase submitted HTML resume: unmarshal parsed_summary: %w", err))
		return true
	}

	enriched, err := MergeBarcodes(parsed.Items, barcodes)
	if err != nil {
		slog.Warn("resume detail phase: submitted HTML barcode merge failed, persisting summary items without barcode",
			"job_id", msg.JobID,
			"error", err,
		)
	} else {
		parsed.Items = enriched
	}

	if err := w.persistReceipt(ctx, msg, &parsed, job.SubmittedBy); err != nil {
		reason := domain.FailureUnknown
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			fmt.Sprintf("failed to persist receipt: %v", err), nil)
		slog.Error("job failed: persistence error on submitted HTML detail resume", "job_id", msg.JobID, "error", err)
		return true
	}

	slog.Info("job completed via submitted detail HTML resume",
		"job_id", msg.JobID,
		"ean_count", len(barcodes),
		"html_length", len(*job.CaptchaResumePageHTML),
	)
	return true
}

// enrichWithBarcodes parses the detail page HTML, merges EAN codes into items by
// positional index, and returns the (possibly mutated) items. On any error it logs
// a warning and returns items unchanged — barcode enrichment is non-fatal.
func enrichWithBarcodes(items []domain.Item, detailHTML string) []domain.Item {
	barcodes, err := ParseDetailPage(detailHTML)
	if err != nil {
		slog.Warn("detail page: EAN extraction failed, items will have no barcode", "error", err)
		return items
	}

	enriched, err := MergeBarcodes(items, barcodes)
	if err != nil {
		slog.Warn("detail page: barcode merge failed, items will have no barcode", "error", err)
		return items
	}

	return enriched
}

// handleCaptchaPause pauses the job in awaiting_captcha, persisting URL, cookies, phase,
// and optionally the already-parsed summary payload (for detail-phase pauses).
func (w *Worker) handleCaptchaPause(ctx context.Context, msg *queue.JobMessage, result *ExecutorResult, phase domain.CaptchaPhase, parsedSummary json.RawMessage) {
	cookiesJSON := json.RawMessage(`[]`)
	if result != nil && len(result.SessionCookies) > 0 {
		cookiesJSON = result.SessionCookies
	}
	currentURL := result.FinalURL
	if currentURL == "" {
		currentURL = msg.FiscalURL
	}

	if err := w.jobs.PauseJobForCaptcha(ctx, msg.JobID, currentURL, cookiesJSON, phase, parsedSummary); err != nil {
		slog.Error("failed to pause job for captcha", "job_id", msg.JobID, "error", err)
		reason := domain.FailureCaptcha
		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
			"captcha detected; failed to pause job", nil)
		return
	}
	slog.Warn("job paused: captcha detected, awaiting user resolution",
		"job_id", msg.JobID,
		"captcha_url", currentURL,
		"phase", phase,
	)
}

// persistReceipt saves the parsed receipt data (store, receipt, items) to PostgreSQL
// and marks the job as completed. submittedBy is the users.id of the House member
// whose authenticated request originated the scraping job; it is stored on the
// receipt so the API can later attribute the receipt to its submitter.
func (w *Worker) persistReceipt(ctx context.Context, msg *queue.JobMessage, parsed *ParsedReceipt, submittedBy int64) error {
	store := &parsed.Store
	if err := w.receipts.UpsertStore(ctx, store); err != nil {
		return fmt.Errorf("upsert store: %w", err)
	}

	createdBy := submittedBy
	receipt := &domain.Receipt{
		HouseID:     msg.HouseID,
		StoreID:     store.ID,
		FiscalKey:   parsed.Receipt.FiscalKey,
		FiscalURL:   msg.FiscalURL,
		IssuedAt:    parsed.Receipt.IssuedAt,
		TotalAmount: parsed.Receipt.TotalAmount,
		CreatedBy:   &createdBy,
	}
	if err := w.receipts.CreateReceipt(ctx, receipt); err != nil {
		return fmt.Errorf("create receipt: %w", err)
	}

	if len(parsed.Items) > 0 {
		for i := range parsed.Items {
			parsed.Items[i].ReceiptID = receipt.ID
		}
		if err := w.receipts.CreateItems(ctx, parsed.Items); err != nil {
			return fmt.Errorf("create items: %w", err)
		}
	}

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

	isTimeout := isTimeoutError(err)

	var reason domain.FailureReason
	if isTimeout {
		reason = domain.FailureTimeout
	} else {
		reason = domain.FailureNavigation
	}

	if nextAttempt < w.config.MaxRetries {
		retryMsg := queue.JobMessage{
			JobID:     msg.JobID,
			FiscalURL: msg.FiscalURL,
			HouseID:   msg.HouseID,
			Attempt:   nextAttempt,
		}

		_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusQueued, nil,
			fmt.Sprintf("attempt %d failed: %v — retrying", nextAttempt, err), nil)

		if enqErr := w.queue.Enqueue(ctx, retryMsg); enqErr != nil {
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

	_ = w.jobs.UpdateJobStatus(ctx, msg.JobID, domain.JobStatusFailed, &reason,
		fmt.Sprintf("all %d attempts exhausted: %v", w.config.MaxRetries, err), nil)
	slog.Error("job failed: max retries exhausted",
		"job_id", msg.JobID,
		"attempts", nextAttempt,
		"max_retries", w.config.MaxRetries,
		"error", err,
	)
}

// handleParsingFailure processes a parsing error — not retried since the same HTML
// would produce the same error.
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
