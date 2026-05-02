# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

**Jullius Scan** is a personal-use Brazilian NFC-e (fiscal receipt) scanner app. The mobile client submits SEFAZ fiscal URLs to the REST API, which enqueues async scraping jobs; a headless-browser worker scrapes and persists the receipt data.

| Path | Language | Role |
|---|---|---|
| `backend/` | Go 1.23 | REST API + async scraping worker |
| `mobile/` | Dart / Flutter | Android/iOS client |
| `testing/e2e/backend/` | TypeScript / Playwright | Backend integration/E2E tests |
| `deploy/` | Docker Compose | Production deployment |
| `openspec/` | YAML + Markdown | Spec-driven change management |

## Commands

```bash
# Backend — start local backing services (postgres:5432, redis:6379)
docker compose -f backend/tools/docker-compose.yml up -d

# Backend — run locally
cd backend && go run ./cmd/api
cd backend && go run ./cmd/worker

# Backend — provision a House member (Firebase Auth + DB) atomically
# Requires DATABASE_URL, FIREBASE_PROJECT_ID, GOOGLE_APPLICATION_CREDENTIALS.
cd backend && echo 's3cr3t' | go run ./cmd/provision-user \
    --email alice@example.com --password-stdin --house-name "Casa Principal" --yes

# Backend — static checks (run before committing)
cd backend && go vet ./...
cd backend && gofmt -l .      # output must be empty; fix with: gofmt -w .

# E2E tests
cd testing/e2e/backend
npm run env:up                                       # start isolated stack
npm test                                             # full suite
npx playwright test tests/smoke/health.spec.ts       # single file
npx playwright test --grep "happy path"              # by name pattern
npx playwright test tests/receipts                   # subdirectory
npm run env:down

# Mobile
cd mobile && flutter pub get
cd mobile && flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
cd mobile && flutter analyze
cd mobile && flutter test
```

## Architecture

### Backend (Go)

Go module: `github.com/erielfranco/jullius-scan/backend`

Two binaries share the `internal/` package tree:
- `cmd/api` — chi HTTP server with Firebase Auth middleware
- `cmd/worker` — Redis BRPOP loop → chromedp scrape → postgres persist

**Domain model:**
```
User --< HouseMember >-- House
                            |
                       ScrapingJob --> Receipt --> Store
                                           |
                                          Item[]
```

**Job lifecycle:** `queued → processing → completed | failed`
Failure reasons: `timeout | captcha | navigation | parsing | unknown`

**Key internal packages:**
- `internal/api/` — chi router, all HTTP handlers, middleware (firebase_auth, house_resolver)
- `internal/database/queries.go` — all SQL; never write inline SQL outside this file
- `internal/domain/` — domain types and sentinel errors
- `internal/queue/client.go` — Redis LPUSH/BRPOP job queue
- `internal/scraper/` — chromedp executor, HTML parser, worker loop

### E2E test stack

Isolated Docker Compose stack with real postgres, redis, API, worker, and SEFAZ mock server. Ports: API `18080`, postgres `15432`, redis `16379`, SEFAZ mock `18091`.

Test fixtures (`tests/fixtures.ts`) inject `bearerToken`, `api` (Playwright HTTP client), and `state` (DB + Redis helpers with per-test reset). Import `{ test, expect }` from `../fixtures` — never from `@playwright/test` directly.

Always use `pollJobUntilTerminal()` to wait for async jobs; never use `setTimeout` or fixed sleeps.

### Mobile (Flutter)

API URL is compile-time configured via `--dart-define=API_BASE_URL=...` (defaults in `lib/config/api_config.dart`). Firebase Auth (`firebase_core` + `firebase_auth`) requires `google-services.json` (Android) and `GoogleService-Info.plist` (iOS) from Firebase Console.

State management: `StatefulWidget` + `setState()` only — no external state libraries.

## Code conventions

### Go
- Imports: three groups (stdlib / third-party / internal), blank line between each
- Errors: `fmt.Errorf("action: %w", err)` — always include context
- Logging: `log/slog` only — no `fmt.Println`, no `log.Printf`
- Context: first parameter of every function touching DB, Redis, or external services
- Enums: typed string aliases (`type JobStatus string`) — never plain `string` or `int` iota
- Nullable DB columns: pointer types (`*int64`, `*time.Time`)
- JSON keys: `snake_case`; `omitempty` on optional/nullable fields only
- Timestamps: RFC3339 strings in response structs; `time.Time` in domain types
- HTTP handlers: methods on `*Handlers`; use `respondJSON`/`respondError` helpers; never write `w.WriteHeader`/`json.Encode` directly
- House-scoped access: every authenticated endpoint must check `job.HouseID != houseID` and return 403 on mismatch

### TypeScript (E2E tests)
- Module system: CommonJS (`"type": "commonjs"`)
- Always `node:` prefix for built-ins (`import path from "node:path"`)
- Shared types in `src/support/types.ts`
- No linter or formatter config

### Dart/Flutter
- Models: immutable classes with `factory fromJson(Map<String, dynamic>)` and `snake_case` JSON keys
- Enums: Dart enums with `fromString()` factory and `unknown` fallback

## OpenSpec change management

Before writing code for a new feature, create or update a `proposal.md` in `openspec/changes/<name>/`. The full workflow is in `.agent/workflows/`.

`tasks.md` uses `- [ ]` / `- [x]` checkboxes; tick only when genuinely complete. Never delete a completed change directory.

## Infrastructure

**Local dev:** `docker compose -f backend/tools/docker-compose.yml up -d` — postgres + redis, no credentials.

**Production (VPS):** `deploy/docker-compose.yml` — Traefik reverse proxy, image tag from `$IMAGE_TAG`. CI/CD in `.github/workflows/deploy.yml` builds both Docker images on push to `main` (affecting `backend/**` or `deploy/**`), pushes to `registry.skadi.digital`, then SSHes to pull + restart.

**Docker images:**
- API: `golang:1.23-alpine` → `alpine:3.20` (no browser)
- Worker: `golang:1.23-alpine` → `debian:bookworm-slim` + Chromium

## What does NOT exist
- No Go unit tests — all backend testing is E2E only
- No linter config (`.golangci.yml`, `.eslintrc`, `.prettierrc`)
- No Makefile
- `testing/e2e/mobile/` is empty — reserved for future Flutter E2E, do not add files there
- Database migrations run automatically on startup; never edit a committed migration — add a new one

## Language policy
- Design documents and specs in `openspec/` → **Brazilian Portuguese**
- All code, identifiers, comments, commit messages, PR descriptions → **English**
