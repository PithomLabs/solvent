## Goal

Produce the repaired production schema design as a **new timestamped file** — `docs/ARCHITECTURE/schema-design-20260901T0750.md` — per your standing rule (original `schema-design.md` and review artifacts untouched). Status stays `PROPOSED — NOT IMPLEMENTED`. No DDL, no commit, no sync. Authoritative attack report: Revision 2 review (`SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW-20260901T0726.md`, 9 blockers / 7 conditions).

## Corrections from your review pass (all incorporated)

1. **C-1 renamed and rescoped** to **"C-1 — Authority lifecycle, activation binding, and snapshot immutability"** throughout the repaired document, so the blocking mapping SDAR-1,2,3,8,9 → C-1 is semantically defensible (they are all parts of the authority-lifecycle/activation model). Seven-condition count unchanged.
2. **`target_activation` invariants made explicit**: the central authority boundary is the composite FK `(tenant_id, target_id, snapshot_id) → target_snapshot(tenant_id, target_id, snapshot_id)` with the `UNIQUE(tenant_id, target_id, snapshot_id)` target declared on the snapshot side — proving the snapshot belongs to the same target and tenant, not left implied. Plus the explicit statement: **"An activation row is itself an authority-bearing immutable fact. It is not merely a cache or projection."** — so no implementation treats `authority_target.state` as authoritative again.
3. **`authority_target.state` mirror eliminated — read-time derivation only**: no stored lifecycle mirror and no `revoked_at` mirror on `authority_target` (nothing mutable to drift into a second source of truth). Lifecycle states are defined purely by facts, derived at read time:
   - `PROPOSED` = target exists ∧ no `target_activation` row exists
   - `ACTIVE` = `target_activation` row exists ∧ no `target_revocation` row exists
   - `REVOKED` = `target_revocation` row exists
   Documented as: authorization logic MUST consult only the facts; the derived states are query-level projections. `authority_target` therefore carries only proposal/pin columns — zero lifecycle columns.
4. **C-7 two distinct predicates, never collapsed**: (a) *attestation issuer validity* — `issued_at < revoked_at` of the attestation's **issuing principal** (resolved from the attestation; IdP-trusted `issued_at`); (b) *discharger authority* — `discharged_by.revoked_at IS NULL` at acceptance. The design states explicitly that a pre-revocation attestation remains historically valid even though the same principal may no longer perform a new discharge, and that an implementation must not merge these into one "principal is valid" check.
5. **Pin hash semantics hardened**: the pinned hash **represents the exact candidate presented for approval — it is not merely an integrity checksum**; and `Approve` must recompute-and-compare the hash **inside the same transaction that creates the snapshot and the activation** (no verify-then-write separation that would reopen a TOCTOU boundary).
6. **`belief_promotion` occurrence rule explicit**: `promotion_epoch` alone ≠ promotion occurrence; **a `belief_promotion` row = a promotion occurrence**. Epoch forgery dies at the FK; the side table represents actual committed promotions, not a counter.
7. **DDL Readiness not predeclared**: the plan does not decide the outcome of the validation it instructs. The repaired document will evaluate DDL readiness after the repair against the Repair-24 checklist and render whatever verdict the completed design actually earns.

## Repaired relational model (10 objects)

1. **`principal`** — PK `principal_id`; `UNIQUE(tenant_id, principal_id)`; type domain CHECK; `revoked_at` write-once; rotation never creates a second identity.
2. **`authority_target`** = proposal/lifecycle only (SDAR-2): full proposal tuple (NOT NULL + non-empty CHECKs), `created_by/at`, `requested_by/at`, `pinned_request_hash`; **no `snapshot_id`, no `state`, no lifecycle mirror columns** (correction 3).
3. **`target_snapshot`** = immutable authority content (SDAR-3): PK; `UNIQUE(tenant_id, target_id, snapshot_id)` (activation FK target — correction 2); four tenant-composite FKs (target, principal, approver, policy); `justification_set` blob; no `approver_principal_type` (SDAR-17).
4. **`target_activation`** = append-only authority-bearing immutable fact (SDAR-1/9): `UNIQUE(tenant_id, target_id)` over entire history ("ever activated", no state predicate); the 3-column snapshot FK; insert-only, no DELETE grant → resurrection structurally impossible.
5. **`target_revocation`** = append-only revocation fact, PK `(tenant_id, target_id)`; authority = activation ∧ ¬revocation (read-time derivation).
6. **`justification`** (SDAR-15): PK + `UNIQUE(tenant_id, target_id, belief_id, promotion_epoch)`; FKs to target, `belief_tenant`, `belief_promotion`.
7. **`belief_promotion`** (locked C-2, additive-column choice recorded as superseded): `UNIQUE(belief_id, promotion_epoch)` + tenant-composite UNIQUE (justification FK target) + FKs to `belief_tenant` and `belief(id)`; epoch 1 = first committed promotion; append-only; evidence-gated backfill, no fabricated history.
8. **`debt_discharge`** (SDAR-5): no status column — immutable accepted facts; revocation FUTURE/UNSCOPED (append-only counter-entry when real); unconditional `UNIQUE(tenant_id, obligation_key, policy_version_id, instrument_ref)`; `instrument_kind` + canonicalization (SDAR-16); `issued_at` IdP-trusted; single-tx discharge + `array_remove` (D9).
9. **`policy_version`** (SDAR-14): PK `(tenant_id, policy_version_id)`; `UNIQUE(tenant_id, revision)` named as the ordering enforcement; transactional max+1, first revision 1, CHECK ≥ 1; `policy_hash` = SHA-256 of exact stored bytes; `effective_at` metadata.
10. **`belief_tenant`** (SDAR-10): FK to `belief(id)`; `UNIQUE(belief_id)` 1:1; insert-only immutable; provisioning-only insertion; missing mapping fails closed.

**Approval pinning (SDAR-8)** per correction 5. **C-4 (SDAR-6)**: honest labeling — ledger binding authoritative; `debt[]` transitional and NOT structurally protected against policy redefinition; labeled in §3/§7/§19, never in the DB column. **Migration (SDAR-18)**: fail-closed; evidence-gated backfill; `solvent_authorize_action` transition-window rule (old path grants no new production authority).

## Document structure

23 sections mirroring the original design, with: C-1..C-7 implemented (C-1 renamed per correction 1); full relationship graph (exact columns/keys/cardinality per edge, activation FK per correction 2); read-time lifecycle derivation section (correction 3); rewritten 24-row impossible-state matrix (real enforcement layers only); concurrency matrix with concrete read/write/serialization-point/40001-retry/final-state rows; 16 DocTrust flows with deterministic outcomes; formal mapping (future theorems only); security-claims table; tradeoffs; second-source audit (including the removed `snapshot_id` and the eliminated state mirror); dogfood; DDL Readiness evaluated after the repair, not predeclared (correction 7); and the final **Repair Coverage** cross-tab: SDAR-1..21 each exactly once, blockers mapped to the rescoped C-1..C-7 (1,2,3,8,9→C-1; 4→C-2; 5→C-3; 6→C-4; 7→C-6), no duplicated blocker count.

## Execution steps

1. Write `docs/ARCHITECTURE/schema-design-20260901T0750.md` (full content prepared; one artifact).
2. Run the validation checklist + Repair Coverage cross-tab checks.
3. `git status --short`; report exact file changed, major repairs, remaining OPEN/DEFERRED, the earned DDL verdict. No commit, no sync.