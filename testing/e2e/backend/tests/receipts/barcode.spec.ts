import fs from "node:fs/promises";
import path from "node:path";

import { test, expect } from "../fixtures";
import { pollJobUntilTerminal } from "../../src/support/polling";
import { fetchCaptchaContext, submitCaptchaResume, solvedCookies } from "../../src/support/captcha";
import type { ReceiptResponse } from "../../src/support/types";

const SEFAZ_MOCK_BASE = "http://sefaz-mock:8091";
const SUMMARY_URL = `${SEFAZ_MOCK_BASE}/nfce-consulta-detalhada.html`;
const DETAIL_CAPTCHA_SUMMARY_URL = `${SEFAZ_MOCK_BASE}/nfce-captcha-detail-summary.html`;
const DETAIL_HTML_PATH = path.join(__dirname, "../../mocks/sefaz/nfce-detalhe-cert.html");

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

  const receiptResponse = await api.get(`/api/v1/receipts/${dbJob!.receipt_id!}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(receiptResponse.status()).toBe(200);
  const receipt = (await receiptResponse.json()) as ReceiptResponse;
  expect(receipt.items).toHaveLength(3);
  expect(receipt.items?.[0]?.barcode).toBeUndefined();
  expect(receipt.items?.[1]?.barcode).toBe("7896068400100");
  expect(receipt.items?.[2]?.barcode).toBe("7896005800058");
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

  // Fetch captcha context — should return the detail URL that triggered the captcha page
  const ctx = await fetchCaptchaContext(api, payload.job_id, bearerToken);
  expect(ctx.sefaz_url).toContain("Nfe_DetalheCert.aspx");
  expect(ctx.sefaz_url).toContain("DETAIL_GATE_TOKEN");

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

  const receiptResponse = await api.get(`/api/v1/receipts/${dbJob!.receipt_id!}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  expect(receiptResponse.status()).toBe(200);
  const receipt = (await receiptResponse.json()) as ReceiptResponse;
  expect(receipt.items?.[1]?.barcode).toBe("7896068400100");
  expect(receipt.items?.[2]?.barcode).toBe("7896005800058");
});

test("detail-phase captcha: resumes from submitted WebView HTML without browser replay", async ({ api, bearerToken, state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: DETAIL_CAPTCHA_SUMMARY_URL },
  });
  expect(submit.status()).toBe(202);
  const payload = await submit.json();

  const pausedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
  });
  expect(pausedJob.status).toBe("awaiting_captcha");

  const ctx = await fetchCaptchaContext(api, payload.job_id, bearerToken);
  const pageHtml = await fs.readFile(DETAIL_HTML_PATH, "utf8");

  await submitCaptchaResume(api, payload.job_id, [], bearerToken, {
    currentUrl: ctx.sefaz_url,
    pageHtml,
  });

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

test("detail-phase captcha: oversized submitted HTML is rejected without resuming job", async ({ api, bearerToken, state }) => {
  const submit = await api.post("/api/v1/receipts", {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { fiscal_url: DETAIL_CAPTCHA_SUMMARY_URL },
  });
  expect(submit.status()).toBe(202);
  const payload = await submit.json();

  const pausedJob = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
    timeoutMs: 60_000,
    intervalMs: 1_000,
  });
  expect(pausedJob.status).toBe("awaiting_captcha");

  const res = await api.post(`/api/v1/jobs/${payload.job_id}/captcha/resume`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: {
      cookies: [],
      current_url: `${SEFAZ_MOCK_BASE}/tax.NET/Sat.NFe.Web/Consultas/Nfe_DetalheCert.aspx?rq=DETAIL_GATE_TOKEN`,
      page_html: "x".repeat(2 * 1024 * 1024 + 1),
    },
  });
  expect(res.status()).toBe(400);
  const body = await res.json();
  expect(body.code).toBe("PAGE_HTML_TOO_LARGE");

  const dbJob = await state.postgres.getJob(payload.job_id);
  expect(dbJob!.status).toBe("awaiting_captcha");
  expect(await state.redis.llen("jullius:scraping:resume")).toBe(0);
});
