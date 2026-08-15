# Solvent

A transactional belief ledger for autonomous agents. The database — not the LLM — decides whether an
action is allowed.

**Live demo: https://byb43s8nh2.us-west-2.awsapprunner.com/demo**
Also live: [`/proof`](https://byb43s8nh2.us-west-2.awsapprunner.com/proof) (the control experiment)
and [`/ledger`](https://byb43s8nh2.us-west-2.awsapprunner.com/ledger) (read-only ledger).

---

## What Solvent is

Persistent memory for evidence, authority, and change, built around one separation:

```
Memory / Retrieval
       ↓
Belief
       ↓
Authority
       ↓
CockroachDB invariant
```

> **Retrieval and judgment are allowed to be wrong. Authority is what the database constrains.**

An agent searches memory, cites what it relied on, and forms a belief. That belief carries *debt* —
review obligations that must be discharged before it can be promoted. Only a promoted belief can
authorize an action. When the belief is later invalidated, the database refuses to let the
authorization outlive it.

## Why ordinary RAG is insufficient

Retrieval returns what is nearest, not what matters. Usually those overlap; when they don't, nothing
in the pipeline notices — and the authority to act was granted by the same context window that
produced the answer. There is no durable boundary between evidence, belief, and permission.

Solvent does not claim better retrieval. It claims that retrieval must not silently become authority.

## Architecture

```
Judge's browser
      ↓  HTTPS
AWS App Runner  us-west-2       Go binary, no framework, ~25 MB image from ECR
      ├──────→ Amazon Bedrock   amazon.titan-embed-text-v2:0, 1024-d, via an instance role
      ↓  pgx
CockroachDB Cloud Serverless    v26.2.5 · multi-region · primary region aws-us-west-2
```

App Runner's region and the cluster's primary region are the same, so no request in the demo path
pays a cross-region round trip. `task deploy` refuses to deploy if that stops being true.

Seven tables in two layers: the four frozen ledger tables (`belief`, `belief_edge`, `evidence`,
`action_intent`) and the corpus/wizard layer (`corpus_issue`, `belief_corpus_citation`,
`refusal_log`). The kernel is domain-agnostic Go; the wizard is one embedded HTML file and a handful
of handlers.

## Three demo beats

| Screen | What you do | What the database says |
|---|---|---|
| **ASK** | Search 7,239 real etcd issues, then try to promote the belief and authorize the deployment | `23514 · promoted_is_debt_free` and `23503 · gate` |
| **DISCHARGE** | Cite retrieved evidence and record six review obligations | The contradiction sweep is refused until something is actually cited |
| **FALSIFY** | Introduce the incident that destroys the belief, then try to retract it | `23514 · live_requires_promoted` — the retraction cannot leave a live intent behind |

Between the second and third beat the *same two statements* that were refused now commit, unchanged:
the intent `deploy etcd v3.5.28` goes live while the audit reads `live_on_nonpromoted = 0`. The safe
path out is explicit — cancel the intent, then retract — which commits, retracts both beliefs, and
leaves the audit at zero.

Retrieval distances and result values shown by the demo are measured from the live request; there is
no fallback that fabricates a result when a real one is unavailable.

## The measured retrieval finding

This is a measured limitation, not a caveat, and it is why the product is shaped this way.

Ask the corpus the question an engineer would actually ask before a deployment:

```
"Is etcd v3.5.x safe to deploy?"
  #19220  0.372424   go.etcd.io/etcd/client/v3 updates
  #12987  0.387913   3.4 to 3.5 upgrade may panic
  #12788  0.401092   Using client in v3.5.0 and etcd server with v3.4.14
```

Plausible, on-topic, and it does not contain the incident that matters. Ask about integrity instead:

```
"etcd v3.5 data inconsistency after upgrade"
  #14139  0.199509   inconsistent data in etcd 3.5.4
```

`0.199509` against `0.372424`. The embedding is behaving correctly — a deployment-phrased question is
simply nearer to release-process material than to defect reports. **An agent that asks "is this safe
to deploy?" and reads the top-k will not see the incident that matters most.**

The canonical incident is worse. `#13766 · Inconsistent revision and data occurs`, the issue that
actually destroys the claim, sits at distance `0.594920` from it — a measured **rank 573 of 7,239**.
No top-k, threshold or reranker puts that in front of a judge.

> That rank is an offline measurement, not something the application renders. On screen the issue is
> introduced by URL and the UI says so: *"Not retrieved — introduced by URL."*

The response is not to tune the retrieval. It is to stop treating retrieval as an authority:

> Retrieval proposes candidates. It decides nothing. The database decides.

A demo where the human reasons correctly and the database never has to intervene would demonstrate
nothing.

## Database enforcement

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

The third only works because of a property we measured rather than assumed: **CockroachDB
re-evaluates `CHECK` constraints on rows modified by `ON UPDATE CASCADE`.** Retracting a belief
cascades the new status into every citing intent, and the `CHECK` on those rows then refuses the
whole transaction. The application never checks this. It cannot forget to.

**On `belief_edge` and traversal.** CockroachDB does not traverse `belief_edge`. The recursive walk
is a `WITH RECURSIVE` CTE in application code. **Belief traversal is application logic; CockroachDB
enforces the invariant at the authority boundary.** That distinction matters if you are reading
`db/001_schema.sql` looking for the graph logic — it is not there, and it is not supposed to be.

## Control experiment

The same race, the same correct application logic, three schemas. Served at
[`/proof`](https://byb43s8nh2.us-west-2.awsapprunner.com/proof).

| Cell | Schema | Isolation | Observed | Where |
|---|---|---|---|---|
| 1 | naive | `READ COMMITTED` | no error, both transactions commit, **AUDIT = 1** | local, v26.2.0 |
| 2 | naive | `SERIALIZABLE` | `40001 RETRY_SERIALIZABLE` | local, v26.2.0 |
| 3 | hardened | `READ COMMITTED` | `23503 · gate` | deployed cluster, v26.2.5 |

Cells 1 and 2 run locally on purpose — the naive strawman must never touch the judge-facing cluster.

Two readings the transcripts insist on:

- **`40001` is a retry signal, not a refusal.** The harness deliberately does not retry, so the raw
  code is visible. Under `crdb.ExecuteTx` the transaction retries and *then* refuses on fresh state.
  The accurate chain is **detect → retry → refusal on fresh state**.
- **Cell 1 was observed, not predicted.** The anomaly was inherited from a PostgreSQL-era claim and
  had never been run against CockroachDB. It reproduced. The corruption came from the schema, not
  from a logic bug.

## Run and deploy

The live URL above is the demo. This is for running the kernel and demo tracks locally.

```bash
task setup          # pinned single-node CockroachDB in Docker + build
task test           # full verification suite
task web:run        # ledger on :8080/, wizard on :8080/demo
task corpus:verify  # counts corpus rows and embeddings, fails on mismatch
task deploy         # six-step fail-closed App Runner deploy
```

The local container (`solvent-crdb`, insecure, SQL on 26260) is a **development dependency, not the
demo**. The demo runs on CockroachDB Cloud Serverless — that is what the live URL talks to, and where
every deployed measurement here came from.

## Verification evidence

| Evidence | What it shows |
|---|---|
| `proof/isolation.log` | The three-cell control experiment, with raw SQLSTATEs |
| `proof/act6_tier_probe.log` | `CHECK` re-evaluation on cascade, verified on the deployed cluster |
| `docs/M2_TRANSCRIPT.md` | Kernel behaviour against a live cluster, per-case SQLSTATE and constraint |
| `docs/M1_I7.md` | Invariant I-7 — every write goes through `crdb.ExecuteTx` |
| `corpus-data/*.meta.json` | Fetch provenance and embedding checkpoint, digest-pinned |
| `scripts/deploy.sh` step 6 | Asserts `#19220 / 0.372424` against the deployed service on every deploy, tolerance `5e-7` |

Run `task test` for the current suite result rather than trusting a number written here.

The corpus is **7,239 etcd issues** (pull requests excluded), all 7,239 embedded with Titan v2 at
1024 dimensions — counted from `corpus_issue` on both the local and hosted clusters, and digest-pinned
in `corpus-data/`. It is the measured ingested set, never GitHub's live issue counter.

## Hackathon tool usage

| | Tool | Status |
|---|---|---|
| 1 | **Distributed Vector Indexing** | verified |
| 2 | **ccloud CLI** | verified |

- **Distributed Vector Indexing** — `corpus_issue.embedding` is a `VECTOR(1024)` column served by
  CockroachDB's native vector index, `corpus_issue_embedding_idx (scenario_id, embedding
  vector_cosine_ops)`, on a multi-region cluster. The scenario prefix isolates each session's
  retrieval. All 7,239 rows carry a genuine Titan v2 embedding, and `/demo` performs live cosine ANN
  retrieval (`<=>`) on every query; query plans are asserted to use the index with bounded prefix
  spans. *(We claim a CockroachDB-native vector index on a multi-region cluster — we have not
  measured index distribution or sharding, and do not claim it.)*
- **ccloud CLI** — used interactively for cluster introspection, and programmatically by
  `task deploy` as a fail-closed JSON preflight: `scripts/ccloud_preflight.sh` parses
  `ccloud cluster list --output json` and aborts the deploy if cluster identity, cloud provider,
  version, state, or primary region is not what the deployment expects.

> The composite foreign-key gate, the `CHECK` re-evaluation on cascade, and transactional retraction
> are **not** counted above. They are the CockroachDB memory-layer architecture this project is built
> on — not rubric tool claims. Stretching one mechanism to satisfy two claims would misrepresent both.

**Cloud Managed MCP Server:** configured, and its unauthenticated `401` observed, but end-to-end
verification is pending an owner-issued service-account key. It is **not** claimed as a verified tool.
`scripts/cloud_mcp_verify.sh` is the one command that would close it.

Separately, this repo ships its own stdio MCP server (`cmd/solvent-mcp`) exposing six domain tools —
a local subprocess, not a hosted service. See `AGENTS.md`.

## License

MIT.
