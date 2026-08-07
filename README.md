# README.md

Standing context for AI agents working in this repo. Read this before editing code.
This is orientation, not a task list — the milestone plan lives in `IMPLEMENTATION_CONTRACT.md`.

## What this is (primary)

This repository is a **transactional belief ledger for autonomous agents**. It is not a
chatbot, a RAG store, or a security tool. Its job is to hold *claims* as first-class
database rows that carry their evidence, their typed provenance, their unpaid epistemic
obligations ("debt"), their dependents, and their promotion state — and to make a claim
**actionable only when it has earned it**, with that gate enforced by the database schema
rather than by any prompt. When new evidence falsifies a claim, every decision downstream
of it is un-promoted in one atomic transaction. Concurrent agents revising the same belief
is a write-skew problem; serializable isolation is why the substrate is CockroachDB.

The mental model, in five lines (all load-bearing):
- **Ideas enter free.** A belief is written cheaply, fully in debt, `status='entered'`.
- **Promotion costs debt.** A belief becomes actionable only when its debt is empty.
- **New evidence creates new debt.** Falsification is debt reappearing, not deletion.
- **Debt does not kill.** Retracted beliefs are preserved — this is the audit story.
- **Knowledge oscillates, it does not accumulate.** Memory here is *subtractive*.

One sentence for humans: *beliefs enter free; actions cost promotion; the gate is in the tables.*

## Invariants — never violate these

- **I-1** No `promoted` belief has non-empty `debt`. (schema CHECK `promoted_is_debt_free`)
- **I-2** No `promoted` belief has `final_truth = true`. (same CHECK)
- **I-3** No `live` action_intent references a non-`promoted` belief. (composite FK + CHECK)
- **I-4** A belief with a live intent cannot become `retracted` until that intent is cancelled.
  (composite FK `ON UPDATE CASCADE` propagates status into the intent, then the CHECK fires)
- **I-5** `AuditLiveOnNonPromoted` returns 0 in every committed state.
- **I-6** Every invariant holds with all `embedding` columns NULL. Vectors are an optimization,
  never part of belief semantics.
- **I-7** Every kernel write goes through `crdb.ExecuteTx`. No raw `db.Exec`/`db.Query` writes.
- **I-8** `RetractCascade` is a single transaction; cancel-before-retract ordering is mandatory.

If a change would require weakening any of I-1..I-8, it is out of scope — stop and report.

## Enforcement locus — the thing agents get wrong

**I-1 through I-4 are enforced by the database, not by Go.** They live in CHECK constraints,
a composite `UNIQUE (id, status)` target, and a `FOREIGN KEY (belief_id, belief_status) …
ON UPDATE CASCADE`. Do **not** re-implement these as application-level guards in Go. Duplicated
guards drift from the schema, contradict it under concurrency, and give a false sense of safety.
The kernel's job is to *route writes through transactions* and *translate the database's refusal
into a typed Go error* — not to pre-check what the schema already refuses. The whole thesis is
that the gate is structural; keep it structural.

## Architecture at a glance

- **Schema (frozen):** four tables in `db/001_schema.sql` — `belief`, `belief_edge`,
  `evidence`, `action_intent`. Do not modify this file (see boundaries).
  - `claim_type` ∈ {`derived`,`accommodated`,`postulated`}; `status` ∈
    {`entered`,`promoted`,`retracted`} — there is no fourth status. `debt` is `TEXT[]`.
  - `evidence.provenance_class` ∈ {`external_feed`,`reproducible_artifact`,`live_scan`,
    `operator_asserted`}; `content_sha256` is required.
- **Kernel:** `internal/kernel` (package `kernel`). Public surface, behaviour only:
  `New`, `EnterBelief`, `AddEvidence`, `RetireDebt`, `Promote` (→ `ErrPromotionBlocked`),
  `IntentOnPromoted` (→ `ErrActionOnUnpromoted`), `RetractCascade`, `AuditLiveOnNonPromoted`,
  plus `FullDebt` and the `ClaimType` constants. This package must not import demo/domain code.
- **Substrate:** CockroachDB. `cockroach-go/v2` (`crdb.ExecuteTx`).
- **Agents (thin):** a *claim agent* (ingest evidence → enter/type claims → retire debt →
  merge duplicates by vector similarity before insert) and a *security agent* (read promoted
  beliefs → write intents). Keep them dumb; the memory layer is the intelligence.

## Go + CockroachDB conventions

- **All writes via `crdb.ExecuteTx(ctx, db, nil, func(tx) error { … })`.** It runs the
  40001 retry/savepoint protocol. A write outside it is a bug (I-7).
- **Classify errors by SQLSTATE, never by string.** Use `errors.As` to reach the pgx
  `SQLState()`; `23514` = CHECK violation (→ `ErrPromotionBlocked`), `23503` = FK violation
  (→ `ErrActionOnUnpromoted`), `40001` = serialization failure (let `ExecuteTx` retry).
- **Serializable is the default on CockroachDB, and that default is the point.** Do not reach
  for `SELECT … FOR UPDATE` as the primary safety mechanism — explicit locks are per-call-site
  vigilance that every future edit must remember; serializable requires remembering nothing.
  `READ COMMITTED` appears **only** in the isolation experiment, deliberately, to contrast.
- **Confidence is derived, never stored.** There is no confidence column and must never be one.
  If a UI needs a legibility signal, derive it (e.g. count of open debt); never persist a scalar.
- **The cascade is one transaction:** recursive walk of `belief_edge` from the root, cancel live
  intents on all descendants, then un-promote them — in that order, in a single `ExecuteTx`.
- **Vector column is nullable and used for merge-not-fork** (dedup on write), not retrieval.
  The ledger must be fully correct with it NULL (I-6).

## Hard boundaries (do not cross without escalation)

- **The schema is frozen.** Do not edit `db/001_schema.sql`. A needed schema change is a
  blocking finding to report, not an edit to make.
- **No new tables, agents, or evidence feeds.** The system is deliberately three-tables-two-
  agents-one-feed-one-graph. Additions belong in a post-hackathon backlog.
- **Kernel stays domain-agnostic.** No feed name (KEV, GitHub, etcd, …) appears in
  `internal/kernel`. Domain lives in the ingestor and the demo only.
- **No fabricated evidence — ever.** No invented IDs, advisories, or URLs. `external_feed`
  rows must have a resolving `source_url` and a matching `content_sha256`; CI enforces this.
  In a system whose entire pitch is provenance, fabricated evidence is self-refuting.
- **No internal vocabulary on user-facing surfaces.** Words like `accommodated`/`postulated`/
  `derived` are schema values, not narration; they stay out of the demo and any UI copy.

## The demo (secondary)

The demo is **Layer 4 and swappable** — it exists to make the kernel legible, and the kernel
must never depend on it. The current domain candidate is a live software-project belief stream
(GitHub-issue timelines; etcd is the leading choice for its dated, public "declared stable →
publicly retracted" oscillation). Whatever the domain, the demo must prove exactly three things,
all backed by real receipts: the **gate** (an action on an unpromoted belief is refused by the
FK, raw error shown), the **cascade** (new evidence falsifies a belief and downstream intents
cancel atomically), and the **three-cell isolation experiment** (naive schema @ READ COMMITTED →
silent corruption; @ SERIALIZABLE → 40001 refusal; hardened schema @ READ COMMITTED → FK refusal).
Swapping the domain changes the ingestor and the narrative; it changes nothing above Layer 4.

## Before you commit

- `go build ./...` and `go vet ./...` clean; no write path outside `crdb.ExecuteTx`.
- Kernel invariant tests green — and green a second time with all embeddings NULL (I-6).
- The pure-SQL proof still passes: `proof/02_lifecycle_and_invariants.sql` and the isolation
  harness in `proof/`. These are the source of truth the Go must reproduce, not diverge from.

## Escalation

If the schema cannot support a required behaviour, if an invariant would have to be relaxed,
or if the composite-FK + `ON UPDATE CASCADE` + CHECK interaction behaves differently on the
target CockroachDB version than the invariants state (verify this early — it was proven on
PostgreSQL), then **stop and report** with the exact file and observed-vs-expected behaviour.
Reporting a blocker is the correct outcome. Silently redesigning around it is the only wrong one.