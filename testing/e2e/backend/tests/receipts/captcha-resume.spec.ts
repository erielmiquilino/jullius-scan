import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import { fetchCaptchaContext, submitCaptchaResume, solvedCookies } from "../../src/support/captcha";
import type { SubmitReceiptResponse } from "../../src/support/types";

const captchaChallengeUrl = config.sefazMockInternalUrl.replace(
  /\/[^/]+$/,
  "/nfce/captcha-challenge",
);

test("job pauses on captcha, resumes with user-supplied cookies, and completes", async ({
  api,
  bearerToken,
  state,
}) => {
  // Submit the URL that triggers the captcha challenge on the mock server.
  const submitRes = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: captchaChallengeUrl },
  });
  expect(submitRes.ok()).toBeTruthy();
  const payload = (await submitRes.json()) as SubmitReceiptResponse;
  expect(payload.job_id).toBeTruthy();

  // Wait until the worker detects the captcha and pauses the job.
  const pausedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 45_000,
    intervalMs: 1_000,
  });
  expect(pausedJob.status).toBe("awaiting_captcha");
  expect(pausedJob.captcha_pending_at).toBeTruthy();

  // Fetch the captcha context (SEFAZ URL + user-agent).
  const ctx = await fetchCaptchaContext(api, payload.job_id, bearerToken);
  expect(ctx.sefaz_url).toContain("/nfce/captcha-challenge");
  expect(ctx.user_agent).toBeTruthy();

  // Simulate the user resolving the captcha by supplying the magic cookie.
  const mockHost = new URL(captchaChallengeUrl).hostname;
  await submitCaptchaResume(api, payload.job_id, solvedCookies(mockHost), bearerToken);

  // Poll until the job completes (worker resumes with cookies and gets the real HTML).
  const completedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
  });
  expect(completedJob.status).toBe("completed");
  expect(completedJob.receipt_id).toBeTruthy();

  // Assert the receipt was persisted in the database.
  const dbJob = await state.postgres.getJob(payload.job_id);
  expect(dbJob?.status).toBe("completed");
  expect(dbJob?.receipt_id).toBeTruthy();

  const receipt = await state.postgres.getReceipt(dbJob!.receipt_id!);
  expect(receipt).toBeTruthy();

  const items = await state.postgres.getItemsByReceiptId(receipt!.id);
  expect(items.length).toBeGreaterThan(0);
});
