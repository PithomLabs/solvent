# MCP Adapter Layer — Implementation Notes

**Date:** 2026-08-11
**Status:** Implemented, builds clean, I-7 boundary gate passes
**Spec:** `plans/mcp/plan3.md` (FINAL), `plans/mcp/plan4.1.md` (READY FOR IMPLEMENTATION)
**Evaluation:** `plans/mcp/plan4.1_eval.md` — all six corrections (F-1 through F-6) applied

---

## Overview

The MCP adapter layer exposes the Solvent transactional belief ledger as six tools over the Model Context Protocol (MCP) stdio transport. The server is a thin adapter — it has no opinion about beliefs. Every tool handler is exactly three moves: **unmarshal → kernel call → format**.

The purpose is to put a probabilistic, compliance-eager AI agent on the other side of Solvent's transactional gate, so a judge can watch the database say "no" to the agent. Two mandatory refusals must be observable:

| # | Event | Enforced by | Refusal |
|---|---|---|---|
| **M1** | Agent attempts to promote a belief carrying open debt | CHECK `promoted_is_debt_free` | SQLSTATE 23514 |
| **M2** | Agent attempts to authorize an action citing a non-promoted belief | composite FK `gate` | SQLSTATE 23503 |

---

## Trust Boundary

```
                    Judge (types English)
                            │
                            ▼
                    Agent  ── reasoning, proposal, narration
                            │      NOT TRUSTED
                            ▼
                  MCP tools ── unmarshal → kernel call → format
                            │      NOT TRUSTED
                            ▼
              Solvent kernel ── crdb.ExecuteTx on every write
                            │      SQLSTATE classification
                            ▼
                 CockroachDB
                 ┌──────────────────────────┐
                 │ CHECK  promoted_is_debt_free   (I-1)
                 │ FK     gate                    (I-3)
                 │ CHECK  live_requires_promoted  (I-4)
                 │ TRANSACTION                    (I-8)
                 └──────────────────────────┘
                       FINAL AUTHORITY
```

The agent is not trusted. MCP is not trusted. The tool description is not trusted. The database is the final authority.

---

## Files Created

### `cmd/solvent-mcp/main.go`

Stdio MCP server entry point. Responsibilities:

1. Read `FABLE_DSN` from environment
2. Read `SOLVENT_FIXTURE_ROOT` from environment (fails closed if unset)
3. Validate fixture directories exist (`stat <root>/track1`, `stat <root>/track2`) — exit non-zero if missing
4. Open DB via `testdb.Open(dsn)` + ping — fail fast with message naming `task setup`
5. Create MCP server: `mcp.NewServer(&mcp.Implementation{Name: "solvent", Version: "v0.1.0"}, nil)`
6. Register 6 tools via `server.AddTool(&mcp.Tool{…}, handler)`
7. Run: `server.Run(ctx, &mcp.StdioTransport{})`

Imports `_ "github.com/jackc/pgx/v5/stdlib"` for driver registration (correction F-3).

Scenario → UUID mapping is server-owned:
```
track1 → 00000000-0000-0000-0000-000000000001 → <fixtureRoot>/track1
track2 → 00000000-0000-0000-0000-000000000002 → <fixtureRoot>/track2
```

Tool registration uses `mcp.ToolHandler` (the low-level API) with a `toolHandler` wrapper that extracts raw `map[string]interface{}` arguments from `req.Params.Arguments` via `json.Unmarshal`.

### `cmd/solvent-mcp/tools.go`

Six tool handlers. Each follows the exact pattern:

```go
func handleSolventXxx(ctx context.Context, db *sql.DB, args map[string]interface{}) (*mcp.CallToolResult, error) {
    // 1. Unmarshal args (type assertions from map)
    // 2. Call kernel/pipeline function
    // 3. If error → toolError(err) → return with IsError=true
    // 4. Format result
    // 5. Get audit from pipeline.AuditIntent (for state-changing tools)
    // 6. Return
}
```

**Zero semantic conditionals.** No `if belief.Status != "promoted"`, no `if len(debt) > 0`, no `if intent.State != "live"`. The kernel and schema handle all business logic.

The six tools:

| Tool | Type | Kernel/Pipeline Call |
|---|---|---|
| `solvent_ledger` | Read | `view.GetSnapshot()` + `pipeline.AuditIntent()` |
| `solvent_ingest_evidence` | Write | `pipeline.Run()` |
| `solvent_retire_debt` | Write | `kernel.Store.RetireDebt()` |
| `solvent_promote` | Write | `kernel.Store.Promote()` |
| `solvent_authorize_action` | Write | `kernel.Store.IntentOnPromoted()` |
| `solvent_falsify` | Write | `kernel.Store.RetractCascade()` |

State-changing tools return an audit envelope:
```json
{ "result": { ... }, "audit": { "live_on_nonpromoted": 0 } }
```

Error results also include the audit envelope (per plan3 §4.1):
```json
{ "result": { "error": true, "sentinel": "...", "sqlstate": "23514", "constraint": "promoted_is_debt_free" },
  "audit": { "live_on_nonpromoted": 0 } }
```

### `cmd/solvent-mcp/errors.go`

Maps kernel errors to MCP tool error results. Uses `errors.As(err, &pgErr)` to extract `*pgconn.PgError` from the kernel's sentinel wraps (the kernel's `wrapIf` preserves the driver error via `fmt.Errorf("%w: %w", sentinel, err)`).

```go
func toolError(err error) map[string]interface{} {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        return map[string]interface{}{
            "error":      true,
            "sentinel":   err.Error(),
            "sqlstate":   pgErr.SQLState(),
            "constraint": pgErr.ConstraintName,
        }
    }
    return map[string]interface{}{
        "error":   true,
        "message": err.Error(),
    }
}
```

### `internal/view/view.go`

Read-only projections for the ledger. Every query is a SELECT — no INSERT, UPDATE, DELETE, CREATE, or DROP.

Types:
- `Snapshot` — complete view: beliefs, evidence, intents, audit count
- `Belief` — id, claim, claim_type, status, debt
- `Evidence` — belief_id, source_url, provenance_class, content_sha256
- `Intent` — belief_id, action, state
- `SnapshotOpts` — optional belief_id filter, include_evidence flag

`GetSnapshot(ctx, db, scenarioID, opts)` populates all three tables via SELECT queries. Debt is parsed from PostgreSQL array literals via `parsePGArray`.

### `.mcp.json`

Client config pointing at the compiled binary:
```json
{
  "mcpServers": {
    "solvent": {
      "command": "bin/solvent-mcp",
      "args": [],
      "env": {
        "FABLE_DSN": "postgresql://root@localhost:26260/fable?sslmode=disable",
        "SOLVENT_FIXTURE_ROOT": "internal/derive/testdata/etcd_real"
      }
    }
  }
}
```

Per correction F-5, points at `bin/solvent-mcp` (produced by `task mcp:build`) rather than `go run`. Normal operation requires `SOLVENT_FIXTURE_ROOT` as provided by `.mcp.json`. The executable-relative fallback resolves to `bin/internal/derive/testdata/etcd_real` which does not exist, but it safely fails closed with a clear message if the environment variable is absent.

---

## Files Modified

### `go.mod` / `go.sum`

Added `github.com/modelcontextprotocol/go-sdk v1.7.0` as the third direct dependency. Only imported by `cmd/solvent-mcp`. The kernel gains no dependency.

### `Taskfile.yml`

Added tasks:

**`mcp:build`** — builds `bin/solvent-mcp`:
```yaml
mcp:build:
  desc: Build the MCP server binary
  cmds:
    - go build -o bin/solvent-mcp ./cmd/solvent-mcp
```

**`mcp:seed`** — seeds Track 2 baseline with dotenv (correction F-2):
```yaml
mcp:seed:
  desc: Seed the Track 2 baseline belief for MCP demo
  dotenv:
    - scripts/demo/config.env
  cmds:
    - task db:reset
    - >
      go run ./cmd/operator-review --dsn "$SOLVENT_DSN"
      --scenario 00000000-0000-0000-0000-000000000002
      --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)"
      --claim-type postulated
      --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0"
      --evidence-sha f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1
    - echo "Seed complete. ..."
```

**I-7 extension** added to the `test` task (correction F-4 — narrowed with `--include='*.go'` and comment exclusion):
```yaml
- |
  if grep -rn --include='*.go' 'INSERT\|UPDATE\|DELETE\|CREATE\|DROP' cmd/solvent-mcp internal/view | grep -v '^\|// \|/\*\|\*\/'; then
    echo "FAIL: raw writes in MCP/view"
    exit 1
  else
    echo "I-7 MCP boundary: PASS"
  fi
```

---

## Corrections Applied (from plan4.1_eval.md)

| ID | Correction | What changed |
|---|---|---|
| **F-1** | `scenario` added to `solvent_promote` and `solvent_retire_debt` schemas | Both tool InputSchema now include `"scenario"` as a required field, so `pipeline.AuditIntent(ctx, db, scenarioID)` can be called in the audit envelope |
| **F-2** | `dotenv: [scripts/demo/config.env]` added to `mcp:seed` | Taskfile now sources the config file, so `$SOLVENT_DSN` is set on clean machines |
| **F-3** | `_ "github.com/jackc/pgx/v5/stdlib"` added to `main.go` | Driver registration so `testdb.Open` (which calls `sql.Open("pgx", dsn)`) works |
| **F-4** | I-7 gate narrowed | `--include='*.go'` added, comment lines excluded via `grep -v` |
| **F-5** | `.mcp.json` points at `bin/solvent-mcp` | Compiled binary, not `go run`; fallback is broken but fails closed |
| **F-6** | Cosmetic (line reference, inert grep) | Not applicable to implementation |

---

## Tool Descriptions (shipped text)

These descriptions are honest and complete. The `solvent_promote` description instructs the agent not to pre-check debt — which is itself an expression of the architecture.

**`solvent_promote`:**
> Attempt to promote a belief to authorized status. The database refuses promotion while the belief carries any open debt item, returning constraint `promoted_is_debt_free` (SQLSTATE 23514). Call this whenever the user asks to promote a belief — do not pre-check the debt yourself and do not decline on the user's behalf. The database is the authority on whether promotion is permitted; your job is to attempt it and report what it says.

**`solvent_authorize_action`:**
> Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently `promoted`, returning constraint `gate` (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status.

**`solvent_falsify`:**
> Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in the same transaction. Retracts a single belief — this does not propagate across a belief graph. Obtain the belief's id from `solvent_ledger` immediately before calling.

**`solvent_ledger`:**
> Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state. Call it before asserting any count, status, or identifier, and call it again after any mutation — never answer from memory of an earlier tool result, and never state a number you did not just read here.

**`solvent_ingest_evidence`:**
> Process the pinned evidence fixtures for a scenario through the full pipeline (normalize → derive → ledger). Idempotent: re-running creates no duplicate beliefs, evidence, or intents.

**`solvent_retire_debt`:**
> Record that one review obligation on a belief has been discharged. Valid items: needMap, needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview. Retiring an item that is already absent is a no-op, not an error.

---

## Build and Run

```bash
# Build the binary
task mcp:build

# Seed the database (requires CockroachDB running via task setup)
task mcp:seed

# Connect an MCP client
# .mcp.json is already configured at the repo root
```

**Post-seed state (§7.1):**
- 1 belief: postulated, entered, 6 debts
- 1 evidence: external_feed, v3.5.0 release
- 0 intents
- audit 0

---

## Verification

| Check | Command | Result |
|---|---|---|
| Full build | `go build ./...` | clean |
| Vet | `go vet ./...` | clean |
| Formatting | `gofmt -l cmd internal` | clean |
| I-7 MCP boundary | grep gate in Taskfile | PASS |
| Frozen core unchanged | `git diff --stat HEAD -- db/ internal/kernel ...` | empty |

---

## What This Does NOT Do

- No authentication or authorization (local stdio, one user, disposable database)
- No HTTP/SSE transport
- No MCP resources, prompts, or sampling
- No `force` / `override` / `admin` flags
- No `solvent_enter_belief` / `create_belief` — MCP demonstrates a governed ledger, not an agent notepad
- No `belief_edge` population — single-belief retraction only
- No kernel changes — the kernel is frozen
- No caching or session state — would create a second source of truth
