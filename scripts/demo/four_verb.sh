#!/usr/bin/env bash
# Four-verb demo: Connect → Ask → Authorize → Reassess
# MCP-only, deterministic, 2–3 minutes, no faked outputs.
# Uses real CockroachDB behavior and real project commands.
#
# Flow:
#   Connect → Ask (blocked: debt remains) → discharge CURRENT obligations
#   → Promote → Authorize live → Falsify/Reassess → Ask again (authority no longer valid)
#
# Prereq: task setup has been run at least once (solvent-crdb container exists).
# This script resets the fable database to a clean baseline.
set -euo pipefail
cd "$(dirname "$0")/../.."

DSN="${FABLE_DSN:-postgresql://root@localhost:26260/fable?sslmode=disable}"
BIN="bin/solvent-mcp"
SCENARIO="track2"
SCENARIO_ID="00000000-0000-0000-0000-000000000002"

echo "=== Four-verb demo: Connect → Ask → Authorize → Reassess ==="
echo "    MCP-only, real CockroachDB, no faked outputs"
echo

# Ensure DB container is up
if ! docker inspect -f '{{.State.Running}}' solvent-crdb 2>/dev/null | grep -q true; then
  echo "Starting solvent-crdb..."
  docker start solvent-crdb >/dev/null 2>&1 || { echo "Container not found; run 'task setup' first"; exit 1; }
  sleep 2
fi

echo "--- Reset + Seed (deterministic baseline) ---"
# Reset fable database (same as task db:reset, but without needing Task installed)
docker exec solvent-crdb cockroach sql --insecure -e "DROP DATABASE IF EXISTS fable CASCADE; CREATE DATABASE fable;" >/dev/null
docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/001_schema.sql >/dev/null
docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/002_corpus.sql >/dev/null
docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/003_wizard.sql >/dev/null
docker exec -i solvent-crdb cockroach sql --insecure --database=fable < db/004_debt_vocabulary.sql >/dev/null
echo "  database fable reset ✓"

go build -o "$BIN" ./cmd/solvent-mcp >/dev/null
echo "  $BIN built ✓"

# Seed track2 baseline: one entered belief with 6 debts, no intents
echo "  seeding $SCENARIO baseline..."
SEED_OUT=$(go run ./cmd/operator-review --dsn "$DSN" \
  --scenario "$SCENARIO_ID" \
  --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" \
  --claim-type postulated \
  --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0" \
  --evidence-sha f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1 2>&1)
echo "  $SEED_OUT"
BELIEF_ID=$(echo "$SEED_OUT" | sed -n 's/^BELIEF_ID=//p')
if [ -z "$BELIEF_ID" ]; then
  # Fallback: query directly
  BELIEF_ID=$(docker exec solvent-crdb cockroach sql --insecure --database=fable --format=tsv -e "SELECT id FROM belief WHERE scenario_id='$SCENARIO_ID' LIMIT 1;" 2>/dev/null | tail -n 1 | tr -d '[:space:]')
fi
if [ -z "$BELIEF_ID" ]; then
  echo "FAIL: no BELIEF_ID after seed"; exit 1
fi
echo "  belief $BELIEF_ID in $SCENARIO (entered, 6 debts) ✓"
echo

export FABLE_DSN="$DSN"
export SOLVENT_FIXTURE_ROOT="${SOLVENT_FIXTURE_ROOT:-$(pwd)/internal/derive/testdata/etcd_real}"

# Helper: speak JSON-RPC to solvent-mcp and pretty-print results
python3 - "$BIN" "$BELIEF_ID" "$SCENARIO" "$SCENARIO_ID" <<'PY'
import json, subprocess, sys, os

binary, belief_id, scenario, scenario_id = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
env = os.environ.copy()
# Ensure the MCP server sees the same DSN and fixture root as the seed step.
proc = subprocess.Popen([binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1, env=env)

def send(obj):
    proc.stdin.write(json.dumps(obj) + "\n")
    proc.stdin.flush()
def read(want_id):
    while True:
        line = proc.stdout.readline()
        if not line:
            print(proc.stderr.read()); sys.exit(1)
        try:
            msg = json.loads(line)
        except: continue
        if msg.get("id") == want_id:
            return msg

def call(name, args, _id):
    send({"jsonrpc":"2.0","id":_id,"method":"tools/call","params":{"name":name,"arguments":args}})
    r = read(_id).get("result",{})
    text = " ".join(c.get("text","") for c in r.get("content",[]))
    is_err = r.get("isError", False)
    return is_err, text, r

# Initialize
send({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"four_verb","version":"1"}}})
read(1)
send({"jsonrpc":"2.0","method":"notifications/initialized"})
# List to confirm
send({"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}})
tools = read(2).get("result",{}).get("tools",[])
print(f"CONNECT: {len(tools)} MCP tools available — {[t['name'] for t in tools]}")
print()

# 1. ASK — explain before promotion
print("1. ASK — solvent_explain before promotion")
_, text, _ = call("solvent_explain", {"scenario": scenario, "belief_id": belief_id}, 10)
try:
    payload = json.loads(text)
    be = payload["beliefs"][0] if payload.get("beliefs") else {}
    print(f"   claim: {be.get('claim','')[:80]}")
    print(f"   status: {be.get('status')} | remaining_debt: {be.get('remaining_debt')}")
    print(f"   can_promote: {be.get('can_promote')} | can_authorize: {be.get('can_authorize')}")
    print(f"   promotion_blocked: {be.get('promotion_blocked_reason','')[:120]}")
    print(f"   predicted_promotion: {be.get('predicted_promotion_sqlstate')} {be.get('predicted_promotion_constraint')}")
    print(f"   human: {be.get('human_summary','')[:200]}")
except Exception as e:
    print(f"   explain parse error {e}: {text[:500]}")
print()

# 2. AUTHORIZE before promotion — should be refused by gate
print("2. AUTHORIZE before promotion — solvent_authorize_action (expect 23503 gate)")
is_err, text, _ = call("solvent_authorize_action", {"scenario": scenario, "belief_id": belief_id, "action": "deploy etcd v3.5.0"}, 11)
print(f"   isError={is_err} | {text[:400]}")
print(f"   → database refused (gate) ✓" if "23503" in text or "gate" in text else "   → unexpected")
print()

# 3. Also promote before debt discharge — should be refused
print("3. PROMOTE before discharge — solvent_promote (expect 23514 promoted_is_debt_free)")
is_err, text, _ = call("solvent_promote", {"scenario": scenario, "belief_id": belief_id}, 12)
print(f"   isError={is_err} | {text[:400]}")
print(f"   → database refused (promoted_is_debt_free) ✓" if "23514" in text else "   → unexpected")
print()

# 4. Discharge CURRENT demo obligations (6 items)
print("4. DISCHARGE — retiring the 6 CURRENT debts one by one")
for debt in ["needProvenanceCheck","needContradictionSweep","needBlastRadius","needRollbackPlan","needVersionPin","needOperatorSignoff"]:
    is_err, text, _ = call("solvent_retire_debt", {"scenario": scenario, "belief_id": belief_id, "debt_item": debt}, 20)
    # envelopeResult returns remaining debt
    try:
        j = json.loads(text)
        remaining = j.get("result",{}).get("debt",[]) if isinstance(j, dict) else []
        print(f"   - {debt}: remaining {remaining}")
    except:
        print(f"   - {debt}: {text[:200]}")
print()

# 5. ASK again — now promotable
print("5. ASK after discharge — solvent_explain should report can_promote=true")
_, text, _ = call("solvent_explain", {"scenario": scenario, "belief_id": belief_id}, 13)
try:
    payload = json.loads(text)
    be = payload["beliefs"][0]
    print(f"   can_promote: {be.get('can_promote')} | remaining_debt: {be.get('remaining_debt')}")
    print(f"   human: {be.get('human_summary','')[:200]}")
except Exception as e:
    print(f"   parse error {e}")
print()

# 6. Promote
print("6. PROMOTE — solvent_promote (now debt-free)")
is_err, text, _ = call("solvent_promote", {"scenario": scenario, "belief_id": belief_id}, 14)
print(f"   isError={is_err} | {text[:400]}")
print(f"   → promoted ✓" if not is_err else "   → failed")
print()

# 7. Authorize — now live
print("7. AUTHORIZE — solvent_authorize_action (now promoted, expect live)")
is_err, text, _ = call("solvent_authorize_action", {"scenario": scenario, "belief_id": belief_id, "action": "deploy etcd v3.5.0"}, 15)
print(f"   isError={is_err} | {text[:400]}")
print(f"   → live intent ✓" if not is_err and "live" in text else "   → check")
print()

# 8. ASK shows live intent
print("8. ASK after authorize — solvent_explain shows live intent")
_, text, _ = call("solvent_explain", {"scenario": scenario, "belief_id": belief_id}, 16)
try:
    payload = json.loads(text)
    be = payload["beliefs"][0]
    print(f"   is_promoted: {be.get('is_promoted')} | can_authorize: {be.get('can_authorize')}")
    print(f"   live_intents: {be.get('live_intents')}")
    print(f"   human: {be.get('human_summary','')[:260]}")
except Exception as e:
    print(f"   parse error {e}")
print()

# 9. REASSESS — falsify/retract
print("9. REASSESS — solvent_falsify (retract + cancel live intent in one tx)")
is_err, text, _ = call("solvent_falsify", {"scenario": scenario, "belief_id": belief_id}, 17)
print(f"   isError={is_err} | {text[:400]}")
print(f"   → retracted ✓" if not is_err else "   → failed")
print()

# 10. ASK after reassessment — authority no longer valid
print("10. ASK after falsify — solvent_explain explains why authority is no longer valid")
_, text, _ = call("solvent_explain", {"scenario": scenario, "belief_id": belief_id}, 18)
try:
    payload = json.loads(text)
    be = payload["beliefs"][0]
    print(f"   status: {be.get('status')} | is_retracted: {be.get('is_retracted')}")
    print(f"   can_authorize: {be.get('can_authorize')} | live_intents: {be.get('live_intents')}")
    print(f"   human: {be.get('human_summary','')[:300]}")
    print(f"   global: {payload.get('global_summary','')[:300]}")
    print(f"   audit_live_on_nonpromoted: {payload.get('audit_live_on_nonpromoted')}")
except Exception as e:
    print(f"   parse error {e}: {text[:500]}")
print()
print("=== Demo complete: AI proposes → database decides → authority exists → reality changes → authority changes ===")

proc.stdin.close()
proc.terminate()
PY

echo
echo "Demo finished. Try:"
echo "  task mcp:verify    # 7 tools over stdio"
echo "  task test          # full suite"
