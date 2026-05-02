-- 008_receipt_created_by.up.sql
-- Track which House member submitted each receipt so the API can expose the
-- author on the receipt detail view. The column is nullable on purpose: rows
-- created before this migration may not have an unambiguous originating job
-- (e.g. job records cleared from history), and we want to keep returning those
-- receipts without inventing an author.

ALTER TABLE receipts
    ADD COLUMN created_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_receipts_created_by ON receipts(created_by);

-- Backfill from the oldest scraping job that ended up pointing to each receipt.
-- DISTINCT ON guarantees one row per receipt_id; ORDER BY created_at ASC picks
-- the original submitter when multiple jobs ended up linked to the same receipt
-- (e.g. retried submissions that converged on the same fiscal_url).
UPDATE receipts r
SET created_by = sub.submitted_by
FROM (
    SELECT DISTINCT ON (receipt_id) receipt_id, submitted_by
    FROM scraping_jobs
    WHERE receipt_id IS NOT NULL
    ORDER BY receipt_id, created_at ASC
) sub
WHERE r.id = sub.receipt_id
  AND r.created_by IS NULL;
