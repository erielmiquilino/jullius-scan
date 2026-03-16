import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { ReceiptResponse, SubmitReceiptResponse } from "../../src/support/types";

test("retrieves completed job and receipt through the API for the seeded house", async ({ api, bearerToken }) => {
  const uniqueUrl = `${config.sefazMockInternalUrl}?case=retrieval`;

  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: uniqueUrl },
  });

  expect(submit.status()).toBe(202);
  const payload = (await submit.json()) as SubmitReceiptResponse;
  const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, { timeoutMs: 30_000, intervalMs: 1_000 });
  expect(job.receipt_id).toBeTruthy();

  const jobResponse = await api.get(`/api/v1/jobs/${payload.job_id}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(jobResponse.status()).toBe(200);
  await expect(jobResponse.json()).resolves.toMatchObject({
    id: payload.job_id,
    house_id: 1,
    status: "completed",
  });

  const listResponse = await api.get("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(listResponse.status()).toBe(200);
  const list = (await listResponse.json()) as ReceiptResponse[];
  expect(list.length).toBeGreaterThan(0);

  const receiptResponse = await api.get(`/api/v1/receipts/${job.receipt_id}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(receiptResponse.status()).toBe(200);
  const receipt = (await receiptResponse.json()) as ReceiptResponse;
  expect(receipt.id).toBe(job.receipt_id);
  expect(receipt.house_id).toBe(1);
  expect(receipt.store?.cnpj).toBe("75492694000201");
  expect(receipt.items?.length ?? 0).toBeGreaterThan(0);
});
