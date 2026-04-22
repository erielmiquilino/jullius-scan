### Requirement: Asynchronous scraping lifecycle
The system SHALL process NFC-e extraction outside the request-response cycle using a persisted job lifecycle with explicit processing states.

#### Scenario: Queue job after API acceptance
- **WHEN** the API accepts a valid receipt submission
- **THEN** the system persists a scraping job in an initial queued or pending state for asynchronous processing

#### Scenario: Transition job across lifecycle states
- **WHEN** a worker starts, completes, or fails a scraping execution
- **THEN** the system records the corresponding lifecycle state transition and timestamps for that job

### Requirement: Redis-backed job queue
The system SHALL use Redis as the queueing mechanism between the API and the scraping worker for MVP job dispatch.

#### Scenario: Enqueue accepted scraping job
- **WHEN** the API accepts a valid receipt submission
- **THEN** it publishes or enqueues the scraping job to Redis for asynchronous worker consumption

#### Scenario: Consume job from shared internal Redis
- **WHEN** the worker is connected to the Redis instance provisioned on the VPS internal network
- **THEN** it consumes pending scraping jobs without requiring polling from PostgreSQL as the primary dispatch mechanism

### Requirement: Bounded browser execution
The scraping worker SHALL run each `chromedp` execution with explicit timeouts and guaranteed cleanup so failed jobs do not leave orphaned browser processes.

#### Scenario: Cancel job after total timeout
- **WHEN** a scraping job exceeds the configured total execution timeout of 45 seconds
- **THEN** the worker cancels the Go context, terminates the browser execution, and marks the job as failed due to timeout

#### Scenario: Cleanup browser on navigation failure
- **WHEN** browser navigation or rendering fails before extraction completes
- **THEN** the worker closes the related browser process or context before releasing the job attempt

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

### Requirement: Normalized receipt persistence
The system SHALL normalize extracted SEFAZ receipt data into relational records for Houses, receipts, stores, and items.

#### Scenario: Persist successful extraction
- **WHEN** the scraping worker successfully parses a SEFAZ document
- **THEN** the system stores the receipt metadata, totals, associated store, and line items in normalized PostgreSQL tables

#### Scenario: Preserve House ownership on persisted receipt
- **WHEN** a successful extraction is persisted
- **THEN** the resulting receipt record remains linked to the House that initiated the extraction request

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
