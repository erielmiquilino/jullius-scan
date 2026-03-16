import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { JobResponse, ReceiptResponse, SubmitReceiptResponse } from "../../src/support/types";

test("processes a receipt from the local SEFAZ mock and persists normalized data", async ({ api, bearerToken, state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: config.sefazMockInternalUrl },
  });

  expect(submit.status()).toBe(202);
  const payload = (await submit.json()) as SubmitReceiptResponse;
  expect(payload.status).toBe("queued");

  const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, { timeoutMs: 30_000, intervalMs: 1_000 });
  expect(job.status).toBe("completed");
  expect(job.receipt_id).toBeTruthy();

  const dbJob = await state.postgres.getJob(payload.job_id);
  expect(dbJob?.status).toBe("completed");
  expect(dbJob?.attempts).toBeGreaterThanOrEqual(1);
  expect(dbJob?.receipt_id).toBeTruthy();

  const receipt = await state.postgres.getReceipt(dbJob!.receipt_id!);
  expect(receipt).not.toBeNull();
  expect(receipt?.fiscal_url).toBe(config.sefazMockInternalUrl);
  expect(receipt?.house_id).toBe(1);

  const store = await state.postgres.getStore(receipt!.store_id);
  expect(store?.cnpj).toBe("75492694000201");
  expect(store?.name).toContain("SUPERMERCADOS MYATA");

  const items = await state.postgres.getItemsByReceiptId(receipt!.id);
  expect(items.length).toBeGreaterThan(0);
  expect(items[0]?.description).toContain("PAO FRANCES");
});
