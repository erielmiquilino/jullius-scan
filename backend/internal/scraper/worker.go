package scraper

import (
	"context"
	"log/slog"
	"time"

	"github.com/erielfranco/jullius-scan/backend/internal/config"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
)

// Worker processes scraping jobs from the Redis queue.
type Worker struct {
	db     *database.DB
	queue  *queue.Client
	config *config.Config
}

// NewWorker creates a new scraping worker.
func NewWorker(db *database.DB, q *queue.Client, cfg *config.Config) *Worker {
	return &Worker{
		db:     db,
		queue:  q,
		config: cfg,
	}
}

// Run starts the worker loop, consuming jobs until the context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("worker started, waiting for jobs")

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

		// Actual chromedp execution will be implemented in task group 3.
		// For now, mark the job lifecycle as a placeholder.
		w.processJob(ctx, msg)
	}
}

// processJob handles a single scraping job with timeout.
// Full chromedp implementation is in task group 3.
func (w *Worker) processJob(ctx context.Context, msg *queue.JobMessage) {
	jobCtx, cancel := context.WithTimeout(ctx, w.config.ScrapeTimeout)
	defer cancel()

	slog.Info("job processing started",
		"job_id", msg.JobID,
		"timeout", w.config.ScrapeTimeout,
	)

	// TODO(task-group-3): Implement chromedp browser automation here.
	// This placeholder logs the job and respects the timeout context.
	select {
	case <-jobCtx.Done():
		slog.Warn("job timed out or cancelled", "job_id", msg.JobID)
	default:
		slog.Info("job placeholder completed (no scraping yet)", "job_id", msg.JobID)
	}
}
