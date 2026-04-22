import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { SubmitReceiptResponse } from "../../src/support/types";

const captchaChallengeUrl = config.sefazMockInternalUrl.replace(
  /\/[^/]+$/,
  "/nfce/captcha-challenge",
);

// This test relies on CAPTCHA_TIMEOUT_MINUTES=1 (or lower) in the E2E stack.
// See docker-compose.e2e.yml where CAPTCHA_TIMEOUT_MINUTES defaults to 2.
// The API expiry goroutine runs every 1 minute, so with a 2-minute timeout
// a job paused for captcha will be expired within ~3 minutes.
test("job in awaiting_captcha is expired to failed after timeout", async ({
  api,
  bearerToken,
  state: _state,
}) => {
  test.setTimeout(5 * 60 * 1_000);

  const submitRes = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: captchaChallengeUrl },
  });
  expect(submitRes.ok()).toBeTruthy();
  const payload = (await submitRes.json()) as SubmitReceiptResponse;
  expect(payload.job_id).toBeTruthy();

  // Wait until the job reaches awaiting_captcha.
  const pausedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 45_000,
    intervalMs: 1_000,
  });
  expect(pausedJob.status).toBe("awaiting_captcha");

  // Wait for the expiry goroutine to fire (timeout + ticker interval = ~3 min).
  // Exclude awaiting_captcha from terminal statuses so we keep polling past it.
  const expiredJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 4 * 60 * 1_000,
    intervalMs: 5_000,
    terminalStatuses: ["failed", "completed"],
  });
  expect(expiredJob.status).toBe("failed");
  expect(expiredJob.failure_reason).toBe("captcha_timeout");
});
