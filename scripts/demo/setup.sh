#!/usr/bin/env bash
# setup.sh — Start pinned CockroachDB in Docker, initialize schema.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/config.env"

echo "=== Solvent Setup ==="
echo

# --- Prerequisites ---
if ! command -v docker &>/dev/null; then
  echo "ERROR: docker is not installed or not in PATH." >&2
  echo "Install Docker: https://docs.docker.com/get-docker/" >&2
  exit 1
fi

if ! command -v go &>/dev/null; then
  echo "ERROR: go is not installed or not in PATH." >&2
  echo "Install Go: https://go.dev/dl/" >&2
  exit 1
fi

# Verify Go version matches go.mod directive.
REQUIRED_GO=$(grep '^go ' "$REPO_ROOT/go.mod" | awk '{print $2}')
INSTALLED_GO=$(go version | awk '{print $3}' | sed 's/^go//')
if [ "$INSTALLED_GO" != "$REQUIRED_GO" ]; then
  echo "WARNING: Go ${REQUIRED_GO} required (per go.mod), found ${INSTALLED_GO}" >&2
fi

if ! command -v task &>/dev/null; then
  echo "ERROR: task is not installed or not in PATH." >&2
  echo "Install Task: https://taskfile.dev/installation/" >&2
  exit 1
fi

if ! docker info &>/dev/null 2>&1; then
  echo "ERROR: Docker daemon is not running." >&2
  echo "Start Docker and try again." >&2
  exit 1
fi

echo "Prerequisites: OK (docker, go, task)"

# --- Start CockroachDB ---
if docker ps --format '{{.Names}}' | grep -q "^${SOLVENT_CRDB_CONTAINER}$"; then
  echo "Container ${SOLVENT_CRDB_CONTAINER} already running."
elif docker ps -a --format '{{.Names}}' | grep -q "^${SOLVENT_CRDB_CONTAINER}$"; then
  echo "Starting existing container ${SOLVENT_CRDB_CONTAINER}..."
  docker start "$SOLVENT_CRDB_CONTAINER" >/dev/null
else
  echo "Creating container ${SOLVENT_CRDB_CONTAINER} (cockroachdb/cockroach:v26.2.0)..."
  docker run -d \
    --name "$SOLVENT_CRDB_CONTAINER" \
    -p "${SOLVENT_PORT}:26257" \
    -p "${SOLVENT_HTTP_PORT}:8080" \
    cockroachdb/cockroach:v26.2.0 \
    start-single-node --insecure --accept-sql-without-tls
fi

# --- Wait for readiness ---
echo -n "Waiting for CockroachDB..."
ready=0
for _ in $(seq 1 60); do
  if docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure -e "SELECT 1" &>/dev/null; then
    ready=1
    break
  fi
  sleep 1
  echo -n "."
done
echo
if [ "$ready" -ne 1 ]; then
  echo "ERROR: CockroachDB did not become ready." >&2
  docker logs "$SOLVENT_CRDB_CONTAINER" --tail 20 >&2
  exit 1
fi
echo "CockroachDB: READY"

# --- Initialize schema ---
echo -n "Initializing database '${SOLVENT_DB_NAME}'..."
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  -e "CREATE DATABASE IF NOT EXISTS ${SOLVENT_DB_NAME}" >/dev/null

# Base-schema existence check (not data-presence): 001_schema.sql has frozen non-idempotent
# CREATE TABLE statements and must only run on a fresh database.
if ! docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" -e "SELECT 1 FROM belief LIMIT 1" &>/dev/null; then
  docker exec -i "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" < "$REPO_ROOT/db/001_schema.sql" >/dev/null
fi
docker exec -i "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" < "$REPO_ROOT/db/002_corpus.sql" >/dev/null
docker exec -i "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" < "$REPO_ROOT/db/003_wizard.sql" >/dev/null
docker exec -i "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" < "$REPO_ROOT/db/004_debt_vocabulary.sql" >/dev/null
echo " OK"

# --- Verify ---
echo -n "Verifying connection..."
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" -e "SELECT count(*) FROM belief" &>/dev/null
echo " OK"

# --- Build binaries ---
echo -n "Building Go binaries..."
(cd "$REPO_ROOT" && go build ./cmd/solvent/ ./cmd/operator-review/) >/dev/null
echo " OK"

echo
echo "=== Setup Complete ==="
echo
echo "Database: ${SOLVENT_DB_NAME} on localhost:${SOLVENT_PORT}"
echo "DSN:      ${SOLVENT_DSN}"
echo
echo "Next steps:"
echo "  task demo:track1     # Watch the vulnerability disclosure flow"
echo "  task demo:replay     # Prove deterministic replay"
echo "  task demo:track2     # Watch the retraction flow"
echo "  task inspect         # Inspect the ledger"
echo "  task test            # Run full test suite"
