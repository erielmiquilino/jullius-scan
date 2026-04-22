ALTER TABLE scraping_jobs
    DROP CONSTRAINT IF EXISTS scraping_jobs_receipt_id_fkey;

ALTER TABLE scraping_jobs
    ADD CONSTRAINT scraping_jobs_receipt_id_fkey
    FOREIGN KEY (receipt_id) REFERENCES receipts(id) ON DELETE SET NULL;
