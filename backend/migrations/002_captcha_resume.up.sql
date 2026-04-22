-- 002_captcha_resume.up.sql
-- Adds captcha pause/resume support to scraping_jobs

ALTER TABLE scraping_jobs
    ADD COLUMN captcha_current_url        TEXT,
    ADD COLUMN captcha_session_cookies    JSONB,
    ADD COLUMN captcha_pending_at         TIMESTAMPTZ,
    ADD COLUMN captcha_resumed_at         TIMESTAMPTZ,
    ADD COLUMN captcha_retry_count        SMALLINT NOT NULL DEFAULT 0;

-- Extend the status column to allow the new awaiting_captcha state.
-- The existing column has no CHECK constraint; we add one that covers all valid values.
ALTER TABLE scraping_jobs
    ADD CONSTRAINT scraping_jobs_status_check
        CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'awaiting_captcha'));

CREATE INDEX idx_scraping_jobs_awaiting_captcha
    ON scraping_jobs (captcha_pending_at)
    WHERE status = 'awaiting_captcha';
