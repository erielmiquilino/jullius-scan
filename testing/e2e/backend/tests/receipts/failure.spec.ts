import { test, expect } from "../fixtures";
import type { SubmitReceiptResponse } from "../../src/support/types";
import { pollJobUntilTerminal } from "../../src/support/polling";

test("pauses the job for captcha resolution when SEFAZ page shows captcha", async ({ api, bearerToken, state: _state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: "http://sefaz-mock:8091/captcha.html" },
  });

  expect(submit.status()).toBe(202);
  const payload = (await submit.json()) as SubmitReceiptResponse;

  const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, { timeoutMs: 30_000, intervalMs: 1_000 });
  expect(job.status).toBe("awaiting_captcha");
});
