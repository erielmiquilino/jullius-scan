## ADDED Requirements

### Requirement: Authenticated receipt submission
The system SHALL expose a protected API endpoint that accepts an NFC-e QR Code URL from an authenticated mobile client and creates a receipt extraction request within the active House of the authenticated user.

#### Scenario: Create extraction request with valid token
- **WHEN** a mobile client sends a receipt submission request with a valid Firebase Auth Bearer token and a valid NFC-e QR Code URL
- **THEN** the API creates a new extraction request linked to the user's House and returns an identifier with an initial processing status

#### Scenario: Reject unauthenticated submission
- **WHEN** a client sends a receipt submission request without a valid Firebase Auth Bearer token
- **THEN** the API rejects the request and does not create an extraction job

### Requirement: House-based receipt access
The system SHALL isolate receipts and scraping jobs by House so that all members of the same House can access shared receipt data and members of other Houses cannot.

#### Scenario: Share receipt across House members
- **WHEN** a receipt extraction request is completed for one member of a House
- **THEN** any authenticated member of that same House can query the shared receipt result

#### Scenario: Prevent cross-house access
- **WHEN** an authenticated user queries a receipt or scraping job belonging to another House
- **THEN** the API denies access to the request data

### Requirement: Receipt status retrieval
The system SHALL allow an authenticated user to query the status and result of receipt extraction requests available to that user's House.

#### Scenario: Return in-progress receipt status
- **WHEN** an authenticated House member queries a receipt extraction request that is still being processed for that same House
- **THEN** the API returns the current processing status without blocking for scraping completion

#### Scenario: Return completed normalized receipt data
- **WHEN** an authenticated House member queries a receipt extraction request that has completed successfully for that same House
- **THEN** the API returns the normalized receipt, store, and item data persisted for that request

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
