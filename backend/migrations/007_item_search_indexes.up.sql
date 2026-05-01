-- 007_item_search_indexes.up.sql
-- Indexes for the item-search-history feature: trigram + diacritic-insensitive
-- substring search over items.description and exact lookup on items.barcode.

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- Postgres' built-in unaccent() is STABLE because it depends on a dictionary,
-- which prevents it from being used directly inside expression indexes. The
-- canonical workaround is a thin SQL wrapper marked IMMUTABLE that calls
-- unaccent() with the explicit dictionary regdictionary. The dictionary file
-- ships with the unaccent extension and does not change at runtime, so
-- treating the wrapper as immutable is safe in practice.
CREATE OR REPLACE FUNCTION immutable_unaccent(text) RETURNS text
    LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
$$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;

CREATE INDEX IF NOT EXISTS idx_items_description_trgm
    ON items
    USING GIN (immutable_unaccent(lower(description)) gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_items_barcode_notnull
    ON items (barcode)
    WHERE barcode IS NOT NULL;
