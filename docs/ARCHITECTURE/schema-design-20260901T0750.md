# Solvent Production Schema Design

## 1. Header & Scope

**Status: PROPOSED — NOT IMPLEMENTED**
**Revision 2 — full relational repair of the schema design against `docs/REVIEWS/SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW-20260901T0726.md` (Revision 2 of the implementation-gate review: 9 unique blocking SDAR findings, 7 normalized DDL conditions).** This document replaces the prior schema-design revision in full. It is the design artifact that precedes DDL. It produces **no DDL**.

### Authoritative inputs

- `docs/REVIEWS/SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW-20260901T0726.md` (authoritative attack report; SDAR-1..21)
- `docs/ARCHITECTURE/schema-readiness-decision.md`
- `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`
- `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`
- `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md`
- `docs/ADR/0001-authority-and-attestation.md`
- `docs/POST_HACKATHON_ARCHITECTURE.md`
- `docs/FOUR_VERB.md`

Repository truth wins over all proposed documents. The only repository-earned invariants are the SQL constraints `promoted_is_debt_free`, `gate`, `live_requires_promoted`, `belief_id_status_key` (`db/001_schema.sql`), plus I-7 (`crdb.ExecuteTx` discipline) and I-8 (`RetractCascade` atomicity). No new `I-*` identifiers are created here. `I-12` remains a **PROPOSED source-spec label**; `I-13` remains **FUTURE / UNSCOPED**; Executor Mediation remains a **DOCUMENTED CONTRACT**, not a database invariant.

### Superseded decisions (recorded so they do not resurface)

| Superseded | Superseded by | Reason |
|---|---|---|
| Additive `belief.promotion_epoch` column (frozen-core exception) | `belief_promotion` side table | Frozen `belief` core is preserved; the side table gives the epoch a real durable referent without touching `db/001_schema.sql`. |
| `authority_target.snapshot_id` as a (mutable) approval pointer | Append-only `target_activation` fact; the column is removed | A mutable authority-bearing pointer cannot be made non-repointable without triggers CockroachDB does not have. |
| Stored `authority_target.state` lifecycle mirror | Read-time derivation from facts (§5.2, §8) | A mutable mirror can drift from the facts and become a second authority source; the facts alone are authoritative. |
| Discharge `status` domain containing a speculative `retracted` value | No status column in MVP; all accepted discharges are immutable facts; revocation is FUTURE / UNSCOPED | A mutable status column reopens the replay the uniqueness rule exists to close. |
| `approver_principal_type` copy inside `target_snapshot` | Removed; I-A4 evaluation reads the principal row via tenant-composite FK | `principal_type` is immutable, so the copy is pure duplicated truth. |

### Out of scope

No DDL, Go, SQL, Lean, MCP, REST, ADR, review-artifact, or implementation changes are part of this artifact. No commit or sync is part of this phase.

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

The design does not start from a preferred table count. The repaired model has ten physical objects; the count is a consequence of the security properties, not a goal. The five logical core authority objects remain `principal`, `authority_target`, `justification`, `debt_discharge`, `policy_version`; `target_snapshot`, `target_activation`, `target_revocation`, `belief_promotion`, and `belief_tenant` are each required by a specific durable fact or blocking finding (C-1/SDAR-1/2/3/8/9, SDAR-4, C-3/SDAR-5, C-6/SDAR-10 respectively).

---

## 3. Seven Decisions — Implemented

### C-1 — Authority lifecycle, activation binding, and snapshot immutability

C-1 is scoped in this design as the **entire authority-lifecycle and activation boundary**, not merely snapshot-content immutability: the proposal representation (SDAR-2), the activation binding (SDAR-1), the snapshot key graph (SDAR-3), the approval-pin transaction semantics (SDAR-8), and lifecycle monotonicity (SDAR-9) are all parts of one model, because they all defend the same property — *active authority cannot silently mutate, and authority cannot exist or persist without an explicit approval fact*.

The authority resolution rule is, and only is:

```text
authority(target)
    = ∃ target_activation(tenant_id, target_id, …)
      ∧ ¬∃ target_revocation(tenant_id, target_id)
      ∧ the activation's snapshot row exists (FK-guaranteed)
```

- `authority_target` is the proposal/lifecycle identity. It carries the mutable proposal tuple (§5.2) and **no snapshot reference column and no lifecycle columns of any kind** — the pointer class of attack (SDAR-1) is eliminated by removing the pointer, and the mirror class of drift (second-source audit) is eliminated by removing the mirror.
- `target_snapshot` is the immutable approved authority representation: full five-tuple, exact justification set, `policy_version_id`, approver, approval timestamp.
- `target_activation` is the append-only activation fact. **An activation row is itself an authority-bearing immutable fact. It is not merely a cache or projection.** Its `UNIQUE(tenant_id, target_id)` applies over the target's **entire history** — "already-activated" means "ever activated", never "currently active". There is no `WHERE state='active'` predicate anywhere. A revoked target cannot obtain a second activation; new authority requires a new target identity and a new approval.
- `target_revocation` is the append-only revocation fact. Revocation narrows; nothing can delete it.
- Live `justification` rows and proposal columns are **never** execution authority. Execution MUST NOT reconstruct authority from the proposal columns after activation; the resolution rule reads activation + snapshot only.

**Immutability mechanism (physical):** the application DB role receives `SELECT`/`INSERT` only on `target_snapshot`, `target_activation`, `target_revocation`, and `belief_promotion`; it has no `UPDATE` or `DELETE` privilege on those tables. The only mutable authority-adjacent table is `authority_target` — and after this repair no column on it is authority-bearing (there is no pointer and no lifecycle mirror; see Second-Source-of-Truth Audit). The exact privilege statements are DDL-time work against the pinned CockroachDB version; the security property fixed here is: **no application UPDATE can repoint, substitute, or resurrect authority, because no authority-bearing mutable column exists and no authority fact is ever UPDATEd or DELETEd.**

Against a fully compromised service this boundary is worthless (the service holds the application role and can INSERT authority facts) — that is the accepted TCB boundary (D2), stated plainly, not marketed as structural prevention.

### C-2 — Promotion epoch

**Decision: `belief_promotion` side table (locked).** The frozen `belief` table is not modified.

```text
belief_promotion(
    promotion_id     — PK
    tenant_id        — FK (tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)
    belief_id        — FK belief_id → belief(id)
    promotion_epoch  — CHECK (promotion_epoch >= 1)
    promoted_at
)
UNIQUE (belief_id, promotion_epoch)
UNIQUE (tenant_id, belief_id, promotion_epoch)   -- composite FK target
```

**The occurrence rule (explicit):**

```text
promotion_epoch alone   ≠   promotion occurrence
a belief_promotion row  =   a promotion occurrence
```

The epoch is a property *of* an occurrence row, never a standalone counter. There is no valid epoch without its row, and no row without a committed promotion. This is what closes the SDAR-4 fabrication attack structurally.

Semantics:

- The first committed promotion of a belief is **epoch 1**. There is no epoch 0 row; "entered" is simply the absence of a `belief_promotion` row.
- Every later committed promotion inserts a new occurrence row with epoch = current max for that belief, + 1, computed inside the promotion transaction.
- Retraction creates no row and removes none; `belief_promotion` is append-only.
- A retracted belief that is later promoted gets a new occurrence row. Prior occurrence rows remain as history.
- Concurrent promotions of the same belief are impossible as a semantic matter (the kernel status transition only fires from a non-promoted state) and are physically serialized by the `belief` row update plus the `UNIQUE(belief_id, promotion_epoch)` backstop; a losing `40001` retry re-reads and either inserts the next occurrence or refuses.
- **A justification can reference only an actually committed promotion occurrence** — enforced by FK to the occurrence row, not prose. An attacker cannot insert `belief_id = X, promotion_epoch = 47` unless occurrence 47 exists as a row.
- The **current promotion occurrence** of a belief is the occurrence row with `max(promotion_epoch)`. This derived value is used by pre-execution verification (§8/§16 rules), never stored anywhere mutable.

No event sourcing. Occurrences are minimal facts, not a history engine.

### C-3 — Discharge uniqueness and instrument replay

**Decision: unconditional global-per-obligation replay protection keyed by canonical instrument identity.**

```text
UNIQUE (tenant_id, obligation_key, policy_version_id, instrument_ref)
```

There is **no** partial predicate and **no** status column: every `debt_discharge` row is an accepted, immutable fact (SDAR-5 repair). The uniqueness is therefore effective for the lifetime of the instrument/obligation identity — no revocation path can free the key, because no revocation path exists in the MVP.

`instrument_kind` (`CHECK (instrument_kind IN ('evidence','attestation'))`) plus canonicalization rules make "one instrument" a well-defined object:

- `instrument_kind = 'evidence'` → `instrument_ref` is the canonical evidence row UUID.
- `instrument_kind = 'attestation'` → `instrument_ref` is the canonical attestation identifier (issuer-assigned `attestation_id` per ADR-0001's field model).
- Canonicalization occurs at the service ingress write path: the discharge transaction stores only the canonical form. Multiple textual spellings of the same instrument MUST NOT become distinct identities. **Residual (stated, not hidden): canonicalization is a service responsibility in MVP**; the DB enforces uniqueness over whatever canonical identity reaches it.

Cross-obligation instrument reuse is a separate semantic question and is not automatically prohibited unless the instrument class is defined as single-use.

### C-4 — Obligation identity (honest transitional statement)

**Decision: `(obligation_key, policy_version_id)` is the durable discharge identity; the frozen `belief.debt TEXT[]` remains a transitional unversioned promotion gate; the gap between them is an explicitly accepted transitional residual.**

The MVP rule, stated exactly:

- In the **discharge ledger**, obligation meaning is policy-version-bound: `needReview + policy v1 ≠ needReview + policy v2` unless the policy model explicitly declares equivalence. This is authoritative for discharge history, replay scope, and audit.
- In the **promotion gate**, `promoted_is_debt_free` reads only the unversioned array: `coalesce(array_length(debt,1),0) = 0`. The frozen array cannot encode policy version.
- Therefore: **transitional debt semantics are NOT structurally protected against a policy redefining the same debt string.** A v1 discharge retires the string `needReview`; if v2 later redefines `needReview`, the already-promoted belief is not retroactively invalidated, and the promotion gate cannot distinguish the two meanings. This residual is accepted for MVP and is **labeled as such in the impossible-state matrix (§7) and the security-claims table (§19)** — it is not placed in the DB-prevention column.
- The discharge transaction keeps the two truths coherent on the key: it records the discharge and performs `array_remove(debt, obligation_key)` for the same `obligation_key` in one `crdb.ExecuteTx`. The ledger's `policy_version_id` binds meaning; the array's presence/absence gates promotion. No stronger claim is made.

If this residual is later judged unacceptable, the smallest bridge is a transactional service check consuming the policy identity before promotion — that is the long-term obligation-rows direction (§10), not an MVP schema element.

### C-5 — Policy version ordering

**Decision: per-tenant monotonic `revision`, transactionally allocated; `effective_at` is metadata.**

- `policy_version` PK is `(tenant_id, policy_version_id)`; `policy_version_id` is a service-generated stable identifier (UUID), independent of content.
- `UNIQUE (tenant_id, revision)` is the ordering enforcement. **The PK does not enforce order — the UNIQUE does**; this document says so explicitly.
- Allocation: `INSERT … SELECT coalesce(max(revision),0)+1` within the write transaction under `crdb.ExecuteTx`; first revision is **1**; `CHECK (revision >= 1)`. A concurrent writer conflicts (`40001`) and the retry re-reads max and recomputes; a residual collision surfaces as a `23505` unique violation on `(tenant_id, revision)` — deterministic ordering either way. No wall-clock timestamp participates in security ordering.
- `effective_at` is descriptive metadata; it is never the ordering primitive and cannot be used to legitimize back-dating.
- Policy rows are append-only; the application role has no UPDATE/DELETE on `policy_version`.
- Approval binds the exact `policy_version_id` (and therefore its revision and content) observed in the approval transaction. A later policy may restrict; it may never broaden an already approved authority — evaluated at pre-execution re-verification (§8/§16).

### C-6 — Belief tenant migration

**Decision: `belief_tenant` association object; frozen `belief` untouched.**

```text
belief_tenant(
    tenant_id,
    belief_id,      — FK belief_id → belief(id)
    mapped_at
)
PRIMARY KEY (tenant_id, belief_id)
UNIQUE (belief_id)          -- MVP one-to-one
```

- The mapping is **immutable** for MVP: insert-only rows; the application role has no UPDATE/DELETE on `belief_tenant`. A belief cannot be silently reassigned from tenant A to tenant B — that would require DELETE + INSERT, and DELETE is not granted.
- **Insertion authority:** mapping rows are created only by the explicit provisioning/migration boundary (bootstrap tenant mapping, legacy-scenario backfill). A normal authority request MUST NOT insert a mapping; the design's transaction contracts do not include one.
- Missing mapping **fails closed**: every authority relationship that reaches a belief goes through `(tenant_id, belief_id) → belief_tenant`, so an unmapped belief cannot participate in any authority relationship.
- `scenario_id` is never treated as an authenticated tenant identity. Backfill uses the explicit legacy scenario→tenant mapping only.

### C-7 — Revocation-time attestation validity

**Decision: revocation is forward-looking for principal authorization; attestation validity is issuance-time bounded; the boundary is explicit, justified, and split into two distinct predicates that an implementation MUST NOT collapse.**

**Predicate (a) — attestation issuer validity:**

```text
issued_at < revoked_at        -- of the attestation's ISSUING principal
```

The revocation instant is inclusive of the principal's loss of authority; an attestation issued at exactly the revocation instant is temporally ambiguous (clock skew may place its issuance after the revocation decision), and Solvent fails closed at ambiguity — hence strict `<`. An attestation with `issued_at < revoked_at` is a legitimate pre-revocation attestation and **remains historically valid**; issuance at or after `revoked_at` is refused. `issued_at` is **IdP-trusted provenance**, not Solvent-verified truth.

**Predicate (b) — discharger authority:**

```text
discharged_by.revoked_at IS NULL    -- evaluated at discharge acceptance
```

**These are two separate predicates over potentially different principals and MUST NOT be merged into a single "principal is valid" check.** A principal whose attestation was issued before its revocation has produced a historically valid attestation, while that same principal may no longer perform a new discharge. The discharge transaction evaluates (a) against the attestation's issuing principal (resolved from the attestation) and (b) against `discharged_by`, independently.

This deliberately does **not** claim that credential compromise can automatically invalidate earlier attestations. Credential/key compromise invalidation is FUTURE / UNSCOPED because Solvent does not own credential lifecycle or key management. No credential table is invented.

Replay is handled by C-3, independently of revocation.

---

## 4. Durable-Fact Derivation

| Durable Fact | Required? | Owner | Representation | Relationships | DB Enforcement | Service Enforcement | Why Durable |
|---|---|---|---|---|---|---|---|
| F1 principal identity | Yes | Identity + Service | `principal` | principal → target/snapshot/discharge/policy/revocation | PK; `UNIQUE(tenant_id, principal_id)`; FKs | authentication/role authorization | historical attribution |
| F1 principal type | Yes | Identity + Service | `principal.principal_type` | referenced by approval/discharge | CHECK domain `('human','agent','workload','service')`; immutable | role semantics (I-A4 reads the row) | approval separation |
| F1 principal revocation | Yes | Service + Identity | `principal.revoked_at` (write-once) | approvals/discharges re-check | NULL-able column; write-once by privilege/posture | forward-looking refusal | prevent new use |
| F1 issuer reference | Yes | Identity | `principal.issuer` | principal attribution | NOT NULL | issuer authenticity | trace assertion source |
| F1 credential lifecycle | No in MVP | IdP | external | outside schema | none | IdP | avoid IAM scope |
| F2 tenant binding | Yes | DB + Service | `tenant_id` on authority objects + `belief_tenant` | composite FKs throughout | composite FK/UNIQUE with declared target keys | tenant authorization | cross-tenant safety |
| F2 authority tuple (proposal) | Yes | Service + DB | `authority_target` proposal tuple columns | snapshot copies at approval | NOT NULL + non-empty CHECKs | semantic interpretation | representable CreateTarget |
| F2 authority tuple (approved) | Yes | DB | `target_snapshot` tuple columns | activation binds snapshot | FK graph; immutable | — | sole execution authority |
| F2 target lifecycle | Yes | Kernel/Service + DB | `target_activation` / `target_revocation` facts; lifecycle states derived at read time (§8) | target → activation → snapshot | insert-only facts; `UNIQUE(tenant_id, target_id)` on activation | legal transitions | non-resurrectable lifecycle |
| F2 target identity | Yes | DB | `authority_target.target_id` | all authority edges | PK + `UNIQUE(tenant_id, target_id)` | lookup | stable authority reference |
| F3 justification relationship | Yes | Service + DB | `justification` | target ↔ belief_tenant ↔ belief_promotion | composite FKs; natural key | AND set evaluation | explicit authorization basis |
| F3 promotion identity | Yes | Kernel + DB | `belief_promotion` occurrence rows | justification → belief_promotion; belief → belief_tenant | PK; `UNIQUE(belief_id, promotion_epoch)`; FKs | promotion semantics | prevent stale revalidation |
| F3 approval justification snapshot | Yes | Service + DB | `target_snapshot.justification_set` | pinned request → snapshot | immutable snapshot | verification | freeze approval meaning |
| F4 obligation identity | Yes | Policy + Service | `obligation_key`, `policy_version_id` | discharge → policy | NOT NULL/FK | qualification | prevent obligation drift |
| F4 discharge identity | Yes | Kernel + DB | discharge PK | discharge → belief_tenant/principal/policy | PK/FKs | attribution | durable audit |
| F4 instrument identity | Yes | Service | `instrument_kind` + canonical `instrument_ref` | discharge uniqueness | UNIQUE with obligation | canonicalization | replay prevention |
| F4 debt state | Yes | Kernel + DB | existing `belief.debt[]` + discharge ledger | single-tx coherence | transitional (C-4 residual) | atomic discharge | preserve current promotion invariant |
| F4 discharge attribution | Yes | Identity + Service | `discharged_by` | `(tenant_id, discharged_by)` → principal | composite FK; non-revoked at acceptance | actor authorization | who discharged |
| F5 policy identity | Yes | Policy + DB | `policy_version (tenant_id, policy_version_id)` | snapshot/discharge → policy | PK/FK | policy semantics | historical reproducibility |
| F5 policy revision | Yes | DB + Service | `(tenant_id, revision)` | ordered policy history | `UNIQUE(tenant_id, revision)`; CHECK ≥ 1 | transactional max+1 allocation | deterministic ordering |
| F5 policy content integrity | Yes | Service | `policy_hash` over exact stored bytes | policy row | NOT NULL | storage-integrity check | identify exact content |
| F5 approval binding | Yes | Service + DB | `policy_version_id` in snapshot + activation | snapshot → policy | composite FK | version compatibility | freeze policy context |
| F6 approval time | Yes | Service + DB | `approved_at` + `activated_at` | snapshot/activation attribution | NOT NULL | time semantics | historical ordering |
| F6 pinned approval request | Yes | Service + DB | `requested_by`, `requested_at`, `pinned_request_hash` on `authority_target` | approval verifies pin in the activation transaction | NOT NULL at approval; recompute-and-compare in-tx | re-verification | approval approves exactly the reviewed candidate |
| F7 tenant-aware belief relation | Yes | DB | `belief_tenant` | tenant ↔ frozen belief | composite PK + `UNIQUE(belief_id)` + FK to belief | tenant authorization | structural isolation |
| Execution-time authority source | Yes | Service + DB | `target_snapshot` via `target_activation` | activation → snapshot | FK + insert-only privileges | exact verification | single authority source |
| Execution-effect claim | No in MVP | Executor | caller/system state only | outside schema guarantee | none | none | avoid false execution proof |

---

## 5. Relational Model

Ten objects:

```text
 1. principal
 2. authority_target      (proposal/pin identity — no lifecycle columns)
 3. target_snapshot       (immutable approved authority content)
 4. target_activation     (append-only authority-bearing activation fact)
 5. target_revocation     (append-only revocation fact)
 6. justification
 7. belief_promotion      (promotion occurrence referent)
 8. debt_discharge
 9. policy_version
10. belief_tenant
```

### 5.1 `principal`

**Purpose:** stable actor identity and attribution reference.

**Authority meaning:** identity attribution and approval-role input.
**Non-meaning:** credentials, authentication secrets, delegation graph, or proof that the principal is authorized for any role.

**Primary key:** `principal_id`.
**Alternate/natural keys:** `UNIQUE (tenant_id, principal_id)` — the tenant-composite FK target.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `principal_id` | NOT NULL | stable Solvent principal identity |
| `principal_type` | NOT NULL | `human`, `agent`, `workload`, or `service` |
| `issuer` | NOT NULL | asserting IdP/workload authority (source of assertion, not a credential identity) |
| `tenant_id` | NOT NULL | owning tenant |
| `revoked_at` | NULL | write-once forward-looking revocation timestamp |
| `created_at` | NOT NULL | attribution/history |

**Keys/constraints:**

- `PRIMARY KEY (principal_id)`
- `UNIQUE (tenant_id, principal_id)`
- `CHECK (principal_type IN ('human','agent','workload','service'))`
- Every authority-side principal reference is the composite FK `(tenant_id, <role>_principal_id) → principal(tenant_id, principal_id)`.

**Immutable fields:** `principal_id`, `principal_type`, `issuer`, `tenant_id`, `created_at`.
**Mutable fields:** `revoked_at` only — **write-once** (set at most once, never cleared). The write-once rule is enforced by the service transaction contract and by the privilege posture (no general UPDATE grant; revocation is a dedicated transaction); it is stated here as the rule DDL must support.
**Lifecycle:** create → active → revoked. Revocation does not erase history.
**Delete behavior:** none. Rows are never deleted.
**Transaction owner:** identity provisioning (create); dedicated revocation transaction (revoke).
**Security rationale:** stable attribution across credential rotation; rotation updates `issuer` semantics only in the IdP and never creates a second principal row. A legitimate type change is modeled as a **new principal identity with explicit historical linkage** (no richer transition model in MVP; revisit only with customer evidence).

### 5.2 `authority_target`

**Purpose:** durable proposal and approval-pin identity for a possible authorization. Carries the candidate tuple so CreateTarget/AttachJustification/RequestAuthorization are representable (SDAR-2), and the pin so approval approves exactly the reviewed candidate (SDAR-8).

**Authority meaning:** proposal record and pin carrier. **Not** the execution-time authorization representation, and **not** a lifecycle record.
**Non-meaning:** after activation, no column on this row is consulted to resolve authority; lifecycle state is not stored here at all.

**Primary key:** `target_id`.
**Alternate keys:** `UNIQUE (tenant_id, target_id)` — composite FK target for snapshot/activation/justification/revocation.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `target_id` | NOT NULL | stable target identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `principal_id` | NOT NULL | authorized principal of the proposal tuple |
| `resource_type` | NOT NULL | opaque resource type |
| `resource_id` | NOT NULL | opaque resource identifier |
| `scope` | NOT NULL | opaque scope representation; exact-equality at the boundary |
| `action_namespace` | NOT NULL | exact action namespace |
| `action_name` | NOT NULL | exact action name |
| `consequence_type` | NOT NULL | consequence descriptor type |
| `consequence_parameters` | NOT NULL | canonical JSON object payload |
| `created_by` | NOT NULL | proposing principal (composite FK) |
| `created_at` | NOT NULL | history |
| `requested_by` | NULL | requester principal (composite FK); set by RequestAuthorization |
| `requested_at` | NULL | request timestamp; set by RequestAuthorization |
| `pinned_request_hash` | NULL | canonical pin over tuple + justification set at request time (§6.4); **represents the exact candidate presented for approval, not merely an integrity checksum** |

**There is no `snapshot_id` column, no `state` column, and no `revoked_at` column.** Lifecycle is derived at read time from the facts (§8); nothing mutable on this row can drift into a second authority source.

**Keys/constraints:**

- `PRIMARY KEY (target_id)`
- `UNIQUE (tenant_id, target_id)`
- `CHECK (length(resource_type) > 0)`, same for `resource_id`, `scope`, `action_namespace`, `action_name`, `consequence_type`
- `CHECK (consequence_parameters` is a valid canonical JSON object`)`
- FK `(tenant_id, principal_id) → principal(tenant_id, principal_id)`
- FK `(tenant_id, created_by) → principal(tenant_id, principal_id)`
- FK `(tenant_id, requested_by) → principal(tenant_id, principal_id)` (when set)

**Immutable fields:** `target_id`, `tenant_id`, `created_by`, `created_at`.
**Mutable fields:** the proposal tuple, `requested_by`, `requested_at`, `pinned_request_hash` — **only before activation**. The proposal tuple is mutable only while no activation fact exists (pre-approval); after activation, changes to these fields cannot alter authorization because the resolution rule never reads them (structurally: no authority query touches `authority_target`).
**Lifecycle:** `CreateTarget` (insert) → `AttachJustification` (separate rows) → `RequestAuthorization` (pin) → `Approve` (snapshot + activation facts). Lifecycle states are **read-time derivations** over the facts (§8), not stored values.
**Delete behavior:** none. Proposed targets that are abandoned remain as audit rows.
**Transaction owner:** authority service (create/attach/request); approval transaction (activate); revocation transaction (revoke — writes the revocation fact, not this row).
**Security rationale:** the four-operation model needs durable candidate state; freezing-by-copy at approval plus the no-pointer/no-mirror rule keeps proposal mutability from touching authority.

### 5.3 `target_snapshot`

**Purpose:** immutable execution-time authority representation.

**Authority meaning:** the sole execution-time authority content.
**Non-meaning:** proof that the executor produced the real-world effect.

**Primary key:** `snapshot_id`.
**Alternate keys:** `UNIQUE (tenant_id, target_id, snapshot_id)` — **declared** as the composite FK target for `target_activation` (the central authority boundary; functionally implied by the PK but declared explicitly so the FK has a real, named target and the same-target/same-tenant proof is structural).

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `snapshot_id` | NOT NULL | immutable snapshot identity |
| `target_id` | NOT NULL | parent target |
| `tenant_id` | NOT NULL | tenant boundary |
| `principal_id` | NOT NULL | authorized principal (copied from proposal) |
| `resource_type` | NOT NULL | opaque resource type (copied) |
| `resource_id` | NOT NULL | opaque resource identifier (copied) |
| `scope` | NOT NULL | opaque scope representation (copied verbatim) |
| `action_namespace` | NOT NULL | exact action namespace (copied) |
| `action_name` | NOT NULL | exact action name (copied) |
| `consequence_type` | NOT NULL | consequence descriptor type (copied) |
| `consequence_parameters` | NOT NULL | canonical JSON object payload (copied) |
| `justification_set` | NOT NULL | immutable serialized set of approved justification records |
| `policy_version_id` | NOT NULL | exact approval policy |
| `approver_principal_id` | NOT NULL | approval actor |
| `approved_at` | NOT NULL | approval time |
| `snapshot_hash` | NOT NULL | canonical content digest for detection/audit, not authority by itself |

There is **no `approver_principal_type` column** (SDAR-17 removed); the principal row owns the type.

`justification_set` is the immutable serialized approval snapshot; per entry, exactly:

```text
belief_id
promotion_epoch
claim
status_at_approval   -- 'promoted'
```

**Keys/constraints (the full key graph — SDAR-3):**

- `PRIMARY KEY (snapshot_id)`
- `UNIQUE (tenant_id, target_id, snapshot_id)`
- FK `(tenant_id, target_id) → authority_target(tenant_id, target_id)`
- FK `(tenant_id, principal_id) → principal(tenant_id, principal_id)`
- FK `(tenant_id, approver_principal_id) → principal(tenant_id, principal_id)`
- FK `(tenant_id, policy_version_id) → policy_version(tenant_id, policy_version_id)`
- Same non-empty CHECKs as the proposal tuple columns.

**Uniqueness placement:** the one-authority-per-target rule lives on `target_activation` (`UNIQUE(tenant_id, target_id)`), **not** on the snapshot. Snapshots and activations are created 1:1 in the same approval transaction, so no duplicate uniqueness is needed on the snapshot; a stray unreferenced snapshot row (service bug) is inert content — it becomes authority only through an activation fact, and only one activation per target can ever exist.

**Immutable fields:** all.
**Mutable fields:** none. Insert-only; the application role has no UPDATE/DELETE.
**Lifecycle:** created exactly once, inside the approval transaction, immediately before its activation fact.
**Delete behavior:** none.
**Transaction owner:** approval transaction.
**Security rationale:** one frozen content object, relationally anchored to target, principal, approver, and policy — every tenant edge composite.

### 5.4 `target_activation`

**Purpose:** the append-only authority-granting fact. Replaces every mutable authority pointer (SDAR-1, SDAR-9).

**Authority meaning:** **an activation row is itself an authority-bearing immutable fact. It is not merely a cache or projection.** Existence = the target was activated (authority granted) exactly once in its history. No implementation may treat any other representation — including any convenience state — as the authority determinant.
**Non-meaning:** current validity — that requires the absence of a revocation fact (read-time derivation, §8).

**Primary key:** `activation_id`.
**Alternate keys:** `UNIQUE (tenant_id, target_id)` — **over the target's entire history**. "Already-activated" means "ever activated." There is deliberately no state predicate (`WHERE state='active'`) on this uniqueness; a revoked target cannot acquire a second activation, so `revoked → active` resurrection requires a DELETE that the application role does not have.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `activation_id` | NOT NULL | activation fact identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `target_id` | NOT NULL | activated target |
| `snapshot_id` | NOT NULL | the exact approved snapshot bound at activation |
| `activated_at` | NOT NULL | activation time |
| `activated_by` | NOT NULL | approving principal (same as snapshot approver; composite FK) |

**Keys/constraints:**

- `PRIMARY KEY (activation_id)`
- `UNIQUE (tenant_id, target_id)` — entire history
- **FK `(tenant_id, target_id, snapshot_id) → target_snapshot(tenant_id, target_id, snapshot_id)`** — the central authority boundary, stated explicitly: the activation can only bind a snapshot that provably belongs to the **same target and the same tenant** (the target key is declared on the snapshot side; §5.3). A snapshot from another target or tenant cannot satisfy this FK.
- FK `(tenant_id, target_id) → authority_target(tenant_id, target_id)`
- FK `(tenant_id, activated_by) → principal(tenant_id, principal_id)`

**Immutable fields:** all. Insert-only; the application role has no UPDATE/DELETE.
**Lifecycle:** inserted exactly once, in the approval transaction, after the snapshot row, same `crdb.ExecuteTx`.
**Delete behavior:** none (revocation narrows via `target_revocation`, never by deleting activation).
**Transaction owner:** approval transaction.
**Security rationale:** the authority grant is an INSERT that itself requires the snapshot (FK) — activation without an approved snapshot is unwritable; re-activation is unwritable (UNIQUE over history); substitution is unwritable (no pointer exists).

### 5.5 `target_revocation`

**Purpose:** append-only revocation fact.

**Authority meaning:** existence removes current authority (read-time resolution requires its absence).
**Non-meaning:** deletion or mutation of the historical approval; history remains intact.

**Primary key:** `(tenant_id, target_id)` — a target is revoked at most once.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `tenant_id` | NOT NULL | tenant boundary |
| `target_id` | NOT NULL | revoked target |
| `revoked_at` | NOT NULL | revocation time |
| `revoked_by` | NOT NULL | revoking principal (composite FK) |
| `reason` | NULL | free-text audit note; never interpreted |

**Keys/constraints:**

- `PRIMARY KEY (tenant_id, target_id)`
- FK `(tenant_id, target_id) → authority_target(tenant_id, target_id)`
- FK `(tenant_id, revoked_by) → principal(tenant_id, principal_id)`

**Immutable fields:** all. Insert-only; no UPDATE/DELETE for the application role.
**Lifecycle:** inserted by the revocation transaction.
**Delete behavior:** none.
**Transaction owner:** revocation transaction.
**Security rationale:** revocation is an inserted fact, so it cannot be undone by any UPDATE the application role can issue; resurrection would require DELETE.

### 5.6 `justification`

**Purpose:** explicit proposal/history relation between a target and an exact promotion occurrence of a belief.

**Authority meaning:** candidate/audit justification — part of the reviewed set only when pinned and approved into a snapshot.
**Non-meaning:** active permission. The live table has no authority-granting state.

**Primary key:** `justification_id`.
**Alternate/natural keys:** `UNIQUE (tenant_id, target_id, belief_id, promotion_epoch)` — makes AttachJustification idempotent and forbids duplicate links (SDAR-15).

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `justification_id` | NOT NULL | relationship identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `target_id` | NOT NULL | target reference |
| `belief_id` | NOT NULL | frozen belief reference through `belief_tenant` |
| `promotion_epoch` | NOT NULL | exact promotion occurrence (the epoch of a real `belief_promotion` row) |
| `attached_by` | NOT NULL | attaching principal (composite FK) |
| `attached_at` | NOT NULL | history |

**Keys/constraints:**

- `PRIMARY KEY (justification_id)`
- `UNIQUE (tenant_id, target_id, belief_id, promotion_epoch)`
- FK `(tenant_id, target_id) → authority_target(tenant_id, target_id)`
- FK `(tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)` — wrong-tenant belief references are structurally impossible
- FK `(tenant_id, belief_id, promotion_epoch) → belief_promotion(tenant_id, belief_id, promotion_epoch)` — a justification can reference only a committed promotion occurrence (SDAR-4)
- FK `(tenant_id, attached_by) → principal(tenant_id, principal_id)`

**Immutable fields:** all. Insert-only (idempotent attach = `ON CONFLICT DO NOTHING` semantics at the service).
**Lifecycle:** created by AttachJustification against a target with no activation fact; never deleted or edited.
**Delete behavior:** none.
**Transaction owner:** authority service (attach).
**Security rationale:** the set can only grow and each member is FK-bound to a real promotion occurrence — this is what makes the timestamp component of the approval pin provably unambiguous (§6.4).

### 5.7 `belief_promotion`

**Purpose:** durable referent for each committed promotion occurrence (SDAR-4; locked C-2 direction).

**Authority meaning:** existence of a `(belief_id, promotion_epoch)` row = that promotion occurred. **A `belief_promotion` row is a promotion occurrence; the epoch alone is nothing.**
**Non-meaning:** current belief status (that lives in frozen `belief.status`); no permission of any kind.

**Primary key:** `promotion_id`.
**Alternate keys:** `UNIQUE (belief_id, promotion_epoch)`; `UNIQUE (tenant_id, belief_id, promotion_epoch)` (the justification FK target).

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `promotion_id` | NOT NULL | promotion occurrence identity |
| `tenant_id` | NOT NULL | tenant boundary (structural, via `belief_tenant`) |
| `belief_id` | NOT NULL | frozen belief reference |
| `promotion_epoch` | NOT NULL | 1 for the first committed promotion; increments per promotion |
| `promoted_at` | NOT NULL | promotion time |

**Keys/constraints:**

- `PRIMARY KEY (promotion_id)`
- `UNIQUE (belief_id, promotion_epoch)`
- `UNIQUE (tenant_id, belief_id, promotion_epoch)`
- `CHECK (promotion_epoch >= 1)`
- FK `(tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)` — tenant relationship is structural
- FK `belief_id → belief(id)` — the occurrence belongs to a real belief

**Immutable fields:** all. Append-only.
**Lifecycle:** inserted by the promotion transaction (future kernel extension) in the same `crdb.ExecuteTx` as the `belief.status` transition; never updated or deleted. Migration inserts rows only where promotion evidence exists (§16).
**Delete behavior:** none.
**Transaction owner:** kernel promotion transaction; migration (evidence-gated backfill only).
**Security rationale:** epoch forgery becomes a constraint violation; "no committed occurrence row = no valid justification for that epoch" is structural.

### 5.8 `debt_discharge`

**Purpose:** durable attribution and replay-controlled discharge history. Every row is an accepted, immutable fact.

**Authority meaning:** durable evidence that a particular obligation was discharged and by whom.
**Non-meaning:** proof that the underlying real-world claim is true; also not, by itself, structural protection of transitional `debt[]` semantics against policy redefinition (C-4 residual, stated).

**Primary key:** `discharge_id`.
**Alternate keys:** `UNIQUE (tenant_id, obligation_key, policy_version_id, instrument_ref)` — unconditional (no partial predicate; no status column).

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `discharge_id` | NOT NULL | discharge identity |
| `tenant_id` | NOT NULL | tenant boundary |
| `belief_id` | NOT NULL | discharged belief (via `belief_tenant`) |
| `obligation_key` | NOT NULL | stable obligation identifier |
| `policy_version_id` | NOT NULL | obligation meaning/version binding |
| `instrument_kind` | NOT NULL | `'evidence'` or `'attestation'` |
| `instrument_ref` | NOT NULL | canonical instrument identity |
| `discharged_by` | NOT NULL | asserting principal (composite FK); predicate (b) of C-7 applies to this principal |
| `issued_at` | NULL | attestation issuance time; IdP-trusted provenance; required (NOT NULL) when `instrument_kind = 'attestation'`; predicate (a) of C-7 applies to the attestation's issuing principal |
| `accepted_at` | NOT NULL | discharge acceptance time |
| `integrity_reference` | NULL | verification metadata / attestation integrity identifier |

There is **no `status` column** in the MVP. All rows are accepted facts. **Discharge revocation is FUTURE / UNSCOPED**; when it becomes real, it is an append-only counter-entry (never an UPDATE of an accepted row).

**Keys/constraints:**

- `PRIMARY KEY (discharge_id)`
- `UNIQUE (tenant_id, obligation_key, policy_version_id, instrument_ref)` — the replay rule, effective for the lifetime of the identity
- `CHECK (instrument_kind IN ('evidence','attestation'))`
- `CHECK (length(obligation_key) > 0)`, `CHECK (length(instrument_ref) > 0)`
- FK `(tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)`
- FK `(tenant_id, policy_version_id) → policy_version(tenant_id, policy_version_id)`
- FK `(tenant_id, discharged_by) → principal(tenant_id, principal_id)`

**Immutable fields:** all. Insert-only; no UPDATE/DELETE for the application role.
**Lifecycle:** created by the discharge transaction: `INSERT debt_discharge` + `UPDATE belief SET debt = array_remove(debt, obligation_key)` in one `crdb.ExecuteTx` (D9 single-transaction rule). No accepted discharge may commit without the corresponding debt retirement, and no debt retirement may commit without the discharge record.
**Delete behavior:** none.
**Transaction owner:** kernel/service discharge transaction.
**Security rationale:** immutable accepted facts + unconditional uniqueness + canonical instrument identity close the replay paths, including through any future revocation mechanism.

### 5.9 `policy_version`

**Purpose:** immutable, tenant-bound policy identity and historical ordering.

**Authority meaning:** historical policy context bound into approvals.
**Non-meaning:** proof that the author was authorized to publish policy (attribution ≠ authorization — Service/Policy/Identity decision).

**Primary key:** `(tenant_id, policy_version_id)`.
**Alternate keys:** `UNIQUE (tenant_id, revision)` — the actual ordering enforcement.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `tenant_id` | NOT NULL | policy owner |
| `policy_version_id` | NOT NULL | service-generated stable identifier (UUID); independent of content |
| `revision` | NOT NULL | strictly increasing per-tenant sequence; first revision = 1 |
| `policy_hash` | NOT NULL | SHA-256 over the exact stored `policy_content` bytes (integrity-of-storage; NOT a version identity) |
| `policy_content` | NOT NULL | policy representation required for historical interpretation |
| `effective_at` | NOT NULL | metadata; never the ordering primitive |
| `author_principal_id` | NOT NULL | attribution only (composite FK) |
| `created_at` | NOT NULL | history |

**Keys/constraints:**

- `PRIMARY KEY (tenant_id, policy_version_id)`
- `UNIQUE (tenant_id, revision)`
- `CHECK (revision >= 1)`
- FK `(tenant_id, author_principal_id) → principal(tenant_id, principal_id)` — attribution only

**Immutable fields:** all. Append-only; the application role has no UPDATE/DELETE.
**Lifecycle:** created by the policy-publication transaction (revision allocated per C-5).
**Delete behavior:** none.
**Transaction owner:** policy service.
**Security rationale:** deterministic ordering independent of wall clocks; approval binds version identity, not hash alone; downgrade may restrict but never broaden (re-verification, §8).

### 5.10 `belief_tenant`

**Purpose:** bridge the frozen belief model into tenant-aware authority relationships without modifying `belief` (C-6).

**Authority meaning:** the structural tenancy of a belief; the gate every belief-touching authority relationship passes through.
**Non-meaning:** authentication of the tenant (tenant identity is asserted at the service/identity boundary).

**Primary key:** `(tenant_id, belief_id)`.
**Alternate keys:** `UNIQUE (belief_id)` — MVP one-to-one; one belief has exactly one tenant mapping.

**Columns:**

| Column | Nullability | Meaning |
|---|---|---|
| `tenant_id` | NOT NULL | future tenant identity |
| `belief_id` | NOT NULL | frozen belief identity |
| `mapped_at` | NOT NULL | migration/audit time |

**Keys/constraints:**

- `PRIMARY KEY (tenant_id, belief_id)`
- `UNIQUE (belief_id)`
- FK `belief_id → belief(id)` — a mapping exists only for a real belief (SDAR-10)

**Immutable fields:** all. Insert-only; the application role has no UPDATE/DELETE — a tenant reassignment would require DELETE + INSERT, and DELETE is not granted.
**Lifecycle:** inserted once per belief by the provisioning/migration boundary only. Normal authority transactions never insert mappings.
**Delete behavior:** none.
**Transaction owner:** provisioning/migration.
**Security rationale:** missing mapping fails closed (unmapped beliefs cannot appear in any authority relationship); reassignment is structurally excluded.

---

## 6. Relationship Graph

Every edge names exact source columns, target key, cardinality, tenant binding, lifecycle, and authority consequence. No edge is described as "composite FK" without its columns.

```text
principal(tenant_id, principal_id)
   ↑  (tenant_id, principal_id)                       [composite FK target: UNIQUE]
   │
   ├── authority_target.principal_id / created_by / requested_by
   ├── target_snapshot.principal_id / approver_principal_id
   ├── target_activation.activated_by
   ├── target_revocation.revoked_by
   ├── justification.attached_by
   ├── debt_discharge.discharged_by
   └── policy_version.author_principal_id              (attribution only)

authority_target(tenant_id, target_id)                 [UNIQUE(tenant_id, target_id)]
   ↑
   ├── target_snapshot(tenant_id, target_id)            1 : 0..1 (created in approval tx)
   ├── target_activation(tenant_id, target_id)          1 : 0..1 (UNIQUE — entire history)
   ├── target_revocation(tenant_id, target_id)          1 : 0..1
   └── justification(tenant_id, target_id)              1 : 0..*

target_snapshot(tenant_id, target_id, snapshot_id)     [UNIQUE — declared FK target]
   ↑
   └── target_activation(tenant_id, target_id, snapshot_id)   1 : 0..1
       THE central authority boundary: an activation can bind only
       a snapshot of the same target and the same tenant.

target_snapshot → policy_version (tenant_id, policy_version_id)   N : 1

justification(tenant_id, belief_id)
   → belief_tenant(tenant_id, belief_id)               N : 1   [tenant gate]

justification(tenant_id, belief_id, promotion_epoch)
   → belief_promotion(tenant_id, belief_id, promotion_epoch)  N : 1   [exact occurrence]

belief_promotion(tenant_id, belief_id) → belief_tenant  N : 1
belief_tenant.belief_id → belief(id)                    N : 1   [frozen core]

debt_discharge(tenant_id, belief_id)      → belief_tenant          N : 1
debt_discharge(tenant_id, policy_version_id) → policy_version      N : 1
debt_discharge(tenant_id, discharged_by)  → principal              N : 1
```

### Lifecycle and authority consequence per edge

| Edge | Cardinality | Tenant binding | Lifecycle | Authority consequence |
|---|---|---|---|---|
| principal → authority_target (any role column) | 1 : 0..* | composite | immutable attribution | none by itself (attribution ≠ authorization) |
| authority_target → target_snapshot | 1 : 0..1 | composite | created in approval tx, immutable | none until activation |
| authority_target → target_activation | 1 : 0..1 ever | composite | insert-only | **grants authority** (with no revocation fact) |
| authority_target → target_revocation | 1 : 0..1 | composite | insert-only | **removes authority** |
| target_activation → target_snapshot | 0..1 : 1 | composite (3-column) | immutable binding | pins the exact authority content |
| justification → authority_target | 0..* : 1 | composite | insert-only | enters the pin; authority only via snapshot |
| justification → belief_tenant | 0..* : 1 | composite | insert-only | wrong-tenant belief impossible |
| justification → belief_promotion | 0..* : 1 | composite (3-column) | insert-only | fabricated epoch impossible |
| belief_promotion → belief_tenant | 0..* : 1 | composite | insert-only | tenant-structural promotion history |
| belief_tenant → belief | 0..1 : 1 | belief is frozen core | immutable mapping | unmapped belief = no authority path |
| debt_discharge → belief_tenant / policy / principal | 0..* : 1 | composite | insert-only | attribution + replay scope |
| policy_version → principal (author) | 0..* : 1 | composite | immutable | attribution only |

### 6.4 Approval pin (SDAR-8 repair)

**Durable pin representation** (columns on `authority_target`, set by RequestAuthorization):

- `requested_by` — requester principal.
- `requested_at` — request time.
- `pinned_request_hash` — **the exact candidate presented for approval — not merely an integrity checksum.**

**Canonical representation hashed** (part of the schema contract, not an implementation detail):

```text
canonical_request_string =
    tenant_id | target_id | principal_id | resource_type | resource_id |
    scope | action_namespace | action_name | consequence_type |
    consequence_parameters | justification_refs
where
    scope and consequence_parameters are canonical JSON
        (UTF-8, sorted keys, no insignificant whitespace)
    justification_refs =
        comma-joined, lexicographically sorted, deduplicated
        "belief_id:promotion_epoch" pairs
    separators: '|' between fields, ',' between pairs
pinned_request_hash = lowercase hex SHA-256 of the UTF-8 canonical_request_string
```

`justification_refs` contains exactly the justifications attached at or before `requested_at`.

**Why the pin is unambiguous:** justifications are insert-only and never deleted or edited (§5.6), so the reviewed set can only **grow**. The approval transaction therefore verifies the pin two ways, and both must hold:

1. Recompute `pinned_request_hash` over the current proposal tuple and the justifications with `attached_at <= requested_at`; mismatch (tuple edited, or any member changed) → **refuse**.
2. Refuse if **any** justification row exists for the target with `attached_at > requested_at` — a post-request attach is divergence even if the hashed subset still matches. (Redundant with rule 1's growth detection when the hash includes all pinned members; retained as an explicit cheap check so the failure is diagnosable.)

Divergence outcome: typed refusal; the requester must issue a new RequestAuthorization (re-pinning the new state) for approval to proceed. There is no silent set expansion.

**Approval transaction contract (single `crdb.ExecuteTx`; no verify-then-write separation):**

The recompute-and-compare of the pin happens **inside the same transaction that creates the snapshot and the activation** — never in a separate pre-check that another write could invalidate. Within that transaction:

- recompute the pin hash and compare (tuple unchanged, set unchanged);
- refuse on any post-request justification row (rule 2);
- verify every justification points to a real promotion occurrence (FK-guaranteed on the rows, re-checked by reading them);
- verify every referenced belief is still valid for that occurrence: `belief.status = 'promoted'` **and** the belief's current promotion occurrence (`max(promotion_epoch)`) equals the justification's epoch;
- verify tenant matches (composite FKs + the service's tenant context);
- verify the policy version is valid (exists — FK — and eligible per current policy rules);
- verify the approver: principal exists, not revoked (`revoked_at IS NULL`), role-authorized per policy (I-A4: approver type and eligibility set are Service/Policy/Identity checks reading the principal row);
- verify the required policy floor applies (service/policy check; I-P1/I-P2 DOCUMENTED CONTRACT; I-P3 candidate — never claimed as schema);
- verify requester/approver separation when required: for HIGH_IMPACT-classified consequences (service classification via policy), `approver ≠ requested_by`.

Then, in the same transaction: `INSERT target_snapshot` → `INSERT target_activation` (FK-bound) → done. On `40001`: the transaction body re-executes, recomputes the pin, and refuses on any divergence. **Retry never reads-and-approves a grown set.**

---

## 7. Impossible States

Enforcement layers: **DB** = constraint/privilege; **TX** = transaction contract; **SVC** = service check; **ID** = identity provider; **POL** = policy layer; **EXE** = executor; **FUT** = future/unscoped; **—** = not applicable.

| Forbidden state | DB | TX | SVC | ID/POL/EXE | Residual |
|---|---|---|---|---|---|
| Active target without approved snapshot | activation FK `(tenant_id, target_id, snapshot_id) → target_snapshot` | snapshot inserted before activation in same tx | — | — | none |
| Active target resolving to a different snapshot | no pointer exists; activation row is insert-only and binds one snapshot by 3-column FK | approval tx only writer | — | — | none (pointer class removed) |
| Revoked → active resurrection | activation `UNIQUE(tenant_id, target_id)` over entire history; revocation row insert-only; no DELETE grant | revocation tx | — | — | none short of TCB compromise |
| Duplicate activation | same UNIQUE (no state predicate) | approval tx | — | — | none |
| Snapshot with wrong target | FK `(tenant_id, target_id) → authority_target` | approval tx | — | — | none |
| Snapshot with wrong tenant | every snapshot FK is tenant-composite | — | tenant context | identity tenant assertion | none |
| Snapshot with wrong principal / approver | FK `(tenant_id, principal_id)` / `(tenant_id, approver_principal_id) → principal` | — | role check | IdP type assertion | none structurally; role authorization is SVC |
| Snapshot with wrong policy | FK `(tenant_id, policy_version_id) → policy_version` | — | policy eligibility | — | none |
| Proposal mutation changing execution authority | resolution never reads `authority_target`; pin hash detects pre-approval edits | approval tx recomputes pin in-tx | refuse divergence | — | post-approval proposal edits alter nothing (audit noise only) |
| Fabricated promotion epoch | justification FK `(tenant_id, belief_id, promotion_epoch) → belief_promotion`; `CHECK (promotion_epoch >= 1)`; occurrence row = occurrence | promotion tx inserts occurrence | — | — | none |
| Stale justification revival across re-promotion | occurrence FK pins the epoch; verification requires current `max(epoch)` = pinned epoch | approval/pre-execution re-verification | re-verification (D5) | — | none under §16 rule |
| Duplicate justification link | `UNIQUE (tenant_id, target_id, belief_id, promotion_epoch)` | idempotent attach | — | — | none |
| Accepted discharge replay (same instrument, same obligation, cross-belief) | `UNIQUE (tenant_id, obligation_key, policy_version_id, instrument_ref)` unconditional | discharge tx + array_remove one tx | instrument canonicalization | attestation authenticity | canonicalization is SVC (stated residual) |
| Discharge revocation freeing replay uniqueness | no status column; accepted rows insert-only, no UPDATE/DELETE grant | — | — | — | revocation itself is FUT (append-only counter-entry when it lands) |
| v1/v2 debt ambiguity at the promotion gate | — | — | **accepted transitional residual (C-4)**: `debt[]` is unversioned; ledger binding is authoritative for history/replay only | policy authors | explicitly NOT DB-prevented |
| Cross-tenant principal reference | `UNIQUE(tenant_id, principal_id)` + composite FKs on every referencing object | — | tenant context | identity tenant assertion | none |
| Cross-tenant belief reference | FK through `belief_tenant(tenant_id, belief_id)`; `UNIQUE(belief_id)` | — | tenant context | — | none |
| Cross-tenant policy reference | FK `(tenant_id, policy_version_id) → policy_version(tenant_id, policy_version_id)` | — | tenant context | — | none |
| Cross-tenant discharge attribution | FK `(tenant_id, discharged_by) → principal` | — | tenant context | — | none |
| Revoked approver approving new authority | — | approval tx reads `principal.revoked_at`, refuses on fresh state | role check | IdP signal | race closed by serial order |
| Invalid attestation issuance (post-revocation) | — | discharge tx checks predicate (a): `issued_at < issuing principal.revoked_at` when `instrument_kind='attestation'` | refuses | IdP authenticity of `issued_at` | compromise invalidation FUT |
| Revoked discharger creating a new discharge | — | discharge tx checks predicate (b): `discharged_by.revoked_at IS NULL` | refuses | IdP signal | race closed by serial order |
| Approver == requester (HIGH_IMPACT) | — | approval tx compares `approver` vs `requested_by` | policy classification | — | classification is POL |
| NULL treated as wildcard | NOT NULL on every authority dimension | — | fail closed | — | none |
| Empty-string / empty-JSON wildcard | CHECKs per §12 table | — | exact-equality matching cannot widen | — | none |
| Migration inference (NULL → wildcard, guessed dimensions) | NOT NULLs on all new tables | — | missing dimension → NOT AUTHORIZED | — | transition window governed by §16 rule |

---

## 8. Snapshot Authority and Lifecycle Derivation

### Authority resolution (the only rule)

```text
authority(target)
    = target_activation row exists              (authority-bearing immutable fact)
      ∧ its target_snapshot exists              (FK-bound at activation)
      ∧ no target_revocation row exists
```

Execution authorization resolves exactly this and **does not** read `authority_target` proposal columns or the live `justification` table. Mutable proposal fields are pre-approval only; after activation, changes to them cannot alter authorization — structurally, because no authority query touches them.

### Lifecycle states are read-time derivations over the facts

There is no stored lifecycle state anywhere. The states are defined purely by the facts and derived at query time:

```text
PROPOSED = target exists           ∧ no target_activation row exists
ACTIVE   = target_activation row exists ∧ no target_revocation row exists
REVOKED  = target_revocation row exists
```

`authority_target.state` is a denormalized convenience value only — and in this design it is not stored at all. Authorization logic MUST consult only the facts; the derived states are query-level projections. There is no mutable mirror to drift into `activation says active / state says revoked` or `revocation exists / state accidentally remains active` — those divergence states are unrepresentable because there is no second copy.

### Post-approval retraction (pinned MVP rule)

A snapshot remains an immutable historical approval. Current validity of each of its justifications is re-checked at every pre-execution verification:

```text
justification currently valid  ⇔  belief.status = 'promoted'
                                 ∧ belief's current promotion occurrence (max(promotion_epoch))
                                   = the snapshot's pinned epoch
```

- Belief retracted → `status ≠ 'promoted'` → verification fails → **DENY** → revocation behavior per the authorization policy.
- Belief re-promoted at epoch N+1 → `max(promotion_epoch) = N+1 ≠ N` → the old snapshot's justification is **not** valid; the naive status-only check that would have revived it is explicitly excluded by the occurrence comparison.
- Revocation of the target (fact row) removes authority outright; re-grant requires a new target and a new approval.

DocTrust flow 7 asserts exactly this rule and has a deterministic expected outcome.

### Why live justification does not control authority

A post-approval link addition is an audit/proposal event. It cannot enlarge existing authority because execution never reads the live set as the authoritative set; the snapshot's `justification_set` is what was pinned and approved.

---

## 9. Belief / Promotion Model

The model distinguishes:

```text
belief_id            = enduring belief identity (frozen core)
belief.status        = current lifecycle state (frozen core; 'entered'/'promoted'/'retracted')
promotion occurrence = one belief_promotion row (promotion_id, epoch, promoted_at) — append-only
current promotion occurrence = the occurrence row with max(promotion_epoch) — derived, never stored mutable
justification reference = (tenant_id, belief_id, promotion_epoch) — FK to an actual occurrence row
```

Semantics (C-2):

- First committed promotion → epoch **1**. No epoch-0 rows exist; "entered" = no occurrence row.
- Each later committed promotion inserts a new occurrence row with epoch = max + 1, computed in the promotion transaction.
- Retraction creates/removes no occurrence row.
- Re-promotion gets a new occurrence row; the old epoch remains as history.
- Occurrence rows are append-only, written by the promotion transaction (future kernel extension) in the same `crdb.ExecuteTx` as the status transition.
- Concurrent promotions: serialized by the `belief` row transition plus `UNIQUE(belief_id, promotion_epoch)`; the `40001` loser re-reads and either writes the next occurrence or refuses.
- A justification can reference only an existing occurrence row (FK). Epoch forgery is a constraint violation.
- Existing promoted beliefs: **no fabricated history** (§16 backfill).

The design intentionally avoids event sourcing: occurrences are minimal facts, not a history engine.

---

## 10. Debt / Discharge Model

Separated concepts:

```text
Obligation   = what must be discharged — identified by (obligation_key, policy_version_id) in the ledger;
               transitional unversioned string in belief.debt[]
Discharge    = immutable accepted fact (debt_discharge row)
Instrument   = canonical (instrument_kind, instrument_ref) identity
Debt state   = currently outstanding obligations — belief.debt[] (transitional promotion gate)
```

### Transitional single-transaction rule (D9, unchanged)

```text
Record accepted discharge (INSERT debt_discharge)
        +
RetireDebt / array_remove(debt, obligation_key)
        +
commit
```

as one `crdb.ExecuteTx`. No accepted discharge may commit without the corresponding debt retirement, and no debt retirement may commit without the discharge record. Promotion (`promoted_is_debt_free`) never observes a half-discharged state.

### C-4 transitional residual (restated)

The ledger's policy binding is authoritative for discharge history, replay scope, and audit. The frozen promotion gate is unversioned. **Transitional debt semantics are NOT structurally protected against a policy redefining the same debt string.** Accepted residual, labeled in §7 and §19; the long-term direction (obligation rows → discharge rows → derived debt) would eliminate it and must preserve the `promoted_is_debt_free` impossibility semantics when it lands.

### Replay

The same canonical instrument cannot satisfy the same `(obligation_key, policy_version_id)` twice within one tenant, across beliefs, ever — unconditional UNIQUE on immutable rows. Cross-obligation instrument reuse is a separate semantic question, not auto-prohibited unless the instrument class is single-use.

### Revocation

Discharge revocation is **FUTURE / UNSCOPED**. When specified, it is an append-only counter-entry; accepted rows are never UPDATEd; the replay uniqueness remains effective for the lifetime of the instrument/obligation identity.

---

## 11. Policy, Principal, and Tenant Models

### Policy

Immutable, tenant-bound, revision-ordered (C-5). `effective_at` is metadata. `author_principal_id` is attribution; authorization to publish policy is Service/Policy/Identity. `policy_hash` hashes the exact stored `policy_content` bytes — storage-integrity metadata, never a version identity. Downgrade may restrict; never broaden; evaluated at pre-execution re-verification against the approval-time binding.

### Principal

`principal_id` is stable; rotation updates nothing in Solvent's identity model (the IdP owns credentials and may evolve its own assertions); `revoked_at` is write-once and forward-looking; `principal_type` is immutable — a legitimate type change is a new principal identity with explicit historical linkage. `created_by`, `requested_by`, `approved_by`, and `discharged_by` are distinct attribution roles and are never conflated.

### Tenant

Tenant identity is asserted outside the data model; bindings are structural inside it. `belief_tenant` is one-to-one with belief for MVP, immutable, insert-only by provisioning only. `scenario_id ≠ tenant_id`. No tenant table, no RLS in MVP (D3).

---

## 12. Scope / Resource / Action / Consequence — Exact Representation and Empty-Value Domain

All four dimensions are opaque to the schema; equality is exact, byte-for-byte on the canonical representation. No normalization, aliasing, wildcards, or ontology.

**Field-by-field empty/NULL domain:**

| Field | NULL | Empty string `''` | Empty JSON `{}` |
|---|---|---|---|
| `resource_type` | rejected (NOT NULL) → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a (text) |
| `resource_id` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |
| `scope` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` (opaque canonical representation) | rejected if the canonical-JSON representation is chosen for scope |
| `action_namespace` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |
| `action_name` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |
| `consequence_type` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |
| `consequence_parameters` | rejected → NOT AUTHORIZED | n/a (canonical JSON) | permitted **only** for a `consequence_type` registered as parameterless (service-validated at CreateTarget); under exact equality `{}` matches only `{}` and cannot widen authority |
| `obligation_key` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |
| `instrument_ref` | rejected → NOT AUTHORIZED | rejected by CHECK `length > 0` | n/a |

The baseline rule: **NULL → invalid / NOT AUTHORIZED, everywhere.** Under exact-equality matching, no stored representation — including empty ones where permitted — can match anything other than itself; wildcard semantics are structurally excluded, and the degenerate string cases are rejected by CHECK rather than interpreted.

### Action

`action_namespace` + `action_name`, preserved verbatim. No lowercase, alias, or hidden canonicalization anywhere in the schema or its adapters.

### Consequence

`consequence_type` + `consequence_parameters` (canonical JSON). Byte equality does not prove semantic equivalence to the executor; the schema does not claim it does. No DEFAULT consequence; missing consequence is an authorization failure.

---

## 13. Authorization Decision / Warrant

MVP does not require a separate `authorization_decision` table. Current authority is:

```text
target_activation (fact)
    + its target_snapshot (immutable content)
    + absence of target_revocation
    + fresh pre-execution re-verification (§16 rule)
```

Creating a decision object would duplicate authority. No persistent warrant table in MVP; a future portable warrant may reference `target_id / snapshot_id / policy_version_id / issued_at` without becoming a second authority source. `action_intent` must not be extended into a de facto warrant store.

---

## 14. Evidence

Current evidence is belief-owned; `content_sha256` is caller-supplied and **not** claimed as verified by Solvent. Transitional discharge instruments reference evidence rows by canonical UUID (`instrument_kind='evidence'`) or carry canonical attestation identifiers (`instrument_kind='attestation'`); canonicalization happens at the service ingress write path and is a stated service responsibility. The future Artifact/Citation split remains a migration target; the discharge schema does not prevent it. The schema preserves attribution and integrity references; it does not prove external truth.

---

## 15. Concurrency Matrix

Every mutation runs under `crdb.ExecuteTx` (I-7). Serializable isolation plus the named serialized state; the `40001` loser retries by re-executing the transaction body, which re-validates all preconditions on fresh state.

| Race | Rows read | Rows written | Serialization point | Winner | Loser (`40001` behavior) | Final committed state |
|---|---|---|---|---|---|---|
| Approve × Approve | target, justifications, belief states, principals, policy; recompute pin | `target_snapshot` INSERT + `target_activation` INSERT | `target_activation` `UNIQUE(tenant_id, target_id)` | first committer activates | retry re-executes: pin recompute ok, but activation INSERT hits the UNIQUE → `23505`/refusal; returns REVIEW | exactly one activation; no double authority |
| Approve × Attach | justifications (set) | justification INSERT vs snapshot+activation INSERT | justification read vs activation write conflict | whichever commits first | approver's retry recomputes pin → new member detected → **explicit refusal** (new request required) | set never silently enlarged |
| Approve × Retract | belief.status + current max epoch | `RetractCascade` writes vs snapshot+activation INSERT | belief row conflict | committed retract | retry re-reads: belief retracted (or epoch advanced) → precondition fails → refusal | no approval over stale/invalid justification |
| Approve × Policy Change | policy_version row | policy INSERT vs snapshot+activation INSERT | none destructive (append-only policy) | both (independent) | n/a — approval binds the version it read; later versions cannot alter the bound snapshot | approval-time semantics durable |
| Discharge × Promote | belief row (debt, status) | `array_remove` vs status UPDATE | same `belief` row | serial order | retry re-reads committed debt state | `promoted_is_debt_free` holds; no half-discharged observation |
| Discharge × Discharge | belief row; uniqueness check | two discharge INSERTs | `UNIQUE(tenant_id, obligation_key, policy_version_id, instrument_ref)` | first accepted | retry → `23505` or refusal after re-read | one accepted discharge |
| Revocation (principal) × Approve | `principal.revoked_at` | revocation UPDATE vs snapshot+activation INSERT | principal row conflict | serial order decides | approver retry re-reads: if revocation committed first → refusal | no new approval by revoked principal |
| Target mutation × Approve | proposal tuple (via pin recompute, in-tx) | tuple UPDATE vs snapshot+activation INSERT | pin hash comparison inside the approval tx | mutated tuple → approval refuses (divergence) | refusal on retry as well | proposal edits require re-request; post-activation edits alter nothing |
| Snapshot create × Activation | — | both INSERTs | same transaction; activation FK requires snapshot row | atomic | n/a | no activation without snapshot |
| Tenant mapping × Justification | `belief_tenant` row | mapping INSERT (provisioning) vs justification INSERT | justification FK on `(tenant_id, belief_id)` | mapping first | justification fails FK (`23503`) if mapping absent | fail closed; unmapped belief has no authority path |

Time-of-check/time-of-use, write skew, stale reads, phantom justifications, duplicate approvals, and double discharge are each covered by the in-transaction pinned-recompute contract plus the named serialization points above — not by invoking serializability as a slogan. In particular, the pin verification and the authority-granting writes are one transaction, so there is no window between "hash verified" and "snapshot written" for another writer to slip through.

---

## 16. Migration Design

No migration SQL is specified here.

### Legacy `action_intent.action TEXT`

Fail-closed rules:

```text
missing authority dimension → NOT AUTHORIZED
```

Never: `NULL → wildcard`, `missing → '*'`, guessed resource/scope/consequence/principal/tenant, alias-based authority. An old action string such as `update_record` is not silently translated into any default. Old and new semantics never share one authorization path unless the old intent is explicitly re-authorized under a new tuple.

### Promotion-occurrence backfill (evidence-gated)

Existing promoted beliefs receive `belief_promotion` occurrence rows **only where the migration has evidence of a real promotion occurrence** (e.g., an audited promotion event in the operator/transcript record). Where historical promotion cannot be established:

- no occurrence row is created;
- no epoch is fabricated (no default epoch 1);
- dependent new authority relations (justification, snapshot, activation) fail closed — the FKs cannot be satisfied.

The demo corpora (`track1`/`track2`) therefore carry no authority under the new model unless their promotion history is explicitly evidenced. This is deliberate: fail closed rather than fabricate provenance.

### `belief_tenant` backfill

One mapping row per existing belief via the explicit fixed legacy `scenario_id → tenant` mapping. No inference from runtime values. `scenario_id` is never treated as authenticated tenancy.

### Transition window

While the new authority model is being introduced, the existing `solvent_authorize_action` path (frozen kernel/MCP, live today) **cannot grant new production authority**; it remains scoped to the legacy demo semantics. The old implementation is not modified in this task; the window rule is an operational/deployment constraint stated here as the contract the rollout must honor.

### Rollback

Rollback must not restore authority by interpreting newly required dimensions as optional. Legacy rows remain non-authorizing until explicit re-authorization.

---

## 17. DocTrust Flows

DocTrust exercises Solvent only through public interfaces. Deterministic expected outcomes:

| # | Flow | Expected outcome |
|---|---|---|
| 1 | CreateTarget(tuple) | target row exists; proposal tuple stored verbatim; read-time state derives `PROPOSED` |
| 2 | AttachJustification (valid promotion occurrence) | justification row exists; duplicate attach is a no-op (idempotent) |
| 3 | AttachJustification with fabricated epoch | FK violation (`23503`), surfaced verbatim |
| 4 | RequestAuthorization | `requested_by/requested_at/pinned_request_hash` set; pin matches current state |
| 5 | Approve (valid pin, all preconditions) | snapshot + activation inserted (1:1), read-time state derives `ACTIVE`; second Approve returns REVIEW |
| 6 | Tuple mismatch at execution | DENY — presented tuple ≠ snapshot tuple, exact equality, server-side |
| 7 | Retract a supporting belief → execution attempt | DENY (verification fails: status or current-occurrence mismatch); snapshot unchanged |
| 8 | Retract → re-promote → execution attempt | still DENY for the old snapshot (current occurrence ≠ pinned epoch) — no revival |
| 9 | Duplicate discharge (same instrument/obligation) | unique violation (`23505`), surfaced verbatim; no second acceptance |
| 10 | Cross-tenant relation (same UUID, different tenant: principal / policy / belief / target / snapshot) | FK violation on every edge |
| 11 | Revoked approver attempts Approve | refusal on fresh state |
| 12 | Policy downgrade then execution | old approval never broadened; restrictions apply at re-verification |
| 13 | Missing dimension (legacy path) | NOT AUTHORIZED |
| 14 | Snapshot substitution attempt (UPDATE/DELETE on authority tables as application role) | permission denied — no UPDATE/DELETE grant |
| 15 | Proposal mutation after request, then Approve | pin-hash divergence → typed refusal, re-request required |
| 16 | Activation resurrection attempt (second activation / DELETE of revocation) | structurally impossible (insert-only facts + UNIQUE over history) |

### What DocTrust proves

That the public integration exercises the repaired relational model as intended for the DocTrust domain, with the deterministic outcomes above.

### What DocTrust does not prove

- domain-agnostic completeness;
- external IdP correctness (including authenticity of `issued_at`);
- semantic truth of the document evidence;
- execution-effect verification (I-13 — FUTURE / UNSCOPED);
- production deployment mediation (Executor Mediation is a deployment precondition);
- service-compromise resistance (compromised service = TCB failure);
- future authority-composition analysis.

---

## 18. Formal Model Mapping

No Lean changes occur in this phase. No claim that current Lean proves the new schema.

| Property | Classification |
|---|---|
| promoted implies debt-free; live intent implies promoted; retract-cascade atomicity | existing theorem (current model scope: `Types/Transitions/Invariants/Preservation.lean`, 11 theorems, zero `sorry`) |
| justification references a committed promotion occurrence (epoch referent) | future theorem + DB property (FK) |
| activation monotonicity (one activation per target, ever; no resurrection) | future theorem + DB property (UNIQUE + insert-only) |
| snapshot is sole execution authority | future theorem + DB/service property |
| tenant-aware relationship integrity | future theorem + DB property (composite FK graph) |
| discharge replay uniqueness | future theorem + DB property (UNIQUE) + service property (canonicalization) |
| policy monotonic revision | future theorem + DB property (UNIQUE) + service property (allocation) |
| approval pin coherence | future theorem + transactional service property |
| authorization-target matching | PROPOSED source-spec property (I-12 label), service-enforced |
| execution-effect matching | FUTURE / UNSCOPED (I-13 label) |
| executor mediation | DOCUMENTED CONTRACT (deployment precondition) |

Existing theorems: `live_intent_implies_promoted`, `no_live_intent_on_retracted_belief`, `promotion_updates_dependent_intent`, `cascade_retraction_*`, `*_preserves_validity` — none formalizes targets, snapshots, epochs, tenants, policy, or discharge, and none is cited as evidence for the new schema.

---

## 19. Schema Security Claims

| Claim | Primary enforcement | Secondary enforcement | Status |
|---|---|---|---|
| Belief promotion is debt-free | DB (`promoted_is_debt_free`, CURRENT) | Kernel/service | CURRENT |
| Live intent requires promotion | DB (`gate`, `live_requires_promoted`, CURRENT) | Kernel | CURRENT |
| Justification references a committed promotion occurrence | DB (FK to `belief_promotion`) | Kernel promotion tx | PROPOSED |
| Active authority uses exact approved snapshot | DB (activation FK graph) + Service | — | PROPOSED |
| Authority cannot be repointed, substituted, or resurrected | DB (insert-only facts, `UNIQUE(tenant_id, target_id)` over history, no UPDATE/DELETE grants, no pointer column) | Service transaction contracts | PROPOSED |
| Proposal tuple cannot change execution authority | DB (resolution never reads it) + TX (in-tx pin recompute) | Service | PROPOSED |
| Approval approves exactly the reviewed candidate | TX (in-tx pin recompute + growth check) | Service | PROPOSED |
| Re-promotion cannot revive old authorization | DB (occurrence FK) + Service (current-occurrence verification) | — | PROPOSED |
| Discharge instrument replay is prevented | DB (unconditional UNIQUE) | Service (canonicalization — stated residual) | PROPOSED |
| Obligation meaning is policy-version bound **in the ledger** | DB (FK) | — | PROPOSED |
| Transitional `debt[]` semantics distinguish policy versions | — | — | **NOT CLAIMED — accepted transitional residual (C-4)** |
| Policy revisions are monotonic per tenant | DB (`UNIQUE(tenant_id, revision)`) + TX (max+1 allocation) | Service | PROPOSED |
| Policy downgrade never broadens approved authority | TX/Service (re-verification) | Policy | PROPOSED |
| Cross-tenant authority relationships are impossible | DB (composite FK graph with declared target keys) | Service | PROPOSED |
| Revoked principal cannot approve anew | TX (fresh-state check) + Service | Identity | PROPOSED |
| Revoked principal cannot create a new discharge (predicate b) | TX (fresh-state check) + Service | Identity | PROPOSED |
| Post-revocation attestation issuance is refused (predicate a) | TX (discharge tx checks `issued_at < issuing principal.revoked_at`) | Identity (authenticity of `issued_at`) | PROPOSED |
| Missing dimensions never become wildcard | DB (NOT NULL + CHECKs) + Service refusal | — | PROPOSED |
| Empty representations never become wildcard | DB (CHECKs per §12) + exact-equality matching | — | PROPOSED |
| Execution tuple matches authorized tuple | Service exact equality | Executor echo contract | PROPOSED (I-12 label) |
| Executor actually produced world effect | — | future trusted executor/receipt | FUTURE / UNSCOPED (I-13 label) |
| Consequential executor has no unmediated path | Deployment/integration | auditing | DOCUMENTED CONTRACT |
| Credential lifecycle is controlled by Solvent | — | — | FALSE / OUT OF SCOPE (IdP-owned) |
| Semantic truth of evidence is guaranteed by DB | — | — | FALSE / OUT OF SCOPE |

### Executor Mediation contract

> A consequential action governed by Solvent must not have an unmediated execution path around the authorized executor.

Deployment precondition, not a kernel/DB invariant.

---

## 20. Tradeoffs

| Decision | Alternative | Selection | Reason |
|---|---|---|---|
| Activation representation | fold activation into snapshot; mutable pointer | separate append-only `target_activation` + `target_revocation` facts; pointer column removed | the activation fact is itself authority-bearing and immutable; every forbidden lifecycle state becomes an INSERT-constraint violation; resurrection needs a DELETE the role lacks |
| Authority state storage | stored `state` mirror; read-time derivation | **read-time derivation only**; no stored lifecycle column | a mutable mirror can drift from the facts and become a second authority source; the facts are authoritative and the states are query projections |
| Proposal location | separate `target_proposal` table | proposal tuple columns on `authority_target`, frozen by copy at approval | smallest model satisfying the four-operation lifecycle; post-approval mutability is harmless because resolution never reads it |
| Approval pin | timestamp only; hash; workflow engine | timestamp + deterministic pin hash with defined canonicalization, verified in the activation transaction | timestamp alone pins only the (append-only) justification set, not the mutable proposal tuple; the hash pins both; in-tx verification closes the TOCTOU gap; no workflow engine |
| Uniqueness placement | `UNIQUE(tenant_id, target_id)` on snapshot | on `target_activation` (entire history) | activation is the authority-granting fact; snapshot is content; no duplicated uniqueness |
| Epoch referent | additive `belief.promotion_epoch` (frozen-core exception) | `belief_promotion` side table (locked) | preserves frozen core; real FK target; occurrence rows, not a counter; evidence-gated backfill without fabricating history |
| Discharge status | mutable `accepted→retracted` | no status column; revocation FUTURE / UNSCOPED as append-only counter-entry | mutable status reopens replay; speculative values are design debt |
| Obligation identity | version-qualified keys | stable key + policy binding (ledger); transitional residual honestly labeled (C-4) | preserves stable vocabulary; makes the gap explicit instead of claiming enforcement that does not exist |
| Policy ordering | `effective_at`; content hash | per-tenant `revision` UNIQUE + transactional max+1 | deterministic ordering independent of clocks and content |
| Tenant binding | additive `tenant_id` on frozen belief | `belief_tenant` insert-only association | frozen core preserved; reassignment structurally excluded |
| Principal type transitions | mutable type history | immutable type; change = new linked identity | avoids identity ambiguity |
| Evidence reference | FK to evidence rows | canonical `(instrument_kind, instrument_ref)` | leaves Artifact/Citation migration open; avoids false provenance claims |

### Rejected complexity (not in MVP)

credential/key table; IAM/role graph; obligation event-sourcing engine; execution receipt system; authority-composition graph; reachability engine; temporal authority engine; persistent warrant table; full tenant service; workflow engine for approval pinning.

---

## 21. DDL Readiness

Evaluated **after** the repair, against the Repair-24 checklist — not assumed in advance. Criterion: READY only if a competent DDL author can implement the relational model without inventing security semantics.

- proposal representation exists (§5.2 — full tuple, requester fields, pin);
- activation representation exists (§5.4 — insert-only authority-bearing fact, `UNIQUE(tenant_id, target_id)` over entire history, 3-column snapshot FK);
- snapshot key graph exists (§5.3 — PK + declared `UNIQUE(tenant_id, target_id, snapshot_id)` + four tenant-composite FKs);
- tenant key graph exists (`UNIQUE(tenant_id, principal_id)` on principal; `(tenant_id, policy_version_id)` PK on policy; composite FKs on every tenant edge — §6);
- promotion referent exists (§5.7 — `belief_promotion` occurrence rows with epoch UNIQUEs and FKs);
- replay semantics exist (§5.8 — unconditional UNIQUE, no status column, canonical instrument identity);
- approval pin exists (§6.4 — durable columns, defined canonicalization, in-activation-transaction verification contract);
- lifecycle is non-resurrectable (insert-only facts; no DELETE grant; no state-predicate uniqueness; read-time state derivation);
- transitional C-4 residual is honestly stated (§3 C-4, §7, §10, §19);
- all critical relationships have actual target keys (§6 edge table);
- impossible-state claims match mechanisms (§7 — every DB cell names a constraint or named fact structure);
- concurrency semantics are executable as written (§15 — reads, writes, serialization points, retry contracts, in-tx pin verification).

**Verdict: READY FOR DDL.** Every semantic choice above is resolved in this document; none is deferred to the DDL author.

**DDL-time verifications (implementation work, not open semantics):** exact `REVOKE`/`GRANT` statements for the application role against the pinned CockroachDB version (the security property — insert-only authority tables — is fixed here; the syntax is verified at DDL); `ON CONFLICT DO NOTHING` support for idempotent attach. All UNIQUE constraints in this design are unconditional, so no partial-index dependency exists.

---

## 22. Final Dogfood Test

### Evidence supporting the design

Every blocking finding from the implementation-gate review (SDAR-1..9) is resolved by a named relational mechanism, not a disclaimer; the non-blocking findings (SDAR-10..21) are resolved or explicitly sequenced (§24 cross-tab). The semantic corpus survived a hostile review unchanged; the relational representation was rebuilt around it.

### Assumptions

- the Solvent service is inside the MVP TCB (compromise = kernel compromise; the privilege boundary defends against accidents, not compromise);
- Identity authenticates principals externally and attests `issued_at`;
- policy semantics remain Service/Policy responsibilities;
- the application role can be privilege-separated from DB administration;
- the kernel will gain the promotion-occurrence insert and the discharge-transaction pairing when DDL lands (design constraint stated; no kernel change in this task).

### Open debt

- DDL-time privilege statement verification (implementation, not semantics);
- obligation-rows long-term model (eliminates the C-4 transitional residual);
- discharge revocation counter-entry (FUTURE / UNSCOPED);
- artifact/citation evidence model;
- production tenant onboarding runbook (backfill execution, evidence audit for legacy promotions).

### Falsifiers

The design must be reassessed if any of these are demonstrated:

- an application-role UPDATE that changes authority resolution;
- a second activation for an ever-activated target;
- a justification bound to a non-committed promotion occurrence;
- a stale epoch revalidating after re-promotion;
- one canonical instrument discharging the same `(obligation_key, policy_version_id)` twice;
- a cross-tenant edge committing;
- an approval retry enlarging the approved set;
- a post-revocation attestation accepted contrary to predicate (a), or a revoked discharger accepted contrary to predicate (b);
- a DDL attempt requiring a security decision this document does not make.

### Reassessment triggers

new customer workflow; new authority dimension; new executor class; evidence provenance requirement; delegated authority; a validated HIGH_IMPACT floor case (I-P3); observed migration ambiguity; CockroachDB behavior that invalidates an enforcement assumption.

### Dogfood conclusion

The design follows Solvent's method and its own review discipline: forbidden states are made impossible by relationships, transitional gaps are labeled rather than marketed, lifecycle truth lives in immutable facts with read-time derivation (no mirror to drift), and the authority boundary is a single, relational, insert-only fact structure. A coherent schema is not treated as proof; the next artifact is the adversarial re-review of this document and then DDL.

---

## 23. Validation

1. Entire document re-read post-write.
2. C-1..C-7 present and implemented in the relational design (§3, §5): C-1 authority lifecycle/activation/snapshot (facts + privileges + read-time derivation), C-2 promotion epoch (`belief_promotion` occurrence rows), C-3 instrument replay (unconditional UNIQUE), C-4 obligation identity (honest residual), C-5 policy revision (UNIQUE + allocation), C-6 `belief_tenant` (FK + immutable 1:1), C-7 attestation validity (two distinct predicates, justified boundary).
3. SDAR-1..21 each resolved, bounded, or honestly open — see §24 cross-tab.
4. No new I-* invariant IDs invented.
5. I-12 = PROPOSED source-spec label; I-13 = FUTURE / UNSCOPED; Executor Mediation = DOCUMENTED CONTRACT.
6. No `approver_principal_type` duplication remains (§5.3).
7. No `expired` target state exists anywhere.
8. No mutable accepted→retracted discharge lifecycle exists (§5.8 — no status column).
9. No snapshot-vs-live semantic ambiguity remains (§8 — snapshot is sole authority; live rows are audit).
10. No authority-bearing pointer can be silently repointed (§5.2 — column removed; §5.4 — insert-only fact).
11. The proposal exists durably before approval, through all four operations (§5.2, §6.4).
12. `promotion_epoch` is bound to an actual committed promotion record (§5.7 FK; occurrence-row rule).
13. `belief_tenant` references an actual belief and is immutable one-to-one (§5.10).
14. Every tenant-scoped relationship has an actual composite key target (§6).
15. Approval retries cannot silently add justifications (§6.4, §15).
16. Policy binding is not claimed stronger than transitional debt semantics provide (§3 C-4, §19).
17. Principal rotation does not create a new principal identity (§5.1, §11).
18. Attestation revocation semantics are explicit — two distinct predicates (§3 C-7).
19. Retraction invalidates the next pre-execution verification of the corresponding old promotion occurrence (§8).
20. Later re-promotion does not revive the old snapshot justification (§8 — occurrence comparison).
21. Empty/NULL authority dimensions cannot become wildcard semantics (§12 table + exact equality).
22. No second authority source introduced (Second-Source audit below).
23. No execution-effect guarantee claimed (§19).
24. DocTrust tests the repaired relational model (§17).
25. No DDL written.
26. No repository implementation files modified (verified via `git status --short` in the delivery report).
27. No commit; no sync.

### Second-Source-of-Truth audit

| Duplicated fact | Authoritative copy | Historical copy | Derived copy | Divergence prevention |
|---|---|---|---|---|
| Proposal tuple vs snapshot tuple | snapshot (post-approval) | proposal columns (pre-approval history) | — | pin hash detects pre-approval divergence; resolution never reads proposal; copy is verbatim in the approval tx |
| Justification set: live rows vs `justification_set` blob | blob for execution (inside snapshot); live rows for audit | blob is also the historical record | — | pin hash + growth check at approval; post-approval divergence is expected (live rows keep growing) and harmless |
| Lifecycle: facts vs derived states | `target_activation` / `target_revocation` facts | — | `PROPOSED/ACTIVE/REVOKED` derived at read time | **no stored state exists to drift**; authorization logic consults only facts |
| Promotion occurrence: `belief_promotion` row vs epoch in justification vs epoch in blob | `belief_promotion` row | justification FK value; blob copy | "current occurrence" = max(epoch), derived | justification FK must match a committed occurrence row; blob copy frozen at approval; current-occurrence verification rule uses the derived max |
| Debt: `belief.debt[]` vs `debt_discharge` | array (promotion gate, transitional) | ledger (attribution/replay/policy meaning) | — | single-tx rule prevents presence divergence; **meaning** divergence is the accepted C-4 residual, labeled |
| Belief status: `belief.status` vs `action_intent.belief_status` | `belief.status` | gate-cached copy, `ON UPDATE CASCADE` | — | CURRENT, verified constraint behavior (inherited) |
| Policy: `policy_content` vs `policy_hash` | content | hash | — | hash defined as SHA-256 of exact stored bytes; storage-integrity metadata only |
| `authority_target.snapshot_id` | **removed** | — | — | no such column exists anywhere in this design |
| `authority_target.state` | **removed** (read-time derivation) | — | derived states | no stored mirror exists to drift |

`authority_target.snapshot_id` and any stored lifecycle state were **removed entirely**: `target_activation`/`target_revocation` make them unnecessary, and retaining them in any form would preserve an independently mutable authority-adjacent field or a drift-prone mirror.

---

## 24. Repair Coverage

Every review finding appears exactly once. The nine blocking findings each map to exactly one of the seven original DDL conditions, with C-1 scoped as **authority lifecycle, activation binding, and snapshot immutability**. No duplicated blocker count.

| Review Finding | Schema Section | Resolution | Status |
|---|---|---|---|
| SDAR-1 (mutable authority pointer) — **blocking, C-1** | §5.2, §5.4, §8 | `snapshot_id` column removed; `target_activation` insert-only authority-bearing fact binds snapshot by 3-column FK; `UNIQUE(tenant_id, target_id)` over entire history; no UPDATE/DELETE grants on authority tables | RESOLVED |
| SDAR-2 (unrepresentable proposal) — **blocking, C-1** | §5.2 | full proposal tuple + requester/pin columns on `authority_target`; mutable only pre-approval; frozen by copy at approval | RESOLVED |
| SDAR-3 (unanchored snapshot) — **blocking, C-1** | §5.3 | PK + declared `UNIQUE(tenant_id, target_id, snapshot_id)` + four tenant-composite FKs with declared targets | RESOLVED |
| SDAR-4 (conceptual epoch) — **blocking, C-2** | §5.7, §5.6, §9 | `belief_promotion` side table (locked direction); occurrence rows, not a counter; justification FK `(tenant_id, belief_id, promotion_epoch)`; epoch ≥ 1; evidence-gated backfill | RESOLVED |
| SDAR-5 (revocation replay hole) — **blocking, C-3** | §5.8, §10 | no status column; accepted rows immutable; revocation FUTURE / UNSCOPED (append-only counter-entry when real) | RESOLVED |
| SDAR-6 (transitional obligation truth) — **blocking, C-4** | §3 C-4, §7, §10, §19 | honest labeling: ledger binding authoritative; `debt[]` transitional and NOT structurally protected; residual accepted and stated in the impossible-state matrix | RESOLVED (labeled residual) |
| SDAR-7 (tenant composite-key graph) — **blocking, C-6** | §5.1, §5.9, §6 | `UNIQUE(tenant_id, principal_id)`; `(tenant_id, policy_version_id)` PK; every principal/policy edge tenant-composite with named columns | RESOLVED |
| SDAR-8 (approval retry set growth) — **blocking, C-1** | §6.4, §15 | pin = `requested_by/requested_at/pinned_request_hash` with defined canonicalization; recompute-and-compare inside the activation transaction; retry never enlarges the set | RESOLVED |
| SDAR-9 (lifecycle resurrection) — **blocking, C-1** | §5.4, §5.5, §8 | activation UNIQUE over entire history (no state predicate); revocation as insert-only fact; no DELETE grant; lifecycle derived from facts at read time | RESOLVED |
| SDAR-10 (`belief_tenant` scaffolding) | §5.10 | FK to belief; insert-only immutability; provisioning-only insertion; fail-closed missing mapping | RESOLVED |
| SDAR-11 (requester identity) | §5.2 | `requested_by`/`requested_at`; `created_by`/`requested_by`/`approved_by`/`discharged_by` distinct | RESOLVED |
| SDAR-12 (empty-value wildcards) | §12 | field-by-field table; CHECKs; exact-equality excludes wildcard semantics | RESOLVED |
| SDAR-13 (attestation boundary) | §3 C-7, §5.8 | `issued_at < revoked_at` committed and justified (predicate a); `discharged_by.revoked_at IS NULL` (predicate b); two predicates, never collapsed; `issued_at` = IdP-trusted | RESOLVED |
| SDAR-14 (policy allocation) | §5.9 | max+1 in-tx; 40001 retry; first revision 1; CHECK ≥ 1; UNIQUE stated as the enforcement (PK does not order) | RESOLVED |
| SDAR-15 (justification natural key) | §5.6 | `justification_id` PK + `UNIQUE(tenant_id, target_id, belief_id, promotion_epoch)`; idempotent attach | RESOLVED |
| SDAR-16 (instrument identity) | §3 C-3, §5.8, §14 | `instrument_kind` + canonical reference forms; canonicalization at service ingress; residual stated | RESOLVED (stated residual) |
| SDAR-17 (approver type duplication) | §5.3 | column removed; I-A4 reads the principal row | RESOLVED |
| SDAR-18 (epoch backfill / transition window) | §16 | evidence-gated backfill, no fabricated epochs, fail-closed; old path grants no new production authority | RESOLVED |
| SDAR-19 (retraction after approval) | §8, §16 | pinned MVP rule: re-verification against current status AND current promotion occurrence; re-promotion never revives; DocTrust flow 7 deterministic | RESOLVED |
| SDAR-20 (spec-defect phrasings) | §5.1, §5.6, §5.8 | `revoked_at` write-once rule; real PKs; no speculative status domain; named type CHECK | RESOLVED |
| SDAR-21 (policy hash) | §5.9 | hash = SHA-256 of exact stored bytes; integrity-of-storage; not version identity | RESOLVED |

Blocking mapping check: SDAR-1→C-1, SDAR-2→C-1, SDAR-3→C-1, SDAR-8→C-1, SDAR-9→C-1 (C-1 = authority lifecycle, activation binding, and snapshot immutability); SDAR-4→C-2; SDAR-5→C-3; SDAR-6→C-4; SDAR-7→C-6. Each blocking finding maps to exactly one of the seven conditions; C-5 and C-7 carry only non-blocking findings (SDAR-14, SDAR-13), consistent with the review's cross-tab. Nine unique blockers, seven conditions, no duplicated blocker count.
