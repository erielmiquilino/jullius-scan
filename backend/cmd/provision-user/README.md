# provision-user

Administrative CLI to provision a House member atomically: creates the user in
Firebase Auth (when missing), upserts the row in `users`, and inserts the link
in `house_members`. Re-running with the same `--email` + House converges to the
same state without duplicating records.

## Required environment

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Postgres connection string used by the API/worker. |
| `FIREBASE_PROJECT_ID` | Firebase project id (same value used by the API). |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to a Firebase service account JSON with **Firebase Authentication Admin** permission. |

## Flags

| Flag | Description |
|---|---|
| `--email` | Email of the user. Required. Lower-cased on input. |
| `--password` | Password for the new Firebase user. Ignored if the user already exists. |
| `--password-stdin` | Read the password from standard input instead of a flag (mutually exclusive with `--password`). |
| `--name` | Display name. Defaults to the email local-part. |
| `--house-id` | House id to link to (mutually exclusive with `--house-name`). |
| `--house-name` | House name; must match exactly one row in `houses`. |
| `--role` | `member` (default) or `owner`. |
| `--yes` | Skip the interactive confirmation prompt. |

## Usage

Add a new member to `Casa Principal` with a password on the command line:

```bash
DATABASE_URL=postgres://jullius:jullius@localhost:5432/jullius?sslmode=disable \
FIREBASE_PROJECT_ID=jullius-scan \
GOOGLE_APPLICATION_CREDENTIALS=/path/to/firebase-sa.json \
go run ./cmd/provision-user \
    --email alice@example.com \
    --password 's3cr3t' \
    --house-name "Casa Principal" \
    --yes
```

Same call but reading the password from stdin (recommended — keeps the password
out of `ps` and shell history):

```bash
echo 's3cr3t' | go run ./cmd/provision-user \
    --email alice@example.com \
    --password-stdin \
    --house-name "Casa Principal" \
    --yes
```

Re-promote an existing Firebase user that lost the database row (no password
needed because the Firebase user already exists):

```bash
go run ./cmd/provision-user \
    --email alice@example.com \
    --house-id 1 \
    --yes
```

## Behavior notes

- **Idempotent.** Running the same command twice is a no-op on the second pass.
- **Password is set only on creation.** If the email already exists in Firebase,
  the supplied password is ignored and the CLI logs a warning.
- **Partial failure (Firebase OK, Postgres failed).** The CLI does not delete
  the Firebase user on rollback. It logs the new UID and exits non-zero — re-run
  the command to finish provisioning (the Firebase lookup detects the existing
  UID and continues from the database step).
- **House selection is mandatory.** Either `--house-id` or `--house-name` must
  be provided; ambiguous names abort the run.
- **Bootstrap remains separate.** `scripts/seed_mvp.sql` continues to be the
  way to create the initial House and the very first owner. This CLI is for
  every member added afterwards.
