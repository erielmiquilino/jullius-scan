## ADDED Requirements

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
The scraping worker SHALL use a browser automation strategy capable of extracting the final public SEFAZ HTML for NFC-e pages that depend on JavaScript execution.

#### Scenario: Extract rendered SEFAZ document
- **WHEN** a worker processes a valid SEFAZ NFC-e URL whose content requires JavaScript to fully render
- **THEN** the worker waits for the relevant page content and extracts the rendered document data for parsing

#### Scenario: Handle non-renderable page failure
- **WHEN** the worker cannot obtain the required SEFAZ content within the configured timeout
- **THEN** the job is marked as failed with an error reason suitable for retry or diagnosis

#### Scenario: Capture accepted captcha limitation
- **WHEN** the worker encounters a SEFAZ Captcha or equivalent anti-bot block that prevents extraction
- **THEN** the job is marked as failed with a reason indicating the accepted MVP limitation and no third-party bypass is attempted

### Requirement: Normalized receipt persistence
The system SHALL normalize extracted SEFAZ receipt data into relational records for Houses, receipts, stores, and items.

#### Scenario: Persist successful extraction
- **WHEN** the scraping worker successfully parses a SEFAZ document
- **THEN** the system stores the receipt metadata, totals, associated store, and line items in normalized PostgreSQL tables

#### Scenario: Preserve House ownership on persisted receipt
- **WHEN** a successful extraction is persisted
- **THEN** the resulting receipt record remains linked to the House that initiated the extraction request

### Requirement: Retry-aware failure handling
The system SHALL support controlled retry behavior for transient scraping failures without leaving jobs in an indeterminate state.

#### Scenario: Retry transient failure
- **WHEN** a scraping attempt fails due to a transient navigation or browser execution issue
- **THEN** the system can schedule a new attempt according to retry policy while retaining the job history

#### Scenario: Exhaust retry policy
- **WHEN** the configured retry limit of 3 attempts is reached without a successful extraction
- **THEN** the system marks the job as terminally failed and exposes that status to the API

#### Scenario: Do not auto-retry captcha block
- **WHEN** a scraping attempt fails because the SEFAZ page presents a Captcha or equivalent anti-bot block
- **THEN** the system records a terminal MVP limitation or non-retriable failure instead of invoking a third-party solving service
