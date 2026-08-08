#!/usr/bin/env bash
# M0 cluster teardown. Verification-only infrastructure (O1).
# Leaves the store directory in place so it can be inspected; remove it manually.
set -euo pipefail

HOST="${FABLE_HOST:-localhost}"
PORT="${FABLE_PORT:-26260}"
DATA_DIR="${FABLE_DATA_DIR:-/tmp/fable-crdb-data}"
COCKROACH="${COCKROACH_BIN:-cockroach}"

if ! "$COCKROACH" sql --insecure --host="${HOST}:${PORT}" -e "SELECT 1" >/dev/null 2>&1; then
  echo "no node responding on ${HOST}:${PORT}"
  exit 0
fi

"$COCKROACH" quit --insecure --host="${HOST}:${PORT}" 2>/dev/null ||
  "$COCKROACH" node drain --insecure --host="${HOST}:${PORT}" --self 2>/dev/null || true

# quit was removed in newer releases; fall back to signalling the process holding the store.
if "$COCKROACH" sql --insecure --host="${HOST}:${PORT}" -e "SELECT 1" >/dev/null 2>&1; then
  pkill -f "start-single-node.*${PORT}" || true
fi

# A drained node releases its SQL port before its HTTP port. Waiting for both keeps
# an immediate down→up cycle from failing on "address already in use".
HTTP_PORT="${FABLE_HTTP_PORT:-8081}"
for _ in $(seq 1 30); do
  if ! ss -ltn 2>/dev/null | grep -qE ":(${PORT}|${HTTP_PORT}) "; then
    break
  fi
  sleep 1
done

echo "node on ${HOST}:${PORT} stopped. Store retained at ${DATA_DIR}"
echo "  rm -rf ${DATA_DIR}   # to discard"
