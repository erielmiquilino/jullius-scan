## 1. Workspace Foundation

- [x] 1.1 Create a dedicated backend E2E workspace outside `backend/` with its own `package.json`, TypeScript config, and Playwright config
- [x] 1.2 Define the repository folder convention that reserves a parallel path for a future mobile E2E suite without sharing the same suite root
- [x] 1.3 Add backend-E2E-specific scripts for install, run, smoke run, and report viewing without coupling them to backend runtime commands
- [x] 1.4 Install and configure the TypeScript workspace dependencies needed for autonomous state control, including `pg` and `ioredis`

## 2. Test Environment Orchestration

- [x] 2.1 Create an isolated E2E environment definition that starts PostgreSQL, Redis, API, worker, and a simple SEFAZ mock server for local and CI execution
- [x] 2.2 Move `nfce-consulta-detalhada.html` from the repository root into the backend E2E workspace (for example `mocks/sefaz/`) and expose it through the mock server so the Go worker can fetch it via a local URL during tests
- [x] 2.3 Implement startup and readiness checks so the suite only begins after the API, worker, PostgreSQL, and Redis are available

## 3. Fixtures and Test Utilities

- [x] 3.1 Implement reset and seed utilities for PostgreSQL and Redis so each run starts from a known state
- [x] 3.2 Create Playwright fixtures that authenticate against the real Firebase REST API using `FIREBASE_API_KEY`, `FIREBASE_TEST_USER_EMAIL`, `FIREBASE_TEST_USER_PASSWORD`, and `FIREBASE_UUID`, then reuse the resulting JWT in API calls without changing the Go product code
- [x] 3.3 Add centralized polling helpers that wait for async job completion and emit diagnostics on timeout or terminal failure
- [x] 3.4 Add direct PostgreSQL and Redis helpers in TypeScript using `pg` and `ioredis` for reset, seed, queue inspection, and preloading deterministic test state

## 4. Backend E2E Scenarios

- [x] 4.1 Implement a happy-path E2E test that submits a receipt request pointing to the local SEFAZ mock URL, verifies job processing, and asserts normalized persistence in PostgreSQL
- [x] 4.2 Implement an E2E test that validates completed job and receipt retrieval through the API for the expected House context
- [x] 4.3 Implement at least one failure-oriented E2E test covering async diagnostics or non-terminal processing issues in a deterministic way

## 5. Reporting and Documentation

- [x] 5.1 Configure backend-E2E-specific reports, traces, and failure artifacts for local debugging and CI retention
- [x] 5.2 Document how to bootstrap, run, and troubleshoot the backend E2E suite plus its isolation from the main backend module
- [x] 5.3 Document the real Firebase test credentials contract, required environment variables, and the role of the local SEFAZ mock server in deterministic runs
- [x] 5.4 Document the architectural boundary for a future Flutter mobile E2E workspace so new suites are added in parallel rather than merged into backend E2E
