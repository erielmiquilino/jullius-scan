## MODIFIED Requirements

### Requirement: Cookie extraction and submission
The mobile app SHALL extract the full authenticated session cookies (including HttpOnly cookies) from the WebView after the user resolves the challenge and submit them to the backend resume endpoint together with the WebView user agent, current URL, and rendered page HTML when available.

#### Scenario: Captcha is detected as resolved
- **WHEN** the WebView finishes navigation or changes its DOM to a page that contains expected NFC-e summary or detail content markers and no captcha markers
- **THEN** the app extracts all cookies associated with the SEFAZ domain via the native cookie manager and sends them to `POST /api/v1/jobs/:id/captcha/resume` with the authenticated user's bearer token, WebView user agent, current URL, and rendered page HTML

#### Scenario: Backend rejects the resume request
- **WHEN** the app submits cookies and page state to the resume endpoint and the backend returns an error response
- **THEN** the app surfaces the error to the user with an option to retry or abort, and does not close the WebView until a retry or abort decision is made

## ADDED Requirements

### Requirement: Resilient captcha completion detection
The mobile app SHALL detect captcha completion using page URL, title, DOM selectors, and visible text so both NFC-e summary pages and detailed document pages can trigger resume submission.

#### Scenario: Detail page appears after captcha
- **WHEN** the user resolves a SEFAZ captcha and the WebView displays a detailed NFC-e page containing product detail or EAN markers
- **THEN** the app recognizes the captcha as resolved and submits the WebView state to the backend without requiring the user to leave the page manually

#### Scenario: Captcha page remains visible
- **WHEN** the WebView still contains Cloudflare, Turnstile, captcha, or challenge markers
- **THEN** the app does not submit a resume payload automatically

### Requirement: DOM-change monitoring for captcha completion
The mobile app SHALL re-check captcha completion while the WebView remains open, including after DOM mutations that do not trigger a full navigation event.

#### Scenario: SEFAZ replaces challenge content in-place
- **WHEN** the captcha page replaces its body with NFC-e content without firing a normal page-finished navigation callback
- **THEN** the app detects the DOM change during periodic or observer-triggered checks and submits the resume payload

### Requirement: Manual captcha completion fallback
The mobile app SHALL provide a visible manual action that lets the user submit the current WebView session after they confirm the captcha is resolved.

#### Scenario: User confirms solved captcha manually
- **WHEN** the user taps the manual continue action after the final SEFAZ document is visible
- **THEN** the app extracts cookies, user agent, current URL, and page HTML and submits them to the backend resume endpoint

### Requirement: Stuck captcha telemetry
The mobile app SHALL record a non-fatal diagnostic event when the WebView appears to remain on a SEFAZ page for an extended period without being recognized as resolved.

#### Scenario: WebView remains unresolved after several checks
- **WHEN** the WebView has completed multiple checks on a SEFAZ host without captcha markers being resolved or submitted
- **THEN** the app records a non-fatal Crashlytics error with job id, check count, host, path, and detection signals without including cookie values or raw HTML
