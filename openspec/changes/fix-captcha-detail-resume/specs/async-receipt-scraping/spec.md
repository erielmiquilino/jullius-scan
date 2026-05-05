## MODIFIED Requirements

### Requirement: Retry-aware failure handling
The system SHALL support controlled retry behavior for transient scraping failures and for human-assisted captcha resolution without leaving jobs in an indeterminate state, including resume from rendered detail HTML submitted by the mobile client.

#### Scenario: Retry transient failure
- **WHEN** a scraping attempt fails due to a transient navigation or browser execution issue
- **THEN** the system can schedule a new attempt according to retry policy while retaining the job history

#### Scenario: Exhaust retry policy
- **WHEN** the configured retry limit of 3 attempts is reached without a successful extraction
- **THEN** the system marks the job as terminally failed and exposes that status to the API

#### Scenario: Resume job after human captcha resolution
- **WHEN** a job in `awaiting_captcha` is resumed with cookies provided by the authenticated client
- **THEN** the worker reinitializes the browser session with those cookies, navigates to the persisted SEFAZ URL, and continues the extraction flow from that point

#### Scenario: Resume detail-phase job from rendered HTML
- **WHEN** a detail-phase captcha job is resumed with rendered NFC-e detail HTML submitted by the authenticated mobile client
- **THEN** the worker parses the submitted detail HTML, merges available barcode data into the persisted summary data, persists the final receipt, and completes the job without requiring another browser navigation

#### Scenario: Submitted detail HTML is invalid
- **WHEN** a detail-phase captcha job is resumed with rendered HTML that cannot be parsed as a detail page
- **THEN** the worker falls back to the existing cookie-based browser resume path instead of silently completing incomplete data

#### Scenario: Captcha session expired on resume
- **WHEN** the worker resumes a job with cookies and the page still presents a captcha challenge
- **THEN** the worker transitions the job back to `awaiting_captcha` up to one additional time, after which any further challenge causes the job to be marked as failed with reason `captcha_expired`

#### Scenario: Awaiting captcha timeout
- **WHEN** a job has been in `awaiting_captcha` longer than the configured timeout without being resumed
- **THEN** the system marks the job as failed with reason `captcha_timeout`
