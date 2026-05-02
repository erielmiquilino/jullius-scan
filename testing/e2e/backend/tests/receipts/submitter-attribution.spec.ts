import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { ReceiptResponse, SubmitReceiptResponse } from "../../src/support/types";

test.describe("receipt submitter attribution", () => {
  test("GET /api/v1/receipts/{id} returns submitted_by for a receipt produced by an authenticated submission", async ({
    api,
    bearerToken,
    state,
  }) => {
    const submit = await api.post("/api/v1/receipts", {
      headers: { Authorization: `Bearer ${bearerToken}` },
      data: { fiscal_url: config.sefazMockInternalUrl },
    });
    expect(submit.status()).toBe(202);
    const submitPayload = (await submit.json()) as SubmitReceiptResponse;

    const job = await pollJobUntilTerminal(api, submitPayload.job_id, bearerToken, {
      timeoutMs: 30_000,
      intervalMs: 1_000,
    });
    expect(job.status).toBe("completed");
    expect(job.receipt_id).toBeTruthy();

    const detail = await api.get(`/api/v1/receipts/${job.receipt_id}`, {
      headers: { Authorization: `Bearer ${bearerToken}` },
    });
    expect(detail.status()).toBe(200);
    const receipt = (await detail.json()) as ReceiptResponse;

    expect(receipt.submitted_by).toBeDefined();
    expect(receipt.submitted_by!.id).toBe(1);
    expect(receipt.submitted_by!.email).toBe(config.firebaseTestUserEmail);
    expect(receipt.submitted_by!.name).toBe("Usuario de Testes E2E");

    // Sanity: created_by also stamped on the receipts row.
    const createdByRow = await state.pg.query<{ created_by: string | null }>(
      "SELECT created_by::text FROM receipts WHERE id = $1",
      [job.receipt_id],
    );
    expect(createdByRow.rows[0]?.created_by).toBe("1");
  });

  test("GET /api/v1/receipts/{id} omits submitted_by when the receipt has no recorded submitter", async ({
    api,
    bearerToken,
    state,
  }) => {
    // Seeded directly via the postgres helper, which leaves created_by NULL.
    const seeded = await state.postgres.seedReceiptWithItems(
      1,
      { cnpj: "12345678000199", name: "LOJA SEM AUTOR" },
      [{ description: "Item sem autor", unitPrice: 10.0 }],
    );

    const detail = await api.get(`/api/v1/receipts/${seeded.receiptId}`, {
      headers: { Authorization: `Bearer ${bearerToken}` },
    });
    expect(detail.status()).toBe(200);
    const receipt = (await detail.json()) as ReceiptResponse;

    expect(receipt.submitted_by).toBeUndefined();
  });

  test("GET /api/v1/receipts (listing) does not include submitted_by per item even when receipts have created_by populated", async ({
    api,
    bearerToken,
    state,
  }) => {
    // Receipt with a known submitter (created_by = 1).
    const seeded = await state.postgres.seedReceiptWithItems(
      1,
      { cnpj: "98765432000111", name: "MERCADINHO LISTAGEM" },
      [{ description: "Item de teste de lista", unitPrice: 5.5 }],
    );
    await state.pg.query(`UPDATE receipts SET created_by = 1 WHERE id = $1`, [seeded.receiptId]);

    const listing = await api.get(`/api/v1/receipts`, {
      headers: { Authorization: `Bearer ${bearerToken}` },
    });
    expect(listing.status()).toBe(200);
    const receipts = (await listing.json()) as ReceiptResponse[];
    expect(receipts.length).toBeGreaterThan(0);

    for (const r of receipts) {
      expect(r.submitted_by).toBeUndefined();
    }
  });
});
