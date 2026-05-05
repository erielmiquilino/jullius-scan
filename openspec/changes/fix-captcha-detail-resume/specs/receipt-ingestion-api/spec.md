## MODIFIED Requirements

### Requirement: Captcha resume submission
The system SHALL expose an authenticated endpoint that accepts validated session cookies and optional WebView page state from the mobile client and re-enqueues or completes the corresponding job for scraping resumption.

#### Scenario: Resume with valid cookies for own House job
- **WHEN** an authenticated House member submits session cookies for a job in `awaiting_captcha` belonging to that same House
- **THEN** the API persists the cookies, optional user agent, optional current URL, and optional rendered page HTML, transitions the job back to `processing`, enqueues it for worker dispatch, and returns a success response

#### Scenario: Resume with rendered detail HTML
- **WHEN** an authenticated House member submits a resume payload for a detail-phase captcha job with rendered page HTML from the WebView
- **THEN** the API accepts the page state without exposing it in API responses and makes it available only to the worker resuming that job

#### Scenario: Cross-house resume attempt
- **WHEN** an authenticated user submits a resume payload for a job belonging to another House
- **THEN** the API denies the request and does not modify the job

#### Scenario: Resume of non-paused job
- **WHEN** a resume payload is submitted for a job whose status is not `awaiting_captcha`
- **THEN** the API responds with a conflict error and does not mutate the job

#### Scenario: Unauthenticated resume attempt
- **WHEN** a client submits a resume payload without a valid Firebase Auth Bearer token
- **THEN** the API rejects the request and does not alter the job state

#### Scenario: Oversized rendered HTML
- **WHEN** a client submits rendered page HTML that exceeds the backend size limit
- **THEN** the API rejects the resume request with a validation error and does not mutate the job
