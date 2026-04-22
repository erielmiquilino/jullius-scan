## MODIFIED Requirements

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

## ADDED Requirements

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
