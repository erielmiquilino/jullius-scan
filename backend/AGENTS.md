# AGENTS.md - backend/

## Module identity

- **Go module**: `github.com/erielfranco/jullius-scan/backend`
- **Go version**: 1.23
- **Entry points**: `cmd/api/main.go` (HTTP server) and `cmd/worker/main.go` (scraping worker)

---

## Directory layout

```
backend/
  cmd/
    api/main.go          <- HTTP server entry point
    worker/main.go       <- Async scraping worker entry point
  internal/
    api/
      router.go          <- chi router setup, middleware chain
      handlers.go        <- HTTP handler methods + request/response types
      middleware/        <- firebase_auth, house_resolver, context, logger
    config/config.go     <- Env-var config loaded at startup
    database/
      db.go              <- pgx connection pool wrapper
      migrate.go         <- golang-migrate runner
      queries.go         <- All SQL query structs (UserQueries, HouseQueries, JobQueries, ReceiptQueries)
    domain/
      models.go          <- Domain types: User, House, HouseMember, Store, Receipt, Item, ScrapingJob
      errors.go          <- Sentinel errors: ErrNotFound, ErrAccessDenied, ErrNoHouseMembership, etc.
    queue/client.go      <- Redis-backed job queue (LPUSH / BRPOP)
    scraper/
      executor.go        <- chromedp headless browser execution
      parser.go          <- HTML scraping / data extraction
      worker.go          <- Worker loop: dequeue, scrape, persist
  migrations/
    001_create_tables.up.sql
    001_create_tables.down.sql
  scripts/
    provision_mvp.sh     <- One-time VPS provisioning
    seed_mvp.sql         <- Manual seed for production
  tools/
    docker-compose.yml   <- Local dev: postgres (5432) + redis (6379)
  Dockerfile.api         <- golang:1.23-alpine -> alpine:3.20 (no browser)
  Dockerfile.worker      <- golang:1.23-alpine -> debian:bookworm-slim + Chromium
  go.mod / go.sum
```

---

## Key dependencies

| Package | Purpose |
|---|---|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/jackc/pgx/v5` | PostgreSQL driver + connection pool |
| `github.com/redis/go-redis/v9` | Redis client |
| `github.com/golang-migrate/migrate/v4` | SQL migration runner |
| `github.com/chromedp/chromedp` | Headless Chromium for SEFAZ scraping |
| `firebase.google.com/go/v4` | Firebase Auth token verification |
| `log/slog` (stdlib) | Structured JSON logging |

---

## Domain model

```
User --< HouseMember >-- House
                            |
                       ScrapingJob --> Receipt --> Store
                                           |
                                          Item[]
```

- A **User** authenticates via Firebase and belongs to one **House** (MVP: one house per user).
- A **ScrapingJob** is created per fiscal URL submission, scoped to a House.
- On success, the job produces a **Receipt** with a **Store** (upserted by CNPJ) and **Item** list.

### Job lifecycle

```
queued -> processing -> completed
                    -> failed  (FailureReason: timeout | captcha | navigation | parsing | unknown)
```

---

## Code patterns - mandatory

### Import order

Always three groups, separated by blank lines:

```go
import (
    // 1. stdlib
    "context"
    "fmt"

    // 2. third-party
    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5"

    // 3. internal
    "github.com/erielfranco/jullius-scan/backend/internal/domain"
)
```

### Constructors

Every struct with dependencies gets a `NewXxx` constructor:

```go
type FooService struct { db *database.DB }
func NewFooService(db *database.DB) *FooService { return &FooService{db: db} }
```

### Query structs

All SQL lives in `internal/database/queries.go` in typed query structs:

```go
type FooQueries struct{ db *DB }
func NewFooQueries(db *DB) *FooQueries { return &FooQueries{db: db} }
func (q *FooQueries) GetByID(ctx context.Context, id int64) (*domain.Foo, error) { ... }
```

Never write raw SQL inline inside handlers or business logic.

### Error handling

```go
// Wrapping - always include context
return fmt.Errorf("find user by firebase_id: %w", err)

// Sentinel errors live in domain/errors.go
var ErrNotFound = errors.New("resource not found")

// Matching
if errors.Is(err, pgx.ErrNoRows) { ... }

// Intentionally ignored - document it
_ = h.jobs.UpdateJobStatus(ctx, id, domain.JobStatusFailed, &reason, "...", nil)
```

Never silently swallow errors without `_ =` assignment and a comment.

### Logging

```go
// Only log/slog - never fmt.Println or log.Printf
slog.Info("receipt submission accepted", "job_id", job.ID, "house_id", houseID)
slog.Error("failed to create scraping job", "error", err, "house_id", houseID)
```

### Context

`context.Context` is always the **first parameter** in any function that touches the DB, Redis, or external services.

### Typed enum aliases

```go
type JobStatus string
const (
    JobStatusQueued     JobStatus = "queued"
    JobStatusProcessing JobStatus = "processing"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
)
```

Use typed string aliases (not plain `string` or `int` iota) for all enum-like domain values.

### Nullable fields

Use pointer types for nullable DB columns:

```go
FailureReason *FailureReason
ReceiptID     *int64
StartedAt     *time.Time
```

### JSON struct tags

- Keys are `snake_case`.
- Use `omitempty` on optional and nullable fields; never on required fields.
- All timestamps serialized as RFC3339 strings — call `.Format("2006-01-02T15:04:05Z07:00")` when building response structs; domain model fields stay as `time.Time`.

```go
// Domain model — raw time.Time
type Receipt struct {
    IssuedAt  time.Time `json:"issued_at"`
    CreatedAt time.Time `json:"created_at"`
}

// Response type — pre-formatted strings + omitempty on optional fields
type JobResponse struct {
    CreatedAt   string  `json:"created_at"`
    StartedAt   *string `json:"started_at,omitempty"`
    CompletedAt *string `json:"completed_at,omitempty"`
}
```

### HTTP handlers

- All handlers are methods on `*Handlers`.
- Use `respondJSON(w, status, data)` and `respondError(w, status, message, code)` helpers - never write `w.WriteHeader` / `json.Encode` directly in handler bodies.
- Every authenticated endpoint must enforce house-scoped access: check `job.HouseID != houseID` (or equivalent) and return 403 if mismatched.
- Route parameters parsed with `parseIDParam(r, "id").`

### Graceful shutdown

Both `cmd/api` and `cmd/worker` use:
`signal.Notify` on `SIGINT`/`SIGTERM` -> `context.Cancel()` -> `srv.Shutdown(ctx)` with a 15-second timeout.

---

## Environment variables

Set these when running locally (copy from `deploy/.env.example`):

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `REDIS_URL` | Redis URL |
| `FIREBASE_PROJECT_ID` | Firebase project for auth token verification |
| `PORT` | HTTP listen port (API only; default 8080) |
| `SEFAZ_URL` | Target SEFAZ URL (worker; override for testing) |

---

## Local development

```bash
# Start backing services (postgres:5432, redis:6379)
docker compose -f backend/tools/docker-compose.yml up -d

# Run the API
go run ./cmd/api

# Run the worker (separate terminal)
go run ./cmd/worker
```

---

## Building and Docker

```bash
# API image (small, no browser)
docker build -f backend/Dockerfile.api -t jullius-api ./backend

# Worker image (Chromium required)
docker build -f backend/Dockerfile.worker -t jullius-worker ./backend
```

- API: `golang:1.23-alpine` build -> `alpine:3.20` runtime (~20 MB).
- Worker: `golang:1.23-alpine` build -> `debian:bookworm-slim` + Chromium runtime (larger; headless browser needed).

---

## Database migrations

Migrations live in `backend/migrations/` and run automatically on startup via `golang-migrate`.

- File naming: `NNN_description.up.sql` / `NNN_description.down.sql`.
- Always provide both `.up` and `.down` files.
- Never edit a committed migration; add a new one instead.

---

## Static checks

There is no linter config. Use these two commands before committing:

```bash
# from backend/
go vet ./...      # catches common bugs (shadowed variables, misused printf, etc.)
gofmt -l .        # lists files that differ from canonical formatting (output must be empty)
```

To auto-format all files: `gofmt -w .`

---

## Testing

There are **no Go unit tests**. All backend testing is done through the E2E suite at `testing/e2e/backend/`. See that directory's `AGENTS.md` for details.

---

## What does NOT exist (yet)

- No `.golangci.yml` or linter config - `go vet` is the only static check.
- No Makefile.
- No Go unit or integration tests.
- No mock generation tooling.
