ALTER TABLE scraping_jobs
    DROP COLUMN IF EXISTS captcha_phase,
    DROP COLUMN IF EXISTS parsed_summary;
