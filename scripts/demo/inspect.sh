#!/usr/bin/env bash
# inspect.sh — Human-readable snapshot of the current ledger state.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/config.env"

echo "=== SOLVENT LEDGER ==="
echo

echo "--- BELIEFS ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING, scenario_id::STRING, claim_type, status, claim, debt::STRING
FROM belief
ORDER BY scenario_id, claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r id scenario ctype status claim debt; do
  echo "  ID:         ${id}"
  echo "  Scenario:   ${scenario}"
  echo "  Type:       ${ctype}"
  echo "  Status:     ${status}"
  echo "  Claim:      ${claim}"
  echo "  Debt:       ${debt}"
  echo
done

echo "--- EVIDENCE ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING, belief_id::STRING, provenance_class, source_url
FROM evidence
ORDER BY belief_id;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r id belief prov url; do
  echo "  ID:         ${id}"
  echo "  Belief:     ${belief}"
  echo "  Class:      ${prov}"
  echo "  URL:        ${url}"
  echo
done

echo "--- INTENTS ---"
docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING, belief_id::STRING, action, state
FROM action_intent
ORDER BY belief_id;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r id belief action state; do
  echo "  ID:         ${id}"
  echo "  Belief:     ${belief}"
  echo "  Action:     ${action}"
  echo "  State:      ${state}"
  echo
done

echo "--- AUDIT ---"
AUDIT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted';
" 2>/dev/null | tail -n1)

if [ "$AUDIT" = "0" ]; then
  echo "  Live intents on non-promoted beliefs: 0 (SAFE)"
else
  echo "  VIOLATION: ${AUDIT} live intents on non-promoted beliefs"
fi
echo
echo "=== END LEDGER ==="
