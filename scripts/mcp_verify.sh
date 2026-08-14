#!/usr/bin/env bash
# Verify the solvent-mcp stdio server by speaking JSON-RPC to it, end to end.
#
# The README claims six tools. Until now nothing asserted that: there was no stdio
# harness anywhere in the repo, so "six tools over stdio" was a documentation claim about
# a binary no test had ever spoken to. This is the assertion.
#
# Four things are checked:
#
#   1. initialize completes and the server identifies itself;
#   2. tools/list returns EXACTLY six tools, by name;
#   3. solvent_retire_debt advertises debt_item as an enum of the six real items --
#      generated from kernel.FullDebt, not transcribed;
#   4. an unrecognised debt_item is REFUSED, and a real one gets past that guard.
#
# Check 4 is the one that matters. This SDK's low-level AddTool does not validate
# arguments against the input schema, and RetireDebt is array_remove -- retiring an item
# that is not present changes nothing and returns success. So without a handler-side
# guard, a typo or a stale vocabulary reports "retired" and then fails one step later at
# promote time as 23514 promoted_is_debt_free, nowhere near the actual mistake.
#
# No seeding is required. The debt_item guard runs BEFORE the belief lookup, so a
# nonexistent belief id is enough to exercise both branches:
#   bogus item  -> "unknown debt_item"          (the guard refused it)
#   real  item  -> "belief ... not found"       (the guard let it through)
set -uo pipefail
cd "$(dirname "$0")/.."

DSN="${FABLE_DSN:-postgresql://root@localhost:26260/fable?sslmode=disable}"
BIN=bin/solvent-mcp

echo "== build =="
go build -o "$BIN" ./cmd/solvent-mcp || { echo "MCP VERIFY BLOCKED: build failed"; exit 1; }
echo "  $BIN"

echo
echo "== speak JSON-RPC over stdio =="
FABLE_DSN="$DSN" \
SOLVENT_FIXTURE_ROOT="${SOLVENT_FIXTURE_ROOT:-internal/derive/testdata/etcd_real}" \
python3 - "$BIN" <<'PY'
import json, subprocess, sys, os

binary = sys.argv[1]
proc = subprocess.Popen([binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE, text=True, bufsize=1)

def send(obj):
    proc.stdin.write(json.dumps(obj) + "\n")
    proc.stdin.flush()

def read_result(want_id):
    # Skip notifications and anything that is not the reply we asked for.
    while True:
        line = proc.stdout.readline()
        if not line:
            err = proc.stderr.read()
            print("  FAIL: server closed stdout")
            if err.strip():
                print("  stderr:", err.strip()[:600])
            sys.exit(1)
        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            continue
        if msg.get("id") == want_id:
            return msg

fails = []
def check(ok, label, detail=""):
    print(("  ok   " if ok else "  FAIL ") + label + (("  -- " + detail) if detail and not ok else ""))
    if not ok:
        fails.append(label)

# 1. initialize
send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
      "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                 "clientInfo": {"name": "mcp_verify", "version": "1"}}})
init = read_result(1)
if "error" in init:
    print("  FAIL initialize:", json.dumps(init["error"])[:400])
    sys.exit(1)
name = init.get("result", {}).get("serverInfo", {}).get("name")
check(name == "solvent", "initialize -> serverInfo.name == solvent", f"got {name!r}")
send({"jsonrpc": "2.0", "method": "notifications/initialized"})

# 2. tools/list -- exactly six, by name
send({"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
tools = read_result(2).get("result", {}).get("tools", [])
got = sorted(t["name"] for t in tools)
want = sorted(["solvent_ledger", "solvent_ingest_evidence", "solvent_retire_debt",
               "solvent_promote", "solvent_authorize_action", "solvent_falsify"])
check(got == want, f"tools/list -> exactly 6 tools", f"got {got}")

# 3. debt_item carries the generated enum
FULL_DEBT = ["needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
             "needRollbackPlan", "needVersionPin", "needOperatorSignoff"]
rd = next((t for t in tools if t["name"] == "solvent_retire_debt"), {})
enum = (rd.get("inputSchema", {}).get("properties", {}).get("debt_item", {}) or {}).get("enum")
check(enum == FULL_DEBT, "debt_item advertises the six items as an enum, in order",
      f"got {enum}")
# The prose list is gone -- it was one of five hand-copies the rename had to find.
desc = rd.get("description", "")
check(not any(d in desc for d in FULL_DEBT),
      "the description no longer transcribes the vocabulary", desc[:120])

def call(args):
    send({"jsonrpc": "2.0", "id": 9, "method": "tools/call",
          "params": {"name": "solvent_retire_debt", "arguments": args}})
    r = read_result(9).get("result", {})
    text = " ".join(c.get("text", "") for c in r.get("content", []))
    return r.get("isError", False), text

NOWHERE = "ffffffff-ffff-4fff-8fff-ffffffffffff"   # syntactically valid, does not exist

# 4a. the retired name from before the Phase 5 rename must be refused
err, text = call({"scenario": "track2", "belief_id": NOWHERE, "debt_item": "needMap"})
check(err and "unknown debt_item" in text,
      "a retired name (needMap) is REFUSED, not silently ignored", text[:200])

# 4b. an outright bogus item must be refused
err, text = call({"scenario": "track2", "belief_id": NOWHERE, "debt_item": "totallyBogus"})
check(err and "unknown debt_item" in text, "a bogus item is REFUSED", text[:200])

# 4c. a real item gets PAST the guard -- it fails later, on the belief lookup
err, text = call({"scenario": "track2", "belief_id": NOWHERE, "debt_item": "needBlastRadius"})
check("unknown debt_item" not in text and "not found" in text,
      "a real item passes the guard and reaches the belief lookup", text[:200])

proc.stdin.close()
proc.terminate()

print()
if fails:
    print("MCP VERIFY BLOCKED — %d check(s) failed:" % len(fails))
    for f in fails:
        print("  -", f)
    sys.exit(1)
print("MCP VERIFY GREEN — 6 tools, enum generated from kernel.FullDebt, unknown items refused.")
PY
