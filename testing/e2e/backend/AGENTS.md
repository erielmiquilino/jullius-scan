# AGENTS.md - testing/e2e/backend/

## What this suite tests

Backend integration / E2E tests for Jullius Scan's REST API and async scraping worker.
Tests run against a fully isolated Docker stack that includes a real PostgreSQL database,
Redis queue, the Go API binary, the Go worker binary, and a local SEFAZ mock server.
No mocking inside the product code - all integration is real.

---

## Technology stack

| Tool | Role |
|---|---|
| `@playwright/test` | Test runner and assertion library |
| TypeScript 5 | Test language (CommonJS module system) |
| `pg` (node-postgres) | Direct SQL assertions against the test database |
| `ioredis` | Direct Redis assertions against the test queue |
| Firebase REST API | Real Firebase JWT tokens via `fetch` — no Firebase SDK installed |
| Docker Compose | Isolated test stack lifecycle |

---

## Directory layout

```
testing/e2e/backend/
  mocks/sefaz/
    mock-server.js                  <- Express server simulating the SEFAZ portal (port 18091)
    nfce-consulta-detalhada.html    <- Realistic NFC-e HTML page returned by the mock
    captcha.html                    <- Captcha page for failure-path testing
  src/
    config.ts                       <- All env-var config (baseUrl, pgUrl, redisUrl, sefazMockInternalUrl)
    global-setup.ts                 <- Playwright globalSetup: waits for stack, seeds base user/house
    scripts/
      seed.ts                       <- seedBaseUserAndHouse() - inserts test user + house
      wait-for-stack.ts             <- Polls API /health until ready
    support/
      types.ts                      <- Shared TS interfaces (JobResponse, ReceiptResponse, etc.)
      firebase.ts                   <- signInWithFirebaseRest() - gets real ID token via REST API
      polling.ts                    <- pollJobUntilTerminal() - polls GET /jobs/:id until done
      postgres.ts                   <- PostgresHelper - getJob(), getReceipt(), getStore(), getItems()
      redis.ts                      <- RedisHelper - queue inspection helpers
      stack.ts                      <- prepareState() / cleanupState() - per-test setup/teardown
  tests/
    fixtures.ts                     <- Playwright fixture extension: bearerToken, api, state
    smoke/
      health.spec.ts                <- GET /health smoke test
    receipts/
      happy-path.spec.ts            <- Full receipt scan success flow
      failure.spec.ts               <- Scraping failure scenarios
      retrieval.spec.ts             <- GET /receipts and GET /jobs idempotency / retrieval
  docker-compose.e2e.yml           <- Isolated test stack definition
  playwright.config.ts             <- Playwright config (single worker, no retries, 60s timeout)
  tsconfig.json                    <- strict, ES2022, commonjs
  package.json
  .env                             <- test env vars (not committed)
  README.md
  testing-boundary.md              <- Scope and non-scope of this test suite
```

---

## Running the tests

```bash
cd testing/e2e/backend

# Start the isolated stack
npm run env:up

# Run all tests
npm test

# Run a single test file
npx playwright test tests/smoke/health.spec.ts
npx playwright test tests/receipts/happy-path.spec.ts

# Run tests matching a name pattern (--grep accepts a regex)
npx playwright test --grep "happy path"
npx playwright test --grep "failure"

# Run all tests in a subdirectory
npx playwright test tests/receipts

# Tear down
npm run env:down

# View HTML report from the last run
npm run report
```

The stack exposes:
- API on port **18080**
- PostgreSQL on port **15432**
- Redis on port **16379**
- SEFAZ mock on port **18091**

---

## Code patterns - mandatory

### Test file structure

- One `test(...)` call per file - no `describe` blocks.
- Always import `{ test, expect }` from `../fixtures` (never from `@playwright/test` directly).
- Import `{ config }` from `../../src/config` for all env-dependent values.

```ts
// Correct
import { test, expect } from "../fixtures";
import { config } from "../../src/config";

// Wrong - never import directly from @playwright/test in test files
import { test, expect } from "@playwright/test"; // WRONG
```

### Fixtures

Every test receives three fixtures from `tests/fixtures.ts`:

| Fixture | Type | What it provides |
|---|---|---|
| `bearerToken` | `string` | Real Firebase ID token for the seeded test user |
| `api` | `APIRequestContext` | Playwright HTTP client pointed at the API base URL |
| `state` | `PreparedState` | DB + Redis helpers; resets and re-seeds all data before each test |

The `state` fixture calls `prepareState()` which:
1. Truncates all tables in the test database (RESTART IDENTITY CASCADE)
2. Flushes all Redis keys
3. Runs `seedBaseUserAndHouse()` to insert the canonical test user + house
4. Returns open `pg.Client` and `Redis` connections for SQL/cache assertions

After each test, `cleanupState()` closes both connections.

### Waiting for async jobs

Always use `pollJobUntilTerminal()` - never use `setTimeout` sleeps:

```ts
const job = await pollJobUntilTerminal(api, payload.job_id, bearerToken, {
  timeoutMs: 30_000,
  intervalMs: 1_000,
});
expect(job.status).toBe("completed");
```

The function polls `GET /api/v1/jobs/:id` until status is `completed` or `failed`, then returns the final `JobResponse`.

### Direct DB assertions

After a job completes, assert the persisted state directly in the DB via `state.postgres`:

```ts
const dbJob = await state.postgres.getJob(payload.job_id);
expect(dbJob?.status).toBe("completed");
expect(dbJob?.receipt_id).toBeTruthy();

const receipt = await state.postgres.getReceipt(dbJob!.receipt_id!);
expect(receipt?.fiscal_url).toBe(config.sefazMockInternalUrl);

const items = await state.postgres.getItemsByReceiptId(receipt!.id);
expect(items.length).toBeGreaterThan(0);
```

Available `PostgresHelper` methods:
- `getJob(jobId)` - fetch scraping_jobs row
- `getLatestJob()` - fetch most recent scraping_jobs row
- `getReceipt(receiptId)` - fetch receipts row
- `getStore(storeId)` - fetch stores row
- `getItemsByReceiptId(receiptId)` - fetch all items for a receipt
- `countRows(tableName)` - row count for any table

### Node imports

Always use the `node:` prefix for built-in modules:

```ts
import path from "node:path"; // correct
import path from "path"; // wrong
```

### TypeScript

- Module system: CommonJS (type commonjs in package.json)
- tsconfig.json: strict mode, ES2022 target, Node module resolution
- All types shared across tests live in `src/support/types.ts`

---

## How to add a new test

1. Create a new file in the appropriate subdirectory of `tests/`:
   - `tests/smoke/` for basic API availability checks
   - `tests/receipts/` for receipt submission / retrieval flows
   - Create a new subdirectory if the feature does not fit existing categories

2. Use the standard test skeleton:

```ts
import { test, expect } from "../fixtures";
import { config } from "../../src/config";
import { pollJobUntilTerminal } from "../../src/support/polling";
import type { SubmitReceiptResponse } from "../../src/support/types";

test("description of what this test verifies", async ({ api, bearerToken, state }) => {
  // 1. Submit / trigger the action
  // 2. Wait for async side effects (pollJobUntilTerminal if async job involved)
  // 3. Assert HTTP response shape
  // 4. Assert DB state via state.postgres.*
});
```

3. Do not add a test that requires changes to docker-compose.e2e.yml without also
   updating README and env:up / env:down scripts if ports change.

---

## SEFAZ mock server

Located at `mocks/sefaz/mock-server.js`. It is a plain Express server that:

- Serves the realistic `nfce-consulta-detalhada.html` page at the URL configured in SEFAZ_URL
- Can be pointed at the captcha page to trigger failure scenarios
- Always returns the same fixture data (store: SUPERMERCADOS MYATA, CNPJ: 75492694000201)

When writing new tests that need different SEFAZ HTML, add a new fixture HTML file in
`mocks/sefaz/` and a new route in the mock server. Do not modify the existing
`nfce-consulta-detalhada.html` fixture - it is shared across all happy-path tests.

---

## Environment configuration

The `.env` file is read by `src/config.ts`. Required variables:

| Variable | Description |
|---|---|
| `BASE_URL` | API base URL (default: http://localhost:18080) |
| `PG_URL` | Test database DSN |
| `REDIS_URL` | Test Redis URL |
| `SEFAZ_MOCK_INTERNAL_URL` | URL visible to the worker container |
| `FIREBASE_API_KEY` | Firebase REST API key |
| `FIREBASE_TEST_EMAIL` | Email of the seeded test Firebase user |
| `FIREBASE_TEST_PASSWORD` | Password of the seeded test Firebase user |

Do not commit `.env`. Copy from the project owner.

---

## What does NOT exist (yet)

- No Flutter / mobile E2E tests - `testing/e2e/mobile/` is empty and reserved; do not add files there.
- No contract tests.
- No performance or load tests.
- No linter or formatter config (no eslint, no prettier).
