## MODIFIED Requirements

### Requirement: Receipt status retrieval
The system SHALL allow an authenticated user to query the status and result of receipt extraction requests available to that user's House, including intermediate states that require user action.

#### Scenario: Return in-progress receipt status
- **WHEN** an authenticated House member queries a receipt extraction request that is still being processed for that same House
- **THEN** the API returns the current processing status without blocking for scraping completion

#### Scenario: Return completed normalized receipt data
- **WHEN** an authenticated House member queries a receipt extraction request that has completed successfully for that same House
- **THEN** the API returns the normalized receipt, store, and item data persisted for that request

#### Scenario: Return awaiting_captcha status
- **WHEN** an authenticated House member queries a job that is paused waiting for human captcha resolution for that same House
- **THEN** the API returns the status `awaiting_captcha` together with the identifier needed to retrieve the captcha context

## ADDED Requirements

### Requirement: Captcha context retrieval
The system SHALL expose an authenticated endpoint that returns the context needed by the mobile client to present the captcha challenge for a job currently in `awaiting_captcha`.

#### Scenario: Retrieve captcha context for own House job
- **WHEN** an authenticated House member requests the captcha context of a job in `awaiting_captcha` belonging to that same House
- **THEN** the API returns the SEFAZ URL to open, the user-agent string used by the worker, and any additional metadata required to initialize the WebView

#### Scenario: Cross-house captcha context access
- **WHEN** an authenticated user requests the captcha context of a job belonging to another House
- **THEN** the API denies the request and does not reveal job metadata

#### Scenario: Captcha context for non-paused job
- **WHEN** an authenticated House member requests captcha context for a job that is not in `awaiting_captcha`
- **THEN** the API responds with a conflict error indicating the job is not waiting for captcha resolution

### Requirement: Captcha resume submission
The system SHALL expose an authenticated endpoint that accepts validated session cookies from the mobile client and re-enqueues the corresponding job for scraping resumption.

#### Scenario: Resume with valid cookies for own House job
- **WHEN** an authenticated House member submits session cookies for a job in `awaiting_captcha` belonging to that same House
- **THEN** the API persists the cookies, transitions the job back to `processing`, enqueues it for worker dispatch, and returns a success response

#### Scenario: Cross-house resume attempt
- **WHEN** an authenticated user submits a resume payload for a job belonging to another House
- **THEN** the API denies the request and does not modify the job

#### Scenario: Resume of non-paused job
- **WHEN** a resume payload is submitted for a job whose status is not `awaiting_captcha`
- **THEN** the API responds with a conflict error and does not mutate the job

#### Scenario: Unauthenticated resume attempt
- **WHEN** a client submits a resume payload without a valid Firebase Auth Bearer token
- **THEN** the API rejects the request and does not alter the job state
