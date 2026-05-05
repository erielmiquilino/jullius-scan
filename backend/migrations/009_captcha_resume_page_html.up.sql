-- Stores the rendered WebView HTML submitted when a human-resolved captcha
-- lands directly on the NFC-e detail page. This is consumed by the worker only
-- and is never returned by the API.

ALTER TABLE scraping_jobs
    ADD COLUMN captcha_resume_page_html TEXT;
