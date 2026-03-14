## 1. Backend Foundation

- [x] 1.1 Create the Go service structure for REST API, scraping worker, shared config, and domain modules
- [x] 1.2 Add PostgreSQL connectivity, migration tooling, and normalized schemas for users, houses, house_members, receipts, stores, items, and scraping jobs
- [x] 1.3 Implement Firebase Auth JWT validation middleware plus resolution of the authenticated user's active House in the API layer
- [x] 1.4 Document or script the manual MVP provisioning steps for Firebase users, House membership records, and baseline seed data

## 2. Receipt Ingestion API

- [ ] 2.1 Implement the authenticated endpoint to submit an NFC-e QR Code URL and create an idempotent scraping job scoped to a House
- [ ] 2.2 Implement receipt/job query endpoints that return House-scoped status and completed normalized receipt data for all members
- [ ] 2.3 Add request validation, House access enforcement, and error contracts for invalid tokens, malformed URLs, and cross-house access

## 3. Asynchronous Scraping Flow

- [ ] 3.1 Implement Redis-backed job dispatch plus persisted job lifecycle management with queued, processing, completed, and failed states plus timestamps
- [ ] 3.2 Build the worker execution flow that fetches pending jobs, runs `chromedp` with explicit timeouts, and guarantees browser cleanup on failure
- [ ] 3.3 Configure the worker with a 45-second timeout per job and a maximum of 3 retry attempts for transient failures
- [ ] 3.4 Implement parsing, House-scoped normalization, and persistence of receipt metadata, totals, store, and item records from extracted HTML
- [ ] 3.5 Classify Captcha blocking as an accepted MVP failure mode without integrating third-party solving services

## 4. Platform Infrastructure

- [ ] 4.1 Create Dockerfiles and Docker Compose services for API, scraping worker, PostgreSQL, and Traefik aligned with the existing registry-based delivery flow
- [ ] 4.2 Configure Traefik labels, internal networking, and environment-based runtime configuration for secrets, Redis connectivity, and service settings
- [ ] 4.3 Create GitHub Actions workflows to build images, publish them to `https://registry.skadi.digital/`, and deploy to the VPS over SSH with the existing `.pem` key

## 5. Mobile Integration Baseline

- [ ] 5.1 Create the Flutter app skeleton with Firebase Auth integration and API client configuration
- [ ] 5.2 Implement the initial receipt submission and status polling flow using QR Code URL input as the temporary entry path
- [ ] 5.3 Keep the app aligned with manual MVP provisioning by assuming a preconfigured House context for authorized users
- [ ] 5.4 Add UI states for pending, success, timeout, captcha-blocked, and failed receipt extraction results aligned with backend job statuses

## 6. Verification and Operations

- [ ] 6.1 Add automated tests for auth validation, House membership access rules, receipt submission idempotency, and job lifecycle transitions
- [ ] 6.2 Add automated or integration checks that validate Redis enqueue/consume flow plus `chromedp` timeout cancellation and browser cleanup behavior
- [ ] 6.3 Add structured logging and operational diagnostics for API requests, scraping attempts, retries, timeouts, captcha failures, and terminal failures without storing raw HTML
- [ ] 6.4 Validate end-to-end deployment on the VPS using `https://registry.skadi.digital/`, SSH-based rollout, and the manually provisioned Redis container, then document rollback steps
