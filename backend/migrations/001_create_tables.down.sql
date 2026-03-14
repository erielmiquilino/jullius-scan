-- 001_create_tables.down.sql
-- Rollback: drop all core tables in reverse dependency order

DROP TABLE IF EXISTS scraping_jobs;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS receipts;
DROP TABLE IF EXISTS stores;
DROP TABLE IF EXISTS house_members;
DROP TABLE IF EXISTS houses;
DROP TABLE IF EXISTS users;
