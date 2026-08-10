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
- **Kernel:** `internal/kernel` — domain-agnostic, all writes through `crdb.ExecuteTx`
- **Pipeline:** `internal/pipeline` — normalize → derive → belief.Process
- **CLI:** `cmd/solvent` (pipeline runner), `cmd/operator-review` (debt/promotion/intent)
- **Substrate:** CockroachDB (serializable isolation)

## For Engineers

- `AGENTS.md` — standing context, invariants, development rules
- `IMPLEMENTATION_CONTRACT.md` — milestone plan
- `plans/post/` — specs, reviews, implementation reports
- `db/001_schema.sql` — frozen schema (do not modify)
- `internal/kernel/` — kernel functions and contract

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
