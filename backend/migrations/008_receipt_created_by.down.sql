-- 008_receipt_created_by.down.sql

DROP INDEX IF EXISTS idx_receipts_created_by;

ALTER TABLE receipts
    DROP COLUMN IF EXISTS created_by;
