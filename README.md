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

- The MCP layer is a thin adapter. It contains no Solvent business logic — no status checks, no debt inspection, no promotion decisions.
- `solvent-mcp` speaks stdio only. It is a local subprocess, not a hosted service.
- Retraction cascades across `belief_edge`, but the graph is deliberately small. `belief_edge` has no
  writer in the kernel — invariant I-7 pins the kernel at exactly seven write paths — so the demo's
  single derivation edge is filed by `internal/demoseed` outside the kernel. `RetractCascade`
  traverses whatever edges exist; the demo simply does not manufacture a deep graph it cannot justify
  from the evidence.

## CockroachDB Cloud Managed MCP Server

This is a **different** server from `solvent-mcp` above. `solvent-mcp` is ours and exposes the
Solvent ledger as six domain tools. The Cloud Managed MCP Server is Cockroach Labs' own hosted
endpoint, which connects an agent directly to the cluster for read-only SQL exploration. Both are
useful and neither substitutes for the other.

**Endpoint:** `https://cockroachlabs.cloud/mcp`

**Cluster:** `great-goat` · `c995cb24-e07b-4470-bfb3-344c44ce0de1` · AWS · v26.2.5 · primary region
`aws-us-west-2`

### Configuration

Add it to Claude Code with read-only access. The Cloud Console's MCP page issues the config snippet
and the credential; the shape is:

```bash
claude mcp add cockroachdb-cloud https://cockroachlabs.cloud/mcp \
  --transport http \
  --header "mcp-cluster-id: <cluster-id-from-the-Cloud-Console>" \
  --header "Authorization: Bearer <service-account-api-key>"
```

Interactive OAuth 2.1 works for a human at a terminal; a service-account API key is what an
autonomous agent needs.

### ⚠️ Verification status: NOT yet verified end-to-end

Stated plainly rather than claimed, because the hackathon scores the integration and not the
paragraph:

| Fact | Status |
|---|---|
| Endpoint exists and enforces authorization | **verified** — unauthenticated `POST` returns `401 {"error":"invalid_request","error_description":"Authorization required"}` |
| Cluster identity and region | **verified** via `ccloud cluster list` and `SHOW REGIONS FROM DATABASE` |
| `ccloud` CLI authenticated against the org | **verified** — `ccloud auth whoami` reports `Pithom Labs (org-32ndt)` |
| An agent actually querying `great-goat` through this endpoint | **NOT verified** |

The single remaining step is issuing a service-account API key in the Cloud Console (or completing
the interactive OAuth flow) and running one query through it. That step creates a credential, so it
is deliberately left to the repository owner rather than automated here.

Note also that the `mcp-cluster-id` header value has not been confirmed: the Cloud Console may expect
the cluster UUID above or the SQL-DNS prefix (`great-goat-30894`). Take it from the Console snippet
rather than from this file.

### Three questions a judge can paste into Claude Code

Once configured, these exercise the managed server against the real cluster. All are read-only.

1. *"List the tables in the `fable` database and show me the `CREATE TABLE` for `action_intent`."*
   — surfaces the `gate` composite foreign key with `ON UPDATE CASCADE` and the
   `live_requires_promoted` check constraint, which are the two things the whole demo rests on.

2. *"How many rows are in `corpus_issue`, how many have a non-null `embedding`, and what vector index
   exists on that table?"* — should report **7,239 / 7,239** and
   `corpus_issue_embedding_idx (scenario_id, embedding vector_cosine_ops)`.

3. *"Count live `action_intent` rows whose belief is not promoted, for scenario
   `00000000-0000-0000-0000-000000000002`."* — Solvent's standing audit invariant. It must be **0**,
   and it must stay 0 in every committed state.

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

- **Schema:** 6 tables in two layers.
  - *Ledger* (frozen, `db/001_schema.sql`): `belief`, `belief_edge`, `evidence`, `action_intent`.
    Carries no vectors; the epistemic gates live here.
  - *Corpus* (`db/002_corpus.sql`): `corpus_issue`, `belief_corpus_citation`. External
    institutional memory with a `VECTOR(1024)` column and a CockroachDB vector index
    prefixed by `scenario_id`. Retrieval proposes candidates; it decides nothing.
- **Corpus:** the etcd issue history, ingested as unattached external evidence.
  Captured with `task corpus:fetch` into the gitignored `corpus-data/`, then loaded
  with `task corpus:ingest`. The snapshot is not vendored: its `.meta.json` sidecar
  records the capture time, the exact REST parameters, the counts, and a SHA-256 of
  the NDJSON, so a given corpus state can be cited precisely and the identical
  artifact can be loaded into more than one database.
  **No belief is created from the corpus.** Thousands of issues are memory; beliefs
  stay sparse and are only ever entered through the kernel.
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
