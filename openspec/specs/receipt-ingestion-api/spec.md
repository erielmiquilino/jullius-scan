### Requirement: Authenticated receipt submission
The system SHALL expose a protected API endpoint that accepts an NFC-e QR Code URL from an authenticated mobile client and creates a receipt extraction request within the active House of the authenticated user.

#### Scenario: Create extraction request with valid token
- **WHEN** a mobile client sends a receipt submission request with a valid Firebase Auth Bearer token and a valid NFC-e QR Code URL
- **THEN** the API creates a new extraction request linked to the user's House and returns an identifier with an initial processing status

#### Scenario: Reject unauthenticated submission
- **WHEN** a client sends a receipt submission request without a valid Firebase Auth Bearer token
- **THEN** the API rejects the request and does not create an extraction job

### Requirement: House-based receipt access
The system SHALL isolate receipts and scraping jobs by House so that all members of the same House can access shared receipt data, can delete shared receipt records belonging to that House, and members of other Houses cannot access or delete those records.

#### Scenario: Share receipt across House members
- **WHEN** a receipt extraction request is completed for one member of a House
- **THEN** any authenticated member of that same House can query the shared receipt result

#### Scenario: Prevent cross-house access
- **WHEN** an authenticated user queries a receipt or scraping job belonging to another House
- **THEN** the API denies access to the request data

#### Scenario: Delete receipt from same House
- **WHEN** an authenticated House member sends `DELETE /api/v1/receipts/{id}` for a receipt belonging to that same House
- **THEN** the API removes the persisted receipt data and returns a successful deletion response

#### Scenario: Prevent cross-house receipt deletion
- **WHEN** an authenticated user sends `DELETE /api/v1/receipts/{id}` for a receipt belonging to another House
- **THEN** the API denies the request and does not remove the receipt

### Requirement: Receipt deletion keeps data integrity
The system SHALL delete a receipt in a way that preserves relational integrity for associated items and historical scraping jobs.

#### Scenario: Delete receipt with line items
- **WHEN** an authenticated House member deletes a receipt that has persisted line items
- **THEN** the system removes the receipt and its items without leaving orphaned rows

#### Scenario: Delete receipt with completed job reference
- **WHEN** an authenticated House member deletes a receipt referenced by one or more scraping jobs
- **THEN** the system clears the affected `receipt_id` references and preserves the historical job records

#### Scenario: Delete nonexistent receipt
- **WHEN** an authenticated House member sends `DELETE /api/v1/receipts/{id}` for a receipt that does not exist in that House context
- **THEN** the API responds with a not found error and does not mutate stored data

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

### Requirement: Submission idempotency for repeated fiscal URLs
The system SHALL handle repeated submission of the same NFC-e fiscal URL in a way that avoids creating inconsistent duplicate receipt records within the same House.

#### Scenario: Reuse existing receipt result
- **WHEN** a House member submits a fiscal URL that already has a completed normalized receipt available for that same House
- **THEN** the API returns or links the existing shared receipt result instead of persisting a conflicting duplicate receipt

#### Scenario: Avoid duplicate concurrent jobs
- **WHEN** a House member submits a fiscal URL that already has an active extraction request in progress for that same House
- **THEN** the API returns the existing in-progress request or an equivalent idempotent response

### Requirement: House bootstrap and membership management
The system SHALL support an MVP House model backed by manual administrative provisioning of Houses and memberships for authorized users.

#### Scenario: Resolve manually provisioned House membership
- **WHEN** an authenticated user whose Firebase identity and database membership were provisioned manually calls the API
- **THEN** the backend resolves that user's active House context and applies it to receipt access rules

#### Scenario: Reject user without provisioned House
- **WHEN** an authenticated user exists in Firebase but does not have the required House membership provisioned in the database
- **THEN** the API denies House-scoped operations until the manual setup is completed
