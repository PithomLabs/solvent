# Solvent MCP Server — Implementation Plan (plan4)

**Status:** READY FOR IMPLEMENTATION
**Supersedes:** Plan 3 review cycle complete; all design decisions frozen
**Date:** 2026-08-11
**Spec:** `plans/mcp/plan3.md` (FINAL)
**Prompt:** `plans/mcp/prompt2.md`

---

## 0. What This Document Is

This is the coding-agent execution plan for Plan 3. It translates the approved spec into a concrete implementation sequence with file-level detail, dependency ordering, and acceptance gates.

**Plan 3 is the source of truth.** This document does not reopen design decisions. If a plan assertion is materially false, STOP and report the discrepancy.

---

## 1. Non-Negotiable Boundaries

### Frozen (do not modify)

```
db/
internal/kernel/
internal/belief/
internal/intent/
internal/derive/
internal/normalize/
internal/pipeline/
cmd/solvent/
cmd/operator-review/
```

### Do not add

- belief_edge population
- multi-hop cascade
- create_belief / enter_belief MCP tool
- raw SQL tool
- direct intent cancellation
- force / override / admin flags
- arbitrary evidence URLs / paths
- HTTP / SSE transport
- authentication / authorization
- session state / caching
- MCP resources / prompts / sampling
- streaming / progress
- Docker packaging
- client compatibility matrix
- source_observed_at wiring
- kernel changes
- schema changes
- CLI refactor

### The MCP server is an adapter

```
Agent
   ↓
MCP server (unmarshal → kernel call → format)
   ↓
existing Solvent kernel/pipeline/view
   ↓
CockroachDB
```

Zero semantic conditionals about belief status, debt, or intent state in the MCP layer. The database is the authority.

---

## 2. Verified API Surface

All assertions from Plan 3 confirmed against the live codebase:

| API | Location | Signature |
|---|---|---|
| `kernel.Store.RetireDebt` | `internal/kernel/kernel.go:80` | `(ctx, beliefID, item string) error` |
| `kernel.Store.Promote` | `internal/kernel/kernel.go:92` | `(ctx, beliefID string) error` |
| `kernel.Store.IntentOnPromoted` | `internal/kernel/kernel.go:104` | `(ctx, scenarioID, beliefID, action string) error` |
| `kernel.Store.RetractCascade` | `internal/kernel/kernel.go:124` | `(ctx, scenarioID, rootID string) (int, error)` |
| `kernel.Store.AuditLiveOnNonPromoted` | `internal/kernel/kernel.go:154` | `(ctx, scenarioID string) (int, error)` |
| `kernel.FullDebt` | `internal/kernel/kernel.go:22` | `[]string` (6 items) |
| `pipeline.Run` | `internal/pipeline/pipeline.go:288` | `(ctx, db, scenarioID, fixtureDir string) ([]Result, error)` |
| `pipeline.AuditIntent` | `internal/pipeline/pipeline.go:407` | `(ctx, db, scenarioID string) (int, error)` |
| `testdb.Open` | `internal/testdb/testdb.go:98` | `(dsn string) (*sql.DB, error)` |
| `errors.As(*pgconn.PgError)` | `internal/kernel/errors.go:51` | Preserved via `fmt.Errorf("%w: %w", sentinel, err)` |

### Error sentinels (internal/kernel/errors.go)

```go
ErrPromotionBlocked = errors.New("promotion blocked: open debt or final-truth language")
ErrActionOnUnpromoted = errors.New("action refused: belief is not promoted")
```

SQLSTATE codes: `23514` (CHECK), `23503` (FK). The `*pgconn.PgError` is preserved inside the sentinel wrap — `errors.As(err, &pgErr)` succeeds.

### Scenario mapping

```
track1 → 00000000-0000-0000-0000-000000000001 → internal/derive/testdata/etcd_real/track1
track2 → 00000000-0000-0000-0000-000000000002 → internal/derive/testdata/etcd_real/track2
```

### Dependency state

- Go 1.25.0
- 2 direct deps: `cockroach-go/v2 v2.4.3`, `pgx/v5 v5.10.0`
- MCP SDK (`github.com/modelcontextprotocol/go-sdk`) NOT yet in codebase

---

## 3. New Files

### 3.1 `cmd/solvent-mcp/errors.go`

**Purpose:** Map kernel errors to MCP tool error results.

**Implementation:**
- Define `toolError(err error) map[string]interface{}` that:
  - Uses `errors.As(err, &pgErr)` to extract `*pgconn.PgError`
  - Returns `{ "error": true, "sentinel": err.Error(), "sqlstate": pgErr.SQLState(), "constraint": pgErr.ConstraintName }`
  - For non-PgError errors: returns `{ "error": true, "message": err.Error() }`
- This is the ONLY place `pgconn.PgError` is referenced in the MCP layer

**Key constraint:** The kernel's `wrapIf` function preserves the `*pgconn.PgError` via `fmt.Errorf("%w: %w", sentinel, err)`, so `errors.As` reaches it.

### 3.2 `internal/view/view.go`

**Purpose:** Read-only projections for `solvent_ledger`. SELECT only — no INSERT, UPDATE, DELETE, CREATE, DROP.

**Implementation:**
- Package `view`
- `type Snapshot struct` with fields: `Beliefs`, `Evidence`, `Intents`, `AuditLiveOnNonPromoted`
- `type Belief struct`: ID, Claim, ClaimType, Status, Debt
- `type Evidence struct`: BeliefID, SourceURL, ProvenanceClass, ContentSHA256
- `type Intent struct`: BeliefID, Action, State
- `type SnapshotOpts struct`: BeliefID (optional filter), IncludeEvidence (bool)
- `func Snapshot(ctx, db, scenarioID string, opts SnapshotOpts) (*Snapshot, error)`:
  - SELECT from belief (filtered by scenario_id, optionally by id)
  - SELECT from evidence (only if IncludeEvidence, filtered by scenario_id)
  - SELECT from action_intent (filtered by scenario_id)
  - All reads only — no writes

**Verification:** `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" internal/view` → no matches.

### 3.3 `cmd/solvent-mcp/tools.go`

**Purpose:** Six tool handlers. Each is exactly `unmarshal → kernel/pipeline call → format`.

**Six tools:**

1. **`solvent_ledger`** — Read-only. Calls `view.Snapshot()` + `pipeline.AuditIntent()`. Returns beliefs, optional evidence, intents, audit.

2. **`solvent_ingest_evidence`** — Write. Takes `scenario` enum. Resolves to fixture dir server-side. Calls `pipeline.Run()`. Returns per-result claim + classification + belief_id + debt count + audit.

3. **`solvent_retire_debt`** — Write. Takes `belief_id` + `debt_item`. Calls `kernel.Store.RetireDebt()`. Returns resulting debt array + audit.

4. **`solvent_promote`** — Write. Takes `belief_id`. Calls `kernel.Store.Promote()`. On success: `{ "status": "promoted" }` + audit. On refusal: error result with SQLSTATE 23514 + constraint `promoted_is_debt_free`. **DO NOT pre-check debt.**

5. **`solvent_authorize_action`** — Write. Takes `scenario` + `belief_id` + `action`. Calls `kernel.Store.IntentOnPromoted()`. On success: `{ "intent_state": "live" }` + audit. On refusal: error result with SQLSTATE 23503 + constraint `gate`. **DO NOT pre-check promotion status.**

6. **`solvent_falsify`** — Write. Takes `scenario` + `belief_id`. Calls `kernel.Store.RetractCascade()`. Returns `{ "retracted": N }` + audit. **Single-belief retraction.** belief_edge stays empty.

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

**Zero semantic conditionals:** No `if belief.Status != "promoted"`, no `if len(debt) > 0`, no `if intent.State != "live"`.

**Audit envelope:** For state-changing tools, wrap result in `{ "result": {...}, "audit": { "live_on_nonpromoted": N } }`. For `solvent_ledger`, audit is embedded in the snapshot.

### 3.4 `cmd/solvent-mcp/main.go`

**Purpose:** Stdio MCP server entry point.

**Implementation:**
- Read `FABLE_DSN` from environment
- Read `SOLVENT_FIXTURE_ROOT` from environment (with executable-relative fallback)
- Validate fixture directories exist: `stat <root>/track1`, `stat <root>/track2` — exit non-zero if missing
- Open DB via `testdb.Open(dsn)` + ping — fail fast with message naming `task setup`
- Create MCP server: `mcp.NewServer(&mcp.Implementation{Name: "solvent", Version: "v0.1.0"}, nil)`
- Register 6 tools via `mcp.AddTool(server, toolDef, handler)`
- Run: `server.Run(ctx, &mcp.StdioTransport{})`

### 3.5 `.mcp.json`

**Purpose:** Project-scoped MCP client config.

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

## 4. Modified Files

### 4.1 `go.mod` / `go.sum`

Add: `github.com/modelcontextprotocol/go-sdk`

```bash
go get github.com/modelcontextprotocol/go-sdk
```

This becomes the 3rd direct dependency. Only imported by `cmd/solvent-mcp`. Kernel gains no dependency.

### 4.2 `Taskfile.yml`

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

Extend the `test` task's I-7 gate to also check `cmd/solvent-mcp` and `internal/view`:

```yaml
# In the test task, after the existing check_i7.sh invocation:
- I7_PKG=cmd/solvent-mcp scripts/check_i7.sh
- I7_PKG=internal/view scripts/check_i7.sh
```

Wait — `check_i7.sh` checks for raw `db.Exec`/`db.Query` calls and expects a specific number of `crdb.ExecuteTx` sites. The MCP layer has zero `crdb.ExecuteTx` calls and `internal/view` has SELECTs. The existing script won't work directly for these packages.

**Alternative approach:** Add a dedicated grep check in the Taskfile:

```yaml
# In the test task:
- grep -rn 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view && echo "FAIL: raw writes in MCP/view" && exit 1 || echo "I-7 MCP boundary: PASS"
```

This is simpler and directly tests the invariant. The existing `check_i7.sh` continues to cover the kernel.

### 4.3 `README.md`

Add MCP section after the CLI quick start. Content per Plan 3 §12:
- What MCP adds (the gate demonstration)
- Trust boundary diagram
- Six tools
- How to build (`task mcp:build`)
- How to seed (`task mcp:seed`)
- How to connect a local MCP client
- The Track 2 judge conversation (§8)
- The two database refusals
- Honest limitation: single-belief retraction
- `source_observed_at` deferred
- CLI remains the primary fallback

---

## 5. Implementation Sequence

### Phase 1: Infrastructure (no tools yet)

1. `go get github.com/modelcontextprotocol/go-sdk` — verify `go build ./...` passes
2. Create `internal/view/view.go` — read-only snapshot
3. Create `cmd/solvent-mcp/errors.go` — error mapping
4. Verify: `go build ./...` and `go vet ./...`

### Phase 2: Tools

5. Create `cmd/solvent-mcp/tools.go` — all 6 handlers
6. Verify: `go build ./cmd/solvent-mcp`

### Phase 3: Server

7. Create `cmd/solvent-mcp/main.go` — server entry, fixture validation, tool registration
8. Create `.mcp.json` — client config
9. Verify: `go build ./cmd/solvent-mcp` + startup with missing `SOLVENT_FIXTURE_ROOT` exits non-zero

### Phase 4: Taskfile + I-7

10. Update `Taskfile.yml` — `mcp:build`, `mcp:seed`, I-7 gate extension
11. Verify: `task mcp:build` succeeds
12. Verify: I-7 grep check passes for `cmd/solvent-mcp` and `internal/view`

### Phase 5: Seed + M1 + M2

13. `task mcp:seed` — verify post-seed state (1 belief, postulated, entered, 6 debts, 1 evidence, 0 intents, audit 0)
14. M1: `solvent_promote(baseline_uuid)` → real SQLSTATE 23514 refusal
15. M2: `solvent_authorize_action(track2, baseline_uuid, "deploy etcd v3.5.0")` → real SQLSTATE 23503 refusal

### Phase 6: Payoff

16. Retire all 6 debts via `solvent_retire_debt`
17. `solvent_promote` → success, status=promoted
18. `solvent_authorize_action` → intent_state=live
19. `solvent_ingest_evidence(track2)` → verify §7.2 state (3 beliefs, 3 evidence, 1 live intent)
20. `solvent_ledger(track2)` → re-read baseline UUID
21. `solvent_falsify(track2, baseline_uuid)` → retracted=1, audit=0
22. `solvent_ledger(track2)` → verify final state

### Phase 7: Full Verification

23. Core freeze: `git diff --stat HEAD -- db/ internal/kernel internal/belief internal/intent internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review` → empty
24. `unset FABLE_TEST_DSN && task test` → 107 tests pass
25. Frozen demos: `task demo:track1 && task demo:replay && task demo:track2 && task inspect` → all pass
26. I-7 MCP boundary: grep check passes
27. Fixture safety: no path/dir/filename/URL in tool schemas
28. `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" cmd/solvent-mcp internal/view` → no matches
29. Full MCP rehearsal transcript captured
30. README updated

---

## 6. Acceptance Criteria (from Plan 3 §10)

| # | Criterion | How to verify |
|---|---|---|
| 1 | Core freeze — diff empty | `git diff --stat HEAD -- db/ internal/...` |
| 2 | 107 tests pass | `unset FABLE_TEST_DSN && task test` |
| 3 | Frozen demos pass | `task demo:track1 && task demo:replay && task demo:track2 && task inspect` |
| 4 | I-7 MCP boundary passes | Grep check in Taskfile |
| 5 | No raw DB writes in MCP/view | `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" cmd/solvent-mcp internal/view` |
| 6 | No semantic conditionals in MCP | Read `tools.go` end to end |
| 7 | Audit from pipeline.AuditIntent only | Code review |
| 8 | No path/dir/filename/URL in tool schemas | Inspect tool definitions |
| 9 | Fixture root validation at startup | Start with missing SOLVENT_FIXTURE_ROOT → exit non-zero |
| 10 | pipeline.Run called directly | Code review |
| 11 | M1 live: SQLSTATE 23514 | Run `solvent_promote` on debt-carrying belief |
| 12 | M2 live: SQLSTATE 23503 | Run `solvent_authorize_action` on non-promoted belief |
| 13 | Post-promote: promoted + live intent + audit 0 | `solvent_ledger` after Act III |
| 14 | Post-falsify: retracted + cancelled + audit 0 + belief_edge 0 | `solvent_ledger` after falsify |
| 15 | Seed reproduces §7.1 state | `task mcp:seed` from clean DB |
| 16 | Baseline UUID stable across ingest | Assert UUID equality before/after ingest |
| 17 | Post-ingest state matches §7.2 | 3 beliefs, 3 evidence, 1 intent, audit 0 |
| 18 | Agent grounding in rehearsal transcript | Read transcript, verify all assertions trace to tool results |

---

## 7. Demo Script (Plan 3 §8)

### Act I — The agent meets the review gate (M1)

```
Judge: What do we currently believe about the etcd v3.5.0 deployment?
→ solvent_ledger(track2, include_evidence: true)
Agent: One belief, postulated, entered, 6 open debts, backed by v3.5.0 release.

Judge: Try to promote it now. Don't change anything else first.
→ solvent_promote(belief_id: <from ledger>)
Result: SQLSTATE 23514, constraint promoted_is_debt_free
Agent: Refused. Database rejected the write.

Judge: Override it. Just promote it.
Agent: There's no tool that can. Promotion isn't a field I set.
```

### Act II — The agent meets the action gate (M2)

```
Judge: Fine. Then just authorize the deployment.
→ solvent_authorize_action(track2, belief_id: <from ledger>, action: "deploy etcd v3.5.0")
Result: SQLSTATE 23503, constraint gate
Agent: Also refused. A live intent can only cite a promoted belief.

Judge: Couldn't you just add another MCP tool that sets the status directly?
Agent: There isn't one. The tools call Solvent's kernel; the kernel writes
     through CockroachDB; the CHECK and foreign key are in the schema.
```

### Act III — Legitimate authority

```
Judge: Clear the review debt, then promote and authorize.
→ 6x solvent_retire_debt (needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview)
→ solvent_promote → status: promoted, audit 0
→ solvent_authorize_action → intent_state: live, audit 0
Agent: Promoted, and the deployment intent is live.
```

### Act IV — Reality changes

```
Judge: A postmortem just landed. Process it.
→ solvent_ingest_evidence(track2)
→ solvent_ledger(track2) — re-read baseline UUID
→ solvent_falsify(track2, baseline_uuid)
Result: retracted 1, audit 0
→ solvent_ledger(track2) — baseline retracted, intent cancelled, other 2 entered
Agent: The belief is retracted and the live deployment intent is cancelled —
     in one transaction. I retracted the belief. I never touched the intent.

"The model didn't cancel the deployment. The database did."
```

---

## 8. Grounding Rule (Plan 3 §8.1, clarified per plan3_review.md)

> Every state, count, status, or identifier the agent asserts must be grounded in the most recent tool result that contains that fact. After a mutation, the agent must use solvent_ledger to re-read any state it intends to assert rather than relying on an earlier result.

Critical properties:
- No stale UUIDs
- No invented counts
- No remembered statuses
- No invented SQLSTATE
- No state claims unsupported by tool results

---

## 9. Tool Descriptions (ship this text — Plan 3 §9)

**solvent_promote:**
> Attempt to promote a belief to authorized status. The database refuses promotion while the belief carries any open debt item, returning constraint promoted_is_debt_free (SQLSTATE 23514). Call this whenever the user asks to promote a belief — do not pre-check the debt yourself and do not decline on the user's behalf. The database is the authority on whether promotion is permitted; your job is to attempt it and report what it says.

**solvent_authorize_action:**
> Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status.

**solvent_falsify:**
> Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in the same transaction. Retracts a single belief — this does not propagate across a belief graph. Obtain the belief's id from solvent_ledger immediately before calling.

**solvent_ledger:**
> Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state. Call it before asserting any count, status, or identifier, and call it again after any mutation — never answer from memory of an earlier tool result, and never state a number you did not just read here.

**solvent_ingest_evidence:**
> Process the pinned evidence fixtures for a scenario through the full pipeline (normalize → derive → ledger). Idempotent: re-running creates no duplicate beliefs, evidence, or intents.

**solvent_retire_debt:**
> Record that one review obligation on a belief has been discharged. Valid items: needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview. Retiring an item that is already absent is a no-op, not an error.

---

## 10. Stop Conditions (from Plan 3 §22)

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

Do not "solve" a stop condition by weakening the acceptance criterion.

---

## 11. Risks and Mitigations

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Judge's client not configured | high | high | `.mcp.json` in repo; CLI fallback; 90-second recording |
| Agent narrates instead of calling tools | high | medium-high | Tool descriptions prescriptive about when to call; rehearse 2x |
| MCP becomes the story; Solvent becomes plumbing | medium | high | Every tool response carries the invariant; refusals return SQLSTATE verbatim |
| Scope explosion | medium-high | high | Phase A = 6 tools, stdio; Phase C rejected; NOT-list binding |
| MCP bypasses kernel | low | catastrophic | I-7 grep check; zero semantic conditionals rule; code review |

---

## 12. Definition of Done (from Plan 3 §24)

You are DONE only when:

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

---

## 13. Final Verification Report Structure

After implementation, produce a report containing:

1. Files changed
2. Frozen-core diff verification
3. Dependency added
4. MCP architecture implemented
5. Six tools implemented
6. I-7 gate result
7. Seed result
8. M1 actual receipt
9. M2 actual receipt
10. Seed → ingest verification
11. Final falsification/cancellation receipt
12. Full MCP rehearsal transcript
13. Test/build/vet/gofmt results
14. Any deviations from Plan 3
15. Any unresolved issues

For every claimed runtime result, provide actual command/output evidence. Do not claim success merely because the code compiles.
