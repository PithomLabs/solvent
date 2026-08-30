#!/usr/bin/env bash
# track1.sh — Track 1 demo: vulnerability disclosure → belief → promotion → intent.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/config.env"

export FABLE_DSN="$SOLVENT_DSN"

echo "=== SOLVENT / TRACK 1 ==="
echo "  Vulnerability disclosure → belief → debt → promotion → live intent"
echo

# --- Reset database ---
echo "--- Resetting database ---"
go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_1" --reset \
  --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track1" \
  2>&1
echo

# --- Run pipeline ---
echo "--- Processing evidence ---"
go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_1" \
  --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track1" \
  2>&1
echo

# --- Query beliefs ---
echo "--- Ledger state after pipeline ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT claim, status, debt::STRING
FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
ORDER BY claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status debt; do
  echo "  [$status] $claim"
  echo "          debt: $debt"
done
echo

# --- Find advisory belief ---
ADVISORY_ID=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
  AND claim_type = 'derived'
LIMIT 1;
" 2>/dev/null | tail -n1)

echo "--- Operator review: retire debts, promote, create intent ---"
go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_1" --belief "$ADVISORY_ID" \
  --action "deploy etcd v3.5.28" \
  --debt needContradictionSweep --debt needBlastRadius --debt needRollbackPlan \
  --debt needVersionPin --debt needOperatorSignoff 2>&1
echo

# --- Final summary ---
echo "--- Final state ---"
BELIEF_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

EVIDENCE_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM evidence WHERE scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

INTENT_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent
WHERE scenario_id = '${SOLVENT_SCENARIO_1}' AND state = 'live';
" 2>/dev/null | tail -n1)

AUDIT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n1)

PROMOTED_COUNT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}' AND status = 'promoted';
" 2>/dev/null | tail -n1)

echo
echo "  Evidence      ${EVIDENCE_COUNT}"
echo "  Beliefs       ${BELIEF_COUNT}"
echo "  Promoted      ${PROMOTED_COUNT}"
echo "  Live intents  ${INTENT_COUNT}"
if [ "$AUDIT" = "0" ]; then
  echo "  Audit         SAFE"
else
  echo "  Audit         VIOLATION (${AUDIT})"
fi
echo
echo "=== TRACK 1 COMPLETE ==="
