# Schema Readiness Decision

## Status

**PROPOSED — NOT IMPLEMENTED**

This is a decision artifact, not a description of shipped behaviour. It determines what the
eventual production schema MUST represent and what it MUST NOT try to represent. It contains
no DDL, adds no table, column, constraint, kernel function, MCP tool, REST endpoint, or Lean
theorem. Nothing here has been implemented.

**Status taxonomy used throughout this document:**

```text
CURRENT / VERIFIED       exists in the repository now; cited by file:line
DOCUMENTED CONTRACT      a named invariant or rule from the architecture corpus
                         (I-*, N-*, ADR decisions). Earned by a document, NOT
                         earned by the repository. Cited to its source spec.
PROPOSED                 decided here, not yet built; binds future design
FUTURE / UNSCOPED        intentionally beyond the current implementation boundary,
                         no design yet
OPEN                     unresolved semantics that affect the current design;
                         must be resolved during schema design
DEFERRED                 intentionally outside the current implementation boundary,
                         with an explicit promotion condition preserved
```

Two disciplines govern every claim:

1. **CURRENT/VERIFIED claims cite repository truth** (`file:line`). Where the repository and
   any document disagree, the repository wins and the disagreement is recorded, not silently
   reconciled.
2. **Invariant IDs are earned, not assigned.** `I-A4`, `I-P1`, `I-11`, `I-12`, `I-13`, `N1`,
   `N2`, and similar identifiers appear here only as **DOCUMENTED CONTRACT** labels with a
   citation to their source specification. They are not repository-verified invariants and
   must never be narrated as if they were. The only repository-verified invariants cited in
   this document are the SQL constraints `promoted_is_debt_free`, `gate`,
   `live_requires_promoted`, and `belief_id_status_key` (`db/001_schema.sql`).

**Repository-truth note.** The documents this artifact synthesizes —
`kernel-enforcement-boundary-and-trust-model.md`, `review7.md`, and `review8.md` — currently
live in the external working folder (`~/Desktop/biz/solvent/v2/`), not in this repository.
The in-repo architecture corpus is `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`,
`docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`, and
`docs/ADR/0001-authority-and-attestation.md` (both `docs/ARCHITECTURE/` specs carry status
**PROPOSED — NOT IMPLEMENTED**). Citations below use actual locations.

---

## Purpose

Solvent has completed the architecture-discovery phase. Reviews #1–#7 established the trust
model, the four-role authority separation, the policy floor, and the enforcement taxonomy;
Review #7 found fourteen structural issues in the trust model; Review #8 adjudicated that
those findings are **schema-design work to be done now**, not further architecture work —
with the exception of the genuinely semantic decisions, which must be explicit before any
table is drawn.

This document is that gate. Its job is to convert the accumulated findings into a finite set
of semantic decisions, each with:

```text
Decision
Invariant
Enforcement layer
Required durable fact
Open question
Schema consequence
```

It answers one question directly: **are we now safe to begin schema design?** It does not
design the schema. It does not authorize DDL.

---

## Executive Decision

**Yes — semantic decisions are sufficiently resolved to begin schema design, under
constraints.** The verdict is not based on table count. It is based on the fact that every
Review #7 finding has been converted here into either an explicit semantic decision (with an
assigned enforcement layer and required durable fact) or an explicit OPEN/DEFERRED item with
an owning phase.

The distinction that governs everything downstream:

```text
SCHEMA DESIGN      → AUTHORIZED by this document
DDL                → NOT AUTHORIZED (requires schema design + adversarial schema review)
IMPLEMENTATION     → NOT AUTHORIZED (requires DDL)
```

Conversely, this document does **not** claim the trust model's proposed relational objects are
final. The candidate schema is an output of the durable-fact analysis, and the analysis is
permitted to conclude that one of the historically-assumed objects is unnecessary, or that an
additional object is required. That is the guard against anchoring on the earlier
"five-table minimum."

---

## Trusted Computing Base

**Decision (D2, PROPOSED — the decision is made here; the threat model it describes governs
all other decisions): the Solvent service is inside the MVP trusted computing base.**

| Layer | TCB status | Trusted for | Not trusted for |
|---|---|---|---|
| **CockroachDB** | In TCB | Transactional consistency; making impossible durable states impossible (`promoted_is_debt_free`, `gate`, `live_requires_promoted` — `db/001_schema.sql:30,69,74`) | Interpretation, semantics, identity |
| **Kernel (Go, `crdb.ExecuteTx`)** | In TCB | State transitions, atomicity, serialization of writes (`kernel/kernel.go:54-169`) | Policy, identity, semantic relevance |
| **Solvent service** | **In TCB (MVP decision, D2)** | Interpretation, set-level logic, tuple verification (I-12, DOCUMENTED CONTRACT), policy binding, ingress verification | — (it is trusted; that is what TCB means) |
| **Identity (IdP)** | Out of TCB, trusted for authenticity | Asserting that a principal is who it claims to be | Authorization decisions, Solvent's ledger semantics |
| **Policy authors (tenant)** | Out of TCB | Choosing obligations within Solvent's floor | Exceeding the floor (the floor is Solvent's, not the tenant's) |
| **Executor** | Out of TCB | Faithful presentation of the authorized tuple (I-12 contract); execution effect (I-13, DOCUMENTED CONTRACT, deferred) | Authorization decisions |
| **Evidence sources** | Out of TCB until ingressed | Content of the external world | — (Solvent proves provenance/integrity of what it received, not world truth — D12) |

**What this means, stated plainly:**

- Solvent protects against malformed/untrusted callers, invalid proposals, and invalid
  persistent state.
- Solvent does **NOT** claim to protect against a fully compromised Solvent service. A
  compromised service is kernel compromise. No language anywhere in the corpus may suggest
  otherwise.

**Defense layers that survive a service bug** (defense-in-depth, not TCB expansion):

1. DB impossibility constraints — even a buggy service cannot commit a promoted belief with
   debt, or a live intent on a non-promoted belief (`db/001_schema.sql:30,69`).
2. Identity verification — authenticity comes from the IdP, not from service judgement.
3. Executor contract — the executor re-presents and the service re-verifies the tuple
   immediately before execution (I-12, DOCUMENTED CONTRACT), so a stale service decision
   cannot silently survive retraction.

---

## Decision Register

| ID | Decision | Status | Trust Layer | Durable Fact Required | Schema Consequence |
|---|---|---|---|---|---|
| D1 | Principal = stable actor reference; credential lifecycle owned by external IdP; `principal_type` immutable; credential table deferred | PROPOSED (decision locked here) | Identity + Service | Principal identity, type, revocation | `principal` rows keyed by stable principal id; no credential rows in MVP |
| D2 | Solvent service is inside the MVP TCB | PROPOSED (decision locked here) | Service | — (threat-model statement) | None directly; bounds all other decisions |
| D3 | No cross-tenant authority relationship, ever; composite tenant-aware binding; multi-tenancy deferred | PROPOSED (invariant locked here); implementation DEFERRED | DB + Service | Tenant binding on every authority-bearing row | `tenant_id` columns with composite tenant-aware keys; no `tenant` table in MVP |
| D4 | Active AuthorityTarget is immutable; any tuple change = a new target | PROPOSED; CURRENT ENFORCEMENT = NONE | DB + Service | Immutable target tuple + state | Schema must make mutation of an active target impossible, not merely unaudited |
| D5 | Justification is an authority-bearing assertion; agents propose, only Approve activates | PROPOSED | Service + DB | Justification links bound to belief promotion state | Justification validity must track promoted belief state; no cached validity |
| D6 | Approval binds an immutable semantic snapshot, not a mutable object | PROPOSED; CURRENT ENFORCEMENT = NONE | Service + DB | Approval snapshot (tuple + justification set + policy version + approver) | Snapshot persisted at Approve; later reads must not reinterpret it |
| D7 | Exactly one valid `proposed → active` transition per target snapshot; transactional state-transition serialization, not a uniqueness trick | PROPOSED (requirement locked here); mechanism OPEN, resolved in schema design | Kernel + DB | Single successful activation per snapshot | State transition, precondition validation, and snapshot binding in one transaction; retry via `40001`; fail closed |
| D8 | Obligation identity = (key, version, type, policy context); `debt[]` strings are transitional | PROPOSED | Service + DB | Discharge records bound to stable obligation identity | Ledger rows name obligation identity; versioned keys promoted with policy versioning |
| D9 | Transitional dual truth (`debt[]` + discharge ledger) accepted iff single-transaction atomicity holds; long-term obligation rows → discharge rows → derived debt | PROPOSED (atomicity rule locked here) | Kernel + DB | Accepted discharge and debt-array removal are one transaction, neither surviving without the other | Discharge ledger append-only; duplicate discharge impossible; revocation = counter-entry, never deletion |
| D10 | Policy versions append-only; cross-row monotonicity is transactional enforcement, not a row-local CHECK; downgrade may restrict, never broaden | PROPOSED (invariant locked here) | Kernel + Service + DB | Policy version identity + binding at approval | `current_authority ≤ approval_time_authority`; version hash bound into approval snapshot |
| D11 | Universal policy floor (I-P1/I-P2 prohibitions, DOCUMENTED CONTRACT); `HIGH_IMPACT ⇒ ≥1 attested\|quorum` (I-P3, DOCUMENTED CONTRACT) remains candidate | DOCUMENTED CONTRACT; CURRENT ENFORCEMENT = NOT IMPLEMENTED; I-P3 OPEN | Service | Policy validation refusals | Schema design must not assume any floor is enforced until it is |
| D12 | `server_verified` is a persisted fact asserted by a named trusted process (service ingress), never a caller-settable Boolean | PROPOSED | Service (in TCB) | Ingress-computed hash verification | Verification result persisted by the ingress code path only |
| D13 | Target ≠ AuthorizationDecision ≠ Warrant; MVP embodies the decision in target activation; no warrant or decision table | PROPOSED | Service | Activation state + snapshot as the authoritative assertion | No third/fourth table in MVP; warrant is a portable reference only |
| D14 | I-12 (authorized tuple == presented tuple) preserved and verified before every HIGH_IMPACT execution; I-13 (world effect) deferred | I-12 PROPOSED (contract DOCUMENTED); I-13 DEFERRED | Service + Executor | Pre-execution re-verification | Schema must not claim execution proof; `executed` is caller-asserted today |
| D15 | Lean proves only properties with real executable semantics; no Lean changes now | PROPOSED (discipline locked here) | Lean | — | Future theorems only after corresponding SQL constraints exist |
| D16 | Derive schema from durable facts, never from table names | PROPOSED (method locked here) | All | — | Candidate schema below is an output, revisable by the analysis |

---

## Principal / Identity Decision

**The contradiction (Review #7 blockers 1 and 14).** The trust model said `principal_id` is
the stable identity *and* that rotation creates "a new row with the same `principal_id`". A
stable key cannot identify multiple credential states. Both cannot be true.

**Decision (D1, PROPOSED):**

```text
Principal              = stable actor identity/reference, persisted by Solvent
Credential lifecycle   = owned by the external IdP (issuance, keys, rotation, MFA)
Credential table       = DEFERRED (see Deferred Concepts)
```

- Solvent persists a **stable principal reference** — which principal an authorization belongs
  to — and preserves attribution across credential events. Solvent does **not** become an
  identity/credential-management product.
- **Revocation is a durable Solvent fact.** `principal.revoked_at` is persisted by Solvent
  (the IdP asserts authenticity, not Solvent's authorization-readiness). Any `Approve`
  referencing a revoked principal must be refused. Revocation semantics are a schema
  consequence, not an IdP callback.
- **`principal_type` (`human | agent | workload | service`) is immutable after first
  persistence.** A type change creates a new principal. This is what will make I-A4
  (agent-approval prohibition, DOCUMENTED CONTRACT —
  `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`; trust model doc, v2
  folder) expressible as a relational invariant instead of a service guess. CURRENT
  ENFORCEMENT = NONE (no `principal` table exists).
- **Historical attribution survives rotation.** `approved_by` / `discharged_by` reference the
  stable principal id; credential rotation never rewrites who approved what.
- **What remains owned by the IdP:** how a principal authenticates, key material, rotation,
  MFA, session validity. Solvent receives an authenticated principal assertion and persists
  the identity fact. Solvent does not verify signatures (Identity layer) and does not store
  credentials.

**Resolved:** rotation semantics (no PK multiplicity — one principal row, zero credential
rows in MVP), revocation, historical attribution, type immutability.

**CURRENT/VERIFIED:** no `principal` concept exists anywhere. The MCP `retire_debt` handler
validates only `belief_id`, `scenario` (against a fixed map, `cmd/solvent-mcp/main.go:31-34`),
and `debt_item` enum (`cmd/solvent-mcp/tools.go:104-121`); no caller identity is captured.
`retire_debt` today records *that* an obligation was discharged, never *who* discharged it.

---

## Tenant Boundary Decision

**Decision (D3, PROPOSED; implementation DEFERRED):**

```text
INVARIANT (decision locked here):   No cross-tenant authority relationship, ever.
                               A relationship involving tenant A must never resolve
                               to a principal, resource, evidence, or policy
                               belonging to tenant B.
SCHEMA CONSEQUENCE:            tenant binding is a required relational boundary in
                               the eventual schema, including composite tenant-aware
                               relationships where necessary — e.g.
                               (target.tenant_id, target.principal_id) →
                               (principal.tenant_id, principal_id) — so that a target
                               cannot reference a principal from another tenant
                               through independent columns.
DEFERRED:                      tenant table, tenant authentication, row-level
                               security, per-tenant deployment. Full multi-tenancy
                               is not built now.
```

Independent `tenant_id` columns are explicitly insufficient; the composite binding is the
minimum that makes the invariant structural rather than a service habit.

**CURRENT/VERIFIED (and CONTRADICTED if anyone claims tenancy today):** the system is
single-tenant. `belief`, `evidence`, and `action_intent` carry `scenario_id` only
(`db/001_schema.sql:8,49,62`); `scenario_id` is a fixed UUID map in the MCP server
(`cmd/solvent-mcp/main.go:31-34`), not an authenticated tenant identifier. No `tenant_id`, no
`tenant` table, no tenant authentication exists anywhere in `db/` or Go code. The locked
invariant binds **future** schema design; it describes nothing that exists.

---

## AuthorityTarget Decision

**Decision (D4, PROPOSED):** the five-tuple shape is the semantic unit of authority:

```text
principal | resource | scope | action | consequence
```

with the distinctions the trust model conflated now fixed:

- **Semantic tuple** — what is being authorized (the meaning).
- **Representation** — how the tuple is encoded (namespaces, `{namespace, name}` for action,
  `{type, parameters}` for consequence).
- **Equality** — exact equality on all five dimensions in canonical representation. Two
  targets differing only in scope are different targets. No normalization, no aliasing, no
  wildcard.
- **Interpretation** — mapping consequence → risk tier is policy/service work, not tuple
  structure.

**Immutability rule (PROPOSED; CURRENT ENFORCEMENT = NONE):**

```text
An active (approved) AuthorityTarget is immutable.
Any change in principal, resource, scope, action, or consequence
creates a new target and requires a new authorization.
```

- **Immutability begins at approval.** A `proposed` target that was never approved is
  abandoned, not edited into a different authorization; the replacement is a new target.
- **Target fields are never updated in place once active.** The schema consequence is that
  mutation of an active target must be *structurally impossible* (no UPDATE path at the DB
  level), not merely forbidden by API surface. "Don't expose an update API" is a convention;
  Review #7 correctly rejected it as an invariant.
- **Justification links bind to target identity.** Because an active target cannot change,
  there is no target-side cascade question: if meaning changes, it is a new target with new
  justification requirements, and the old authorization simply is not it.

**CURRENT/VERIFIED:** no `authority_target` table or tuple columns exist. The closest present
analogue is `action_intent.action TEXT` (`db/001_schema.sql:65`) — un-namespaced, one
dimension, no principal/resource/scope/consequence. Migration rules in §Migration Rules.

---

## Justification Decision

**Decision (D5, PROPOSED): the Belief → Target justification link is itself an
authority-bearing assertion.** It is not audit decoration. It is the claim that a specific
belief's truth is relevant to a specific target's consequence, and it is the hinge between
epistemic state and authority state.

- **Agents may propose justifications. Agents may never independently activate them.**
  Activation happens only through `Approve` (the authority-granting transition). This is the
  resolution of the moved-broadening attack: a legitimate promoted belief plus an
  attacker-created target plus an attacker-attached link must still terminate at a human
  approval decision that sees exactly what it is approving (D6).
- **"Belief is promoted" ≠ "belief legitimately justifies this consequence."** Promotion makes
  a belief *eligible* to participate in justification; it says nothing about semantic
  relevance. Solvent's boundary against a semantically bogus link is the approval separation
  plus the policy floor — not semantic judgment by the kernel. This residual (a human can
  approve a link that does not actually support the consequence) is bounded and honest: it is
  recorded in the Self-Attack section, not hidden.
- **Justification validity must track belief state, transactionally.** The `ON UPDATE
  CASCADE` pattern from `gate` propagates a status *value*; it does not by itself mean
  "invalidated authority". The required invariant (PROPOSED, derived here — not yet a
  documented contract anywhere):

```text
active target  ⇒  every justifying belief of its approval snapshot
                  is promoted at approval time,
                  and re-verified (not cached) at every pre-execution check.
```

  A retracted belief must cause the next verification to fail closed. This is a schema
  consequence: justification validity is checked by reading committed state inside the
  verification transaction (the same philosophy as `gate` — the schema detonates the invalid
  state — extended to set-level membership, which is application logic per the frozen
  architecture rules in AGENTS.md).

---

## Approval Snapshot Decision

**Decision (D6, PROPOSED; CURRENT ENFORCEMENT = NONE):** the human approves **this exact
authorization**, not a mutable object whose meaning can change later. The approval binds a
stable semantic snapshot containing at minimum:

```text
target identity (the full five-tuple)
justification set (belief ids + claims + status at approval time)
policy version (identity/hash)
approver principal (stable id + type)
```

Anything else necessary (e.g. obligation set in effect) is a schema-design determination, but
the four above are the floor. Later reads of the approval must re-present the snapshot, never
re-derive it from mutable state. This is what makes the approval meaningful under policy
change (D10) and target immutability (D4), and what gives the approver comprehension the
trust boundary actually requires.

---

## Concurrency Decision

**Decision (D7, requirement PROPOSED; mechanism OPEN — owned by schema design).** Review #7
correctly destroyed the `UNIQUE(target_id) WHERE state='active'` argument: a target already
has a unique id, so that constraint constrains nothing about the `proposed → active` race.
The corrected semantic requirement:

```text
For a given target snapshot, exactly ONE transaction can successfully
complete the proposed → active transition.
```

The eventual schema/transaction design must provide:

- **serialization** of the transition against the committed `proposed` state;
- **atomic precondition validation** — the precondition check (snapshot binding, policy
  version, approver eligibility, justification validity) and the transition happen in one
  transaction, so no check can pass on state that then changes;
- **retry semantics** — `40001` is a retry signal via `crdb.ExecuteTx` (the existing pattern:
  `kernel/kernel.go:59`; B-18, `kernel/kernel_test.go:324-387`), and after retry the
  transaction refuses on fresh state rather than on stale state;
- **fail-closed behaviour** — a loser of the race maps to a review outcome and a re-read,
  never to a silent second authority.

This document requires the semantics and points at the precedent (`crdb.ExecuteTx` + the
`gate` pattern of schema-detonated invalid states); it does not invent a locking
implementation. The same requirements cover `Discharge vs Promotion` and `Approve vs policy
change` (see Concurrency Rules).

---

## Obligation / Debt Decision

**Decision (D8, PROPOSED): an obligation is not "a string in `debt[]`".** Conceptually it
must have identity sufficient to distinguish:

```text
obligation key      (e.g. needOperatorSignoff)
version             (e.g. @v2)
type                (deterministic | attested | quorum — DOCUMENTED CONTRACT,
                     authority-target-and-debt-discharge-spec.md)
policy context      (which policy defined/qualified it)
```

`needOperatorSignoff@v2, type=attested` must not accidentally become equivalent to
`needOperatorSignoff@v3, type=deterministic` under a mutable policy. The minimum stable
identity needed for `DebtDischarge`, replay prevention, policy evolution, and historical
audit is at least `(key, version)` bound to the policy context that defined it; whether
version-qualified keys are required in the first schema, or stable keys plus policy-version
binding suffice, is an OPEN question owned by schema design (see Remaining Open Questions).

**CURRENT/VERIFIED:** `debt` is a `TEXT[]` of six fixed items (`db/001_schema.sql:25-27`); the
vocabulary is a frozen default amended once (`db/004_debt_vocabulary.sql`). Qualification is a
compile-time regex table, etcd-specific by design (`internal/belief/mapping.go:15-33`), with
no runtime configuration and no obligation type. One evidence row can retire multiple debt
items; nothing records *why* an item left the array or *who* retired it.

---

## Debt / Discharge Source of Truth Decision

**Decision (D9, atomicity rule PROPOSED and locked here).** The transitional dual truth —
`belief.debt[]` as the CHECK-facing current state plus an append-only discharge ledger as the
attributable history — is **accepted**, if and only if the following atomicity rule holds:

```text
accepted discharge  +  removal from current debt  =  ONE transaction.
Neither may exist without the other for a completed discharge.
Promotion may never observe a half-discharged state.
```

This directly repairs the CURRENT gap: `RetireDebt` is `array_remove` in its own transaction
(`kernel/kernel.go:83-84`, `kernel/sql.go:23`) and `Promote` is a separate transaction
(`kernel/kernel.go:95`) — and the ledger side of that story does not exist at all, so
`promoted_is_debt_free` (`db/001_schema.sql:30`) checks an array with no attributable history.
The N1/N2 promotion conditions referenced across the review chain are DOCUMENTED CONTRACT
(authority-target-and-debt-discharge-spec.md), not repository-verified invariants; N2 in
particular cannot be verified without the discharge ledger this decision requires.

Additional required semantics:

- **Duplicate discharge:** impossible — one discharge per (belief, obligation) identity; the
  ledger enforces it, not habit.
- **Concurrent discharge:** serialized by the transaction; the loser sees the winner's
  committed state and becomes a no-op or a retry, never a double removal with one receipt.
- **Cross-belief replay:** prevented. CURRENT/VERIFIED: `evidenceExists` dedups per-belief
  only (`internal/belief/belief.go:103`), so the same artifact can discharge the same
  obligation across different beliefs today. The ledger's uniqueness scope is a schema-design
  determination (per-belief vs per-obligation-global) — OPEN, owned by schema design.
- **Discharge revocation:** a counter-entry in the append-only ledger (which re-opens the
  debt), never deletion of history.
- **Policy invalidation:** a policy change may re-open debt prospectively; it must never
  retroactively un-promote a validly promoted belief except through reassessment/retraction.

**Long-term direction (FUTURE/UNSCOPED, not MVP):** obligation rows → discharge rows →
derived outstanding debt, at which point `debt[]` becomes a derived value and the dual truth
disappears.

---

## Policy Version Decision

**Decision (D10, invariant PROPOSED and locked here).** Review #7 is accepted verbatim:
**cross-row monotonicity is not a row-local CHECK.** `CHECK (effective_at > previous)` cannot
exist as a row-local constraint. The correct classification:

```text
append-only policy versions
+ transactional ordering (creation rule enforced in the write transaction)
+ service verification
```

- **Version identity:** each version carries a stable identity (content hash) so approvals
  can bind to it (D6) and audits can re-derive exactly which rules were in effect.
- **Immutability:** committed versions never change; a policy change is a new version.
- **The critical invariant (PROPOSED, locked here):**

```text
current effective authority  ≤  authority granted under approval-time policy.
```

  A later weaker policy may **restrict, revoke, or require re-approval**; it must **never
  broaden** an already-granted authority. A later stronger policy does not silently alter the
  historical meaning of an old approval — the approval snapshot (D6) preserves what was
  actually approved.

**CURRENT/VERIFIED:** no `policy_version` object exists in any form. Policy is implicit in the
compile-time debt mapping (`internal/belief/mapping.go`). Nothing is version-bound.

---

## Policy Floor Decision

**Decision (D11).** The universal semantic floor is carried as **DOCUMENTED CONTRACT** (I-P1,
I-P2 — `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`) with **CURRENT
ENFORCEMENT = NOT IMPLEMENTED**. These are not currently enforced and must not be narrated as
if they were:

```text
DOCUMENTED CONTRACT                       CURRENT ENFORCEMENT
no empty obligation sets (I-P1)           NOT IMPLEMENTED
no self-satisfying obligations (I-P1)     NOT IMPLEMENTED
no "any evidence qualifies" (I-P1)        NOT IMPLEMENTED
agent text cannot count as human          NOT IMPLEMENTED
  attestation (I-P1)
deterministic cannot rely on unverified   NOT IMPLEMENTED
  external_feed alone (I-P2)
quorum cannot count duplicate             NOT IMPLEMENTED
  artifact/principal/model outputs
  as independent sources (I-P2)
```

The stronger candidate posture — `HIGH_IMPACT ⇒ ≥1 attested or quorum` (I-P3, DOCUMENTED
CONTRACT) — remains **OPEN**: there is no concrete customer or use-case evidence for it yet,
and this document does not manufacture any.

The discipline Review #7 demanded is adopted: prose distinguishes **DOCUMENTED CONTRACT**
from **CURRENT ENFORCEMENT**. A coherent design is not evidence of enforcement.

---

## Evidence Verification Decision

**Decision (D12, PROPOSED).** `server_verified = true` must never be a stored Boolean that
anyone can set. The property is: **what trusted process is entitled to assert that the hash
was independently computed?**

- The trusted process is the **Solvent service ingress** — which is legitimate precisely
  because the service is in the TCB (D2), and consistent with ADR Decision 6
  (`docs/ADR/0001-authority-and-attestation.md:265`: the evidence gateway is part of the
  TCB).
- The ingress computes the hash over the received bytes at the verification point, and the
  **result is persisted by the ingress code path only**. No caller-supplied flag can produce
  a verified fact.
- Distinguished, not conflated: **evidence content** (what was received), **artifact
  identity** (its hash), **provenance** (where it came from, `provenance_class`),
  **citation** (what a belief/discharge names), **source truth** (what the world actually is
  — forever outside Solvent's guarantee).

```text
Solvent proves: provenance / integrity of what it received.
Solvent never proves: truth of the external world.
```

**CURRENT/VERIFIED:** `content_sha256` is caller-supplied and passed through
(`kernel/kernel.go:73`); no server-side verification exists (I-11, DOCUMENTED CONTRACT,
deferred).

---

## AuthorizationDecision / Warrant Decision

**Decision (D13, PROPOSED).** The three concepts are distinct and remain distinct:

```text
AuthorityTarget         = candidate authorization subject ("what could be authorized")
AuthorizationDecision   = the authoritative assertion that the target IS authorized
Warrant                 = portable reference to that decision ("go check, don't trust me")
```

- **MVP embodiment:** the AuthorizationDecision is embodied in the target's activation plus
  its immutable approval snapshot (D4 + D6). No separate `authorization_decision` table in
  MVP — the state transition and snapshot together *are* the authoritative assertion.
- **No Warrant table.** Warrant is a portable reference (ADR Decision 1, central
  verification, `docs/ADR/0001-authority-and-attestation.md:95`; ADR Decision 4, semantic
  truth vs protocol representation, `:232`). The key requirement is unchanged:
  **a Warrant can never become a second authority source.** Its validity is re-verified
  against the ledger at use time, every time.
- Both the Warrant table and the AuthorizationDecision table are DEFERRED with explicit
  promotion conditions (see Deferred Concepts).

---

## Execution Boundary Decision

**Decision (D14).** I-12 and I-13 are DOCUMENTED CONTRACT
(authority-target-and-debt-discharge-spec.md, Revision 1):

```text
I-12 AuthorizationTargetMatch   PROPOSED (contract DOCUMENTED; CURRENT: NONE)
  Solvent verifies: authorized tuple == presented tuple, by exact equality,
  server-side, before every HIGH_IMPACT execution. `unavailable` is a distinct,
  typed outcome from `DENY` (fail closed per ADR Decision 2, :113).

I-13 ExecutionEffectMatch       DEFERRED
  presented tuple == actual world effect. Requires trusted-executor receipts;
  out of scope. The schema design must NOT accidentally claim execution proof.
```

**CURRENT/VERIFIED:** `action_intent.state='executed'` is a caller-asserted string
(`db/001_schema.sql:66`), not a proof of anything. The schema design must treat it as a
claim, never as evidence.

---

## Formal / Lean Decision

**Decision (D15, discipline PROPOSED and locked here): no Lean changes in this phase.** Lean
proves only properties with real executable semantics, and never runs ahead of the SQL that
would enforce them.

**CURRENT/VERIFIED Lean scope** (`formal/lean/Solvent/`): abstract state-machine preservation
for the four kernel transitions — promotion validity, live-intent-implies-promoted, retraction
preservation, and authority-impossibility theorems (`formal/lean/Solvent/Preservation.lean`,
`Invariants.lean`, `Examples.lean`).

**Future Lean candidates** (FUTURE/UNSCOPED; each only after its corresponding SQL
constraints exist): target activation validity; justification snapshot validity;
discharge/promotion coherence; target immutability, if representable; stale-authorization
refusal.

---

## Minimum Durable Facts

Derived per D16: for each property — what is the semantic fact, is it durable, who is
authoritative, must a violation be impossible in committed state, and what minimum
representation does that require.

| # | Fact | Security property | Why durable | Enforcement layer | Current evidence | Open debt |
|---|---|---|---|---|---|---|
| F1 | Principal identity, type, revocation | Authority is attributable; agents cannot approve HIGH_IMPACT (I-A4 becomes relational) | Approvals must remain auditable and re-examinable after credential rotation | Identity (authenticity) + Service (persistence) + DB (type immutability, revocation refusal) | None — no principal object exists (`cmd/solvent-mcp/tools.go:104`) | Whole object |
| F2 | Immutable target tuple + activation state | Authority cannot be mutated after grant; target ≠ belief | A mutated active authorization is indistinguishable from a new one without durability | DB (no UPDATE path for active) + Kernel (transition) + Service (tuple verification) | None — `action_intent.action TEXT` only (`db/001_schema.sql:65`) | Whole object |
| F3 | Justification links bound to belief promotion state | Authority does not survive belief retraction | The epistemic→authority hinge must be checkable at every use | Service (set-level check) + DB (belief status) | `gate` FK pattern as precedent (`db/001_schema.sql:74`) | Justification object |
| F4 | Discharge records bound to obligation identity | Debt-free claims are attributable and replay-proof | `promoted_is_debt_free` currently checks an array with no history | Kernel (single-tx rule) + DB (ledger uniqueness) | `array_remove`, no ledger (`kernel/sql.go:23`) | Whole object |
| F5 | Policy version identity + approval binding | Downgrade cannot broaden; approvals keep their meaning | Historical meaning of an approval must be re-derivable forever | Kernel (transactional ordering) + Service (verification) + DB (append-only) | None — policy is compile-time (`internal/belief/mapping.go`) | Whole object |
| F6 | Approval snapshot | The human approved *this*, not a mutable object | Approver comprehension is the semantic boundary against moved-broadening | Service + DB | None | Snapshot columns/rows at Approve |
| F7 | Tenant binding on authority-bearing rows | No cross-tenant authority relationship | Cross-tenant authority must be structurally impossible, not checked | DB (composite keys) + Service | `scenario_id` only, fixed map (`cmd/solvent-mcp/main.go:31`) | Columns + composite bindings |

The F1–F7 numbering is internal to this document's derivation. These are **required durable
facts**, not invariants — the invariant IDs remain earned by schema and tests, which do not
exist yet.

---

## Candidate Minimum Schema

**CANDIDATE — NOT DDL.** Derived from F1–F7, not assumed from the earlier five-object list.
If the schema-design phase finds one of these unnecessary or one missing, the finding
overrides this list.

| Object | Purpose | Required invariant | Dependencies | Why it cannot be deferred | What it does NOT represent |
|---|---|---|---|---|---|
| `principal` (F1) | Stable actor identity + type + revocation | Stable PK; `principal_type` immutable; revoked principals refuse new approvals | IdP assertion (external) | Without it, I-A4 is unauditable guesswork and authority is unattributable | Credentials, keys, authentication material |
| `authority_target` (F2) | The five-tuple authorization subject + activation state + approval snapshot (F6 lives here) | Active target immutable (no UPDATE path); one `proposed → active` transition per snapshot | `principal`, `policy_version` | Without it there is no authority object at all — only un-gated action strings | Beliefs, epistemic state, execution effects |
| `justification` (F3) | Belief ↔ target links, proposed vs accepted | Active target's justifying beliefs promoted at approval; validity re-checked at use, never cached | `belief` (existing), `authority_target` | The moved-broadening attack lands exactly here; without the link, belief and authority are independent | Semantic relevance (that is the approver's judgment) |
| `debt_discharge` (F4) | Append-only attributable discharge history | Single-tx with debt removal; duplicate discharge impossible; revocation = counter-entry | `belief` (existing), `principal`, obligation identity | `promoted_is_debt_free` without a ledger is an unauditable array mutation | Obligation definitions (deferred), evidence truth |
| `policy_version` (F5) | Append-only policy identity + ordering | Append-only; transactional cross-row ordering; version hash bound into approvals | Service verification | Without it, downgrade broadening is undetectable and approvals lose meaning | Policy interpretation logic (service layer) |

Plus: `tenant_id` columns on all of the above (F7) with composite tenant-aware keys where a
relationship crosses objects — **not** a `tenant` table.

Note the derivation outcomes versus the earlier assumption: the approval snapshot (F6) is
carried by `authority_target` rather than becoming a sixth table; tenant is columns, not a
table; and the obligation object remains deferred because F4's ledger can bind obligation
identity as stable text + policy context in the transitional model (D8). This is what
"derive, don't assume" produces — it happens to land near five, but for stated reasons.

---

## Deferred Concepts

Each item is intentionally outside the current implementation boundary. The promotion
condition is the contract: when it fires, the concept returns to design.

| Concept | Why deferred | Promotion condition |
|---|---|---|
| Credential / IdentityBinding table | D1: credential lifecycle is IdP-owned; Solvent persists only the stable principal reference | Solvent must make an authorization decision that depends on authentication state (e.g. credential-revocation windows binding approvals) |
| Obligation table | D8/D9: the transitional `debt[]` + ledger model is safe under the single-tx rule; obligation identity rides as stable keys + policy context | Policy versioning lands and version-qualified obligation keys (D8) become load-bearing; or derived-debt migration begins |
| Warrant table | D13: warrant is a portable reference; central verification (ADR Decision 1) needs no durable warrant row | Offline/bearer verification, delegation chains, or multi-use warrants require durable warrant state |
| AuthorizationDecision table | D13: activation state + immutable snapshot *is* the decision in MVP | Decisions must outlive targets, be revoked independently, or be composed (delegation, attenuation) |
| Tenant table + tenant auth + RLS | D3: the invariant is locked and columns are reserved; the deployment is single-tenant today | First multi-tenant deployment; tenant authentication exists |
| EvidenceArtifact / Citation objects | D12: current `evidence` rows carry content, provenance, and citation roles adequately for MVP | One artifact must be cited by many beliefs/discharges with independent lifecycle (replay-scope control, artifact revocation) |
| ExecutionReceipt (I-13) | D14: execution-effect proof requires a trusted executor and receipts; explicitly out of scope | A trusted executor integration with signed post-execution receipts exists |

Also outside this boundary (FUTURE/UNSCOPED, recorded honestly — deferred, not solved):
temporal authority / validity horizons (`docs/ADR/0001-authority-and-attestation.md:543`
defers them; reassessment semantics may restrict/revoke/quarantine, but freshness machinery is
future work); A2A protocol exposure; authority composition / consequence-chain analysis
(future architectural capability); capability containment is carried as a core property of the
tuple+snapshot design but its cross-target propagation analysis is future work.

---

## Cross-Layer Invariant Register

Statuses inside cells: **Enforced** = CURRENT/VERIFIED; **Required** = PROPOSED (locked here
or DOCUMENTED CONTRACT, unimplemented); "—" = not that layer's responsibility.

| Invariant | Semantic meaning | DB | Kernel | Service | Identity | Policy | Executor | Lean |
|---|---|---|---|---|---|---|---|---|
| `promoted_is_debt_free` | Promoted belief carries no debt | **Enforced** (`db/001_schema.sql:30`) | Retires debt atomically | Policy floor quality (D11, unimplemented) | — | Qualifies instruments | — | Promotion validity (current) |
| `live_requires_promoted` | Live intent cites a promoted belief | **Enforced** (`db/001_schema.sql:69`) | Cancel-before-retract | — | — | — | — | live_intent_implies_promoted (current) |
| `gate` | Composite FK detonates live-on-non-promoted | **Enforced** (`db/001_schema.sql:74`) | `crdb.ExecuteTx` serialization | — | — | — | — | Retraction preservation (current) |
| Target immutability | Active target tuple never mutated | **Required**: no UPDATE path (unimplemented) | Transition-only writes | Never reinterprets | — | — | Presents tuple exactly | Future candidate |
| Justification validity | Active authority ⇒ justifying beliefs promoted, checked at use | Belief status (exists) | — | Set-level check per verification | — | — | — | Future candidate |
| Approval separation (I-A4, DOCUMENTED CONTRACT) | Agents never approve HIGH_IMPACT | `principal_type` immutability (future) | — | Enforce at Approve | Asserts type authenticity | — | — | Future candidate |
| Tenant consistency | No cross-tenant authority relationship | Composite keys (future, D3) | — | — | Binds tenant in assertion | — | — | — |
| Policy downgrade monotonicity | `current_authority ≤ approval_time_authority` | Append-only versions (future) | Transactional ordering | Verify at verification time | — | Owns the rules | — | Future candidate |
| Discharge uniqueness | One discharge per (belief, obligation); no half-discharge | Ledger uniqueness (future) | Single-tx rule (D9) | — | — | — | — | Future candidate |
| I-12 (DOCUMENTED CONTRACT) | authorized tuple == presented tuple | — | — | Verify before every HIGH_IMPACT execution | — | — | Present tuple exactly; re-present | — |
| I-13 (DOCUMENTED CONTRACT) | world effect == authorized consequence | — | — | — | — | — | Trusted executor receipts | — |

---

## Concurrency Rules

Semantic requirements only; final mechanisms belong to schema design (D7).

1. **`proposed → active`:** exactly one successful transition per target snapshot;
   precondition validation + transition + snapshot binding in one transaction; `40001` retry
   then refuse on fresh state; loser gets a review outcome, never a second authority.
2. **Discharge vs Promotion:** accepted discharge + debt removal in one transaction (D9);
   promotion never observes a half-discharged state.
3. **Target mutation vs Approve:** the approval binds the snapshot; any concurrent change
   makes it a different target — the approval fails and the caller re-reads.
4. **Policy change vs Approve:** append-only versions; the approval binds a version identity;
   a version landing mid-approval is either the bound one or the approval retries — it is
   never silently reinterpreted.
5. **Verification vs Retraction:** pre-execution verification reads committed state inside
   the verification transaction; retraction wins → next verification is `DENY`. Stale
   `ALLOW` must not outlive retraction; verification is before **every** HIGH_IMPACT
   execution and is never cached.
6. **Execute vs Retraction:** the executor's present → verify → execute sequence
   re-verifies immediately before execution; the residual gap between verify and execute is
   what deferred I-13 receipts would close (recorded, not hidden).

---

## Migration Rules

The invariant that survives every migration:

```text
missing new authority dimensions  →  NOT AUTHORIZED
NULL / missing                    →  never a wildcard
```

- Old `action_intent.action TEXT` rows do not become wildcard authority. An intent lacking
  principal/resource/scope/consequence is not authorizable under the new model — fail closed.
- Old rows never receive inferred resources, invented scopes, or defaulted principals.
- No rollback path may resurrect authority that the forward path revoked.
- Mixed old/new semantics must not coexist in the authority path: the new model applies to
  anything that claims authority; legacy intents remain demo-scope artifacts.
- Consistent with the additive-only discipline (`docs/POST_HACKATHON_ARCHITECTURE.md` §4.1:
  new migration files, never modify `db/001_schema.sql`; additive columns, tables, CHECKs).

---

## DocTrust Readiness

**What the schema must expose for DocTrust** (through MCP/REST only — DocTrust must not share
the kernel): target lifecycle (`proposed`/`active` + snapshot), justification links with
belief status, discharge ledger with attribution, policy version identity, and refusal
outcomes that name their true source (SQLSTATE verbatim where engine-raised; application
refusals unattributed).

**What DocTrust must test** (extending the four-verb slice `Connect → Ask → Authorize →
Reassess`, `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh`): the four-role separation
(create / attach / request / approve by distinct principals); retraction → next authorize is
`DENY`; policy downgrade → existing authority not broadened; tuple mismatch (presented ≠
authorized) → refusal; revoked principal → approval refused.

**What DocTrust cannot prove:** domain-agnostic generality (single document domain);
semantic relevance of justifications; I-13 execution effect; multi-tenant isolation (it is
single-tenant); that the C-1 model is production-ready. DocTrust is Reference
Implementation #1, not proof of correctness (ADR Decision 8,
`docs/ADR/0001-authority-and-attestation.md:313`).

---

## Critical Self-Attack

Per the gate discipline: each composition is attacked, and the precise boundary that stops it
is named — or the failure is recorded with its owning layer.

**1. Attacker-controlled agent + valid tenant identity + legitimate promoted belief +
legitimate discharge + attacker-created target + attacker-attached justification +
strong-looking legitimate policy + human approver + exact tuple verification → HIGH_IMPACT.**
Every primitive is individually legitimate; the question is which boundary stops manufactured
authority. Answer: **the approval boundary (D5 + D6) is the semantic stop** — the agent can
propose the target and the link but cannot activate; the human approves an immutable snapshot
that names the exact consequence (so the deception cost is comprehension, not structure);
I-A4 keeps the approver human; the policy floor keeps the policy from being vacuous.
**Residual, recorded honestly:** a *deceived but legitimate* human approver is outside
Solvent's structural guarantee — semantic relevance of a justification is judgment, not a
constraint. If an attack succeeds because "the service should do the right thing" beyond this
boundary, it is out of scope by D2 (TCB), not silently absorbed.

**2. Policy downgrade + existing active target → broader effective authority.** Stopped by
D10's locked invariant (`current_authority ≤ approval_time_authority`) + F5 (version binding
in the approval snapshot). A downgrade may restrict, revoke, or force re-approval; it cannot
broaden. CURRENT ENFORCEMENT = NONE — the invariant binds the schema, it does not describe
shipped behaviour.

**3. Identity rotation + historical approval → confused principal.** Stopped by D1: the
approval binds the stable principal id, not credential state; rotation creates no principal
row multiplicity; attribution survives. Residual: a *revoked* principal's historical approvals
remain historical fact (attribution) — revocation gates new approvals, it does not rewrite
the ledger.

**4. Tenant A evidence → tenant B target.** Will be stopped by D3's composite tenant-aware
binding. **CURRENT: UNENFORCED** — nothing stops it today because neither tenants nor targets
exist; the invariant is a schema-design requirement, and the gap is open until that schema
lands.

**5. `proposed` + concurrent approvals → double authority.** Stopped by D7: exactly one
successful transition per snapshot; loser fails closed into review. The
`UNIQUE(target_id) WHERE state='active'` argument is rejected as the mechanism.

**6. Belief retract → old justification → stale warrant.** Stopped by D5 + Concurrency Rule
5: justification validity is re-read at every pre-execution verification, never cached; the
`gate` pattern is the precedent for schema-detonated invalid states. The verify→execute
residual is I-13's recorded deferral.

---

## Dogfood Evidence Ledger

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Four frozen tables with `promoted_is_debt_free` / `gate` / `live_requires_promoted` DB-enforced | `db/001_schema.sql:30,69,74,34` | High | — | Discharged |
| RetireDebt is `array_remove` in its own transaction, separate from Promote | `kernel/kernel.go:83-95`, `kernel/sql.go:23` | High | Transitional dual truth has no ledger side (D9) | Discharged (as current state) |
| Evidence dedup is per-belief only; cross-belief replay unprevented | `internal/belief/belief.go:103` | High | Ledger uniqueness scope (schema design) | Discharged (as current state) |
| Debt qualification is compile-time regex, etcd-specific, no obligation identity | `internal/belief/mapping.go:15-33` | High | Obligation object deferred (D8) | Discharged (as current state) |
| MCP captures no principal identity; scenarios are a fixed local map | `cmd/solvent-mcp/tools.go:104-121`, `cmd/solvent-mcp/main.go:31-34` | High | Principal object (D1) | Discharged (as current state) |
| No principal / authority_target / justification / debt_discharge / policy_version / tenant_id exists in db/ or Go | `db/001_schema.sql` (all), `db/002_corpus.sql:25,76`, `db/003_wizard.sql:50`, repo grep | High | This is the gap this artifact gates | Discharged |
| `content_sha256` is caller-supplied pass-through | `kernel/kernel.go:73` | High | Ingress verification (D12; I-11 deferred) | Discharged (as current state) |
| `executed` intent state is caller-asserted, not proof | `db/001_schema.sql:66` | High | I-13 deferred (D14) | Discharged (as current state) |
| I-P1/I-P2 floor: DOCUMENTED CONTRACT, CURRENT ENFORCEMENT = NOT IMPLEMENTED | No policy validation code exists; both `docs/ARCHITECTURE/` specs are PROPOSED — NOT IMPLEMENTED | High | Policy validation implementation | Discharged (wording corrected) |
| Lean covers the four current transitions only | `formal/lean/Solvent/Preservation.lean`, `Invariants.lean`, `Examples.lean` | High | Future theorems gated on SQL (D15) | Discharged |
| Four-verb slice is legible and exercised | `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh` | High | — | Discharged |
| `task test` passes on the current tree | Suite run at validation time: all Go waves + `MCP VERIFY GREEN` (7 tools); requires local CRDB via `scripts/m0_up.sh` and fixture-seeded `fable` database (repo's own CLI) — no test counts hardcoded, they go stale silently | Medium | Suite depends on local infrastructure being up and seeded | Discharged (as repository-health observation, not as evidence this design is correct) |
| The candidate five-object schema is sufficient | Derived in this document from F1–F7 | — | Schema design + adversarial schema review may revise it | Open |
| Single-tx discharge rule suffices for the transitional model | Design decision D9 | — | No implementation exists | Open |
| Approval snapshot prevents moved-broadening | Design reasoning D5/D6 — coherent reasoning is not evidence | — | Implementation + adversarial schema review | Open |
| The trust-model spec and review files are external to the repo | `~/Desktop/biz/solvent/v2/` vs repo `docs/ARCHITECTURE/` listing | High | Where the corpus of record lives | Open |
| DocTrust proves domain-agnostic correctness | — | — | — | Contradicted (single domain, single tenant) |
| `UNIQUE(target_id) WHERE state='active'` serializes approval | — | — | — | Contradicted (Review #7; D7 replaces it) |

---

## Remaining Open Questions

Genuinely unresolved semantics that affect the current design — owned by schema design, and
therefore not gate-blockers, but they must be resolved *inside* the schema-design phase, not
after it:

1. **Scope semantics (D4):** what operations must scope support (equality only? containment?)
   to make "two targets differing only in scope are different targets" implementable, and
   what representation makes that decidable?
2. **Obligation identity precision (D8):** for the first schema, are version-qualified
   obligation keys required, or do stable keys + `policy_version` binding suffice? (D9's
   ledger uniqueness depends on the answer.)
3. **Discharge ledger uniqueness scope (D9):** per (belief, obligation) only, or global per
   obligation (cross-belief replay prevention at the ledger level)?
4. **Consequence → risk-tier mapping ownership (D4/D11):** the mapping is service-layer, but
   does the floor need to constrain it relationally to prevent a tenant classifying every
   consequence LOW_RISK, or is the floor sufficient?
5. **Justification link state (D5):** do links carry their own state (proposed/accepted), or
   is a link's acceptance embodied entirely in the approval snapshot? Affects whether
   AttachJustification needs durable per-link state.

Explicitly **not** open (FUTURE/UNSCOPED, with promotion conditions in §Deferred Concepts):
temporal authority, I-13 receipts, A2A, authority composition/consequence-chain analysis,
credential storage, tenant implementation. None of these block schema design; none is claimed
solved.

---

## Schema Design Gate

**SAFE TO BEGIN SCHEMA DESIGN — WITH CONSTRAINTS.**

Schema design is authorized; schema implementation (DDL) is not. Unresolved items remain
explicitly OPEN or DEFERRED — none is claimed solved — and the schema must be constrained so
that future work cannot contradict the locked semantics.

The schema-design phase must honor, at minimum:

1. Principal identity ≠ credential lifecycle (D1).
2. Tenant consistency must be relationally enforceable — composite tenant-aware binding (D3).
3. Belief ≠ authority (D4/D5).
4. Promotion ≠ permission — promotion grants justification eligibility only (D5).
5. Target ≠ AuthorizationDecision ≠ Warrant (D13).
6. Active target is immutable; tuple change = new target (D4).
7. Justification validity tracks promoted belief state, verified at use, never cached (D5).
8. Discharge and promotion are transactionally coherent — the single-tx rule (D9).
9. Policy downgrade cannot broaden existing authority (D10).
10. An agent cannot approve HIGH_IMPACT (I-A4, DOCUMENTED CONTRACT; D1 makes it relational).
11. I-12 ≠ I-13; the schema claims no execution proof (D14).
12. NULL/missing authority dimensions never mean wildcard (Migration Rules).
13. Capability containment is a core property of tuple + snapshot design.
14. Authority composition / consequence-chain analysis remains a future capability — the
    schema must not foreclose it but must not build it.
15. Reassessment may restrict, revoke, or quarantine — never silently broaden (D10/D14).

And the standing discipline: every CURRENT/VERIFIED claim cites repository truth; invariant
IDs are earned, not assigned; every future claim is PROPOSED / FUTURE-UNSCOPED / OPEN /
DEFERRED; the candidate schema is CANDIDATE — NOT DDL; coherent reasoning is not evidence.

---

## Next Step

The smallest next artifact is the **actual schema design** for the candidate objects in this
document (`principal`, `authority_target` with its approval snapshot, `justification`,
`debt_discharge`, `policy_version`, plus tenant columns and composite keys), resolving the
five OPEN questions in §Remaining Open Questions as it goes.

Sequence:

```text
Schema Readiness Decision (this document — AUTHORIZED)
        ↓
Actual schema design document
        ↓
Adversarial schema review
        ↓
DDL
        ↓
Kernel implementation
        ↓
DocTrust reference integration
```

Not: this document → DDL. No further broad architecture review precedes the schema design.
