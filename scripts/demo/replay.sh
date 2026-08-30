#!/usr/bin/env bash
# replay.sh — Prove deterministic replay: re-running the pipeline creates no duplicates.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/config.env"

export FABLE_DSN="$SOLVENT_DSN"

echo "=== SOLVENT / REPLAY ==="
echo "  Proving idempotent replay on Track 1"
echo

# --- Record state before replay ---
echo "--- State before replay ---"
BELIEFS_BEFORE=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

EVIDENCE_BEFORE=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM evidence WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

INTENT_BEFORE=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

echo "  Beliefs:    ${BELIEFS_BEFORE}"
echo "  Evidence:   ${EVIDENCE_BEFORE}"
echo "  Intents:    ${INTENT_BEFORE}"
echo

# --- Replay ---
echo "--- Replaying pipeline (same scenario, no --reset) ---"
go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_1" \
  --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track1" \
  2>&1 | grep -v '^$'
echo

# --- Record state after replay ---
echo "--- State after replay ---"
BELIEFS_AFTER=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

EVIDENCE_AFTER=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM evidence WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

INTENT_AFTER=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

echo "  Beliefs:    ${BELIEFS_AFTER}"
echo "  Evidence:   ${EVIDENCE_AFTER}"
echo "  Intents:    ${INTENT_AFTER}"
echo

# --- Verify ---
if [ "$BELIEFS_BEFORE" = "$BELIEFS_AFTER" ] && \
   [ "$EVIDENCE_BEFORE" = "$EVIDENCE_AFTER" ] && \
   [ "$INTENT_BEFORE" = "$INTENT_AFTER" ]; then
  echo "Replay verified: no duplicate ledger state created."
else
  echo "ERROR: Counts changed after replay!" >&2
  echo "  beliefs:  ${BELIEFS_BEFORE} → ${BELIEFS_AFTER}" >&2
  echo "  evidence: ${EVIDENCE_BEFORE} → ${EVIDENCE_AFTER}" >&2
  echo "  intents:  ${INTENT_BEFORE} → ${INTENT_AFTER}" >&2
  exit 1
fi
echo
echo "=== REPLAY COMPLETE ==="
