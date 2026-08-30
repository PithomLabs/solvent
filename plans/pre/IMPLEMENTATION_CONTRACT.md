# IMPLEMENTATION_CONTRACT.md

**Status:** FROZEN. This is a contract between architect and implementer, not a design doc.
**Inputs the implementer is given:** this file + `db/001_schema.sql` (the approved DDL).
**Authority:** Where this file and any other document disagree, this file wins. Where this
file and `db/001_schema.sql` disagree, STOP and report (see §9). Do not redesign anything.

---

## 1. Project Objective

Build a transactional belief ledger for autonomous agents on CockroachDB. Claims are
first-class database rows carrying evidence, typed provenance, unpaid epistemic obligations
("debt"), dependents, and promotion state. A claim becomes actionable only when its debt is
retired — a gate enforced by schema constraints and a composite foreign key, never by prompt.
When new evidence falsifies a claim, every dependent decision is un-promoted in one atomic
transaction. The MVP demonstrates this end-to-end over one real evidence feed, with two thin
agents and one dependency-graph view, and proves that serializable isolation is what prevents
concurrent agents from acting on a belief that has already been retracted.

---

## 2. Frozen Architecture

- **Three tables (+ one edge table, already approved):** `belief`, `belief_edge`, `evidence`,
  `action_intent`. No additional tables in the MVP. (The edge table was authorized by the
  gatekeeper as a correctness requirement, not a scope expansion.)
- **Two agents:** (a) **Claim Agent** — ingests evidence, extracts/types claims, retires debt,
  merges duplicates via vector similarity before insert; (b) **Security Agent** — reads promoted
  beliefs and writes action intents. Nothing else is an agent. The evidence ingestor is a
  deterministic pipe, folded into the Claim Agent, and does not count as a third agent.
- **One evidence feed:** a single external feed supplies real, timestamped, publicly verifiable
  evidence. The kernel is feed-agnostic; the feed name appears only in the ingestor and demo,
  never in the kernel package.
- **One graph:** a single dependency-graph view (Evidence → Belief → Action), nodes colored by
  status, with live transitions during the cascade. No second visualization.

---

## 3. Frozen Schema

The schema is `db/001_schema.sql` as approved. Do not modify it. It defines exactly four
tables and the following load-bearing constraints, which the kernel relies on and must not
duplicate in application code:

- `belief.claim_type` ∈ {`derived`,`accommodated`,`postulated`}; `belief.status` ∈
  {`entered`,`promoted`,`retracted`}; `belief.debt` is `TEXT[]`.
- `belief` CHECK `promoted_is_debt_free`: a row with `status='promoted'` must have empty `debt`
  and `final_truth=false`.
- `belief` UNIQUE `(id, status)` — the composite FK target.
- `action_intent` FK `(belief_id, belief_status) REFERENCES belief(id, status) ON UPDATE CASCADE`,
  plus CHECK `live_requires_promoted`: a row with `state='live'` must have `belief_status='promoted'`.
- `evidence.provenance_class` ∈ {`external_feed`,`reproducible_artifact`,`live_scan`,
  `operator_asserted`}; `content_sha256` NOT NULL.
- `belief.embedding` is nullable; the ledger is fully functional with every embedding NULL.

If any statement below cannot be implemented against this schema without changing it, STOP (§9).

---

## 4. Kernel API to Implement

Package `internal/kernel`. Signatures and behavioral contracts only. Every write path MUST run
inside `crdb.ExecuteTx` (serialization-failure retry). Error classification MUST use pgx
SQLSTATE codes, never substring matching on error text.

- `New(db *sql.DB) *Store`
  Wraps an open pool pointed at a database with `db/001_schema.sql` applied.

- `EnterBelief(ctx, scenarioID, claim string, ct ClaimType) (id string, err error)`
  Inserts a belief at the door: `status='entered'`, `debt` = full starting debt, unpromoted.
  Never gated. Returns the new id.

- `AddEvidence(ctx, scenarioID, beliefID, provenanceClass, sourceURL, contentSHA256 string) err`
  Inserts one evidence row for a belief. `contentSHA256` is required. Does not change belief state.

- `RetireDebt(ctx, beliefID, item string) err`
  Removes one debt item from `belief.debt`. Idempotent if the item is absent.

- `Promote(ctx, beliefID string) err`
  Sets `status='promoted'`. If the schema CHECK rejects it (open debt or final-truth language),
  return `ErrPromotionBlocked`. The gate is the database's, not the function's.

- `IntentOnPromoted(ctx, scenarioID, beliefID, action string) err`
  Inserts a live `action_intent`. If the composite FK rejects it (belief not currently promoted),
  return `ErrActionOnUnpromoted`.

- `RetractCascade(ctx, scenarioID, rootID string) (retracted int, err error)`
  In ONE transaction: (1) compute the transitive descendant set of `rootID` over `belief_edge`;
  (2) set all `live` intents on those beliefs to `cancelled`; (3) set those beliefs' `status` from
  `promoted` to `retracted`. Step 2 MUST precede step 3. Returns the number of beliefs retracted.

- `AuditLiveOnNonPromoted(ctx, scenarioID string) (int, error)`
  Returns the count of live intents whose belief is not promoted. MUST always return 0 in
  committed state (this is Invariant I-5 expressed as a query).

Exported sentinels required: `ErrPromotionBlocked`, `ErrActionOnUnpromoted`.
Exported value required: full starting debt list (the six debt items).

---

## 5. Invariants That Must Never Be Violated

- **I-1** No belief with `status='promoted'` has non-empty `debt`. (Schema CHECK.)
- **I-2** No belief with `status='promoted'` has `final_truth=true`. (Schema CHECK.)
- **I-3** No `live` action_intent references a belief that is not currently `promoted`.
  (Composite FK + `live_requires_promoted` CHECK.)
- **I-4** A belief carrying a live intent cannot transition to `retracted` without that intent
  first being cancelled. (Enforced by ON UPDATE CASCADE propagating status into the intent row,
  which then violates `live_requires_promoted` unless already cancelled.)
- **I-5** `AuditLiveOnNonPromoted` returns 0 in every committed state.
- **I-6** All invariant tests pass with every `embedding` column NULL. Vectors are an
  optimization, never part of belief semantics.
- **I-7** Every kernel write goes through `crdb.ExecuteTx`. No raw `db.Exec`/`db.Query` writes.
- **I-8** `RetractCascade` is a single transaction. Cancel-before-retract ordering is mandatory.

Any change that would require weakening or removing any of I-1..I-8 is out of scope: STOP (§9).

---

## 6. Ordered Implementation Plan (smallest compilable milestones)

- **M0 — Cluster + schema up.** Apply `db/001_schema.sql` to a CockroachDB cluster. Confirm
  cluster version, that `READ COMMITTED` is available, and that the composite FK + CHECK accept
  a promoted row and reject a debted one. No Go yet beyond a connection smoke test.
- **M1 — Kernel package compiles.** Implement all of §4 against the schema. `go build ./...` and
  `go vet ./...` clean. No agents, no UI.
- **M2 — Invariant test suite green.** Table-driven tests exercising I-1 through I-8 against a
  live cluster, including the single-transaction cascade and the NULL-embedding pass (I-6).
- **M3 — Isolation experiment.** A harness running the cascade-vs-new-intent race in three
  configurations: (1) naive schema @ READ COMMITTED → silent corruption committed
  (audit ≠ 0); (2) naive schema @ SERIALIZABLE → one txn refused with 40001, audit = 0;
  (3) hardened schema (the real `action_intent`) @ READ COMMITTED → FK refusal (23503), audit = 0.
  "Naive" means a comparison intent table without the composite FK; both transactions run correct
  application logic. This milestone is committed to CI.
- **M4 — Evidence ingestor + Claim Agent.** Pull the one real feed, hash and store snapshots,
  enter/type claims, retire debt, and merge duplicate claims via vector similarity before insert.
- **M5 — Security Agent + gate demo path.** Agent reads promoted beliefs and writes intents;
  provide the button-pushable path that attempts an action on an unpromoted belief and surfaces
  the raw constraint error.
- **M6 — Dependency-graph view.** One graph, status colors, live cascade transitions.
- **M7 — Demo acceptance green.** The mechanical acceptance checks (see §7 M7) pass against a
  fresh scenario.

Milestones are strictly ordered. Do not start Mn+1 before Mn meets its acceptance criteria.

---

## 7. Acceptance Criteria Per Milestone

- **M0:** Schema applies with zero errors; a promoted-with-debt UPDATE fails with SQLSTATE 23514;
  a promoted-debt-free UPDATE succeeds; `SHOW`/settings confirm READ COMMITTED availability.
- **M1:** `go build ./...` and `go vet ./...` exit 0. Every §4 function exists with the stated
  signature. Grep confirms zero raw write calls outside `crdb.ExecuteTx` (I-7).
- **M2:** All invariant tests pass. Specifically: promote-with-debt → `ErrPromotionBlocked`;
  intent-on-unpromoted → `ErrActionOnUnpromoted`; retract-with-live-intent (skipping cancel) is
  refused; full cascade cancels intents and retracts descendants; `AuditLiveOnNonPromoted` = 0;
  the entire suite passes a second time with all embeddings NULL.
- **M3:** Three cells produce, verbatim: cell 1 `audit ≠ 0`; cell 2 `ERROR 40001` and `audit = 0`;
  cell 3 `ERROR 23503` and `audit = 0`. Harness runs in CI and is deterministic.
- **M4:** Every `external_feed` evidence row has a resolving `source_url` and a matching
  `content_sha256`; a CI check walks all such rows and passes. A duplicate claim is merged, not
  forked (verified by count). No fabricated CVE IDs, advisories, or URLs.
- **M5:** On camera / on demand, an action attempt against an unpromoted belief returns the raw
  FK error (23503) with no LLM narration substituted for it.
- **M6:** Ingesting the falsifying evidence drives a visible cascade: dependent nodes change
  status and at least one live intent becomes cancelled, in the single-transaction path.
- **M7:** `demo_accept.sh` (mechanical subset) is green: run reaches the close; three distinct
  correct receipts; no internal vocabulary (`accommodated`/`postulated`/`derived`) in narrated
  output; every cell carries a receipt; cold-open number-free. Timing gates are human-verified at
  recording, not asserted by CI.

---

## 8. Explicit Non-Goals (deferred until after the hackathon)

Do not build, and do not spend time on: any table beyond the four; a third agent; a second
evidence feed; the full NVD embedding corpus (a small subset only); a live AWS estate or scanner
integration; SBOM/artifact ingestion beyond the single feed; OpenVEX export; multi-region
deployment (screenshots only, no build-out); stored confidence scores (confidence is derived
display, never a column); a fourth belief status (`promotable` is derived, not stored); any Lean
formalization; integration into the bchat host (the kernel stands alone; bchat integration was
rejected); agent sophistication (agents stay thin); authentication/RBAC beyond what the demo
session needs; a second visualization; a settings/config UI. Anything not required by §6 is out
of scope for the MVP.

---

## 9. Escalation Policy

If implementation reveals any of the following, STOP and report to the architect in a short note
naming the milestone, the exact statement or file, and the observed vs. expected behavior — do
NOT redesign, do NOT weaken an invariant, do NOT add a table/agent/feed to work around it:

- `db/001_schema.sql` cannot support a §4 behavior or a §5 invariant as written.
- Any invariant I-1..I-8 would have to be relaxed to make a milestone pass.
- The composite FK + `ON UPDATE CASCADE` + CHECK interaction behaves differently on the target
  CockroachDB version than §5 states (this is the one interaction most likely to differ from the
  Postgres proof; verify at M0 and report before proceeding).
- A milestone's acceptance criterion cannot be met without violating a non-goal in §8.
- Any ambiguity in this contract that has more than one reasonable implementation with different
  observable behavior.

Reporting the blocker is the correct outcome, exactly as a legitimate blocking finding — not a
failure. Silence-and-workaround is the only prohibited response.
