import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { SubmitReceiptResponse } from "../../src/support/types";

test("deletes a receipt, clears historical references, and enforces house isolation", async ({ api, bearerToken, state }) => {
  const uniqueUrl = `${config.sefazMockInternalUrl}?case=deletion`;

  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: uniqueUrl },
  });

  expect(submit.status()).toBe(202);
  const payload = (await submit.json()) as SubmitReceiptResponse;

  const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 30_000,
    intervalMs: 1_000,
  });

  expect(job.status).toBe("completed");
  expect(job.receipt_id).toBeTruthy();

  const receiptId = job.receipt_id!;

  const deleteResponse = await api.delete(`/api/v1/receipts/${receiptId}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(deleteResponse.status()).toBe(204);

  const deletedReceipt = await state.postgres.getReceipt(receiptId);
  expect(deletedReceipt).toBeNull();

  const deletedItems = await state.postgres.getItemsByReceiptId(receiptId);
  expect(deletedItems).toHaveLength(0);

  const dbJob = await state.postgres.getJob(payload.job_id);
  expect(dbJob?.receipt_id).toBeNull();
  expect(dbJob?.status).toBe("completed");

  const getDeletedReceipt = await api.get(`/api/v1/receipts/${receiptId}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(getDeletedReceipt.status()).toBe(404);

  const deleteAgain = await api.delete(`/api/v1/receipts/${receiptId}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(deleteAgain.status()).toBe(404);

  const listResponse = await api.get("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(listResponse.status()).toBe(200);
  await expect(listResponse.json()).resolves.toEqual([]);

  const houseInsert = await state.pg.query<{ id: string }>(
    "INSERT INTO houses (id, name) VALUES (2, 'Casa Secundaria E2E') RETURNING id::text AS id",
  );
  const foreignHouseId = Number(houseInsert.rows[0].id);

  const storeInsert = await state.pg.query<{ id: string }>(
    `INSERT INTO stores (cnpj, name, address)
     VALUES ('11111111000191', 'LOJA BLOQUEADA E2E', '')
     RETURNING id::text AS id`,
  );
  const foreignStoreId = Number(storeInsert.rows[0].id);

  const foreignReceiptInsert = await state.pg.query<{ id: string }>(
    `INSERT INTO receipts (house_id, store_id, fiscal_key, fiscal_url, issued_at, total_amount)
     VALUES ($1, $2, '99999999999999999999999999999999999999999999', 'https://sefaz.example/foreign-receipt', NOW(), 42.50)
     RETURNING id::text AS id`,
    [foreignHouseId, foreignStoreId],
  );
  const foreignReceiptId = Number(foreignReceiptInsert.rows[0].id);

  const foreignDelete = await api.delete(`/api/v1/receipts/${foreignReceiptId}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(foreignDelete.status()).toBe(403);

  const foreignReceipt = await state.postgres.getReceipt(foreignReceiptId);
  expect(foreignReceipt).not.toBeNull();
  expect(foreignReceipt?.house_id).toBe(foreignHouseId);
});
