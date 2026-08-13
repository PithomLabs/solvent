# Execution Plan — MCP Prelude Integration (Beat 0)

**Source:** `/home/chaschel/Desktop/cockroach/demo/plan.md`
**Review:** `/home/chaschel/Desktop/cockroach/demo/plan_review.md` (APPROVED WITH MINOR NITS)

---

## Status

Code changes already implemented. Only verification and receipt recording remain.

---

## Pre-existing Code State (verified)

| File | Change | Status |
|---|---|---|
| `scripts/demo/walkthrough.sh` | `beat0()` function (lines 95-174) | DONE |
| `scripts/demo/walkthrough.sh` | `START_BEAT=0` (line 36) | DONE |
| `scripts/demo/walkthrough.sh` | Dispatcher `if [ "$START_BEAT" -le 0 ]; then beat0; fi` (line 526) | DONE |
| `scripts/demo/walkthrough.sh` | Header comments updated (lines 14-15) | DONE |
| `voiceover_final.md` | STAGE 0.5 MCP bridge inserted (lines 35-53) | DONE |
| `voiceover_final.md` | Aether section removed | DONE |
| `voiceover_final.md` | Timings shifted, close at 2:52–3:00 | DONE |

Nit 2 (output masking): `set -euo pipefail` active, no `tail -1` on `task db:reset`/`task mcp:seed` — failures propagate.
Nit 3 (conclusion extraction): Sanity check for short conclusions at line 165-167.
Nit 4 (timing optimization): No timing optimization in code — timing is a film-edit target only.

---

## Execution Steps

### Step 1 — Interactive recording (`task demo:film`)

```bash
task demo:film
```

Press Enter through all 8 beats (Beat 0 → Beat 7).

**Verify:**
- Beat 0: runs real Claude/MCP prelude, extracts and displays agent conclusion
- Beat 1: produces 0/0/0 (db:reset isolates from Beat 0)
- Beat 2: derives 3 beliefs with debt
- Beat 3: produces SQLSTATE 23514 (CHECK constraint refusal)
- Beat 4: produces SQLSTATE 23503 (composite FK refusal)
- Beat 5: promotes belief, creates live intent, Audit SAFE
- Beat 6: 3/3/1 → 3/3/1 (idempotent replay)
- Beat 7: retracted + cancelled + SAFE

**Record:** Full terminal output. Extract Beat 0 agent conclusion verbatim.

### Step 2 — No-pause smoke test

```bash
task demo:film --no-pause
```

**Verify:** All 8 beats complete, exit 0.

### Step 3 — Automated rehearsal

```bash
task demo:film:auto
```

**Verify:** All 8 beats complete with auto-advance, exit 0.

### Step 4 — Skip Beat 0 (`--from 1`)

```bash
task demo:film --from 1
```

**Verify:** Beat 0 skipped, starts at CLEAN SLATE (Beat 1: 0/0/0).

### Step 5 — Full regression suite

```bash
unset FABLE_TEST_DSN && task test
```

**Verify:** 107 PASS (go test, go build, go vet, gofmt, I-7 MCP boundary).

### Step 6 — Build and vet

```bash
go build ./... && go vet ./... && gofmt -l cmd internal
```

**Verify:** No errors, no output from gofmt.

### Step 7 — belief_edge invariant

```bash
docker exec solvent-crdb cockroach sql --insecure --database=fable -e "SELECT count(*) FROM belief_edge;"
```

**Verify:** Returns 0.

### Step 8 — Frozen-core diff

```bash
git diff --stat HEAD -- db/ internal/ cmd/
```

**Verify:** Empty output (no changes to frozen core).

---

## Receipt Recording

After all verification, write execution receipts to:

**`/home/chaschel/Desktop/cockroach/demo/mcp_filmable_walkthrough.md`**

Update:
- Status from "PLAN — awaiting execution" → "EXECUTED"
- All "Pending execution" sections with actual results
- Section 8 regression table with actual pass/fail
- Section 4 MCP evidence with actual tool calls observed
- Section 5 agent result with verbatim conclusion
- Section 9 files modified to reflect the new single-command architecture (beat0 integrated into walkthrough.sh, not a separate mcp_film.sh)

Also address plan_review Nit 1: Change "None triggered" pre-claim to actual verified results.

---

## Risks

- Beat 0 requires `claude` CLI authenticated and available in PATH (verified: `/home/chaschel/.local/bin/claude`)
- Beat 0 requires CockroachDB running (verified: `solvent-crdb` Up 33 minutes)
- Beat 0 requires MCP binary (verified: `bin/solvent-mcp` exists)
- Agent run takes ~30-40s real time — acceptable for filming, not a timeout risk
- Agent conclusion is non-deterministic — this is by design (not a bug)

---

## Stop Conditions

None expected based on current state. If triggered, STOP AND REPORT:
- Real MCP prelude cannot execute
- Agent conclusion must be fabricated
- Beat 0 contaminates Beat 1
- Seven-beat walkthrough regresses
- Frozen file must change
