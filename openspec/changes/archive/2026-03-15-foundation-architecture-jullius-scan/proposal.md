## Why

Jullius Scan needs a clear foundation before feature implementation so the mobile app, asynchronous scraping backend, and VPS deployment model evolve from the same architectural contract. Defining these decisions now reduces rework around scraping orchestration, receipt persistence, authentication boundaries, and production operations.

## What Changes

- Establish the initial system architecture for Jullius Scan with a Flutter mobile client, Go REST API, PostgreSQL persistence layer, and Docker-based VPS deployment.
- Define the asynchronous receipt scraping flow from QR Code submission through job processing, SEFAZ HTML extraction, normalization, and result retrieval.
- Specify the scraping subsystem expectations for JS-heavy SEFAZ pages using `chromedp`, including failure handling and retriable execution.
- Document infrastructure components and operational boundaries for Traefik routing, container composition, local VPS registry publishing, and GitHub Actions based CI/CD.
- Define initial data and security expectations for Firebase Auth JWT validation, normalized receipt storage, and API ownership of user-scoped access.

## Capabilities

### New Capabilities
- `receipt-ingestion-api`: Mobile clients can authenticate, submit NFC-e QR Code based extraction requests, and query receipt processing results through the Go REST API.
- `async-receipt-scraping`: The backend processes scraping jobs asynchronously with `chromedp`, extracts SEFAZ receipt data from public HTML, and persists normalized receipt, item, and store records.
- `vps-container-platform`: The system can run in Docker Compose on a VPS with Traefik routing, PostgreSQL, backend services, and CI/CD delivery through GitHub Actions to a local VPS registry.

### Modified Capabilities

- None.

## Impact

- Creates the first OpenSpec baseline for backend, mobile integration, scraping orchestration, authentication, database modeling, and deployment.
- Affects future code under the Flutter app, Go API, scraping worker components, database migrations, Docker Compose definitions, Traefik labels, and GitHub Actions workflows.
- Introduces dependencies on PostgreSQL, Firebase Auth JWT verification, `chromedp`, Docker/Docker Compose, Traefik, and VPS-hosted registry infrastructure.
