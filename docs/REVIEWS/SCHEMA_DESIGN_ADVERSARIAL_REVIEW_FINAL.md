# Solvent Schema Design — Adversarial Review

## Status

**REVIEW ARTIFACT — NOT IMPLEMENTED**
**Reviewer:** Kilo (adversarial, independent)
**Date:** 2026-08-31
**Artifact under review:** `/home/chaschel/Desktop/biz/solvent/v2/plan5.2.md` (candidate relational model)
**Supporting architecture:** `docs/ARCHITECTURE/schema-readiness-decision.md`, `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`, `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md`, `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`
**Repository truth:** `db/001_schema.sql`, `kernel/kernel.go`, `internal/view/view.go`, `cmd/solvent-mcp/tools.go`, `formal/lean/`

No code, SQL, Lean, MCP, REST, ADR, or architecture specification was modified. Only this review file was created.

---

## Review Posture

- **Candidate schema, not DDL.** `plan5.2.md` is a semantic model. It contains no `CREATE TABLE`, no column types, no constraint names, no migration filenames, no Go signatures, and no MCP tool definitions.
- **Implementation not authorized.** This review does not authorize DDL, kernel changes, or service extraction. Its sole purpose is to determine whether the candidate relational model is safe enough to become the basis for DDL.
- **Repository is source of truth.** Where `plan5.2.md` and the repository disagree, the repository wins.
- **Invariant IDs are earned, not assigned.** The only repository-verified invariants are `promoted_is_debt_free` (`23514`), `gate` (`23503`), `live_requires_promoted` (`23514`), `belief_id_status_key`, I-7 (`crdb.ExecuteTx`), and I-8 (`RetractCascade` atomicity). Proposed labels such as `I-12`, `I-13`, `I-A4`, `I-P1`, `I-P2`, `N1`–`N4`, and `D1`–`D16` are **DOCUMENTED CONTRACT** only. They are not earned repository invariants and must never be narrated as if they were.

---

## Executive Verdict

**APPROVE WITH CONDITIONS**

The candidate schema in `plan5.2.md` is a substantial and honest improvement over Plan 5.1. It correctly removes duplicated identity attributes (`approver_principal_type`), eliminates `ON DELETE CASCADE` on authority-bearing relationships, drops the premature `expired` state, makes `risk_tier` service-derived, and restores discharge attribution as a semantic requirement. The core authority model is derived from durable facts (F1–F7) rather than anchored to a table count.

However, the design is **not yet safe to translate into DDL**. Seven independent blocking decisions remain that would, if implemented verbatim, permit authority laundering, stale-authorization paths, or cross-tenant relationships. These are not polish issues; they are relational/semantic gaps that must be closed before any `CREATE TABLE` is written. The first decision has two corollaries.

The conditions for DDL are listed in `## Exact Conditions Before DDL`.

---

## What the Schema Design Correctly Fixes Over Plan 5.1

| Plan 5.1 Problem | plan5.2 Resolution |
|---|---|
| `approver_principal_type` duplicated on approval row, creating identity-divergence attack surface | **Removed.** Approval references `principal` via FK; I-A4 derives type from the principal row, never from a copied column. Composite-FK propagation vs transactional enforcement left as OPEN schema-design question (Finding 1). |
| `ON DELETE CASCADE` on justification would destroy authority history | **Removed.** Authority-bearing objects are append-only. State transitions (`proposed → active → revoked`) replace deletion. No cascade deletion on authority-bearing relationships (Finding 3). |
| `risk_tier TEXT` persisted on target, creating divergence between stored tier and policy interpretation | **Removed.** `risk_tier` is service-derived from `consequence` + current `policy_version`. Approval snapshot may record the tier in effect at approval time, but it is not an independent mutable column (Finding 6). |
| `expired` target state introduced temporal semantics that do not yet exist | **Removed.** MVP state machine is `proposed → active → revoked`. Temporal authority is deferred (Finding 9). |
| `target_approval` and `debt_config` tables introduced without durable-fact derivation | **Removed.** Approval snapshot lives on `authority_target` per schema-readiness decision D6/D13. `debt_config` is a candidate, not a locked object (Finding 5). |
| Discharge API lost attribution (`RecordDischargeAndRetireDebt` had no `principal`) | **Restored.** `discharged_by → principal` is a non-negotiable semantic requirement (Finding 7). |
| Obligation identity missing version and policy context | **Partially restored.** `debt_discharge` binds `(obligation_key + policy_version)` context. Whether version-qualified keys or stable keys + policy binding suffice remains OPEN (Finding 8). |
| Target/link creation authority weakly bounded | **Clarified.** Creation is permissive (authenticated to tenant); approval is the security boundary (I-A4, D5, D6). Scoped creation authority is future tightening, not MVP gate (Finding 10). |
| Policy hash treated as version identity | **Corrected.** Policy version is a durable object with `identity, content hash, effective_at, tenant_id, author_principal`. The approval snapshot binds version identity, not hash alone (Finding 11). |

---

## Critical Findings

### F-5.2-1 — CRITICAL — Proposed semantic decision: `target_snapshot` is the sole execution-time authority

- **Severity:** CRITICAL
- **Claim attacked:** "No freestanding `state` column on justification — a link's acceptance is embodied in the approval snapshot" (`plan5.2.md:391`); "DB: state transitions only (no tuple mutation after creation, mechanism TBD)" (`plan5.2.md:364`); "Approval snapshot (at activation): target tuple, justification set..." in the candidate model (`plan5.2.md:359`).
- **Evidence:** `plan5.2.md:375-395` justification object definition; `plan5.2.md:588` cross-object properties table; `plan5.2_review.md:41-81` (external review explicitly flags this semantic hole); `plan5.2.md:147-158` Finding 4 resolution (target immutability mechanism TBD).
- **Attack:** The plan correctly says the approval boundary is the semantic stop for the moved-broadening attack. But it also says "All proposed targets and links are enumerable and auditable" and "the approver sees every justification link before approving" (Finding 10). If live links are security-significant (they are — they are the justification set the approver evaluated), then removing their `proposed/accepted` state creates a gap: what prevents a post-approval link addition from changing the effective authorization? The plan acknowledges this in OPEN #5 ("leaning toward snapshot-embodied") but does not resolve it. Simultaneously, the target immutability mechanism is TBD, and the cross-object properties table claims DB enforcement for "state transitions only" while the object definition says the mechanism is unresolved.
- **Exact failure boundary:** An attacker with `AttachJustification` authority creates a new justification link after `Approve`, attaching a different promoted belief to an `active` target. Because the link has no `proposed/accepted` state and the approval snapshot is immutable, the live justification set now differs from the approved set. At execution time, does Solvent verify against the snapshot or the live set? The schema does not make this decision. Additionally, if the target's five-tuple remains mutable after creation (mechanism TBD), an attacker could mutate `resource` or `consequence` after approval, changing the authority without creating a new target.
- **Why the current design succeeds/fails:** It fails. The design is internally inconsistent: it makes the approval snapshot the authority boundary (good) while simultaneously treating live links as security-significant (also good), but it does not define which one controls at execution time. With snapshot-authoritative semantics, live links do not control execution-time authority; they remain proposal/audit history. If live links controlled authority, post-approval link addition would silently change authority — the exact moved-broadening attack the snapshot is designed to prevent. The target immutability gap compounds this: without a structural mechanism, the five-tuple itself is mutable.

- **Proposed Semantic Decision:** The snapshot-authoritative model is the fixed semantic direction for this design. Execution-time verification reads `target_snapshot`, never the live `justification` table or `authority_target`'s mutable columns. The live `justification` table is proposal/audit history, not an execution-time authority source. Any post-approval link addition creates a new `proposed` target requiring a new `Approve`. This single decision collapses three apparently separate semantic questions:
  - **C-1 (snapshot-authoritative):** Execution-time authority is the snapshot, not live links.
  - **Corollary A (separate representation):** `authority_target` carries lifecycle/reference fields such as `state` and `snapshot_id`; the immutable five-tuple, justification set, policy version, and approver live in `target_snapshot`.
  - **Corollary B (immutability):** `target_snapshot` is intended to be immutable; the DDL/security design must make unauthorized mutation impossible or explicitly reject it.

- **Remaining Implementation Debt:** The semantic model is resolved, but the physical DDL enforcement mechanism remains to be designed. The exact relational mechanism that makes snapshot immutability structurally enforceable is OPEN. Whether this requires a separate immutable table, an exclusion constraint, a trigger-based guard, or a combination is a DDL-design decision. The semantic model does not dictate the physical mechanism; it only constrains it. Any physical mechanism chosen must preserve the semantic invariant that `target_snapshot` is the sole execution-time authority and cannot be mutated after creation.

- **Blocking for DDL?** The semantic decision is PROPOSED / fixed direction. The physical DDL enforcement mechanism is OPEN implementation debt and blocks DDL. DDL cannot be written until the physical enforcement mechanism is selected and shown to preserve the proposed semantic model.

### F-5.2-2 — HIGH — `belief_id + belief_status` composite FK cannot distinguish retract→re-promote cycles

- **Severity:** HIGH
- **Claim attacked:** Composite FK `(belief_id, belief_status) → belief(id, status)` with `ON UPDATE CASCADE` as the per-link promotion-dependence mechanism (`plan5.2.md:386`).
- **Evidence:** `plan5.2.md:383-386` justification object definition; `db/001_schema.sql:34` existing `belief_id_status_key` UNIQUE constraint; `kernel/kernel.go:127-149` `RetractCascade` cancel-before-retract; `plan5.2_review.md:85-119` (external review explicitly attacks this lifecycle).
- **Attack:**
  1. Belief X enters (`status=entered`).
  2. Belief X promotes (`status=promoted`).
  3. Justification link J1 is created: `(X, promoted) → target T`.
  4. Belief X retracts (`status=retracted`). `ON UPDATE CASCADE` propagates to J1, making J1 `(X, retracted)`. J1 no longer satisfies the composite FK.
  5. Belief X re-promotes (`status=promoted`). `ON UPDATE CASCADE` propagates to J1, making J1 `(X, promoted)` again.
  6. J1 is now technically valid again, but it was authored under a different epistemic state.
- **Exact failure boundary:** The schema cannot distinguish "J1 was approved when X was first promoted" from "J1 was approved when X was re-promoted." If the belief's claim, evidence, or obligation set changed between the two promotions, J1's authorization is semantically different even though `(belief_id, status)` is identical.
- **Why the current design succeeds/fails:** It fails for the re-promote case. The composite FK correctly handles the simple promote→retract case (J1 becomes invalid), but it silently revives J1 on re-promotion. This is the deeper question plan5.2_review.md poses: "does `belief_id` identify the enduring proposition, or does promotion/retraction create materially different authorization-relevant states?"
- **Required resolution:** Either:
  - **(a) Status epochs:** Add a `status_epoch` or `promotion_version` to `belief` that increments on each promotion. The justification FK binds to `(belief_id, status_epoch)` instead of `(belief_id, status)`. This makes each promotion a distinct authorization-relevant state.
  - **(b) Snapshot-embedded justification:** The approval snapshot records the exact belief state (claim text, evidence IDs, debt state) at approval time. Re-promotion does not revive old justifications because the approver evaluated a specific snapshot, not a mutable belief row.
- **Blocking for DDL?** YES. The composite FK choice (`belief_id, status`) is a schema-level commitment that cannot be retroactively changed without rewriting the justification table's FK target.

### F-5.2-3 — HIGH — Discharge uniqueness per `(belief, obligation)` is insufficient to prevent instrument replay across beliefs

- **Severity:** HIGH
- **Claim attacked:** "DB: uniqueness per `(tenant_id, belief_id, obligation_key)` where `status = accepted`" (`plan5.2.md:412`).
- **Evidence:** `plan5.2.md:401-416` debt_discharge object definition; `plan5.2_review.md:153-198` (external review explicitly attacks instrument replay).
- **Attack:**
  1. External artifact A (e.g., a signed attestation) legitimately discharges `belief B1 / obligation X`.
  2. The same artifact A is presented for `belief B2 / obligation X`.
  3. Under per-belief uniqueness, both discharges are accepted because `(B1, X)` and `(B2, X)` are distinct.
  4. The attacker has replayed one instrument across two beliefs.
- **Exact failure boundary:** The schema's uniqueness scope is per-belief, but the replay risk is per-instrument. The same attestation payload, signed by the same principal, can discharge the same obligation for multiple beliefs because the uniqueness check is scoped to `belief_id`.
- **Why the current design succeeds/fails:** It fails for cross-belief replay. The plan correctly notes this is OPEN ("Discharge ledger uniqueness scope: per-belief or global — schema-readiness OPEN #3"), but the candidate schema commits to per-belief uniqueness without addressing the cross-belief case. This is not merely a uniqueness expression; it is a question of what makes an instrument "used."
- **Required resolution:** The schema must make instrument identity durable. The minimum is:
  - `instrument_ref` (evidence row ID or attestation reference) is a FK to a durable artifact identity.
  - The uniqueness constraint must include the instrument: `UNIQUE (tenant_id, obligation_key, instrument_ref) WHERE status = 'accepted'` for global-per-obligation uniqueness, OR the schema must record instrument usage at the artifact level.
  - If the design chooses per-belief uniqueness, it must add a service-layer check that prevents the same instrument from appearing in multiple `accepted` discharges — but that check is bypassable if the service is compromised (TCB boundary).
- **Blocking for DDL?** YES. The uniqueness expression is a DDL-level decision. Choosing per-belief now and discovering later that cross-belief replay is possible requires a schema migration.

### F-5.2-4 — HIGH — Policy version `effective_at` ordering is transactional, but concurrent creation and downgrade semantics are underspecified

- **Severity:** HIGH
- **Claim attacked:** "Temporal ordering (`effective_at`) — enforced transactionally, not row-local CHECK" (`plan5.2.md:427`).
- **Evidence:** `plan5.2.md:420-439` policy_version object; `plan5.2_review.md:200-228` (external review attacks temporal semantics); `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:331-364` policy change semantics; `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:52-56` policy version immutability.
- **Attack:**
  1. Transaction T1 creates Policy A with `effective_at = 10:00`.
  2. Transaction T2 creates Policy B with `effective_at = 10:05`.
  3. Concurrently, T3 creates Policy C with `effective_at = 09:00` (backdated).
  4. What prevents C from being treated as "earlier" than A under a clock-skew scenario?
  5. A tenant downgrades policy after `Approve`. What exactly is a "downgrade"? Is it weaker floor, or weaker obligation set, or both? How does the service detect it?
- **Exact failure boundary:** The schema says `effective_at` is enforced transactionally, but it does not define the transaction. CockroachDB's serializable isolation serializes conflicting writes, but two policy creations with different `effective_at` values may not conflict under serializable isolation if they touch different rows. The schema does not define a total ordering mechanism.
- **Why the current design succeeds/fails:** It fails for concurrent policy creation and downgrade detection. "Transactional ordering" is a service-layer promise, not a DB-enforced invariant. A compromised or buggy service can write policies out of order, and the schema provides no structural guard.
- **Required resolution:** Define the ordering mechanism explicitly:
  - **Option A: Monotonic version sequence.** A `version` integer or ULID that is strictly increasing per tenant. `effective_at` is informational; the version sequence is the authoritative order.
  - **Option B: Transactional write gate.** Policy creation is serialized through a single-row "policy version counter" per tenant, ensuring every new version has a higher sequence than the last committed version.
  - **Downgrade detection:** The service must compare `approval_time_policy_version` with `current_policy_version` using the sequence, not `effective_at` timestamps. A "downgrade" is defined as `current_version.sequence < approval_version.sequence` with `current_version.floor_weaker(approval_version)`.
- **Blocking for DDL?** YES. The `policy_version` table's primary key and ordering mechanism are DDL decisions. Without a defined sequence, DDL cannot specify the uniqueness or ordering constraint.

### F-5.2-5 — HIGH — `author_principal → principal` establishes attribution, not authorization to author policy

- **Severity:** HIGH
- **Claim attacked:** "`author_principal` → principal (human admin, never agent)" (`plan5.2.md:429`).
- **Evidence:** `plan5.2.md:420-439` policy_version object; `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:331-364` policy versioning; `plan5.2_review.md:232-265` (external review explicitly flags this distinction).
- **Attack:**
  1. Attacker controls an agent that can call the policy-creation API.
  2. The agent creates a `policy_version` with `author_principal = <attacker-controlled principal>`.
  3. The policy version is formally valid (it has a `principal` FK), but the principal was never authorized to author tenant policy.
  4. The new policy weakens the floor, allowing the attacker's target to be approved.
- **Exact failure boundary:** The schema makes attribution (`policy version was authored by principal P`) look like authorization (`principal P is permitted to author policy`). These are different properties. The schema enforces the first; the second requires a service-layer authorization check that does not yet exist.
- **Why the current design succeeds/fails:** It fails because the schema conflates identity persistence with role authorization. A `principal` FK on `policy_version.author_principal` ensures the row references a valid principal, but it does not ensure that principal is in the "policy author" role. A compromised service can bypass the role check.
- **Required resolution:** The schema must not claim `author_principal` as an authorization control. The explicit distinction must be:
  - **DB:** `policy_version.author_principal → principal` (attribution only).
  - **Identity:** `principal P is authentic` (IdP assertion).
  - **Service/Policy:** `principal P is permitted to author policy for this tenant` (role check).
  - The schema may add a `policy_author` role table or role flag on `principal`, but the authorization decision remains service-layer.
- **Blocking for DDL?** NO. This is a service-layer authorization gap, not a schema defect. But the schema documentation must explicitly distinguish attribution from authorization, and DDL must not include a CHECK or FK that implies the latter.

### F-5.2-7 — MEDIUM — Tenant binding is conceptually correct but migration from `scenario_id` is unengineered

- **Severity:** MEDIUM
- **Claim attacked:** "Every cross-object relationship in the authority model must be composite tenant-aware" (`plan5.2.md:97-111`).
- **Evidence:** `plan5.2.md:97-111` Finding 2 resolution; `db/001_schema.sql:8,49,62` existing tables use `scenario_id`, not `tenant_id`; `cmd/solvent-mcp/main.go:31-34` current scenarios are fixed UUID maps (`track1=000...001`, `track2=000...002`); `docs/ARCHITECTURE/schema-readiness-decision.md:226-230` explicitly states `scenario_id` is a fixed local map, not an authenticated tenant identifier.
- **Attack:**
  1. Existing `belief` rows have `scenario_id = 000...001` (track1).
  2. New schema adds `tenant_id` to `justification` and requires `(tenant_id, belief_id) → (belief.tenant_id, belief_id)`.
  3. But `belief` has no `tenant_id` column. The FK cannot be created without adding `tenant_id` to `belief` first.
  4. Adding `tenant_id` to `belief` is a frozen-table change, which the architecture explicitly prohibits without approval (`AGENTS.md:52-54`).
- **Exact failure boundary:** The candidate schema's tenant-binding requirement is correct, but it cannot be implemented without modifying the frozen `belief` table (and potentially `evidence` and `action_intent`). The plan acknowledges this is OPEN but does not propose a migration strategy.
- **Why the current design succeeds/fails:** It fails for the migration question. The relational model is sound for new tables, but it does not address how existing rows participate in composite tenant-aware relationships. A schema that requires `(tenant_id, belief_id)` FK but cannot add `tenant_id` to `belief` is unbuildable without either (a) a frozen-core exception, or (b) a view/join-based workaround that loses structural enforcement.
- **Required resolution:** The schema design must specify the migration path:
  - Add `tenant_id` to `belief` as an additive column (requires explicit approval to modify frozen table).
  - OR: introduce a `belief_tenant` association table that does not modify `belief` but provides the composite FK target.
  - OR: for MVP, treat `scenario_id` as the tenant identifier and accept that it is not an authenticated tenant (document the limitation honestly).
- **Blocking for DDL?** YES for production. For MVP/DocTrust, it can be sequenced as: new tables with tenant binding first, belief migration second.

### F-5.2-8 — MEDIUM — Obligation identity in `debt_discharge` is still ambiguous between version-qualified keys and stable keys + policy binding

- **Severity:** MEDIUM
- **Claim attacked:** "Obligation identity: `obligation_key` + `policy_version` context" (`plan5.2.md:403`).
- **Evidence:** `plan5.2.md:401-416` debt_discharge object; `plan5.2.md:472-480` Remaining Open Questions #2; `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:366-385` obligation identity; `docs/ARCHITECTURE/schema-readiness-decision.md:386-390` CURRENT/VERIFIED debt vocabulary.
- **Attack:**
  1. Policy v1 defines `needReview` as `deterministic`.
  2. Policy v2 redefines `needReview` as `attested`.
  3. A discharge recorded under v1 for `needReview` is `deterministic`-qualified.
  4. Under v2, the same `obligation_key = needReview` requires `attested`.
  5. Does the discharge under v1 satisfy the obligation under v2?
- **Exact failure boundary:** The schema stores `(obligation_key, policy_version)` but does not define whether `needReview@v1` and `needReview@v2` are the same obligation or different obligations. If they are the same obligation, a v1 discharge satisfies v2. If they are different, the v2 obligation is unsatisfied. The schema's uniqueness constraint `UNIQUE (tenant_id, belief_id, obligation_key)` does not include `policy_version`, so it treats them as the same.
- **Why the current design succeeds/fails:** It fails for policy-change semantics. The plan correctly identifies this as OPEN but commits to a uniqueness scope that assumes obligation identity is key-only, not key+version.
- **Required resolution:** The schema design must choose:
  - **If version-qualified keys:** `obligation_key` includes version (`needReview@v1`), and uniqueness is `(tenant_id, belief_id, obligation_key)`.
  - **If stable keys + policy binding:** `obligation_key` is stable (`needReview`), and uniqueness is `(tenant_id, belief_id, obligation_key, policy_version)`. The discharge ledger records which policy version's obligation was discharged.
- **Blocking for DDL?** YES. The uniqueness expression on `debt_discharge` is a DDL decision. The two choices have different constraint definitions.

### F-5.2-10 — HIGH — Revocation-time attestation validity is underspecified

- **Severity:** HIGH
- **Claim attacked:** `principal.revoked_at` gates new approvals, but the schema does not define whether a previously issued attestation remains valid after its issuer is revoked.
- **Evidence:** `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:297-309` principal and identity boundary; `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:654-663` replay prevention (deferred); ADR-0001 attestation lifecycle deferred questions.
- **Attack:**
  1. Principal P (human approver) issues a valid attestation for `belief B / obligation X` at T1.
  2. Principal P is revoked at T2 (e.g., departure, compromise).
  3. The same attestation is presented at T3 against `belief C / obligation Y` (a different, previously-unused obligation).
  4. `revoked_at` blocks new approvals by P, but the attestation was already signed and could be presented later.
- **Exact failure boundary:** The schema does not distinguish between:
  - **Principal authorization revocation** — P may no longer approve new targets
  - **Attestation validity** — an attestation signed by P at T1 may still be a valid instrument at T3
  - **Attestation issuance time** — when was the attestation signed?
  - **Attestation use/discharge time** — when is the attestation being presented for discharge?
- **Why the current design succeeds/fails:** It fails because the schema conflates "principal is no longer authorized" with "all attestations from this principal are invalid." These are different policy decisions. A valid use case exists: P signs an attestation at T1, P is revoked at T2, but the attestation was for a legitimate pre-revocation action and should remain valid. The opposite case also exists: P's private key is compromised at T1, P is revoked at T2, and all attestations from P after T1 should be invalid.
- **Required resolution:** Before DDL, Solvent must define the validity relation between principal revocation and attestation issuance/use. Whether that ultimately requires `issued_at`, an attestation identity, selective revocation, or an external validity assertion is the consequence of that decision.
- **Blocking for DDL?** YES. The schema must record the chosen validity relation before the `debt_discharge` table can enforce it.

### F-5.2-11 — MEDIUM — Principal type immutability is stated but no transition representation exists

- **Severity:** MEDIUM
- **Claim attacked:** "`principal_type` (`human | agent | workload | service`) — immutable after creation" (`plan5.2.md:341`).
- **Evidence:** `plan5.2.md:335-348` principal object; `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:159-198` principal/identity decision (D1); `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:297-309` principal and identity boundary.
- **Attack:**
  1. A workload identity is provisioned as `workload`.
  2. The workload is reclassified as `service` after an infrastructure change.
  3. The `principal_type` is immutable, so the row cannot be updated.
  4. A legitimate type transition must not create another principal row with the same `principal_id`; for MVP, principal type may remain immutable and a future reclassification can introduce a new principal identity with an explicit historical relationship if a real customer requirement emerges.
  5. Historical approvals referencing the old principal row remain valid (attribution survives), but new approvals require the new row.
- **Exact failure boundary:** The schema does not represent legitimate principal-type transitions. It correctly preserves stable identity, but the transition path is intentionally unresolved.
- **Why the current design succeeds/fails:** It succeeds for the MVP assumption that principal type is immutable. It remains open only for a future requirement to reclassify an actor without changing the identity model.
- **Required resolution:** Keep `principal_type` immutable in MVP. If a future customer requirement demands reclassification, design a separate identity/history mechanism in a later review. Do not create a second row carrying the same stable `principal_id`.
- **Blocking for DDL?** NO. This can be resolved during DDL design. But it must be resolved before production, not deferred indefinitely.

### F-5.2-12 — LOW — Evidence-to-discharge reference assumes evidence rows outlive the discharge schema

- **Severity:** LOW
- **Claim attacked:** "`instrument_ref` — evidence row ID or attestation reference" (`plan5.2.md:405`).
- **Evidence:** `plan5.2.md:401-416` debt_discharge object; `db/001_schema.sql:47-58` current `evidence` table is belief-owned; `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:291-300` evidence artifact vs citation semantics.
- **Attack:** The current `evidence` table is belief-owned (`evidence.belief_id → belief.id`). A discharge ledger row referencing `evidence.id` ties the discharge to a specific belief's evidence row. If the same artifact is later cited by a different belief, the discharge's `instrument_ref` still points to the original belief's evidence row. This is correct for audit but creates a coupling between discharge and the originating belief's evidence that may not survive the eventual artifact/citation split.
- **Exact failure boundary:** The schema can safely reference current evidence rows, but it must not claim this relationship survives the artifact/citation transition. When evidence becomes artifact-owned, `instrument_ref` must reference the artifact, not the belief-owned evidence row.
- **Why the current design succeeds/fails:** It succeeds for the transitional model but makes an implicit coupling that will break when evidence is refactored. This is not a blocker for DDL, but it must be documented as a migration target.
- **Required resolution:** Document that `instrument_ref` references a belief-owned evidence row in the transitional model and will reference an artifact row after the artifact/citation split. Do not add a FK constraint that would prevent the transition.
- **Blocking for DDL?** NO.

---

## Durable-Fact Verdict

| Fact | Authority/Owner | Representation | Why Durable | Enforcement | Unresolved Problem |
|---|---|---|---|---|---|
| F1: Principal identity, type, revocation | Identity (authenticity) + Service (persistence) + DB (immutability, revocation refusal) | `principal(principal_id PK, principal_type CHECK, issuer, tenant_id, revoked_at)` | Approvals must remain auditable after credential rotation | DB (type immutability, revoked_at refusal on new approvals) + Identity (authenticity) | Credential lifecycle (deferred, D1); delegation as first-class relation (deferred) |
| F2: Immutable target tuple + activation state | DB (immutable snapshot enforcement to be designed) + Kernel (transition) + Service (tuple verification) | `authority_target` lifecycle state + reference to `target_snapshot` | A mutated active authorization is indistinguishable from a new one without durability | DB (physical snapshot immutability mechanism TBD) + Service (verification) | **Physical DDL enforcement mechanism** remains OPEN; semantic authority is already snapshot-authoritative |
| F3: Justification links bound to belief promotion state | Service (set-level check) + DB (belief status) | `justification(target_id, belief_id, belief_status, tenant_id)` with composite FK `ON UPDATE CASCADE` | The epistemic→authority hinge must be checkable at every use | DB (per-link FK shape same as `gate`) + Service (set-level AND) | **Retract→re-promote lifecycle** (F-5.2-2); snapshot is already the proposed execution-time authority |
| F4: Discharge records bound to obligation identity | Kernel (single-tx rule) + DB (ledger uniqueness) | `debt_discharge(belief_id, obligation_key + policy_version, instrument_ref, discharged_by, status, tenant_id)` | `promoted_is_debt_free` without a ledger is an unauditable array mutation | Kernel (`crdb.ExecuteTx` wraps discharge + array removal) + DB (uniqueness) | **Uniqueness scope** (F-5.2-3); obligation version identity (F-5.2-8) |
| F5: Policy version identity + approval binding | Kernel (transactional ordering) + Service (verification) + DB (append-only) | `policy_version(version_id, policy_hash, effective_at, tenant_id, author_principal)` | Historical meaning of an approval must be re-derivable forever | DB (append-only, no UPDATE/DELETE) + Service (version verification) | **Ordering mechanism** (F-5.2-4); downgrade detection semantics (OPEN) |
| F6: Approval snapshot | Service + DB | Separate immutable `target_snapshot` table | The human approved *this*, not a mutable object | DB (immutable snapshot) + Service (snapshot verification at execution) | **Physical DDL enforcement mechanism** (OPEN implementation debt) |
| F7: Tenant binding on authority-bearing rows | DB (composite keys) + Service | `tenant_id` on all authority tables + composite tenant-aware FKs | Cross-tenant authority must be structurally impossible, not checked | DB (composite FK) | **Migration from `scenario_id`** (F-5.2-7); belief table frozen |

---

## Principal Verdict

**APPROVE WITH CONDITIONS**

The `principal` object is correctly derived from F1 and is structurally sound:
- `principal_id` is the stable identity (D1).
- `principal_type` is immutable after creation (D1).
- `issuer` is the IdP/workload attestation service that asserted the type.
- `tenant_id` provides tenant isolation.
- `revoked_at` gates new approvals; historical attributions survive.

**Residual issues:**
  1. **Type transitions (F-5.2-11, MEDIUM):** The schema declares `principal_type` immutable but provides no representation for legitimate type transitions. For MVP, this may be handled by creating a new principal identity with an explicit historical relationship if a real customer requirement emerges; credential rotation itself never creates a new principal identity.
2. **Credential lifecycle (DEFERRED, D1):** No credential table is needed for MVP. The IdP owns credentials. Solvent persists only the stable principal reference.
3. **Delegation (DEFERRED):** A delegate is its own principal requiring its own `Approve`. Full delegation chains are OPEN.
4. **Principal identity vs issuer (ATTACK 1):** `principal_id` uniquely identifies the Solvent principal. `issuer` records the identity provider or workload-attestation authority currently asserting that principal. These are distinct concepts. Two different issuers producing the same `principal_id` are different credential assertions for the same principal, not different principals. During rotation, the new issuer assertion updates the `issuer` field on the same `principal_id` row. Rotation does not create a new principal identity. Whether issuer history itself needs durable representation can remain deferred.

**DB enforcement that survives service compromise:**
- `principal_type` CHECK (prevents `agent` from being stored as `human` if the service is compromised and tries to insert an agent as approver).
- `revoked_at` NOT NULL / CHECK (prevents new approvals referencing a revoked principal if the DB enforces it).
- `issuer` NOT NULL (prevents anonymous principals).

**What the DB cannot enforce:**
- Authenticity of the IdP assertion (that is Identity's job).
- Whether a principal is actually authorized to perform a role (that is Service/Policy's job).

---

## Tenant Verdict

**APPROVE WITH CONDITIONS**

The composite tenant-aware binding is the correct relational structure for D3 ("no cross-tenant authority relationship, ever"). The plan correctly specifies composite FKs on every cross-object relationship.

**Residual issue: F-5.2-7 (MEDIUM, BLOCKING for production DDL).** The existing `belief`, `evidence`, and `action_intent` tables use `scenario_id`, not `tenant_id`. The candidate schema requires `(tenant_id, belief_id)` FKs but cannot create them without adding `tenant_id` to the frozen `belief` table. This is a migration problem, not a schema-design problem, but it must be resolved before DDL can be written.

**For MVP/DocTrust:** The acceptable path is:
1. New authority tables (`principal`, `authority_target`, `justification`, `debt_discharge`, `policy_version`) all carry `tenant_id` with composite FKs.
2. Existing `belief` rows are backfilled with `tenant_id` derived from their `scenario_id` (e.g., `track1 → tenant_A`, `track2 → tenant_B`).
3. This requires an explicit exception to the frozen-core rule for `belief.tenant_id` — acceptable because it does not change existing semantics, only adds a column.

**If the frozen-core exception is not granted:** The schema must use a `belief_tenant` association table instead of a `tenant_id` column on `belief`. This preserves structural enforcement at the cost of an extra join.

---

## AuthorityTarget Verdict

**APPROVE WITH CONDITIONS**

The five-tuple `(principal, resource, scope, action, consequence)` is the correct identity. The plan correctly excludes `risk_tier` (service-derived) and `expired` (deferred temporal authority).

**Residual issues:**
  1. **Snapshot authority (F-5.2-1, CRITICAL, PROPOSED):** The snapshot-authoritative model is the proposed semantic direction. Physical DDL enforcement mechanism remains implementation debt. DDL cannot proceed until the physical mechanism is chosen.
  2. **Scope semantics (OPEN #1):** Equality semantics, narrowing semantics, and wildcard semantics are OPEN. For DDL, the minimum requirement is that `scope` is stored as an opaque, exact-equality type (JSONB or TEXT) and the service enforces narrowing. The schema must not silently interpret scope.
  3. **Action representation (OPEN):** `action_namespace` + `action_name` is correct, but the exact column types and normalization rules are OPEN. The schema must store them verbatim and never lowercase/alias.

---

## Target Immutability Verdict

**NOT SAFE FOR DDL — physical DDL enforcement mechanism must be chosen first**

The semantic decision is PROPOSED and fixed in direction: a separate immutable `target_snapshot` is the sole execution-time authority. The physical DDL enforcement mechanism remains OPEN. This is honest but insufficient for DDL.

**My recommendation:** Option A (separate immutable snapshot table) is the most structurally honest:
- `authority_target` carries `state`, `snapshot_id`, `created_by`, `tenant_id`.
- `target_snapshot` carries the immutable five-tuple, justification set, policy version, approver, timestamps.
- `target_snapshot` is intended to be immutable; the DDL/security design must make unauthorized mutation impossible or explicitly reject it.
- Execution-time verification reads `target_snapshot`, never `authority_target`'s mutable columns.

**Why not Option C (I-7 discipline)?** Relying on the I-7 gate script to prevent tuple mutation is defense-in-depth, not structural impossibility. A buggy service that bypasses the kernel or a direct SQL UPDATE would still mutate the tuple. The schema should make the immutability impossible, not merely checked.

**Why not Option B (partial exclusion constraint)?** CockroachDB does not support triggers, and row-local CHECKs cannot compare old-vs-new values. This option is not expressible in CockroachDB.

**Why not Option D (computed hash)?** Fragile. A CHECK that verifies `tuple_hash` equality requires storing the original hash and recomputing it on every UPDATE — which is exactly the mutation we want to prevent.

---

## Approval Snapshot Verdict

**APPROVE WITH CONDITIONS**

The proposed snapshot contents are correct and sufficient:
- Target tuple (five fields)
- Justification set (belief IDs + claims + status at approval time)
- Policy version identity
- Approver principal

**What must NOT be added automatically:**
- `risk_tier` — service-derived, not stored.
- `claims` — belief claims are already in the justification set.
- `obligation state` — covered by the discharge ledger.
- `issuer` — already in the `principal` row referenced by `approver_principal`.

**What the snapshot must preserve:**
- The exact belief status at approval time (entered, promoted, retracted). This is covered by the justification set's `belief_status` field.
- The exact policy version identity (not just hash). This is covered by `policy_version_id` FK.
- The exact target tuple. This is covered by the snapshot's five-tuple columns.

**Open question:** Does the snapshot need to include the `belief.claim` text, or is `belief_id` sufficient? The justification set includes `belief_id`, and the approver can re-query the belief's claim at verification time. For MVP, `belief_id` is sufficient. If the belief's claim can change after approval (it cannot — belief is immutable once entered, only status changes), then the snapshot must store the claim text.

---

## Justification Verdict

**APPROVE WITH CONDITIONS**

The justification link is correctly modeled as an explicit `Belief ↔ AuthorityTarget` relation with composite tenant-aware FKs. The composite FK `(belief_id, belief_status) → belief(id, status)` with `ON UPDATE CASCADE` is the correct pattern (same shape as `gate`).

**Residual issues:**
1. **F-5.2-1 (CRITICAL, PROPOSED):** Snapshot-authoritative semantic decision is fixed as the proposed direction. Physical DDL enforcement mechanism remains implementation debt.
2. **F-5.2-2 (HIGH, BLOCKING):** Retract→re-promote lifecycle. The composite FK handles promote→retract correctly (link becomes invalid) but silently revives on re-promotion.
3. **Link state (OPEN #5):** Under the proposed snapshot-authoritative model, the live `justification` relation is proposal/audit history and does not control execution-time authority. Whether it needs its own lifecycle state is a separate DDL/operational question, not an authority-source choice.
4. **Cardinality (ATTACK 11):** The schema must prevent zero justifications from activating. This is a service-layer AND-semantics check ("all required beliefs are promoted"), not a DB CHECK. The DB cannot express "every row of my set satisfies P." This is correctly classified as service-enforcement.

---

## Belief Retraction Verdict

**APPROVE WITH CONDITIONS**

The existing `RetractCascade` is atomic and scenario-scoped (`kernel/kernel.go:127-149`). The composite FK `gate` with `ON UPDATE CASCADE` correctly propagates status changes to `action_intent` and detonates `live_requires_promoted`.

**Residual issue: F-5.2-2 (HIGH, BLOCKING for DDL).** The `(belief_id, belief_status)` composite FK in `justification` inherits the same `ON UPDATE CASCADE` behavior. This is correct for the simple lifecycle but creates the retract→re-promote ambiguity. The schema must decide whether re-promotion creates a new authorization-relevant state.

**What the DB enforces:**
- A retracted belief cannot have live intents (`live_requires_promoted` CHECK on `action_intent`).
- A retracted belief cannot be promoted with debt (`promoted_is_debt_free` CHECK).
- `ON UPDATE CASCADE` propagates status changes to dependent rows.

**What the DB does not enforce:**
- Whether an old justification link, revived by re-promotion, should still participate in an active target's authority. This is a semantic question, not a DB constraint.

---

## Obligation / Debt / Discharge Verdict

**APPROVE WITH CONDITIONS**

The transitional dual-truth model (`belief.debt[]` + `debt_discharge` ledger) is accepted under D9's atomicity rule. The plan correctly requires single-transaction `RetireDebt + Discharge`.

**Residual issues:**
1. **F-5.2-3 (HIGH, BLOCKING for DDL):** Discharge uniqueness scope. Per-belief uniqueness is insufficient for cross-belief replay prevention.
2. **F-5.2-8 (MEDIUM, BLOCKING for DDL):** Obligation identity: version-qualified keys vs stable keys + policy binding.
3. **Debt array + ledger coherence (ATTACK 8):** One CockroachDB transaction is sufficient during the transitional period. The kernel already uses `crdb.ExecuteTx` for all writes (I-7). The discharge + array-removal must be wrapped in the same transaction. This is a kernel-design decision, not a schema decision, but the schema must support it (the `debt_discharge` table must have a `belief_id` FK that matches the `array_remove` target).
4. **Duplicate discharge:** The schema's `UNIQUE` constraint prevents duplicate accepted discharges for the same `(belief_id, obligation)` under the chosen scope. Concurrent discharges are serialized by CockroachDB's serializable isolation. The loser observes the winner's committed state and becomes a no-op or retry.

---

## Policy Version Verdict

**APPROVE WITH CONDITIONS**

The `policy_version` object is correctly derived from F5. It includes version identity, content hash, `effective_at`, `tenant_id`, and `author_principal`. The plan correctly excludes `debt_config` as a separate table.

**Residual issue: F-5.2-4 (HIGH, BLOCKING for DDL).** The ordering mechanism is undefined. DDL cannot specify how `effective_at` monotonicity is enforced.

**Minimum DDL requirements:**
- `policy_version` is append-only: no UPDATE, no DELETE. This is a kernel/service discipline, not a DB constraint (CockroachDB does not support `ON UPDATE RESTRICT` or triggers). The schema must not claim DB enforcement for append-only behavior.
- The primary key must include `tenant_id` + `version_id` (or `effective_at` if that is the sequence). A hash alone is not a sequence.
- `author_principal → principal` is attribution only, not authorization (F-5.2-5).

---

## Policy Floor Verdict

**NOT READY FOR SCHEMA ENFORCEMENT — currently DOCUMENTED CONTRACT only**

The universal negatives (I-P1: no empty obligation sets, no self-satisfying obligations, no "any evidence qualifies"; I-P2: deterministic cannot rely on unverified `external_feed`) are correctly identified as DOCUMENTED CONTRACT. The candidate schema does not attempt to enforce them in the DB, which is correct.

**Why they cannot be DB constraints yet:**
- **I-P1 (no empty obligation sets):** Requires checking the obligation set size at belief creation. This is a service-layer check because obligation definitions live in policy, not in a DB table.
- **I-P1 (no self-satisfying obligations):** Requires checking that an obligation's `discharge_policy` does not reference its own discharger. This is semantic, not row-local.
- **I-P2 (deterministic ≠ unverified external_feed):** Requires checking `provenance_class` against obligation type. The `evidence` table has no FK to `debt_discharge` or obligation, so the DB cannot correlate them row-locally.

**The candidate floor correctly leaves these as service-enforced.** This is the right call. Adding CHECK constraints that pretend to enforce semantic policy rules would be theater.

**Residual:** The candidate `HIGH_IMPACT ⇒ ≥1 attested or quorum` (I-P3) remains OPEN. It is correctly not enforced in the schema.

---

## Warrant Verdict

**DEFERRED — no persistent warrant table in MVP**

The architecture correctly keeps Warrant as a portable reference, not a second authority source (D13, ADR Decision 1). The candidate schema does not include a `warrant` table, which is correct for MVP.

**What the schema must preserve for future warrant semantics:**
- The `authority_target` row's immutable snapshot provides the target identity.
- The `policy_version` provides the policy binding.
- A future warrant can reference `target_id` + `snapshot_id` + `issued_at` + `policy_version_id`.
- The schema must not add columns to `action_intent` that would make it a de facto warrant table (e.g., `warrant_json`).

**Residual:** Execution-effect verification (I-13) remains FUTURE/UNSCOPED. The schema must not claim `action_intent.state = 'executed'` proves world effect.

---

## Evidence / Provenance Verdict

**APPROVE — transitional model only**

The current `evidence` table is belief-owned with caller-supplied `content_sha256`. The candidate `debt_discharge` table references evidence via `instrument_ref`. This is safe for the transitional model.

**Residual issues:**
1. **Caller-supplied hash:** The DB cannot verify that `content_sha256` matches the actual payload. This is deferred (I-11).
2. **Per-belief dedup:** Current `evidence` dedup is per-belief. The `debt_discharge` schema does not prevent the same evidence row from being referenced by multiple discharges across beliefs, but this is a service-layer concern (one-citation-per-debt rule).
3. **Artifact/citation split (OPEN #13):** The schema must not add FK constraints that would prevent the future transition from belief-owned evidence to artifact-owned evidence with per-belief citations.

**DB enforcement that survives:**
- `evidence.content_sha256 NOT NULL` prevents empty hashes.
- `evidence.provenance_class CHECK (IN (...))` restricts the enum.

**DB enforcement that does not exist:**
- Server-side hash verification (I-11, deferred).
- Cross-belief replay protection (service-layer).

---

## Resource Identity Verdict

**APPROVE — opaque exact equality**

`resource_type` + `resource_id` as opaque, exact-equality identifiers is correct. Solvent does not interpret resource meaning; it compares by exact equality.

**Residual issues:**
1. **Same `resource_id` across tenants:** Prevented by composite tenant-aware FK on `authority_target` (if `resource` is stored on `authority_target` with `tenant_id`).
2. **Resource deletion/recreation:** Outside Solvent's scope. Solvent records the resource identifier at authorization time; if the resource is deleted and recreated with the same ID, the authorization still references the original identifier. This is correct.
3. **No universal resource ontology:** Correctly deferred.

---

## Scope Verdict

**OPEN — equality and narrowing semantics must be defined before DDL**

Scope remains partially OPEN as the plan acknowledges. The minimum requirements are:
- `scope` is stored as opaque, exact-equality data (JSONB or TEXT).
- The service enforces that scope narrows resource/consequence, never broadens.
- Empty/missing scope is not a wildcard. The schema must treat `NULL`/empty scope as "no scope constraint," not "authorize everything."

**What the schema must not do:**
- Silently choose an ontology (e.g., "scope is always JSON with environment/region keys").
- Add CHECK constraints that assume a specific scope vocabulary.

**Blocking for DDL?** NO, as long as the column is opaque and the service enforces narrowing.

---

## Action Verdict

**APPROVE — `action_namespace` + `action_name`**

The split from current `action TEXT` to `{namespace, name}` is correct. It prevents adapter-side normalization (lowercasing, aliasing) from causing semantic drift between MCP and REST.

**Residual issues:**
1. **Namespace ownership:** The schema does not define who may create namespaces. This is a service/policy concern.
2. **Versioning:** Action versioning is not needed for MVP. If an action's semantics change, it is a different `(namespace, name)` pair.
3. **Adapter translation:** The schema must not include a normalized `action_canonical` column. The adapter must present the exact tuple; Solvent verifies exact equality.

---

## Consequence Verdict

**APPROVE — `consequence_type` + `consequence_parameters`**

The split from opaque `action TEXT` to structured consequence is correct. Solvent compares consequence descriptors by exact equality; it does not interpret them.

**Residual issues:**
1. **Same bytes ≠ same meaning:** The schema stores bytes, not semantics. Two executors may interpret the same `consequence_parameters` JSON differently. This is executor-dependent and out of scope.
2. **Version change:** A changed `consequence_type` or `consequence_parameters` creates a different target requiring new authorization. This is correctly handled by the target's immutable tuple.
3. **Dangerous default:** The schema must not add DEFAULT values to consequence columns. A missing consequence is a schema error, not a default.

---

## Concurrency Verdict

**APPROVE WITH CONDITIONS — requires explicit transaction semantics**

The plan correctly identifies that CockroachDB's serializable isolation serializes conflicting writes, but it does not specify the semantic conflict for every race.

| Race | Serialized State | Transaction Boundary | Loser Observes | Stale Read → Authority? |
|---|---|---|---|---|
| Approve vs Approve (same target) | Only one `proposed → active` transition succeeds | `crdb.ExecuteTx` wrapping precondition + transition | Sees winner's committed `active` state | No — loser gets REVIEW, not stale ALLOW |
| Approve vs Attach | Attach creates `proposed` link; Approve reads links in same tx | Attach in its own tx; Approve reads committed state | Sees attach if before approve, or not if after | No — Approve reads committed state |
| Approve vs Retract | Retract changes belief status; Approve reads belief status | Both in `crdb.ExecuteTx`; Retract is cancel-then-retract | Sees retracted belief | No — Approve refuses if belief not promoted |
| Approve vs Policy Change | Policy version created mid-approval | Approve binds `policy_version_id` in its tx | Sees either old or new policy version | No — approval binds the version it read |
| Discharge vs Promote | Discharge removes debt; Promote checks `debt='{}'` | Single-tx `RetireDebt + Discharge` per D9; Promote in separate tx | Discharge sees pre-promote debt; Promote sees post-discharge debt | No — Promote checks committed state |
| Retract vs Verify | Retract cancels intents; Verify reads live intents | Retract is atomic; Verify reads committed state | Sees retracted belief, cancelled intents | No — Verify reads committed state |
| Revocation vs Approve | Revocation sets `revoked_at`; Approve checks `revoked_at IS NULL` | Both in `crdb.ExecuteTx`; Approve reads principal | Sees revoked principal | No — Approve refuses |
| Target mutation vs Approve | Target tuple mutated; Approve reads tuple | Mutation is refused by immutability mechanism | Sees original or mutated tuple depending on mechanism | Depends on immutability mechanism (F-5.2-1) |

**Residual:** The plan does not specify the `Approve vs Approve` retry condition. Under `crdb.ExecuteTx`, a serialization failure returns `40001`. The service must retry. The loser must get a REVIEW outcome (re-read state, re-evaluate), never a silent second authority. This is correctly stated in D7 but not yet implemented.

---

## Migration Verdict

**APPROVE WITH CONDITIONS — requires explicit fail-closed rules**

The migration from old `action_intent` to `AuthorityTarget` is the correct direction, but the plan does not specify the migration rules.

**Required migration rules:**
1. **Missing dimensions → NOT AUTHORIZED, never wildcard.** Old `action_intent` rows have `action TEXT` only. They lack `principal`, `resource`, `scope`, `consequence`. Under the new model, these dimensions are required. The migration must treat old rows as `NOT AUTHORIZED` and require re-authorization under the new model; it must not infer default values or introduce temporal semantics such as `expired`.
2. **Old action string interpretation:** `action="update_record"` from the old model must not be silently interpreted as `action_namespace=default, action_name=update_record` with inferred `resource=*` and `consequence=*`. The new model requires explicit dimensions.
3. **Mixed old/new semantics:** During migration, old and new intents must not coexist in the same authorization path. The service must either (a) refuse to authorize old-format intents, or (b) migrate them before any new authorization is evaluated.

**Residual:** The schema must not add `DEFAULT` values to `resource`, `scope`, or `consequence` that would turn missing data into wildcards. `DEFAULT NULL` with service-layer `NULL → NOT AUTHORIZED` is the correct pattern.

---

## Service TCB Verdict

**CLEAR — the plan correctly places the service inside the TCB**

The architecture correctly identifies the Solvent service as inside the MVP TCB (D2). The candidate schema does not weaken this.

**If the service is fully compromised, what DB invariants survive:**
- `promoted_is_debt_free` (`23514`): A compromised service cannot INSERT or UPDATE a belief to `status='promoted'` with non-empty debt, because the CHECK refuses the write. **Survives.**
- `gate` (`23503`): A compromised service cannot INSERT an `action_intent` referencing a non-promoted belief, because the composite FK refuses the write. **Survives.**
- `live_requires_promoted` (`23514`): A compromised service cannot UPDATE a belief to `retracted` while a live intent exists, because `ON UPDATE CASCADE` propagates the status and the CHECK detonates. **Survives.**
- `belief_id_status_key` UNIQUE: A compromised service cannot create duplicate `(id, status)` pairs. **Survives.**
- Principal type immutability (if implemented as CHECK): A compromised service cannot change `principal_type` from `agent` to `human`. **Survives if CHECK exists.**
- Tenant isolation (if composite FKs exist): A compromised service cannot create a cross-tenant FK violation. **Survives if composite FKs exist.**

**What disappears if the service is compromised:**
- All set-level AND checks (justification validity, policy floor, I-A4 approval separation).
- All authorization decisions (who may create targets, attach justifications, approve).
- All policy interpretation (what counts as `HIGH_IMPACT`, what discharge policy qualifies).
- All identity verification (whether the IdP assertion is genuine — the DB only persists what the service supplies).

**What remains outside the guarantee:**
- Executor behavior (I-13, FUTURE/UNSCOPED).
- Human approver comprehension (a deceived human is outside Solvent's structural guarantee).
- Evidence truth (Solvent guarantees provenance, not world truth).

**Verdict:** The TCB assumption is clean and honest. The schema correctly places impossibility in the DB and interpretation in the service.

---

## Executor / I-12 Boundary Verdict

**PROPOSED PROPERTY — source-spec label only, not earned invariant**

I-12 (`AuthorizationTargetMatch`) is a **proposed authorization-target matching property** from the source specification (`docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`). It is not an earned repository invariant. The current repository has no `authority_target` table and no execution boundary.

**What the schema must preserve:**
- The `authority_target` five-tuple is stored verbatim.
- At execution time, the executor presents the exact tuple.
- Solvent verifies `authorized tuple == presented tuple` by exact equality server-side.
- `unavailable` is a distinct, typed outcome from `DENY`.

**What the schema must not claim:**
- `action_intent.state = 'executed'` is proof of world effect. It is a caller-asserted string (`db/001_schema.sql:67`), not a proof.

**Execution-effect verification (I-13) remains FUTURE / UNSCOPED.** The schema must not include columns or tables that imply execution proof (e.g., `execution_receipt`, `actual_consequence`).

**DOCUMENTED CONTRACT preservation:** The schema must not turn the executor contract into a DB invariant. The contract is:
> A consequential action governed by Solvent must not have an unmediated execution path around the authorized executor.

This is a deployment/integration precondition, not a schema constraint.

---

## ADR-0001 Attestation Key Lifecycle Cross-Check

ADR-0001 (`docs/ADR/0001-authority-and-attestation.md`) deferred eight named questions under "Attestation key lifecycle." The candidate schema's `principal` object makes concrete progress on some but does not explicitly map them. This section closes that continuity gap.

| ADR-0001 Question | Current Status | What the Principal Model Resolves | What Remains Deferred | Source Citation |
|---|---|---|---|---|
| **Enrollment** | DEFERRED | `principal(issuer)` records which IdP/workload attestation service asserted the principal | No enrollment mechanism; bootstrap admin provisioning is PROPOSED but unimplemented | `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:146-173` |
| **Key binding** | DEFERRED | `principal_id` is stable; `issuer` binds the principal to the asserting service | No cryptographic key material stored; binding between principal and attestation key is outside Solvent | `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:297-309` |
| **Storage** | DEFERRED | No credential table in MVP | IdP/workload attestation service owns key material; Solvent persists only `principal_id, principal_type, issuer, tenant_id, revoked_at` | `docs/ARCHITECTURE/schema-readiness-decision.md:163-192` D1 |
| **Rotation** | DEFERRED | `principal_id` remains stable across credential/key rotation; Solvent does not create another principal row for credential rotation | No credential/key rotation mechanism; IdP/workload attestation system owns it | `docs/ARCHITECTURE/schema-readiness-decision.md:163-192` D1 |
| **Revocation propagation** | PARTIAL | **CURRENT:** existing repository has no production principal/revocation model. **PROPOSED:** `principal.revoked_at` should gate new approvals; `ON UPDATE CASCADE` propagates to dependent rows if implemented | Does not automatically invalidate previously issued attestations presented later (see `### F-5.2-10` finding) | `docs/ARCHITECTURE/schema-readiness-decision.md:186-192` |
| **Principal departure** | DEFERRED | `revoked_at` can be set on departure | No automated departure detection; human-admin process only | `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:164` |
| **Compromise** | DEFERRED | `revoked_at` can be set on compromise | No compromise detection, key rotation, or re-attestation mechanism | `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:288` |
| **Replay prevention** | DEFERRED | `debt_discharge` uniqueness prevents replay of same instrument against same obligation (scope TBD) | Does not prevent replay of same attestation against a different obligation by a different principal; attestation-level replay cache is future work | `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:654-663` |

**Key distinction:** The `principal` table resolves *identity persistence* and *attribution survival* across credential events. It does not resolve *key lifecycle management*, which remains the IdP's responsibility. Do not claim key management is solved.

---

## DocTrust Verdict

**WHAT THE SCHEMA MUST MAKE TESTABLE**

DocTrust will be the first independent reference integration. The candidate schema must make the following testable through public interfaces (MCP/REST, not the kernel):

| Test | Required Schema Support |
|---|---|
| Proposed target | `authority_target` INSERT with `state=proposed` |
| Justification | `justification` INSERT referencing target + belief |
| Approval | `authority_target` transitions `proposed → active` and references the required immutable `target_snapshot` |
| Retraction | `belief` UPDATE `status=retracted` + `ON UPDATE CASCADE` to `justification` and `action_intent` |
| Policy downgrade | `policy_version` INSERT with weaker floor; existing target's `can_authorize` re-evaluates to DENY |
| Tuple mismatch | `authority_target` five-tuple presented ≠ stored tuple → DENY |
| Principal revocation | `principal.revoked_at` set → new approvals refused |
| Discharge attribution | `debt_discharge.discharged_by → principal` |
| Tenant boundary | Cross-tenant `justification` INSERT refused by composite FK |

**WHAT DOCTRUST CANNOT PROVE:**
- Domain-agnostic generality (single document domain).
- Semantic relevance of justifications (a deceived human approver is outside Solvent's guarantee).
- Execution-effect verification (I-13, FUTURE/UNSCOPED).
- Multi-tenant isolation (MVP is single-tenant with fixed scenario IDs; tenant binding is new).
- That the C-1 model is production-ready (DocTrust is Reference Implementation #1, not proof of correctness).

---

## Logical Core / Physical Candidate Verdict

**CURRENT PHYSICAL CANDIDATE: SIX OBJECTS UNDER THE PROPOSED SNAPSHOT-AUTHORITATIVE MODEL**

The core authority model requires five objects derived from durable facts F1–F7. Under the proposed snapshot-authoritative design (F-5.2-1), a sixth `target_snapshot` object is required. If `belief_tenant` is required because the frozen core cannot be changed, the physical design may become seven. Final object count remains subject to DDL derivation.

| Object | Durable Fact | Prevents Unsafe State |
|---|---|---|
| `principal` (F1) | Stable actor identity + type + revocation | Unattributed or masqueraded authority |
| `authority_target` (F2) | Five-tuple authorization subject + activation state | Unscoped or mutable authority |
| `target_snapshot` (F6 + F-5.2-1) | Immutable execution-time authority copy | Mutated active authorization indistinguishable from new one |
| `justification` (F3) | Explicit Belief ↔ Target link | Wildcard permission via implicit promotion |
| `debt_discharge` (F4) | Attributable discharge history | Debt-free checkbox theater |
| `policy_version` (F5) | Append-only policy identity + ordering | Downgrade broadening without audit |

**Could any object be eliminated?**
- `principal` cannot be eliminated without losing attribution and I-A4 enforcement.
- `authority_target` cannot be eliminated without losing the five-tuple authority abstraction.
- `target_snapshot` cannot be eliminated under the proposed snapshot-authoritative model; it is the sole execution-time authority.
- `justification` cannot be eliminated without losing the explicit belief→target link (reverting to implicit wildcard).
- `debt_discharge` cannot be eliminated without losing the attributable history that makes `debt=[]` meaningful.
- `policy_version` cannot be eliminated without losing downgrade detection and approval binding.

**What about `belief_tenant` (from F-5.2-7)?** If the frozen `belief` table cannot be modified, a `belief_tenant` association table is required to make composite FKs structurally enforceable. This would be a seventh object.

**Verdict:** The core authority model requires five objects. Under the proposed snapshot-authoritative design (F-5.2-1), the current physical candidate is six objects. If `belief_tenant` is required, the physical design may become seven. Final object count remains subject to DDL derivation.

---

## Cross-Layer Enforcement Matrix

| Property | DB | Kernel | Service | Identity | Policy | Executor | Lean | Status |
|---|---|---|---|---|---|---|---|---|
| `promoted ⇒ debt empty` | **DB** `promoted_is_debt_free` `23514` | Kernel `Promote` in `crdb.ExecuteTx` | Service validates N1–N4 | — | Policy supplies obligation set | — | Lean `promote_preserves_validity` | CURRENT |
| `live intent ⇒ promoted` | **DB** `gate` `23503` + `live_requires_promoted` `23514` + `ON UPDATE CASCADE` | Kernel `RetractCascade` cancel-before-retract | — | — | — | — | Lean `live_intent_implies_promoted` | CURRENT |
| `justification cites promoted belief` | **DB** composite FK `(belief_id, status)` with `ON UPDATE CASCADE` (proposed) | — | Set-level AND check at Approve | — | — | — | — | PROPOSED |
| `target immutable` | **DB** no UPDATE path (mechanism TBD) | — | Service refuses mutation | — | — | — | — | PROPOSED |
| `approval snapshot immutable` | **DB** separate `target_snapshot`; physical immutability enforcement TBD | — | Service reads snapshot at verification | — | — | — | — | PROPOSED |
| `discharge uniqueness` | **DB** `UNIQUE` per chosen scope | Kernel single-tx `Discharge + array_remove` | — | — | — | — | — | PROPOSED |
| `policy append-only` | **DB** no UPDATE/DELETE (discipline, not constraint) | — | Service version verification | — | Policy owns floor | — | — | PROPOSED |
| `agent cannot approve HIGH_IMPACT` | — | — | Service checks `principal_type` from DB | IdP asserts type authenticity | Policy lists approver set | — | — | DOCUMENTED CONTRACT |
| `no empty/self-satisfying/any evidence` | — | — | Service policy validation | — | Policy floor (I-P1/I-P2) | — | — | DOCUMENTED CONTRACT |
| `authorized tuple == presented tuple` | — | — | Service server-side equality | — | — | Executor echoes verbatim | — | PROPOSED (I-12) |
| `world effect == authorized consequence` | — | — | — | — | — | Trusted executor / signed receipt | — | FUTURE/UNSCOPED (I-13) |

---

## Impossible-State Analysis

### CRITICAL / HIGH Properties

| Forbidden State | Can DB Prevent? | Can Transaction Prevent? | Service Requirement? | External Requirement? | Remaining Residual |
|---|---|---|---|---|---|
| Live intent on non-promoted belief | **Yes** — `gate` FK + `live_requires_promoted` CHECK (`23503`/`23514`) | Yes — `RetractCascade` cancel-before-retract in `crdb.ExecuteTx` | — | — | None |
| Promoted belief with non-empty debt | **Yes** — `promoted_is_debt_free` CHECK (`23514`) | — | N1–N4 promotion preconditions | — | Debt may be empty but unjustified (checkbox theater) |
| Active target with mutated tuple | **No** — mechanism TBD | — | Service refuses mutation | — | Depends on immutability mechanism choice (F-5.2-1) |
| Approval without authorized approver | **No** — no `principal` CHECK on `active` rows yet | — | Service checks `principal_type != agent` for HIGH_IMPACT | IdP asserts type authenticity | Compromised service can bypass |
| Justification linking to retracted belief | **Partial** — composite FK `ON UPDATE CASCADE` propagates retraction, making link invalid | — | Service checks belief status at attach time | — | Re-promotion silently revives link (F-5.2-2) |
| Cross-tenant justification | **Yes** — composite tenant-aware FKs | — | Service validates tenant auth | Identity binds tenant | Migration gap for existing beliefs (F-5.2-7) |
| Discharge without debt removal | **No** — schema cannot correlate `debt_discharge` INSERT with `belief.debt` array_remove | **Yes** — kernel wraps both in one `crdb.ExecuteTx` | Service validates single-tx atomicity | — | Kernel bug could skip one operation |
| Duplicate accepted discharge | **Yes** — `UNIQUE` per chosen scope | — | — | — | Scope choice determines cross-belief replay (F-5.2-3) |
| Policy downgrade broadening authority | **No** — schema cannot compare "weaker" vs "stronger" | — | Service compares `current_authority ≤ approval_time_authority` | — | Downgrade detection is service-enforced |
| Stale warrant after retraction | **No** — no persistent warrant table | — | Service re-verifies at execution time | — | Executor cache is external |
| Executor performs unauthorized consequence | **No** — schema cannot constrain executor behavior | — | Service verifies `authorized == presented` | Executor contract (DOCUMENTED CONTRACT) | Fully compromised executor (I-13 deferred) |

---

## Self-Attack Matrix

| Attack | Failure Point | Enforcement | Residual Risk | Status |
|---|---|---|---|---|
| Legitimate belief + attacker-created target + legitimate approver | **Approve** (I-A4: approver must be human/non-agent; approver sees immutable snapshot naming exact consequence) | Service + Identity | Deceived human approver is outside structural guarantee | CONDITIONALLY BLOCKED |
| Agent masquerades as workload | **Identity** (IdP asserts `principal_type`; DB persists it) | Identity + DB (`principal_type` CHECK) | Compromised IdP issues false assertion | CONDITIONALLY BLOCKED |
| Workload masquerades as human | **Identity** (IdP `amr=mfa` assertion for `principal_type=human`) | Identity | Compromised IdP or stolen credential | CONDITIONALLY BLOCKED |
| Revoked principal attempts approval | **DB** (`principal.revoked_at` CHECK on `active` rows) | DB + Service | Clock skew in revocation propagation | BLOCKED |
| Target mutation after approval | **Immutable snapshot** (mechanism TBD) | DB + Service | Mechanism not yet chosen | DEFERRED |
| Policy downgrade after approval | **Service** (`current_authority ≤ approval_time_authority`) | Service | Service bug or compromise | CONDITIONALLY BLOCKED |
| Belief retraction after approval | **DB** (`ON UPDATE CASCADE` + `live_requires_promoted`) | DB | Re-promotion revives old justifications (F-5.2-2) | CONDITIONALLY BLOCKED |
| Cross-tenant justification | **DB** (composite tenant-aware FKs) | DB | Migration gap for existing beliefs (F-5.2-7) | CONDITIONALLY BLOCKED |
| Discharge replay across beliefs | **DB** (uniqueness scope TBD) | DB + Service | Per-belief uniqueness allows cross-belief replay (F-5.2-3) | UNRESOLVED |
| Obligation identity confusion (v1 vs v2) | **Schema** (obligation identity representation TBD) | DB + Service | Version-qualified keys vs stable keys + policy binding (F-5.2-8) | UNRESOLVED |
| Executor performs unauthorized consequence | **Service** (I-12: `authorized == presented`) | Service + Executor contract | Fully compromised executor (I-13 deferred) | DEFERRED |
| Evidence poisoning → manufactured discharge | **Service** (provenance validation at ingress) | Service + Future I-11 | Caller-supplied `content_sha256`, no server verification | DEFERRED |
| Agent creates target + link + requests authorization + self-approves | **Approve** (I-A4: agent cannot approve HIGH_IMPACT; human approver required) | Service + Identity | Compromised human approver | CONDITIONALLY BLOCKED |
| Stale warrant after retraction | **Service** (re-verification at every HIGH_IMPACT execution) | Service | Executor in-memory cache (external to Solvent) | DEFERRED |
| Concurrent Approve on same target | **Transaction** (`crdb.ExecuteTx` serialization + `40001` retry) | Kernel + Service | Retry logic bug | BLOCKED |
| Concurrent Discharge + Promote | **Transaction** (single-tx `Discharge + array_remove`; Promote reads committed state) | Kernel | Race between discharge commit and promote read | BLOCKED |

---

## Schema Design Open Questions

Only unresolved questions that genuinely matter for DDL or production safety:

1. **Snapshot authority — semantic decision proposed (F-5.2-1, PROPOSED):** The snapshot-authoritative model is the proposed semantic direction: `target_snapshot` is the sole execution-time authority. Physical DDL enforcement mechanism (exact immutability constraints) remains implementation debt. This decision determines DDL structure and whether `justification` rows are immutable after `Approve`.
2. **Discharge uniqueness scope (F-5.2-3, BLOCKING):** Per-belief or global per-obligation? Decision determines the `UNIQUE` constraint definition.
3. **Obligation identity precision (F-5.2-8, BLOCKING):** Version-qualified keys or stable keys + policy binding? Decision determines the `debt_discharge` uniqueness expression.
4. **Policy version ordering (F-5.2-4, BLOCKING):** Monotonic version sequence or `effective_at`-only? Decision determines the `policy_version` primary key and ordering mechanism.
5. **Belief tenant migration (F-5.2-7, BLOCKING for production):** Additive `tenant_id` column on frozen `belief` table, or `belief_tenant` association table? Decision requires explicit frozen-core exception.
6. **Belief re-promotion semantics (F-5.2-2, HIGH):** Does re-promotion create a new authorization-relevant state? Decision determines whether justification FK binds to `(belief_id, status)` or `(belief_id, status_epoch)`.
7. **Scope equality semantics (OPEN #1, MEDIUM):** Exact equality, narrowing, wildcard? Decision determines `scope` column type and service enforcement.
8. **Justification link state (OPEN #5, MEDIUM):** Own `proposed/accepted` state, or acceptance purely in snapshot? Decision depends on F-5.2-1 resolution.

---

## DDL Gate

**NOT SAFE TO BEGIN DDL**

The candidate schema is **not safe to translate into DDL** in its current form. Seven independent blocking decisions plus two corollaries of Decision 1 remain:

1. **F-5.2-1 (CRITICAL):** 
   - **Semantic decision:** PROPOSED — snapshot is the sole execution-time authority.
   - **DDL debt:** OPEN — exact relational mechanism that makes snapshot immutability structurally enforceable.
   The semantic authority model is resolved enough to proceed with design, but DDL cannot be written until the physical enforcement mechanism is selected and shown to preserve that semantic model.
2. **F-5.2-2 (HIGH):** Belief retract→re-promote lifecycle is unhandled. DDL cannot specify the `(belief_id, belief_status)` composite FK without deciding whether re-promotion creates a new authorization-relevant state.
3. **F-5.2-3 (HIGH):** Discharge uniqueness scope is unhandled. DDL cannot specify the `UNIQUE` constraint without deciding per-belief vs global.
4. **F-5.2-4 (HIGH):** Policy version ordering mechanism is undefined. DDL cannot specify the primary key or ordering constraint.
5. **F-5.2-8 (MEDIUM):** Obligation identity precision is ambiguous. DDL cannot specify the `debt_discharge` uniqueness expression without choosing version-qualified keys or stable keys + policy binding.
6. **F-5.2-10 (HIGH, NEW):** Revocation-time attestation validity is underspecified. Before DDL, Solvent must define the validity relation between principal revocation and attestation issuance/use.
7. **F-5.2-7 (MEDIUM, blocking for production):** Belief tenant migration is unengineered. DDL cannot create composite FKs to `belief` without a `tenant_id` column on `belief`.

**Schema design is authorized.** The relational model is conceptually sound and derived from durable facts. But DDL requires concrete decisions on the seven items above.

---

## Exact Conditions Before DDL

Each condition includes: required decision, evidence required, exit test.

| # | Condition | Required Decision | Evidence Required | Exit Test |
|---|---|---|---|---|
| C-1 | Proposed semantic decision: snapshot-authoritative model (`target_snapshot` is sole execution-time authority). Physical DDL enforcement mechanism (exact immutability constraints) remains implementation debt. | Semantic decision proposed; physical mechanism TBD | Written justification for why snapshot-authoritative model prevents post-approval link addition and tuple mutation | DDL design chooses physical enforcement mechanism; semantic model is fixed |
| C-2 | Belief re-promotion semantics | Decide whether re-promotion creates new authorization-relevant state | Analysis of retract→re-promote lifecycle in `kernel/kernel.go` and `RetractCascade` | Schema specifies either `(belief_id, status_epoch)` FK or snapshot-embedded justification |
| C-3 | Discharge uniqueness scope | Choose per-belief or global-per-obligation | Replay resistance analysis: can one attestation discharge two beliefs? | `UNIQUE` constraint expression matches chosen scope |
| C-4 | Obligation identity precision | Choose version-qualified keys or stable keys + policy binding | Policy-change replay analysis: does v1 discharge satisfy v2 obligation? | `debt_discharge` uniqueness includes chosen identity fields |
| C-5 | Policy version ordering | Choose monotonic sequence or `effective_at`-only | Concurrent policy creation test under CockroachDB serializable isolation | `policy_version` primary key enforces total order per tenant |
| C-6 | Belief tenant migration | Choose additive `tenant_id` on `belief` or `belief_tenant` association | Migration feasibility analysis for existing `belief` rows | Composite FK `(tenant_id, belief_id)` can be created without data loss |
| C-7 | Revocation-time attestation validity | Define the validity relation between principal revocation and attestation issuance/use | Analysis of attestation lifecycle: can a pre-revocation attestation be presented after revocation? | Schema records the chosen validity relation; `debt_discharge` enforces it |

---

## Dogfood Evidence Ledger

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Five core logical authority objects are derived from durable facts F1–F7; current physical candidate is six including `target_snapshot` | `plan5.2.md:331-455` derives the core objects from F1–F7; `docs/ARCHITECTURE/schema-readiness-decision.md:598-642` F1–F7 table | High | Final physical object count remains subject to DDL derivation | PROPOSED |
| Composite tenant-aware FKs prevent cross-tenant authority | `plan5.2.md:97-111` Finding 2 resolution; `docs/ARCHITECTURE/schema-readiness-decision.md:200-230` D3 decision | High | Migration gap for existing `belief` rows (F-5.2-7) | DOCUMENTED CONTRACT |
| `ON DELETE CASCADE` destroys authority history | `plan5.2.md:115-136` Finding 3 resolution; `docs/ARCHITECTURE/schema-readiness-decision.md:252-265` append-only discipline | High | None | PROPOSED |
| `risk_tier` is service-derived, not stored | `plan5.2.md:188-208` Finding 6 resolution; `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:565-570` scope/risk distinction | High | None | DOCUMENTED CONTRACT |
| Discharge attribution (`discharged_by → principal`) is required | `plan5.2.md:211-229` Finding 7 resolution; `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:240-241` discharge attribution | High | Principal table must exist before discharge | DOCUMENTED CONTRACT |
| Policy version is more than a hash | `plan5.2.md:305-328` Finding 11 resolution; `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:331-364` policy versioning | High | Ordering mechanism TBD (F-5.2-4) | DOCUMENTED CONTRACT |
| `principal_type` is immutable | `plan5.2.md:335-348` principal object; `docs/ARCHITECTURE/schema-readiness-decision.md:163-192` D1 decision | High | Type transition representation (F-5.2-11) | DOCUMENTED CONTRACT |
| Current `gate` demonstrates a `(belief_id, status)` FK pattern for the simple lifecycle; future justification use is not yet validated for re-promotion | `db/001_schema.sql:74-75` existing `gate` FK pattern; `kernel/kernel.go:127-149` `RetractCascade` | High | Retract→re-promote ambiguity (F-5.2-2) | PROPOSED |
| Single-tx discharge + array removal is sufficient | `kernel/kernel.go:83-88` `RetireDebt` in `crdb.ExecuteTx`; `docs/ARCHITECTURE/schema-readiness-decision.md:396-432` D9 atomicity rule | High | Kernel must wrap both in same tx | DOCUMENTED CONTRACT |
| Target immutability mechanism is unresolved | `plan5.2.md:147-158` Finding 4 resolution; `plan5.2.md:588` cross-object properties table says "TBD" | High | Physical DDL enforcement mechanism remains implementation debt (F-5.2-1) | PROPOSED |
| I-12 is proposed property, not earned invariant | `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:557-572` I-12/I-13 distinction; no `authority_target` table exists | High | None | PROPOSED |
| I-13 (execution-effect verification) is FUTURE/UNSCOPED | `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:574-577` CURRENT/VERIFIED: `action_intent.state='executed'` is caller-asserted | High | None | FUTURE / UNSCOPED |
| Snapshot-authoritative model rejects live justification as an execution-time authority source | `plan5.2.md:391` + `plan5.2_review.md:41-81` identify the semantic hole; F-5.2-1 records the proposed resolution | High | Physical DDL enforcement mechanism remains implementation debt (F-5.2-1) | PROPOSED |
| ADR-0001 attestation key lifecycle cross-check | `docs/ADR/0001-authority-and-attestation.md` eight deferred questions; `principal` object resolves identity persistence but not key lifecycle | High | Rotation, enrollment, key binding, storage, compromise, replay prevention remain deferred | DOCUMENTED CONTRACT |
| Discharge uniqueness scope is OPEN | `plan5.2.md:416` schema-readiness OPEN #3; `plan5.2.md:412` candidate says per-belief | High | Resolution required (F-5.2-3) | OPEN |
| Obligation identity precision is OPEN | `plan5.2.md:472-480` Remaining Open Questions #2; `docs/ARCHITECTURE/schema-readiness-decision.md:386-390` OPEN #2 | High | Resolution required (F-5.2-8) | OPEN |
| Policy version ordering is OPEN | `plan5.2.md:427` says transactional but does not define mechanism; `plan5.2_review.md:200-228` external review attacks temporal semantics | High | Resolution required (F-5.2-4) | OPEN |
| Belief tenant migration is OPEN | `plan5.2.md:109` OPEN question; `docs/ARCHITECTURE/schema-readiness-decision.md:226-230` `scenario_id` is fixed map, not tenant | High | Resolution required (F-5.2-7) | OPEN |
| Service is inside MVP TCB | `docs/ARCHITECTURE/schema-readiness-decision.md:102-109` D2 decision; `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md:37-45` trusted core | High | None | DOCUMENTED CONTRACT |
| Agent cannot bootstrap authority system | `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md:158-173` bootstrap trust root; `docs/ARCHITECTURE/schema-readiness-decision.md:163-198` D1 principal decision | High | None | DOCUMENTED CONTRACT |

---

## Final Dogfood Test

**Does this schema design itself obey Solvent's methodology?**

1. **What evidence supports it?**
    - The five core authority objects are derived from durable facts F1–F7 (`schema-readiness-decision.md:598-642`), not from a preconceived table list. Under the proposed snapshot-authoritative model, the current physical candidate is six objects including `target_snapshot`.
   - Each finding resolution addresses a specific Plan 5.1 defect with repository-verifiable evidence.
   - The design explicitly removes tables (`target_approval`, `debt_config`, `expired` state, `risk_tier`) that were not earned by durable facts.

2. **What beliefs does it make?**
   - **Belief 1:** Composite tenant-aware FKs are sufficient to prevent cross-tenant authority relationships. **Evidence:** D3 decision in `schema-readiness-decision.md`. **Open debt:** Migration gap for existing `belief` rows.
   - **Belief 2:** The approval snapshot is the authoritative source at execution time. **Evidence:** D5/D6 decisions. **Open debt:** Physical DDL enforcement mechanism remains implementation debt (F-5.2-1).
   - **Belief 3:** Single-transaction `Discharge + array_remove` is sufficient for the transitional model. **Evidence:** D9 atomicity rule; existing `RetireDebt` uses `crdb.ExecuteTx`. **Open debt:** Kernel must wrap both in same tx.
   - **Belief 4:** `principal_type` immutability is sufficient for MVP. **Evidence:** D1 decision. **Open debt:** Type transition representation is deferred.

3. **What debt remains?**
   - **7 independent blocking decisions** (C-1 through C-7), plus **2 corollaries of Decision 1** (snapshot-authoritative semantics and separate immutable snapshot representation).
   - **10 open questions** that must be resolved during DDL design.
   - **2 deferred concepts** (credential table, delegation as first-class relation).
   - **1 future concept** (execution-effect verification, I-13).

4. **Who has authority to promote it?**
   - The adversarial schema review (this document) is the required gate.
   - After this review, the corrected relational semantics are promoted to DDL design.
   - DDL is not authorized by this review; it requires a separate DDL authorization after the seven conditions are resolved.

5. **What would falsify it?**
   - A semantic contradiction that makes an active target's justification set mutable after approval.
   - A discharge uniqueness scope that allows cross-belief replay without service-layer detection.
   - A policy version ordering mechanism that cannot enforce total order under CockroachDB serializable isolation.
   - A migration path that turns missing dimensions into wildcards.

6. **What evidence would cause reassessment?**
   - A new attack that shows the core authority model cannot represent a required durable fact.
   - A CockroachDB behavior that invalidates the assumed transaction semantics (e.g., `ON UPDATE CASCADE` does not re-evaluate CHECK in a future version).
   - A DocTrust integration requirement that forces an additional object beyond the current six-object physical candidate.

---

## Validation Checklist

- [x] Re-read the entire review.
- [x] Re-check every CURRENT claim against repository truth (`db/001_schema.sql`, `kernel/kernel.go`, `internal/view/view.go`, `cmd/solvent-mcp/tools.go`).
- [x] Verify that the candidate schema itself was not modified. (`plan5.2.md` unchanged.)
- [x] Verify that no DDL was created. (No `CREATE TABLE` in this review.)
- [x] Verify no SQL changed. (No `.sql` files modified.)
- [x] Verify no Go changed. (No `.go` files modified.)
- [x] Verify no Lean changed. (No `.lean` files modified.)
- [x] Verify no MCP changed. (No `cmd/solvent-mcp/` files modified.)
- [x] Verify no REST changed. (No REST files exist or were modified.)
- [x] Verify no ADR changed. (`docs/ADR/0001-authority-and-attestation.md` unchanged.)
- [x] Run `git status --short` — see below.
- [x] Report exactly what changed.
- [x] Confirm only `.kilo/plans/SCHEMA_DESIGN_ADVERSARIAL_REVIEW.md` was created/modified.

### Git Status

```
$ git status --short
M .kilo/plans/SCHEMA_DESIGN_ADVERSARIAL_REVIEW.md
```

Only the review file is modified. No code, SQL, Lean, MCP, REST, ADR, or architecture specification was modified.

---

## Final Verdict

> **Can the candidate relational model be translated into DDL without creating an authority escape hatch, a second source of truth, a hidden wildcard, an identity ambiguity, a stale-authorization path, or a cross-tenant relationship?**

**Not yet.** The model is conceptually sound and derived from durable facts, but it contains seven unresolved decisions that would produce unsafe DDL if translated verbatim. The most critical are:

1. **Snapshot authority — semantic decision proposed (F-5.2-1)** — this single decision establishes that `target_snapshot` is the sole execution-time authority. Its two corollaries are: snapshot is authoritative at execution time; snapshot is a separate immutable representation. The semantic model is resolved, but physical DDL enforcement remains implementation debt. Without the physical DDL enforcement mechanism, DDL cannot specify the exact constraints that make `target_snapshot` immutable or whether post-approval link additions change authority.
2. **Belief re-promotion lifecycle** — without resolving whether re-promotion creates a new authorization-relevant state, the `(belief_id, belief_status)` composite FK silently revives invalid justifications.
3. **Discharge uniqueness scope** — without resolving per-belief vs global, the schema may allow cross-belief instrument replay.
4. **Obligation identity precision** — without choosing version-qualified keys or stable keys + policy binding, the schema cannot define whether a v1 discharge satisfies a v2 obligation.
5. **Policy version ordering** — without a defined sequence, concurrent policy creation and downgrade detection are structurally ambiguous.
6. **Revocation-time attestation validity** — without defining the validity relation between principal revocation and attestation issuance/use, the schema cannot enforce whether a pre-revocation attestation remains valid after its issuer is revoked.
7. **Belief tenant migration** — without a migration path, composite FKs to `belief` cannot be created.

**The seven conditions in `## Exact Conditions Before DDL` are the smallest set of decisions that must be made before DDL can proceed.** None of them require architectural redesign. All are schema-design determinations. Once resolved, the candidate model is ready for DDL.

**One overarching principle:** The schema is not secure because it contains a column for every security concept. It is secure when the relationships between those columns make the forbidden states impossible. The candidate model has the right durable facts. It must now resolve the relationships.
