# AGENTS.md — Jullius Scan (root)

## What this repository is

**Jullius Scan** is a personal-use Brazilian NFC-e (fiscal receipt) scanner application.
A user photographs or copies a SEFAZ fiscal URL, the mobile app submits it to the REST API,
the API enqueues an async job, and the worker scrapes the SEFAZ portal with a headless browser
and persists the parsed receipt data.

Components:

| Path | Language | Role |
|---|---|---|
| `backend/` | Go 1.23 | REST API + async scraping worker |
| `mobile/` | Dart / Flutter | Android/iOS mobile client |
| `testing/e2e/backend/` | TypeScript / Playwright | Backend integration/E2E test suite |
| `testing/e2e/mobile/` | — | Reserved for future Flutter E2E — **do not add files here** |
| `deploy/` | Docker Compose | Production deployment manifests |
| `openspec/` | YAML + Markdown | Spec-driven change management artifacts |
| `.agent/workflows/` | Markdown | AI agent workflow instructions |

Each subdirectory has its own **AGENTS.md** with detailed instructions:
- `backend/AGENTS.md` — Go code patterns, dependencies, env vars, build commands
- `testing/e2e/backend/AGENTS.md` — test patterns, fixtures, single-test commands
- `mobile/` — Flutter app; see `mobile/pubspec.yaml` for dependencies

---

## Quick command reference

```bash
# Backend — start local backing services
docker compose -f backend/tools/docker-compose.yml up -d

# Backend — run API / worker locally
go run ./cmd/api          # from backend/
go run ./cmd/worker       # from backend/

# Backend — static checks (no linter config exists; use these)
go vet ./...              # from backend/
gofmt -l .               # list files that differ from canonical format

# E2E tests — full suite
cd testing/e2e/backend
npm run env:up
npm test
npm run env:down

# E2E tests — single file
npx playwright test tests/smoke/health.spec.ts

# E2E tests — single test by name pattern
npx playwright test --grep "happy path"

# E2E tests — a subdirectory
npx playwright test tests/receipts

# Mobile — resolve dependencies
cd mobile && flutter pub get

# Mobile — run on connected device/emulator
cd mobile && flutter run

# Mobile — run with custom API URL
cd mobile && flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080

# Mobile — static analysis
cd mobile && flutter analyze

# Mobile — run tests
cd mobile && flutter test
```

---

## Language and naming conventions

- **Design documents, proposals, and task files** inside `openspec/` are written in **Brazilian Portuguese**.
- **All code** (identifiers, JSON keys, SQL column names, comments, commit messages, PR descriptions) is in **English**.
- Never mix languages inside a single file.

---

## Code style — Go (backend/)

- **Formatting**: `gofmt` — no config, just standard Go formatting.
- **Imports**: three groups separated by blank lines — stdlib / third-party / internal.
- **Error wrapping**: `fmt.Errorf("action: %w", err)` — always include caller context.
- **Logging**: `log/slog` only — no `fmt.Println`, no `log.Printf`.
- **Context**: first parameter of every function that touches DB, Redis, or external services.
- **Enums**: typed string aliases (`type JobStatus string`) — never plain `string` or `int` iota.
- **Nullable DB columns**: pointer types (`*int64`, `*time.Time`, `*FailureReason`).
- **SQL**: all queries in `internal/database/queries.go` typed structs — no inline SQL in handlers.
- **JSON struct tags**: `omitempty` for optional/nullable fields; snake_case keys.
- **Time serialization**: RFC3339 format `"2006-01-02T15:04:05Z07:00"` via `time.Time.Format`.

## Code style — TypeScript (testing/e2e/backend/)

- **Module system**: CommonJS (`"type": "commonjs"` in package.json).
- **tsconfig**: strict mode, ES2022 target, Node module resolution.
- **Node built-ins**: always use `node:` prefix (`import path from "node:path"`).
- **Test imports**: always `{ test, expect }` from `"../fixtures"`, never from `@playwright/test`.
- **Types**: all shared interfaces live in `src/support/types.ts`.
- **Async jobs**: always use `pollJobUntilTerminal()` — never `setTimeout` / fixed sleeps.
- **No linter or formatter** is configured (no eslint, no prettier).

## Code style — Dart/Flutter (mobile/)

- **State management**: `StatefulWidget` with `setState()` — no external state libraries.
- **API configuration**: build-time override via `--dart-define=API_BASE_URL=...`; defaults in `lib/config/api_config.dart`.
- **HTTP client**: `package:http` with Firebase Bearer token injected in `Authorization` header.
- **Models**: immutable classes with `factory fromJson(Map<String, dynamic>)` constructors and `snake_case` JSON keys matching the backend.
- **Enums**: Dart enums with `fromString()` factory and `unknown` fallback for forward compatibility.
- **Polling**: async loop with configurable interval/max duration — no `Timer` or fixed `Future.delayed` chains.
- **Firebase**: `firebase_core` + `firebase_auth` — requires `google-services.json` (Android) and `GoogleService-Info.plist` (iOS) from Firebase Console.
- **Analysis**: default `flutter_lints` rules from `analysis_options.yaml`.

---

## OpenSpec change management workflow

This project uses spec-driven change management with a consistent directory layout.

### Directory layout

```
openspec/
  config.yaml                     ← schema: spec-driven (default)
  changes/
    <change-name>/
      proposal.md                 ← why & what (written first)
      design.md                   ← technical decisions
      tasks.md                    ← checklist of implementation tasks
      specs/                      ← optional: behaviour specs / test plans
```

### AI workflow commands

The same 11 workflow commands are registered in four locations so they work across editors:

| Location | Format |
|---|---|
| `.agent/workflows/` | Markdown (generic agents) |
| `.cursor/commands/` | Markdown (Cursor IDE) |
| `.github/prompts/` | Markdown (GitHub Copilot) |
| `.opencode/command/` | Markdown (OpenCode) |

Key workflow commands:

| Command | What it does |
|---|---|
| `opsx:new <name>` | Scaffold a new change directory |
| `opsx:continue <name>` | Advance to the next unfinished artifact |
| `opsx:status <name>` | Show artifact completion status |
| `opsx:implement <name>` | Begin implementation from tasks.md |

### Rules for working with changes

- Always create or update a `proposal.md` before writing any code for a new feature.
- `tasks.md` uses `- [ ]` / `- [x]` checkboxes; tick items only when the work is genuinely complete.
- Never delete a completed change directory; it is the permanent record.
- `openspec/changes/foundation-architecture-jullius-scan/` and `openspec/changes/add-backend-e2e-test-platform/` are already complete.

---

## Infrastructure overview

### Local development (backend)

```bash
# Start postgres + redis (no app containers)
docker compose -f backend/tools/docker-compose.yml up -d
```

Runs postgres on **5432** and redis on **6379** with no credentials (dev only).

### E2E test stack

```bash
cd testing/e2e/backend
npm run env:up      # start isolated stack (postgres 15432, redis 16379, api 18080, worker, sefaz-mock 18091)
npm run env:down    # tear down
npm test            # run all Playwright tests
```

### Production (VPS)

- `deploy/docker-compose.yml` — Traefik reverse proxy on `proxy-net`, image tag from `$IMAGE_TAG`.
- CI/CD: `.github/workflows/deploy.yml` builds both Docker images on push to `main` affecting `backend/**`
  or `deploy/**`, pushes to `registry.skadi.digital`, then SSHes into VPS to pull + restart.

---

## Commits and PRs

- Commit messages and PR titles/bodies must be in **English**.
- Follow conventional-commit style: `feat:`, `fix:`, `chore:`, `test:`, `docs:`, `refactor:`.
- Never commit `.env` files or secrets.

---

## What does NOT exist (yet)

- No Go unit tests in `backend/` — only E2E tests exist.
- No linter configuration files (`.golangci.yml`, `.eslintrc`, `.prettierrc`).
- No Makefile.
- `testing/e2e/mobile/` is empty on purpose — reserved for future Flutter E2E tests.
