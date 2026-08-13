#!/usr/bin/env bash
# walkthrough.sh — Filmable, paced demo walkthrough for Solvent.
#
# Sequences the existing demo commands with stage banners and pauses
# so a presenter can record each beat with voiceover.
#
# Usage:
#   bash scripts/demo/walkthrough.sh              # interactive (Enter to advance)
#   bash scripts/demo/walkthrough.sh --auto 6     # auto-advance after 6s
#   bash scripts/demo/walkthrough.sh --no-pause   # run straight through
#   bash scripts/demo/walkthrough.sh --from 1     # skip Beat 0, start at CLEAN SLATE
#
# --from safety:
#   0 — needs Claude CLI, MCP binary, CockroachDB running (full MCP prelude)
#   1 — always safe (full reset) — starts the courtroom walkthrough
#   2 — needs post-beat-1 state
#   3 — needs beliefs from beat 2
#   4 — needs beliefs from beat 2 (same as 3)
#   5 — needs beliefs from beat 2 (promotes the derived belief)
#   6 — needs post-beat-5 state (promoted + intent, scenario 1)
#   7 — always safe (does its own db:reset)
#
# Prerequisites: task setup was run once. Container must be running (use task db:up).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
source "$SCRIPT_DIR/config.env"

export FABLE_DSN="$SOLVENT_DSN"

# --- Pacing controls ---
AUTO_MODE=0
AUTO_DELAY=6
NO_PAUSE=0
START_BEAT=0

while [ $# -gt 0 ]; do
  case "$1" in
    --auto)
      AUTO_MODE=1
      if [ "${2:-}" ] && [ "${2:-}" -eq "${2:-}" ] 2>/dev/null; then
        AUTO_DELAY="$2"; shift
      fi
      shift ;;
    --no-pause)
      NO_PAUSE=1; shift ;;
    --from)
      START_BEAT="$2"; shift 2 ;;
    *)
      echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# --- Helpers ---
banner() {
  echo
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  $1"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo
}

pause() {
  if [ "$NO_PAUSE" = "1" ]; then return; fi
  if [ "$AUTO_MODE" = "1" ]; then
    echo
    echo "  (${AUTO_DELAY}s) ..."
    sleep "$AUTO_DELAY"
    return
  fi
  echo
  read -rp "  Press Enter to continue..."
}

query_count() {
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "$1" 2>/dev/null | tail -n1
}

query_one() {
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "$1" 2>/dev/null | tail -n1
}

narrate() {
  echo
  echo "  $1"
  echo
}

# ======================================================================
# BEAT 0 — REAL AGENT → MCP → SOLVENT
# ======================================================================
beat0() {
  banner "STAGE 0 · REAL AGENT → MCP → SOLVENT"

  # Build MCP binary only if missing
  if [ ! -f "$REPO_ROOT/bin/solvent-mcp" ]; then
    echo "--- Building MCP server ---"
    go build -o "$REPO_ROOT/bin/solvent-mcp" "$REPO_ROOT/cmd/solvent-mcp" 2>/dev/null
  fi

  # Seed database — failures exit immediately (set -euo pipefail)
  echo "--- Preparing database for MCP prelude ---"
  task db:reset 2>&1
  task mcp:seed 2>&1
  echo

  # Set up isolated agent workspace
  echo "--- Setting up agent workspace ---"
  task agent:workspace 2>&1
  echo

  # Show the agent prompt
  WORKSPACE="${HOME}/.solvent-agent-box"
  PROMPT=$(cat "$WORKSPACE/prompt.txt")
  narrate "Agent prompt:"
  echo "  \"${PROMPT}\""
  echo

  # Run the real Claude agent
  narrate "MCP:"
  echo "  Running real Claude agent against Solvent..."
  echo

  JSONL="$WORKSPACE/run_mcp_prelude.jsonl"
  cd "$WORKSPACE"
  env -u CLAUDECODE -u CLAUDE_CODE_ENTRYPOINT -u CLAUDE_CODE_SESSION_ID \
      -u CLAUDE_CODE_CHILD_SESSION -u CLAUDE_CODE_MESSAGING_SOCKET \
      -u CLAUDE_PID -u CLAUDE_EFFORT -u CLAUDE_CODE_EXECPATH \
    claude -p "$(cat prompt.txt)" \
      --mcp-config ./solvent-mcp.json \
      --strict-mcp-config \
      --tools "" \
      --allowedTools "mcp__solvent__*" \
      --model opus \
      --permission-mode dontAsk \
      --verbose \
      --output-format stream-json < /dev/null > "$JSONL" 2>/dev/null

  cd "$REPO_ROOT"
  echo

  # Extract the real agent conclusion (last assistant message)
  CONCLUSION=$(python3 -c "
import json
last_assistant = None
with open('$JSONL') as f:
    for line in f:
        try:
            d = json.loads(line.strip())
            if d.get('type') == 'assistant':
                last_assistant = d
        except:
            pass
if last_assistant:
    content = last_assistant.get('message', {}).get('content', [])
    for c in content:
        if c.get('type') == 'text':
            print(c['text'])
" 2>/dev/null || echo "(could not parse conclusion)")

  # Sanity check: extracted conclusion should be substantive
  if [ ${#CONCLUSION} -lt 10 ]; then
    echo "  (WARNING: extracted conclusion is very short — may not be substantive)"
  fi

  narrate "Agent conclusion:"
  echo "  ${CONCLUSION}"
  echo

  pause
}

# ======================================================================
# BEAT 1 — CLEAN SLATE
# ======================================================================
beat1() {
  banner "STAGE 1 · CLEAN SLATE"

  echo "--- Resetting database ---"
  task db:reset 2>&1
  echo

  echo "--- Empty ledger ---"
  BELIEF_COUNT=$(query_count "SELECT count(*) FROM belief;")
  EVIDENCE_COUNT=$(query_count "SELECT count(*) FROM evidence;")
  INTENT_COUNT=$(query_count "SELECT count(*) FROM action_intent;")
  echo "  Beliefs:    ${BELIEF_COUNT}"
  echo "  Evidence:   ${EVIDENCE_COUNT}"
  echo "  Intents:    ${INTENT_COUNT}"

  pause
}

# ======================================================================
# BEAT 2 — EVIDENCE → BELIEFS
# ======================================================================
beat2() {
  banner "STAGE 2 · EVIDENCE → BELIEFS"

  echo "--- Ingesting evidence ---"
  go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_1" --reset \
    --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track1" \
    2>&1 | grep -v '^$'
  echo

  echo "--- Beliefs entered (with debt) ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT claim, status, debt::STRING
FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
ORDER BY claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status debt; do
    echo "  [${status}] ${claim}"
    echo "          debt: ${debt}"
  done
  echo

  # Derive belief IDs for later beats.
  # DERIVED_BELIEF_ID: the advisory (derived) belief — used for debt gate + promotion.
  DERIVED_BELIEF_ID=$(query_one "
SELECT id::STRING FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
  AND claim_type = 'derived'
LIMIT 1;")
  export DERIVED_BELIEF_ID

  # ACCOMMODATED_BELIEF_ID: an accommodated belief — semantically coherent with deploy action.
  ACCOMMODATED_BELIEF_ID=$(query_one "
SELECT id::STRING FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
  AND claim_type = 'accommodated'
LIMIT 1;")
  export ACCOMMODATED_BELIEF_ID

  pause
}

# ======================================================================
# BEAT 3 — DEBT GATE (SQLSTATE 23514)
# ======================================================================
beat3() {
  banner "STAGE 3 · DEBT GATE"

  narrate "First, an agent tries to promote this claim while it still owes proof. The database won't let it—you can't grant authority to a claim that hasn't paid its debts."

  echo "  Target: derived belief (still carries debt)"
  echo "  Attempt: promote → schema must refuse (CHECK constraint)"
  echo

  # --debt fake passes the "requires --debt" guard; retire is a no-op.
  # Promote then fails: CHECK constraint promoted_is_debt_free rejects it.
  set +e
  REFUSAL=$(go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_1" --belief "$DERIVED_BELIEF_ID" \
    --action "deploy etcd v3.5.28" \
    --debt fake 2>&1)
  set -e

  echo "--- Raw database refusal (SQLSTATE 23514) ---"
  echo "$REFUSAL"
  echo "---"

  pause
}

# ======================================================================
# BEAT 4 — ACTION GATE (SQLSTATE 23503)
# ======================================================================
beat4() {
  banner "STAGE 4 · ACTION GATE"

  # Prove the target belief is genuinely unpromoted.
  echo "--- Target belief (unpromoted) ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING, claim, status
FROM belief
WHERE id = '${ACCOMMODATED_BELIEF_ID}'::UUID;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r id claim status; do
    echo "  ID:     ${id}"
    echo "  Claim:  ${claim}"
    echo "  Status: ${status}"
  done
  echo

  narrate "Now it goes further—it tries to authorize a real deployment against a belief that was never promoted. Watch the database refuse the action itself. Not a policy, not a prompt—a foreign key. It even tells you why: the key (belief, promoted) doesn't exist, because this belief is only 'entered'."

  echo "  Attempt: create intent against unpromoted belief"
  echo "  (the composite FK must refuse this)"
  echo

  # Same INSERT the kernel runs (sqlIntentOnPromoted).
  # belief_status defaults to 'promoted' — but the real status is 'entered'.
  # The composite FK (belief_id, 'promoted') -> belief(id, status) fails
  # because no row matches (id, 'promoted'). SQLSTATE 23503.
  set +e
  ACTION_REFUSAL=$(docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" -e "
INSERT INTO action_intent (scenario_id, belief_id, action)
VALUES ('${SOLVENT_SCENARIO_1}'::UUID, '${ACCOMMODATED_BELIEF_ID}'::UUID, 'deploy etcd v3.5.28');
" 2>&1)
  set -e

  echo "--- Raw database refusal (SQLSTATE 23503) ---"
  echo "$ACTION_REFUSAL"
  echo "---"

  narrate "This is the exact insert the system runs internally—issued directly here so you can watch the gate refuse it in isolation."

  pause
}

# ======================================================================
# BEAT 5 — CLEAR DEBT → PROMOTE → AUTHORIZE
# ======================================================================
beat5() {
  banner "STAGE 5 · CLEAR DEBT → PROMOTE → AUTHORIZE"

  echo "--- Retiring debts on derived belief ---"
  go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_1" --belief "$DERIVED_BELIEF_ID" \
    --action "deploy etcd v3.5.28" \
    --debt needInvariant --debt needToyCheck --debt needNullModel \
    --debt needObstruction --debt needFaithfulnessReview 2>&1
  echo

  narrate "Once the obligations are cleared, the advisory becomes the warrant, and the deployment intent goes live."

  echo "--- Ledger state ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT id::STRING, claim, status, debt::STRING
FROM belief
WHERE scenario_id = '${SOLVENT_SCENARIO_1}'
ORDER BY claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r id claim status debt; do
    echo "  [${status}] ${claim}"
    echo "          debt: ${debt}"
  done
  echo

  echo "--- Intent ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT a.action, a.state
FROM action_intent a
WHERE a.scenario_id = '${SOLVENT_SCENARIO_1}';
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r action state; do
    echo "  action: ${action}"
    echo "  state:  ${state}"
  done
  echo

  AUDIT_COUNT=$(query_count "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted'
  AND a.scenario_id = '${SOLVENT_SCENARIO_1}';")
  if [ "$AUDIT_COUNT" = "0" ]; then
    echo "  Audit: SAFE"
  else
    echo "  Audit: VIOLATION (${AUDIT_COUNT})"
  fi

  pause
}

# ======================================================================
# BEAT 6 — REPLAY (IDEMPOTENCE)
# ======================================================================
beat6() {
  banner "STAGE 6 · REPLAY (IDEMPOTENCE)"

  narrate "We replay the same evidence without resetting the ledger. Three beliefs, three evidence records, one intent—and still three, three, one. No duplicates."

  echo "--- State before replay ---"
  BB=$(query_count "SELECT count(*) FROM belief WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  EB=$(query_count "SELECT count(*) FROM evidence WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  IB=$(query_count "SELECT count(*) FROM action_intent WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  echo "  Beliefs:  ${BB}"
  echo "  Evidence: ${EB}"
  echo "  Intents:  ${IB}"
  echo

  echo "--- Replaying pipeline (no --reset) ---"
  go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_1" \
    --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track1" \
    2>&1 | grep -v '^$'
  echo

  echo "--- State after replay ---"
  BA=$(query_count "SELECT count(*) FROM belief WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  EA=$(query_count "SELECT count(*) FROM evidence WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  IA=$(query_count "SELECT count(*) FROM action_intent WHERE scenario_id = '${SOLVENT_SCENARIO_1}';")
  echo "  Beliefs:  ${BA}"
  echo "  Evidence: ${EA}"
  echo "  Intents:  ${IA}"
  echo

  if [ "$BB" = "$BA" ] && [ "$EB" = "$EA" ] && [ "$IB" = "$IA" ]; then
    echo "  Replay verified: no duplicate ledger state created."
  else
    echo "  ERROR: Counts changed after replay!" >&2
    exit 1
  fi

  pause
}

# ======================================================================
# BEAT 7 — THE REVERSAL
# ======================================================================
beat7() {
  banner "STAGE 7 · THE REVERSAL"

  echo "--- Resetting database for scenario 2 ---"
  task db:reset 2>&1
  echo

  echo "--- Ingesting postmortem + baseline belief ---"
  go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_2" --reset \
    --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track2" \
    2>&1 | grep -v '^$'
  echo

  echo "--- Entering baseline belief ---"
  ENTER_OUTPUT=$(go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_2" \
    --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" \
    --claim-type postulated \
    --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0" \
    --evidence-sha f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1 2>&1)
  echo "$ENTER_OUTPUT"
  BELIEF_ID_T2=$(echo "$ENTER_OUTPUT" | grep "BELIEF_ID=" | sed 's/BELIEF_ID=//')
  echo

  echo "--- Promoting baseline belief ---"
  go run "$REPO_ROOT/cmd/operator-review" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_2" --belief "$BELIEF_ID_T2" \
    --action "deploy etcd v3.5.0" \
    --debt needMap --debt needInvariant --debt needToyCheck \
    --debt needNullModel --debt needObstruction --debt needFaithfulnessReview 2>&1
  echo

  echo "--- State before falsification ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT b.claim, b.status, a.action, a.state as intent_state
FROM belief b
LEFT JOIN action_intent a ON a.belief_id = b.id
WHERE b.scenario_id = '${SOLVENT_SCENARIO_2}' AND b.id = '${BELIEF_ID_T2}';
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status action intent; do
    echo "  Belief:  ${claim}"
    echo "  Status:  ${status}"
    echo "  Action:  ${action:-none}"
    echo "  Intent:  ${intent:-none}"
  done
  echo

  echo "--- Ingesting postmortem + falsifying baseline ---"
  go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_2" \
    --fixtures "$REPO_ROOT/internal/derive/testdata/etcd_real/track2" \
    2>&1 | grep -v '^$'

  FALSIFY_OUTPUT=$(go run "$REPO_ROOT/cmd/solvent" --dsn "$FABLE_DSN" \
    --scenario "$SOLVENT_SCENARIO_2" \
    --falsify "$BELIEF_ID_T2" 2>&1)
  echo "$FALSIFY_OUTPUT"
  echo

  echo "--- Final state ---"
  docker exec "$SOLVENT_CRDB_CONTAINER" cockroach sql --insecure \
    --database="$SOLVENT_DB_NAME" --format=tsv -e "
SELECT b.claim, b.status, a.state as intent_state
FROM belief b
LEFT JOIN action_intent a ON a.belief_id = b.id AND a.scenario_id = '${SOLVENT_SCENARIO_2}'
WHERE b.scenario_id = '${SOLVENT_SCENARIO_2}'
ORDER BY b.claim;
" 2>/dev/null | tail -n +2 | while IFS=$'\t' read -r claim status intent; do
    echo "  [${status}] ${claim}"
    if [ -z "$intent" ] || [ "$intent" = "NULL" ]; then
      echo "          intent: none"
    else
      echo "          intent: ${intent}"
    fi
  done
  echo

  narrate "Then new evidence arrives. The reversal scenario retracts the old warrant and cancels the action it authorized. Here is that scenario's final ledger: the belief is retracted, the intent is cancelled, and the audit remains safe."

  AUDIT_COUNT=$(query_count "
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted'
  AND a.scenario_id = '${SOLVENT_SCENARIO_2}';")
  if [ "$AUDIT_COUNT" = "0" ]; then
    echo "  Audit: SAFE"
  else
    echo "  Audit: VIOLATION (${AUDIT_COUNT})"
  fi
  echo

  echo "--- Full ledger inspection ---"
  bash "$SCRIPT_DIR/inspect.sh" 2>&1

  echo
  echo "=== WALKTHROUGH COMPLETE ==="
}

# ======================================================================
# MAIN
# ======================================================================
echo "SOLVENT — FILMABLE WALKTHROUGH"
echo "  Mode: $([ "$AUTO_MODE" = "1" ] && echo "auto (${AUTO_DELAY}s)" || ([ "$NO_PAUSE" = "1" ] && echo "no-pause" || echo "interactive"))"
echo "  Starting at beat: ${START_BEAT}"
echo

if [ "$START_BEAT" -le 0 ]; then beat0; fi
if [ "$START_BEAT" -le 1 ]; then beat1; fi
if [ "$START_BEAT" -le 2 ]; then beat2; fi
if [ "$START_BEAT" -le 3 ]; then beat3; fi
if [ "$START_BEAT" -le 4 ]; then beat4; fi
if [ "$START_BEAT" -le 5 ]; then beat5; fi
if [ "$START_BEAT" -le 6 ]; then beat6; fi
if [ "$START_BEAT" -le 7 ]; then beat7; fi
