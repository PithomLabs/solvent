#!/usr/bin/env bash
# track2.sh — Track 2 demo: historical decision → promotion → intent → postmortem → retraction.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/config.env"

export FABLE_DSN="$SOLVENT_DSN"

# SHA256 of the v3.5.0 release fixture (canonical input).
V350_SHA="f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1"

echo "=== SOLVENT / TRACK 2 ==="
echo "  Historical decision → promotion → intent → postmortem → RetractCascade"
echo

# --- Reset database ---
echo "--- Resetting database ---"
# --file-demo-edge files the one derivation edge (internal/demoseed) once the
# fixtures have created the two claims it connects. The cloud initializer calls the
# same function, so local and cloud canonical state match.
go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_2" --reset --file-demo-edge \
  --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track2" \
  2>&1 | grep -v '^$'
echo

# --- Step 1: Enter baseline belief ---
echo "--- Step 1: Enter postulated historical deployment decision ---"
ENTER_OUTPUT=$(go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_2" \
  --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" \
  --claim-type postulated \
  --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0" \
  --evidence-sha "$V350_SHA" 2>&1)
echo "$ENTER_OUTPUT"

BELIEF_ID=$(echo "$ENTER_OUTPUT" | grep "BELIEF_ID=" | sed 's/BELIEF_ID=//')
echo "  Baseline belief: ${BELIEF_ID}"
echo

# --- Step 2: Operator review ---
echo "--- Step 2: Operator review — retire debts, promote, create intent ---"
go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_2" --belief "$BELIEF_ID" \
  --action "deploy etcd v3.5.0" \
  --debt needProvenanceCheck --debt needContradictionSweep --debt needBlastRadius \
  --debt needRollbackPlan --debt needVersionPin --debt needOperatorSignoff 2>&1
echo

# --- Show state before falsification ---
echo "--- State before falsification ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT b.claim, b.status, a.action, a.state as intent_state
FROM belief b
LEFT JOIN action_intent a ON a.belief_id = b.id
WHERE b.scenario_id = '${SOLVENT_SCENARIO_2}' AND b.id = '${BELIEF_ID}';
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status action intent; do
  echo "  Belief:  ${claim}"
  echo "  Status:  ${status}"
  echo "  Action:  ${action:-none}"
  echo "  Intent:  ${intent:-none}"
done
echo

# --- Step 3: Falsify ---
echo "--- Step 3: Ingest postmortem + falsify baseline belief ---"
go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_2" \
  --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track2" \
  2>&1 | grep -v '^$'

go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
  --scenario "$SOLVENT_SCENARIO_2" \
  --falsify "$BELIEF_ID" 2>&1
echo

# --- Final summary ---
echo "--- Final state ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT b.claim, b.status, a.state as intent_state
FROM belief b
LEFT JOIN action_intent a ON a.belief_id = b.id AND a.scenario_id = '${SOLVENT_SCENARIO_2}'
WHERE b.scenario_id = '${SOLVENT_SCENARIO_2}'
ORDER BY b.claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status intent; do
  echo "  [$status] $claim"
  if [ -z "$intent" ] || [ "$intent" = "NULL" ]; then
    echo "          intent: none"
  else
    echo "          intent: $intent"
  fi
done

AUDIT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = '${SOLVENT_SCENARIO_2}';
" 2>/dev/null | tail -n1)

echo
if [ "$AUDIT" = "0" ]; then
  echo "  Audit         SAFE"
else
  echo "  Audit         VIOLATION (${AUDIT})"
fi
echo
echo "=== TRACK 2 COMPLETE ==="
