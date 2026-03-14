-- 001_create_tables.up.sql
-- Core tables for Jullius Scan MVP

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    firebase_id VARCHAR(128) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_firebase_id ON users(firebase_id);

CREATE TABLE IF NOT EXISTS houses (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS house_members (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    house_id BIGINT NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, house_id)
);

CREATE INDEX idx_house_members_user_id ON house_members(user_id);
CREATE INDEX idx_house_members_house_id ON house_members(house_id);

CREATE TABLE IF NOT EXISTS stores (
    id BIGSERIAL PRIMARY KEY,
    cnpj VARCHAR(18) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    address TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stores_cnpj ON stores(cnpj);

CREATE TABLE IF NOT EXISTS receipts (
    id BIGSERIAL PRIMARY KEY,
    house_id BIGINT NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
    store_id BIGINT NOT NULL REFERENCES stores(id),
    fiscal_key VARCHAR(64) NOT NULL,
    fiscal_url TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    total_amount NUMERIC(12, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(house_id, fiscal_url)
);

CREATE INDEX idx_receipts_house_id ON receipts(house_id);
CREATE INDEX idx_receipts_fiscal_url ON receipts(fiscal_url);

CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    receipt_id BIGINT NOT NULL REFERENCES receipts(id) ON DELETE CASCADE,
    description VARCHAR(512) NOT NULL,
    quantity NUMERIC(10, 4) NOT NULL DEFAULT 1,
    unit VARCHAR(20) NOT NULL DEFAULT 'UN',
    unit_price NUMERIC(12, 4) NOT NULL DEFAULT 0,
    total_price NUMERIC(12, 2) NOT NULL DEFAULT 0
);

CREATE INDEX idx_items_receipt_id ON items(receipt_id);

CREATE TABLE IF NOT EXISTS scraping_jobs (
    id BIGSERIAL PRIMARY KEY,
    house_id BIGINT NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
    submitted_by BIGINT NOT NULL REFERENCES users(id),
    fiscal_url TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0,
    failure_reason VARCHAR(50),
    error_detail TEXT DEFAULT '',
    receipt_id BIGINT REFERENCES receipts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_scraping_jobs_house_id ON scraping_jobs(house_id);
CREATE INDEX idx_scraping_jobs_status ON scraping_jobs(status);
CREATE INDEX idx_scraping_jobs_fiscal_url ON scraping_jobs(fiscal_url);
