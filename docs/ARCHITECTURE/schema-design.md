# Solvent Production Schema Design

## 1. Header & Scope

**Status: PROPOSED — NOT IMPLEMENTED**

This document is the production relational schema design for Solvent's MVP authority model. It is the design artifact that precedes DDL. It is intentionally precise about durable facts, relationships, keys, constraints, lifecycle, transaction boundaries, migration behavior, and enforcement ownership, while producing **no DDL**.

### Authoritative inputs

- `docs/ARCHITECTURE/schema-readiness-decision.md`
- `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`
- `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`
- `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md`
- `docs/ADR/0001-authority-and-attestation.md`
- `docs/POST_HACKATHON_ARCHITECTURE.md`
- `docs/FOUR_VERB.md`
- `docs/REVIEWS/SCHEMA_DESIGN_ADVERSARIAL_REVIEW_FINAL.md`

Repository truth wins over all proposed documents. Current repository claims in this document are grounded in the reviewed repository evidence cited by the preceding architecture and adversarial-review artifacts; DDL generation must re-verify those claims against the actual checkout.

### Out of scope

No DDL, Go, SQL, Lean, MCP, REST, ADR, or implementation changes are part of this artifact. No commit or sync is part of this phase.

---

## 2. Method

The design is derived in this order:

```text
Durable facts F1–F7
        ↓
security-sensitive relationships
        ↓
forbidden / impossible states
        ↓
relational representation
        ↓
keys / FKs / CHECKs / uniqueness
        ↓
transaction boundaries
        ↓
migration semantics
```

The design does not start from a preferred table count.

### Invariant-status discipline

The only repository-earned invariants retained here are the already verified database/kernel properties: `promoted_is_debt_free`, `gate`, `live_requires_promoted`, `belief_id_status_key`, plus I-7 (`crdb.ExecuteTx`) and I-8 (`RetractCascade` atomicity) as established in the reviewed repository evidence.

No new `I-*` identifiers are created by this document. `I-12` remains a **PROPOSED source-spec label** for authorization-target matching. `I-13` remains **FUTURE / UNSCOPED** for execution-effect verification. Executor Mediation remains a **DOCUMENTED CONTRACT**, not a database invariant.

The durable-fact labels F1–F7 and decision labels C-1–C-7 are architectural document labels, not earned repository invariants.

---

## 3. Seven Decisions Resolved Inline

### C-1 — Snapshot-authoritative execution model

**Decision: PROPOSED — separate, insert-only `target_snapshot` is the sole execution-time authority.**

`authority_target` is the lifecycle/proposal identity. It carries only the minimum mutable lifecycle data needed to locate current authority state: tenant, state, snapshot reference, creation attribution, and lifecycle timestamps.

`target_snapshot` is an immutable approval representation containing:

- full five-tuple: principal, resource, scope, action, consequence;
- exact justification set, including `belief_id`, `promotion_epoch`, claim text, and belief status at approval;
- exact `policy_version_id`;
- approver `principal_id` and `principal_type` as approval-time attribution;
- approval timestamp.

The live `justification` table is audit/proposal history. It is **not** consulted as an execution-time authority source.

A post-approval justification change does not mutate existing authority. A new target/approval is required.

### C-1 physical immutability mechanism

The relational design uses three layers:

1. `target_snapshot` is insert-only by application design.
2. Authority lifecycle changes occur on `authority_target` (`proposed → active → revoked`); approval data is never rewritten in place.
3. The application DB role receives `SELECT`/`INSERT` access needed for operation but no `UPDATE` or `DELETE` privilege on `target_snapshot`. The exact privilege syntax must be verified against the pinned CockroachDB version during DDL implementation; the security property is fixed here.

This is a deployment/schema mechanism, not an invented CockroachDB feature. Administrative ownership may retain controlled maintenance access outside the application role, but that role is outside normal authorization paths.

### C-2 — Belief re-promotion

**Decision: `promotion_epoch` is the authorization-relevant promotion identity.**

The future relational model adds an additive `belief.promotion_epoch` concept:

- `0` while the belief has not yet been promoted;
- incremented for every successful promotion;
- a retracted belief that is later promoted receives a new epoch.

A `justification` references `(belief_id, promotion_epoch)`, not merely `(belief_id, status)`.

This is the smallest durable mechanism that distinguishes:

```text
belief X / promotion #1
        ≠
belief X / promotion #2
```

after retract → re-promote, without event sourcing.

The approval snapshot copies the exact epoch as well. The snapshot remains the execution-time authority; the epoch prevents stale justification from silently becoming valid again.

### C-3 — Discharge uniqueness and instrument replay

**Decision: global-per-obligation replay protection keyed by durable instrument identity.**

An accepted discharge is unique on:

```text
(tenant_id, obligation identity, instrument_ref)
```

where obligation identity is resolved by C-4 as `(obligation_key, policy_version_id)`.

The rule prevents the same instrument from discharging the same semantic obligation more than once, including across different beliefs. This is intentionally stronger than per-belief uniqueness.

`instrument_ref` is an opaque, durable instrument identifier whose authenticity and eligibility are verified by the service. It may refer to current belief-owned evidence or an external attestation identifier in the transitional model; the future Artifact/Citation model may replace the reference target without changing the replay principle.

### C-4 — Obligation identity

**Decision: stable `obligation_key` + bound `policy_version_id`.**

The same `obligation_key` can exist in multiple policy versions, but the obligation meaning is bound to the policy version under which it was evaluated.

Therefore:

```text
needReview + policy v1
        ≠
needReview + policy v2
```

unless the policy model explicitly declares semantic equivalence.

`debt_discharge` records both the stable key and the policy-version binding. A v1 discharge does not silently satisfy a semantically different v2 obligation.

### C-5 — Policy version ordering

**Decision: per-tenant monotonic `revision`, allocated transactionally; `effective_at` is metadata.**

For each tenant, a new policy version receives a strictly increasing `revision` value. Allocation is serialized transactionally under CockroachDB's serializable execution; concurrent attempts that conflict are retried (`40001`) and re-read before succeeding.

`effective_at` is retained as descriptive/effective metadata and is not the authority ordering primitive.

Policy rows are append-only from the application role. The application must not UPDATE or DELETE an existing policy version. Approval binds to the exact `policy_version_id` and revision observed at approval.

A later policy may restrict authority; it may never broaden an already approved authority.

### C-6 — Belief tenant migration

**Decision: `belief_tenant` association object; frozen `belief` table remains untouched.**

`belief_tenant` is keyed by `(tenant_id, belief_id)` and has `UNIQUE(belief_id)` for MVP, making the mapping one-to-one and preventing a belief from being simultaneously attached to multiple tenants.

All new authority relationships that cross into the frozen belief model reference the association using tenant-aware composite FKs.

Backfill is explicit:

```text
legacy scenario_id
        ↓
explicit scenario → tenant mapping
        ↓
belief_tenant(tenant_id, belief_id)
```

`scenario_id` is never treated as an authenticated tenant identity.

### C-7 — Revocation-time attestation validity

**Decision: revocation is forward-looking for principal authorization; attestation validity is issuance-time bounded.**

A principal's `revoked_at` blocks new approvals after revocation.

For attestation-backed discharge, an attestation is acceptable with respect to principal revocation only when its trusted issuance time satisfies:

```text
issued_at <= revoked_at
```

for a revoked principal. An attestation issued after revocation is refused.

This deliberately does **not** claim that a credential compromise can automatically invalidate every earlier attestation. Credential/key compromise invalidation remains deferred because Solvent does not own credential lifecycle or key management.

Replay is handled separately by C-3.

---

## 4. Durable-Fact Derivation

| Durable Fact | Required? | Owner | Representation | Relationships | DB Enforcement | Service Enforcement | Why Durable |
|---|---|---|---|---|---|---|---|
| F1 principal identity | Yes | Identity + Service | `principal` | principal → target/discharge/policy | PK/FKs; stable identity | authentication/role authorization | historical attribution |
| F1 principal type | Yes | Identity + Service | `principal.principal_type` | referenced by approval/discharge | CHECK/domain; update restricted by security boundary | role semantics | approval separation |
| F1 principal revocation | Yes | Service + Identity | `principal.revoked_at` | approvals/discharges | NOT NULL when set; referenced in checks/queries | forward-looking authorization | prevent new use |
| F1 issuer reference | Yes | Identity | `principal.issuer` | principal attribution | NOT NULL | issuer authenticity | trace assertion source |
| F1 credential lifecycle | No in MVP | IdP | external | outside schema | none | IdP | avoid IAM scope |
| F2 tenant binding | Yes | DB + Service | `tenant_id` on authority objects + `belief_tenant` | composite FKs | composite FK/UNIQUE | tenant authorization | cross-tenant safety |
| F2 authority tuple | Yes | Service + DB | snapshot tuple | principal/resource/scope/action/consequence | NOT NULL; exact representation | semantic interpretation | exact authority |
| F2 target lifecycle | Yes | Kernel + DB | `authority_target.state` | target → snapshot | state domain/check | legal transition | current authority status |
| F2 target identity | Yes | DB | `authority_target.target_id` | target → snapshot | PK/FK | lookup | stable authority reference |
| F3 justification relationship | Yes | Service + DB | `justification` | target ↔ belief_tenant | composite FKs | AND set evaluation | explicit authorization basis |
| F3 promotion identity | Yes | Kernel + DB | `belief.promotion_epoch` concept + justification FK | belief → promotion | composite reference | promotion semantics | prevent stale revalidation |
| F3 approval justification snapshot | Yes | Service + DB | `target_snapshot.justification_set` | snapshot contains exact references | immutable snapshot | verification | freeze approval meaning |
| F4 obligation identity | Yes | Policy + Service | `obligation_key`, `policy_version_id` | discharge → policy | NOT NULL/FK | qualification | prevent obligation drift |
| F4 discharge identity | Yes | Kernel + DB | discharge PK | discharge → belief/principal | PK/FKs | attribution | durable audit |
| F4 instrument identity | Yes | Service | `instrument_ref` | discharge → instrument semantics | UNIQUE with obligation | authenticity/replay | replay prevention |
| F4 debt state | Yes | Kernel + DB | existing `belief.debt[]` + discharge ledger | discharge ↔ debt entry | transitional support | atomic discharge | preserve current promotion invariant |
| F4 discharge attribution | Yes | Identity + Service | `discharged_by` | → principal | FK | actor authorization | who discharged |
| F5 policy identity | Yes | Policy + DB | `policy_version_id` | target snapshot → policy | PK/FK | policy semantics | historical reproducibility |
| F5 policy revision | Yes | DB + Service | `(tenant_id, revision)` | ordered policy history | UNIQUE | transactional allocation | deterministic ordering |
| F5 policy content integrity | Yes | Service | `policy_hash` | policy row | NOT NULL | canonicalization/hash verification | identify exact content |
| F5 approval binding | Yes | Service + DB | `policy_version_id` in snapshot | snapshot → policy | FK | version compatibility | freeze policy context |
| F6 approval time | Yes | Service + DB | `approved_at` | snapshot attribution | NOT NULL | time semantics | historical ordering |
| F7 tenant-aware belief relation | Yes | DB | `belief_tenant` | tenant ↔ frozen belief | composite FK + UNIQUE(belief_id) | tenant authorization | structural isolation |
| Execution-time authority source | Yes | Service + DB | `target_snapshot` | target → snapshot | FK + immutable privileges | exact verification | single authority source |
| Execution-effect claim | No in MVP | Executor | caller/system state only | outside schema guarantee | none | none | avoid false execution proof |

---

## 5. Relational Model

### 5.1 `principal`

**Purpose:** stable actor identity and attribution reference.

**Primary key:** `principal_id`.

**Candidate natural identity:** externally resolved identity reference governed by the IdP; not used as Solvent's primary key.

**Fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `principal_id` | NOT NULL | stable Solvent principal identity |
| `principal_type` | NOT NULL | `human`, `agent`, `workload`, or `service` |
| `issuer` | NOT NULL | asserting IdP/workload authority |
| `tenant_id` | NOT NULL | owning tenant |
| `revoked_at` | NULL | forward-looking revocation timestamp |
| `created_at` | NOT NULL | attribution/history |

**Keys/constraints:**

- PK on `principal_id`.
- CHECK/domain for permitted principal types.
- Composite tenant-aware references from authority objects must carry the tenant relation.
- Identity fields (`principal_id`, `principal_type`, `issuer`, `tenant_id`) are immutable after creation at the application security boundary; `revoked_at` is the intended mutable lifecycle field.

**Lifecycle:** create → active → revoked. Revocation does not erase history.

**Authority meaning:** identity attribution and approval-role input.

**Non-meaning:** credentials, authentication secrets, delegation graph, or proof that the principal is permitted to perform every role.

**Type transitions:** MVP treats `principal_type` as immutable. A legitimate type change is modeled as a new principal identity with explicit external/historical linkage rather than duplicating one stable `principal_id` across rows. A richer type-history model is deferred unless a real customer case requires it.

---

### 5.2 `authority_target`

**Purpose:** lifecycle/proposal identity for a possible authorization.

**Primary key:** `target_id`.

**Fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `target_id` | NOT NULL | stable target identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `state` | NOT NULL | `proposed`, `active`, `revoked` |
| `snapshot_id` | NULL until active | approved snapshot FK |
| `created_by` | NOT NULL | proposing principal |
| `created_at` | NOT NULL | history |
| `revoked_at` | NULL | lifecycle history |

No execution-time authority tuple is stored in mutable columns on this row. The tuple lives in `target_snapshot`.

**Lifecycle:**

```text
proposed → active → revoked
```

No `expired` state in MVP.

**Constraints:**

- An active target must reference exactly one snapshot.
- A proposed target may not be used as execution authority.
- Revocation does not delete the row.
- Tuple changes require a new target/snapshot/approval sequence.

**Authority meaning:** lifecycle pointer to authority state.

**Non-meaning:** not itself the complete execution-time authorization representation.

---

### 5.3 `target_snapshot`

**Purpose:** immutable execution-time authority representation.

**Primary key:** `snapshot_id`.

**Fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `snapshot_id` | NOT NULL | immutable snapshot identity |
| `target_id` | NOT NULL | parent target |
| `tenant_id` | NOT NULL | tenant boundary |
| `principal_id` | NOT NULL | authorized principal |
| `resource_type` | NOT NULL | opaque resource type |
| `resource_id` | NOT NULL | opaque resource identifier |
| `scope` | NOT NULL | opaque scope representation; exact-equality at kernel boundary |
| `action_namespace` | NOT NULL | exact action namespace |
| `action_name` | NOT NULL | exact action name |
| `consequence_type` | NOT NULL | consequence descriptor type |
| `consequence_parameters` | NOT NULL | canonicalized descriptor payload |
| `justification_set` | NOT NULL | immutable serialized set of approved justification records |
| `policy_version_id` | NOT NULL | exact approval policy |
| `approver_principal_id` | NOT NULL | approval actor |
| `approver_principal_type` | NOT NULL | approval-time attribution snapshot |
| `approved_at` | NOT NULL | approval time |
| `snapshot_hash` | NOT NULL | canonical content digest for detection/audit, not authority by itself |

`justification_set` is an immutable serialized approval snapshot containing, for each approved justification, at least:

```text
belief_id
promotion_epoch
claim
status_at_approval
```

This deliberate denormalization is a **historical snapshot**, not a second live authorization graph. The live `justification` table remains separately queryable as proposal/audit history.

**Immutability:** application role has no UPDATE/DELETE privilege; no mutation path is used by the authorization service. The exact privilege statements belong to DDL, not this design.

**Authority meaning:** sole execution-time authority representation.

**Non-meaning:** proof that the executor produced the real-world effect.

---

### 5.4 `justification`

**Purpose:** explicit proposal/history relation between a target and a promotion epoch of a belief.

**Primary key:** implementation-chosen stable row ID or deterministic composite identity; the security requirement is the relationship below.

**Required fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `justification_id` | NOT NULL | relationship identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `target_id` | NOT NULL | target reference |
| `belief_id` | NOT NULL | frozen belief reference through `belief_tenant` |
| `promotion_epoch` | NOT NULL | exact promotion occurrence |
| `attached_by` | NOT NULL | attaching principal |
| `attached_at` | NOT NULL | history |

The live link has no authority-granting `accepted` state. Acceptance is represented by the immutable snapshot.

**Relationships:**

```text
(tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)
(target_id, tenant_id) → authority_target(target_id, tenant_id)
```

and the promotion reference is conceptually bound to the `belief_id + promotion_epoch` pair.

**Authority meaning:** candidate/audit justification.

**Non-meaning:** active permission.

---

### 5.5 `debt_discharge`

**Purpose:** durable attribution and replay-controlled discharge history.

**Primary key:** `discharge_id`.

**Fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `discharge_id` | NOT NULL | discharge identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `belief_id` | NOT NULL | discharged belief |
| `obligation_key` | NOT NULL | stable obligation identifier |
| `policy_version_id` | NOT NULL | obligation meaning/version binding |
| `instrument_ref` | NOT NULL | durable discharge instrument identity |
| `discharged_by` | NOT NULL | asserting principal |
| `issued_at` | NULL | attestation issuance time when applicable |
| `accepted_at` | NOT NULL | discharge acceptance time |
| `status` | NOT NULL | `accepted`, `retracted` if future revocation semantics require it; accepted is the MVP qualifying state |
| `integrity_reference` | NULL | verification metadata / attestation identifier |

**Uniqueness:**

```text
UNIQUE (
    tenant_id,
    obligation_key,
    policy_version_id,
    instrument_ref
)
WHERE status = 'accepted'
```

This is the global-per-obligation instrument replay rule. It intentionally prevents the same instrument from being accepted for the same semantic obligation under two different beliefs in the same tenant.

**Relationships:**

- `belief_id` → `belief_tenant` via tenant-aware FK.
- `discharged_by` → `principal`.
- `policy_version_id` → `policy_version`.

**Lifecycle:** proposed/verified externally → accepted → optional future retraction model. No destructive deletion of accepted history.

**Authority meaning:** durable evidence that a particular obligation was discharged and by whom.

**Non-meaning:** proof that the underlying real-world claim is true.

---

### 5.6 `policy_version`

**Purpose:** immutable, tenant-bound policy identity and historical ordering.

**Primary key:** `(tenant_id, policy_version_id)`.

**Required fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `tenant_id` | NOT NULL | policy owner |
| `policy_version_id` | NOT NULL | immutable version identity |
| `revision` | NOT NULL | strictly increasing per-tenant revision |
| `policy_hash` | NOT NULL | content integrity identifier |
| `policy_content` | NOT NULL | policy representation required for historical interpretation |
| `effective_at` | NOT NULL | metadata; not authoritative ordering |
| `author_principal_id` | NOT NULL | attribution |
| `created_at` | NOT NULL | history |

**Constraints:**

- UNIQUE `(tenant_id, revision)`.
- FK `author_principal_id` → principal within tenant semantics.
- Application role cannot UPDATE/DELETE existing policy versions.
- `effective_at` cannot be used as the sole ordering mechanism.

**Authority meaning:** historical policy context.

**Non-meaning:** proof that the author was authorized to publish policy; that is Service/Policy/Identity authorization.

---

### 5.7 `belief_tenant`

**Purpose:** bridge the frozen belief model into future tenant-aware authority relationships without modifying `belief`.

**Primary key:** `(tenant_id, belief_id)`.

**Fields:**

| Field | Nullability | Meaning |
|---|---|---|
| `tenant_id` | NOT NULL | future tenant identity |
| `belief_id` | NOT NULL | frozen belief identity |
| `mapped_at` | NOT NULL | migration/audit time |

**Uniqueness:** `UNIQUE(belief_id)` in MVP.

**Lifecycle:** backfilled once per belief; mapping is immutable.

**Non-meaning:** authentication of the tenant. Tenant identity comes from the service/identity boundary.

---

### Candidate proof

The physical candidate is therefore:

```text
1. principal
2. authority_target
3. target_snapshot
4. justification
5. debt_discharge
6. policy_version
7. belief_tenant
```

The five logical core authority objects remain:

```text
principal
authority_target
justification
debt_discharge
policy_version
```

`target_snapshot` is required by C-1 and `belief_tenant` by C-6. No other object is required by the current durable-fact derivation.

An additional object may be introduced only if DDL derivation proves that one of these objects cannot enforce the stated durable fact without a second source of truth.

---

## 6. Relationship Map & Security Interrogation

```text
principal
   │
   ├──────────────→ authority_target
   │                     │
   │                     └────────→ target_snapshot
   │
   ├──────────────→ debt_discharge
   │
   └──────────────→ policy_version (author attribution)

belief ──→ belief_tenant ←── authority relationship
                         │
                         └────→ justification ───→ authority_target

authority_target ───────→ target_snapshot ───────→ policy_version
```

### Interrogation matrix

| Relationship | Wrong tenant blocked? | Wrong principal blocked? | Stale belief blocked? | Proposal→authority blocked? | Mutable→approved blocked? |
|---|---|---|---|---|---|
| target → snapshot | Composite tenant FK | snapshot principal is immutable | snapshot freezes epoch | only active target points to snapshot | snapshot immutable |
| justification → target | composite tenant FK | attached-by is attributed; authority principal is separate | promotion_epoch binds state | justification alone cannot activate | snapshot is authority |
| justification → belief_tenant | yes | N/A | epoch required | yes | yes through snapshot |
| discharge → belief_tenant | yes | discharged_by FK | obligation/discharge evaluated against current state + exact instrument | no direct activation semantics | immutable discharge history |
| discharge → policy_version | yes | N/A | exact policy binding | N/A | policy immutable |
| snapshot → principal | yes | exact principal ID | approval attribution frozen | only active snapshot is authority | snapshot immutable |
| snapshot → approver | yes | exact approval identity | approval-time type copied for historical meaning | approval only through service rules | immutable |

The schema must never rely solely on the existence of an FK to prove that the referenced principal was *authorized* to perform the operation. Attribution and authorization remain distinct.

---

## 7. Impossible States

| Forbidden state | DB | Transaction | Service | External | Residual |
|---|---|---|---|---|---|
| promoted belief + non-empty debt | Existing `promoted_is_debt_free` | existing promotion transaction | N1–N4 | — | none for this invariant |
| live intent + non-promoted belief | existing `gate` + `live_requires_promoted` | cancel-before-retract | — | — | none under current model |
| active target + mutable authority tuple | snapshot has no app UPDATE/DELETE; target stores no mutable execution tuple | approval transition binds snapshot | refuses new authority by target mutation | DB admin outside application role | exact privilege model validated at DDL |
| active target + changed justification set | execution reads snapshot only | snapshot creation and activation atomic | new link requires new proposal/approval | — | audit links may differ without changing authority |
| active target + revoked approver | target/snapshot retain approval identity; approval checks current principal state | approve transaction | revoked principal cannot newly approve | IdP authenticity | historical approval remains attributed |
| cross-tenant justification | composite FK through `belief_tenant` | relationship insert atomic | tenant authorization | identity tenant assertion | existing core migration |
| discharge without debt removal | cannot correlate array and ledger directly | one `ExecuteTx` for discharge + `array_remove` | service requires exact operation | — | kernel bug remains TCB failure |
| duplicate accepted instrument for same obligation | composite UNIQUE partial rule | serialization/retry | service verifies instrument | source attestation system | broader artifact replay deferred |
| v1 discharge satisfies semantically different v2 obligation | policy FK + policy-bound obligation identity | discharge transaction | qualification uses bound version | — | explicit equivalence only |
| policy revision regression | UNIQUE tenant/revision | serialized allocation | reject stale revision | — | service bug can still attempt rejected write |
| approval under invalid policy | FK to immutable policy | approval binds exact version | validates policy eligibility | — | compromised service remains TCB risk |
| post-revocation attestation accepted | `issued_at`/revoked relation represented in discharge fields where applicable | discharge validation transaction | refuses if issuance is after revocation | IdP/attestation authenticity | compromise invalidation deferred |
| NULL resource interpreted as wildcard | NOT NULL on snapshot resource fields | — | missing legacy dimensions refuse authorization | — | none |
| NULL scope interpreted as wildcard | scope representation is NOT NULL | — | opaque semantics; no wildcard interpretation | — | scope ontology deferred |
| missing consequence interpreted as wildcard | NOT NULL | — | fail closed | — | none |
| agent self-approves HIGH_IMPACT | principal attribution + service approval check | approval tx | policy floor | IdP authenticity | fully compromised service outside DB guarantee |

---

## 8. Snapshot Authority

There are three distinct states:

```text
Proposal
    = mutable candidate target + live justification/audit relations

Approved snapshot
    = immutable exact authority representation

Current authority
    = active target whose snapshot is the current execution-time source
```

### Single execution-time source

Execution authorization reads:

```text
authority_target.state
        +
target_snapshot
```

and **does not** reconstruct authorization from mutable `authority_target` fields or the live `justification` set.

### Post-approval retraction

A previously approved snapshot remains historically valid. However, authorization verification must re-evaluate the snapshot against current revocation/reassessment rules before use, consistent with the service-layer re-verification requirement.

Therefore:

```text
belief retraction
    ≠ automatic mutation of snapshot

belief retraction
    → reassessment/re-verification
    → DENY / REVOKE when the current authorization policy requires it
```

The snapshot remains the record of what was approved; current verification determines whether that approval remains usable.

### Why live justification does not control authority

A post-approval link addition is an audit/proposal event. It cannot enlarge existing authority because execution never reads the live set as the authoritative set.

---

## 9. Belief / Promotion Model

The model distinguishes:

```text
belief_id
    = enduring belief identity

promotion status
    = current lifecycle state

promotion_epoch
    = authorization-relevant occurrence of promotion

justification reference
    = belief_id + promotion_epoch
```

Example:

```text
belief X / epoch 1
    ↓
retract
    ↓
belief X / epoch 2
```

A justification referring to epoch 1 does not become a justification for epoch 2.

### Epoch semantics

- Entered state starts at epoch 0.
- Every successful promotion increments the epoch exactly once.
- Retraction does not increment the epoch.
- A later promotion increments it again.
- Promotion and epoch update occur in the same transaction as the promotion state change.

The design intentionally avoids event sourcing. The epoch is a compact durable discriminator, not a full history engine.

---

## 10. Debt / Discharge Model

The design separates:

```text
Obligation
    = what must be discharged

Discharge
    = durable record that an obligation was discharged

Instrument
    = the evidence/attestation identity used to support the discharge

Debt state
    = currently outstanding obligation representation
```

### Transitional current model

Current `belief.debt[]` remains the promotion-facing representation while the discharge ledger provides durable attribution and replay control.

The required transaction is:

```text
Record accepted discharge
        +
RetireDebt / array_remove
        +
commit
```

as one transaction.

No accepted discharge may commit without the corresponding debt retirement, and no debt retirement may commit without the discharge record.

### Long-term direction

The preferred eventual direction is:

```text
Obligation rows
    →
Discharge rows
    →
derived outstanding debt
```

The transition must preserve the semantic impossibility represented by `promoted_is_debt_free`.

### Replay

The same instrument cannot satisfy the same obligation twice within one tenant, even for different beliefs. Cross-obligation instrument reuse is a separate semantic question and is not automatically prohibited unless the instrument class itself is defined as single-use.

---

## 11. Policy, Principal, and Tenant Models

### Policy

A policy version is immutable and tenant-bound. `revision` is the authoritative per-tenant ordering. `effective_at` describes intended/effective time but does not establish ordering.

`author_principal` records attribution only. Whether that principal is authorized to publish policy is a Service/Policy/Identity decision.

### Principal

`principal_id` is stable. Credential rotation does not create a new principal identity. The IdP/workload attestation system owns credentials and cryptographic keys.

`principal_type` is immutable for a principal identity. MVP does not attempt to version type transitions.

### Tenant

Tenant identity is asserted outside the data model; tenant bindings are structurally preserved inside the data model.

`belief_tenant` is one-to-one with belief for MVP. No belief may belong to two tenants through this association.

---

## 12. Scope / Resource / Action / Consequence

### Resource

```text
resource_type
resource_id
```

Both are opaque and compared by exact equality. Solvent does not invent a universal resource hierarchy.

### Scope

Scope is opaque data with no implicit wildcard semantics. Missing/NULL scope is never treated as unrestricted authority. The schema stores the representation; the service owns any future narrowing relation.

For MVP, the recommended representation is a canonical JSON value or text blob with deterministic byte equality. The exact SQL type is an implementation decision and must not change the semantic rule.

### Action

```text
action_namespace
action_name
```

Values are preserved verbatim. No automatic lowercase, alias, or hidden canonicalization is introduced by the schema.

### Consequence

```text
consequence_type
consequence_parameters
```

The representation is exact. The schema does not claim byte equality proves semantic equivalence to the executor.

No DEFAULT consequence is permitted. Missing consequence is an authorization failure.

---

## 13. Authorization Decision / Warrant

### Authorization decision

MVP does not require a separate `authorization_decision` table.

The current authority representation is:

```text
active authority_target
        +
immutable target_snapshot
```

This combination provides:

- lifecycle state;
- exact approved tuple;
- exact justification snapshot;
- exact policy binding;
- exact approver attribution.

Creating a separate decision object would duplicate authority unless a future durable fact proves it necessary.

### Warrant

No persistent warrant table in MVP.

A future portable warrant may reference:

```text
target_id
snapshot_id
policy_version_id
issued_at
```

but it does not become a second authority source.

`action_intent` must not be extended into a de facto warrant store.

---

## 14. Evidence

Current evidence is belief-owned. The production schema may temporarily reference current evidence rows through `instrument_ref` or an external attestation identifier, but the schema does not claim that the caller-supplied `content_sha256` is verified by Solvent.

The future Artifact/Citation model remains a migration target:

```text
Artifact = what was observed
Citation = why that artifact is relevant to this belief
```

The discharge schema must not prevent that future split.

### Evidence sharing

The same underlying artifact may eventually support multiple beliefs, but those beliefs do not share the same citation record. Transitional belief-owned evidence references remain historical references and must not be mistaken for reusable artifact identity.

### Provenance

The schema preserves attribution and integrity references. It does not prove external truth.

---

## 15. Concurrency Matrix

| Race | Serialized State | Transaction | Winner / Loser | Failure Code | Re-evaluation |
|---|---|---|---|---|---|
| Approve vs Approve | same target state/snapshot activation | one `ExecuteTx` | one activates; loser retries/re-reads and returns REVIEW | `40001` on serialization | yes |
| Approve vs Attach | target approval vs new justification | separate tx; approve reads committed set then snapshots | whichever commits first is reflected; later attach cannot mutate snapshot | `40001` as applicable | later attach requires new approval |
| Approve vs Retract | belief promotion state | transactional state change | committed retract causes approval refusal | `40001` or semantic refusal | yes |
| Approve vs Policy Change | approval policy binding vs policy revision | approval binds exact `policy_version_id` in tx | approved snapshot binds version seen by transaction | `40001` on conflict | service re-evaluates current policy |
| Discharge vs Promote | belief debt state | discharge + `array_remove` in one tx; promote separate tx | promotion sees committed debt state | `40001` as applicable | yes |
| Discharge vs Discharge | accepted instrument/obligation uniqueness | insert under UNIQUE; retry on serialization | one accepted row wins; duplicate rejected/re-read | unique violation / `40001` | no second acceptance |
| Revocation vs Approve | principal revocation timestamp | both transactional | whichever serial order commits; approval must refuse if revocation committed first | `40001` or semantic refusal | yes |
| Target mutation vs Approve | snapshot immutability | mutation denied by privilege/security boundary | approve sees immutable snapshot | permission failure | no mutation path |
| Snapshot creation vs Approval | snapshot existence + target activation | same approval transaction | activation cannot commit without snapshot FK | FK/`40001` as applicable | retry/re-read |
| Tenant binding vs relationship creation | `belief_tenant` mapping | mapping must exist before relationship | relationship fails if mapping absent | FK violation | service maps tenant first |

`crdb.ExecuteTx` remains the required transaction wrapper for kernel write paths. Serializable isolation is used as the concurrency mechanism, but semantic conflicts are defined explicitly rather than assumed away.

---

## 16. Migration Design

The legacy model has:

```text
action_intent.action TEXT
```

The production authority model requires:

```text
principal
resource
scope
action
consequence
policy
```

### Fail-closed rules

Missing any required authority dimension means:

```text
NOT AUTHORIZED
```

Never:

```text
NULL → wildcard
missing → '*'
missing resource → all resources
missing scope → unrestricted
missing consequence → arbitrary consequence
missing principal → anonymous authority
missing tenant → global authority
```

An old action string such as `update_record` is not silently translated into a default namespace, default resource, or default consequence.

Old and new semantics must not share one authorization path unless the old intent has been explicitly re-authorized under the new tuple.

### Belief tenant backfill

For each existing belief, an explicit fixed mapping converts legacy `scenario_id` to the designated legacy tenant. This creates exactly one `belief_tenant` row per belief.

No inference from arbitrary runtime values is permitted.

### Rollback

Rollback must not restore authority by interpreting newly required dimensions as optional. Legacy rows remain non-authorizing until explicit re-authorization.

No migration SQL is specified here.

---

## 17. DocTrust Flows

DocTrust is the first reference implementation and must exercise Solvent only through public interfaces.

### Minimum flows

1. Create a proposed target.
2. Attach one or more justifications.
3. Request authorization.
4. Approve a valid target and inspect its immutable snapshot.
5. Present an exact tuple and receive ALLOW.
6. Present a tuple mismatch and receive DENY.
7. Retract a supporting belief and verify re-evaluation/revocation behavior.
8. Attempt duplicate instrument discharge and verify rejection.
9. Revoke a principal and verify new approval is refused.
10. Change policy and verify downgrade cannot broaden previously approved authority.
11. Attempt a cross-tenant justification and verify structural rejection.

### What DocTrust proves

It proves the public integration exercises the model as intended for the DocTrust domain.

### What DocTrust does not prove

- domain-agnostic completeness;
- external IdP correctness;
- semantic truth of the document evidence;
- execution-effect verification;
- general authority-composition safety;
- full multi-tenant production readiness;
- correctness of every future executor integration.

---

## 18. Formal Model Mapping

No Lean changes occur in this phase.

| Schema property | Formal status |
|---|---|
| promoted implies debt-free | CURRENT formal/repository area, subject to existing model scope |
| live intent implies promoted | CURRENT formal/repository area |
| retract cascade atomicity | CURRENT repository/kernel property; future formal strengthening possible |
| promotion epoch prevents stale re-promotion | FUTURE formal property |
| snapshot is sole execution authority | FUTURE formal property |
| snapshot immutability | schema/service property; future formalization |
| tenant-aware relationship integrity | schema property; future formalization |
| discharge replay uniqueness | schema/service property; future formalization |
| policy monotonic revision | schema/service property; future formalization |
| authorization-target matching | PROPOSED source-spec property, not earned invariant |
| execution-effect matching | FUTURE / UNSCOPED |

No claim here that current Lean proves the new production schema.

---

## 19. Schema Security Claims

| Claim | Primary enforcement | Secondary enforcement | Status |
|---|---|---|---|
| Belief promotion is debt-free | DB | Kernel/service | CURRENT |
| Live intent requires promotion | DB | Kernel | CURRENT |
| Active authority uses exact approved snapshot | DB + Service | Kernel | PROPOSED |
| Snapshot cannot be mutated by application | DB privilege boundary | Service | PROPOSED |
| Justification is explicit | DB FKs | Service | PROPOSED |
| Re-promotion cannot revive old authorization reference | DB relationship using epoch | Service | PROPOSED |
| Discharge instrument replay is prevented | DB UNIQUE | Service | PROPOSED |
| Obligation meaning is policy-version bound | DB FK | Service | PROPOSED |
| Policy revisions are monotonic per tenant | DB UNIQUE + transactional allocation | Service | PROPOSED |
| Cross-tenant authority relationships are impossible | DB composite FKs | Service | PROPOSED |
| Revoked principal cannot approve new authority | Service + DB state reference | Identity | PROPOSED |
| Post-revocation attestation issuance is refused | Service using issuance/revocation relation | Identity | PROPOSED |
| Missing dimensions never become wildcard | DB NOT NULL + Service refusal | — | PROPOSED |
| Execution tuple matches authorized tuple | Service exact equality | Executor echo contract | PROPOSED (I-12 label) |
| Executor actually produced world effect | — | future trusted executor/receipt | FUTURE / UNSCOPED (I-13 label) |
| Consequential executor has no unmediated path | Deployment/integration | auditing | DOCUMENTED CONTRACT |
| Credential lifecycle is controlled by Solvent | — | IdP | FALSE / OUT OF SCOPE |
| Semantic truth of evidence is guaranteed by DB | — | provenance/process | FALSE / OUT OF SCOPE |

### Executor Mediation contract

> A consequential action governed by Solvent must not have an unmediated execution path around the authorized executor.

This is a deployment precondition, not a kernel/DB invariant.

---

## 20. Tradeoffs

| Decision | Alternative | Selection | Reason |
|---|---|---|---|
| Snapshot table vs inline | inline snapshot columns | separate immutable `target_snapshot` | one execution-time source; stronger privilege boundary |
| Promotion epoch vs status-only FK | `(belief_id,status)` | `promotion_epoch` | distinguishes retract→re-promote without event sourcing |
| Instrument uniqueness | per-belief | global-per-obligation + instrument | blocks cross-belief replay |
| Obligation identity | version-qualified key | stable key + policy binding | preserves stable vocabulary while making policy meaning explicit |
| Policy ordering | `effective_at` only | monotonic per-tenant `revision` | deterministic concurrency and downgrade comparison |
| Tenant binding | add `tenant_id` to frozen belief | `belief_tenant` | preserves frozen core and structural composite-FK boundary |
| Principal type transitions | mutable type history | immutable principal type for MVP | avoids identity ambiguity; revisit with customer evidence |
| Evidence reference | current evidence FK | transitional opaque instrument reference | leaves Artifact/Citation migration possible; avoids false provenance completeness |

### Rejected complexity

Not part of MVP schema:

- credential/key table;
- full IAM/role graph;
- obligation event-sourcing engine;
- execution receipt system;
- authority-composition graph;
- reachability engine;
- temporal authority engine;
- persistent Warrant table;
- full tenant service.

---

## 21. DDL Readiness

**NOT READY FOR DDL.**

The schema semantics are sufficiently concrete to define the relational model, but seven physical/semantic conditions remain required before DDL can be safely emitted:

1. **C-1:** validate the exact CockroachDB privilege/enforcement mechanism that makes `target_snapshot` insert-only for the application role.
2. **C-2:** confirm the DDL representation of `promotion_epoch` on the frozen-core migration and its composite relationship to justification.
3. **C-3:** finalize the canonical durable `instrument_ref` identity and the exact partial-UNIQUE implementation.
4. **C-4:** finalize the exact obligation/policy foreign-key and uniqueness representation.
5. **C-5:** finalize transactional revision allocation and its CockroachDB retry behavior.
6. **C-6:** finalize the `belief_tenant` composite-FK graph without modifying the frozen belief table.
7. **C-7:** finalize how trusted attestation issuance time is represented/validated at discharge.

These are implementation/DDL-gate conditions, not reasons to reopen the core architecture.

### Safety condition

DDL must not be generated if any of the seven conditions is resolved only by a prose promise that has no enforceable schema/service boundary.

---

## 22. Final Dogfood Test

### Evidence supporting the design

The design derives from the durable-fact set F1–F7 and explicitly incorporates the adversarial findings around snapshot authority, retract→re-promote, replay, obligation identity, policy ordering, tenant migration, and revocation-time attestation validity.

### Assumptions

- the Solvent service is inside the MVP TCB;
- Identity authenticates principals externally;
- policy semantics remain Service/Policy responsibilities;
- the application role can be privilege-separated from DB administration;
- DocTrust can exercise public MCP/REST interfaces;
- the existing four-table core remains authoritative for current behavior until migration.

### Open debt

- exact DDL mechanics for snapshot immutability;
- final physical form of promotion epoch migration;
- instrument identity verification;
- exact policy revision allocation implementation;
- attestation validity representation;
- production tenant onboarding and legacy backfill execution;
- future artifact/citation model.

### Falsifiers

The design must be reassessed if any of these are demonstrated:

- a post-approval mutation can change execution authority without changing the snapshot;
- a stale promotion can silently become valid again;
- one accepted instrument can discharge the same obligation twice;
- a v1 discharge silently satisfies a materially different v2 obligation;
- policy revisions can regress or reorder under concurrency;
- cross-tenant references can be committed;
- a post-revocation attestation is accepted contrary to the defined issuance rule;
- DocTrust requires an additional durable fact not represented by the model.

### Reassessment triggers

- new customer workflow;
- new authority dimension;
- new executor class;
- evidence provenance requirement;
- delegated authority;
- real high-impact authorization case;
- observed migration ambiguity;
- CockroachDB behavior that invalidates an enforcement assumption.

### Dogfood conclusion

The schema design itself follows Solvent's method:

```text
Evidence
  ↓
Belief
  ↓
Debt / obligation
  ↓
Discharge
  ↓
Promotion
  ↓
Explicit justification
  ↓
Immutable approval snapshot
  ↓
Authorization
```

A coherent schema is not treated as proof. Remaining uncertainty is explicitly assigned to schema, service, identity, policy, executor, or future work.

---

## 23. Validation

Before DDL authorization, perform the following validation against the actual repository and pinned CockroachDB version:

1. Re-read the complete schema-design document.
2. Re-read `schema-readiness-decision.md`.
3. Re-read the final adversarial schema review.
4. Verify every CURRENT claim against `db/001–004` and the relevant kernel/internal/MCP/Lean files.
5. Verify every PROPOSED claim is labeled.
6. Verify every OPEN item is genuinely unresolved and implementation-relevant.
7. Verify every DEFERRED concept has a promotion condition.
8. Verify no new I-* invariant ID was invented.
9. Verify I-12 is described only as a proposed source-spec label.
10. Verify I-13 remains FUTURE / UNSCOPED.
11. Verify Executor Mediation remains DOCUMENTED CONTRACT.
12. Verify `target_snapshot` is consistently the sole execution-time authority.
13. Verify no live justification mutation can modify approved authority.
14. Verify `promotion_epoch` semantics are consistent everywhere.
15. Verify `belief_tenant` is one-to-one on `belief_id` for MVP.
16. Verify every authority relationship is tenant-aware.
17. Verify obligation identity always includes policy binding.
18. Verify replay uniqueness is instrument-aware.
19. Verify policy ordering uses revision, not `effective_at` alone.
20. Verify revocation and attestation issuance-time semantics are explicit.
21. Verify NULL/missing dimensions never imply wildcard authority.
22. Verify DocTrust flows exercise public interfaces only.
23. Verify no DDL was created as part of this design artifact.
24. Verify no Go/SQL/Lean/MCP/REST/ADR/architecture implementation file changed.
25. Verify `git status --short` in the actual repository.
26. Report exact files changed and confirm no commit or sync occurred.

**Design gate after validation:** `NOT READY FOR DDL` until C-1 through C-7 have explicit implementation-level exit tests. The next artifact is the focused DDL-gate resolution and/or DDL itself, not another general architecture rewrite.
