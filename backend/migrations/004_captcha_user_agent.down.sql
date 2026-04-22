-- 004_captcha_user_agent.down.sql

ALTER TABLE scraping_jobs
    DROP COLUMN IF EXISTS captcha_user_agent;
