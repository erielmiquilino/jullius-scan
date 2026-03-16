# Backend E2E

This workspace contains the backend E2E automation for Jullius Scan.

It is intentionally separate from `backend/` and from the reserved future path `testing/e2e/mobile/`.

## What It Covers

- API health and authenticated HTTP flows
- Redis-backed job creation and consumption
- PostgreSQL persistence checks with direct SQL access
- Local SEFAZ mock responses for deterministic scraping scenarios
- Real Firebase authentication through the Firebase REST API

## Required Environment Variables

Copy `.env.example` to `.env` before running the suite.

Required credentials contract:

- `FIREBASE_PROJECT_ID`
- `FIREBASE_API_KEY`
- `FIREBASE_TEST_USER_EMAIL`
- `FIREBASE_TEST_USER_PASSWORD`
- `FIREBASE_UUID`
- `FIREBASE_CREDENTIALS_JSON` (recommended for containerized token verification by the Go API)

The suite signs in through the Firebase REST API, obtains a real JWT, and reuses that token against the Go API. No auth bypass is introduced in the backend product.

For containerized E2E runs, the API should receive Firebase service credentials through `FIREBASE_CREDENTIALS_JSON` so the Admin SDK can verify real ID tokens inside Docker.

## Main Commands

- `npm run env:up` - start the isolated E2E stack and wait for healthchecks
- `npm run env:wait` - verify API, Postgres, Redis, and SEFAZ mock readiness
- `npm run test:smoke` - run smoke tests only
- `npm run test:full` - run the full backend E2E suite
- `npm run report` - open the Playwright HTML report
- `npm run env:logs` - inspect container logs from the E2E stack
- `npm run env:down` - stop and remove the E2E stack

## Local SEFAZ Mock

The deterministic scraping scenarios do not call the public SEFAZ.

- Mock assets live under `mocks/sefaz/`
- `mocks/sefaz/nfce-consulta-detalhada.html` is the happy-path fixture consumed by the worker
- `mocks/sefaz/captcha.html` simulates a captcha/anti-bot failure mode
- `mocks/sefaz/mock-server.js` exposes those files inside the E2E Docker Compose stack

The worker should use the internal URL `http://sefaz-mock:8091/nfce-consulta-detalhada.html` during tests.

## Typical Workflow

1. Copy `.env.example` to `.env`
2. Run `npm run env:up`
3. Run `npm run test:smoke`
4. Run `npm run test:full`
5. If needed, inspect logs with `npm run env:logs`
6. Open artifacts with `npm run report`
7. Tear down with `npm run env:down`

## Future Mobile E2E Boundary

The directory `testing/e2e/mobile/` is reserved for a future Flutter mobile E2E workspace.
Do not add mobile automation dependencies, fixtures, or commands to this backend workspace.
