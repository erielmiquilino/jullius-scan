import { Client } from "pg";
import { config } from "../config";
import type { DatabaseItemRow, DatabaseJobRow, DatabaseReceiptRow, DatabaseStoreRow } from "./types";

const parseNullableInt = (value: unknown): number | null => {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  return Number(value);
};

const parseIntValue = (value: unknown): number => Number(value);

const normalizeJob = (row: DatabaseJobRow): DatabaseJobRow => ({
  ...row,
  id: parseIntValue(row.id),
  house_id: parseIntValue(row.house_id),
  submitted_by: parseIntValue(row.submitted_by),
  receipt_id: parseNullableInt(row.receipt_id),
});

const normalizeReceipt = (row: DatabaseReceiptRow): DatabaseReceiptRow => ({
  ...row,
  id: parseIntValue(row.id),
  house_id: parseIntValue(row.house_id),
  store_id: parseIntValue(row.store_id),
});

const normalizeStore = (row: DatabaseStoreRow): DatabaseStoreRow => ({
  ...row,
  id: parseIntValue(row.id),
});

const normalizeItem = (row: DatabaseItemRow): DatabaseItemRow => ({
  ...row,
  id: parseIntValue(row.id),
  receipt_id: parseIntValue(row.receipt_id),
});

export async function pgClient(): Promise<Client> {
  const client = new Client({ connectionString: config.pgUrl });
  await client.connect();
  return client;
}

export async function resetDatabase(): Promise<void> {
  const client = await pgClient();
  try {
    await client.query(`
      TRUNCATE TABLE
        items,
        receipts,
        stores,
        scraping_jobs,
        house_members,
        houses,
        users
      RESTART IDENTITY CASCADE
    `);
  } finally {
    await client.end();
  }
}

export class PostgresHelper {
  constructor(private readonly client: Client) {}

  async getJob(jobId: number): Promise<DatabaseJobRow | null> {
    const result = await this.client.query<DatabaseJobRow>(
      `SELECT id, house_id::bigint AS house_id, submitted_by::bigint AS submitted_by, fiscal_url, status, attempts, failure_reason, error_detail, receipt_id::bigint AS receipt_id, captcha_phase
       FROM scraping_jobs
       WHERE id = $1`,
      [jobId],
    );
    return result.rows[0] ? normalizeJob(result.rows[0]) : null;
  }

  async getLatestJob(): Promise<DatabaseJobRow | null> {
    const result = await this.client.query<DatabaseJobRow>(
      `SELECT id, house_id::bigint AS house_id, submitted_by::bigint AS submitted_by, fiscal_url, status, attempts, failure_reason, error_detail, receipt_id::bigint AS receipt_id
       FROM scraping_jobs
       ORDER BY id DESC
       LIMIT 1`,
    );
    return result.rows[0] ? normalizeJob(result.rows[0]) : null;
  }

  async getReceipt(receiptId: number): Promise<DatabaseReceiptRow | null> {
    const result = await this.client.query<DatabaseReceiptRow>(
      `SELECT id, house_id::bigint AS house_id, store_id::bigint AS store_id, fiscal_key, fiscal_url, total_amount::float8 AS total_amount
       FROM receipts
       WHERE id = $1`,
      [receiptId],
    );
    return result.rows[0] ? normalizeReceipt(result.rows[0]) : null;
  }

  async getStore(storeId: number): Promise<DatabaseStoreRow | null> {
    const result = await this.client.query<DatabaseStoreRow>(
      `SELECT id, cnpj, name, address
       FROM stores
       WHERE id = $1`,
      [storeId],
    );
    return result.rows[0] ? normalizeStore(result.rows[0]) : null;
  }

  async getItemsByReceiptId(receiptId: number): Promise<DatabaseItemRow[]> {
    const result = await this.client.query<DatabaseItemRow>(
      `SELECT id, receipt_id::bigint AS receipt_id, description, quantity::float8 AS quantity, unit, unit_price::float8 AS unit_price, total_price::float8 AS total_price, barcode
       FROM items
       WHERE receipt_id = $1
       ORDER BY id`,
      [receiptId],
    );
    return result.rows.map(normalizeItem);
  }

  async countRows(tableName: "users" | "houses" | "house_members" | "scraping_jobs" | "receipts" | "stores" | "items"): Promise<number> {
    const result = await this.client.query<{ count: string }>(`SELECT COUNT(*)::text AS count FROM ${tableName}`);
    return Number(result.rows[0]?.count || "0");
  }
}
