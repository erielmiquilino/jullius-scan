-- 004_captcha_user_agent.up.sql
-- Stores the WebView user-agent used when the user solved the captcha challenge,
-- so the worker can replay the session with the exact same UA.

ALTER TABLE scraping_jobs
    ADD COLUMN captcha_user_agent TEXT;
