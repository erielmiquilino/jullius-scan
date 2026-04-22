-- 002_captcha_resume.down.sql

DROP INDEX IF EXISTS idx_scraping_jobs_awaiting_captcha;

ALTER TABLE scraping_jobs
    DROP CONSTRAINT IF EXISTS scraping_jobs_status_check;

ALTER TABLE scraping_jobs
    DROP COLUMN IF EXISTS captcha_retry_count,
    DROP COLUMN IF EXISTS captcha_resumed_at,
    DROP COLUMN IF EXISTS captcha_pending_at,
    DROP COLUMN IF EXISTS captcha_session_cookies,
    DROP COLUMN IF EXISTS captcha_current_url;
