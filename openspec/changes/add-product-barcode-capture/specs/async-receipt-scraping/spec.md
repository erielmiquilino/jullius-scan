## ADDED Requirements

### Requirement: Two-phase SEFAZ extraction with detail page EAN capture
The scraping worker SHALL, after successfully rendering and parsing the NFC-e summary page, navigate to the SEFAZ detail consultation page within the same browser session and extract the product EAN (barcode) for each item when available.

#### Scenario: Capture EAN from detail page on success
- **WHEN** the worker parses the NFC-e summary page without error and the detail page link is present
- **THEN** the worker navigates to the detail page in the same browser session, extracts the EAN Comercial of each item, and persists the receipt with `barcode` populated for items that have one

#### Scenario: Detail page unreachable or parse fails
- **WHEN** the worker fails to navigate to the detail page (timeout, navigation error) or cannot parse EAN blocks
- **THEN** the worker logs a warning, persists the receipt and items with `barcode = NULL`, and marks the job as `completed` — detail extraction failure SHALL NOT fail an otherwise successful summary extraction

#### Scenario: Detail and summary item counts diverge
- **WHEN** the number of item rows parsed from the detail page differs from the number parsed from the summary page
- **THEN** the worker logs a warning, persists all items with `barcode = NULL`, and does not attempt partial merge

### Requirement: Captcha pause on detail page transition
The scraping worker SHALL detect anti-bot captcha challenges on both the summary page and the summary → detail transition, pausing the job in `awaiting_captcha` with the correct resume URL and phase for each case.

#### Scenario: Captcha detected on detail page navigation
- **WHEN** the worker navigates from summary to detail and encounters a captcha challenge (SEFAZ or Cloudflare) before the detail content renders
- **THEN** the worker captures the detail page URL and current browser cookies, persists them linked to the job with `captcha_phase = 'detail'` and `captcha_current_url` set to the detail URL, and transitions the job to `awaiting_captcha`

#### Scenario: Persist parsed summary before detail pause
- **WHEN** the worker pauses a job for captcha during the summary → detail transition
- **THEN** the worker persists the already-parsed summary payload (store, receipt metadata, items without barcode) in `scraping_jobs.parsed_summary` so that on resume the summary does not need to be re-scraped

### Requirement: Resume awareness of captcha phase
The scraping worker SHALL read the persisted `captcha_phase` when resuming a job from `awaiting_captcha` and choose the correct entry URL and processing path accordingly.

#### Scenario: Resume in summary phase
- **WHEN** the worker resumes a job with `captcha_phase = 'summary'` or `captcha_phase IS NULL`
- **THEN** the worker navigates to the original `fiscal_url`, parses the summary page, and continues to the detail phase as in a fresh execution

#### Scenario: Resume in detail phase
- **WHEN** the worker resumes a job with `captcha_phase = 'detail'` and `parsed_summary IS NOT NULL`
- **THEN** the worker navigates directly to `captcha_current_url` (the detail page URL) using the resumed cookies, skips re-parsing the summary, and merges the detail-page EANs onto the persisted summary payload before persisting the full receipt

## MODIFIED Requirements

### Requirement: Normalized receipt persistence
The system SHALL normalize extracted SEFAZ receipt data into relational records for Houses, receipts, stores, and items, including a nullable `barcode` (EAN) field per item when captured from the SEFAZ detail page.

#### Scenario: Persist successful extraction
- **WHEN** the scraping worker successfully parses a SEFAZ document
- **THEN** the system stores the receipt metadata, totals, associated store, and line items in normalized PostgreSQL tables

#### Scenario: Preserve House ownership on persisted receipt
- **WHEN** a successful extraction is persisted
- **THEN** the resulting receipt record remains linked to the House that initiated the extraction request

#### Scenario: Persist item barcode when captured
- **WHEN** the detail page extraction provides an EAN for a given item position
- **THEN** the system persists that EAN in the `barcode` column of the corresponding `receipt_items` row

#### Scenario: Persist NULL barcode when unavailable
- **WHEN** an item has no EAN on the detail page, or detail page extraction failed entirely
- **THEN** the system persists `barcode = NULL` for that item without aborting the receipt persistence

### Requirement: Captcha-paused job state
The system SHALL represent jobs that require human-assisted captcha resolution as a distinct lifecycle state with explicit transitions, persisted session context, and an explicit phase marker distinguishing summary-page pauses from detail-page pauses.

#### Scenario: Transition into awaiting_captcha on summary phase
- **WHEN** the worker detects an anti-bot challenge while loading or rendering the NFC-e summary page
- **THEN** the job status transitions from `processing` to `awaiting_captcha`, and the system records `captcha_phase = 'summary'`, the current URL, session cookies, and pending timestamp for the job

#### Scenario: Transition into awaiting_captcha on detail phase
- **WHEN** the worker detects an anti-bot challenge while navigating from summary to the NFC-e detail page
- **THEN** the job status transitions from `processing` to `awaiting_captcha`, and the system records `captcha_phase = 'detail'`, the detail page URL, session cookies, the already-parsed summary payload, and pending timestamp

#### Scenario: Transition out of awaiting_captcha on resume
- **WHEN** a client resumes the job by supplying validated session cookies
- **THEN** the job transitions from `awaiting_captcha` back to `processing` and becomes eligible for worker dispatch again, preserving the recorded `captcha_phase`

#### Scenario: House ownership preserved across pause and resume
- **WHEN** a job is paused and later resumed for captcha resolution
- **THEN** only members of the House that originally submitted the job are able to supply the resume payload, and the house linkage of the job is never changed
