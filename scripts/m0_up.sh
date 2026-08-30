#!/usr/bin/env bash
# M0 cluster bringup. Verification-only infrastructure (O1).
#
# Starts a fresh single-node CockroachDB on port 26260 / HTTP 8081. Port 26257 and
# 8080 are deliberately avoided: an unrelated container occupies them on this host
# and M0 must not touch it.
set -euo pipefail

HOST="${FABLE_HOST:-localhost}"
PORT="${FABLE_PORT:-26260}"
HTTP_PORT="${FABLE_HTTP_PORT:-8081}"
DATA_DIR="${FABLE_DATA_DIR:-/tmp/fable-crdb-data}"
DB="${FABLE_DB:-fable}"
COCKROACH="${COCKROACH_BIN:-cockroach}"
LOG_FILE="${FABLE_LOG:-${DATA_DIR}/node.log}"

DSN="postgresql://root@${HOST}:${PORT}/${DB}?sslmode=disable"

if "$COCKROACH" sql --insecure --host="${HOST}:${PORT}" -e "SELECT 1" >/dev/null 2>&1; then
  echo "node already up on ${HOST}:${PORT}"
else
  # Pre-flight both ports. A start that dies on a bound HTTP port after the store
  # has been partially initialized leaves that store unusable, so refuse early
  # rather than half-initialize it.
  for p in "$PORT" "$HTTP_PORT"; do
    if ss -ltn 2>/dev/null | grep -q ":${p} "; then
      echo "port ${p} is already bound; refusing to start (would corrupt ${DATA_DIR})" >&2
      echo "  free the port, or override FABLE_PORT / FABLE_HTTP_PORT" >&2
      exit 1
    fi
  done

  echo "starting single-node cluster on ${HOST}:${PORT} (http ${HTTP_PORT}), store=${DATA_DIR}"
  mkdir -p "$DATA_DIR"

  # Deliberately NOT --background: that flag's readiness poll targets the default
  # port 26257, which on this host belongs to an unrelated container, so it never
  # returns. Detach explicitly and poll the port we actually asked for.
  nohup "$COCKROACH" start-single-node \
    --insecure \
    --listen-addr="${HOST}:${PORT}" \
    --http-addr="${HOST}:${HTTP_PORT}" \
    --store="$DATA_DIR" \
    >"${LOG_FILE}" 2>&1 &
  disown || true

  ready=0
  for _ in $(seq 1 60); do
    if "$COCKROACH" sql --insecure --host="${HOST}:${PORT}" -e "SELECT 1" >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  if [ "$ready" -ne 1 ]; then
    echo "node did not become ready on ${HOST}:${PORT}; see ${LOG_FILE}" >&2
    tail -n 20 "${LOG_FILE}" >&2 || true
    echo >&2
    echo "if the store was partially initialized, discard it before retrying:" >&2
    echo "  rm -rf ${DATA_DIR}" >&2
    exit 1
  fi
fi

"$COCKROACH" sql --insecure --host="${HOST}:${PORT}" \
  -e "CREATE DATABASE IF NOT EXISTS ${DB}" >/dev/null

"$COCKROACH" sql --insecure --host="${HOST}:${PORT}" --format=tsv -e "SELECT version()" | tail -n1

echo
echo "database '${DB}' ready. Export the DSN with:"
echo "  export FABLE_DSN='${DSN}'"
