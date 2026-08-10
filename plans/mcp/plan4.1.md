# Solvent MCP Server — Implementation Plan (plan4.1)

**Status:** READY FOR IMPLEMENTATION
**Date:** 2026-08-11
**Spec:** `plans/mcp/plan3.md` (FINAL)
**Review:** `plans/mcp/plan4_review.md` — APPROVED WITH ONE MECHANICAL CORRECTION
**Source plan:** `plans/mcp/plan4.md`

---

## Pre-implementation fix

The review identified one shell-chaining bug in plan4.md's Taskfile grep gate. This plan uses the corrected version throughout.

```sh
# WRONG (plan4.md line 267 — can produce both FAIL and PASS):
grep -rn 'INSERT|UPDATE|DELETE|CREATE|DROP' cmd/solvent-mcp internal/view && echo "FAIL" && exit 1 || echo "PASS"

# CORRECT (used in this plan):
if grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view; then
  echo "FAIL: raw writes in MCP/view"
  exit 1
else
  echo "I-7 MCP boundary: PASS"
fi
```

---

## Phase 1: Dependency

### Step 1 — Add MCP SDK

```bash
go get github.com/modelcontextprotocol/go-sdk
```

Third direct dependency. Only imported by `cmd/solvent-mcp`. Kernel gains no dependency.

### Step 2 — Verify clean build

```bash
go build ./...
go vet ./...
```

Must pass before proceeding. No other code changes yet.

---

## Phase 2: Error mapping

### Step 3 — Create `cmd/solvent-mcp/errors.go`

**Purpose:** Map kernel errors to MCP tool error results. This is the only place `pgconn.PgError` is referenced in the MCP layer.

**Key property:** The kernel's `wrapIf` function (`internal/kernel/errors.go:51`) uses `fmt.Errorf("%w: %w", sentinel, err)`, which preserves the `*pgconn.PgError` inside the sentinel. So `errors.As(err, &pgErr)` reaches it.

**Implementation:**
- Import `github.com/jackc/pgx/v5/pgconn` and `errors`
- Define `func toolError(err error) map[string]interface{}`
- Use `errors.As(err, &pgErr)` to extract `*pgconn.PgError`
- If PgError: return `{ "error": true, "sentinel": err.Error(), "sqlstate": pgErr.SQLState(), "constraint": pgErr.ConstraintName }`
- If not PgError: return `{ "error": true, "message": err.Error() }`

---

## Phase 3: Read-only view

### Step 4 — Create `internal/view/view.go`

**Purpose:** Read-only projections for `solvent_ledger`. SELECT only — no INSERT, UPDATE, DELETE, CREATE, DROP.

**Types:**
```go
type Snapshot struct {
    Beliefs                  []Belief
    Evidence                 []Evidence
    Intents                  []Intent
    AuditLiveOnNonPromoted   int
}

type Belief struct {
    ID        string
    Claim     string
    ClaimType string
    Status    string
    Debt      []string
}

type Evidence struct {
    BeliefID         string
    SourceURL        string
    ProvenanceClass  string
    ContentSHA256    string
}

type Intent struct {
    BeliefID string
    Action   string
    State    string
}

type SnapshotOpts struct {
    BeliefID        string  // optional filter
    IncludeEvidence bool
}
```

**Function:**
```go
func Snapshot(ctx context.Context, db *sql.DB, scenarioID string, opts SnapshotOpts) (*Snapshot, error)
```

- SELECT from `belief` (filtered by scenario_id, optionally by id)
- SELECT from `evidence` (only if IncludeEvidence, filtered by scenario_id)
- SELECT from `action_intent` (filtered by scenario_id)
- All reads only

**Verification:** `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" internal/view` → no matches.

---

## Phase 4: Tool handlers

### Step 5 — Create `cmd/solvent-mcp/tools.go`

**Purpose:** Six tool handlers. Each is exactly `unmarshal → kernel/pipeline call → format`.

**Zero semantic conditionals:** No `if belief.Status != "promoted"`, no `if len(debt) > 0`, no `if intent.State != "live"`.

**Handler pattern (every tool):**
```go
func handleSolventXxx(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
    // 1. Unmarshal args
    // 2. Call kernel/pipeline function
    // 3. If error → toolError(err) → return with IsError=true
    // 4. Format result
    // 5. Get audit from pipeline.AuditIntent (for state-changing tools)
    // 6. Return
}
```

**Six tools:**

1. **`solvent_ledger`** — Read-only. Calls `view.Snapshot()` + `pipeline.AuditIntent()`. Returns beliefs, optional evidence, intents, audit. Audit embedded in snapshot.

2. **`solvent_ingest_evidence`** — Write. Takes `scenario` enum. Resolves to fixture dir server-side. Calls `pipeline.Run()`. Returns per-result claim + classification + belief_id + debt count + audit.

3. **`solvent_retire_debt`** — Write. Takes `belief_id` + `debt_item`. Calls `kernel.Store.RetireDebt()`. Returns resulting debt array + audit.

4. **`solvent_promote`** — Write. Takes `belief_id`. Calls `kernel.Store.Promote()`. On success: `{ "status": "promoted" }` + audit. On refusal: error result with SQLSTATE 23514 + constraint `promoted_is_debt_free`. **DO NOT pre-check debt.**

5. **`solvent_authorize_action`** — Write. Takes `scenario` + `belief_id` + `action`. Calls `kernel.Store.IntentOnPromoted()`. On success: `{ "intent_state": "live" }` + audit. On refusal: error result with SQLSTATE 23503 + constraint `gate`. **DO NOT pre-check promotion status.**

6. **`solvent_falsify`** — Write. Takes `scenario` + `belief_id`. Calls `kernel.Store.RetractCascade()`. Returns `{ "retracted": N }` + audit. **Single-belief retraction.**

**Audit envelope:** For state-changing tools, wrap result in `{ "result": {...}, "audit": { "live_on_nonpromoted": N } }`. For `solvent_ledger`, audit is embedded in the snapshot.

**Tool descriptions (ship this text from Plan 3 §9):**

- `solvent_promote`: "Attempt to promote a belief to authorized status. The database refuses promotion while the belief carries any open debt item, returning constraint promoted_is_debt_free (SQLSTATE 23514). Call this whenever the user asks to promote a belief — do not pre-check the debt yourself and do not decline on the user's behalf."
- `solvent_authorize_action`: "Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status."
- `solvent_falsify`: "Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in the same transaction. Retracts a single belief — this does not propagate across a belief graph. Obtain the belief's id from solvent_ledger immediately before calling."
- `solvent_ledger`: "Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state."
- `solvent_ingest_evidence`: "Process the pinned evidence fixtures for a scenario through the full pipeline. Idempotent: re-running creates no duplicate beliefs, evidence, or intents."
- `solvent_retire_debt`: "Record that one review obligation on a belief has been discharged. Valid items: needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview."

---

## Phase 5: Server entry

### Step 6 — Create `cmd/solvent-mcp/main.go`

**Purpose:** Stdio MCP server entry point.

**Implementation:**
- Read `FABLE_DSN` from environment
- Read `SOLVENT_FIXTURE_ROOT` from environment (with executable-relative fallback)
- Validate fixture directories exist: `stat <root>/track1`, `stat <root>/track2` — exit non-zero if missing
- Open DB via `testdb.Open(dsn)` + ping — fail fast with message naming `task setup`
- Create MCP server: `mcp.NewServer(&mcp.Implementation{Name: "solvent", Version: "v0.1.0"}, nil)`
- Register 6 tools via `mcp.AddTool(server, toolDef, handler)`
- Run: `server.Run(ctx, &mcp.StdioTransport{})`

**Scenario → UUID mapping (server-owned):**
```
track1 → 00000000-0000-0000-0000-000000000001 → <fixtureRoot>/track1
track2 → 00000000-0000-0000-0000-000000000002 → <fixtureRoot>/track2
```

---

## Phase 6: Client config

### Step 7 — Create `.mcp.json`

```json
{
  "mcpServers": {
    "solvent": {
      "command": "go",
      "args": ["run", "./cmd/solvent-mcp"],
      "env": {
        "FABLE_DSN": "postgresql://root@localhost:26260/fable?sslmode=disable",
        "SOLVENT_FIXTURE_ROOT": "internal/derive/testdata/etcd_real"
      }
    }
  }
}
```

No secrets. No machine-specific absolute paths. Uses relative paths from repo root.

---

## Phase 7: Taskfile

### Step 8 — Update `Taskfile.yml`

Add tasks:

```yaml
mcp:build:
  desc: Build the MCP server binary
  cmds:
    - go build -o bin/solvent-mcp ./cmd/solvent-mcp

mcp:seed:
  desc: Seed the Track 2 baseline belief for MCP demo
  cmds:
    - task db:reset
    - >
      go run ./cmd/operator-review --dsn "$SOLVENT_DSN"
      --scenario 00000000-0000-0000-0000-000000000002
      --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)"
      --claim-type postulated
      --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0"
      --evidence-sha f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1
    - echo "Seed complete. Post-seed state: 1 belief (postulated, entered, 6 debts), 1 evidence, 0 intents, audit 0."
```

Extend the `test` task's I-7 gate (after the existing `check_i7.sh` invocation):

```yaml
# I-7 MCP boundary — no raw writes in cmd/solvent-mcp or internal/view
- |
  if grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view; then
    echo "FAIL: raw writes in MCP/view"
    exit 1
  else
    echo "I-7 MCP boundary: PASS"
  fi
```

---

## Phase 8: Build verification

### Step 9 — Build

```bash
task mcp:build
```

Must succeed. Binary at `bin/solvent-mcp`.

### Step 10 — I-7 gate

```bash
grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view
```

Must find zero matches.

Also verify no semantic conditionals in MCP layer:

```bash
grep -n 'if.*Status\|if.*Debt\|if.*State' cmd/solvent-mcp/tools.go
```

Must find zero matches (or only Go structural `if err != nil` patterns).

---

## Phase 9: Seed + M1 + M2

### Step 11 — Seed

```bash
task mcp:seed
```

**Post-seed state (assert):**
- 1 belief: postulated, entered, 6 debts
- 1 evidence: external_feed, v3.5.0 release
- 0 intents
- audit 0

---

## Phase 10: Full payoff sequence

### Step 12 — Retire debts + promote + authorize + ingest + falsify

Execute the full Track 2 narrative via MCP tools:

1. `solvent_retire_debt` × 6 (all debt items)
2. `solvent_promote(baseline_uuid)` → success, status=promoted
3. `solvent_authorize_action(track2, baseline_uuid, "deploy etcd v3.5.0")` → intent_state=live
4. `solvent_ingest_evidence(track2)` → verify §7.2 state
5. `solvent_ledger(track2)` → re-read baseline UUID
6. `solvent_falsify(track2, baseline_uuid)` → retracted=1
7. `solvent_ledger(track2)` → verify final state

**§7.2 post-ingest state:**
- 3 beliefs: baseline (postulated, promoted, debt {}), accommodated (entered, 4 debts), derived (entered, 6 debts)
- 3 evidence rows, 3 distinct content_sha256
- 1 live intent on baseline
- audit 0
- belief_edge 0

**Post-falsify state:**
- baseline → retracted, intent → cancelled
- other 2 beliefs still entered
- retracted = 1, audit 0, belief_edge 0

---

## Phase 11: 18 acceptance criteria

### Step 13 — Full verification

| # | Criterion | Command |
|---|---|---|
| 1 | Core freeze | `git diff --stat HEAD -- db/ internal/kernel internal/belief internal/intent internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review` |
| 2 | 107 tests | `unset FABLE_TEST_DSN && task test` |
| 3 | Frozen demos | `task demo:track1 && task demo:replay && task demo:track2 && task inspect` |
| 4 | I-7 MCP boundary | Grep check in Taskfile |
| 5 | No raw writes | `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" cmd/solvent-mcp internal/view` |
| 6 | No semantic conditionals | Read `tools.go` end to end |
| 7 | Audit from pipeline only | Code review |
| 8 | No path/dir/filename/URL in schemas | Inspect tool definitions |
| 9 | Fixture root validation | Start with missing SOLVENT_FIXTURE_ROOT → exit non-zero |
| 10 | pipeline.Run called directly | Code review |
| 11 | M1: SQLSTATE 23514 | Run `solvent_promote` on debt-carrying belief |
| 12 | M2: SQLSTATE 23503 | Run `solvent_authorize_action` on non-promoted belief |
| 13 | Post-promote state | `solvent_ledger` after Act III |
| 14 | Post-falsify state | `solvent_ledger` after falsify |
| 15 | Seed reproduces §7.1 | `task mcp:seed` from clean DB |
| 16 | Baseline UUID stable | Assert equality before/after ingest |
| 17 | §7.2 matches exactly | 3 beliefs, 3 evidence, 1 intent, audit 0 |
| 18 | Agent grounding | Read rehearsal transcript |

---

## Phase 12: README

### Step 14 — Add MCP section

Positioned as the **second act**, after the CLI quick start. Content:

1. What MCP adds (the gate demonstration)
2. Trust boundary diagram (from Plan 3 §2)
3. Six tools
4. How to build (`task mcp:build`)
5. How to seed (`task mcp:seed`)
6. How to connect a local MCP client
7. The Track 2 judge conversation (verbatim from Plan 3 §8)
8. The two database refusals
9. Promotion → live intent → falsification → cancellation
10. Honest limitation: single-belief retraction
11. `source_observed_at` deferred
12. CLI remains the primary fallback

---

## Files created

| File | Purpose |
|---|---|
| `cmd/solvent-mcp/main.go` | Stdio server, tool registration, fixture validation |
| `cmd/solvent-mcp/tools.go` | 6 handlers |
| `cmd/solvent-mcp/errors.go` | `*pgconn.PgError` → tool error result |
| `internal/view/view.go` | Read-only snapshot (SELECT only) |
| `.mcp.json` | Client config |

## Files modified

| File | Change |
|---|---|
| `go.mod` / `go.sum` | + MCP SDK dependency |
| `Taskfile.yml` | + `mcp:build`, `mcp:seed`, grep gate fix, I-7 extension |
| `README.md` | + MCP section |
| `plans/mcp/plan4.md` | Fix grep gate shell pattern (pre-implementation) |

## Files NOT modified

- `db/` — frozen
- `internal/kernel/` — frozen
- `internal/belief/` — frozen
- `internal/intent/` — frozen
- `internal/derive/` — frozen
- `internal/normalize/` — frozen
- `internal/pipeline/` — frozen
- `cmd/solvent/` — frozen
- `cmd/operator-review/` — frozen

---

## Stop conditions

STOP and report rather than improvising if:

- Frozen-core files need modification
- Kernel semantics need modification
- Schema needs modification
- belief_edge needs population
- source_observed_at needs kernel changes
- Existing exported APIs are incompatible with the plan
- The MCP SDK API differs materially from the specified design
- The Track 2 seed → ingest state differs materially from §7.2
- M1 cannot produce the real 23514 refusal
- M2 cannot produce the real 23503 refusal
- The MCP layer requires semantic business logic to function
- The 107-test baseline regresses
- Frozen demos regress

---

## Definition of done

1. 107 existing tests remain green
2. Frozen core remains byte-for-byte unchanged
3. MCP builds
4. I-7 MCP boundary check passes
5. Six tools work
6. Fixture boundary is enforced
7. M1 observed against real CockroachDB
8. M2 observed against real CockroachDB
9. Track 2 seed → ingest state verified
10. Promotion → live intent works
11. Falsification → cancellation works
12. belief_edge remains 0
13. At least one real MCP-client rehearsal succeeds
14. Transcript captured from CURRENT implementation
15. README explains honest scope
16. No unapproved scope added
