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

  /**
   * Inserts a complete (store, receipt, items) tuple directly into the database,
   * bypassing the worker. Used by tests that exercise read-only flows (item search)
   * where exercising the scraper would only add latency and flakiness.
   *
   * The store is upserted by CNPJ so multiple receipts can share the same store.
   * issuedAt defaults to NOW() if omitted.
   */
  async seedReceiptWithItems(
    houseId: number,
    store: { cnpj: string; name: string; address?: string },
    items: Array<{
      description: string;
      barcode?: string | null;
      quantity?: number;
      unit?: string;
      unitPrice: number;
      totalPrice?: number;
    }>,
    options: { issuedAt?: Date; fiscalKey?: string; fiscalUrl?: string; totalAmount?: number } = {},
  ): Promise<{ storeId: number; receiptId: number; itemIds: number[] }> {
    const issuedAt = options.issuedAt ?? new Date();
    const totalAmount =
      options.totalAmount ??
      items.reduce((acc, it) => acc + (it.totalPrice ?? (it.quantity ?? 1) * it.unitPrice), 0);
    const fiscalUrl = options.fiscalUrl ?? `https://seeded.example/${store.cnpj}/${issuedAt.toISOString()}/${Math.random()}`;
    const fiscalKey = options.fiscalKey ?? `SEED-${Math.random().toString(36).slice(2, 18).toUpperCase()}`;

    const storeRow = await this.client.query<{ id: string }>(
      `INSERT INTO stores (cnpj, name, address)
       VALUES ($1, $2, $3)
       ON CONFLICT (cnpj) DO UPDATE SET name = EXCLUDED.name, address = EXCLUDED.address
       RETURNING id`,
      [store.cnpj, store.name, store.address ?? ""],
    );
    const storeId = Number(storeRow.rows[0].id);

    const receiptRow = await this.client.query<{ id: string }>(
      `INSERT INTO receipts (house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount)
       VALUES ($1, $2, $3, $4, $5, $6)
       RETURNING id`,
      [houseId, storeId, fiscalKey, fiscalUrl, issuedAt.toISOString(), totalAmount],
    );
    const receiptId = Number(receiptRow.rows[0].id);

    const itemIds: number[] = [];
    for (const it of items) {
      const quantity = it.quantity ?? 1;
      const totalPrice = it.totalPrice ?? quantity * it.unitPrice;
      const itemRow = await this.client.query<{ id: string }>(
        `INSERT INTO items (receipt_id, description, quantity, unit, unit_price, total_price, barcode)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id`,
        [receiptId, it.description, quantity, it.unit ?? "UN", it.unitPrice, totalPrice, it.barcode ?? null],
      );
      itemIds.push(Number(itemRow.rows[0].id));
    }

    return { storeId, receiptId, itemIds };
  }

  /**
   * Inserts an additional house (used by isolation tests).
   *
   * The base seed (seedBaseUserAndHouse) inserts house id=1 with an explicit value,
   * which leaves the BIGSERIAL sequence behind. We advance it here before relying
   * on the default to avoid collisions with that pre-seeded row.
   */
  async seedHouse(name: string): Promise<number> {
    await this.client.query(
      `SELECT setval(pg_get_serial_sequence('houses', 'id'),
                     GREATEST(1, COALESCE((SELECT MAX(id) FROM houses), 0)))`,
    );
    const row = await this.client.query<{ id: string }>(
      `INSERT INTO houses (name) VALUES ($1) RETURNING id`,
      [name],
    );
    return Number(row.rows[0].id);
  }
}
