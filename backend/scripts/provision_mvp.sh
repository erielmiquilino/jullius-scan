#!/bin/bash
# provision_mvp.sh
# Manual MVP provisioning helper for Jullius Scan.
#
# This script:
# 1. Runs database migrations
# 2. Executes the seed SQL to create initial House, users, and memberships
#
# Prerequisites:
# - PostgreSQL is running and accessible
# - Firebase users have been created in Firebase Console
# - seed_mvp.sql has been updated with real Firebase UIDs
#
# Usage:
#   export DATABASE_URL="postgres://jullius:jullius@localhost:5432/jullius?sslmode=disable"
#   ./scripts/provision_mvp.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

DATABASE_URL="${DATABASE_URL:?DATABASE_URL environment variable is required}"
MIGRATIONS_DIR="${ROOT_DIR}/migrations"
SEED_FILE="${SCRIPT_DIR}/seed_mvp.sql"

echo "=== Jullius Scan MVP Provisioning ==="
echo ""

# Step 1: Check prerequisites
echo "[1/3] Checking prerequisites..."
if ! command -v psql &> /dev/null; then
    echo "ERROR: psql is not installed or not in PATH"
    exit 1
fi

echo "  - psql found: $(which psql)"
echo "  - Database URL: ${DATABASE_URL%%@*}@***"
echo ""

# Step 2: Run migrations (if golang-migrate CLI is available)
echo "[2/3] Running database migrations..."
if command -v migrate &> /dev/null; then
    migrate -path "$MIGRATIONS_DIR" -database "$DATABASE_URL" up
    echo "  - Migrations applied successfully"
else
    echo "  - 'migrate' CLI not found, skipping auto-migration"
    echo "  - Migrations will run on API startup instead"
fi
echo ""

# Step 3: Run seed data
echo "[3/3] Applying seed data..."
if [ ! -f "$SEED_FILE" ]; then
    echo "ERROR: Seed file not found: $SEED_FILE"
    exit 1
fi

# Check if placeholder values are still present
if grep -q 'REPLACE_WITH_FIREBASE_UID' "$SEED_FILE"; then
    echo ""
    echo "WARNING: seed_mvp.sql still contains placeholder Firebase UIDs!"
    echo "Please update the following before running:"
    echo "  - REPLACE_WITH_FIREBASE_UID_1 -> your Firebase UID"
    echo "  - owner@example.com -> your email"
    echo "  - Owner Name -> your name"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted. Update seed_mvp.sql and try again."
        exit 1
    fi
fi

psql "$DATABASE_URL" -f "$SEED_FILE"
echo ""
echo "=== Provisioning complete ==="
echo ""
echo "Next steps:"
echo "  1. Verify users and house memberships in the output above"
echo "  2. Start the API server and test with a Firebase Auth token"
echo "  3. The authenticated user should resolve to 'Casa Principal'"
