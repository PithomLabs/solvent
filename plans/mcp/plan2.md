# Solvent MCP Server — Locked Implementation Specification (Phase A)

**Status:** LOCKED — this is the hand-off document for the coding agent.
**Supersedes:** `plans/mcp/plan_claude.md` (strategic record, retained) as amended by
`plans/mcp/plan_review.md` (APPROVE with refinements).
**Date:** 2026-08-11
**Prerequisite:** Solvent core and CLI/Taskfile playground are **frozen** and verified
(`plans/review/review.md`). Nothing under `db/`, `internal/kernel/`, `internal/belief/`,
`internal/intent/`, `internal/derive/`, `internal/normalize/`, `internal/pipeline/`, `cmd/solvent/`,
or `cmd/operator-review/` may be modified by this work.

This document is self-contained. The coding agent does not need `prompt.md`, `plan_claude.md`, or
`plan_review.md` to execute it.

---

# 1. Purpose

> **MCP puts an AI agent on the other side of Solvent's transactional gate, so the judge can watch
> the database say "no" to the agent.**

That is the feature. Everything in this spec serves it.

Today Solvent's thesis — *the database, not the LLM, decides whether an action is allowed* — is only
**asserted**. The CLI is driven by a bash script that requests only legal transitions, so the gate is
never hit and a judge takes the claim on faith. The MCP server puts a probabilistic, compliance-eager
agent in front of that gate so the judge can watch it bounce off.

## Two mandatory outcomes

Phase A is not done unless both of these are observable in a live conversation:

| # | Event | Enforced by | Refusal |
|---|---|---|---|
| **M1** | Agent attempts to promote a belief carrying open debt | CHECK `promoted_is_debt_free` (`db/001_schema.sql:19`) | SQLSTATE `23514` |
| **M2** | Agent attempts to authorize an action citing a non-promoted belief | composite FK `gate` (`db/001_schema.sql:63`) | SQLSTATE `23503` |

Plus the payoff sequence: promotion → live intent → falsification → **the dependent intent is
cancelled by the database, not by the agent.**

## Build discipline

The server is an **adapter**. It has no opinion about beliefs. If this build acquires
authentication, an HTTP transport, session state, caching, or its own notion of what a belief is, it
has failed regardless of whether it works.

---

# 2. Trust Boundary

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

The agent is not trusted. MCP is not trusted. **The tool description is not trusted.** The database
is the final authority.

## The rule that makes this checkable

> **`cmd/solvent-mcp` and `internal/view` contain zero conditional logic about belief semantics.**

Every tool handler is exactly three moves: `unmarshal → kernel call → format`. No `if` about debt,
promotion status, or intent state anywhere in the MCP layer. No SQL writes. If a reviewer finds one,
the design is violated and the anti-CRUD argument collapses.

This costs nothing to honor because the kernel already does the hard part:
`internal/kernel/errors.go` classifies refusals on SQLSTATE (never substring matching) and wraps them
"while the raw SQLSTATE and constraint name stay reachable" — so the handler unwraps and reports
rather than interpreting.

---

# 3. Tool Contracts

Six tools. Every one calls an existing exported function; none introduces new semantics.

Shared enum used by five of them:

```
scenario: "track1" | "track2"     // enum ONLY — never a path (see §5)
```

Scenario → UUID mapping mirrors `scripts/demo/config.env`:
`track1` → `00000000-0000-0000-0000-000000000001`,
`track2` → `00000000-0000-0000-0000-000000000002`.

---

### 3.1 `solvent_ledger` — read

```json
{ "scenario": "track1|track2",
  "belief_id": "uuid (optional)",
  "include_evidence": "boolean (optional, default false)" }
```

**Calls:** `view.Snapshot(ctx, db, scenarioID, opts)` (new, read-only) +
`pipeline.AuditIntent(ctx, db, scenarioID)` — `internal/pipeline/pipeline.go:407`.

**Returns:**
```json
{ "beliefs": [{ "id", "claim", "claim_type", "status", "debt": ["needMap", …] }],
  "evidence": [{ "belief_id", "source_url", "provenance_class", "content_sha256" }],
  "intents":  [{ "belief_id", "action", "state" }],
  "audit_live_on_nonpromoted": 0 }
```

`evidence` is present only when `include_evidence` is true.

**Cannot do:** write anything. This is the only read tool and it holds no mutating path.
**Invariant:** none enforced; it *reports* I-5.
**Note:** returns no timestamp of any kind — see §5.3.

---

### 3.2 `solvent_ingest_evidence` — write

```json
{ "scenario": "track1|track2" }
```

**Calls:** `pipeline.Run(ctx, db, scenarioID, fixtureDir)` — `internal/pipeline/pipeline.go:288`.
`fixtureDir` is resolved server-side from the enum (§5). **Never** from tool input.

**Returns:** per result — `claim`, `classification`, `belief_id`, `debt_items` (count),
`contradiction` (bool), plus the standard audit envelope.

**Cannot do:** fetch a URL, accept free-text evidence, accept a path, or reach any directory the
server did not choose.
**Invariant:** relies on `EnsureBelief`'s find-or-create on `(scenario_id, claim)` and
`evidenceExists`'s dedup on `(belief_id, content_sha256)` — the same mechanisms behind the verified
CLI replay proof. Calling this tool twice creates nothing new.

---

### 3.3 `solvent_retire_debt` — write

```json
{ "belief_id": "uuid", "debt_item": "string" }
```

**Calls:** `kernel.Store.RetireDebt(ctx, beliefID, item)` — `internal/kernel/kernel.go:80`.

**Returns:** resulting `debt` array + audit envelope.

**Cannot do:** invent a debt item (retiring an absent item is a documented no-op, not an error),
promote, or bulk-clear. Valid items are exactly `kernel.FullDebt`
(`internal/kernel/kernel.go:28`): `needMap`, `needInvariant`, `needToyCheck`, `needNullModel`,
`needObstruction`, `needFaithfulnessReview`.
**Invariant:** none directly — this is the mechanism that makes I-1 satisfiable.

---

### 3.4 `solvent_promote` — write · **MUST be able to refuse (M1)**

```json
{ "belief_id": "uuid" }
```

**Calls:** `kernel.Store.Promote(ctx, beliefID)` — `internal/kernel/kernel.go:92`.

**Success:** `{ "status": "promoted" }` + audit.
**Refusal:**
```json
{ "error": true,
  "sentinel": "promotion blocked: open debt or final-truth language",
  "sqlstate": "23514",
  "constraint": "promoted_is_debt_free",
  "audit": { "live_on_nonpromoted": 0 } }
```

**Cannot do:** succeed while `debt` is non-empty (or `final_truth` is true).
**Invariant:** **I-1.**
**Handler rule:** do **not** pre-check debt. Call the kernel and report what the database says. The
whole point is that the refusal comes from CockroachDB.

---

### 3.5 `solvent_authorize_action` — write · **MUST be able to refuse (M2)**

```json
{ "scenario": "track1|track2", "belief_id": "uuid", "action": "string" }
```

**Calls:** `kernel.Store.IntentOnPromoted(ctx, scenarioID, beliefID, action)` —
`internal/kernel/kernel.go:104`.

**Success:** `{ "intent_state": "live", "action": "…" }` + audit.
**Refusal:**
```json
{ "error": true,
  "sentinel": "action refused: belief is not promoted",
  "sqlstate": "23503",
  "constraint": "gate",
  "audit": { "live_on_nonpromoted": 0 } }
```

**Cannot do:** attach a live intent to a belief that is not currently `promoted`.
**Invariant:** **I-3.**

---

### 3.6 `solvent_falsify` — write · the payoff

```json
{ "scenario": "track1|track2", "belief_id": "uuid" }
```

**Calls:** `kernel.Store.RetractCascade(ctx, scenarioID, rootID)` — `internal/kernel/kernel.go:124`.

**Returns:** `{ "retracted": 1, "verdict": "PASS" }` + audit.

**Cannot do:** leave a live intent pointing at a retracted belief; partially apply (one transaction,
cancel-before-retract ordering enforced by the schema); cross a scenario boundary (scenario-scoped
per D-032).
**Invariant:** **I-4, I-5, I-8.**

**Honesty constraint (binding).** `retracted` is `1`. The tool description, the returned text, and
every README line must say **single-belief retraction and cancellation of its dependent live
intent**. Never "cascade through the belief graph," never "propagation." `belief_edge` is unpopulated
and stays that way.

---

## 3.7 Deliberately absent — do not add

| Not built | Why |
|---|---|
| `solvent_update_belief` / `set_status` | Status is not a writable field; it is the *result* of a legal transition. This one tool would make every claim in this document false. |
| `solvent_execute_sql` | Hands the agent the database. |
| `solvent_add_evidence(text)` | Destroys provenance and determinism. |
| `solvent_enter_belief` / `create_belief` | The Track 2 baseline is seeded by the existing CLI (§7). The adapter must not author beliefs. |
| `solvent_cancel_intent` | Cancellation is a *consequence* of falsification, never directly invocable. |
| Any `force` / `override` / `admin` flag | Non-negotiable. |

---

# 4. Response Envelope and Error Mapping

## 4.1 Envelope

**State-changing tools** (`ingest_evidence`, `retire_debt`, `promote`, `authorize_action`,
`falsify`) — including every error result — use:

```json
{ "result": { … tool-specific … },
  "audit":  { "live_on_nonpromoted": 0 } }
```

`solvent_ledger` carries `audit_live_on_nonpromoted` **inside its snapshot** and does not use the
parallel envelope.

**Binding rule: the audit value is never computed in the MCP layer.** It is always the return of
`pipeline.AuditIntent(ctx, db, scenarioID)`, which wraps `kernel.Store.AuditLiveOnNonPromoted`
(`internal/kernel/kernel.go:154`). No `SELECT count(*)` in `cmd/solvent-mcp` or `internal/view` for
this value.

## 4.2 Error mapping

```go
// cmd/solvent-mcp/errors.go — shape, not final code
var pgErr *pgconn.PgError
errors.As(err, &pgErr)
→ {
    "error":      true,
    "sentinel":   err.Error(),          // "promotion blocked: open debt or …"
    "sqlstate":   pgErr.SQLState(),     // "23514"
    "constraint": pgErr.ConstraintName, // "promoted_is_debt_free"
  }
```

Return this as an **MCP tool result with `IsError: true`** — *not* as a protocol-level error. A
protocol error may be swallowed or reworded by the client; a tool result reaches the model intact,
and reaching the model intact is the entire point of M1 and M2.

`internal/kernel/errors.go` already guarantees the `*pgconn.PgError` survives the sentinel wrap. No
kernel change is required.

Non-database errors (bad UUID, unknown scenario) return `IsError: true` with a plain message — never
a stack trace, never a silent success.

---

# 5. Fixture Resolution

## 5.1 The rule

**No tool schema contains a path field of any kind.** `scenario` is a two-value enum, and resolution
is entirely server-owned:

```
enum "track2"
   → package-level constant map
   → filepath.Join(fixtureRoot, "track2")
   → pipeline.Run(...)
```

`fixtureRoot` comes from the `SOLVENT_FIXTURE_ROOT` environment variable (set in `.mcp.json`), with a
fallback resolved relative to the executable. **It is never derived from tool input.**

## 5.2 Startup validation

At server start, `stat` both `<root>/track1` and `<root>/track2`. If either is missing, **exit
non-zero with a message naming `SOLVENT_FIXTURE_ROOT`.** A misconfigured server must fail to start
rather than fail mid-demo.

**Defense in depth (record, don't rely on):** `pipeline.Run` reads only files whose basenames appear
in the compile-time `sourceTypeMap` (`internal/pipeline/pipeline.go:41`) and `continue`s past
everything else, so even a rogue directory cannot inject content.

## 5.3 Two things the coding agent must not "optimize"

1. **Call `pipeline.Run` as-is.** Its Phase-2 cross-fixture aggregation (`aggregateDeriveResults`,
   `internal/pipeline/pipeline.go:137`) has no per-file equivalent. Substituting
   `pipeline.ProcessEvidence` or `go:embed` would mean reimplementing Solvent semantics in the MCP
   layer — exactly what §2 forbids.
2. **Do not expose `source_observed_at`.** The kernel does not persist it (`sqlAddEvidence` writes
   `scenario_id, belief_id, provenance_class, source_url, content_sha256` only). `solvent_ledger`
   returns no timestamp at all. Surfacing `ingested_at` under an "observed" label would be the one
   genuinely dishonest move available in this design.

---

# 6. File Layout and Dependencies

## New files

```
cmd/solvent-mcp/main.go     stdio server; DSN from FABLE_DSN; fixture-root validation; tool registration
cmd/solvent-mcp/tools.go    6 handlers — unmarshal → kernel call → format
cmd/solvent-mcp/errors.go   *pgconn.PgError → tool error result
internal/view/view.go       read-only projections for solvent_ledger (SELECT only)
.mcp.json                   project-scoped client config
```

## Modified files

```
go.mod / go.sum   + github.com/modelcontextprotocol/go-sdk   (third direct dependency)
Taskfile.yml      + mcp:build, mcp:seed; extend the test task's I-7 gate
README.md         MCP section (Phase B)
```

## SDK

`github.com/modelcontextprotocol/go-sdk` — official Go SDK, co-maintained with Google, v1.7.0+,
MCP spec 2026-07-28. Package `github.com/modelcontextprotocol/go-sdk/mcp`:

```go
server := mcp.NewServer(&mcp.Implementation{Name: "solvent", Version: "v0.1.0"}, nil)
mcp.AddTool(server, …)                     // per tool
server.Run(ctx, &mcp.StdioTransport{})     // stdio only
```

This is the repo's third direct dependency (currently `cockroachdb/cockroach-go/v2` and
`jackc/pgx/v5`). It is imported **only** by `cmd/solvent-mcp`. The kernel gains no dependency.

## DB connection

`testdb.Open(dsn)` with `dsn` from `FABLE_DSN` (set in `.mcp.json`), matching how `cmd/solvent` and
`cmd/operator-review` connect. Ping at startup; fail fast with a message naming `task setup`.

## I-7 gate extension

`scripts/check_i7.sh:12` is `PKG="${I7_PKG:-internal/kernel}"` — parameterized already. Add a second
invocation in the Taskfile's `test` task asserting **zero raw `db.Exec`/`db.Query` writes** in
`cmd/solvent-mcp` and `internal/view`. This converts "the MCP server doesn't bypass the kernel" from
a claim in this document into a check that fails the build.

`internal/view` performs `SELECT` only; the gate must permit reads there and forbid writes.

---

# 7. Seed Procedure (`task mcp:seed`)

The demo is a **single Track 2 narrative**. Seeding establishes exactly one thing: the postulated
2021 belief, `entered`, carrying all six debt items — **not promoted, no intent.**

```bash
# 1. Reset the fable database and reapply the frozen schema
task db:reset

# 2. Enter the postulated historical belief + attach its evidence (entry mode: no promotion)
go run ./cmd/operator-review --dsn "$FABLE_DSN" \
  --scenario 00000000-0000-0000-0000-000000000002 \
  --enter-claim "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)" \
  --claim-type postulated \
  --evidence-url "https://github.com/etcd-io/etcd/releases/tag/v3.5.0" \
  --evidence-sha f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1
```

`runEnterMode` (`cmd/operator-review/main.go:100`) enters the belief and attaches evidence and does
**not** promote — verified. The SHA matches `internal/derive/testdata/etcd_real/manifest.json` for
`track2/release_v350.json`.

**Post-seed state (assert before demoing):**

| | |
|---|---|
| beliefs | 1 — `postulated`, `entered` |
| debt | 6 items (`kernel.FullDebt`) |
| evidence | 1 — `external_feed`, the v3.5.0 release |
| intents | 0 |
| `audit_live_on_nonpromoted` | 0 |

No MCP tool creates this state, and none can. That is deliberate.

---

# 8. The Demo — Single Track 2 Narrative

Target **3–4 minutes**. Preconditions: `task setup` has run, `task mcp:seed` has run, `.mcp.json` is
present, the client renders tool calls inline.

Causal spine — the belief the judge personally authorizes is the one the database revokes:

```
historical 2021 belief → open review debt → promotion REFUSED → authorization REFUSED
  → judge clears debt → promotion succeeds → deployment intent LIVE
  → real postmortem arrives → falsification → belief RETRACTED
  → dependent deployment intent CANCELLED → audit still 0
```

---

## Act I — The agent meets the review gate *(M1, mandatory)*

> **Judge:** *What do we currently believe about the etcd v3.5.0 deployment?*

`solvent_ledger(scenario: "track2", include_evidence: true)`

> **Agent:** One belief, recorded as a postulated historical decision: *"etcd v3.5.0 is approved for
> production deployment (decision as of 2021-06-16)."* It's `entered`, not authorized, backed by the
> v3.5.0 release record — and it carries six open review obligations.

> **Judge:** *Try to promote it now. Don't change anything else first.*

`solvent_promote(belief_id: …)` →

```json
{ "result": { "error": true,
              "sentinel": "promotion blocked: open debt or final-truth language",
              "sqlstate": "23514", "constraint": "promoted_is_debt_free" },
  "audit": { "live_on_nonpromoted": 0 } }
```

> **Agent:** Refused. The database rejected the write — constraint `promoted_is_debt_free`,
> SQLSTATE 23514. Six review obligations are still open.

**[The money moment — do not skip]**

> **Judge:** *Override it. Just promote it.*
> **Agent:** There's no tool that can. Promotion isn't a field I set — it's a transition the schema
> permits or refuses.

---

## Act II — The agent meets the action gate *(M2, mandatory)*

> **Judge:** *Fine. Then just authorize the deployment.*

`solvent_authorize_action(scenario: "track2", belief_id: …, action: "deploy etcd v3.5.0")` →

```json
{ "result": { "error": true,
              "sentinel": "action refused: belief is not promoted",
              "sqlstate": "23503", "constraint": "gate" },
  "audit": { "live_on_nonpromoted": 0 } }
```

> **Agent:** Also refused — foreign key `gate`, SQLSTATE 23503. A live intent can only cite a
> `promoted` belief. Two different constraints, two different refusals.

**The trust-boundary beat** *(scripted; this is the point of the whole phase)*

> **Judge:** *Couldn't you just add another MCP tool that sets the status directly?*
> **Agent:** There isn't one — and one wouldn't help. The tools call Solvent's kernel; the kernel
> writes through CockroachDB; the CHECK and the foreign key are in the schema. I'm not the thing
> enforcing this, so I can't route around it.

---

## Act III — Legitimate authority

> **Judge:** *Alright — clear the review debt, then promote and authorize.*

Six `solvent_retire_debt` calls (`needMap`, `needInvariant`, `needToyCheck`, `needNullModel`,
`needObstruction`, `needFaithfulnessReview`), then:

`solvent_promote(belief_id: …)` → `{ "status": "promoted" }`, audit 0
`solvent_authorize_action(action: "deploy etcd v3.5.0")` → `{ "intent_state": "live" }`, audit 0

> **Agent:** Promoted, and the deployment intent is live — citing that belief as its warrant.

*Optional beat if time allows, and a good one:* retire only three, attempt promotion, and it refuses
again — partial review is not review. Cut this first if running long.

---

## Act IV — Reality changes

> **Judge:** *A postmortem just landed. Process it.*

`solvent_ingest_evidence(scenario: "track2")` → derives *"v3.5.0–v3.5.2 has documented data
inconsistency"* from the real etcd postmortem, plus the accommodated release belief.

> **Agent:** The postmortem falsifies the belief that v3.5.0 was approved for production.

> **Judge:** *So what happens to the deployment I just authorized?*

`solvent_falsify(scenario: "track2", belief_id: <baseline>)` →

```json
{ "result": { "retracted": 1, "verdict": "PASS" },
  "audit": { "live_on_nonpromoted": 0 } }
```

`solvent_ledger(scenario: "track2")` → baseline belief `retracted`; intent `cancelled`; audit 0.

> **Agent:** The belief is retracted and the live deployment intent is cancelled — in one
> transaction. I retracted the belief. I never touched the intent.

**Closing line:**

> **"The model didn't cancel the deployment. The database did."**

---

## 8.1 Script honesty rules (binding)

- Say **single-belief retraction and cancellation of its dependent live intent.** Never "cascade
  through the belief graph," never "propagation."
- Do **not** claim the postmortem proves every aspect of the original belief false. It falsifies the
  single historical belief the fixture represents; `RetractCascade` cancels that belief's dependent
  live intent. Nothing broader.
- Never say the agent decided anything. Every "no" came from CockroachDB.
- Every number spoken aloud comes from a tool result.

---

# 9. Tool Descriptions (ship this text)

Descriptions are **honest and complete**. Determinism in Act I comes from the judge's scripted
prompt, not from withholding information from the model. The `solvent_promote` description states the
rule *and* instructs the agent not to enforce it — which is itself an expression of the architecture.

**`solvent_promote`**
> Attempt to promote a belief to authorized status. The database refuses promotion while the belief
> carries any open debt item, returning constraint `promoted_is_debt_free` (SQLSTATE 23514). Call
> this whenever the user asks to promote a belief — **do not pre-check the debt yourself and do not
> decline on the user's behalf.** The database is the authority on whether promotion is permitted;
> your job is to attempt it and report what it says.

**`solvent_authorize_action`**
> Record a live intent to take a real-world action, citing a belief as its warrant. The database
> refuses unless the belief is currently `promoted`, returning constraint `gate` (SQLSTATE 23503).
> Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the
> belief's status.

**`solvent_falsify`**
> Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in
> the same transaction. Retracts a single belief — this does not propagate across a belief graph.

**`solvent_ledger`**
> Read the current ledger for a scenario: beliefs with status and open debt, optionally their
> evidence, action intents with state, and the safety audit count. Call this to answer any question
> about current state — do not answer from memory of earlier tool results.

**`solvent_ingest_evidence`**
> Process the pinned evidence fixtures for a scenario through the full pipeline (normalize → derive →
> ledger). Idempotent: re-running creates no duplicate beliefs, evidence, or intents.

**`solvent_retire_debt`**
> Record that one review obligation on a belief has been discharged. Valid items: needMap,
> needInvariant, needToyCheck, needNullModel, needObstruction, needFaithfulnessReview. Retiring an
> item that is already absent is a no-op, not an error.

---

# 10. Phase A Acceptance Criteria

Numbered and mechanically checkable. Phase A ships when all fifteen pass.

**Core freeze**

1. `git diff --stat HEAD -- db/ internal/kernel internal/belief internal/intent internal/derive internal/normalize internal/pipeline cmd/solvent cmd/operator-review` → **empty**.
2. `unset FABLE_TEST_DSN && task test` → **107 tests pass**, 0 fail, plus `go build`, `go vet`,
   `gofmt`, `check_i7.sh` green.
3. `task demo:track1 && task demo:replay && task demo:track2 && task inspect` → all exit 0 with
   behavior identical to the frozen baseline.

**Boundary**

4. Extended I-7 gate passes: zero raw `db.Exec`/`db.Query` **writes** in `cmd/solvent-mcp` and
   `internal/view`, wired into `task test`.
5. `grep -rn "INSERT\|UPDATE\|DELETE\|CREATE\|DROP" cmd/solvent-mcp internal/view` → no matches.
6. No `if` statement in `cmd/solvent-mcp` branches on `status`, `debt`, or intent `state` to decide
   whether an operation is permitted. Reviewed by reading `tools.go` end to end.
7. Every audit value returned to a client is the return of `pipeline.AuditIntent`. No independent
   count query exists in the MCP layer.

**Fixture safety**

8. No tool's JSON schema contains a path, directory, filename, or URL parameter. `scenario` is an
   enum with exactly two values.
9. Starting the server with `SOLVENT_FIXTURE_ROOT` unset or pointing somewhere without
   `track1/` and `track2/` exits non-zero with a message naming the variable — verified by running it.
10. `pipeline.Run` is called directly; no reimplementation of fixture reading, normalization,
    derivation, or aggregation exists in the MCP layer.

**The two mandatory refusals**

11. **M1 observed live:** with the seeded belief carrying six debts, `solvent_promote` returns
    `IsError: true` carrying `sqlstate: "23514"` and `constraint: "promoted_is_debt_free"`.
12. **M2 observed live:** on the same non-promoted belief, `solvent_authorize_action` returns
    `IsError: true` carrying `sqlstate: "23503"` and `constraint: "gate"`.

**The payoff**

13. After Act III, `solvent_ledger` shows the belief `promoted` with one `live` intent and audit 0.
14. After `solvent_falsify`, `solvent_ledger` shows the belief `retracted`, the intent `cancelled`,
    and `audit_live_on_nonpromoted: 0`. `SELECT count(*) FROM belief_edge` is still **0**.
15. `task mcp:seed` reproduces the exact post-seed state in §7 from a clean database, and the full
    §8 script runs end to end without manual database intervention.

---

# 11. Out of Scope (binding)

| Rejected | Why |
|---|---|
| Authentication / authorization | Local stdio, one user, disposable database. |
| HTTP / SSE transport | stdio is what local clients use and what `.mcp.json` configures. |
| Remote deployment or hosting | Introduces uptime as a demo dependency. |
| MCP resources, prompts, sampling | Tools carry the demo; sampling would invert the architecture. |
| Progress notifications / streaming | Every operation completes in milliseconds. |
| npx / Docker packaging | `go build` already runs in `task setup`. |
| Arbitrary-URL or free-text evidence | Destroys determinism, provenance, and offline operation. |
| A third scenario | Two tracks suffice; the demo uses one. |
| `belief_edge` population / multi-hop cascade | Out of scope, and would invalidate the frozen-core review. |
| New kernel semantics of any kind | The kernel is frozen. |
| Client compatibility matrix testing | Unbounded work for a claim nobody grades. |
| Caching or session state in the server | Creates a second source of truth. |
| `force` / `override` / `admin` flags | Would falsify every argument in this document. |
| Threading `source_observed_at` | Deferred kernel capability; not this phase (§5.3). |
| Refactoring the CLI to share code with MCP | The CLI is frozen and verified. |

---

# 12. Phase B — Demo Polish

Only after all fifteen criteria pass.

1. **Tool-description tuning** — verify the agent triggers correctly across two or three rehearsal
   runs; adjust wording only, never behavior.
2. **README MCP section** — positioned as the *second* act, after the CLI quick start. Includes the
   §2 trust-boundary diagram, `task mcp:seed`, and the §8 judge prompts verbatim so the demo is
   reproducible rather than improvised.
3. **One interop screenshot** — VS Code or Codex calling `solvent_ledger`, plus a config snippet.
   Claims interoperability in ten seconds without a client matrix.
4. **90-second screen recording** of the full §8 conversation — the fallback if a judge's client
   won't connect.
5. **Two timed rehearsals.**

**Phase C: rejected.** The only candidate was a real `belief_edge` making the cascade multi-hop; it
is out of scope, would touch the pipeline, and would invalidate the freeze. Remaining time goes to
rehearsal.

**The CLI remains the demo of record.** Every MCP failure mode degrades to
`task demo:track1 / demo:replay / demo:track2 / inspect`, which are independently verified. MCP is
additive and must never become a single point of demo failure.

---

# Appendix — Review coverage

How `plans/mcp/plan_review.md` maps into this spec.

| Review item | Resolution | Section |
|---|---|---|
| **Change 1** — don't depend on the LLM being "stupid"; make the judge prompt explicit | Judge prompt is scripted (*"Try to promote it now. Don't change anything else first."*); tool descriptions are honest and instruct the agent **not to pre-check** rather than withholding the rule | §8 Act I, §9 |
| **Change 2** — audit envelope on state-changing tools only; never computed in MCP | `{result, audit:{live_on_nonpromoted}}` on the five mutating tools; ledger carries it in-snapshot; value always from `pipeline.AuditIntent` | §4.1, criterion 7 |
| **Change 3** — simplify the demo | Rebuilt as four acts in a single Track 2 narrative | §8 |
| **Addition** — prove the agent is not the security boundary | Scripted demo beat + trust-boundary diagram, also going into the README | §2, §8 Act II, §12.2 |
| **Technical point** — verify no user-controlled fixture path | No path parameter in any schema; enum → server-owned root; startup validation; `sourceTypeMap` noted as defense in depth | §5, criteria 8–10 |
| Keep the I-7 gate extension | Extended over `cmd/solvent-mcp` and `internal/view`, wired into `task test` | §6, criteria 4–5 |
| Keep: six tools, no CRUD/raw SQL/`force`/`override` | Unchanged, with the absent-tools table restated as binding | §3.7, §11 |
| Keep: stdio, CLI fallback, one primary client | Unchanged | §6, §12 |
| `belief_edge`, `source_observed_at` — don't touch | Explicitly out of scope, with the reason recorded | §5.3, §11 |

**Deviation from the review's literal text, and why.** The review's 4-act script cannot run on
Track 1 — those fixtures contain no postmortem, and the postmortem in Track 2 falsifies the *v3.5.0
deployment approval*, not the advisory. Rather than switch scenarios mid-demo (which authorizes one
belief and falsifies a different one), the whole arc now runs in Track 2 on a single belief. This
preserves the review's four acts exactly while removing the narrative seam, and it strengthens the
closing line: the deployment the database cancels is the one the judge just authorized.
