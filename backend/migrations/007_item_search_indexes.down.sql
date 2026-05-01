-- 007_item_search_indexes.down.sql
-- Drops the indexes and helper function added by the up migration.
-- Extensions are intentionally NOT removed to avoid breaking any other code
-- that may have started depending on them.

DROP INDEX IF EXISTS idx_items_barcode_notnull;
DROP INDEX IF EXISTS idx_items_description_trgm;
DROP FUNCTION IF EXISTS immutable_unaccent(text);
