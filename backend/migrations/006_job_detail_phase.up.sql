-- captcha_phase distinguishes where in the scraping pipeline the job was paused:
--   'summary' (or NULL): paused while loading the NFC-e summary page (original behavior)
--   'detail'           : paused while navigating from summary to the NFC-e detail page
--
-- parsed_summary stores the already-parsed summary payload (store + receipt + items
-- without barcode) as JSONB so the worker can skip re-scraping the summary page when
-- resuming a detail-phase captcha pause.
ALTER TABLE scraping_jobs
    ADD COLUMN captcha_phase    TEXT,
    ADD COLUMN parsed_summary   JSONB;
