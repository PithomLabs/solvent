# Kernel Enforcement Boundary and Trust Model

## Status

**PROPOSED — NOT IMPLEMENTED**

This is a design specification, not a description of shipped behaviour. It establishes the **trust boundaries** and **minimum enforcement layers** for Solvent's production authority model. Nothing in this document has been implemented. No table, column, constraint, kernel function, MCP tool, REST endpoint, Lean theorem, Warrant, Attestation, or migration described here exists.

- Every claim about what exists **today** is labelled **CURRENT** and cites repository truth (`file:line`). Where a cited document is itself unimplemented future design (e.g. `docs/ADR/0001` decisions, `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md` `PROPOSED`), the citation is to the *decision*, not to behaviour.
- Every claim about what is **proposed** is labelled **PROPOSED** or **OPEN**.
- Every detail that survives only on reasoning, with no repository or use-case evidence, is **OPEN** and remains debt until Adversarial Review #3 is resolved.

This specification is itself treated as a belief: its **Self-Attack Results** and **Dogfood Evidence Ledger** mark nothing `Discharged` without repository-verifiable evidence. It must survive review before any `AuthorityTarget`/`DebtDischarge`/`policy_version`/`principal` schema, kernel change, or DocTrust integration may begin (`## Implementation Gate`).

*Locked inputs for this spec (per synthesis):* `scope` is an **open constraint set** whose semantic rule is locked (`scope may narrow but never broaden resource/consequence`), `principal + resource + scope + action + consequence` remains the `AuthorityTarget` identity, bootstrap is **human-only tenant-admin via deployment/provisioning** (mechanism `PROPOSED`, existence locked), universal policy negatives are **locked**, candidate `HIGH_IMPACT ⇒ ≥1 attested|quorum` stays `OPEN`.

*Repository truth wins over documents. When a proposed document says something exists but the repository shows it does not, it is classified `PROPOSED / NOT IMPLEMENTED / CONTRADICTED` — not silently reconciled.*

---

## Purpose

The production MVP plan (`mvp_plan2.md:1` 1386 lines) → Adversarial Review #1 (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md` `APPROVE WITH CONDITIONS`) → `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md` (22 sections, `PROPOSED`, F-01/F-04/F-06) → Adversarial Review #2 (`docs/REVIEWS/AUTHORITY_SPEC_ADVERSARIAL_REVIEW_2.md` + `plan3_imp_review.md` `APPROVE WITH CONDITIONS` `C-1..C-4`) → `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md` (`PROPOSED`, C-1 creation/approval + C-3 floor, human-bootstrap `PROPOSED` but existence locked) and finally **Adversarial Review #3** (`authority-creation-policy-floor-adversarial-review-3.md` verdict `REJECT / REDESIGN` + `review4.md:1` correction) together exposed a single architectural fork:

> Which security properties must be **impossible in committed database state**, which belong in the **Solvent service** (interpretation / transaction boundary), which belong to **external identity**, which belong to **customer policy**, and which belong to the **executor / external system**?

Review #3 correctly said **too many new boundaries were prose-only** (`I-A1..I-A4`, `I-P1..I-P4`, justification, target, discharge, policy version all `service` with `no DB column` `db/001_schema.sql:60-76` has no `principal`/`resource`/`consequence`). Review #4 correctly said **not everything belongs in SQL** — `human vs agent` cannot become a `CHECK`, executor tuple equality is naturally service, policy stays service — and that jumping straight to six DDL tables would promote implementation before semantics.

This document **adjudicates** that disagreement **property-by-property**, defines the trust boundaries between eight participants, and derives the **smallest durable state** that must exist for unsafe authority to be *impossible* rather than *checked by the service*. It is the bridge from conceptual `AuthorityTarget` to the next *reviewable* schema design.

---

## Executive Decision

*This section must answer 8 direct questions. Answers are `PROPOSED` and adversarially challenged in `## Self-Attack Results` and `## Dogfood Evidence Ledger`.*

**1. What is the smallest trusted core of Solvent?**

```
CockroachDB (with 3 current + 2 proposed row-local invariants)
  + transactional kernel (crdb.ExecuteTx, RetractCascade atomicity, discharge+array single-tx)
  + Solvent service (interpretation, AND over justification set, policy versioning, warrant verification)
```

*Not* the trusted core: external identity provider, customer policy documents, evidence sources, executors, Warrant caches. Those are *trusted for a purpose* (see `## Trust Boundary Map`) but outside the core that Solvent itself can make impossible. The core is small on purpose.

**2. What MUST be database-enforced?**

Row-local impossibility, durable, tenant-scoped, FK-bound, or transaction-atomic:

- `promoted ⇒ debt empty ∧ not final_truth` (existing `promoted_is_debt_free` `23514` `db/001_schema.sql:30`)
- `live intent ⇒ promoted` (existing `gate` `23503` + `live_requires_promoted` `23514` `db/001_schema.sql:68-75` with `ON UPDATE CASCADE` re-evaluation)
- Per-link justification: `Justification(belief_id, belief_status)` references `belief(id,status)` with `ON UPDATE CASCADE` (proposed, same shape as `gate`; row-local — existence of the shape is proven by the frozen pattern)
- Per-discharge uniqueness on `(belief_id, obligation)` for `accepted` discharges (proposed, row-local `UNIQUE WHERE status='accepted'`)
- Target tuple immutability once `active`: the five-field tuple (`principal, resource, scope, action, consequence`) has no `UPDATE` path — a new target is a new row (proposed, additive-only discipline `docs/POST_HACKATHON_ARCHITECTURE.md:491-503`)
- Policy version immutability: `policy_version(version, effective_at, policy_hash)` is append-only; `effective_at` is monotonic per tenant (`CHECK (effective_at > previous)` via ordering, or `UNIQUE(tenant_id, version)`)
- Tenant isolation FK: every authority-bearing row carries `tenant_id` (future) referencing `principal.tenant_id` — the FK makes cross-tenant authority *impossible*, not merely “checked”

**3. What MUST be service-enforced?**

Interpretation, set-level logic, and transaction boundaries the DB cannot check row-locally:

- `Target valid ⇒ all justification links valid` (set-level `AND` over the justification set — SQL has no row-local `every row of my set satisfies P`)
- `Discharge ↔ array / derived debt` coherence when the transitional `belief.debt[]` + `DebtDischarge` ledger co-exist (single-transaction `RetireDebt + Discharge` — see `## Debt / Discharge Boundary`)
- `CreateTarget` well-formedness, `AttachJustification` qualification (`discharge_policy` match), `RequestAuthorization` AND collection, `Approve` preconditions (`N1∧N2∧N3∧N4` per `authority-target-and-debt-discharge-spec.md:688`), `I-P1/I-P2` universal negatives (empty/self-satisfying/`any evidence qualifies`), candidate `I-P3` flag (`HIGH_IMPACT ⇒ ≥1 attested|quorum` as `predicted_*` until promoted), warrant re-verification under current policy (I-12), and `I-A3` target-immutability enforcement until additive columns exist.

**4. What MUST come from identity?**

Authenticity of *who* is acting. An external **Identity Provider / workload identity system** (IdP, workload attestation) asserts:

- `principal {principal_id, principal_type ∈ {human, agent, workload, service}, issuer, tenant_id}` is genuine
- `principal_type` (the `agent` vs `human` distinction Review #3 says has no `principal_type` column) is *attested* by the IdP, not self-asserted by the caller
- `workload` vs `service account` vs `delegated` masquerading is prevented per issuer
- Attestation `integrity_proof` (payload binds `belief_id + obligation` inside signed payload `docs/ADR/0001:175-178`) is valid

Solvent **persists** `principal_id, principal_type, issuer` (DB) so that later states are *impossible* without that principal (e.g. `Approve` without `human/non-agent` fails), but Solvent **does not prove** `human` — the IdP does. This is the correct split Review #4 insists on.

**5. What MUST remain customer policy?**

Tenant-owned `obligation set` (which debts a belief enters with), `discharge_policy` (which `provenance_class` / actor classes / `quorum N` qualify per `obligation_type` `deterministic|attested|quorum`), `resource` namespace definitions, and risk-tier mapping (`consequence → tier`). Policy is versioned (`policy_version`) and bound at `Approve` (`policy_version_hash` snapshotted). Policy **cannot** define `empty`, `self-satisfying`, or `any evidence qualifies` (I-P1), and `deterministic` cannot accept `external_feed` with caller-supplied hash (I-P2) — those are Solvent's floor, not the customer's choice.

**6. What MUST be enforced by the executor?**

The **executor contract** (C-4, `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:758`):

- Present the **exact** authorized tuple verbatim before execution (no normalization)
- Never mutate `resource/scope/action/consequence` after authorization — any change is a different target requiring new authorization (`I-12 AuthorizationTargetMatch`: `authorized tuple == presented tuple`, server-side equality **before** execution)
- Fail closed on `unavailable` or `DENY` (`HIGH_IMPACT` never executes on `DENY`/`REVIEW`/`unavailable`)
- Never apply one `ALLOW` to a second tuple or second execution (1:1 `Warrant → Execution` for MVP is `OPEN`)

Solvent **cannot** guarantee `presented tuple == actual world effect` (`I-13 ExecutionEffectMatch` — `presented == executed` requires a trusted executor or signed receipt, deferred).

**7. What can never be guaranteed by Solvent?**

- That the executor *actually did* what it presented (I-13, world effect)
- That a human approver **understood** what they approved (attestation proves intent to attest, not comprehension — can be manipulated by AI summary)
- That evidence is **true** (Solvent guarantees provenance and that deterministic evidence was server-verified, not that the external source is correct)
- That the bootstrap tenant admin is **benign** (a compromised bootstrap is a tenant-compromise, not a Solvent bypass — `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:112` — but Solvent documents it)

**8. What is the minimum schema necessary to implement next?**

*Derived from the security properties, not copied from Review #3.* The smallest durable state that makes the production-critical properties **impossible to violate in a committed state** is **5 tables** (your lock #2):

```
principal                 — identity persistence (who exists, of what type/issuer/tenant, revocation)
authority_target          — what is being permitted (principal/resource/scope/action/consequence, state, policy_version_hash, justification snapshot, tuple hash for immutability)
justification             — explicit Belief ↔ Target link (belief_id+belief_status, target_id, obligation_key, state, justification_id)
debt_discharge            — Obligation → Discharge binding (belief_id, obligation_key, instrument_ref, discharged_by→principal, accepted/refused, UNIQUE(belief_id,obligation) WHERE status='accepted')
policy_version            — policy history (version, effective_at, policy_hash, author_principal, tenant_id)
```

Defer as `OPEN`/`DEFERRED` (semantics preserved, not erased): `warrant` as persistent table (warrant stays a **reference/verification**, not a row, until `Tenant Boundary` multi-warrant lifecycle is needed), `obligation` as separate table (its semantics live temporarily in validated `debt[]` + `discharge_policy` JSON on `principal`/`policy_version`), `tenant` as separate table (its boundary lives temporarily as `tenant_id` FK on the 5 tables). This is **minimum, not final** — Review #3's six additional tables remain `OPEN` with a stated condition for promotion (see `## Cross-Layer Invariants`).

---

## Trust Boundary Map

```
                ┌─────────────────────────────────────────────────────────┐
                │                EXTERNAL TRUST ROOTS                     │
                │  External Identity (IdP / workload)   Customer Policy   │
                │  External Evidence Sources                              │
                └───────────────┬──────────────────────────┬──────────────┘
                                │                          │
                                ▼                          ▼
┌──────────┐  untrusted   ┌──────────────┐  untrusted  ┌───────────────┐
│ AI Agent │─────────────▶│     MCP      │────────────▶│ Solvent       │
│          │  proposes,   │   + REST     │  adapters,  │ Service       │
│ (reason) │  cites,      │  (protocol)  │  propose/   │ (interpret,   │
│          │  requests,   │              │  request,   │  AND over set,│
│          │  echoes      │  verify that │  echo,      │  policy ver., │
│          │  tuple       │  tuple not   │  present    │  warrant ver.)│
└──────────┘  normalized  └──────────────┘  mutated    └──────┬────────┘
                                                             │ verifies
                                                             ▼
                                                ┌───────────────────────┐
                                                │   Solvent Kernel      │
                                                │  (transactional      │
                                                │   EnterBelief /       │
                                                │   Promote /           │
                                                │   RetractCascade /    │
                                                │   Discharge+array     │
                                                │   in one tx)          │
                                                └──────────┬────────────┘
                                                           │ enforces
                                                           ▼
                              authoritative state    ┌───────────────┐  derived / not yet authoritative
                              ──────────────────────▶│ CockroachDB   │◀────────────────────────
                              `belief` `debt[]`      │ (CHECK, FK,   │  `solvent_explain` predicted_* //
                              `evidence` (ingest)    │  cascade,     │  warrant cache /
                              `principal` (persist)  │  tx atomicity)│  scope equality (until CHECK)
                              `authority_target`     │  makes states │  world effect (I-13)
                              `justification`        │  *impossible* │  policy interpretation
                              `debt_discharge`       └──────┬────────┘  actual execution
                              `policy_version`               │ re-verifies
                                                             ▼
                                                ┌───────────────────────┐
                                                │      Executor         │
                                                │  (echoes exact tuple,│
                                                │   verifies with      │
                                                │   Solvent, enforces  │
                                                │   fail-closed,       │
                                                │   optionally receipts)│
                                                └──────────┬────────────┘
                                                           │ performs
                                                           ▼
                                                ┌───────────────────────┐
                                                │  Real-World System    │
                                                │  (database, treasury,│
                                                │   DNS, patient record)│
                                                └───────────────────────┘

Legend: `→` = data/control flow; `─` = stores. Shading: double-box = authoritative ledger; single-box = derived/service/executor.
```

**For each boundary:**

| Boundary | Who is trusted | What is asserted | What is verified (and by whom) | What is merely logged |
|---|---|---|---|---|
| **AI Agent → MCP/REST** | Agent (untrusted) | `belief claim`, `evidence text`, `target tuple` proposal, `justification` proposal | Solvent service verifies `obligation ∈ current debt`, `discharge_policy` qualifies instrument, `principal` is authenticated (via Identity) | Agent's paraphrase, `retire_debt` caller identity (current `cmd/solvent-mcp/tools.go:104` only checks enum) |
| **External Evidence → Solvent Service** | Evidence source (untrusted until provenance checked) | `content_sha256`, `provenance_class`, `source_url` | *Not yet*: I-11 server-verified hash, provenance-class enforcement, replay & TOCTOU check (`internal/belief/belief.go:103`) are `OPEN`/`DEFERRED` → currently **merely logged** per `evidence` row; proposed I-11 would verify at ingress | `snapshot JSONB`, `source_observed_at` |
| **External Identity → Solvent Service** | IdP / workload identity (trusted for authenticity, not for authorization) | `principal {id, type∈{human,agent,workload,service}, issuer, tenant_id} is genuine` | Solvent service **verifies** the assertion (signature/issuer) and **persists** it (`principal` row); the `human vs agent` distinction becomes DB-persisted `principal_type` so `I-A4` can be a CHECK later, but the *authenticity* remains IdP | Approver's comprehension, attestation summaries |
| **Customer Policy → Solvent Service** | Tenant policy author (must be `human tenant-admin` via bootstrap `I-A1`, not agent) | `obligation set`, `obligation_type`, `discharge_policy`, `risk_tier → consequence` mapping | Service verifies against **floor** (`I-P1` no empty/self-satisfying/`any evidence qualifies`, `I-P2` `deterministic≠unverified external_feed`) and version-binds (`policy_version` immutable, `effective_at` monotonic) — currently **OPEN** (no `policy_version` table) so merely logged | Tenant's business justification for choosing a permissive-but-still-valid floor |
| **MCP/REST → Solvent Service** | Adapter (trusted for transport, not for semantics) | Canonical `principal/resource/scope/action/consequence` tuple, exact equality | Service verifies **adapter did not normalize** (`docs/REVIEWS/...:62` — no lowercasing/aliasing), `resource` opaque equality, `scope` narrows-only | Adapter's description prose |
| **Solvent Service → Solvent Kernel** | Service (trusted for interpretation) | `EnterBelief`, `RetireDebt + Discharge` in one tx, `Promote` (`N1∧N2∧N3∧N4`), `CreateTarget` `proposed`, `AttachJustification` `proposed`, `Approve → active`, `RetractCascade` | Service calls `kernel` transactions; kernel's `crdb.ExecuteTx` guarantees serialization and the schema's `promoted_is_debt_free`/`gate`/`live_requires_promoted` make live-on-unpromoted **impossible** (existing) | — |
| **Solvent Kernel → CockroachDB** | CockroachDB (trusted for transactional invariants) | `state='promoted' ⇒ debt empty` (`db/001_schema.sql:30` `23514`), `state='live' ⇒ belief_status='promoted'` (`:68` `23514`), `belief_id/status → belief` (`:74` `23503`), `RetractCascade` cancel-before-retract atomicity | DB refuses the write (`40001` retries via `crdb.ExecuteTx`, `kernel/kernel_test.go:327` B-18) — no application pre-check can be forgotten | `approver ≠ requester` for `HIGH_IMPACT` (not row-local, so not yet DB) |
| **Solvent Service → Executor** (authorization) | Executor (untrusted for W, trusted for X) | `Warrant` reference `target_id + justification_snapshot + issued_at + policy_version_hash` is currently valid | **I-12 AuthorizationTargetMatch:** service verifies `authorized tuple == presented tuple` by exact equality **before** execution (server-side, not client-attested); `unavailable` is distinct from `DENY` (typed) | Warrant JSON itself (transport) — not authority until verified |
| **Executor → Real-World System** (execution) | Real-World System (trusted for effect, not for permission) | `world effect == presented tuple` (I-13 `ExecutionEffectMatch`) | **Not yet:** trusted executor / signed receipt required; currently **merely logged** as `action_intent.state='executed'` (`db/001_schema.sql:67`) — a caller-asserted string, not a proof | Actual world effect |
| **External Evidence ↔ Policy** | — (interaction) | `deterministic` qualified by policy | Service verifies `provenance_class` set is restricted, not universal (`I-P2`), and `quorum` counts distinct principals+channels, not `same artifact N times` | — |

**Authoritative vs derived:** authoritative = `belief.status/debt`, `principal` persisted, `authority_target.state=active`, `debt_discharge` `accepted`, `policy_version` rows — all durable and FK-bound. Derived = `solvent_explain` `predicted_*` SQLSTATEs (`internal/view/explain.go:predicted_*`), `human_summary`, `warrant` `ALLOW` cache, `scope` equality in service until `CHECK`. **Untrusted inputs:** every `MCP` argument (`args["belief_id"]`, `action TEXT`), `evidence.content_sha256` (caller-supplied, `kernel/kernel.go:73` pass-through), `policy` JSON (customer-supplied), `Warrant` JSON (executor-supplied).

---

## Authority Lifecycle

```
External Identity ──┐
                    ▼
Evidence → Belief → Obligations/Debt → DebtDischarge → Promotion → AuthorityTarget → Warrant → Execution → Reassessment
    ▲                                                     │  (AND over    │  (portable, │ (echo+verify) │ (contradiction/
    └────────── External Evidence ──────────────────────────┘  justification) │  central)  │  └─► Real-World   │  retraction/
                                                               │ Set, N1∧N2…│  │  verify) │      System       │  expiry/
                                                               └─────────────┘  └──────────┘                  └─► Audit)
                                                              Customer Policy (versioned)
```

Verification points (the only places a state becomes authority):

- **Promotion:** verifies `N1 debt=[] ∧ ¬final_truth` today (`db/001_schema.sql:30`); proposed adds `N2` (every obligation has `accepted` discharge), `N3` (not retracted, no open contradiction), `N4` (policy permits) — service-transactional now, row-local `I-9/I-10` later.
- **Target activation (`Approve`):** verifies the whole `AND` justification set is `promoted`, every required obligation has an `accepted` discharge satisfying its type (`I-P2`), `no open contradiction`, policy `version` in effect, and approver is `human/non-agent ≠ requester` for `HIGH_IMPACT` (`I-A4`) — all checked atomically at `Approve` time.
- **Warrant verification (I-12):** verifies `authorized tuple == presented tuple` plus `target still active`, `justification beliefs still promoted`, `policy_version still in effect` (or current policy still permits), no intervening retraction — **before every** `HIGH_IMPACT` execution, not cached.
- **Reassessment:** verifies freshness — retraction (`RetractCascade`) makes `live_requires_promoted` impossible, policy change is version-bound, temporal `validity_horizon`/`expires_at` is deferred (`docs/ADR/0001:543`).

---

## Enforcement Taxonomy

Heuristic: `impossible → Database/transaction, transition → Kernel/service transaction, interpretation → Service, identity authenticity → Identity provider, enterprise policy → Policy layer + service, actual world effect → Executor, state-machine preservation → Lean`. **Challenged per property:**

| Williamson | If service is buggy but DB is correct, can property be violated? | If DB correct but identity malicious, can property be violated? | If Solvent+identity correct but executor malicious, what remains? |
|---|---|---|---|
| `live intent on retracted belief` | No — `gate`/`live_requires_promoted` under `crdb.ExecuteTx` refuses | No — identity irrelevant | Already impossible; executor cannot resurrect |
| `HIGH_IMPACT approved by agent` (`I-A4`) with only service | **Yes** — service bug lets `agent` be recorded as `human`; DB has no `principal_type` CHECK to catch it | **Yes** — service accepts `principal_type=human` token that IdP never issued for that principal; DB persists the lie | `I-A4` must be split: *authenticity* → IdP, *persistence+CHECK* → DB (`principal_type CHECK ≠ 'agent'` for `HIGH_IMPACT` approval), *role mapping* → service |
| `deterministic discharged by unverified external_feed` (`I-P2`) | **Yes** — service that misclassifies `external_feed` as reproducible passes; DB `provenance_class` is only an enum `CHECK`, not a semantic binding | **Partially** — identity can be correct but evidence provenance is policy content | Requires DB `CHECK` on `(obligation_type, provenance_class)` **or** service‐validated `discharge_policy` that is itself versioned and immutable — hence `policy_version` must be durable |
| `resource=''` treated as wildcard (`F-11`) | **Yes** if fail-open read path converts `NULL → *`; DB `NULL` is `NOT AUTHORIZED` only if service enforces it today | N/A | Must be DB `CHECK (resource IS NOT NULL)` on `active` rows + service `NULL ⇒ NOT AUTHORIZED` — neither exists, so currently service-only |
| `one discharge → one obligation` (I-10) | **Yes** — service could insert two discharges per obligation with same evidence artifact via two citations | No | Needs DB `UNIQUE(belief_id, obligation) WHERE status='accepted'` — future row-local check, not array logic |

---

## Security Property Classification

*`Discharged` in Dogfood means repository-verified; here `Must be impossible?` means the *desired* future guarantee.*

| Property | Security importance | Must be impossible? | DB | Kernel/Service | Identity | Policy | Executor | Lean | Rationale |
|---|---|---|---|---|---|---|---|---|---|
| belief promotion (`entered→promoted`) | CRITICAL | Yes | **DB** `promoted_is_debt_free` `23514` **today** | Kernel `Promote` in `crdb.ExecuteTx` | — | — | — | **Lean** `promote_preserves_validity` | Must be impossible as committed state (row-local) |
| debt-free promotion (`debt=[]`) | CRITICAL | Yes | **DB** `CHECK` | Service validates promotion preconditions `N1∧N2∧N3∧N4` (N2 `accepted` ledger, N3 contradiction, N4 policy) | — | **Policy** supplies obligation set; `Policy` is versioned | — | Lean for abstract `ValidLedger` | Debt-free alone is currently necessary only; with `DebtDischarge` rows it becomes `derived debt = ∅` |
| accepted discharge (`DebtDischarge` `accepted`) | CRITICAL | Yes | **DB** `UNIQUE(belief_id,obligation) WHERE status='accepted'` (proposed) + `FK(belief_id)→belief` `ON UPDATE CASCADE` | **Kernel/Service** single transaction `Remove debt + Discharge` (proposed `N2`) | — | **Policy** qualifies instrument kind | — | — | Prevents one instrument → many obligations without explicit policy |
| discharge attribution (`discharged_by` genuine) | HIGH | Yes (persistence) but **authenticity** elsewhere | **DB** persist `discharged_by→principal` FK + `principal_type CHECK` | Service validates `discharged_by` is the verified attester | **Identity** proves `human vs agent` (IdP-signed `integrity_proof` binds `belief_id+obligation`) | — | — | **Lean** for `discharge → promoted` later | Split: persistence vs authenticity |
| evidence provenance (server-verified hash, replay) | HIGH | Yes for hash verification | **DB** `evidence.content_sha256 NOT NULL` today but **caller-supplied** (`kernel/kernel.go:73`); future `CHECK` on server-verified flag + `UNIQUE(content_sha256, belief_id)` for replay | Service verifies `provenance_class` set is restricted, not universal | — | — | — | Deferred | Must move from `logged` to `verified` at ingress |
| target creation | MEDIUM | Partial | **DB** `AuthorityTarget.state CHECK(proposed/active/revoked)` + `created_by→principal` FK (proposed) | **Service** validates tenant-auth + well-formedness + no wildcard | **Identity** authenticates `principal` | **Policy** restricts which `consequence` each `principal` may propose | — | — | Creation is proposing only (`I-A1`), not authority; least critical to be impossible |
| justification-link creation | CRITICAL | Yes (per-link) | **DB** `Justification(belief_id,belief_status)→belief` composite FK `ON UPDATE CASCADE` (proposed, same shape as `gate`) — per-link half of `I-9` | **Service** validates `obligation∈current debt`, `instrument` qualifies, link is `proposed` | — | — | — | Lean later | Each link must be auditable row; per-link promotion-dependence is row-local |
| authorization request | LOW | No | — | **Service** collects `AND` set, checks each belief `promoted` (transient) | — | — | — | — | Request is a question, not a state |
| approval (`proposed→active`) | CRITICAL | Yes | **DB** `state CHECK(active≠proposed without approved_by)` + `approved_by→principal(principal_type!=agent)` `CHECK` for `HIGH_IMPACT` (proposed `I-A4`) | **Service** atomically checks `N1∧N2∧N3∧N4` + `all links promoted` + `no open contradiction` + `policy_version` | **Identity** provides `principal_type` authenticity | **Policy** `HIGH_IMPACT` approver set | — | Lean later | Authorization-granting transition; must be impossible without valid approval |
| agent cannot approve HIGH_IMPACT (`I-A4`) | CRITICAL | Yes | **DB** `CHECK (approver_principal_type != 'agent')` on `active` rows **if** `consequence ∈ HIGH_IMPACT` (proposed; requires `principal` table) | **Service** enforces `approver ≠ requester` for HIGH_IMPACT | **Identity** proves `human vs agent` (IdP) | **Policy** lists approver set per `consequence` | — | — | Split as `Enforcement Taxonomy` row: persistence vs authenticity |
| target immutability (`I-A3`) | HIGH | Yes | **DB** no `UPDATE` path for `active` tuple in `additive-only` discipline (`docs/POST_HACKATHON_ARCHITECTURE.md:491`) — new target = new row | **Service** refuses mutation, creates new `proposed` | — | — | **Executor** must re-present exact tuple (I-12) | — | Time-of-check/time-of-use |
| justification-set immutability | HIGH | Yes (for `active`) | **DB** `active` target's justification set is FK-bound; no late addition | **Service** treats late addition as new authorization (new warrant) | — | — | — | — | Set-level `AND` ("all links valid ⇒ target valid") remains service — not row-local |
| principal identity (`human/agent/workload/service`) | HIGH | Persistence yes, authenticity no | **DB** `principal(principal_id PK, principal_type CHECK, issuer, tenant_id, revoked_at)` + `FK` to every authority-bearing row | — | **Identity** proves `type` (IdP/workload attestation) | — | — | — | Spec's 4 principal classes have zero DB representation today |
| tenant isolation | HIGH | Yes | **DB** `tenant_id` FK + `CHECK` + RLS candidate on every authority-bearing table (future) | **Service** validates `authenticated tenant == row tenant` | **Identity** authenticates tenant binding | — | — | — | Current `scenario_id` `000…001/002` `main.go:31` is *not* tenant identity |
| resource identity | CRITICAL | Yes | **DB** `AuthorityTarget.resource_type/resource_id` opaque exact equality, no normalization | — | — | — | **Executor** echoes verbatim | — | Same verb different resource is different consequence |
| scope narrowing only | HIGH | Yes (rule) / `OPEN` (mechanics) | **DB** preserved: `unspecified ≠ wildcard` is service rule now; future `CHECK (scope NOT EXPANDED)` if `scope` gains columns | **Service** enforces `scope` never broadens `resource/consequence` + audit | — | **Policy** supplies scope dimensions | — | Open scope vocabulary is the remaining gap |
| action identity | HIGH | Yes | **DB** `action_namespace`/`action_name` exact equality (proposed) vs current `action TEXT` (`db/001_schema.sql:65` un-namespaced) | **Service** refuses silent normalization (no lowercasing) | — | — | — | — | Two systems defining different semantics for same string |
| consequence identity | CRITICAL | Yes | **DB** `{consequence_type, consequence_parameters}` exact equality | — | — | **Policy** constrains allowed types per tenant | **Executor** echoes | — | Consequence ≠ action (`update_record` on `patient/123` vs `cache/temp-123`) |
| policy minimum floor (I-P1 no empty/self-satisfying/`any`) | HIGH | Yes (refusal) | **DB** future `CHECK` on `obligation` rows (`count ≥1`, `discharge_policy` not universal) | **Service** validates policy at write time (now) | — | **Policy** owns set, but *within* floor | — | — | Customer can configure *within* floor, not *below* it |
| policy version binding (I-P4) | HIGH | Yes | **DB** `policy_version(version, effective_at, policy_hash, author_principal→principal)` append-only; `AuthorityTarget.policy_version_hash→policy_version` FK | **Service** snapshots hash at `Approve` | **Identity** `author_principal` must be human admin | **Policy** document may live outside, hash verifiable | — | — | Without it, I-P4 is a hash string with no referent |
| warrant validity (central verification) | HIGH | No (projection) | — (Warrant stays a **reference**, not a table until needed) | **Service** `Warrant reference → central verification against current ledger` (`docs/ADR/0001:95-111`) | — | — | **Executor** re-verifies **before every** execution | Lean `stale_authority_cannot_persist` sketch | Cached `ALLOW` must not outlive retraction; Warrant is *evidence* of authority, not authority |
| warrant replay | MEDIUM | Partial | **DB** `UNIQUE(belief_id, obligation) WHERE status='accepted'` helps indirectly | **Service** replay cache on `attestation_id` + `integrity_proof` binding `(belief_id, obligation)` inside signed payload `docs/ADR/0001:175-178` | **Identity** revocation propagation | — | — | — | Three surfaces: attestation across beliefs/items (payload binding), attestation after revocation (lifecycle), evidence re-ingestion (dedup) |
| warrant concurrency (`Approve vs Approve` on same target) | HIGH | Yes | **DB** `UNIQUE(target_id, state) WHERE state='active'` + `crdb.ExecuteTx` serialization (same as `Promote`) | **Service** `crdb.ExecuteTx` retry on `40001` | — | — | — | — | Must fail closed, not double-activate |
| retraction (explicit `RetractCascade`) | CRITICAL | Yes | **DB** `Withdraw` cancels `live` intents then retracts in `one tx` (`kernel/kernel.go:127`), `WITH RECURSIVE` `UNION` termination (`kernel/sql.go:65`), `_test.go` `RetractCascade` | **Kernel** scenario-scoped, `I-7` guard | — | — | — | Lean `cascade_retraction_cannot_leave_live_intent` |
| freshness (was authorized vs is authorized now) | HIGH | No (verification) | — (`validity_horizon`/`expires_at` deferred `docs/ADR/0001:543`) | **Service** `authorization-time freshness` check + lazy expiration + periodic worker | — | **Policy** validity horizon | **Executor** `authorization-time verification` before every `HIGH_IMPACT` | — | Wall-clock never mutates state alone; verification must happen at use |
| execution-time verification (I-12 AuthorizationTargetMatch) | CRITICAL | No (service boundary) | — (stores tuple) | **Service** `authorized == presented` exact equality **before** execution | — | — | **Executor** presents verbatim, enforces `fail-closed` on `unavailable` | — | Distinguish authorization vs execution |
| actual execution effect (I-13 ExecutionEffectMatch) | HIGH but **deferred** | No | — (requires `receipt` table not yet in MVP) | **Deferred** | — | — | **Executor** *proves* effect (trusted executor / signed receipt) | **Deferred** | Guarantee requires world observability; Solvent only guarantees `presented == authorized` |
| audit history (`refusal_log`, `human_summary`) | MEDIUM | Partial | **DB** `refusal_log` (`db/003_wizard.sql:46`) append-only, `evidence.content_sha256` not-verified, `solvent_explain` `predicted_*` derived | **Service** `audit stream` (immutable enough) | **Identity** `approved_by` attribution | — | **Executor** `receipt` stream (deferred) | — | Solvent's value is its audit, not just its gate |

---

## Root of Authority

### External root vs Solvent-internal authority

```
Solvent-internal authority (proposed, not yet durable)
    AuthorityTarget(proposed) → Attach → Request → Approve → active
         ↑  requires `human/non-agent` approver (I-A4)
         │  verifies `N1∧N2∧N3∧N4` + all links promoted
         │
External root of trust (PROPOSED, outside Solvent)
    Deployment / provisioning
        → human tenant admin(s) (human-only, small, enumerable)
        → policy + authority configuration (obligation set, principal/resource namespaces, HIGH_IMPACT approver set)
        → Solvent service (consumes root)
```

- **What exactly is the trust root?** A **human-only tenant-admin bootstrap provisioned out-of-band of Solvent's own `CreateTarget` path**. Concretely PROPOSED as *one of* (not decided): deployment-time signed manifest (sealed with release key), IdP group membership (`tenant-admin` group in the enterprise IdP), hardware token enrollment, or `cmd/solvent-mcp`-like provisioning CLI run by a human operator. Any of these is acceptable *iff* it is a human-operated provisioning control plane, not Solvent calling itself.
- **Is it an identity provider? deployment config? human action? signed manifest? combination?** For MVP, **deployment config + human provisioning action attested by the enterprise IdP** (combination). Pure IdP is not root (IdP admin could be agent); pure deployment config is not root (config file can be agent-written). The combination — *a human whose `principal_type=human` is asserted by the IdP, performing the provisioning* — is the minimal non-Agent root.
- **What must Solvent verify?** That the bootstrap principal row has `principal_type='human'`, `issuer` is the enterprise IdP (or workload attestation service), and the row was created via the provisioning API that requires `human` client-credentials (mTLS with `client-certificate` or `IdP token` with `amr=mfa`). What Solvent **merely consumes** is the *list* of bootstrap admins (the manifest) — its *authenticity* is checked by the provisioning control plane's signature/IdP verification, not recomputed by Solvent.
- **How is root authority revoked?** Revoking a bootstrap admin is itself an `Approve`-gated `policy_version` change: a second `human tenant admin` approves the removal. Until then, revocation is **service + append-only `principal(revoked_at)`** (proposed DB `CHECK (revoked_at IS NULL OR revoked_at > created_at)`, future). A compromised bootstrap is a **tenant-compromise** (`docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:112` — correctly states) — mitigated by IdP MFA, hardware token, and immediate revocation, not by Solvent re-proving human.
- **Can an agent ever be a bootstrap principal?** **Never.** `AI agent cannot bootstrap the authority system that will subsequently authorize that same agent.` (`mvp_plan3_a2.md`) — locked. If an agent_BOOTSTRAP were allowed, the recursion `authority to create authority requires authority` would have its base case be the agent whose authority we are trying to bound.
- **Can one human bootstrap another?** Yes — the *second* human is bootstrapped by the *first* via the same out-of-band provisioning path (and is `human → human`, not `agent → human`). The `principal` table's `issuer` remains the IdP, and the `author_principal` in `policy_version` is the original human. No agent ever appears in this chain.
- **Can policy bootstrap itself?** **Never.** `policy_version` rows are append-only and `author_principal` must be a bootstrap-admin; an `Approve` cannot create the `policy_version` that would allow that `Approve`. Policy bootstrap via policy is the recursion that the external root breaks.
- **Do not turn Solvent into IAM:** Correct. Solvent **persists** `principal {type, id, issuer, tenant_id, revoked_at}` (`DB` — makes states impossible), but the proof that `type=human` is genuine belongs to the IdP. Solvent's `I-A4` is therefore `Authenticity → IdP, Persistence+CHECK → DB, Role mapping → Service` (see `## Enforcement Taxonomy` row for `agent cannot approve`).

---

## Principal and Identity Boundary

*Minimum semantic model, not a full IAM schema (explicitly `OPEN` where noted).*

- **Is principal identity enough?** No — `principal_id` alone conflates `human`, `agent`, `workload`, `service account`, and `delegated principal` (the four classes `docs/ARCHITECTURE/...:475`). Each must be a distinct `principal_type` with different capabilities.
- **Is principal type enough?** No alone — `type=human` without an `issuer` is self-asserted. Minimum is `{principal_id, principal_type, issuer, tenant_id}` persisted (`principal` row). For MVP, `issuer` is the enterprise IdP identifier or workload attestation service; delegation as a **first-class relation** (`delegated_principal → delegating_principal`) is **OPEN/deferred** — a delegate is its own principal requiring its own `Approve` for now.
- **Is an explicit issuer required?** **Yes** for persistence. Without `issuer`, a weaponized workload can assert `principal_type=human` and pass `I-A4`.
- **Is tenant binding required?** **Yes** — `tenant_id` on every `principal` and authority-bearing row. An `agent` in tenant A must not discharge `needOperatorSignoff` for tenant B. The spec's `scope` already anticipates `tenant=acme`, but tenant is not `scope` — it is the isolation FK ( `## Tenant Boundary`).
- **Is delegation a first-class relation?** For MVP, **no** — treat a delegate as its own principal requiring independent approval. Full chain (`delegated_principal → delegating_principal` with independently verifiable relationship) is `OPEN` per `docs/ARCHITECTURE/...:475`.
- **How to prevent `workload → human` masquerading?** `workload` identity is **workload-attested** (e.g. SPIFFE, cloud instance identity), not password-based. The service rejects any `principal_type=human` that was not issued by the enterprise IdP's `human` factor (`amr=mfa` or hardware token). This is `Identity` enforcement, not `DB` — the DB only persists the `issuer` that the Identity layer already verified.
- **How does Solvent know which IdP issued the assertion?** The `issuer` field is part of the persisted `principal` row, supplied by the calling adapter (`MCP` header / `REST` `Authorization` token) and **verified** by the Solvent service against the IdP's JWKS / workload attestation service before any `CreateTarget` is accepted. Until then, every `action_intent` caller is effectively anonymous (`cmd/solvent-mcp/main.go:39-44` — `FABLE_DSN` with no auth).
- **What survives identity rotation?** `principal_id` is stable; rotation produces a **new row** with same `principal_id` but new `issuer` / key material and a `revoked_at` on the old row. Authority history (`debt_discharge.discharged_by`, `authority_target.approved_by`) continues to reference the old `principal_id` (history is immutable); new approvals require the new `issuer`'s assertion.
- **What happens when a principal is revoked?** `principal(revoked_at)` is set `NOW()` in one transaction; any `Approve` that references a `revoked_at IS NOT NULL` principal is refused by a `CHECK`/`FK` on `active` rows (proposed). Already-discharged obligations remain accepted unless the discharge itself is revoked (separate `DebtDischarge.status='revoked'`).
- **Distinguish:** `identity proof` = `IdP assertion {principal_id, type, issuer, expiry}` (ephemeral, verified); `principal record` = durable `principal` row (persisted, FK-bound, makes states impossible); `authorization role` = `policy_version` + `I-A4` mapping `principal_type → may Approve` (service/policy). The three are not the same object.

---

## AuthorityTarget Boundary

*PROPOSED, no `AuthorityTarget` table today (`db/001_schema.sql:60-76` only `belief_id/action/state`).*

The target is the **exact tuple** (`docs/ARCHITECTURE/...:448` five fields):

```
AuthorityTarget
    principal     {principal_type, principal_id}  — opaque, exact equality
    resource      {resource_type, resource_id}    — opaque, no normalization
    scope         explicit constraint set (dimensions OPEN, see below) — narrows only
    action        {action_namespace, action_name} — canonical, no adapter normalization
    consequence   {consequence_type, consequence_parameters} — agreed descriptor
```

**Is creating an AuthorityTarget merely data entry, or does it already carry authority consequences?**

Mere **data entry in state `proposed`**, never authority. Creation does not attach a belief, does not discharge, does not promote, does not `Approve`. An attacker who creates 1000 plausible targets (`resource=treasury-account`, `consequence=funds_transferred` with legitimate `Belief A` attached) has created 1000 **proposed** rows that are **enumerable but not `active`**. This is why the review's `target immutability enforcement` and `link creation authorization` are high-severity: the *storage* is cheap, the *activation* is not.

**What creation restrictions are genuinely necessary (minimum)?**

- `authenticated to tenant` (Identity) — anonymous creation is refused
- Well-formedness: opaque `resource`/`consequence` are syntactically valid, not `*` (wildcards only if policy explicitly lists them — none for MVP), no empty `action`
- No `*` wildcard unless policy explicitly allows it — and MVP policy **does not**
- **No** check that the creator “owns” the resource yet — *ownership* is the consequence-specific authorization that `Approve` enforces; requiring scoped authority to create would create the same `authority-to-create-authority` recursion as `Approve`. Keep creation permissive, **approval strict** — this matches the `proposing ≠ approving` principle and makes the 1000-target flood a DoS issue (rate limiting, not privilege escalation).

**Whether resource/consequence ownership matters?** Yes for `Approve`, not for `Create`. An approver for `treasury-account` must have authority scoped to that `resource`/`consequence`.

**Whether target creators need scoped authority?** For MVP, creation needs only `authenticated to tenant`; scoped authority is needed only to `Approve`. Tightening creation to scoped authority is a **future tightening** if link-sprawl DoS (F-14 in Self-Attack) becomes a product problem, not a security prerequisite.

**Whether justification attachment needs separate authority?** **Yes as `AttachJustification` authority**, but sharing the same `authenticated to tenant` as creation is sufficient for MVP. Attaching `Belief A → Target("move $10M")` is already blocked by `Approve`, not by `Attach`; requiring separate `Attach` authority now would create two indistinguishable gates (`I-A2` + `I-A4`). Keep `Attach` as `authenticated + belief exists + obligation ∈ current debt` and let `I-A4` be the security boundary.

**Whether approval should bind to an immutable snapshot?** **Yes.** `Approve` must bind to an **immutable snapshot**: `target_id + justification set (belief ids + obligation keys) + policy_version_hash` at `Approve` time (`policy_version_hash` snapshotted, as `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md` already plans). Any later justification addition is a **new authorization** (new Warrant).

**Whether high-impact approval requires separation of duties?** **Yes** — `approver ≠ requester` **and** `approver_type = human|service` (not `agent`) for `HIGH_IMPACT` (`I-A4`). This is the `Solvent-level separation of duties` that is a deliberate, small kernel of policy inside the authority model, not a generic policy language.

---

## Justification Boundary

*The `Belief → supports → AuthorityTarget` link.*

```
Belief
   │
   │  "this belief supports this target"
   ▼
AuthorityTarget  ──explicit Justification link──▶  many-to-many via Justification rows, AND semantics
```

**Who may create this claim?** Any `authenticated` principal (agent or human) may *propose* a `Justification` in state `proposed`. The link itself is **not discharged** and **not approved** until the target is `Approve`d.

**What makes it valid?** Three checks, all service-layer for MVP (with DB FK for per-link promotion dependence as the only row-local DB piece):

1. Both ends exist and are in the right state: `target` is `proposed`, `belief` is `entered` or `promoted` (not `retracted`), same `tenant_id`.
2. The `obligation_key` is in that belief's **current** obligation set (policy-owned; `internal/belief/mapping.go:15-34` is the current, too-permissive reality that this replaces).
3. The instrument that would discharge that obligation qualifies per the obligation's `discharge_policy` **type** (`deterministic` vs `attested` — see `## Policy Boundary`).

**Can an agent propose it?** **Yes.** The agent's job is to propose well-formed justifications (`internal/view/explain.go` `predicted_*` is the Ask surface).

**Can an agent finalize it?** **Never.** A `Justification` in `proposed` becomes part of an `active` target's justification set **only when the target is `Approve`d** by an `I-A4`-qualifying approver. There is no `FinalizeJustification` operation independent of `Approve`.

**Can the target creator be the justification approver?** `CreateTarget` and `AttachJustification` may share the same principal (agent); the *approver* may not be the same agent as the requester for `HIGH_IMPACT`. The justification's *validity* does not depend on who proposed it — only on the belief's current `promoted` status and the `DebtDischarge` ledger (`N2`), which are checked at `Approve` time.

**Can one belief be attached to arbitrary targets?** **Yes as a `proposed` link**, but that link never becomes `active` authority without `Approve`. The attack `Belief "customer identity verified" → Target "transfer treasury funds"` is therefore a **visible** `proposed` row, not a hidden permission.

**Can a customer policy define arbitrary links?** **No** — the policy defines the **obligation set**, not the link set. Links are data, not policy. A policy that pre-declares `Belief A → Target X` would conflate policy with data; the review's `customer-defined policy bypass` is blocked because obligations are policy, but links are service-validated data.

**Can a link be modified after approval?** **No — immutability after `active`.** `Approve` snapshots the justification set (`justification_snapshot`). Late addition is `Not the authorized target` (`I-12`) — it requires a **new target / new approval / new Warrant**.

**What invalidates a link?** Belief retraction (`RetractCascade` → `gate` + `live_requires_promoted`) makes any `active` target that depends on that belief **fail closed at next warrant verification** (`I-12` + central verification, not instant DB invalidation of the target row itself until the set-level `AND` becomes DB-enforceable). The link row remains for audit; the target's `active` status becomes unverifiable.

---

## Debt / Discharge Boundary

**Current reality:**

```
belief.debt[] TEXT[]  +  RetireDebt array_remove  (caller asserts item)
```
- `content_sha256` is caller-supplied (`kernel/kernel.go:73`), no server verification (`I-P2` violated today)
- `RetireDebt` is `array_remove`, idempotent, no evidence linkage, no actor attribution (`kernel/kernel.go:81-88`)
- `Promote` swallows `ErrPromotionBlocked` as non-error in pipeline (`internal/belief/belief.go:87-96`), one caller may retire any item (`cmd/solvent-mcp/tools.go:104` enum check only)
- Same `content_sha256` can be used across beliefs (`internal/belief/belief.go:103` `evidenceExists` checks per-belief with TOCTOU), so cross-belief reuse is unrestricted
- `belief.debt[]` and a future `DebtDischarge` ledger would be **two representations of one semantic fact**

**Proposed canonical source (minimum):**

```
Obligation (policy-owned, per belief type; 1..* at entry, size ≥1)
    ↓  discharge_policy ∈ {deterministic|attested|quorum} + required provenance + actor class + min independent instruments
DebtDischarge (append-only, instrument_kind∈{evidence,attestation}, discharged_by→principal, integrity proof, status∈{accepted,refused,revoked})
    ↓  derived outstanding debt = obligations − accepted discharges (for that belief, that obligation)
    ↓  promotion predicate: outstanding == ∅ ∧ ¬final_truth ∧ not retracted ∧ policy permits (N1∧N2∧N3∧N4, docs/ARCHITECTURE/...:688)
```

- **Is the array transitional?** **Yes.** Transitional dual truth `debt[] (CHECK truth)` + `DebtDischarge` ledger `(justification truth)` is explicitly tolerated for migration, with **single-transaction atomicity** `RetireDebt + Discharge` as the bridge (see `## Concurrency Boundary`). **Preferred long-term** is `Obligation rows → Discharge rows → derived outstanding debt`, but exact schema is `OPEN`.
- **Must discharge be a durable fact?** **Yes** — `accepted` is append-only; `refused`/`pending` are retained for audit; `revoked` marks a prior `accepted` instead of deleting it.
- **Does discharge need identity?** **Yes** — `discharged_by` → `principal` (attributable `human` or verified system, never `agent:` for attested, `I-P2`).
- **Does discharge need evidence?** **Yes** — `instrument_kind` + `instrument_ref` (`evidence` row id or `attestation` reference with payload-bound `belief_id+obligation` inside integrity proof `docs/ADR/0001:175-178`).
- **Can discharge be revoked?** **Yes** (compromise, departure) → re-issues the obligation and revokes the ledger row — writes `discharged_by=revoker, status=revoked, revoked_at` **and** re-adds the obligation (two writes, one tx, same as `RetractCascade` discipline).
- **Can policy invalidate old discharge?** **Yes** — `policy_version` change invalidates future `Approve`s that would rely on that discharge; the old discharge row remains (`immutable historical evidence` under `I-P4`).
- **Can one instrument satisfy multiple obligations?** Only via **distinct DebtDischarge rows** per `obligation`, each with its own policy check (`## Cardinality` moved attack — one instrument, many obligations is allowed only when each obligation's policy explicitly qualifies that instrument kind; otherwise it is a bypass).
- **How is replay prevented?** Attestation: `attestation_id` unique per `(belief_id, obligation)` inside signed payload (`docs/ADR/0001:175-178`); Evidence: server-verified `content_sha256` + per-belief dedup (`internal/belief/belief.go:103` narrowly, but cross-belief reuse blocked by `I-P2` policy that requires `reproducible_artifact` not `external_feed`).
- **How is atomicity preserved?** `array truth` vs `ledger truth` divergence is the **dual-truth** attack (Review #3 F-02). Until rows-derived debt replaces the array, every `RetireDebt` must be in the same `crdb.ExecuteTx` closure that inserts the `DebtDischarge(accepted)` row (see `## Concurrency Boundary`).

---

## Policy Boundary

*Solvent lets tenant policy determine obligation set, obligation type, discharge qualification, and potentially risk-related requirements — but not below the floor.*

**Minimum semantic validity vs recommended posture vs tier:**

| Tier | Minimum semantic validity (locked as `I-P1/I-P2`, refused at policy write) | Recommended posture (candidate, flagged) | HIGH_IMPACT vs LOW_RISK |
|---|---|---|---|
| `READ` / `DRAFT` | `I-P1` only (non-empty, non-self-satisfying, no universal) | nothing beyond `I-P1` | No approval, no attestation — scoped to `resource` (reading `patient 123` ≠ reading cache) |
| `LOW_RISK` | `I-P1` + `I-P2` deterministic provenance (`deterministic ≠ unverified external_feed`) | `≥1` obligation should remain `deterministic` with server-verified artifact if available | Policy-dependent; no `HIGH_IMPACT` approver required |
| `HIGH_IMPACT` | `I-P1` + `I-P2` + **stronger floor** (see below) | **Candidate** `≥1 attested or quorum` (`I-P3`) **pending** real high-impact customer use case; until then `solvent_explain` flags `predicted to be weaker than candidate floor` | Requires explicit human/non-agent `Approve` distinct from requester (`I-A4`) |

**Universal prohibitions (always `refused` at policy write, never customer-overridable, not even with “we explicitly want weaker”):**

- `empty obligation sets` → `invalid` (would make `debt=[]` vacuously promotable)
- `self-satisfying obligations` (always satisfied) → `invalid`
- `any evidence qualifies` / universal `provenance_class` → `invalid`
- `agent-generated attestation` for `attested` (where `attested_by` is `agent:`) → `invalid`
- `external_feed` with caller-supplied `content_sha256` satisfying `deterministic` alone → `invalid` (requires `I-11` server-verified hash)
- `quorum` counted as `same artifact N times`, `same principal N times`, or `same model N outputs` → `invalid` (counts `N independent` sources, distinct principals **and** provenance channels, not count)

**Is `HIGH_IMPACT ⇒ ≥1 attested or quorum` universal?** **No — candidate `OPEN`.** Making it a universal kernel invariant would hard-code “human approval” as the answer for every industry, which the product brief explicitly warns against (some environments have genuinely stronger deterministic controls). The universal *is* `≥1 obligation must be non-trivial` + `I-P2`; the candidate stronger floor is a **product default** that becomes `Discharged` only when a real high-impact tenant demonstrates its need. Until then, a tenant that configures `HIGH_IMPACT` with only `deterministic` obligations is **allowed but flagged** in `solvent_explain` as `candidate floor` and still requires human approval per `I-A4`.

**Customer-controlled bypass:** `Customer policy flexibility ≠ permission to make Solvent meaningless.` A tenant admin who controls their own policy (via `policy_version`, author `human` per `I-P4`) could still define `empty` obligations. The floor prevents *semantic* bypass even though the write path is *authorized* — the policy `write` itself is validated against `I-P1/I-P2` and refused before any target is created.

---

## Warrant Boundary

*Warrant as **portable representation/reference**, not an authority source (`docs/ADR/0001:440-466` illustrative).*

- **Issuance:** at `Approve` time; binds `target_id + AuthorityTarget tuple + justification snapshot (belief ids & `promoted` status) + `issued_at` + `policy_version_hash` + `approver` principal. No separate `warrant` table required for MVP — the reference can be the `target_id` + `warrant_id` derivation; persistence is deferred (see `Executive Decision` 5-table minimum: `warrant` is `DEFERRED`).
- **Binding:** `target_version` — any tuple mutation is a different `AuthorityTarget` (immutable); `justification snapshot` — the exact `belief.promoted` states at `Approve` time are snapshotted. Target version change → new warrant; justification retraction → warrant invalidates.
- **Validity:** central verification against current ledger: `target still active`, `all justification beliefs still promoted` (again, the `AND` set), `policy_version` not revoked, `issued_at` within validity, no intervening `RetractCascade` or temporal expiry (deferred). `Warrant is not authority; possessing it grants nothing` (`docs/ADR/0001:479`).
- **Revocation:** retracting any justification belief makes every warrant that cited it **immediately** fail `I-12` at next verification (the `ON UPDATE CASCADE` propagation + `live_requires_promoted` analog for targets). Policy upgrade that tightens the floor makes warrants that would not meet the stricter floor fail at next verification (see `## Policy Change`). Explicit `warrant revoked` row is not required for MVP.
- **Replay:** attestation-style `attestation_id` uniqueness and `DebtDischarge` ledger handle replay for promotion; warrant replay **as a second execution** is `Warrant→Execution 1:1` for MVP (`OPEN` — whether `N` idempotent executions per warrant is allowed is deferred to `concurrency` + `I-13`).
- **Caching:** `cached ALLOW` that outlives `Retraction` is a **fail-open bug**. Caching is allowed only until next `warrant verification` or until `policy_version` changes; executors must re-verify before every `HIGH_IMPACT` execution. Bounded `ALLOW` caching with `expires_at` is `I-13` deferred, not MVP.
- **Concurrency:** two concurrent `Approve`s on same `proposed` target must serialize via `crdb.ExecuteTx` + `UNIQUE(target_id) WHERE state='active'` (proposed, `I-7` discipline) — exactly as `Promote` serializes on `23514`.
- **Policy version:** warrant carries `policy_version_hash` at issuance; verification checks `hash == current` *or* `hash is still permissible under current` — the delta is logged (`global_summary` shows `policy_version_hash@approval ≠ current`).

Attack residual: `same warrant used twice / by another principal / with altered consequence / cached during outage / two concurrent executions` all fail at `I-12` (`authorized == presented` equality) except `cached during outage` which fail-closed per `docs/ADR/0001:113-134` — `unavailable` is `REVIEW`/`DENY`, never `ALLOW`.

---

## Execution Boundary

Preserve `I-12 AuthorizationTargetMatch` vs `I-13 ExecutionEffectMatch`:

- **I-12** `authorized target == presented target` — **Solvent guarantees** (service + optional DB `CHECK`/`CHECK` on stored tuple) the executor was authorized for **exactly** what it presented, verified server-side **before** execution.
- **I-13** `presented target == actual world effect` — **Solvent does NOT guarantee**; the executor (or external system) must prove the world effect matches the presented target via a **trusted executor / signed execution receipt** (deferred).

**What Solvent can actually guarantee?** That `authorized == presented` was true **at verification time**.

**What must the executor guarantee?** That it `echoed` verbatim (no normalization), enforced `fail-closed` on `unavailable`, never mutated after verification, never applied one `ALLOW` to a second tuple, and — for `HIGH_IMPACT` — optionally produced an `I-13` receipt (deferred to the executor contract, `Review #2 C-4`).

**What must the external system guarantee?** The real-world system's observability that the effect occurred — Solvent can *never know* that the Treasury transfer actually settled, DNS actually propagated, or the cache actually cleared, without an **executor-supplied observation** that is itself trusted.

**Do high-impact actions require receipts?** For MVP, **no** — `I-13` is `DEFERRED` (`docs/ARCHITECTURE/...:527` “post-execution attestation deferred”). For a `HIGH_IMPACT` tenant that cannot tolerate `I-13` uncertainty, a receipt is the **product-priced** upgrade path (not a universal MVP requirement).

**Can Solvent ever know the real-world effect occurred?** **No** without a trusted executor. Solvent's position is `presented == authorized is verified; presented == effect is executor's claim until attested by a trusted observer` — the trust boundary diagram's honest split.

---

## Tenant Boundary

*Future SaaS — current `scenario_id` `000…001/002` (`main.go:31`) is a fixed **scenario**, not an authenticated tenant.*

**Minimum boundary for:**

- `tenant` (the `tenant_id` itself): isolated by `tenant_id` FK from deployment/provisioning `principal.tenant_id` (future `DB` `CHECK` + RLS candidate). CURRENT: no `tenant` table, no `tenant_id` column anywhere.
- `principal` : `principal.tenant_id` FK → `tenant`; a `human` in tenant A never appears in tenant B. No cross-tenant principal reuse.
- `resource` : `resource_type/resource_id` are **tenant-scoped opaque identities**; `resource=patient/123` in tenant A ≠ `resource=patient/123` in tenant B (exact equality includes implied `tenant_id` prefix, enforced by service).
- `evidence / belief / target / warrant / justification / policy_version` : each row carries `tenant_id` (future) `→ tenant`, FK-bound, and every `SELECT`/`INSERT` is tenant-scoped. The current `scenario_id` even as a toy tenant is insufficient because it is a **fixed constant**, not `authenticated tenant binding` from Identity.

**Attacks:**

- `tenant A evidence → tenant B belief → target → authority`: fails at `evidence.tenant_id ≠ belief.tenant_id` (future `CHECK` / service `authenticated tenant == row tenant`), but **currently succeeds** — `belief.scenario_id` is the only scoping and any caller who can connect to the database can access any scenario (`docs/REVIEWS/...:424` multi-tenant verdict — correctly says **No**, the abstraction is not yet safe for SaaS tenancy). Hence `tenant` is the **most urgent future FK** even though `tenant` as a separate table is `DEFERRED` (see `Executive Decision`).
- Identical `resource_id` across tenants: blocked by tenant-scoped equality (above).
- Shared evidence artifacts: `artifact` (reusable) may be global, but `citation` (per-belief relevance `OPEN #13`) is **tenant-scoped** — an artifact produced in tenant A can be cited only by a `citation.tenant_id == A` row, never by tenant B's belief, without an explicit cross-tenant `citation` that itself requires `tenant-admin` approval (defense-in-depth, beyond MVP).
- Delegated principals: a `delegated principal` retains its own `tenant_id` (own tenancy), not the delegator's; delegation across tenants is `OPEN` and out of MVP.
- Cross-tenant warrants: a warrant's `target_id` → `tenant_id` binding makes cross-tenant use a `tenant_id` mismatch at `I-12` → `DENY`.

**Must eventually be tenant-bound (DB `CHECK`/`FK`):** every authority-bearing row (`belief`, `evidence`, `AuthorityTarget`, `Justification`, `DebtDischarge`, `PolicyVersion`, `Warrant` reference). For MVP, the four-verb slice remains safely **single-tenant/local** and treats multi-tenant as `OPEN` debt, but the trust map must already show the boundary (see `## Trust Boundary Map`).

---

## Concurrency Boundary

*Preserve existing `crdb.ExecuteTx` `40001` retry semantics (`kernel/kernel_test.go:327` B-18).*

| Race | Where atomicity must happen | What state must be serialized | What can safely retry | What must fail closed |
|---|---|---|---|---|
| `Promote vs Retract` | `Promote` (`UPDATE belief status`) vs `RetractCascade` (cancel `live` then retract) | `belief.status` + `action_intent.state` (the `gate`/`live_requires_promoted` pair) | `40001` serialization failure on either side → `crdb.ExecuteTx` retries then refuses on fresh state | Concurrent `Promote` that would leave `live on non-promoted` is impossible (`B-13` global audit) |
| `Discharge vs Promotion` (future `RetireDebt+Discharge` vs `Promote`) | **Single transaction** `array_remove + Discharge(accepted)` (proposed `N2`) | `belief.debt[]` emptiness + `DebtDischarge` ledger (transitional dual truth) | Either side retries on `40001`; the loser sees the winner's committed debt | Half-discharged promotion is impossible — both sides are `N1∧N2` |
| `Target mutation vs Approve` | `Approve`'s snapshot of target tuple + justification set | `AuthorityTarget` tuple + `Justification` set + `policy_version_hash` | `Approve` retries; concurrent `AttachJustification` between snapshot and `Approve` causes `Approve` to fail (snapshot mismatch) — correct | Mutated tuple is a different target → `NOT AUTHORIZED` (`target immutability`) |
| `Policy change vs Approve` | `policy_version` append + `Approve` | `policy_version` rows (append-only) | `Approve` retries; if `effective_at` moves forward during `Approve`, the `Approve` binds to the newer version or fails and the caller retries with the new hash | Policy downgrade never silently expands existing `active` targets — each `active` is bound to its approval-time hash |
| `Approve vs Approve` (same target) | `Approve` `proposed→active` | `AuthorityTarget.state` with `UNIQUE(target_id) WHERE state='active'` (proposed) | **One** succeeds, **one** gets `23514` unique violation → service maps to `REVIEW` (not `DENY`), caller re-reads with `solvent_explain` | Double activation is `REVIEW`, not `ALLOW` |
| `Warrant verification vs Retraction` | Verification's `SELECT` of `belief.status` + `target.state` vs `RetractCascade`'s `UPDATE`s | `belief.status` and `Justification` validity | Verification retries; `RetractCascade` wins → verification now sees `retracted` and returns `DENY` | Stale `ALLOW` must not outlive retraction — verification is **before every** `HIGH_IMPACT` execution, not cached |
| `Execute vs Retraction` | Execution's pre-execution re-verification vs `RetractCascade` | Real-world system + ledger | Executor's `present → verify → execute` sequence must re-verify immediately before `execute`; retraction between `verify` and `execute` is the residual that `I-13` deferred receipts would close | Execute without re-verification is executor breach of contract, not Solvent bug — but Solvent must make re-verification the documented, testable path |

For every race, `crdb.ExecuteTx` serialization is the existing mechanism; the new `AuthorityTarget`/`Justification`/`DebtDischarge` races use the same `crdb.ExecuteTx` discipline and `40001 → retry → refusal on fresh state` (`AGENTS.md:102`).

---

## Lean Boundary

*Do not change Lean now.*

**Should be formally proven (eventually):**

- `proposed target → debt-free promoted belief` preservation (`promote_preserves_validity` already proves the N1 half)
- `live intent → promoted` (`live_intent_implies_promoted` — already)
- `active AuthorityTarget`'s justification snapshot → `all beliefs promoted at snapshot time` (future `Approval` transition)
- `discharge → debt` coherence (`N2` one-1 rule, future)
- `approval separation` (`I-A4` human ≠ agent, future)

**Should be DB-enforced (now or soon):**

- N1, gate, live_requires_promoted, `Justification` per-link composite FK (proposed), `DebtDischarge` uniqueness (`UNIQUE(belief_id, obligation) WHERE status='accepted'`)
- `Target` immutability (no `UPDATE` path), `policy_version` append-only

**Should be service-tested (always):**

- `N2` discharge-array atomicity while transitional, `obligation qualifies`, `warrant re-verification`, `Approve` AND-set collection, tenant scoping (until `tenant_id` exists)

**Should be identity-tested (outside Solvent):**

- `principal_type` authenticity (`human` really human), `issuer` matches IdP, workload attestation

**Should be integration-tested (DocTrust + warrant):**

- `solvent_explain` shows `predicted` vs `real 23503/23514`, `mcp_verify.sh` 7 tools, `four_verb.sh` demo (10 steps)

**Should remain outside Solvent:**

- Evidence *truth* (Solvent checks provenance, not whether the release really is safe), policy *wisdom* (universal negatives keep it non-vacuous), actual world effect (I-13)

*Avoid proving propositions that the implementation does not actually enforce* (`docs/REVIEWS/...:152`). A Lean theorem `live_target_implies_promoted` that has no `AuthorityTarget` table would be a proof about an abstraction, not about the ledger.

---

## Cross-Layer Invariants

*IDS follow this spec; mapping to review I-9..I-13 is in the table, not identity. No syntax is final.*

- **I-9 AuthorityTargetBinding** — An `AuthorityTarget`'s tuple is opaque exact equality (`principal/resource/scope/action/consequence`), `I-12` is the check that must match at verification.
- **I-10 DebtDischargeIntegrity** — `discharged_by` is an attributable `principal`, `belief_id+obligation` is inside the integrity proof, one `accepted` per `obligation` (`UNIQUE` where `accepted`).
- **I-11 EvidenceProvenance** — `server-verified` `content_sha256` and `provenance_class` enforcement at ingress, replay protection.
- **I-12 AuthorizationTargetMatch** — `authorized tuple == presented tuple` server-side **before** execution (see `## Execution Boundary`).
- **I-13 ExecutionEffectMatch** *(reserved, deferred)* — `presented == world effect` via trusted executor / signed receipt, not in MVP.

| Spec ID | Spec meaning | Adversarial review relationship |
|---|---|---|
| I-9 | Authority target bound to authorized principal/resource/scope/action/consequence + AND over justification set | Review I-9: resource/consequence binding (`docs/REVIEWS/...:202`) — same intent, widened to full tuple |
| I-10 | Debt discharge bound to exact obligation, with integrity/attribution | Review I-10: attestation binding (`:203`) — spec I-10 covers review I-10's attestation binding as its `attested` case, plus deterministic |
| I-11 | Evidence provenance and replay enforced at ingress | Review I-11: evidence provenance (`:204`) — aligned |
| I-12 | Authorized tuple matches presented tuple (I-12 AuthorizationTargetMatch) | Review I-9 second half / F-01: the authorization-boundary execution half; renamed from `ExecutorConsequenceMatch` to avoid overclaim |
| I-13 (reserved) | World effect matches authorized tuple (I-13 ExecutionEffectMatch) | Review I-13 (`:206`) execution-effect / deferred |
| — | Availability / fail-closed semantics | Review I-12 (`:205`) — governed by `docs/ADR/0001` Decision 2, outside I-9..I-13 |
| I-A1 | Every target creation is an auditable `proposed` row | Review I-A1 (target creation authority) — from `I-A1..I-A4` which had zero DB backing |
| I-A4 | Agent never approves `HIGH_IMPACT` (requires human/non-agent distinct from requester, bootstrap root) | Review I-A4 |
| I-P1 | Policy never vacuous (`non-empty`, `non-self-satisfying`, `no universal qualifier`) | Review I-P1 |
| I-P2 | Qualification integrity (`deterministic≠unverified external_feed`, `attested≠agent`) | Review I-P2 — currently contradicted by `kernel/kernel.go:73` pass-through |
| I-P4 | Policy version binding (`policy_version_hash` snapshotted at approval) | Review I-P4 — currently no `policy_version` table |

---

## Self-Attack Results

*For every major rule: `blocked by invariant / service contract / policy floor / identity / detected by audit / deferred / unresolved`.*

| # | Attack | Disposition |
|---|---|---|
| 1 | Legitimate promoted belief + malicious target creation (treasury `transfer`) | **Blocked by service contract + identity** (`I-A4`: `HIGH_IMPACT` requires human/non-agent `Approve` ≠ attacker; agent `Approve` refused). Target stays `proposed`; `I-12` never reached. Residual: compromised `human` IdP token can approve — **deferred** to IdP MFA/hardware. |
| 2 | Legitimate promoted belief + malicious justification link (`Belief A → transfer treasury`) | **Blocked by service contract** (`I-A2` per-link check: belief must be promoted, obligation ∈ current debt, instrument qualifies) **and** `I-A4` at `Approve` (link visible but never `active`). The moved-broadening attack is `Detected by audit` (enumerable row) until `I-A2` is fully `OPEN #8` gated. |
| 3 | Legitimate target + malicious approver identity (workload masquerades as human) | **Blocked by identity** (`I-A4` + IdP asserts `principal_type ≠ agent`) + **detected by audit** (issuer mismatch). A buggy identity adapter that misclassifies `workload` as `human` bypasses — hence `I-A4` must be split: authenticity→IdP, persistence+CHECK→DB. |
| 4 | Strong policy + malicious link creator (strong `deterministic` + `attested` floor, but attacker attaches) | **Blocked by service contract** (`I-A2` link validation + `I-A4` approval). Strong policy alone does not compensate for unauthorized link — both layers must hold. |
| 5 | Weak policy + honest approver (tenant policy `{needProvenanceCheck: any external_feed}`) | **Blocked by policy floor** (`I-P1` universal “any evidence qualifies” is `refused` at policy write, never customer-overridable). Candidate floor `I-P3` is `OPEN`, but the weak policy never becomes writable. |
| 6 | Policy downgrade after approval (`Policy A` strong → `Policy B` weaker) | **Detected by audit** (`policy_version_hash@approval ≠ current`) + **blocked by service contract** on *new* approvals (re-evaluated under current). Downgrade itself is `Approve`-gated via bootstrap `I-A4`. Existing `active` targets without `PolicyVersionBinding` are **deferred** until `policy_version` exists (Review #3 C-2). |
| 7 | Policy upgrade during execution (`Policy A` → stricter `Policy B` mid-execution) | **Blocked by service contract** (`execution-time verification` under current policy fails). Downgrade never expands existing `active` (I-P4 fail-closed). |
| 8 | Stale warrant after retraction (`Warrant` issued before `RetractCascade`) | **Blocked by service contract** (central verification invalidates immediately, `docs/ADR/0001:101`). Cached executor beyond verification is **deferred** (no warrant table, review I-12). |
| 9 | Cross-tenant evidence citation (`tenant A evidence → tenant B belief`) | **Blocked by invariant** (future `tenant_id` FK on every authority-bearing row; tenant-scoped equality). **Currently unresolved** — `scenario_id` is not `tenant_id` (future SaaS). |
| 10 | Workload masquerading as human for `HIGH_IMPACT` approve | **Blocked by identity** (enterprise IdP asserts `human`, workload attestation asserts `workload`; service enforces `I-A4` `approver ≠ agent` and `principal_type ∈ {human, workload-hardened}`) + **detected by audit** (issuer). Tenant-controlled IdP config remains **unresolved** trust (IdP compromise = tenant compromise). |
| 11 | Agent masquerading as service account (`agent:` counted as `service`) | **Blocked by identity** (IdP issuer distinguishes). Same `OPEN` as #10 — service must reject `agent:` self-assertion. |
| 12 | Fake quorum (5 outputs from same model counted as 5 independent sources) | **Blocked by policy floor** (`I-P2` `quorum` = `N` distinct principals **and** distinct provenance channels + cryptographic distinctness, not count). Until quorum has a customer, **deferred**. |
| 13 | Evidence replay (same `content_sha256`+`belief_id+obligation` re-presented) | **Blocked by invariant/attestation** (payload binds `belief_id+obligation` inside signed proof; `attestation_id` unique per `(belief_id, obligation)`). Revoked replay is **deferred** to lifecycle (F-05). |
| 14 | Debt-array / discharge-ledger divergence (`debt=[]` but ledger says unpaid) | **Blocked by design** (single-transaction `array_remove + Discharge(accepted)`, `crdb.ExecuteTx` discipline, `kernel/kernel.go:127` analog). **Undecided** whether pure `CHECK` can guarantee it without `Obligation rows → derived debt` migration (`OPEN #14`). |
| 15 | Target mutation after approval (`resource` changed from `payment-api` to `treasury-account`) | **Blocked by invariant** (target immutability `I-A3`: `active` tuple has no `UPDATE` path, additive-only; mutation is a different target). Service refuses. |
| 16 | Executor correctly presents `A` but performs `B` (`patient/123 modify → patient/999 delete`) | **Blocked by service contract** (`I-12`: `presented ≠ authorized` → refused). Residual where executor presents correctly then does otherwise: **unresolved** until `I-13` receipts (`deferred`). Spec binds authorization, not execution. |
| 17 | Customer creates 1000 targets to overwhelm approvers (DoS, not privilege escalation) | **Detected by audit** (1000 `proposed` rows enumerable) + **deferred** rate-limiting policy (not a security boundary, but an MVP product requirement). Suppressed via `pending` queue and bounded target scope, not via `I-A1`. |
| 18 | Bootstrap admin compromise | **Detected by audit** (bootstrap is human, enumerable, revokable `principal(revoked_at)`). **Deferred** revocation mechanism is append-only `principal(revoked_at)` + new `policy_version`. Tenant-compromise, not Solvent bypass — mitigated by IdP MFA/hardware, documented as such. |
| 19 | Two simultaneous approvals (`Approve` vs `Approve` on same `proposed` target) | **Blocked by invariant** (proposed `UNIQUE(target_id) WHERE state='active'` + `crdb.ExecuteTx` serialization; one succeeds, one gets `23514` → `REVIEW`). |
| 20 | Network partition during execution (`present → verify → execute` split) | **Blocked by service contract** (`HIGH_IMPACT` fail-closed on `unavailable` ≠ `DENY`; `solvent_explain` `predicted_*` vs real `23503/23514` distinction, `docs/FOUR_VERB.md`). Residual where verification succeeded but partition occurs before `execute`: `unresolved` until `I-13` receipt or idempotent re-verification. |

---

## Dogfood Evidence Ledger

*Strict rule: A coherent argument is **not** evidence. Repository-verifiable implementation, measured experiment, or an accepted-and-cited decision **as a decision** may discharge. Future claims without repo/use-case evidence stay `Open`.*

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Existing kernel 4 tables + `promoted_is_debt_free` / `gate` / `live_requires_promoted` are DB-enforced | `db/001_schema.sql:34` `belief_id_status_key`, `:68` `live_requires_promoted` `23514`, `:74` `gate` `23503` + `kernel/kernel_test.go:249` B-09, `internal/view/explain_test.go` | High | — | **Discharged** |
| `solvent_explain` is read-only and never invents SQLSTATEs | `internal/view/explain.go` `predicted_*` vs real `23503`/`23514`; `Taskfile.yml:224` MCP boundary grep no `INSERT`/`UPDATE` in `internal/view`/`cmd/solvent-mcp`; `scripts/mcp_verify.sh` 7 tools deterministic | High | — | **Discharged** |
| Agent output is not authority — `gate` is the authority boundary | `docs/ADR/0001:99` Decision 1 central verification + `README.md:111` “The database decides” | High | — | **Discharged** |
| Bootstrap recursion terminated at human tenant-admin (mechanism unspecified) | Design principle: `Deployment → Human admin → policy → Solvent` locked, existence not `OPEN`; exact manifest/IdP group/hardware remains `PROPOSED` | Low | Provisioning manifest/IdP group not specified | **Open** |
| “Same principal can perform everything” (Model A) is safe | **Contradicted** — moved-broadening attack (`Belief A → Target("move $10M")`) proves legitimate belief + attacker link becomes authority if no creation/approval separation. DocTrust must not share kernel and must use MCP/REST. | Low | — | **Contradicted** |
| “Target creator and approver may optionally differ” is safe for HIGH_IMPACT | **Contradicted** for `HIGH_IMPACT` — Review #3 requires `human/non-agent ≠ requester` (I-A4) for HIGH_IMPACT | Low | — | **Contradicted** |
| “Policy-controlled roles alone without universal floor” is safe | **Contradicted** — weak policy `any evidence qualifies` would pass if customer so configures; requires universal `I-P1`/`I-P2` floor | Low | — | **Contradicted** |
| “Existing authority required to create a new target” solves bootstrap | **Contradicted** as infinite recursion (`adv_plan_prompt.md:301`); requires human bootstrap exception, which is `I-A4`'s human-root | Low | — | **Contradicted** |
| `CreateTarget`/`AttachJustification`/`Request`/`Approve` as four distinct transitions | Design reasoning in four-role matrix; no `AuthorityTarget` table exists (`db/001_schema.sql` has no such table) | Medium | `AuthorityTarget` + `Justification` table existence | **Open** |
| Target immutability — exact tuple equality, no mutation | Design reasoning, additive-only discipline `docs/POST_HACKATHON_ARCHITECTURE.md:491-503` | Medium | No tuple column exists | **Open** |
| `NonVacuousPolicy` universal negatives (`I-P1`) sufficient to block meaningless `debt=[]` | Design reasoning from `F-04` checkbox theater; consequences demonstrated with synthetic `operator_asserted` rows | Medium | `obligation` table & policy-validation service `OPEN` | **Open** |
| `QualificationIntegrity` (`I-P2` deterministic≠unverified `external_feed`) | Design reasoning; current `kernel.go:73` pass-through `content_sha256` shows why verification is needed; I-11 deferred | Medium | Server-verified hash implementation `Deferred` | **Open** |
| Candidate `HIGH_IMPACT ⇒ ≥1 attested|quorum` (`I-P3`) | **Candidate**, `OPEN` pending real high-impact customer | Medium | Customer validation | **Open** |
| `PolicyVersionBinding` (`I-P4` version/hash/snapshot) | Design reasoning from audit requirement `adv_plan_prompt.md`; no `policy_version` ledger today | Medium | Ledger schema `OPEN` | **Open** |
| Central Warrant verification remains authoritative | Accepted decision `docs/ADR/0001:95` `Warrant is not authority` | High (as decision) | — | **Discharged** (as decision) |
| MCP Elicitation is channel, not signature | Accepted decision `docs/ADR/0001:187` bounded PKI | High (as decision) | — | **Discharged** (as decision) |
| Four-verb slice `Connect→Ask→Authorize→Reassess` is legible and DB-enforced | `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh` 10 steps green, `mcp_verify.sh` 7 tools | High | — | **Discharged** |
| Evidence ingress hardening (`I-11` server-verified hash, replay) | Not begun; fixture-only (`internal/pipeline/pipeline.go:41-57`) | — | Gateway design (F-02) | **Deferred** |
| Execution-effect verification (`I-13`) | Not in scope; receipts/trusted executor deferred | — | Post-execution attestation | **Deferred** |
| Temporal scope/freshness (`N5`) | `docs/ADR/0001:543` defers validity-horizon | — | `validity_horizon`/`expires_at` machinery | **Deferred** |
| Warrant as portable reference, not second source of truth | Accepted decision `docs/ADR/0001:479` | High (as decision) | — | **Discharged** (as decision) |

---

## Open Architectural Questions

Only genuinely unresolved questions — each blocks its schema decision:

1. **Target/link creation policy granularity** — exact shape of `allowed_principals` per `resource`/`consequence` for MVP (full ACL table vs short allow-list in tenant config) (`OPEN #8` incarnation).
2. **Scope dimensions** — which constraint dimensions are canon for `Scope` equality (tenant/environment/region/time-window/field) and their comparison semantics; time depends on deferred temporal authority.
3. **Attestation key/identity provisioning** — exact mechanism for bootstrap human-admin keys vs managed keys vs enterprise-identity-backed vs WebAuthn (`docs/ADR/0001:187` choices remain `OPEN`).
4. **Evidence artifact vs citation table shape** — `artifact` + `artifact_citation` vs alternative, prerequisite to `I-11` (`OPEN #13`).
5. **Debt array convergence** — whether to keep transitional `debt[] + ledger` dual truth or migrate to preferred `obligation rows → discharge rows → derived debt` while preserving `promoted_is_debt_free` (`OPEN #14`).
6. **N4 placement** — fourth promotion conjunct (`N1∧N2∧N3∧N4`) vs target-level `Authority Eligibility` (`OPEN #16`).
7. **Execution-effect scope** — whether `I-13 ExecutionEffectMatch` belongs in MVP via receipts or stays deferred; where the verification vs trust boundary ends.
8. **Bootstrap provisioning specifics** — signed manifest vs IdP group vs hardware token as the concrete artifact for the human-only root (OPEN, but existence of a non-agent root is locked).
9. **Obligation identity** — stable `obligation_key` naming, versioning, and `obligation_type` assignment (who assigns `deterministic|attested|quorum` to which `needProvenanceCheck` etc.).

---

## Implementation Gate

*Exact conditions that must be met before `AuthorityTarget` schema design, policy schema design, or kernel changes may begin. `PROPOSED — NOT IMPLEMENTED` stays until all hold.*

1. **This trust-model spec is promoted through Adversarial Review #3** — the review that attacks the bootstrap root, target/link creation authority, policy downgrade path, and whether “agent may create target but not activate it” is enforceable rather than contractual, and whose verdict is `APPROVE` or `APPROVE WITH CONDITIONS` with those conditions matching this gate.
2. **Every `OPEN` in `## Open Architectural Questions` is either resolved, or explicitly bounded out of the first implementation phase with its deferral recorded as debt** (exactly as this spec does for `OPEN #1, #3, #8, #13, #14, #16`).
3. **Every proposed invariant (`I-A1..I-A4`, `I-P1..I-P4`, `I-9..I-13`) has an assigned enforcement layer** (`Lean` / `SQL` / `Service` / `Policy` / `Identity`) with rationale — no invariant left “somewhere.”
4. **Bootstrap trust root is `human-only tenant-admin via deployment/provisioning`, with the exact artifact `PROPOSED` but the principle locked** — `AI agent cannot bootstrap` is the boundary that makes `I-A4` meaningful.
5. **Policy floor universals are `I-P1` + `I-P2` and are test-enforceable at the service layer** (empty/self-satisfying/`any evidence` rejected, `deterministic` rejects unverified `external_feed`).
6. **Agent vs human matrix is written as the integration-facing contract** and is `OPEN` for `workload` → `human` masquerading until `principal` table exists — but the `agent may create, never approve HIGH_IMPACT` rule is locked.
7. **Executor contract is written before any executor integration claims `I-12`** (`AuthorizationTargetMatch`: exact tuple equality, no mutation, `active` immutability, one decision → one tuple, fail-closed on `unavailable`).
8. **Dogfood ledger is re-scored after Review #3:** nothing may move to `Discharged` without repository/use-case evidence.
9. **Self-attack `valid → manufactured authority?` has a precise failure point:** the 20-attack table shows `I-A4` (identity) + `I-P1/I-P2` (policy floor) as the *exact* boundaries for the two critical attacks (`weak policy + malicious link` and `strong policy + malicious link`).
10. **Exact files changed:** `git status --short` shows only `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md` (and its review) — no `Go`/`SQL`/`Lean`/`MCP`/`REST`/`ADRs`/`authority-target-and-debt-discharge-spec.md` edits.

**If `NOT SAFE:`** list the exact `OPEN` that blocks (e.g., `OPEN #8` four-role incarnation not yet chosen → cannot write `Justification` table; `OPEN #13` artifact vs citation not yet bounded → cannot write `evidence_artifact` DDL). **If `SAFE:`** list the exact `constraints that the next phase must honor` (the `I-9..I-13` row-local vs service split above, the 5-table minimum `principal, authority_target, justification, debt_discharge, policy_version` with tenant FKs, and the `NULL→*` fail-closed rule).

---

## Recommended Next Step

**Next gate is `Adversarial Review #3` of *this* trust-model spec**, scoped to the four bullets above: bootstrap trust root, `agent may create but not activate` enforceability (does `I-A4` split `Authenticity→IdP / Persistence+CHECK→DB / Role→Service` hold?), policy downgrade (`P_A strong → P_B weaker` — does `I-P4` version binding + re-verification make downgrade attributable but not expanding?), and whether exact-tuple equality is enforceable rather than contractual (is `I-12` service `authorized==presented` sufficient for MVP, or must `I-13` receipts be in MVP?).

Only after that review's verdict (`APPROVE`/`APPROVE WITH CONDITIONS` matching this gate) is it **safe to begin schema design** for the 5-table minimum, and only then **kernel changes** (promoted ⇒ debt-free now also ⇒ `N2` accepted ledger, `N4` policy).

`Recommended Next Step` is **not** to write DDL now, but to submit this spec for that review — the first trustworthy bridge between Solvent's conceptual `AuthorityTarget` and its eventual production kernel, without optimizing for feature velocity over getting the authority model right before it becomes expensive to change.

---

*Report:*

```
exact files changed
  docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md  (new, PROPOSED)
git status --short
  ?? docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md
  ?? docs/REVIEWS/  (pre-existing, contains MVP_PLAN_ADVERSARIAL_REVIEW.md and authority-target spec reviews, not part of this change)
whether any implementation file changed
  No — no Go, SQL, Lean, MCP, REST, or ADRs changed; no schema; no tables; authority-target-and-debt-discharge-spec.md untouched
```

Only created: `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md`. No other changes. Do not commit. Do not sync.
