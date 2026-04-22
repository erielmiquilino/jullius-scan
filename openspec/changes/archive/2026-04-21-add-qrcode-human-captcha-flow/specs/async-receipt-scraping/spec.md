## MODIFIED Requirements

### Requirement: JavaScript-capable receipt extraction
The scraping worker SHALL use a browser automation strategy capable of extracting the final public SEFAZ HTML for NFC-e pages that depend on JavaScript execution, and SHALL pause the job for human captcha resolution when an anti-bot challenge is detected.

#### Scenario: Extract rendered SEFAZ document
- **WHEN** a worker processes a valid SEFAZ NFC-e URL whose content requires JavaScript to fully render
- **THEN** the worker waits for the relevant page content and extracts the rendered document data for parsing

#### Scenario: Handle non-renderable page failure
- **WHEN** the worker cannot obtain the required SEFAZ content within the configured timeout
- **THEN** the job is marked as failed with an error reason suitable for retry or diagnosis

#### Scenario: Pause job on captcha challenge
- **WHEN** the worker encounters a SEFAZ or Cloudflare captcha challenge that prevents direct extraction
- **THEN** the worker captures the current navigated URL and the accumulated browser cookies, persists them linked to the job, transitions the job status to `awaiting_captcha`, and releases the browser process without marking the job as failed

### Requirement: Retry-aware failure handling
The system SHALL support controlled retry behavior for transient scraping failures and for human-assisted captcha resolution without leaving jobs in an indeterminate state.

#### Scenario: Retry transient failure
- **WHEN** a scraping attempt fails due to a transient navigation or browser execution issue
- **THEN** the system can schedule a new attempt according to retry policy while retaining the job history

#### Scenario: Exhaust retry policy
- **WHEN** the configured retry limit of 3 attempts is reached without a successful extraction
- **THEN** the system marks the job as terminally failed and exposes that status to the API

#### Scenario: Resume job after human captcha resolution
- **WHEN** a job in `awaiting_captcha` is resumed with cookies provided by the authenticated client
- **THEN** the worker reinitializes the browser session with those cookies, navigates to the persisted SEFAZ URL, and continues the extraction flow from that point

#### Scenario: Captcha session expired on resume
- **WHEN** the worker resumes a job with cookies and the page still presents a captcha challenge
- **THEN** the worker transitions the job back to `awaiting_captcha` up to one additional time, after which any further challenge causes the job to be marked as failed with reason `captcha_expired`

#### Scenario: Awaiting captcha timeout
- **WHEN** a job has been in `awaiting_captcha` longer than the configured timeout without being resumed
- **THEN** the system marks the job as failed with reason `captcha_timeout`

## ADDED Requirements

### Requirement: Captcha-paused job state
The system SHALL represent jobs that require human-assisted captcha resolution as a distinct lifecycle state with explicit transitions and persisted session context.

#### Scenario: Transition into awaiting_captcha
- **WHEN** the worker detects an anti-bot challenge during extraction
- **THEN** the job status transitions from `processing` to `awaiting_captcha` and the system records the current URL, session cookies, and pending timestamp for the job

#### Scenario: Transition out of awaiting_captcha on resume
- **WHEN** a client resumes the job by supplying validated session cookies
- **THEN** the job transitions from `awaiting_captcha` back to `processing` and becomes eligible for worker dispatch again

#### Scenario: House ownership preserved across pause and resume
- **WHEN** a job is paused and later resumed for captcha resolution
- **THEN** only members of the House that originally submitted the job are able to supply the resume payload, and the house linkage of the job is never changed

## REMOVED Requirements

### Requirement: Capture accepted captcha limitation
**Reason**: Superseded by the human-in-the-loop captcha resolution flow. Captchas no longer cause terminal job failure; they pause the job for the authenticated user to resolve via the mobile WebView.
**Migration**: Replace checks for terminal captcha failure with handling of the new `awaiting_captcha` status and the resume endpoint. Existing jobs previously failed with `reason=captcha` remain historical and are not auto-retried.
