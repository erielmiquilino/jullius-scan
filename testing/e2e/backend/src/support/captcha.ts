import type { APIRequestContext } from "@playwright/test";
import type { CaptchaContextResponse, SessionCookie } from "./types";

export async function fetchCaptchaContext(
  api: APIRequestContext,
  jobId: number,
  bearerToken: string,
): Promise<CaptchaContextResponse> {
  const res = await api.get(`/api/v1/jobs/${jobId}/captcha`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  if (!res.ok()) {
    throw new Error(`fetchCaptchaContext failed: ${res.status()} ${await res.text()}`);
  }
  return res.json() as Promise<CaptchaContextResponse>;
}

export async function submitCaptchaResume(
  api: APIRequestContext,
  jobId: number,
  cookies: SessionCookie[],
  bearerToken: string,
): Promise<void> {
  const res = await api.post(`/api/v1/jobs/${jobId}/captcha/resume`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
    data: { cookies },
  });
  if (!res.ok()) {
    throw new Error(`submitCaptchaResume failed: ${res.status()} ${await res.text()}`);
  }
}

// solvedCookies returns the mock cookie payload that causes the SEFAZ mock to
// serve the real NFC-e HTML instead of the captcha challenge page.
export function solvedCookies(sefazHost: string): SessionCookie[] {
  return [
    {
      name: "e2e_captcha_solved",
      value: "1",
      domain: sefazHost,
      path: "/",
      http_only: false,
      secure: false,
    },
  ];
}
