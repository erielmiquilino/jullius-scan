import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import { fetchCaptchaContext, submitCaptchaResume } from "../../src/support/captcha";
import type { SubmitReceiptResponse, SessionCookie } from "../../src/support/types";

const captchaChallengeUrl = config.sefazMockInternalUrl.replace(
  /\/[^/]+$/,
  "/nfce/captcha-challenge",
);

// Invalid cookies that will NOT unlock the SEFAZ mock — simulates an expired
// or wrong session returned by the user.
function invalidCookies(sefazHost: string): SessionCookie[] {
  return [
    {
      name: "e2e_captcha_solved",
      value: "0",
      domain: sefazHost,
      path: "/",
      http_only: false,
      secure: false,
    },
  ];
}

test("resume with invalid cookies causes second awaiting_captcha then failed with captcha_expired", async ({
  api,
  bearerToken,
  state: _state,
}) => {
  // Submit a URL that will trigger captcha.
  const submitRes = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: captchaChallengeUrl },
  });
  expect(submitRes.ok()).toBeTruthy();
  const payload = (await submitRes.json()) as SubmitReceiptResponse;

  // Wait for first awaiting_captcha.
  const firstPause = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 45_000,
    intervalMs: 1_000,
  });
  expect(firstPause.status).toBe("awaiting_captcha");

  await fetchCaptchaContext(api, payload.job_id, bearerToken);

  // Resume with invalid cookies (captcha_retry_count will be 0, so worker pauses again).
  const mockHost = new URL(captchaChallengeUrl).hostname;
  await submitCaptchaResume(api, payload.job_id, invalidCookies(mockHost), bearerToken);

  // Worker processes the resume, hits captcha again (retry_count=0 → second pause).
  const secondPause = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 45_000,
    intervalMs: 1_000,
  });
  expect(secondPause.status).toBe("awaiting_captcha");

  // Resume again with invalid cookies (retry_count=1 → terminal failure).
  await submitCaptchaResume(api, payload.job_id, invalidCookies(mockHost), bearerToken);

  const finalJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 45_000,
    intervalMs: 1_000,
  });
  expect(finalJob.status).toBe("failed");
  expect(finalJob.failure_reason).toBe("captcha_expired");
});
