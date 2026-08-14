# Solvent

A transactional belief ledger for autonomous agents. The database — not the LLM — decides whether an action is allowed.

## Live demo

**https://byb43s8nh2.us-west-2.awsapprunner.com/demo**

Deployed on AWS App Runner against a CockroachDB Cloud Serverless cluster. Three screens, and the
database refuses you on two of them:

| Screen | What you do | What the database says |
|---|---|---|
| **ASK** | Search 7,239 real etcd issues, then try to promote the belief and authorize the deployment | `23514 · promoted_is_debt_free` and `23503 · gate` |
| **DISCHARGE** | Cite retrieved evidence and record six review obligations | The contradiction sweep is refused until something is actually cited |
| **FALSIFY** | Introduce the incident that destroys the belief, then try to retract it | `23514 · live_requires_promoted` — the retraction cannot leave a live intent behind |

Every number on screen is measured at request time. Nothing is scripted, and there is no fallback path
that fabricates a result when a real one is unavailable.

Also live: [`/proof`](https://byb43s8nh2.us-west-2.awsapprunner.com/proof) — the same race run three
ways, where correct application logic on a weak schema commits silent corruption. And
[`/ledger`](https://byb43s8nh2.us-west-2.awsapprunner.com/ledger) — the read-only ledger.

```
Judge's browser
      ↓  HTTPS
AWS App Runner  us-west-2       Go binary, no framework, ~25 MB image from ECR
      ├──────→ Amazon Bedrock   amazon.titan-embed-text-v2:0, 1024-d, via an instance role
      ↓  pgx
CockroachDB Cloud Serverless    great-goat · v26.2.5 · primary region aws-us-west-2
```

App Runner's region and the cluster's primary region are the same, so no request in the demo path pays a
cross-region round trip. `task deploy` refuses to deploy if that ever stops being true.

## The CockroachDB tools this submission claims

| | Tool | Status |
|---|---|---|
| 1 | **Distributed Vector Indexing** | ✅ verified |
| 2 | **ccloud CLI** | ✅ verified |

**What was actually done with them**

- **Distributed Vector Indexing** — `corpus_issue.embedding` is a `VECTOR(1024)` column indexed by
  `corpus_issue_embedding_idx (scenario_id, embedding vector_cosine_ops)`. All **7,239 of 7,239** rows
  carry a genuine Amazon Titan v2 embedding, and the deployed `/demo` search performs live cosine ANN
  retrieval (`<=>`) against them on every query. Verified end to end: the deployed service returns
  `0.372424`, `0.387913`, `0.401092` for the deployment question and `0.199509` for the integrity
  question — **byte-identical to a local run against a different cluster**, which is what shows the
  retrieval is deterministic and the hosted vectors really are the checkpointed ones.
- **ccloud CLI** — used interactively through Claude Code for cluster introspection (`auth whoami`,
  cluster identity, region, version), and programmatically by `task deploy` as a **fail-closed JSON
  preflight**: `scripts/ccloud_preflight.sh` parses `ccloud cluster list --output json` and aborts the
  deploy if the cluster id, name, cloud provider, version, state, or primary region is not what the
  deployment expects. It selects the cluster by id rather than by position, because the org holds two.

**Cloud Managed MCP Server:** additionally configured; endpoint and cluster documented below;
end-to-end verification pending an owner-issued service-account key. It is not claimed as a verified
tool until that verification runs — `scripts/cloud_mcp_verify.sh` is the one command that closes it.

> The composite foreign-key gate, the `CHECK` re-evaluation on cascade, and serializable isolation are
> **not** counted above. They are the CockroachDB memory-layer architecture this whole project is built
> on, described under [Architecture](#architecture) — not rubric tool claims. Stretching one mechanism
> to satisfy two different claims would misrepresent both.

## Quick Start — local development

The live URL above is the demo. This section is for running the kernel and the demo tracks on your own
machine.

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

The full verification suite: every package's tests, `go vet`, `gofmt`, the I-7 static write-path gate,
the MCP boundary greps, and `scripts/mcp_verify.sh`, which speaks JSON-RPC to the MCP server over stdio.
All must pass.

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

Because the last row is not verified, **this server is not counted among the claimed tools above.** It
is listed as additionally configured, and it moves into the claimed table only when the verification
below passes — not before.

The single remaining step issues a service-account API key in the Cloud Console (or completes the
interactive OAuth flow) and runs one query through it. That step creates a credential, so it is
deliberately left to the repository owner rather than automated here. Everything around it is done:

```bash
export CC_MCP_API_KEY='<service-account-key-from-the-Cloud-Console>'
scripts/cloud_mcp_verify.sh
```

The script sends one read-only query and reports which configuration worked. It tries **both**
`mcp-cluster-id` values, because that header has never been confirmed: the Cloud Console may expect the
cluster UUID (`c995cb24-e07b-4470-bfb3-344c44ce0de1`) or the SQL-DNS prefix (`great-goat-30894`). Rather
than guess in prose, the script tests both and tells you. It reads the key from the environment only,
never from a tracked file.

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

## Retrieval, and what it does not surface

This is a measured limitation, not a caveat. It is also the reason the product is shaped the way it is,
so it is stated plainly rather than buried.

Ask the corpus the question an engineer would actually ask before a deployment:

```
"Is etcd v3.5.x safe to deploy?"
  #19220  0.372424   go.etcd.io/etcd/client/v3 updates
  #12987  0.387913   3.4 to 3.5 upgrade may panic
  #12788  0.401092   Using client in v3.5.0 and etcd server with v3.4.14
  #13269  0.409815   a security problem in using etcd v3.4.15
```

Plausible, on-topic, and it does not contain the incident that matters. Ask about integrity instead:

```
"etcd v3.5 data inconsistency after upgrade"
  #14139  0.199509   inconsistent data in etcd 3.5.4
  #9630   0.277597   Inconsistent data in an etcd cluster
  #13580  0.286439   Data inconsistency in etcd version 3.4.16
```

`0.199509` against `0.372424`. The embedding is behaving correctly — a deployment-phrased question is
simply nearer to release-process material than to defect reports. **An agent that asks "is this safe to
deploy?" and reads the top-k will not see the incident that matters most.**

And the canonical incident is worse than that. `#13766 · Inconsistent revision and data occurs`, the
issue that actually destroys the claim, sits at distance `0.594920` from it — **rank 573 of 7,239**. No
top-k, no threshold and no reranker puts that in front of a judge. Which is why, on screen 3, a *human*
introduces it by URL, and the screen says so: *"Not retrieved — introduced by URL."*

The response is not to tune the retrieval. It is to stop treating retrieval as an authority:

> Retrieval proposes candidates. It decides nothing. The database decides.

Screen 2 makes that concrete. The judge discharges `needContradictionSweep` by citing `#14139` at
`0.200` — a genuinely relevant issue, and still the wrong conclusion. Solvent records the citation *and*
the outcome, and then refuses to let the authorization outlive the belief it was granted on. The closing
audit line says exactly that:

> `needContradictionSweep` was discharged by citing #14139 at 0.200. The sweep was performed. The
> judgment was wrong. Solvent recorded both, and refused to let the intent outlive the belief.

A demo where the human reasons correctly and the database never has to intervene would demonstrate
nothing.

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

### The memory layer — why this is a database project, not an app project

Not counted among the tool claims above, because it is the substrate rather than a tool. It is also the
only reason the demo works.

Three schema-level mechanisms carry the entire guarantee:

```sql
-- 1. Debt must be empty before a belief can be promoted.
CHECK (status <> 'promoted' OR debt = '{}')          -- promoted_is_debt_free -> 23514

-- 2. An intent may only cite a belief that is promoted RIGHT NOW.
FOREIGN KEY (belief_id, belief_status)
  REFERENCES belief(id, status) ON UPDATE CASCADE     -- gate -> 23503

-- 3. And a live intent may not survive its belief losing that status.
CHECK (state <> 'live' OR belief_status = 'promoted') -- live_requires_promoted -> 23514
```

The third one is the interesting one, and it only works because of a property we had to measure rather
than assume: **CockroachDB re-evaluates `CHECK` constraints on rows modified by `ON UPDATE CASCADE`.**
So retracting a belief cascades the new status into every citing intent, and the `CHECK` on those rows
then refuses the whole transaction. The application never checks this. It cannot forget to.

Verified on the deployed cluster, not just locally — `great-goat`, v26.2.5, against DDL generated by
`db/001_schema.sql` itself. The transcript is `proof/act6_tier_probe.log`, and
[`/proof`](https://byb43s8nh2.us-west-2.awsapprunner.com/proof) shows what the same race does to a
schema without these constraints: under `READ COMMITTED`, both transactions commit, no error is raised
anywhere, and a live intent ends up authorizing action on a retracted belief. Correct application logic,
silent corruption.

**On `belief_edge` and traversal.** CockroachDB does not traverse `belief_edge`. The recursive walk is a
`WITH RECURSIVE` CTE in application code. The traversal is application logic; what the database enforces
is that the traversal cannot finish having left a live intent behind. That distinction matters if you are
reading `db/001_schema.sql` looking for the graph logic — it is not there, and it is not supposed to be.

## Security disclosure

The exposed `bchat` credential was rotated on 2026-08-14; historical Git objects were deliberately not
rewritten. Rotation invalidates the credential; it does not remove the string from history, so a clone
still contains it — it simply no longer authenticates. Rewriting published history immediately before a
submission deadline was judged the greater risk.

No credential is committed now. `scripts/demo/config.env` is tracked and therefore deliberately leaves
the cloud DSN unset, with the reason stated inline; the Cloud MCP service-account key is read from the
environment and never written to a tracked file.

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

| Layer | Packages |
|---|---|
| Normalize | `internal/normalize` |
| Derive | `internal/derive` |
| Kernel | `kernel`, `internal/belief`, `internal/intent` |
| Pipeline | `internal/pipeline`, `cmd/solvent` |
| Corpus + retrieval | `internal/corpus`, `cmd/corpus-ingest` |
| Decision wizard | `internal/wizard`, `demo/cloud/web` |
| MCP adapter | `cmd/solvent-mcp` |

## Databases

**The demo runs on CockroachDB Cloud Serverless** (`great-goat`, v26.2.5, primary region
`aws-us-west-2`) — that is what the live URL talks to, and what every deployed measurement in this
README came from.

For local development, `task setup` starts a disposable single-node CockroachDB container
(`solvent-crdb`, insecure, SQL on 26260 / HTTP on 8081) holding the `fable` database. It is a
development dependency, not the demo.

Both carry the same schema, applied in four layers: `db/001_schema.sql` (the frozen ledger),
`db/002_corpus.sql` (corpus + vector index), `db/003_wizard.sql` (citation relation, refusal log), and
`db/004_debt_vocabulary.sql` (the deployment debt vocabulary). `001` holds the only copy of the debt
default, so a fresh database reads it there — but an existing database never re-applies `001`, which is
why `004` carries an `ALTER COLUMN ... SET DEFAULT` rather than editing `001` alone.

## License

MIT
