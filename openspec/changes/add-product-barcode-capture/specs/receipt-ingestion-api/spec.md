## ADDED Requirements

### Requirement: Expose item barcode in receipt responses
The API SHALL include an optional `barcode` field for each item in receipt responses, populated with the EAN captured from the SEFAZ detail page when available, or omitted / null when not available.

#### Scenario: Return barcode when captured
- **WHEN** an authenticated House member queries a completed receipt whose items were persisted with a captured EAN
- **THEN** each item in the API response includes a `barcode` field with the stored EAN string

#### Scenario: Omit barcode when unavailable
- **WHEN** an authenticated House member queries a completed receipt whose items have `barcode = NULL` in the database
- **THEN** the API response for each such item either omits the `barcode` field or returns it as `null`, without breaking the response schema for clients that do not yet read the field
