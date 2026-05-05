## 1. Backend resume contract

- [x] 1.1 Extend captcha resume request parsing to accept optional `current_url` and `page_html` without logging raw HTML or cookie values.
- [x] 1.2 Persist resume page state in the scraping job model and database so the worker can consume it after enqueue.
- [x] 1.3 Validate resume page HTML size and reject oversized payloads without mutating the job.

## 2. Worker detail resume

- [x] 2.1 Complete detail-phase captcha jobs from submitted rendered detail HTML when it parses successfully.
- [x] 2.2 Keep the existing cookie-based browser resume path as fallback when submitted HTML is absent or invalid.
- [x] 2.3 Clear or avoid exposing submitted page HTML after job completion.

## 3. Mobile WebView reliability

- [x] 3.1 Expand the NFC-e content detector to recognize summary and detail pages and reject challenge/captcha pages.
- [x] 3.2 Add periodic and DOM-mutation-triggered checks while the WebView is open.
- [x] 3.3 Submit cookies, user agent, current URL, and rendered page HTML to the resume endpoint.
- [x] 3.4 Add a manual "Já resolvi, continuar" fallback action that submits the current WebView state.
- [x] 3.5 Record non-fatal Crashlytics telemetry when a SEFAZ page remains unresolved after repeated checks.

## 4. Tests and validation

- [x] 4.1 Add or update backend E2E coverage for detail-phase resume from submitted HTML and oversized HTML rejection.
- [x] 4.2 Run backend formatting/static checks.
- [x] 4.3 Run relevant backend E2E tests.
- [x] 4.4 Run Flutter analysis/tests for mobile changes.
