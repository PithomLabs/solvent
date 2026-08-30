#!/usr/bin/env bash
# inspect.sh — Human-readable snapshot of the canonical demo scenario's ledger.
#
# Scoped to SOLVENT_SCENARIO_2, the judge-facing scenario the web app renders.
#
# It used to read every row in every table, and the AUDIT gate at the bottom was
# unscoped too. That made this script a cross-scenario false-alarm generator: one
# live intent left behind by a track1 run, a test scenario, or an aborted seed and
# `task inspect` would print VIOLATION about a Track 2 demo that is perfectly
# correct. This is the tool most likely to be run in front of an audience, so it
# reports on the scenario being demonstrated and nothing else.
#
# Pass --all to see every scenario (diagnostics, not the demo view).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/config.env"

SCOPE_LABEL="scenario ${SOLVENT_SCENARIO_2}"
B_WHERE="WHERE scenario_id = '${SOLVENT_SCENARIO_2}'"
E_WHERE="WHERE scenario_id = '${SOLVENT_SCENARIO_2}'"
A_WHERE="WHERE scenario_id = '${SOLVENT_SCENARIO_2}'"
AUDIT_AND="AND a.scenario_id = '${SOLVENT_SCENARIO_2}'"

if [ "${1:-}" = "--all" ]; then
  SCOPE_LABEL="ALL scenarios (diagnostic)"
  B_WHERE=""; E_WHERE=""; A_WHERE=""; AUDIT_AND=""
fi

q() { docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
        --database="$SOLVENT_DB_NAME" --format=tsv -e "$1" 2>/dev/null | tail -n +2; }

echo "=== SOLVENT LEDGER ==="
echo "  scope: ${SCOPE_LABEL}"
echo

echo "--- BELIEFS ---"
q "SELECT id::STRING, scenario_id::STRING, claim_type, status, claim, debt::STRING
   FROM belief ${B_WHERE} ORDER BY scenario_id, claim;" \
| while IFS=$'\t' read -r id scenario ctype status claim debt; do
  echo "  ID:         ${id}"
  echo "  Scenario:   ${scenario}"
  echo "  Type:       ${ctype}"
  echo "  Status:     ${status}"
  echo "  Claim:      ${claim}"
  echo "  Debt:       ${debt}"
  echo
done

echo "--- EVIDENCE ---"
q "SELECT id::STRING, belief_id::STRING, provenance_class, source_url
   FROM evidence ${E_WHERE} ORDER BY belief_id;" \
| while IFS=$'\t' read -r id belief prov url; do
  echo "  ID:         ${id}"
  echo "  Belief:     ${belief}"
  echo "  Class:      ${prov}"
  echo "  URL:        ${url}"
  echo
done

echo "--- INTENTS ---"
q "SELECT id::STRING, belief_id::STRING, action, state
   FROM action_intent ${A_WHERE} ORDER BY belief_id;" \
| while IFS=$'\t' read -r id belief action state; do
  echo "  ID:         ${id}"
  echo "  Belief:     ${belief}"
  echo "  Action:     ${action}"
  echo "  State:      ${state}"
  echo
done

# belief_edge carries no scenario_id (D-032), so it is scoped through its parent.
echo "--- DERIVATION EDGES ---"
q "SELECT p.claim, c.claim, e.kind
   FROM belief_edge e
   JOIN belief p ON p.id = e.parent_id
   JOIN belief c ON c.id = e.child_id
   $( [ -n "$B_WHERE" ] && echo "WHERE p.scenario_id = '${SOLVENT_SCENARIO_2}'" )
   ORDER BY p.claim, c.claim;" \
| while IFS=$'\t' read -r parent child kind; do
  echo "  ${parent}"
  echo "    --${kind}--> ${child}"
  echo
done

echo "--- AUDIT ---"
AUDIT=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
  --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted' ${AUDIT_AND};
" 2>/dev/null | tail -n1)

if [ "$AUDIT" = "0" ]; then
  echo "  Live intents on non-promoted beliefs: 0 (SAFE)"
else
  echo "  VIOLATION: ${AUDIT} live intents on non-promoted beliefs"
fi
echo
echo "=== END LEDGER ==="
