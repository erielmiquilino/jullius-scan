## MODIFIED Requirements

### Requirement: Captcha resolution via in-app WebView
The mobile app SHALL open an embedded WebView with the SEFAZ URL provided by the backend when a job enters the `awaiting_captcha` state, allowing the authenticated user to solve the captcha challenge manually. The opened URL MAY point to the NFC-e summary page or to the NFC-e detail consultation page depending on the phase in which the backend paused the job.

#### Scenario: Job transitions to awaiting_captcha while user is tracking it
- **WHEN** the app is polling the status of a job and receives `awaiting_captcha`
- **THEN** the app fetches the captcha context from the backend and presents a WebView loaded with the provided SEFAZ URL, regardless of whether the pause occurred on the summary page or the detail page

#### Scenario: User cancels captcha resolution
- **WHEN** the user closes or cancels the WebView screen before completing the challenge
- **THEN** the app returns to the job tracking view, the job remains in `awaiting_captcha` on the backend, and the user can retry later until the backend timeout expires

### Requirement: Cookie extraction and submission
The mobile app SHALL extract the full authenticated session cookies (including HttpOnly cookies) from the WebView after the user resolves the challenge and submit them to the backend resume endpoint, triggering resumption from whichever phase (summary or detail) the backend paused at.

#### Scenario: Captcha is detected as resolved
- **WHEN** the WebView finishes navigation to a page that contains the expected NFC-e content markers (either summary or detail page content)
- **THEN** the app extracts all cookies associated with the SEFAZ domain via the native cookie manager and sends them to `POST /api/v1/jobs/:id/captcha/resume` with the authenticated user's bearer token

#### Scenario: Backend rejects the resume request
- **WHEN** the app submits cookies to the resume endpoint and the backend returns an error response
- **THEN** the app surfaces the error to the user with an option to retry or abort, and does not close the WebView until a retry or abort decision is made
