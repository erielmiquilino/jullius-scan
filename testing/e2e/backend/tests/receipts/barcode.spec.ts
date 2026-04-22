import { test, expect } from "../fixtures";
import { pollJobUntilTerminal } from "../../src/support/polling";
import { fetchCaptchaContext, submitCaptchaResume, solvedCookies } from "../../src/support/captcha";

const SEFAZ_MOCK_BASE = "http://sefaz-mock:8091";
const SUMMARY_URL = `${SEFAZ_MOCK_BASE}/nfce-consulta-detalhada.html`;
const DETAIL_CAPTCHA_SUMMARY_URL = `${SEFAZ_MOCK_BASE}/nfce-captcha-detail-summary.html`;

test("happy path: items contain barcode when detail page is available", async ({ api, bearerToken, state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: SUMMARY_URL },
  });
  expect(submit.status()).toBe(202);
  const payload = await submit.json();

  const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
  });
  expect(job.status).toBe("completed");
  expect(job.receipt_id).toBeTruthy();

  const dbJob = await state.postgres.getJob(payload.job_id);
  const items = await state.postgres.getItemsByReceiptId(dbJob!.receipt_id!);
  expect(items.length).toBe(3);

  // Item 1 (PAO FRANCES) has SEM GTIN in detail page → barcode should be null
  expect(items[0]!.barcode).toBeNull();

  // Item 2 (ARROZ) and Item 3 (CAFE) have EAN codes
  expect(items[1]!.barcode).toBe("7896068400100");
  expect(items[2]!.barcode).toBe("7896005800058");
});

test("detail-phase captcha: job pauses with captcha_phase=detail, resumes and completes with barcode", async ({ api, bearerToken, state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: DETAIL_CAPTCHA_SUMMARY_URL },
  });
  expect(submit.status()).toBe(202);
  const payload = await submit.json();

  // Job should pause at awaiting_captcha due to detail-page captcha
  const pausedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
  });
  expect(pausedJob.status).toBe("awaiting_captcha");

  // Verify captcha_phase is 'detail' in the database
  const dbJobPaused = await state.postgres.getJob(payload.job_id);
  expect(dbJobPaused!.captcha_phase).toBe("detail");

  // Fetch captcha context — should return the detail captcha challenge URL
  const ctx = await fetchCaptchaContext(api, payload.job_id, bearerToken);
  expect(ctx.sefaz_url).toContain("detail-challenge");

  // Submit solved cookies — the mock accepts e2e_captcha_solved=1
  const sefazHost = new URL(DETAIL_CAPTCHA_SUMMARY_URL).hostname;
  await submitCaptchaResume(api, payload.job_id, solvedCookies(sefazHost), bearerToken);

  // Job should now complete with barcode data
  const completedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
    terminalStatuses: ["completed", "failed"],
  });
  expect(completedJob.status).toBe("completed");

  const dbJob = await state.postgres.getJob(payload.job_id);
  const items = await state.postgres.getItemsByReceiptId(dbJob!.receipt_id!);
  expect(items.length).toBe(3);
  expect(items[1]!.barcode).toBe("7896068400100");
  expect(items[2]!.barcode).toBe("7896005800058");
});
