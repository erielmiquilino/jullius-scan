## ADDED Requirements

### Requirement: Receipt submitter attribution
The system SHALL persist, for every newly created receipt, the identity of the House member whose authenticated submission produced the receipt, and SHALL expose that identity in the receipt detail response so that House members can see who recorded each receipt.

#### Scenario: Persist submitter on receipt creation
- **WHEN** the worker successfully completes a scraping job and creates a `receipts` row from that job
- **THEN** the system stores in the receipt the identifier of the House member referenced by `scraping_jobs.submitted_by` of the originating job, so that the receipt remains attributable even if the originating job record is later cleared

#### Scenario: Return submitter in receipt detail
- **WHEN** an authenticated House member requests `GET /api/v1/receipts/{id}` for a receipt that has a recorded submitter belonging to that same House
- **THEN** the API includes a `submitted_by` object in the response containing the submitter's identifier, display name, and email

#### Scenario: Omit submitter when unknown
- **WHEN** an authenticated House member requests `GET /api/v1/receipts/{id}` for a receipt whose submitter cannot be determined (for example, a receipt created before this capability existed and not eligible for backfill)
- **THEN** the API responds successfully and omits the `submitted_by` field rather than returning a placeholder or error

#### Scenario: Listing endpoint not affected
- **WHEN** an authenticated House member requests `GET /api/v1/receipts`
- **THEN** the API responds with the list of receipts in the same shape as before this capability was introduced and SHALL NOT include `submitted_by` per item

## MODIFIED Requirements

### Requirement: House bootstrap and membership management
The system SHALL support an MVP House model backed by administrative provisioning of Houses and memberships for authorized users, where the initial House and owner are created by a one-time SQL seed and additional members are added by an administrative command-line tool that creates the user atomically in Firebase Auth and the application database.

#### Scenario: Resolve manually provisioned House membership
- **WHEN** an authenticated user whose Firebase identity and database membership were provisioned (either by the initial seed or by the administrative provisioning CLI) calls the API
- **THEN** the backend resolves that user's active House context and applies it to receipt access rules

#### Scenario: Reject user without provisioned House
- **WHEN** an authenticated user exists in Firebase but does not have the required House membership provisioned in the database
- **THEN** the API denies House-scoped operations until the provisioning is completed

#### Scenario: New member added via provisioning CLI gains House access
- **WHEN** the administrative provisioning CLI completes successfully for a given email and House, and that user subsequently authenticates against the API
- **THEN** the backend resolves the user's active House context to the House selected during provisioning and grants the same receipt access as any other member of that House
