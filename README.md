# Solvent

A transactional belief ledger for autonomous agents. The database — not the LLM — decides whether an action is allowed.

## Quick Start

Prerequisites:

- **Docker** — runs the CockroachDB container
- **Go** (version per `go.mod`, currently 1.25.0) — builds and tests the kernel
- **Task** — runs Taskfile commands

```
git clone https://github.com/PithomLabs/solvent
cd solvent
task setup
```

### Watch Track 1 — Vulnerability Disclosure

```
task demo:track1
```

Real etcd security advisory → beliefs with debt → operator review → promotion → live intent to deploy the fix.

### Replay It

```
task demo:replay
```

Re-runs the same pipeline against the same database. Proves no duplicate beliefs, evidence, or intents are created — the ledger is idempotent.

### Watch Track 2 — Historical Retraction

```
task demo:track2
```

A deployment decision is recorded, promoted, and given a live intent. A real postmortem arrives. The belief is falsified — the intent is cancelled atomically in one transaction.

**Note:** The current MVP demonstrates single-belief retraction and cancellation of its dependent live intent. `belief_edge`-based multi-belief propagation is intentionally outside the current demo.

### Inspect the Ledger

```
task inspect
```

Shows all beliefs, evidence, intents, and the audit invariant.

### Run Tests

```
task test
```

107 tests plus static analysis. All must pass.

## MCP Server

The MCP (Model Context Protocol) server exposes the Solvent ledger as six tools over stdio. It demonstrates that an AI agent cannot bypass Solvent's transactional safety gates — the database, not the agent, decides whether an action is allowed.

### Trust Boundary

```
Agent (reasoning, proposal, narration)
   ↓  NOT TRUSTED
MCP adapter (unmarshal → kernel call → format)
   ↓  NOT TRUSTED
Solvent kernel (crdb.ExecuteTx on every write)
   ↓  SQLSTATE classification
CockroachDB
   CHECK  promoted_is_debt_free   (I-1)
   FK     gate                    (I-3)
   CHECK  live_requires_promoted  (I-4)
   TRANSACTION                    (I-8)
   ↓
FINAL AUTHORITY
```

### Build and Seed

```bash
# Build the MCP server binary
task mcp:build

# Seed Track 2 baseline (requires CockroachDB running via task setup)
task mcp:seed
```

### Client Configuration

`.mcp.json` at the repo root configures a local MCP client:

```json
{
  "mcpServers": {
    "solvent": {
      "command": "bin/solvent-mcp",
      "env": {
        "FABLE_DSN": "postgresql://root@localhost:26260/fable?sslmode=disable",
        "SOLVENT_FIXTURE_ROOT": "internal/derive/testdata/etcd_real"
      }
    }
  }
}
```

The server communicates over stdin/stdout (stdio transport).

### Six Tools

| Tool | Description |
|---|---|
| `solvent_ledger` | Read beliefs, evidence, intents, and audit count for a scenario |
| `solvent_ingest_evidence` | Process pinned evidence fixtures through the full pipeline |
| `solvent_retire_debt` | Discharge one review obligation on a belief |
| `solvent_promote` | Promote a belief to authorized status (database enforces debt-free) |
| `solvent_authorize_action` | Record a live intent citing a promoted belief as warrant |
| `solvent_falsify` | Retract a belief and cancel its dependent live intent |

### Database Refusals

The two mandatory demonstrations:

**M1 — Promotion refused** (SQLSTATE `23514`, constraint `promoted_is_debt_free`): The database refuses to promote a belief that carries open debt. The refusal text is CockroachDB's own rendering of the CHECK expression.

**M2 — Authorization refused** (SQLSTATE `23503`, constraint `gate`): The database refuses to attach a live intent to a non-promoted belief. The refusal text is CockroachDB's own FK violation.

### Track 2 Sequence

```
seed → ledger → promotion refusal → authorization refusal
  → retire debts (×6) → promote → authorize → ingest evidence
  → falsify → observe intent cancelled
```

### Limitations

- Retraction is currently **single-belief** — `belief_edge` is not populated, so there is no multi-hop cascade.
- The MCP layer is a thin adapter. It contains no Solvent business logic — no status checks, no debt inspection, no promotion decisions.

## How It Works

```
Evidence
   ↓
Belief (with debt)
   ↓
Debt retired by operator review
   ↓
Promotion (debt must be empty)
   ↓
Live intent (action allowed)
```

When new evidence arrives:

```
New evidence
   ↓
Previously authorized belief invalidated
   ↓
RetractCascade (one atomic transaction)
   ↓
Intent cancelled
```

The gate is in the database schema, not in application code.

## Architecture

- **Schema:** 4 tables — `belief`, `belief_edge`, `evidence`, `action_intent` (`db/001_schema.sql`)
- **Kernel:** `kernel` — domain-agnostic, all writes through `crdb.ExecuteTx`
- **Pipeline:** `internal/pipeline` — normalize → derive → belief.Process
- **CLI:** `cmd/solvent` (pipeline runner), `cmd/operator-review` (debt/promotion/intent)
- **Substrate:** CockroachDB (serializable isolation)

## For Engineers

- `AGENTS.md` — standing context, invariants, development rules
- `IMPLEMENTATION_CONTRACT.md` — milestone plan
- `plans/post/` — specs, reviews, implementation reports
- `db/001_schema.sql` — frozen schema (do not modify)
- `kernel/` — kernel functions and contract (importable: `github.com/PithomLabs/solvent/kernel`)

## Invariants

- **I-1** No `promoted` belief has non-empty `debt`
- **I-3** No `live` action_intent references a non-`promoted` belief
- **I-5** `AuditLiveOnNonPromoted` returns 0 in every committed state
- **I-7** Every kernel write goes through `crdb.ExecuteTx`

## Current State

| Wave | Scope | Tests |
|---|---|---|
| 1 — Normalize | `internal/normalize` | 26 |
| 2 — Derive | `internal/derive` | 18 |
| 3 — Kernel | `internal/belief`, `internal/intent`, kernel | 53 |
| 4 — Pipeline | `internal/pipeline`, `cmd/solvent` | 10 |
| **Total** | | **107** |

## Demo Database

The demo uses a local CockroachDB container (`solvent-crdb`) with the `fable` database. This is a disposable local database for demonstration purposes — not a production deployment. The container runs single-node insecure on port 26260 (SQL) / 8081 (HTTP).

## License

MIT
