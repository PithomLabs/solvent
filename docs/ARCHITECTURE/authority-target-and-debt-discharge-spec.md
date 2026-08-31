# Authority Target and Debt Discharge Specification

## Status

**PROPOSED — NOT IMPLEMENTED**

*Revision 1 — 2026-08-31: incorporates Review #1 (`plan3_imp_review.md:1`, verdict APPROVE WITH CONDITIONS) — Evidence Artifact vs Citation (`OPEN #13`), I-12 rename → `AuthorizationTargetMatch` with reserved I-13, link/target creation authority (`OPEN #8`), debt-representation convergence (`OPEN #14` preferred rows), N4 placement (`OPEN #16`), and hostile 1→3 anti-broadening example. Next gate is **Adversarial Review #2** attacking remaining OPEN items (link-creation authority, artifact/citation cardinality, N2 consistency, N4 placement, authorization- vs execution-verification). Still PROPOSED — no schema/Go/Lean/MCP change.*

This document is a design specification, not a description of shipped behaviour. It
proposes semantics for `AuthorityTarget`, `DebtDischarge`, and executor binding that
do **not** exist in the current codebase. Nothing in this document has been
implemented. No table, column, constraint, kernel function, MCP tool, Lean theorem,
or migration described here exists.

- Every claim about what exists **today** is labelled CURRENT and cites repository
  truth (`file:line`). Where a cited document is itself unimplemented future design
  (e.g. `docs/ADR/0001` decisions), the citation is to the *decision*, not to behaviour.
- Every claim about what is proposed is labelled PROPOSED.
- Every detail that survives only on reasoning, with no repository or use-case
  evidence, is labelled **OPEN**.

This specification is itself treated as a belief under the project's own methodology:
it carries a Self-Attack Results section and a Dogfood Evidence Ledger
(`## Dogfood Evidence Ledger`) in which nothing is marked `Discharged` without
evidence. It must survive its own adversarial review before any kernel or schema
change begins (`## Implementation Gate`).

## Purpose

The adversarial review of the production MVP plan
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md`, verdict APPROVE WITH CONDITIONS)
identified three tightly coupled findings that must be resolved **semantically**
before the schema or kernel may change:

- **F-01 (CRITICAL)** — Consequence binding gap: Solvent can authorize action A while
  the executor performs action B (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:36`).
- **F-04 (HIGH)** — Debt-free is necessary, not sufficient: `debt = []` is treated as
  proof of adequate justification, which reduces discharge to checkbox theater
  (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:66`).
- **F-06 (HIGH)** — Belief as an authority object is underspecified: the cardinality
  between beliefs and the consequences they justify is undefined
  (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:86`).

The purpose of this document is to turn the conceptual evolution

```text
CURRENT:  Evidence → Belief → Debt → Promotion → ActionIntent
FUTURE:   Evidence → Belief → Obligations → Verified Discharge → Promotion
              → Principal + Resource + Scope + Action + Consequence
              → Authority / Warrant → Action → Reassessment
```

into a precise domain model: definitions exact enough that an engineer could
implement them without inventing missing rules, adversarial enough that its own
weaknesses are recorded rather than hidden, and disciplined enough that anything
unsupported is marked OPEN instead of promoted by rhetoric.

This document proposes **domain semantics first**. It deliberately does not rush into
schema design; where a schema mechanism is sketched (e.g. the justification link in
`## Cardinality`), it is PROPOSED and exists only to prove the semantics are
expressible in the database's enforcement vocabulary. The smallest enforceable schema
is a later, separately reviewed artifact.

Three architectural decisions are treated in this document as **proposed architecture
under adversarial challenge**, not as settled fact:

- **Decision A — Belief is never authority.** A promoted belief is *eligible to
  justify* an authority target; promotion does not itself grant any permission.
- **Decision B — AuthorityTarget is a separate consequential object.** Authority is an
  explicit relationship between a target
  (`principal + resource + scope + action + consequence`) and one or more promoted
  justifications; the Belief↔Target mapping is many-to-many through explicit
  justification links; MVP semantics for multiple required beliefs is AND.
- **Decision C — Debt-free is necessary, not sufficient.** `Obligation ≠ Discharge`;
  a future `DebtDischarge` carries attribution, evidence/attestation, integrity, and
  provenance; one discharge corresponds to one obligation.

## Current Model

Everything in this section is CURRENT and verified against the repository.

### The four frozen ledger tables

`db/001_schema.sql` defines the entire authority-bearing surface:

- **`belief`** (`db/001_schema.sql:6-35`): `id, scenario_id, claim TEXT, claim_type
  ('derived'|'accommodated'|'postulated'), status ('entered'|'promoted'|'retracted'),
  debt TEXT[], final_truth BOOLEAN`. Every belief enters with the full six-item debt
  (`db/001_schema.sql:25-27`), mirrored byte-for-byte in `kernel.FullDebt`
  (`kernel/kernel.go:31-34`). The `promoted_is_debt_free` CHECK
  (`db/001_schema.sql:30-32`) refuses `status='promoted'` unless the debt array is
  empty and `final_truth` is false (SQLSTATE `23514`). `belief_id_status_key`
  (`db/001_schema.sql:34`) is the UNIQUE `(id, status)` target of the action gate.
- **`belief_edge`** (`db/001_schema.sql:37-45`): derivation/contradiction graph
  (`kind ∈ {'derives','contradicts'}`). The database never traverses this graph;
  recursive traversal is application logic (`WITH RECURSIVE` in Go,
  `kernel/kernel.go:127` → `kernel/sql.go`), and the schema's role is to enforce
  that the traversal cannot finish having left a live intent behind.
- **`evidence`** (`db/001_schema.sql:47-58`): `belief_id, provenance_class
  ('external_feed'|'reproducible_artifact'|'live_scan'|'operator_asserted'),
  source_url, snapshot JSONB, content_sha256, source_observed_at, ingested_at`.
  `content_sha256` is NOT NULL but **caller-supplied** — nothing at the kernel or
  handler verifies it against the payload (`kernel/kernel.go:73-79` passes it through).
- **`action_intent`** (`db/001_schema.sql:60-77`): `belief_id, belief_status
  (default 'promoted'), action TEXT, state ('live'|'cancelled'|'executed')`. This is
  the entire authorization vocabulary: **there is no principal, no resource, no
  scope, and no consequence column anywhere in the schema.** The `gate` composite
  FOREIGN KEY `(belief_id, belief_status) REFERENCES belief(id, status) ON UPDATE
  CASCADE` (`db/001_schema.sql:74-75`) refuses an intent on a non-promoted belief
  (SQLSTATE `23503`); `live_requires_promoted` (`db/001_schema.sql:68-69`) detonates
  if a live intent survives a status change (SQLSTATE `23514`).

The corpus/wizard layer (`corpus_issue`, `belief_corpus_citation` in
`db/002_corpus.sql`; `relation` column and `refusal_log` in `db/003_wizard.sql:46-61`)
carries retrieval provenance and refusal audit but no authority semantics. The
architecture is frozen at these tables (`AGENTS.md:52-54`); no further tables without
explicit approval.

### The kernel

Eight functions (`kernel/contract.go:16-25`), all writes through `crdb.ExecuteTx`:

- `EnterBelief` / `EnsureBelief` — insert/find-or-create a belief with full debt.
- `AddEvidence` — append one evidence row; belief state unchanged.
- `RetireDebt(beliefID, item)` — `array_remove`, idempotent, no return value, no
  evidence linkage, no actor attribution (`kernel/kernel.go:81-88`).
- `Promote(beliefID)` — plain `UPDATE ... SET status='promoted'`; the function
  inspects nothing; the schema CHECK is the gate (`kernel/kernel.go:90-100`).
- `IntentOnPromoted(scenarioID, beliefID, action string)` — `action` is an opaque
  string (`kernel/kernel.go:107`).
- `RetractCascade` — cancel live intents first, then retract, one transaction,
  scenario-scoped (`kernel/kernel.go:114-150`).
- `AuditLiveOnNonPromoted` — the I-5 read path; must be 0 in every committed state.

Errors are classified by SQLSTATE only (`kernel/errors.go:22-25`), wrapped so the
driver error stays reachable (`kernel/errors.go:51-56`); `40001` is a retry signal
that reaches `crdb.ExecuteTx` unmasked, never a refusal.

### How debt is discharged today

The full current discharge path, with its actual enforcement:

1. Evidence fixtures are normalized and derived (`internal/pipeline/pipeline.go`),
   with a compile-time filename→source-type map (`internal/pipeline/pipeline.go:41-57`)
   — ingestion is fixture-only; there is no hardened external gateway.
2. `belief.Process` attaches evidence, then retires debt per a **regex table**
   (`internal/belief/belief.go:77-85` → `internal/belief/mapping.go:15-34`).
   A `release`-type evidence item whose text matches `(?i)release` retires **both**
   `needProvenanceCheck` and `needContradictionSweep` at once
   (`internal/belief/mapping.go:20`). `needOperatorSignoff` is retired by a
   `maintainer_comment` matching `(?i)\b(security review|reviewed by)\b`
   (`internal/belief/mapping.go:26`).
3. `belief.Process` then calls `Promote` and **swallows** `ErrPromotionBlocked` as a
   non-error (`internal/belief/belief.go:87-96`).
4. Via MCP, the agent itself calls `solvent_retire_debt(scenario, belief_id,
   debt_item)` (`cmd/solvent-mcp/tools.go:104`). The handler's only guard is an enum
   check — the item must be one of the six known strings
   (`cmd/solvent-mcp/tools.go:130`) — plus a cross-scenario ownership check. Any
   caller may retire any item; there is no attestation, no principal, no channel
   independence (`docs/ADR/0001:136-148` names this the weak path).
5. Contradictions detected by the pipeline are logged and **discarded** — no ledger
   mutation (`internal/pipeline/pipeline.go:89-101`, `internal/belief/belief.go:38-48`).

`promoted_is_debt_free` checks **cardinality only** — `array_length(debt,1) = 0` —
never element values or how they were removed (`db/004_debt_vocabulary.sql:18-20`).

### The MCP surface and projections

Seven tools over stdio (`cmd/solvent-mcp/main.go:88-247`): `solvent_ledger,
solvent_ingest_evidence, solvent_retire_debt, solvent_promote,
solvent_authorize_action, solvent_falsify, solvent_explain` — scenario-scoped to
fixed `track1`/`track2` UUIDs (`cmd/solvent-mcp/main.go:31-33`). (`docs/ADR/0001`
predates `solvent_explain` and says "six"; the code and `docs/FOUR_VERB.md:25-31`
say seven.) `internal/view` is SELECT-only (`internal/view/view.go:1-5`);
`solvent_explain` mirrors the CHECKs as `predicted_*` SQLSTATEs and never invents
engine output (`internal/view/explain.go:1-12,109-163`).

### The formal model

`formal/lean/` proves abstract state-machine preservation: `ValidLedger` is the
conjunction of the three invariants; the transition theorems (`promote_preserves_validity`,
`retractCascade_preserves_validity`, `live_intent_implies_promoted`,
`cascade_retraction_cannot_leave_live_intent`, and siblings in
`formal/lean/Solvent/Preservation.lean`) hold with zero `sorry`. The abstract
`ActionIntent` carries only `state, beliefId, beliefStatus`
(`formal/lean/Solvent/Types.lean:47-51`) — the Lean model has no principal, resource,
scope, or consequence either. It is explicitly not a refinement proof of the
Go/CockroachDB implementation (`formal/lean/README.md`).

### What does not exist today

No `AuthorityTarget`. No `DebtDischarge` or discharge record of any kind — the debt
array's emptiness is unexplained by any row anywhere. No attestation object, table,
signature, or key (`docs/ADR/0001:230`). No warrant — authority **is** the
relationship between a live intent and a currently-promoted belief
(`docs/ADR/0001:240`). No principal or risk-tier columns
(`docs/POST_HACKATHON_ARCHITECTURE.md:399-419` proposes them as future). No
contradiction relation. No temporal validity. No executor contract, echo, or
post-execution proof.

## Problem

### F-01 — Consequence binding gap

CURRENT enforcement of "what may happen" is the string `action_intent.action`
(`db/001_schema.sql:65`) plus the `(belief_id, belief_status)` gate
(`db/001_schema.sql:74-75`). Nothing binds an executor's *actual* resource or
consequence to what was authorized. `IntentOnPromoted` accepts any opaque string
(`kernel/kernel.go:107`); the MCP handler forwards it; the ledger records it. The
attack from the review: an `ALLOW` for `export report.pdf` is used to export a
different tenant's document or `/etc/passwd`, because `resource` was never part of
the gate and no post-execution proof is required
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:42`). "Solvent authorizes A, executor
performs B" is not closed by anything in the current repository.

### F-04 — Debt-free is necessary, not sufficient

CURRENT promotion succeeds exactly when the debt array is empty
(`db/001_schema.sql:30-32`). But the path that empties the array is the weakest link
in the system:

- Any caller may retire any known item (`cmd/solvent-mcp/tools.go:104-130`) with no
  attestation and no attribution.
- One evidence row retires multiple obligations in a single step
  (`internal/belief/mapping.go:20`), which violates the spirit of the existing
  one-citation-per-retrieval-debt rule (`AGENTS.md:74`).
- `needOperatorSignoff` — the item whose *name* says a human decided — is retired by
  a regex over comment text (`internal/belief/mapping.go:26`).
- `belief.Process` auto-promotes and treats a blocked promotion as a non-event
  (`internal/belief/belief.go:87-96`).

The review's attack stands unmodified: an agent splits a human-review obligation into
trivial sub-items, discharges each with synthetic `operator_asserted` rows, promotes
in minutes, and the audit shows "all obligations discharged"
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:72`). `debt = []` is a **state
representation**, and the current system treats it as the whole proof.

### F-06 — Belief as authority object underspecified

A promoted belief can carry any number of live intents; nothing in the schema or
kernel constrains *which* consequences that belief may justify. `belief.claim` is
opaque text (`db/001_schema.sql:9`), evidence attaches to one belief
(`db/001_schema.sql:50`), but nothing stops staging-canary evidence from justifying a
production deployment intent for a different service
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:92`). The cardinalities
1→1, 1→N, N→1, N→N between beliefs and consequences are all currently *possible and
undistinguished*. If belief is too coarse, the gate authorizes too much; too fine,
and debt becomes combinatorial (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:91`).

### Coupled neighbors (referenced, not resolved here)

- **F-02** evidence provenance: `content_sha256` is caller-supplied and ingestion is
  fixture-only (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:46`). Discharge
  semantics depend on evidence trust; this spec defines what discharge must *claim*,
  not how evidence ingress is hardened.
- **F-05** attestation lifecycle: keys, revocation, replay are bounded to the
  attestation boundary by `docs/ADR/0001:187-211` but unresolved.
- **F-11** migration: addressed in `## Migration`.

## Design Principles

1. **Belief is epistemic, not authority.** A belief answers "what do we currently
   hold to be sufficiently supported?" It never answers "who may do what?".
2. **Debt represents unresolved obligation, not truth itself.** An empty debt array
   is a state, not a proof; the proof, when it exists, lives in discharge records
   with attribution and integrity.
3. **Promotion grants eligibility only, and only within a defined target.** A
   promoted belief may participate in explicit AuthorityTargets; it grants nothing
   by itself. Authority exists only as the target+justification relationship.
4. **Consequence is distinct from action.** Action is what the executor does;
   consequence is what happens to the world. The same verb against different
   resources is a different consequence.
5. **Authority must not exceed justification.** Specific evidence → broad belief →
   broader authority is forbidden without an explicit policy step that itself is
   auditable.
6. **Executor behavior must remain bound to the authorized consequence.** What is
   executed must equal what was authorized, verified — not promised — at the
   boundary Solvent controls.
7. **Future Warrant is a transport/verification representation, not a second source
   of truth.** Carrying a warrant confers nothing; the ledger's current state
   decides, verified centrally.

## Domain Objects

All objects below are PROPOSED except where a CURRENT paragraph says otherwise.

### Evidence

**Evidence Artifact vs Citation — semantics-only, no table sketch yet (OPEN #13).**

- **Evidence Artifact** = the thing actually observed (e.g., a corpus issue, a release manifest, a scan output). An artifact is reusable across beliefs in principle; reusability is a consequence of the model, not its definition.
- **Evidence Citation / Observation** = *this artifact is relevant to this particular belief* (the belief-specific relevance claim). Today a citation is collapsed into the evidence row itself (one row = one belief); the future model separates the reusable artifact from the per-belief citation that makes it discharge-relevant.

This distinction resolves the hidden contradiction (`plan3_imp_review.md:29`): a shared signed manifest should not be duplicated per belief, nor smuggled via duplicated references — the artifact is shared, citations are per-belief.

- **Grounded in CURRENT truth:** corpus artifacts are intentionally a shared provenance layer (`db/002_corpus.sql:6-9`), while ledger evidence rows are currently belief-owned (`db/001_schema.sql:50` `belief_id` FK, one row → one belief).
- **DebtDischarge connection:** sharing is allowed at the **artifact** level via **distinct citations**; **records are never shared** — each `Obligation → exactly one accepted Discharge`, each discharge cites one citation (`## Debt and Discharge Semantics`).
- **OPEN #13:** whether the eventual implementation uses `artifact` + `artifact_citation` tables or another representation remains OPEN; this semantic split is a prerequisite to the eventual I-11 provenance schema.

CURRENT `evidence` table is belief-owned; PROPOSED artifact/citation split is semantics-only, representation stays OPEN #13. PROPOSED extension for I-11 (server-verified hashing, ingress provenance enforcement, replay protection) is deferred; evidence **proposes**, it never discharges by existing.

### Belief

An epistemic proposition with a lifecycle (`entered → promoted → retracted`), a
claim type, and a set of open obligations. CURRENT: the `belief` table. PROPOSED
change: none to its columns or semantics. A belief encodes no principal, resource,
scope, action, or consequence — by Decision A, permanently.

### Obligation

A specific, typed condition that must be discharged before a belief is *eligible*
for promotion. Not "uncertainty" — structured unresolved work.

PROPOSED shape (semantic, not schema):

```text
Obligation
    obligation_key    stable identifier (e.g. needOperatorSignoff)
    obligation_type   'deterministic' | 'attested' | 'quorum'
    discharge_policy  what qualifies: which provenance classes, which actor
                      classes, how many independent instruments
    owner             customer policy — never the agent that benefits
```

The type vocabulary deliberately matches the future `debt_config` direction already
recorded (`docs/POST_HACKATHON_ARCHITECTURE.md:363-397`: `deterministic`, `attested`,
`quorum`). CURRENT approximation: the six-item `FullDebt` vocabulary
(`kernel/kernel.go:31-34`) with implicit, unwritten policies — which is exactly the
gap; the policy exists nowhere a CHECK or test can read it.

The obligation *set* for a belief is owned by policy (tenancy configuration), not by
the agent. An agent may not define its own exam, nor grade itself — the review finds
"one actor can create and discharge its own debt" today and recommends validation
that rejects empty or tautological debt sets
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:69-73`).

### DebtDischarge

A first-class, append-only fact: *this specific obligation of this specific belief
was satisfied by this specific instrument, accepted by/through this actor, at this
time, with this integrity claim.* It is the object that makes an empty debt array
meaningful.

PROPOSED shape:

```text
DebtDischarge
    discharge_id      unique id of this event
    belief_id         exact belief
    obligation_key    exact obligation instance being discharged
    instrument_kind   'evidence' | 'attestation'
    instrument_ref    evidence row id, or attestation reference
    discharged_by     attributable principal (human or verified system)
    issued_at         when the instrument was produced
    recorded_at       when Solvent accepted it
    status            'accepted' | 'refused' | 'revoked'
    integrity         proof binding (belief_id, obligation_key) into the
                      instrument's signed/verified payload
```

Rules (each is future invariant material, see `## Security Invariants`):

- **One discharge corresponds to one obligation.** A discharge names exactly one
  `(belief_id, obligation_key)`. It cannot clear a second obligation, even with the
  same instrument.
- **One instrument (Artifact) may support multiple obligations only via distinct Citations, and only when each obligation's discharge policy explicitly qualifies that instrument kind.** The same signed manifest Artifact can legitimately prove provenance for two beliefs via two separate Citations (OPEN #13 artifact/citation split); it can never silently double as an operator signoff. Two obligations, two discharge records (each citing its own citation) — the *policy* may allow sharing the artifact, the *record* may not be shared.
- **Replay is structurally refused:** a discharge is bound to its exact
  `(belief_id, obligation_key)`; re-presenting an attestation for a different belief
  or item is a mismatch, not a reuse.
- **An accepted discharge can be revoked** (e.g. attester key compromise), which
  re-issues the obligation — the debt array and the discharge ledger must then agree
  (see `## Promotion Preconditions` for which one wins: the array is the CHECK truth,
  the ledger is the justification truth, and a revocation must write both).
- **The agent may request or facilitate a discharge; it may not manufacture one.**
  This is `docs/ADR/0001:181-183` verbatim in intent.

### AuthorityTarget

The exact real-world consequence a specific principal is being permitted to cause —
see `## AuthorityTarget Definition`. The target answers *what is being authorized*.
It is not the warrant (which answers *why it is currently valid*).

### Justification link (the Belief↔Target relationship)

An explicit, auditable link between one AuthorityTarget and one belief it requires.
PROPOSED semantics (see `## Cardinality`): the target's justification set is a set of
such links; **all** must currently reference promoted beliefs (AND semantics). The
link is the only path by which a belief can participate in authority — there is no
implicit "promoted beliefs authorize their natural consequences."

### Warrant

Future, illustrative — see `## Warrant Semantics`. A portable reference to a
currently-valid authorization decision. Not a second source of truth.

### Action / Execution

What the executor actually does. PROPOSED: an execution claim must present the
authorized tuple verbatim; Solvent verifies exact equality before the executor may
proceed (see `## Executor Binding`). CURRENT: nothing records execution beyond
`action_intent.state ∈ {'live','cancelled','executed'}` — and only the intent's own
`action` string, decided by the caller, is on record
(`db/001_schema.sql:65-67`).

## Cardinality

Formal relations over PROPOSED objects. "Authorized" below means "has a currently
valid justification set", not "was once linked".

| Relation | Cardinality | Rule | Rationale |
|---|---|---|---|
| Belief → AuthorityTarget | 0..* via justification links | A belief may justify many targets, but **only** through an explicit link per target; each target is authorized independently | "Release X passed validation" may justify deploy-to-staging and deploy-to-production, but promotion creates neither; both targets must be created and authorized on their own (anti-broadening, F-06) |
| AuthorityTarget → Belief | 1..* required, AND semantics | A target must name ≥1 required belief; **all** required beliefs must be currently promoted for the target to be valid | Real productions need conjunctions: intended-release AND security-validation. OR remains future policy (no theorem prover) |
| Belief → Obligations | 1..* at entry, policy-owned | Every belief enters with the policy-defined obligation set; the set may shrink only via accepted discharges | CURRENT analogue: `FullDebt` six items at entry (`kernel/kernel.go:31-34`) |
| Obligation instance → Discharges | exactly 0..1 **accepted** (0..* recorded) | The discharge ledger is append-only; at most one discharge per obligation instance may hold status `accepted`; revocation re-issues the obligation and requires a fresh discharge | Blocks one-magic-citation-clears-everything (F-04); preserves audit history |
| AuthorityTarget → Warrant | 0..* over time, ≤1 current | Warrants reference a target and a justification snapshot; validity is determined by central verification against the ledger, not by counting warrants | Reassessment may invalidate and re-issue; the warrant is a projection (ADR Decision 4) |
| Warrant → Execution | MVP proposes 1; >1 is **OPEN** | An execution must cite the authorization it executed under; whether one authorization may cover multiple idempotent executions is deferred | Idempotency keys and batch semantics are unresolved (`## Open Architectural Questions`) |

### Why many-to-many through an explicit link (Decision B, challenged — sharpened by Review #1, the moved attack)

The four candidate cardinalities, evaluated:

- **1 belief → 1 target** — fails production: deployment genuinely needs "intended
  release" + "security validation" together (N→1 case). Also multiplies beliefs
  per consequence, exploding debt.
- **1 belief → many targets implicitly** — the broadening bug: staging evidence
  authorizes production deployment because both read "deploy". Rejected.
- **N beliefs → 1 target, explicit** — required (AND), kept.
- **M:N via explicit justification links** — the only model where *every* edge is a
  row someone created on purpose and an auditor can enumerate. Kept.

**Challenge sharpened by Review #1 (the moved attack).** M:N with links invites link-sprawl **and the moved broadening attack**: `Belief A = promoted` (legitimate) + attacker creates `Belief A → AuthorityTarget("move $10M")` → legitimate knowledge is linked to an illegitimate consequence. M:N invites the same via diligence failure — an operator auto-links every promoted belief to every target "to be safe", recreating wildcard authority through diligence failure. Mitigation is not a new mechanism but an existing discipline: links are auditable rows, so sprawl and attacker-created links are *visible* — unlike implicit broadening. Residual risk — **who may create a target, who may attach a belief to a target, who may request authorization, who may approve the target** (`plan3_imp_review.md:231`) and whether link creation itself needs justification — is **OPEN** and is now an explicit security boundary and schema-design prerequisite (`OPEN #8`).

**Schema expressibility note (PROPOSED, not a design).** The frozen pattern already
demonstrates how a link row can carry DB-enforced promotion-dependence: the `gate`
composite FK `(belief_id, belief_status) → belief(id, status)` makes an intent row
physically unable to cite a non-promoted belief, and `ON UPDATE CASCADE` +
`live_requires_promoted` make retraction detonate surviving liveness
(`db/001_schema.sql:68-75`). A justification link with the same composite FK shape
would inherit exactly this behaviour per-link. This is stated only to prove the AND
semantics are expressible in schema; the actual table design is deferred to the next
artifact. Whether *set-level* AND ("all links valid ⇒ target valid") can be a CHECK
or must remain service logic is **OPEN** — SQL evaluates row-locally, and
"every row of my set satisfies P" is not a row-local predicate.

## AuthorityTarget Definition

PROPOSED object. Five fields; each subsection answers its sub-questions and marks
what stays OPEN. After the five, `### Sufficiency challenge` asks whether five is
enough.

```text
AuthorityTarget
    principal     {principal_type, principal_id}
    resource      {resource_type, resource_id}
    scope         explicit constraint set (dimensions OPEN)
    action        {action_namespace, action_name}   (representation OPEN)
    consequence   {consequence_type, consequence_parameters}
```

### Principal

The entity on whose behalf the action is authorized: human, agent, service account,
workload, or (future) delegated principal. Solvent consumes a canonical principal
reference `{principal_type, principal_id}`; **identity resolution — authentication,
mapping to enterprise identity, workload attestation — is outside the kernel**, at the
service boundary, consistent with `docs/ADR/0001:499` (Solvent does not own identity).

- *One authority, multiple principals?* No: one target, one principal. A second
  principal is a second target (its own row, its own audit). This keeps tuple
  equality meaningful.
- *Delegation?* A delegated principal must have an independently verifiable
  relationship to the delegating principal. Full delegation chains are **OPEN** and
  out of MVP; MVP treats a delegate as its own principal requiring its own target.
- CURRENT: no principal exists anywhere (`db/001_schema.sql:60-76`;
  `docs/POST_HACKATHON_ARCHITECTURE.md:399-403` proposes a `principal UUID` column,
  unimplemented).

### Resource

The object against which the consequence occurs: URI, database object, API resource,
document, patient record, cloud object, workflow, account.

- **Opaque to Solvent.** `{resource_type, resource_id}` compared by exact equality.
  Solvent does not know what a patient is; it knows `patient/123 ≠ patient/999`.
- *Children/hierarchy?* Hierarchy may exist **only as explicitly declared
  relationships** — never as path-prefix implication. `/production` does not imply
  `/production/payment-api`. Inheriting authority downward is the classic broadening
  bug. Whether any hierarchy relation is needed at all for MVP is **OPEN**
  (default: none).
- *Equality semantics:* exact pair equality on `(resource_type, resource_id)`, no
  normalization, no case folding, no trailing-slash tolerance — the same discipline
  the project already applies to byte-sensitive query text and to never normalizing
  action strings at adapters (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:62`).

### Scope

**Q2 decision, locked semantically, open in vocabulary:**

> Scope constrains an AuthorityTarget; it may narrow authority but must never
> silently broaden the resource or consequence.

Conceptually:

```text
Scope = explicit set of constraints on where/when/how the authority applies
```

Examples: `environment=production`, `tenant=acme`, `region=us-east-1`,
`time-window=09:00–17:00`, `field=medication`. The **exact dimensions are OPEN**
until the repository or a concrete use case supplies evidence; the spec refuses to
bake today's guesses into the kernel.

Locked invariants (conceptual; enforcement representation OPEN):

```text
scope cannot expand resource
scope cannot expand consequence
scope cannot change the authorized action implicitly
unspecified scope ≠ wildcard authority
```

- *Part of identity, policy, or both?* Scope is part of the **target tuple**
  (identity) whose *dimensions* are supplied by policy. Two targets differing only
  in scope are two targets. `environment=production` does not mean "all production
  resources" unless a resource wildcard is explicitly, separately granted — which
  MVP does not grant at all.
- *Independent of resource?* Yes: resource names the object; scope bounds the
  conditions. `resource=payment-api` + `scope={environment=production}` authorizes
  payment-api *in production conditions*, not "everything in production".
- *Time-window constraints*: semantically in scope's constraint set; enforcement
  requires the deferred temporal machinery (`docs/ADR/0001:543` defers
  `validity_horizon`/`expires_at`). A time dimension is therefore
  **OPEN/deferred**, not silently enforceable now.

### Action

The verb of the operation: deploy, approve, update, delete, export, send, prescribe.

- Action strings are **not globally meaningful**: two systems can define different
  semantics for the same string. Action therefore carries a **canonical namespace**:
  `{action_namespace, action_name}`. Exact representation is **OPEN** (deferred to
  the API contract design), but the rule is fixed: **no adapter-side silent
  normalization** — the MCP/REST surfaces must not lowercase, alias, or canonicalize
  action names, precisely because normalization divergence between adapters is an
  attack (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:62`).
- CURRENT: `action TEXT` (`db/001_schema.sql:65`), opaque, un-namespaced.

### Consequence

The real-world effect that obtains if the authorized action succeeds. See
`## Consequence Semantics`. Structured as
`{consequence_type, consequence_parameters}`, agreed between Solvent and the executor
at authorization time, compared by exact equality, never interpreted semantically by
Solvent.

### Sufficiency challenge — are five fields enough?

The prompt requires not assuming sufficiency. Attacks on the five-tuple:

1. **Time** — authority valid 09:00–17:00 is a different object than an unbounded
   one. Not a sixth field: belongs to scope's constraint set, but enforcement is
   deferred (temporal authority, `docs/ADR/0001:543`). Marked **OPEN**; until
   temporal machinery exists, no time-scoped target may claim enforcement.
2. **Risk tier** — `HIGH_IMPACT` vs `LOW_RISK` shapes availability semantics
   (`docs/ADR/0001:113-134`) but is *policy over the target*, not part of its
   identity. Kept out of the tuple deliberately; **OPEN** how policy attaches.
3. **Idempotency/quantity** — "authorize once" vs "authorize up to N executions";
   deferred with Warrant→Execution cardinality. **OPEN**.
4. **Target identity rule (locked):** the tuple *is* the identity. Any change to any
   field — including scope constraints — produces a **different target** requiring
   **new authorization**. There is no in-place mutation of an authorized target.

## Consequence Semantics

Action describes what the executor does; resource is what it acts on; **consequence
is what happens to the world.**

```text
Action:      update_record
Resource:    patient/123
Consequence: {clinical_record_modified, record=patient/123, fields=[medication]}

Action:      update_record
Resource:    cache/temp-123
Consequence: {ephemeral_cache_state_modified, key=temp-123}
```

Same verb, same action string, different consequence — and the authorization
decision must differ with it, without Solvent containing any healthcare or
cache-specific logic.

- **Representation:** a canonical descriptor `{consequence_type,
  consequence_parameters}` — an explicit semantic object, not free text. Solvent
  compares descriptors by exact equality and stores them; it does not interpret
  them. It is **not** an ontology engine: no inference that `clinical_record_modified`
  subsumes `record_modified`.
- **Who supplies it:** declared in the authorization request, bound into the target,
  echoed by the executor, verified by equality. Policy may *constrain* which
  consequence types a tenant permits (e.g. a deny-list of types requiring extra
  justification), which is policy over targets, not interpretation inside them.
  An executor-reported-after-the-fact consequence cannot be authoritative for the
  authorization decision (it can only feed the deferred post-execution receipt —
  `## Executor Binding`).
- **Consequence ≠ action, and neither implies the other:** renaming an action does
  not change its consequence; the same action against two resources yields two
  consequences. A taxonomy of standard consequence types is **OPEN**
  (`## Open Architectural Questions`).
- CURRENT: no consequence exists anywhere in the schema, kernel, MCP, or Lean model.

## Debt and Discharge Semantics

### obligation

A typed, policy-owned condition attached to a belief at entry (or re-issued later —
see revocation). The obligation says *this must happen*. It does not say it happened.

### discharge

An accepted `DebtDischarge` record. The discharge says *this specific thing happened,
and here is why we accept that it satisfies that obligation*. The distinction
`obligation ≠ discharge` is the heart of F-04: the current system records only the
obligation's *absence*.

### qualifying evidence

What qualifies is fixed by the obligation's `discharge_policy`, by type:

- `deterministic` — machine-verifiable instruments: `provenance_class ∈
  {'reproducible_artifact','live_scan'}` with **server-verified** content hashes.
  An `external_feed` row alone never discharges a deterministic obligation without
  the ingress hardening of I-11, which does not exist yet (`## Self-Attack Results`).
- `attested` — an attestation meeting `docs/ADR/0001:160-179`: bound to the exact
  `(belief_id, debt_item)` inside the signed payload, attributable principal,
  independent channel, integrity proof, one attestation per debt item
  (`docs/ADR/0001:425`). The agent may request; it may not manufacture
  (`docs/ADR/0001:181-183`).
- `quorum` — **OPEN/deferred**: min_sources semantics exist as a direction
  (`docs/POST_HACKATHON_ARCHITECTURE.md:371-377`) but no use case yet forces them.

### attribution

`discharged_by` is a principal — human or verified system — never `agent:` for
attested obligations. The current path violates this openly: the MCP caller *is* the
discharger (`cmd/solvent-mcp/tools.go:104-130`), which the ADR already names as the
weak path (`docs/ADR/0001:140-148`).

### integrity

The binding `(belief_id, obligation_key)` must live **inside** the instrument's
verified payload (`docs/ADR/0001:175-178` — binding in the signed payload, not an
adjacent field). This is what makes cross-belief replay a mismatch instead of a reuse.

### replay

Three separate replay surfaces, three separate rules:

1. **Attestation replay across beliefs/items** — blocked by payload binding (above).
2. **Attestation replay of the same item after revocation** — requires attestation
   lifecycle (revocation propagation, `docs/ADR/0001:199-211`) — **deferred** (F-05).
3. **Evidence re-ingestion** — dedup today is per-belief `content_sha256` with an
   accepted TOCTOU window (`internal/belief/belief.go:103`); cross-belief reuse is
   unrestricted until I-11 exists — **deferred** (F-02).

### sufficiency for promotion

See `## Promotion Preconditions`. Discharge suffices for the promotion *transition*
only in conjunction; it never suffices for authority (Decision A).

### Attacks specific to these semantics

- *Obligation splitting* (`needOperatorSignoffPart1..6`): the obligation set is
  policy-owned; an agent cannot mint obligations. A tenant policy that itself defines
  vacuous obligations is a real hole — minimum-validity rules are **OPEN**
  (`## Open Architectural Questions`).
- *Self-graded exam*: the discharge policy is data the agent cannot write, and
  attested discharges require an independent channel. CURRENT code enforces neither;
  both are PROPOSED.

## Promotion Preconditions

The F-04 core. The question is not "what blocks promotion" but **exactly what makes
promotion legitimate** — stated as a precise condition set with an enforcement layer
for each element, and an honest list of what stays undecided.

### The proposed sufficient condition set

Promotion of belief `b` at time `t` is legitimate iff **all** of:

| # | Condition | Kind | Enforcement |
|---|---|---|---|
| N1 | `debt(b,t) = ∅` and `final_truth = false` | Necessary | **Schema, today**: `promoted_is_debt_free` CHECK, SQLSTATE `23514` (`db/001_schema.sql:30-32`) |
| N2 | Every obligation issued to `b` reached state `discharged` **only through an accepted DebtDischarge** satisfying its discharge policy; no obligation vanished except by accepted discharge or policy-approved re-issue | Necessary | **PROPOSED service layer** over an append-only discharge ledger; schema-enforceability **OPEN** (`## Lean / SQL / Service Responsibility`) |
| N3 | `b` is not retracted and no **open** contradiction is recorded against it | Necessary | Split: retraction half is **schema-real today** (`db/001_schema.sql:12`); contradiction half is **deferred** — contradictions are currently discarded with no ledger mutation (`internal/pipeline/pipeline.go:89-101`; `docs/POST_HACKATHON_ARCHITECTURE.md:2.1-2.3` defers the relation), so this conjunct is service-best-effort until it exists |
| N4 | Tenant policy permits promoting this claim (claim-type rules, obligation-set validity) | Necessary | **PROPOSED policy layer**, representation **OPEN** |

**Sufficiency statement (the answer to "is `debt=[]` sufficient?"):**

> `N1 ∧ N2 ∧ N3 ∧ N4` is proposed as **sufficient for the promotion transition** —
> and nothing less is. `N1` alone (the current system) is **necessary only**. And
> promotion itself — under any conditions — is **never sufficient for authority**:
> authority additionally requires an AuthorityTarget whose entire justification set
> is currently promoted (Decision A + B).

Equivalently:

```text
CURRENT (machine-checked today):  promotion ⇔ N1
PROPOSED:                          promotion ⇔ N1 ∧ N2 ∧ N3 ∧ N4
                                   promotion ⇏ authority  (ever)
                                   authority  ⇒ explicit target ∧ all justifications promoted
```

### Why N2 is stated as "reached empty only through accepted discharges"

An array's emptiness cannot explain itself. The debt array is the **CHECK truth**
(what the schema can enforce row-locally); the discharge ledger is the
**justification truth** (why the array may be empty). F-04 is precisely the gap
between them. Two consistency rules follow:

1. **No discharge without array change, no array change without discharge.** A
   retired item must have an accepted discharge record; an accepted discharge must
   retire its item — in one transaction, so the two truths cannot disagree in any
   committed state (the same single-writer-transaction discipline as
   `RetractCascade`, `kernel/kernel.go:127-149`, and I-7).
2. **Revocation writes both.** Revoking a discharge re-issues the obligation into the
   array and marks the ledger record revoked — atomically.

**Challenge (recorded, not resolved).** Rule 1 as a *schema* enforcement means a
CHECK or trigger correlating `belief.debt` mutations with discharge rows —
row-local SQL cannot cheaply assert "this `array_remove` has a matching accepted
discharge in this transaction". Candidate mechanisms (transactional stored logic,
restructuring debt as rows instead of an array) each carry real cost; the choice is
**OPEN** and deliberately not made in a semantics-first document. Until it is
closed, N2 lives at the service layer and is *auditable*, not *impossible* — an
honest weakening stated in `## Self-Attack Results`.

### Representation convergence (OPEN #14 — preferred long-term: obligation rows → derived debt)

The duality `belief.debt[]` (CHECK truth) + `DebtDischarge` ledger (justification truth) (`plan3_imp_review.md:245`) is **transitional**. The review correctly notes it creates two representations of one semantic fact (`debt=[] vs ledger says unpaid` and vice versa, `plan3_imp_review.md:260`). The **long-term preferred canonical model** (opinionated, per your “both” distinction) is:

```text
Obligation rows (policy-owned, per belief)
    ↓
DebtDischarge rows (append-only, accepted/refused/revoked)
    ↓
derived outstanding obligations = obligations − accepted discharges
    ↓
promotion predicate: outstanding == ∅ ∧ final_truth=false ∧ not retracted
```

The current `debt[]` array is a transitional representation of that derived set — it exists because the frozen kernel (`db/001_schema.sql:25-27`) already has a `TEXT[]` column and the `promoted_is_debt_free` CHECK (`db/001_schema.sql:30-32`) already makes the state `promoted ∧ debt≠[]` impossible. Any migration to rows must **preserve that impossibility semantics**: the state `promoted ∧ ∃ outstanding obligation` remains impossible in every committed state, by whatever mechanism replaces the array (row-local CHECK on a derived view, transactional stored logic, or obligation-row existence check). The exact schema mechanism is **OPEN** (no table sketch in this spec, per your `semantics-only` choice for evidence and `SEMANTICS LOCKED / LONG-TERM DIRECTION PREFERRED / EXACT SCHEMA OPEN`); what is locked is the direction and the preservation requirement.

### Necessary vs sufficient vs undecided — summary

- **Necessary, machine-checked now:** N1.
- **Necessary, proposed:** N2 (service + ledger), N3 (retraction half real,
  contradiction half deferred), N4 (policy, representation OPEN).
- **Proposed jointly sufficient:** N1–N4, for the promotion transition only.
- **Undecided:** whether N2 becomes schema-enforced; whether a temporal freshness
  conjunct (N5: evidence age / validity horizon) joins the set when temporal
  authority exists (`docs/ADR/0001:543`); whether quorum obligations add a conjunct;
  what minimum-validity rule protects against vacuous customer obligation sets.

**Challenge (the strongest counter-case, recorded).** A critic can say N2–N4 are
"conditions Solvent cannot yet enforce, assembled to feel rigorous." The honest
reply is in the Dogfood Evidence Ledger: N2–N4 are `Open`, not `Discharged`; this
spec's claim is only that they are *the right conditions*, and that shipping
authority-bearing integrations before they are enforced re-creates F-04 with more
vocabulary. That claim is argument, and is marked accordingly.

### N4's placement is contested (OPEN #16 — epistemic promotion vs policy as target-level authority eligibility)

`N4` — *tenant policy permits promoting this claim* (`plan3_imp_review.md:316`: `tenant policy permits promoting this claim`) — is **conceptually distinct** from `N1–N3`. `N1–N3` are epistemic (is the belief's own justification complete and not retracted?); `N4` is policy (does tenancy configuration allow this claim type and this obligation set to be promoted at all?). The question (`plan3_imp_review.md:305-355`):

> Is policy part of epistemic promotion, or is it a separate `Authority Eligibility` for targets?

is **OPEN** and not resolved in a semantics-first document. An alternative clean separation would be:

```text
Epistemic Promotion (belief):  N1 ∧ N2 ∧ N3  → "the belief has earned its epistemic status"
Authority Eligibility (target): epistemically promoted ∧ policy-valid ∧ justified ∧ current → "this belief may participate in this target"
```

Under that separation a policy change could revoke `AuthorityTarget` participation without changing the belief's `status='promoted'`, preserving `truth status ≠ authorization policy` (`plan3_imp_review.md:342`). The **conservative choice in this spec is to keep `N4` inside the conjunction** (`Promotion = N1∧N2∧N3∧N4`, `plan3_imp_review.md:309`) so no policy-permissive promotion is accidentally granted, but to mark the placement itself as **OPEN #16** — no redesign, no new column, no policy table sketched.

## Executor Binding

How the executor proves "I executed the thing Solvent authorized" — and what
prevents `patient/123 · modify_record` from becoming `patient/999 · delete_record`.

### The MVP contract: exact-tuple echo with server-side equality

PROPOSED authorization surface (semantic; transport is OPEN):

```text
Authorize(principal, resource, scope, action, consequence, justification)
    → decision bound to that exact tuple
```

The decision — and any warrant representing it — is bound to the **exact target
tuple** by canonical serialization. At execution time the executor must present the
**same tuple**; Solvent verifies **exact equality server-side**:

```text
authorize target
    → executor echoes target
    → Solvent verifies equality (server-side, not client-attested)
    → execute
```

The substitution attack dies at the verification boundary:

```text
Authorized: principal=A, resource=patient/123, action=update_record,
            consequence=modify_clinical_record
Executed:   principal=A, resource=patient/999, action=delete_record,
            consequence=erase_record
→ tuple inequality → NOT the authorized target → refused
```

### The executor contract, defined precisely

"The executor contract" is not prose; it is this set of obligations:

1. **Echo.** The executor MUST present the full target tuple — verbatim, no
   normalization — for verification before executing.
2. **Fail-closed.** If verification fails, or Solvent is unreachable for a
   consequential action, the executor MUST NOT execute (`docs/ADR/0001:113-122`:
   no answer is not ALLOW).
3. **No mutation.** The executor MUST NOT mutate resource, scope, action, or
   consequence after authorization. Any change is a **different target** requiring
   **new authorization** (`## AuthorityTarget Definition` — target identity rule).
4. **One decision, one tuple.** The executor MUST NOT apply one authorization
   decision to a second tuple (including "the same but for another record").
5. **CURRENT honesty clause.** Solvent cannot enforce conditions 1–4 *inside* the
   executor. What the MVP closes is the **substitution window at the verification
   boundary**: the forged tuple does not verify. What remains open — a fully
   compromised executor that verifies correctly and then does something else — is
   **deferred** to post-execution receipts/attestation and audit, and is recorded as
   such in `## Self-Attack Results`. This spec claims binding of *authorization*,
   not of *execution*, and forbids reading it as the latter.

### Mechanisms evaluated

| Mechanism | Verdict for MVP |
|---|---|
| Authorization-time echo + server-side equality | **Required** (this contract) |
| Executor precondition (verify-before-execute) | **Required** (conditions 1–2) |
| Executor callback | Deferred — audit value, no authorization value |
| Post-execution attestation / signed execution receipt | Deferred — closes the compromised-executor residual; future **I-13** strengthening (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:43` "optional post-execution attestation"), see `I-13 ExecutionEffectMatch` deferred |
| Resource/action/consequence hash | Implementation detail of tuple binding (canonical serialization implies it); not a separate semantic |
| Idempotency key | **OPEN** — needed exactly when a warrant covers >1 execution, which is itself OPEN |
| Execution result recording | Deferred with receipts |

### Two boundaries, two properties

This contract closes the **authorization substitution window**, not the execution-proof window — two boundaries, two properties (`plan3_imp_review.md:104`):

- **Authorization boundary — I-12 `AuthorizationTargetMatch` (this spec, enforced now at the verification boundary):** `authorized tuple == presented tuple`. Solvent verifies that the executor presented the same `principal/resource/scope/action/consequence` it was authorized to perform, by server-side exact equality. Tuple mutation is a different target and is refused here.
- **Execution boundary — I-13 `ExecutionEffectMatch` (reserved, deferred):** `presented tuple == actual world effect`. Solvent proves, via a trusted executor or signed execution receipt, that the world effect matches the presented tuple. A fully compromised executor that verifies the correct tuple and then does something else **passes I-12 and fails I-13** — the current `## Self-Attack Results` records this residual as `deferred` precisely because no receipt/trusted-executor mechanism exists in the MVP.

Conflating the two would let `ExecutorConsequenceMatch` sound like Solvent guarantees execution; separating them makes the architecture honest: **Solvent guarantees the executor *was authorized* for exactly what it presented (I-12); it does not yet guarantee what the executor *actually did* (I-13).**

## Warrant Semantics

Future, **illustrative only — no schema commitment**. Aligns with `docs/ADR/0001`
Decisions 1 and 4; this section adds nothing beyond them except target-tuple
precision.

A Warrant is a **portable reference to a currently-valid authorization decision**:

- **References:** the AuthorityTarget tuple, the justification set it was validated
  against (belief ids + snapshot of their status), issued_at, and its own identity.
- **Asserts:** "Solvent authorized this exact target, referencing these beliefs, as
  of `issued_at`."
- **Does NOT assert:** future validity; truth of the beliefs; permission for any
  other tuple; permission independent of verification. **Warrant is not authority;
  possessing it grants nothing** (`docs/ADR/0001:479`).
- **Not bearer, not PKI.** Central verification by Solvent is the primary model
  (`docs/ADR/0001:95-111`); a signed secondary mode for offline verification may
  come later, never as the primary. Key management stays bounded to the attestation
  boundary, not distributed to executors (`docs/ADR/0001:187-197`).
- **Three separations, kept explicit:** the *semantic* warrant object (this section);
  the *transport* representation (JSON shape, `docs/ADR/0001:440-466` is
  illustrative); the *verification* mechanism (central check against current ledger
  state: belief still promoted, debt still empty, no intervening retraction, no
  expiry violation, policy pass — `docs/ADR/0001:479`).
- **Validity:** determined *now*, per verification, from the ledger — never cached
  past the ledger's ability to retract. A retracted justification belief kills every
  warrant that cited it, centrally and immediately; that is the entire reason
  central verification was chosen (`docs/ADR/0001:101`).

CURRENT: no warrant exists; authority is the live-intent relationship
(`docs/ADR/0001:240`).

## Migration

From `action_intent.action TEXT` (CURRENT, `db/001_schema.sql:65`) to
`principal/resource/scope/action/consequence`. **Designed here, implemented never in
this document.** F-11 (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:136-143`).

### Locked principle: fail-closed

```text
old action-only row → missing new dimensions → NOT AUTHORIZED
```

Never `NULL → wildcard`. Never `DEFAULT ''` — an empty-string resource read as
"any resource" is precisely the F-11 attack
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:142`). Any interpretation of a missing
dimension as permissive requires an explicit, separately justified rule; none is
proposed.

### Strategy

- **Additive columns, nullable** (`principal, resource, scope, action_namespace,
  consequence` — final shape belongs to the schema design; a companion target table
  is the alternative, choice **OPEN**). Additive-only matches the recorded evolution
  discipline: never modify `db/001_schema.sql`, new numbered migrations only
  (`docs/POST_HACKATHON_ARCHITECTURE.md:491-503`), and the working precedent of
  `db/004_debt_vocabulary.sql` — which changed a default additively and deliberately
  did **not** rewrite existing rows (`db/004_debt_vocabulary.sql:33-39`).
- **Read path fails closed:** any authorization/verification encountering a row with
  missing dimensions treats it as `NOT AUTHORIZED — reauthorization required under
  the target model`. NULL means *unknown*, and unknown never means *yes*.
- **Old rows:** remain historically visible; their `action` string stays as recorded
  history. They cannot seed new consequential authority.

### Backfill

None automatic. New-dimensional values are minted **only by explicit
reauthorization** — a human/agent must request authority under the new model, at
which point a proper target is created. Backfill-by-inference (guessing a resource
from the action string) is forbidden: it manufactures dimensions Solvent never knew.

### Compatibility

Dual-period rule: during any transition the *old* path is not "still allowed" — it
is refused for consequential authorization. The four-verb demo's
`solvent_authorize_action` continues to work **only because it does not claim
resource-scoped authority** (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:44`
states this boundary); the moment target semantics land, action-only intents are
inert for new authority.

### Invalidation

`RetractCascade` semantics are unchanged and remain the invalidation mechanism for
justification beliefs (`kernel/kernel.go:114-150`). Target-scoped invalidation
(revoking a target without retracting beliefs) is **OPEN** — no mechanism is
proposed until targets exist.

### Rollback

Additive columns/tables are dropped; old semantics resume unchanged; target rows
become inert history. Rollback cannot resurrect authority that the fail-closed
period refused — correct, since that authority never existed.

### Required test debt (before any implementation)

`TestMigration_OldActionIsNotOverAuthorized` — an old action-only row must never
authorize under the new model, including via replayed warrant references
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:143`).

## Security Invariants

Proposed future invariants, prose only — no final syntax. IDs follow this spec's
numbering; the mapping table relates them to the adversarial review's future
invariants explicitly, **asserting mapping, never identity**.

- **I-9 `AuthorityTargetBinding`** — An authorization decision is bound to exactly
  one target tuple (principal, resource, scope, action, consequence); the decision is
  valid only while every justification link in the target's set references a
  currently promoted belief. Any tuple change requires new authorization.
- **I-10 `DebtDischargeIntegrity`** — A discharge is bound to exactly one obligation
  of one belief, by an attributable principal, through an instrument whose verified
  payload contains that `(belief_id, obligation)` binding. An empty debt array is
  promotable only if every issued obligation has an accepted discharge (N2). The
  array and the ledger cannot disagree in a committed state.
- **I-11 `EvidenceProvenance`** — Evidence identity and provenance are verified at
  ingress (server-side content hashing, provenance-class enforcement, replay
  protection). Evidence that cannot be traced is decoration, not discharge
  material. (Extends `AGENTS.md` "Evidence must be attributable" to ingress
  enforcement; currently unimplemented — F-02.)
- **I-12 `AuthorizationTargetMatch`** — What was authorized equals what is presented for execution: the executor-presented tuple must match the **authorized tuple** by server-side exact equality before execution; resource/consequence mutation voids authorization. This is the **authorization boundary** — Solvent verifies what was authorized, not what actually happened.
- **I-13 `ExecutionEffectMatch` (reserved, deferred)** — What was actually executed as a world effect equals the authorized tuple. Requires a trusted executor, signed execution receipt, or equivalent post-execution proof. Not enforced by this spec; see *Two boundaries, two properties* in `## Executor Binding` (`plan3_imp_review.md:104`).

### Cross-reference to the adversarial review's future invariants

| Spec ID | Spec meaning | Adversarial review relationship |
|---|---|---|
| I-9 | Authority target bound to the authorized principal/resource/scope/action/consequence | Review I-9: resource/consequence binding (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:202`) — same intent, widened to the full tuple and justification-set validity |
| I-10 | Debt discharge bound to the exact obligation, with required integrity/attribution | Review I-10: attestation binding (`:203`) — spec I-10 covers review I-10's attestation binding as its `attested` case, plus deterministic instruments |
| I-11 | Evidence provenance and replay enforced at ingress | Review I-11: evidence provenance (`:204`) — aligned |
| I-12 | Authorized tuple matches the presented tuple (exact equality at the authorization boundary) | Review I-9 / F-01 consequence binding (`:36-44`) — the authorization-boundary half of review I-9; renamed from `ExecutorConsequenceMatch` to avoid overclaim |
| I-13 (reserved) | World effect matches the authorized tuple (requires execution receipt / trusted executor) | Review I-13 (`:206`) — deferred temporal/reassessment-adjacent, plus new execution-effect verification (`plan3_imp_review.md:144` two boundaries); deferred, not in this spec |
| — | Availability / fail-closed semantics | Review I-12 (`:205`) — governed by `docs/ADR/0001` Decision 2, outside this spec's four |

The review's numbering is not renumbered and not redefined; where meanings differ
(spec I-12 ≠ review I-12), the table states the mapping rather than implying
identity.

## Lean / SQL / Service Responsibility

| Property | Enforcement layer | Why |
|---|---|---|
| Promoted belief has no debt (N1) | SQL CHECK — today (`db/001_schema.sql:30-32`); Lean — proved (`promote_preserves_validity` family, `formal/lean/Solvent/Preservation.lean`) | The state must be *impossible*, and it is row-local — SQL's home ground |
| Live intent references promoted belief | SQL composite FK + CHECK — today (`db/001_schema.sql:68-75`); Lean — proved (`live_intent_implies_promoted`) | Impossible-state, row-local |
| Retraction leaves no live dependent intent | SQL transaction + cascade re-evaluation — today (`kernel/kernel.go:114-150`); Lean — proved (`cascade_retraction_cannot_leave_live_intent`) | Atomicity; the schema enforces ordering even when code forgets it |
| Justification link cites only promoted beliefs (per-link half of I-9) | SQL — candidate: same composite-FK pattern as `gate` (`db/001_schema.sql:74` shape, PROPOSED); Lean — only after the SQL exists | The frozen pattern already proves this shape works row-locally; formalizing before the constraint exists would prove an abstraction (F-12 discipline, `docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:146-153`) |
| Target valid ⇒ all justification links valid (set-level AND of I-9) | Service now; schema **OPEN** | "Every row of my set satisfies P" is not row-local; a CHECK may not express it without restructuring |
| Discharge ↔ array consistency (I-10, one-accepted-per-obligation) | Service + append-only ledger now; unique-constraint candidate later; **OPEN** | Correlating an array mutation with ledger rows across tables is not row-local; a unique `(belief_id, obligation)` constraint on accepted discharges is the promotable core |
| Attestation authenticity / channel independence | Attestation mechanism + service (ADR boundary, `docs/ADR/0001:187-228`) | Identity/integrity interpretation, not ledger state |
| Authorized tuple matches presented tuple — **I-12 `AuthorizationTargetMatch`** | Service (server-side equality) + executor contract; service integration tests | Authorization boundary (spec I-12 ≠ review I-12, see cross-reference); DB stores the tuple, boundary verifies — does **not** prove world effect (see I-13 `ExecutionEffectMatch` deferred) |
| Warrant currently valid | Solvent service (central verification, `docs/ADR/0001:95-111`) | Validity is a *projection of current ledger state*, by decision |
| Obligation qualifies for discharge (policy match) | Service + evidence/attestation rules | Domain/policy semantics, explicitly not kernel |
| Migration fail-closed (no NULL→wildcard) | Service read-path + `TestMigration_OldActionIsNotOverAuthorized` | The rule is interpretation of missing data; the test pins it |

Rule (inherited from the review and `AGENTS.md`): *if a state must be impossible,
put it in the database; if it requires interpretation, put it in the service; if it
is a state-machine property, prove it in Lean — but only once the corresponding
executable semantics exist.* Lean is not expanded for symmetry
(`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:153`).

## Cross-Domain Examples

Four scenarios; the Solvent mechanism is identical in each — only policy and
evidence vocabulary change. All PROPOSED.

### Security — DNS change

```text
Principal:    {agent, dns-ops-agent}
Resource:     {dns_zone, example.com}
Action:       {dns, update_record}
Scope:        {environment=production}            (dimension illustrative, vocabulary OPEN)
Consequence:  {production_dns_record_modified, record=api.example.com}
Beliefs:      "change request 4117 is the approved record set"
              "the new record has propagated in staging"
Obligations:  needOperatorSignoff (attested: on-call human, independent channel)
              needProvenanceCheck (deterministic: signed change artifact)
Discharge:    two accepted DebtDischarge records, one per obligation
```

### Document — contract approval

```text
Principal:    {agent, compliance-agent}
Resource:     {contract, 78219}
Action:       {docproc, approve}
Scope:        {department=procurement}
Consequence:  {contract_marked_approved, contract=78219}
Beliefs:      "contract 78219 is complete"
              "all required clauses are present"
Obligations:  needProvenanceCheck (deterministic: document hash)
              needOperatorSignoff (attested: legal reviewer)
Discharge:    one accepted record per obligation; the same document hash may be
              cited by both records only if each policy qualifies it — the signoff
              policy does not, so a human attestation is required regardless
```

### Healthcare — clinical record modification

```text
Principal:    {agent, clinical-agent}
Resource:     {patient_record, 123}
Action:       {emr, update_record}
Scope:        {section=medication}
Consequence:  {clinical_treatment_record_modified, record=123, field=medication}
Beliefs:      "medication M is indicated for patient 123"
              "the contraindication check passed for patient 123"
Obligations:  needContradictionSweep (deterministic)
              needOperatorSignoff (attested: attending clinician — never the agent)
Discharge:    clinician attestation bound to (belief, needOperatorSignoff) inside
              the signed payload; agent-facilitated, never agent-asserted
```

### Finance — payment approval

```text
Principal:    {agent, payments-agent}
Resource:     {transaction, 9912}
Action:       {payments, approve}
Scope:        {program=reimbursement}
Consequence:  {funds_released, transaction=9912}
Beliefs:      "customer is verified"
              "transaction 9912 is within policy"
Obligations:  needProvenanceCheck (deterministic: identity verification artifact)
              needBlastRadius (deterministic: limits check)
              needOperatorSignoff (attested: payments operator)
Discharge:    three accepted records; the AND over both beliefs and all three
              obligations is what the target requires — no single belief or
              discharge suffices
```

### Generality / Anti-Broadening — Verified Customer Identity

*Fifth example — the strongest generality test, placed here per Q2 to demonstrate `promoted belief ≠ permission` in an ordinary business scenario rather than only in the attack section. Also the hostile 1→3 demonstration referenced by `Self-Attack #15`.*

```text
Belief:
    "customer identity was verified"

        ≠

Permission to:
    view profile
    issue refund
    transfer funds
```

Explicitly:

```text
Belief:
    "customer identity was verified"

Target A: view customer profile
    Principal:   {agent, support-agent}
    Resource:    {customer_profile, customer_123}
    Action:      {profile, view}
    Scope:       {tenant=acme, environment=production}
    Consequence: {customer_profile_viewed, customer=customer_123}
    Required justifications: Belief "customer identity was verified" (alone may suffice)
    Obligations:  needProvenanceCheck (deterministic: identity artifact)
    Discharge:    one accepted record citing the customer-verified artifact via its
                  per-belief citation (OPEN #13). Promotion alone creates no permission;
                  Target A must be explicitly created and authorized.

Target B: issue refund
    Principal:   {agent, refunds-agent}
    Resource:    {payment, refund_9912}
    Action:      {payments, issue_refund}
    Scope:       {program=reimbursement}
    Consequence: {funds_released, refund=refund_9912}
    Required justifications: Belief "customer identity was verified"
                         AND Belief "transaction 9912 is within policy"
    Obligations:  needProvenanceCheck + needBlastRadius + needOperatorSignoff
    Discharge:    three accepted records; the AND over both beliefs is required — the
                  single customer-identity belief participates in Target B but does not
                  authorize it alone.

Target C: transfer funds
    Principal:   {agent, treasury-agent}
    Resource:    {account, treasury_acme}
    Action:      {treasury, transfer}
    Scope:       {environment=production, amount_tier=high}
    Consequence: {funds_transferred, amount=high, account=treasury_acme}
    Required justifications: Belief "customer identity was verified"
                         AND Belief "transaction 9912 is within policy"
                         AND Belief "high-impact transfer reviewed"
    Obligations:  needProvenanceCheck + needOperatorSignoff (attested: treasury operator)
                  + additional high-impact requirements (policy-owned)
    Discharge:    requires all; promotion of the single identity belief is necessary
                  for each target but never sufficient for any.

Conclusion: A promoted belief can **participate** in multiple targets, but promotion never creates those permissions implicitly. Each target must be **explicitly created, explicitly justified with its required AND beliefs/obligations, and independently authorized** via exact tuple equality (`I-12 AuthorizationTargetMatch`). This is the same explicit-link discipline that blocks implicit broadening (F-06) — here exercised on a general-purpose, non-security workflow. The corresponding attack — an attacker linking a legitimate promoted belief to an illegitimate high-impact target — is recorded as `Self-Attack #15 (unresolved — moved broadening)` and is visible as an auditable row rather than an implicit privilege.
```

In each case: same four tables plus the proposed target/justification/discharge
objects; same CHECKs; same tuple equality; same fail-closed rules. Solvent learns no
medicine, finance, or DNS — it learns none of them.

## Open Architectural Questions

Only genuinely unresolved questions. Each blocks the corresponding schema decision.

1. **Scope dimensions** — which constraint dimensions exist and their comparison
   semantics (equality, ranges, set membership). Time-window scope additionally
   depends on deferred temporal authority. (Q2, deliberately open.)
2. **Action namespace representation** — `{namespace, name}` vs prefixed strings vs
   registry; who maintains namespaces; versioning of verb semantics.
3. **Consequence taxonomy and versioning** — is `{type, parameters}` stable enough
   for cross-system equality? What happens when an executor upgrades and rewrites a
   type? (Equality across versions is an OPEN hazard the tuple-identity rule makes
   sharp: a version bump is a different target.)
4. **Delegation chains** — representation and independent verifiability of
   delegating-principal relationships; explicitly out of MVP.
5. **Principal representation MVP** — opaque `{type, id}` vs UUID column
   (`docs/POST_HACKATHON_ARCHITECTURE.md:399-403` direction) vs richer reference;
   where identity resolution sits.
6. **Schema enforceability of N2 / I-10** — discharge-vs-array consistency in
   transactional SQL; row-locality limits; whether debt becomes rows.
7. **Set-level AND (I-9 second half)** — CHECK vs service; trigger-free options.
8. **Who may create justification links and targets** — authorization for
   authorization objects; sprawl controls beyond audit visibility.
9. **Vacuous-policy defense** — minimum-validity rules for tenant obligation sets
   (the customer-defines-meaningless-obligations attack remains unresolved).
10. **Warrant → Execution cardinality** — single vs idempotent multi-execution;
    idempotency key design.
11. **Target-scoped revocation** — revoking a target without retracting its beliefs.
12. **Risk tier attachment** — how `HIGH_IMPACT` classification
    (`docs/ADR/0001:113-134`) attaches to targets without joining the identity tuple.
13. **Evidence artifact vs citation** — whether to model a reusable `Evidence Artifact` (what was observed) separately from a per-belief `Evidence Citation` (why this artifact is relevant to this belief); which representation, if any, reaches the ledger; how sharing a single artifact across beliefs via distinct citations interacts with I-11 provenance and one-citation-per-debt. Semantics locked as `Artifact vs Citation` with reuse at artifact level via distinct citations (`OPEN #13`); concrete table sketch remains OPEN.
14. **Debt duality convergence — array vs rows** — whether `belief.debt[]` (CHECK truth) + `DebtDischarge` ledger (justification truth) remains service-enforced dual truth or converges to the preferred long-term canonical `Obligation rows → DebtDischarge rows → derived outstanding debt` while preserving `promoted_is_debt_free` impossibility (`db/001_schema.sql:30-32`). Long-term direction preferred as rows-derived, exact schema OPEN (`OPEN #14`).
15. **Execution-effect verification (I-13 `ExecutionEffectMatch`)** — what, if anything, proves that the world effect actually matches the presented tuple; whether a trusted executor or signed execution receipt is in scope for MVP; where the authorization-vs-execution boundary is drawn (I-12 vs I-13). Deferred (see `Two boundaries, two properties`).
16. **N4 placement** — whether tenant-policy permissibility (`N4`) is a fourth promotion conjunct (`Promotion = N1∧N2∧N3∧N4`) or a target-level `Authority Eligibility` (belief promoted, target policy-valid + justified + current). Conservatively kept as `N1∧N2∧N3∧N4` in this spec; placement itself is **OPEN #16**.

## Recommended Next Decisions

Decisions that must be made **before any schema change**, in priority order:

1. **Consequence descriptor representation + versioning** (OPEN #3) — the tuple's
   most novel member; everything in I-9/I-12 keys on its equality semantics.
2. **Action namespace representation** (OPEN #2) — same reason, smaller blast radius.
3. **Justification-link schema mechanism** — adopt or reject the composite-FK
   extension of the `gate` pattern (`## Lean / SQL / Service Responsibility`); this
   determines whether per-link I-9 is impossible-by-schema or service-checked.
4. **N2 enforceability decision** (OPEN #6/#7) — which parts of discharge integrity
   become constraints vs service checks; the answer sizes the migration.
5. **Migration shape** — additive columns vs companion target table (OPEN), plus the
   fail-closed read-path contract and `TestMigration_OldActionIsNotOverAuthorized`.
6. **Principal MVP representation** (OPEN #5) and the identity-resolution boundary.
7. **Executor contract document** — the five conditions of `## Executor Binding`
   written as the integration-facing contract before any executor integration
   claims I-12.

## Implementation Gate

Conditions that must hold **before Phase 1/2 kernel or schema changes may begin**:

1. **This specification is promoted through its own adversarial review** — the review
   that follows this document is the gate, exactly as the MVP plan's review gated
   this spec.
2. Every **OPEN** item above is either resolved, or explicitly bounded out of the
   first implementation phase with its deferral recorded as debt.
3. Every proposed invariant (I-9..I-12) has an assigned enforcement layer in the
   responsibility table with rationale — no invariant left "somewhere".
4. The migration design (including the fail-closed read path and the required test)
   is reviewed; `NULL→wildcard` is impossible by construction and by test.
5. **The frozen architecture is preserved**: `db/001_schema.sql` is not modified; new
   tables arrive as new numbered migrations
   (`docs/POST_HACKATHON_ARCHITECTURE.md:491-503`) and only with explicit approval
   per `AGENTS.md:54`.
6. No new authority source: the four tables' existing constraints keep their exact
   semantics; additions are additive (`docs/ADR/0001:520-529`).
7. The executor contract exists in writing before any integration claims
    consequence binding.
8. The dogfood ledger below is re-scored after review: nothing may move to
    `Discharged` without implementation evidence.
9. The link/target creation authority boundary (OPEN #8) — who may create a target, attach a belief, request authorization, approve the target — is either resolved or explicitly bounded out of the first implementation phase with its deferral recorded as debt.
10. The debt-representation convergence direction (OPEN #14) is agreed as the preferred long-term `Obligation rows → DebtDischarge rows → derived debt` with the explicit constraint that any migration preserves `promoted_is_debt_free` impossibility (`db/001_schema.sql:30-32`) by the same I-7 transactional discipline that guards `RetractCascade`.

## Self-Attack Results

The strongest attacks against this proposed model, each with its honest disposition.
Categories exactly as required: *blocked by invariant / blocked by service contract /
blocked by attestation / detected by audit / deferred / unresolved*.

| # | Attack | Disposition |
|---|---|---|
| 1 | Executor substitutes resource/consequence after authorization (`patient/999 · delete_record`) | **Blocked by service contract** (I-12: server-side tuple equality — the forged tuple does not verify). Residual — compromised executor that verifies correctly then does otherwise: **deferred** (post-execution receipts). |
| 2 | Dishonest agent discharges its own belief's obligations | **Blocked by attestation** (proposed: independent channel + payload binding, `docs/ADR/0001:160-183`). CURRENT code does not block this — the proposal is not yet behaviour; scored `Open` in the ledger. |
| 3 | One evidence row / one citation clears many obligations | **Blocked by invariant** (proposed I-10: one discharge → one obligation; shared instrument only via explicit per-obligation policy). CURRENT `internal/belief/mapping.go:20` does the opposite — proposal vs reality stated. |
| 4 | Agent splits obligations into trivial sub-items (`…Part1..6`) | **Blocked by service contract + detected by audit** (policy-owned obligation sets; append-only discharge ledger makes the split visible). Minimum-validity rule for *tenant-defined* vacuous sets: **unresolved** (OPEN #9). |
| 5 | Compromised evidence source manufactures justification (forged artifact retires provenance debt) | **Deferred** to ingress hardening (I-11 / F-02); until then deterministic discharges rest on caller-supplied hashes — **unresolved in current code** and named as such. |
| 6 | Migration broadens old rows (`resource=''` read as wildcard) | **Blocked by invariant** (fail-closed rule; NULL/missing = NOT AUTHORIZED) **and by test** (`TestMigration_OldActionIsNotOverAuthorized`). |
| 7 | Concurrent promotion vs retraction creates an invalid intermediate (live intent on retracted belief; discharge vs array race) | **Blocked by invariant** for existing semantics (`gate`/`live_requires_promoted` under `crdb.ExecuteTx`; `40001` retries then refuses on fresh state). For proposed N2 consistency: **blocked by design** (single-transaction discharge+array write) — **undecided** whether schema can guarantee it alone (OPEN #6). |
| 8 | Stale warrant replayed after belief retraction | **Blocked by service contract** (central verification: retraction invalidates immediately, `docs/ADR/0001:101`). Executor caching beyond verification: **deferred** (review F-07 / review I-12 availability). |
| 9 | Scope silently broadens (`environment=production` read as "all production resources") | **Blocked by invariant** (scope narrows-only; unspecified ≠ wildcard). Enforcement representation: **OPEN** (#1) — the *rule* is fixed, its mechanical check is not yet designed. |
| 10 | Weak discharge chain becomes strong authority (F-01+F-04 interaction) | **Blocked by invariant** (Decision A/B: promotion ⇒ eligibility only; authority requires explicit target with full AND justification — authority strength ≤ justification strength by construction). "Strength" is not formally defined: **unresolved** as a metric; the structural block is the proposed answer. |
| 11 | Buggy executor performs the wrong action after verifying the right tuple | **Unresolved** beyond contract terms 1–4; **deferred** (receipts, audit). This spec binds authorization, not execution, and says so. |
| 12 | Customer configures meaningless obligation policy; promotion becomes theater again | **Unresolved** (OPEN #9). Recorded, not hidden. |
| 13 | Justification-link sprawl recreates wildcard authority through operator diligence failure | **Detected by audit** (links are explicit rows; sprawl is enumerable). Preventive controls (who may link): **unresolved** (OPEN #8). |
| 14 | Attacker replays a valid attestation for a second belief/item | **Blocked by attestation** (payload contains `(belief_id, obligation)` binding — cross-use is a mismatch, `docs/ADR/0001:175-178`). Replay of a revoked attestation: **deferred** (key lifecycle, F-05). |
| 15 | Attacker creates a new AuthorityTarget and links a legitimate promoted belief to an illegitimate high-impact consequence (`Belief A → Target("move $10M")`) — the moved broadening attack (`plan3_imp_review.md:182`, `mvp_plan3_a2.md`) | **Unresolved — moved broadening**. The link is an **auditable row** so the attack is **Detected by audit** (enumerable, visible). Preventive enforcement (who may create a target, attach a belief, request authorization, approve) is **OPEN #8** — four-role decomposition is the prerequisite. Until the link-creation boundary is enforced, M:N explicit links **move** the broadening attack from implicit promotion to explicit linking rather than close it. **Deferred** to schema-design; new **Implementation Gate #9** requires it. See also cross-domain `## Cross-Domain Examples` fifth case for the generality anti-broadening demonstration. |

## Dogfood Evidence Ledger

This specification treated as a belief. Per the locked rule: `Discharged` requires
repository-verifiable evidence (code, schema, or an accepted-and-cited decision *as
a decision*); reasoned design stays `Open`; deliberate postponement is `Deferred`;
only `Discharged / Open / Deferred / Contradicted` are used. Nothing is `Discharged`
because it sounds reasonable.

| Design Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| CURRENT promotion treats `debt=[]` as sufficient | `db/001_schema.sql:30-32` (CHECK is the only gate); `kernel/kernel.go:90-100` (Promote inspects nothing); `internal/belief/belief.go:87-96` (auto-promote; block swallowed) | High | — | Discharged |
| CURRENT one evidence row can retire multiple obligations | `internal/belief/mapping.go:20` (one `release` match retires two items); retire loop `internal/belief/belief.go:77-85` | High | Contradicts `AGENTS.md:74` spirit today | Discharged |
| CURRENT `needOperatorSignoff` discharges on a text pattern with any caller | `internal/belief/mapping.go:26`; `cmd/solvent-mcp/tools.go:104-130` (enum check only) | High | Attestation absent | Discharged |
| A belief alone is not authority today; authority is the live-intent relationship | `db/001_schema.sql:60-77` (gate FK); `formal/lean/Solvent/Preservation.lean` (`live_intent_implies_promoted`); `docs/ADR/0001:240` | High | — | Discharged |
| No principal/resource/scope/consequence exists anywhere current | `db/001_schema.sql` (whole file); `formal/lean/Solvent/Types.lean:47-51`; `docs/POST_HACKATHON_ARCHITECTURE.md:399-419` (proposed only) | High | — | Discharged |
| Contradictions are discarded without ledger mutation today | `internal/pipeline/pipeline.go:89-101`; `internal/belief/belief.go:38-48` | High | Contradiction relation deferred | Discharged |
| CURRENT ledger evidence is belief-owned (one row = one `belief_id`); no reusable artifact object exists | `db/001_schema.sql:50` (`belief_id` FK, one row → one belief); `db/002_corpus.sql:6-9` (corpus is shared, ledger is not) | High | — | Discharged |
| CURRENT execution is caller-asserted `action TEXT` with no receipt | `db/001_schema.sql:65-67` (`action` string decided by caller, `state` only); no `ExecutionEffectMatch` | High | — | Discharged |
| Artifact vs Citation reuse — one artifact may justify many beliefs via distinct citations; records never shared | Semantics locked in `## Domain Objects → Evidence` (OPEN #13: Artifact = observed, Citation = relevance to one belief); sharing at artifact level via distinct citations | Medium | Table representation OPEN #13 | Open |
| Link/target creation authority — who may create a target, attach a belief, request authorization, approve | Design reasoning — links are explicit auditable rows, creator not yet gated (`## Cardinality` moved attack, Self-Attack #15) | Medium | OPEN #8 (four-role decomposition: create target / attach belief / request / approve) | Open |
| Debt array vs rows — transitional `belief.debt[] + Discharge ledger` vs preferred long-term `Obligation rows → Discharge rows → derived debt` | Design reasoning; preferred direction `## Promotion Preconditions` representation convergence (`OPEN #14`); must preserve `promoted_is_debt_free` impossibility (`db/001_schema.sql:30-32`) | Medium | Exact schema OPEN #14 | Open |
| N4 placement — policy as 4th promotion conjunct vs target-level Authority Eligibility | Design reasoning; conservatively `Promotion = N1∧N2∧N3∧N4` kept, placement contested (`plan3_imp_review.md:305-355`) | Medium | OPEN #16 | Open |
| Decision A: promotion must remain eligibility, never permission | Design reasoning; consistent with `docs/ADR/0001:349` (agent output is not authority) | Medium | No implementation; needs target layer to be meaningful | Open |
| Decision B: M:N via explicit justification links; AND for MVP | Design reasoning (`## Cardinality` evaluation) | Medium | Link-creation authority (OPEN #8); set-level enforceability (OPEN #7) | Open |
| Decision C + N1–N4: the promotion sufficient-condition set | Design reasoning (`## Promotion Preconditions`); N1 is schema-real, N2–N4 are not | Medium | N2 schema enforceability (OPEN #6); temporal conjunct undecided | Open |
| Exact-tuple echo + server-side equality is the MVP executor-binding minimum | Design reasoning; direction matches `docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:43` | Medium | Residual compromised-executor risk deferred; idempotency OPEN | Open |
| Scope is an open constraint set; narrows, never broadens | Semantic rule locked by review decision; no dimension evidence in repo | Medium (rule) / Low (dimensions) | All dimensions (OPEN #1) | Open |
| Resource opacity + explicit-only hierarchy prevents prefix broadening | Design reasoning; consistent with the equality discipline in `AGENTS.md` | Medium | Hierarchy relation need unproven | Open |
| Fail-closed migration (missing dimensions ⇒ NOT AUTHORIZED) | Design reasoning; F-11 attack analysis; `db/004_debt_vocabulary.sql` precedent for additive-only | Medium | Test not yet written (named debt) | Open |
| Central warrant verification retained; warrant ≠ authority | Accepted decision `docs/ADR/0001:95-111,479` — evidence of a *decision*, not behaviour | High (as decision) | Outage/cache tests (review F-07) | Deferred |
| A2A remains outside this model | `docs/ADR/0001:300-311` (Decision 7 sequencing) | High | — | Deferred |
| Attestation lifecycle (keys, revocation, replay cache) | `docs/ADR/0001:187-211` bounds it; unresolved | — | Full lifecycle spec (F-05) | Deferred |
| Evidence ingress hardening (I-11) | Not begun; fixture-only today (`internal/pipeline/pipeline.go:41-57`) | — | Gateway design (F-02) | Deferred |
| Temporal scope/freshness (N5, review I-13) | `docs/ADR/0001:543` defers temporal authority | — | Validity-horizon machinery | Deferred |
| Quorum obligations | Direction only (`docs/POST_HACKATHON_ARCHITECTURE.md:371-377`); no forcing use case | — | Use case | Deferred |
| Authority strength ≤ justification strength, structurally | Decision A/B construction (attack #10 blocked structurally) | Medium | No formal "strength" definition; no Lean formalization until schema exists | Open |

---

*This document changed no code, no SQL, no Lean, no MCP behaviour, no ADR, and no
table. The four frozen ledger tables of `db/001_schema.sql` are untouched. No new
authority source was introduced. Every future object described here — AuthorityTarget,
Justification link, DebtDischarge, Warrant — is PROPOSED and NOT IMPLEMENTED.*
