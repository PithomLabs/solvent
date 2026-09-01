# Solvent Schema Design — Adversarial Implementation-Gate Review

## Status

**REVIEW ARTIFACT — NOT IMPLEMENTED**
**Revision 2 — supersedes `SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW.md` for handoff to the repair task** (Revision 1 content is preserved; this revision adds the BLOCKER-CROSS-TAB, the TAXONOMY/ID CHECK, the anti-anchoring disposition of the preidentified attack leads, the locked C-2 direction (`belief_promotion` side table), and the C-4 / C-7 nuance framings. Findings, severities, verdicts, and the blocker set are unchanged except where explicitly noted: Required Conditions were re-normalized from 8 items to 7 to remove a double-counted blocker reference.)

- **Artifact under review:** `docs/ARCHITECTURE/schema-design.md` (1,181 lines, "PROPOSED — NOT IMPLEMENTED")
- **Reviewer:** hostile implementation-gate review (independent, adversarial)
- **Date:** 2026-09-01
- **Question under test:** Can a competent engineer translate this design into DDL and still preserve Solvent's authority semantics without accidentally creating an authority escape hatch, stale authorization path, replay path, identity ambiguity, tenant escape, second source of truth, or hidden wildcard?
- **No DDL, no code, no schema changes, no commit, no sync.** The only artifact produced by this review is this file.

### Posture

Assume a hostile caller. Assume a buggy service. Assume concurrency. Assume stale state. Assume malformed inputs. Assume an attacker who understands the relational model better than the implementer. Where the design says "the service checks it," this review asks whether a DB relationship could have made the state impossible — and says so when the answer is yes.

### Invariant-ID discipline

The only repository-earned invariants are the SQL constraints `promoted_is_debt_free`, `gate`, `live_requires_promoted`, `belief_id_status_key` (`db/001_schema.sql`), plus I-7 (`crdb.ExecuteTx` discipline) and I-8 (`RetractCascade` cancel-before-retract ordering). `I-12` remains a PROPOSED source-spec label; `I-13` remains FUTURE / UNSCOPED; Executor Mediation remains a DOCUMENTED CONTRACT. No new `I-*` identifiers are created or consumed by this review. F1–F7, C-1–C-7, D1–D16, N1–N4, I-A*, I-P* remain document labels, not earned invariants.

---

## Review Scope

Everything in `docs/ARCHITECTURE/schema-design.md` was attacked:

- the seven claimed resolutions C-1 through C-7 (§3);
- the durable-fact derivation table (§4);
- all seven proposed objects — `principal`, `authority_target`, `target_snapshot`, `justification`, `debt_discharge`, `policy_version`, `belief_tenant` (§5), their keys, FKs, CHECKs, nullability, lifecycle, and transaction boundaries;
- the relationship map and interrogation matrix (§6);
- the impossible-states table (§7);
- snapshot authority, retraction-after-approval semantics (§8);
- promotion-epoch semantics (§9);
- debt/discharge and replay (§10);
- policy/principal/tenant models (§11);
- scope/resource/action/consequence representations (§12);
- warrant/decision absence (§13);
- evidence transitional model (§14);
- the concurrency matrix (§15);
- migration fail-closed rules (§16);
- DocTrust flows (§17);
- formal mapping (§18);
- security claims table (§19);
- tradeoffs (§20);
- DDL-readiness self-verdict (§21).

Attack groups 1–28 from the review mandate were each worked; the material ones produced the findings below. Groups that produced no finding are attested in the per-object verdicts (Resource/Action/Consequence identity, Warrant, Evidence provenance boundary, Formal model).

### Anti-anchoring rule (applied)

**The preidentified attack leads were hypotheses, not findings.** Each lead was independently confirmed, weakened, or rejected against the actual schema-design text; no finding was forced merely because it appeared in the lead list, and genuinely new findings outside the leads were retained. Disposition of the nine leads:

| Lead | Disposition |
|---|---|
| Mutable `snapshot_id` pointer on an active target | **Confirmed** — SDAR-1 (CRITICAL) |
| Proposal state has nowhere to store its candidate tuple | **Confirmed** — SDAR-2 (CRITICAL) |
| Justification→epoch binding "conceptual," no FK target, frozen-core conflict | **Confirmed, direction updated** — SDAR-4 (HIGH); see the C-2 locked-direction note below |
| Discharge `status='retracted'` UPDATE path reopening replay | **Confirmed** — SDAR-5 (HIGH) |
| Transitional dual truth defanging C-4 | **Confirmed, reframed into two questions** — SDAR-6 (HIGH); see C-4 nuance below |
| Missing/undeclared FKs and composite key scaffolding | **Confirmed, broader than led** — SDAR-3 (CRITICAL) and SDAR-7 (HIGH) |
| Retry changes the approved justification set (Approve × Attach) | **Confirmed** — SDAR-8 (HIGH) |
| Second-source items (`approver_principal_type`, blob vs rows, UNIQUE vs PK wording) | **Partially confirmed** — SDAR-17 (MEDIUM); the blob and wording items folded into the audit and SDAR-14 |
| Smaller items (requester identity, empty-value CHECKs, C-7 boundary, allocation, backfill, DocTrust flow 7) | **Confirmed** — SDAR-10..13, 14, 18, 19 |

Leads that were **rejected** (no finding warranted, and none was forced): warrant absence as a replay surface (§13 holds); action/consequence identity handling (§12 preserves exact values); migration fail-closed direction (§16 is correct); evidence provenance overclaim (§14 is honest); formal-model borrowing (§18 is clean).

### C-2 locked direction note

The currently locked repair direction for C-2 is the **`belief_promotion` side table** (`promotion_id, belief_id, promotion_epoch, promoted_at` with `UNIQUE(belief_id, promotion_epoch)` and FK to `belief`), superseding the earlier additive `belief.promotion_epoch`-column choice. This review verifies what the artifact under review actually implemented — which is **neither** direction (the binding is "conceptual"). The additive-column route is treated as admissible only with explicit approved evidence of a frozen-core exception; absent that evidence, an additive column would itself be a finding. SDAR-4's required resolution leads with the side table.

### What was NOT attacked

Nothing was improved, fixed, or rewritten in the artifact under review. No fix SQL, no design edits, no DDL.

---

## Repository Truth

Verified against the actual checkout. The design's CURRENT claims were checked and are accurate in substance:

- `db/001_schema.sql` — frozen core `belief`, `belief_edge`, `evidence`, `action_intent`. Constraints verified verbatim: `promoted_is_debt_free` (`status <> 'promoted' OR (coalesce(array_length(debt,1),0) = 0 AND NOT final_truth)`), `belief_id_status_key UNIQUE (id, status)`, `live_requires_promoted`, `gate FOREIGN KEY (belief_id, belief_status) REFERENCES belief (id, status) ON UPDATE CASCADE`. `belief.debt TEXT[]` default is the six-item vocabulary (re-stated in `db/004_debt_vocabulary.sql`).
- `action_intent.action TEXT NOT NULL` — free-form string, no namespace/resource/scope/consequence/principal/tenant column anywhere. No principal, issuer, tenant, or policy concept exists in the current schema.
- `kernel/` — eight verbs (`EnterBelief`, `AddEvidence`, `RetireDebt`, `Promote`, `IntentOnPromoted`, `RetractCascade`, `AuditLiveOnNonPromoted`, `EnsureBelief`), all mutations under `crdb.ExecuteTx`; `RetireDebt` is idempotent `array_remove`; classification by SQLSTATE only (23503/23514; 40001 reaches `crdb.ExecuteTx` unmasked).
- `internal/belief/belief.go` — contradiction path logs and discards (no retraction); discharge dedup `evidenceExists` is per-belief only — same artifact can discharge the same obligation across beliefs today (confirmed the replay motivation for C-3).
- `cmd/solvent-mcp/` — seven tools, hardcoded `track1`/`track2` scenario UUIDs; no principal identity anywhere; cross-scenario guard is application-side.
- `formal/lean/` — three ledger invariants mirroring the SQL constraints; eleven preservation theorems; zero `sorry`. Nothing formalizes targets, snapshots, epochs, tenants, policy, or discharge.
- `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md` ("PROPOSED — NOT IMPLEMENTED") — establishes the enforcement taxonomy used here: `impossible → Database/transaction; transition → Kernel/service transaction; interpretation → Service; identity authenticity → Identity provider; enterprise policy → Policy layer + service; actual world effect → Executor`. TCB = CockroachDB + kernel + Solvent service; a fully compromised service is kernel compromise and no schema claim may suggest otherwise.
- `docs/REVIEWS/SCHEMA_DESIGN_ADVERSARIAL_REVIEW_FINAL.md` — C-1..C-7 exit tests used as the acceptance bar for §3 of the design. (`.kilo/plans/SCHEMA_DESIGN_ADVERSARIAL_REVIEW.md` is the editorial draft of the same review; findings, severities, and conditions are identical.)

Repository truth wins. No object in the design's §5 exists in the repository today; all seven are PROPOSED.

---

## Executive Verdict

## REJECT / REDESIGN

Not "REJECT" of the architecture — the semantic corpus (belief ≠ authority, promotion ≠ permission, snapshot-authoritative execution, epoch discriminator, instrument-aware replay protection, policy-bound obligation identity, monotonic revision, `belief_tenant` association, issuance-bounded attestation validity) survives adversarial pressure and is directionally correct. The rejection is of the **relational representation as written**, on three grounds that a competent engineer could not translate into DDL without inventing security semantics the design does not supply:

1. **The `proposed` lifecycle state is unrepresentable** — the candidate authority tuple has no home between CreateTarget and Approve (SDAR-2). The four-operation model that the whole authority-creation spec rests on cannot be expressed by these objects.
2. **The approval pointer is mutable** — the object that carries "active authority" contains an updatable `snapshot_id`, so active authority can be silently repointed, violating the design's own locked principle 10 and defeating the privilege mechanism it deploys (SDAR-1, SDAR-9).
3. **The execution-time authority object declares no keys and no foreign keys** — `target_snapshot` is relationally unanchored; every tenant/principal/policy/snapshot relationship the interrogation matrix (§6) asserts as "composite tenant FK" is absent from the object definitions that would have to carry it (SDAR-3, SDAR-7).

Additionally, two of the seven claimed decision resolutions fail their own exit tests from the preceding review (C-2: the epoch binding is "conceptual," with no FK target and no committed referent, SDAR-4; C-4: the transitional model never consumes the policy binding the resolution promises, SDAR-6), and the discharge revocation sketch reopens the exact replay path C-3 was resolved to close (SDAR-5).

**The DDL gate verdict is NOT SAFE TO BEGIN DDL.** The unique blocking set is **nine findings** (SDAR-1 through SDAR-9), normalized into **seven Required Conditions** (see BLOCKER-CROSS-TAB in Validation). The redesign required is targeted, not architectural: fix the proposal representation, anchor and pin the snapshot, give the epoch a real referent via the `belief_promotion` side table, close the discharge UPDATE path, and add the composite key scaffolding.

---

## Critical Findings

### SDAR-1 — CRITICAL — Active authority can be silently repointed through the mutable `snapshot_id` pointer

- **Claim attacked:** §5.2 + §7 ("active target + mutable authority tuple … snapshot has no app UPDATE/DELETE") + §19 ("Snapshot cannot be mutated by application — DB privilege boundary") + locked principle 10 ("Active authority must not silently mutate").
- **Evidence:** `authority_target` fields are `target_id, tenant_id, state, snapshot_id (NULL until active), created_by, created_at, revoked_at`. The row must be mutable by the application role: the `proposed → active` and `→ revoked` transitions are UPDATEs to `state` (and `snapshot_id`, `revoked_at`). No constraint, trigger (CockroachDB has none), or privilege boundary makes `snapshot_id` write-once. The design's immutability mechanism (§3 C-1, §5.3) REVOKEs UPDATE/DELETE on `target_snapshot` — it defends the snapshot's *content*, not the *pointer* to it.
- **Attack:**
  ```sql
  -- any code path holding legitimate UPDATE on authority_target,
  -- e.g. the revoke handler with a wrong WHERE clause, or a confused retry:
  UPDATE authority_target
     SET snapshot_id = $prestaged_snapshot   -- earlier insert, different tuple
   WHERE tenant_id = $t AND target_id = $tgt; -- state not even checked
  ```
  The active target now executes under a different five-tuple, different justification set, different policy binding, different approver — with no approval event ever having occurred for the new snapshot. Alternatively, an *additional* snapshot row for the same target (nothing in §5.3 declares `UNIQUE(target_id)`) is pre-staged and swapped in. Execution authorization reads `state + snapshot` (§8) — both attacker-controlled after the swap.
- **Exact failure boundary:** the swap succeeds whenever any transaction can UPDATE `authority_target` — which the lifecycle itself requires. It fails only if the service never emits the wrong UPDATE, which is precisely the "application won't do that" answer the mandate forbids crediting.
- **Why the schema fails:** the design's own impossible-states row ("active target + mutable authority tuple") is satisfied only in the narrow sense that the tuple columns moved into the snapshot. The *reference* to the authority representation remained mutable, and the reference is as authority-bearing as the content. The privilege mechanism is aimed at the wrong object.
- **Required resolution:** make the (target → snapshot) binding structurally append/transition-only. Candidate mechanisms the redesign must choose between and specify concretely: (a) fold `state` into the snapshot itself (activation = inserting an activation record or setting a snapshot-local column, making `authority_target` insert-only); (b) a separate `target_activation(target_id, snapshot_id, activated_at, revoked_at)` transition table where activation rows are insert-only and `authority_target` is a derived/projection object; (c) pin the pointer with an INSERT-only activation row plus privilege separation on `authority_target` itself. Whichever is chosen, the design must name the exact object that is immutable and the exact object that is mutable, and no authority-bearing mutable column may remain on any row the application role can UPDATE.
- **DDL-blocking? YES**
- **Exit test:** no UPDATE issued under the application role can change which snapshot an active target resolves to, and no second snapshot can become resolvable for an active target without a new approval transaction; both demonstrated by attempting the direct SQL above against the DDL.

### SDAR-2 — CRITICAL — The `proposed` target cannot represent its candidate tuple; CreateTarget is unimplementable as specified

- **Claim attacked:** §5.2 ("No execution-time authority tuple is stored in mutable columns on this row. The tuple lives in `target_snapshot`"), the lifecycle `proposed → active → revoked`, and locked principles 7–8 (CreateTarget/AttachJustification/RequestAuthorization/Approve are distinct operations; creation and justification do not grant authority).
- **Evidence:** `authority_target` carries `target_id, tenant_id, state, snapshot_id, created_by, created_at, revoked_at` — no tuple fields, no proposal fields. `target_snapshot` requires `approver_principal_id NOT NULL`, `approved_at NOT NULL`, `justification_set NOT NULL` ("approved justification records"), `policy_version_id NOT NULL` as the *approval* binding. A snapshot therefore cannot be created at proposal time without fabricating approval-time facts.
- **Attack:** attempt to implement `CreateTarget(principal, resource, scope, action, consequence)` per the authority-creation spec. There is nowhere to put the tuple: `authority_target` forbids it by prose; `target_snapshot` requires approver and approval time that do not exist yet. Either the tuple is held in service memory between Create and Approve (unauditable, unrecoverable, and invisible to AttachJustification, which the C-1 spec requires to check "obligation_key in the belief's current obligation set" against a *specific candidate target*), or the implementer invents an ad-hoc proposal store — i.e., invents security semantics this design was required to supply.
- **Exact failure boundary:** DDL translation fails at the first operation of the lifecycle. Every downstream operation (Attach, Request, Approve) references a proposal whose content cannot be represented.
- **Why the schema fails:** the design resolved C-1 by moving the tuple out of `authority_target` and into an immutable snapshot, but never asked where the *mutable candidate* lives before approval. The preceding review's Corollary A said `authority_target` "carries lifecycle/reference fields such as `state` and `snapshot_id`" — it did not license deleting the proposal representation.
- **Required resolution:** choose and specify one: (a) mutable proposal-tuple columns on `authority_target` that are frozen by copy into the snapshot at approval and thereafter never read as authority (with an explicit statement that the copy, not the columns, is authoritative — plus a mechanism, e.g. the SDAR-1 fix, ensuring the columns cannot diverge in *meaning* after activation); or (b) a mutable `target_proposal` object approval-transmutes into the immutable snapshot. The design must state which fields are mutable before approval, prove the approval transaction copies them verbatim, and show that post-approval mutation of the proposal columns cannot change execution authority.
- **DDL-blocking? YES**
- **Exit test:** a DocTrust-style flow can CreateTarget(tuple) → Attach → Request → Approve with every intermediate state durably represented in the schema, and after approval, mutating every still-mutable column on the proposal object changes no execution-time decision.

### SDAR-3 — CRITICAL — `target_snapshot` declares no primary-key relationships, no foreign keys, and no tenant anchoring

- **Claim attacked:** §5.3 (fields only; "**Immutability:** application role has no UPDATE/DELETE privilege" is the entire constraint story) vs §6 interrogation matrix (asserts "Composite tenant FK" for target → snapshot, "exact principal ID" for snapshot → principal, exact policy binding for snapshot → policy).
- **Evidence:** §5.3 lists `snapshot_id, target_id, tenant_id, principal_id, resource_type, resource_id, scope, action_namespace, action_name, consequence_type, consequence_parameters, justification_set, policy_version_id, approver_principal_id, approver_principal_type, approved_at, snapshot_hash` and stops. Unlike §5.1/§5.4/§5.5, it declares **no keys/constraints section**: no FK to `authority_target`, no FK to `principal`, no composite FK to `policy_version (tenant_id, policy_version_id)`, no `UNIQUE(target_id)` or equivalent, no declared composite `(target_id, tenant_id)` anchor. The application role holds INSERT on `target_snapshot` (it must, to approve).
- **Attack (each independently succeeds against the schema as written):**
  1. Insert a snapshot with `tenant_id = B`, `target_id = A-target`, `principal_id = B-principal` — if the target FK is single-column, tenant A's target references a snapshot claiming tenant B. The §6 matrix's "composite tenant FK" is asserted nowhere in the object that needs it.
  2. Insert a snapshot whose `principal_id` does not exist, or exists under another tenant — no FK declared.
  3. Insert a snapshot whose `policy_version_id` names a version from another tenant — no composite FK declared (and `policy_version` PK `(tenant_id, policy_version_id)` makes the composite mandatory; a single-column FK would even be unwritable).
  4. Insert multiple snapshots for one target — undeclared; combined with SDAR-1 the "current" one is whichever the pointer says.
  5. Insert a snapshot for a target in another tenant's subtree and let a buggy activation bind it.
- **Exact failure boundary:** every security-sensitive property the snapshot is supposed to carry by relationship (tenant match, principal match, policy match, one-authority-per-target) is prose, not structure. DDL written from §5.3 omits these constraints because the design does not state them.
- **Why the schema fails:** the design's central object is the only one whose relational anchor was never specified. A schema can contain every correct column and still be unsafe because the relationships manufacture authority — here the relationships are simply missing.
- **Required resolution:** declare the full key graph for `target_snapshot`: PK; FK `(target_id, tenant_id) → authority_target(target_id, tenant_id)`; FK `(tenant_id, principal_id) → principal(tenant_id, principal_id)`; FK `(tenant_id, policy_version_id) → policy_version(tenant_id, policy_version_id)`; FK `(tenant_id, approver_principal_id) → principal(...)`; and the one-authority representation rule (e.g., `UNIQUE(target_id, tenant_id)` on the *approved* snapshot, or the SDAR-1 activation record carrying uniqueness). Every FK must be tenant-composite.
- **DDL-blocking? YES**
- **Exit test:** each of the five inserts above is rejected by the schema (constraint violation, SQLSTATE surfaced verbatim), not by service code.

---

## High Findings

### SDAR-4 — HIGH — C-2 is not resolved to a database mechanism: the epoch binding is "conceptual," has no committed referent, and neither admissible direction was implemented

- **Claim attacked:** §3 C-2 ("A `justification` references `(belief_id, promotion_epoch)`"), §5.4 ("the promotion reference is conceptually bound to the `belief_id + promotion_epoch` pair"), §9, §19 ("Re-promotion cannot revive old authorization reference — DB relationship using epoch — PROPOSED"), §21 condition 2.
- **Evidence:** a composite FK from `justification` needs a real referent for `(belief_id, promotion_epoch)`. The **locked expected direction** is the `belief_promotion` side table: `belief_promotion(promotion_id, belief_id, promotion_epoch, promoted_at)` with `PRIMARY KEY(promotion_id)`, `UNIQUE(belief_id, promotion_epoch)`, `FK(belief_id) → belief(id)` — preserving the frozen core. The alternative is an additive `belief.promotion_epoch` column with `UNIQUE(id, promotion_epoch)`, which is a **frozen-core exception admissible only with explicit approved evidence** (none exists in the artifact or the corpus; the earlier additive-column choice is explicitly superseded). The design under review implements **neither**: its own word for the binding is "conceptually" (§5.4), while §19 simultaneously claims the enforcement layer is "DB relationship using epoch." There is no such relationship. Additionally unspecified: epoch backfill for existing promoted beliefs; monotonicity/decrease prevention (no CHECK); epoch-on-retraction semantics beyond prose; interaction with `belief_id_status_key UNIQUE (id, status)` (the existing composite target remains, and two FK targets on the same table would encode overlapping truths about promotion state).
- **Attack:** with no referent, a justification row can bind `(belief_id, promotion_epoch = 47)` for a belief whose real epoch is 2 — a manufactured epistemic state. The approval snapshot (which copies the epoch from the justification) then preserves the fabricated epoch as "the exact promotion occurrence" the approver approved. The stale-justification-revival attack C-2 was chosen to defeat is closed only if the service always writes true epochs — the same "the service will check it" posture the mandate rejects. The rule that must hold: **no committed `belief_promotion` row = no valid justification for that epoch.**
- **Exact failure boundary:** the C-2 exit test from the preceding review ("Schema specifies either `(belief_id, status_epoch)` FK or snapshot-embedded justification") is not met: the design specifies neither. It gestures at the first while structurally forbidding it.
- **Why the schema fails:** the design wants the frozen core untouched (C-6) and an epoch FK (C-2) — these are reconcilable via the side table, and the design reconciled them by not noticing the problem.
- **Required resolution (leading with the locked direction):** implement the **`belief_promotion` side table** — append-only, `PRIMARY KEY(promotion_id)`, `UNIQUE(belief_id, promotion_epoch)`, `FK(belief_id) → belief(id)`, tenant-structural via `FK(tenant_id, belief_id) → belief_tenant(tenant_id, belief_id)` if tenant-aware relationships require it, `CHECK(promotion_epoch >= 1)` — with `justification` carrying a real composite FK `(tenant_id, belief_id, promotion_epoch) → belief_promotion`. First committed promotion = epoch 1; increment per promotion; retraction adds/removes no row; concurrent promotions serialize on the kernel's status transition plus the UNIQUE. Alternatively, the additive `belief.promotion_epoch` column with `UNIQUE(id, promotion_epoch)` is acceptable **only** with explicit approved evidence of the frozen-core exception. Either way: backfill rule for already-promoted beliefs stated with an exact epoch value and fail-closed semantics (no fabricated provenance).
- **DDL-blocking? YES**
- **Exit test:** inserting a justification with an epoch that was never a committed promotion of that belief is rejected by a constraint, and the backfill semantics for pre-migration promoted beliefs are stated with an exact epoch value.

### SDAR-5 — HIGH — The discharge `status='retracted'` sketch creates an UPDATE path that reopens instrument replay through revocation

- **Claim attacked:** §5.5 (`status … 'accepted', 'retracted' if future revocation semantics require it`), the partial UNIQUE `WHERE status = 'accepted'`, D9 ("Discharge revocation: a counter-entry in the append-only ledger … never deletion of history").
- **Evidence:** the uniqueness rule excludes non-accepted rows. The moment `status` is an updatable column with a `retracted` value, the sequence *accept → retract (UPDATE status) → re-accept the same instrument* passes the partial UNIQUE twice for one instrument. The preceding review's D9 position is that revocation is a counter-entry, never a mutation of the accepted row. The design's own §10 says "No destructive deletion of accepted history" but §5.5's speculative status value implies exactly a mutation path.
- **Attack:** instrument A discharges obligation X (accepted). Operator revokes the discharge (UPDATE status='retracted'). Attacker — or a legitimate retry that was never told the first attempt succeeded — re-presents instrument A; the partial UNIQUE now finds no accepted row and accepts it again. If the first acceptance already retired the debt string, the second acceptance is a free replay that can retire the *same* obligation on another belief (the cross-belief path C-3 exists to close) after the revocation window "freed" the uniqueness slot.
- **Exact failure boundary:** every revocation widens the replay window; uniqueness is only as strong as the status column is immutable — and the design proposes it mutable.
- **Why the schema fails:** it resolved C-3 with a rule whose enforcement evaporates under the lifecycle the same section sketches.
- **Required resolution:** accepted rows are immutable; if revocation is needed later it is a separate insert-only counter-entry. If a revocation structure is not required for MVP — which is the case — the design must state explicitly that discharge revocation is **FUTURE / UNSCOPED** rather than carry a speculative status value. The accepted-row uniqueness must remain effective for the lifetime of the instrument/obligation identity.
- **DDL-blocking? YES**
- **Exit test:** no sequence of transactions under the application role can produce two accepted rows with the same `(tenant_id, obligation_key, policy_version_id, instrument_ref)`, including through any revocation path the design admits; the status domain contains no speculative values.

### SDAR-6 — HIGH — C-4: the obligation→policy binding is correctly represented but never consumed by the transitional promotion path; the claim must match the residual

- **Claim attacked:** §3 C-4 ("A v1 discharge does not silently satisfy a semantically different v2 obligation"), §5.5 (`policy_version_id` on `debt_discharge`), §7 ("v1 discharge satisfies semantically different v2 obligation … DB: policy FK + policy-bound obligation identity"), §10 transitional model.
- **Evidence:** the review distinguishes two questions. **(A) Is the obligation identity correctly represented?** Yes — `(obligation_key, policy_version_id)` as the durable discharge identity is the right model, and the v1≠v2 statement is correct as far as the ledger is concerned. **(B) Is that identity actually consumed by the current transitional promotion path?** No — the promotion gate is `promoted_is_debt_free`: `coalesce(array_length(debt,1),0) = 0` over unversioned strings (`belief.debt TEXT[]`, `db/001_schema.sql`). The discharge transaction the design requires is "record accepted discharge + `array_remove(debt, obligation_key)` + commit" (§10). Nothing in that transaction reads `policy_version_id`; nothing relates the array element to a policy version; the array element has no version to relate. The ledger's policy binding is written and then consulted by no enforcement path in the MVP model.
- **Attack:** policy v1 defines `needReview` as `deterministic`; a discharge under v1 retires the string `needReview` and the belief is promoted. Policy v2 redefines `needReview` as `attested`. The belief is *already promoted* — v1's discharge satisfied, irrevocably, what v2 declares semantically different. Symmetrically, a v2-obligation discharge recorded in the ledger retires the same v1-era string. The ledger's binding is audit decoration in exactly the sense the design elsewhere refuses ("a receipt that cannot be traced to a specific row is decoration").
- **Exact failure boundary:** question B fails in the transitional model; C-4's guarantee holds only in the future obligation-rows model (§10 long-term direction), which this design explicitly does not build. The §7 impossible-states row's "DB: policy FK + policy-bound obligation identity" is false as written — the FK exists on a row nobody reads.
- **Why the schema fails:** the defect is not the model — it is the claim. §7 and §19 assert DB enforcement the transitional model does not deliver. The prior corpus (D8/D9) deliberately accepts transitional dual truth *with* single-transaction atomicity; it does not license claiming version-blindness away.
- **Required resolution (MVP-acceptability determination):** the residual is **acceptable for MVP production only under honest labeling**: state that `(obligation_key, policy_version_id)` is the durable discharge identity, that current promotion from `belief.debt[]` remains a transitional service-level semantics because the frozen array does not encode policy version, and explicitly that **transitional debt semantics are NOT structurally protected against a policy redefining the same debt string** — with §7 and §19 corrected to match. If that residual is judged unacceptable for MVP, the alternative is the smallest durable bridge that consumes the policy identity in the discharge transaction (same-`ExecuteTx` verification that `obligation_key` exists in the target belief's current `debt` array and that the discharge's `policy_version_id` is the currently authoritative obligation vocabulary — transactional service enforcement, not a row-local CHECK). Either branch is admissible; silence and overclaim are not.
- **DDL-blocking? YES** (truth-labeling condition: DDL may proceed only when the claims table no longer asserts DB enforcement that the transitional model does not deliver).
- **Exit test:** every §7/§19 row that cites the obligation-policy FK names the enforcement layer that actually consumes it, and a test demonstrates a v1 discharge against a v2-redefined obligation is either blocked or explicitly documented as an accepted transitional residual.

### SDAR-7 — HIGH — The declared composite tenant FKs are impossible against the declared keys; cross-tenant principal and policy references remain representable

- **Claim attacked:** §4 ("Composite tenant-aware references from authority objects must carry the tenant relation"), §6 interrogation matrix ("wrong tenant blocked? yes" on five rows), §19 ("Cross-tenant authority relationships are impossible — DB composite FKs — PROPOSED"), D3 (composite binding is the minimum that makes the invariant structural).
- **Evidence:**
  - `principal` PK is global `principal_id`; there is no `UNIQUE (tenant_id, principal_id)`. A composite FK `(tenant_id, discharged_by) → principal(tenant_id, principal_id)` — the only form that enforces tenant match — has no declared target. §5.1 says only "Composite tenant-aware references from authority objects must carry the tenant relation," which is a wish, not a key.
  - `debt_discharge.discharged_by → principal` is declared single-column (§5.5 Relationships). As declared, a tenant-A discharge can reference a tenant-B principal. Same for `policy_version.author_principal_id → principal` ("within tenant semantics" — no composite key exists to express that with).
  - `target_snapshot` (per SDAR-3) declares no principal or policy FKs at all.
  - `justification (target_id, tenant_id) → authority_target(target_id, tenant_id)` works only because `target_id` is already the PK — unique by accident of the key choice, not by tenant design.
- **Attack:** tenant A creates a discharge with `discharged_by = <tenant-B principal>`; the FK accepts it; attribution, revocation checks, and any future I-A4 role evaluation now resolve against the wrong tenant's principal. Tenant A's snapshot references tenant B's policy version; approval-time policy semantics silently cross the boundary. Identical UUIDs across tenants — the exact test the mandate requires — succeed everywhere the FK is single-column.
- **Exact failure boundary:** every "wrong tenant blocked? yes" cell in §6 that depends on a principal or policy FK is unsupported by the declared keys. The D3 invariant "no cross-tenant authority relationship, ever" is structurally unenforced in the parts of the graph the design left single-column.
- **Why the schema fails:** composite FKs require composite UNIQUE/PK targets; the design declared the FKs' intent but not the keys' existence, and DDL from this document would produce single-column FKs (they are the only writable ones).
- **Required resolution:** add `UNIQUE (tenant_id, principal_id)` to `principal`; re-declare every principal/policy reference as a tenant-composite FK (`discharged_by`, `author_principal_id`, snapshot `principal_id` and `approver_principal_id`, justification `attached_by`, authority_target `created_by` — the design currently does not even mention the last three as FKs); verify each FK's target key exists. No sentence may say "composite FK" without naming its exact source and target keys.
- **DDL-blocking? YES**
- **Exit test:** for each of the six referencing objects, an insert naming a same-UUID principal/policy from a different tenant is rejected by a named constraint.

### SDAR-8 — HIGH — Approval transaction semantics are unspecified, and serializable retry can change the approved justification set after human review

- **Claim attacked:** §15 ("Approve vs Attach: approve reads committed set then snapshots; whichever commits first is reflected"), the critical question of Attack Group 3 ("Can the exact set of beliefs that authorized a target ever differ from the set the approver actually approved?"), and locked principle 6 (MVP justification semantics are AND).
- **Evidence:** nowhere does the design specify what the Approve transaction validates: that each justification's belief is `promoted` *now*; that the live justification's `promotion_epoch` matches a committed promotion occurrence; that the justification set is exactly what was presented for approval; that the approver satisfies role/policy requirements; that the policy version is valid for approval. The only specified mechanics are "one `ExecuteTx`" and "activation cannot commit without snapshot FK." The retry behavior makes this concrete: under `crdb.ExecuteTx`, a `40001` aborts and *re-executes* the transaction body. If an Attach commits between the human's review and the first attempt, the retried approval silently includes the new justification in `justification_set` — the snapshot's "exact approved justification set" now contains a link no human ever saw.
- **Attack (interleaving):**
  1. T_human reviews justification set {B1} for target T. Begins Approve tx; reads {B1}.
  2. T_agent commits Attach(B2, T).
  3. T_human's tx conflicts (read of `justification` vs T_agent's write) → `40001`.
  4. `crdb.ExecuteTx` retries; re-reads {B1, B2}; snapshots both; sets state='active'.
  5. Execution-time verification reads the snapshot: authority now rests on B2, which the approver never reviewed.
- **Exact failure boundary:** the AND-set that authorizes the target is whatever the *last retry* reads, not what the approver approved. The snapshot is faithfully immutable — of the wrong set.
- **Why the schema fails:** "serializable" is true and irrelevant here; serialization preserves consistency, not *human intent*. The design's matrix row even normalizes this ("whichever commits first is reflected") without noticing that the loser is the human's approval.
- **Required resolution:** the design must specify the Approve transaction's contract: (a) the exact precondition reads (belief status + committed promotion occurrence per justification, tenant, policy validity, approver state); (b) a set-pinning mechanism — preferred: RequestAuthorization pins the candidate justification set, and Approve verifies that exact set still matches, refusing with an explicit re-request on divergence. The pinned request must have defined durable state (e.g., `requested_by`/`requested_at` on the proposal object, with the pin defined as "justifications attached at or before `requested_at`" — sound because justifications are append-only and can only grow, so divergence is completely detectable). On `40001`, the retry must revalidate the pinned request; divergence → explicit refusal / re-review; **never silent set expansion**. No workflow engine is needed for this.
- **DDL-blocking? YES** (this is the transaction design DDL must support; the schema must carry the pinning state).
- **Exit test:** the interleaving above ends in either an explicit refusal or a re-approval event — never in an active snapshot containing an unreviewed justification.

### SDAR-9 — HIGH — Target lifecycle monotonicity has no enforcement: `revoked → active` resurrection and `proposed → active` without approval are representable

- **Claim attacked:** §5.2 lifecycle ("Revocation does not delete the row"), §5.2 constraints, §7 ("active target + revoked approver … historical approval remains attributed"), locked principle 9 (approval is the authority-granting transition).
- **Evidence:** `state` is a plain mutable column with a CHECK domain only (`'proposed','active','revoked'`). CockroachDB has no triggers; no transition table; no partial constraint enforces direction. UPDATE privilege on `authority_target` exists (the lifecycle requires it). Nothing prevents: `UPDATE ... SET state='active' WHERE target_id=$t` on a revoked target (resurrection with its old snapshot — the full old authority, no new approval); `proposed → active` issued by any code path other than Approve; `revoked → proposed` (state laundering); or `state='active'` with `snapshot_id NULL` (§5.2 states the requirement as prose — "An active target must reference exactly one snapshot" — but never as a declared CHECK).
- **Attack:** revoke target T during an incident. The revoke handler's caller, or a stale retry of the original approval (client retried after a timeout, `crdb.ExecuteTx` already committed), re-sends `state='active'`. The idempotency protection the design gives Discharge ("unique violation / 40001") has no analogue for activation: the second `proposed → active` UPDATE on a row already 'revoked' simply succeeds if written without a `WHERE state='proposed'` guard — and the design does not obligate one structurally.
- **Exact failure boundary:** the entire lifecycle is service discipline. The design's locked principle 9 ("Approval creates the authoritative state") is enforceable here and was not enforced.
- **Why the schema fails:** the preceding review's D7 position — serialization must be transactional structure, not API surface — was applied to Approve×Approve but not to the state machine itself.
- **Required resolution:** make transitions structural: either (a) declared CHECKs pinning the invariants that are row-local (`state <> 'active' OR snapshot_id IS NOT NULL`; `state <> 'revoked' OR revoked_at IS NOT NULL`), plus an insert-only activation structure per SDAR-1 that inherently cannot re-activate (a second activation row for an already-activated target violates its uniqueness); or (b) a `target_state_transition` append-only table where each transition is an INSERT and "current state" is the latest row — resurrection then requires a fresh `proposed → active` insert, which the Approve-only transaction contract gates. The design must pick one and declare the CHECKs explicitly.
- **DDL-blocking? YES**
- **Exit test:** direct SQL setting a revoked target to active (or activating without a snapshot) is rejected by a named constraint or made impossible by the transition structure.

---

## Medium / Low Findings

### SDAR-10 — MEDIUM — `belief_tenant` lacks an FK to `belief`, an immutability mechanism, and an insertion authority

- **Claim attacked:** §5.7 ("backfilled once per belief; mapping is immutable"), §16 backfill rules.
- **Evidence/Attack:** `belief_tenant(tenant_id, belief_id, mapped_at)` declares no FK `belief_id → belief(id)` — a mapping can exist for a nonexistent belief, and the composite FKs from `justification`/`debt_discharge` into `belief_tenant` then "validate" against a phantom. "Mapping is immutable" has no mechanism (application role UPDATE). Insertion authority is unspecified: new beliefs are created by the frozen kernel (`EnterBelief`), which knows nothing of tenants — so every kernel-created belief has no mapping until some service inserts one, and that insert is itself a tenant-assignment act performed by the same role that writes authority rows (confused-deputy surface: a bug that maps a belief to tenant B moves the belief's entire authority surface). Partial backfill states (belief with no mapping) fail closed via FK — acceptable — but the design should say so explicitly. Deletion/recreation of mappings is not addressed.
- **Required resolution:** declare FK `belief_id → belief(id)`; `UNIQUE(belief_id)` already prevents reassignment-by-insert; make the immutability claim structural (no UPDATE path / insert-only privilege posture) or drop the claim; name the component allowed to insert mappings and state that mapping creation is a provisioning event, not an authority operation.
- **DDL-blocking? NO** (the 1:1 rule and composite-FK direction are sound; these are scaffolding gaps) — but the FK is cheap and should land with DDL.

### SDAR-11 — MEDIUM — Requester identity is absent; HIGH_IMPACT separation of duties is unrecordable

- **Claim attacked:** authority-creation spec (for HIGH_IMPACT, `approver ≠ requester`); locked principle 7.
- **Evidence/Attack:** `authority_target.created_by` records the *creator*. The RequestAuthorization operation — a distinct locked operation — has no representation at all: no row, no column, no timestamp. If requester ≠ creator (the common case when an agent creates and a human operator requests), nothing distinguishes them, and the separation-of-duties check "approver ≠ requester" cannot be evaluated from durable state. This is not necessarily a new table — but the design must either record requester attribution (a nullable `requested_by`/`requested_at` on the proposal object from SDAR-2's resolution) or state explicitly that MVP evaluates separation against `created_by` and accept that a creator-approved target is representable. `created_by`, `requested_by`, and `approved_by` are different actors/roles and must be distinguishable.
- **DDL-blocking? NO** — sequencable, but must be decided before DocTrust flow 4 can assert the separation property.

### SDAR-12 — MEDIUM — No strict domain CHECKs on opaque fields; empty-string and empty-JSON wildcard probes succeed

- **Claim attacked:** §12 ("No DEFAULT consequence is permitted. Missing consequence is an authorization failure"), §7 NULL-wildcard rows, Attack Groups 21/23.
- **Evidence/Attack:** `scope`, `resource_type`, `resource_id`, `action_namespace`, `action_name`, `consequence_type`, `consequence_parameters`, `obligation_key`, `instrument_ref` are all NOT NULL — and nothing more. `''`, `{}`, `[]` all insert cleanly. `scope = ''` byte-compares equal to itself, satisfies exact equality, and its *interpretation* ("does empty scope mean unrestricted?") is left to the service — the design says "the service owns any future narrowing relation," which is honest, but the schema could cheaply refuse the degenerate representations that historically become wildcards. The design's own fail-closed rule (`resource=''` must mean NOT AUTHORIZED, per the trust-model doc) is not expressed anywhere in §12. Fields must not be forced into identical validation where that would invent semantics — the design decides per field which empty representations are invalid.
- **Required resolution:** declare per-field rules — NULL → invalid/NOT AUTHORIZED everywhere; empty string → invalid unless explicitly meaningful; empty JSON → invalid where a structured payload is required (e.g., `consequence_parameters`) — or state explicitly that empty representations are legal and are interpreted as NOT AUTHORIZED by the service. One or the other; ambiguity here is how wildcards are born.
- **DDL-blocking? NO** (one-line CHECKs at DDL time) — but the choice must be made in this design, not at the keyboard.

### SDAR-13 — MEDIUM — C-7 boundary must be explicit and internally consistent; enforcement is service-only; revoked dischargers unaddressed

- **Claim attacked:** §3 C-7 (`issued_at <= revoked_at`), §5.5 (`issued_at` NULL "when applicable"), §19 ("Post-revocation attestation issuance is refused — Service … — PROPOSED").
- **Evidence/Attack:** (a) the temporal boundary is stated but not *resolved*: the design must commit to exactly one of `issued_at < revoked_at` or `issued_at <= revoked_at` and justify it against the attestation semantics — the review recommends `<` (fail-closed at the ambiguous instant) but does not mandate it; what is mandatory is that the boundary be explicit and internally consistent. (b) The relation is enforced nowhere in the schema — `issued_at` and `principal.revoked_at` live in different tables and no CHECK can span them; the design is candid that it is service enforcement, but then §7's row ("post-revocation attestation accepted … DB: `issued_at`/revoked relation represented in discharge fields where applicable") implies a DB representation that does not constrain anything. (c) `issued_at` is NULL for non-attestation instruments — legitimate, but then "attestation validity" cannot be type-distinguished in the schema; if a future instrument kind carries issuance semantics, the NULL is ambiguous. (d) Unaddressed entirely: may a *revoked principal* appear as `discharged_by` on a new discharge? Revocation "blocks new approvals" (§3 C-7) — discharges are a different write path, and a revoked agent's discharge goes unmentioned. (e) Attestation *authenticity* of the issuance timestamp is IdP-trusted — correctly out of scope — but the design should say that `issued_at` is a trusted input, not a Solvent-verified fact. The three distinct concerns — attestation issuer validity, discharger authority, principal revocation — must be separated.
- **Required resolution:** commit to one boundary and justify it; state that the revocation/issuance relation is transactional service enforcement (and correct §7's DB column); define whether `discharged_by` must be non-revoked at acceptance time; mark `issued_at` as IdP-trusted provenance; do not invent a credential table.
- **DDL-blocking? NO** — semantics, not structure — but it must be settled before the discharge transaction is written.

### SDAR-14 — MEDIUM — Policy revision allocation and version-identity generation are unspecified; the C-5 exit test's wording is not met

- **Claim attacked:** §3 C-5 ("allocated transactionally under serializable execution"), §5.6 (PK `(tenant_id, policy_version_id)`, `UNIQUE (tenant_id, revision)`), preceding review's C-5 exit test ("policy_version primary key enforces total order per tenant").
- **Evidence/Attack:** "allocated transactionally" names no mechanism: max+1 scan, dedicated counter row, or sequence-per-tenant are different designs with different contention and failure modes, and the retry story differs for each (a max+1 scan under two concurrent writers yields one `40001` then a unique violation on `(tenant_id, revision)` if the loser's retry re-reads correctly — workable, but the design must say which). The PK `(tenant_id, policy_version_id)` does not itself enforce order (the exit test's literal wording); `UNIQUE (tenant_id, revision)` does — acceptable, but the deviation should be stated rather than silent; the design must not claim the PK enforces order if the actual enforcement is the UNIQUE. Also unspecified: how `policy_version_id` is generated (random UUID? content-derived?); whether `policy_hash` is computed over a specified canonicalization (without one, "same content under different revision" vs "different content, colliding hash" are both untestable); and whether `revision` has a CHECK (`>= 1`) with a defined first revision.
- **Required resolution:** name the allocation mechanism and its retry contract; state how `policy_version_id` is minted; specify the hash canonicalization or demote `policy_hash` to integrity-of-storage; add the domain CHECK; define the transaction that allocates revision.
- **DDL-blocking? NO** — but condition 5 of the design's own DDL-readiness list cannot close without it.

### SDAR-15 — MEDIUM — Justification has no natural key; duplicate links are representable

- **Claim attacked:** §5.4 ("Primary key: implementation-chosen stable row ID or deterministic composite identity").
- **Evidence/Attack:** nothing prevents N rows for `(target_id, belief_id, promotion_epoch)` — duplicate Attach operations all succeed. Under AND semantics duplicates are harmless for activation, but they corrupt the audit history ("the exact set the approver approved" gains phantom multiplicity), they poison any future quorum semantics (I-P2's "not the same artifact N times" begins with not counting the same link N times), and the approval snapshot's serialization must dedup — an unspecified step. The design left the PK itself undecided ("implementation-chosen"), which is not a resolution.
- **Required resolution:** declare `PRIMARY KEY (justification_id)` and `UNIQUE (tenant_id, target_id, belief_id, promotion_epoch)` (idempotent attach becomes insert-or-ignore).
- **DDL-blocking? NO.**

### SDAR-16 — MEDIUM — Instrument identity is pointer-only; byte-identical evidence can acquire distinct identities

- **Claim attacked:** §3 C-3 ("`instrument_ref` is an opaque, durable instrument identifier whose authenticity and eligibility are verified by the service"), §14, Attack Group 8.
- **Evidence/Attack:** the uniqueness rule is only as good as identity minting. The transitional `instrument_ref` "may refer to current belief-owned evidence or an external attestation identifier" — two callers referencing the same evidence row by different spellings (UUID vs URL vs hash), or the same artifact ingested twice into two belief-owned evidence rows (`evidenceExists` dedup is per-belief only — verified in `internal/belief/belief.go`), get two instrument identities and defeat the UNIQUE. The design acknowledges the service "verifies" the instrument but does not specify what identity function it applies; the preceding review's F-5.2-12 flagged exactly this transitional coupling.
- **Required resolution:** specify the transitional identity function — at minimum an `instrument_kind` discriminator plus a canonical reference form (evidence row reference = canonical evidence UUID; external attestation = canonical attestation identifier), with the explicit residual that canonicalization is a service responsibility in MVP unless a stronger DB representation is justified. Multiple spelling forms of the same instrument must not become distinct identities.
- **DDL-blocking? NO** — but C-3's exit test ("duplicate discharge rejected") is only demonstrable once identity minting is deterministic.

### SDAR-17 — MEDIUM — `approver_principal_type` in the snapshot reintroduces the duplication the preceding review removed

- **Claim attacked:** §5.3 (`approver_principal_type … approval-time attribution snapshot`).
- **Evidence/Attack:** `SCHEMA_DESIGN_ADVERSARIAL_REVIEW_FINAL.md` explicitly lists among its fixes over the prior plan: "removed duplicated `approver_principal_type` (I-A4 derives type from principal row via FK)." The design reinstates it. Since §5.1 makes `principal_type` immutable, the copy is derivable forever — a pure second source of truth whose only future use is to *diverge* (and any I-A4 evaluation reading the copy instead of the principal row is reading a value no revocation or correction can reach). If the intent is historical attribution of what the approver's type was at approval, immutability makes that identical to reading the row. If legitimate type transitions ever need history, the selected principal transition model owns that — not a casually duplicated field.
- **Required resolution:** remove the column; I-A4 evaluation reads the principal row through the tenant-composite FK. If a divergence argument exists (type corrected retroactively — but §5.1 forbids type mutation), state it; otherwise this is duplicated truth with no semantics.
- **DDL-blocking? NO.**

### SDAR-18 — MEDIUM — Migration leaves epoch backfill and mixed-path authorization sequencing unspecified

- **Claim attacked:** §9 epoch semantics ("Entered state starts at epoch 0"), §16, §21 condition 2.
- **Evidence/Attack:** existing promoted beliefs (the demo corpus, `track1`/`track2`) have no epoch. Backfill options have different semantics: epoch 0 (meaning "never promoted," false for promoted rows), epoch 1 (meaning "first promotion," true but then a pre-migration justification binding epoch 1 *revalidates* against the backfilled epoch — the exact revival C-2 exists to prevent, laundered through backfill). The safe direction: existing promoted beliefs receive **no** valid promotion epoch automatically unless the migration has evidence of a real promotion occurrence; if the migration cannot reconstruct the historical promotion, authority-dependent relations must not be silently revalidated; if a backfill uses epoch 1, the design must state what evidence makes that historical promotion legitimate. Fail closed rather than fabricate provenance. Mixed paths: after the authority tables exist, `action_intent` remains live under the frozen kernel; the design correctly says old and new semantics must not share an authorization path, but does not state which paths remain open during the transition window or how `solvent_authorize_action` (MCP, live today) is constrained — the transition-window rule must be explicit: **the old path must not grant new production authority while new authority semantics are being introduced** (without modifying the frozen kernel in the design task).
- **Required resolution:** specify the evidence-gated backfill rule with its fail-closed semantics and the transition-window authorization-path policy.
- **DDL-blocking? NO** — sequencing, but it must be decided before migration design hardens.

### SDAR-19 — MEDIUM — Retraction-after-approval semantics are delegated to service discretion; DocTrust flow 7 is not deterministic

- **Claim attacked:** §8 ("belief retraction → reassessment/re-verification → DENY / REVOKE when the current authorization policy requires it"), §17 flow 7 ("verify re-evaluation/revocation behavior").
- **Evidence/Attack:** the design correctly refuses to auto-mutate the snapshot and correctly derives that the approved record persists — but then leaves the operative question ("does existing authority immediately cease, or remain valid until reassessment?") to "the current authorization policy requires it," which is unspecified policy. Two conforming implementations disagree (immediate DENY vs valid-until-revoke), and DocTrust flow 7 has no deterministic expected outcome. The preceding corpus (D5) actually pins a stronger rule: justification validity must be *re-verified, not cached, at every pre-execution check*. That is derivable and should be stated as the MVP rule: **every pre-execution authorization verification re-checks the current validity of each snapshot justification against its exact promotion occurrence**; if the belief has been retracted, the snapshot remains immutable historical approval but verification fails → DENY / REVOKE per the authorization policy; and the later re-promotion at a new epoch does **not** revive the old snapshot's justification.
- **Required resolution:** pin the MVP rule (recommended: every pre-execution verification re-checks each snapshot justification's belief status and promotion occurrence against current state; a failed check yields DENY and, per policy, a revocation transition; re-promotion never revives) and make flow 7 assert it.
- **DDL-blocking? NO.**

### SDAR-20 — LOW — Spec-defect residue in the derivation table and object specs

- **Evidence:** §4 row "F1 principal revocation … NOT NULL when set" is not a constraint (a nullable column is NULL-able; "NOT NULL when set" is a tautology) — the real rule (revoked_at set exactly once, never cleared) is unstated and unenforced. §5.4's PK is "implementation-chosen"; §5.5's status domain contains a speculative value (SDAR-5); §5.1's "CHECK/domain for permitted principal types" does not name the domain. None of these blocks DDL individually; collectively they are the kind of imprecision that becomes a per-engineer decision.
- **Required resolution:** tighten the phrasings; declare the `revoked_at` write-once rule (service or privilege posture).

### SDAR-21 — LOW — `policy_hash` canonicalization is unspecified; hash ≠ version semantics preserved but hash utility undefined

- **Evidence:** §5.6 requires `policy_hash NOT NULL` and `policy_content NOT NULL`; the design correctly treats version identity (not the hash) as authoritative. But without a canonicalization rule the hash cannot detect the mutations it exists to detect ("identify exact content" per §4). Cheapest resolution: define the hash input as the exact stored `policy_content` bytes and demote the hash to integrity-of-storage.
- **DDL-blocking? NO.**

---

## C-1 Verdict — Snapshot Authority

**NOT RESOLVED — FAILS ITS EXIT TEST.**

The semantic direction (snapshot-authoritative execution; live justification as audit-only) is correctly restated and consistently applied in §8 and §13. The physical mechanism fails the preceding review's own bar ("The schema should make the immutability impossible, not merely checked") at three points: the mutable `snapshot_id` pointer on the one row the lifecycle must be able to UPDATE (SDAR-1); the complete absence of declared keys/FKs on `target_snapshot` (SDAR-3); and the homeless proposal tuple (SDAR-2). The privilege posture (app role SELECT/INSERT only on the snapshot) is a genuine and creditable mechanism *against the buggy service* — but it guards the snapshot's content while leaving its selection mutable, so the property it exists to deliver ("active authority must not silently mutate") is not delivered. `snapshot_hash` is correctly subordinated ("detection/audit, not authority by itself").

**Blocking conditions:** SDAR-1, SDAR-2, SDAR-3.

---

## C-2 Verdict — Promotion Epoch

**NOT RESOLVED — FAILS ITS EXIT TEST.**

Epoch choice is right (smallest durable discriminator; no event sourcing; snapshot copies the epoch; retraction does not increment). But the design's own text concedes the binding is "conceptual" (§5.4) while §19 claims "DB relationship using epoch" — a claim with no referent. The locked expected direction is the **`belief_promotion` side table** (`UNIQUE(belief_id, promotion_epoch)`, FK to `belief`, frozen core preserved); the additive `belief.promotion_epoch` column is admissible only as an explicitly approved frozen-core exception. The design implements neither, so the conflict it wanted to avoid is unresolved rather than avoided. Backfill for existing promoted beliefs is unspecified and must be evidence-gated and fail-closed (SDAR-18). An un-referenced epoch is a service-asserted integer that a hostile or buggy writer can fabricate (epoch 47). See SDAR-4 for the resolution.

**Blocking condition:** SDAR-4.

---

## C-3 Verdict — Discharge Replay

**RESOLVED AT THE RULE LEVEL; UNDERMINED BY THE LIFECYCLE SKETCH — CONDITIONAL.**

The rule itself is correct and correctly stronger than per-belief uniqueness: `UNIQUE (tenant_id, obligation_key, policy_version_id, instrument_ref) WHERE status='accepted'`, explicitly distinguishing obligation uniqueness from instrument replay, and explicitly scoping cross-obligation reuse as a separate question (correctly not auto-prohibited). Two defects: the speculative `retracted` status implies the UPDATE path that revives replay (SDAR-5 — resolved by declaring discharge revocation FUTURE / UNSCOPED for MVP and keeping accepted rows immutable), and instrument identity minting is unspecified so "one instrument" is not yet a well-defined object (SDAR-16). With those repaired, this is the strongest-resolved of the seven.

**Blocking condition:** SDAR-5. Sequenced: SDAR-16.

---

## C-4 Verdict — Obligation Identity

**REPRESENTATION RESOLVED; CONSUMPTION IS A DECLARED TRANSITIONAL RESIDUAL — CONDITIONAL.**

Two distinct questions, answered separately. **(A) Is the obligation identity correctly represented?** Yes — `(obligation_key, policy_version_id)` is the right durable discharge identity, and the v1≠v2 statement is correct as far as the ledger goes. **(B) Is that identity actually consumed by the current transitional promotion path?** No — the promotion gate reads only the unversioned `debt[]` array; the binding is recorded and read by nothing. The review's determination: this residual is **acceptable for MVP production only under honest labeling** — the design must state that transitional debt semantics are NOT structurally protected against a policy redefining the same debt string, and correct the §7/§19 rows that currently assert DB enforcement. It is *not* acceptable to leave the claim as written. If MVP production cannot tolerate the residual, the smallest durable bridge that consumes the policy identity in the discharge transaction is the alternative. This is a truth-labeling condition, not a redesign demand: the ideal future model (obligation rows → discharge rows → derived debt) remains the long-term direction.

**Blocking condition:** SDAR-6 (truth-labeling).

---

## C-5 Verdict — Policy Revision

**RESOLVED IN DIRECTION; MECHANISM DEFERRED — ACCEPTABLE WITH CONDITIONS.**

Per-tenant monotonic `revision` with `effective_at` as metadata is the correct resolution; refusing timestamps as authority ordering is explicit; append-only intent is stated; approval binds exact `policy_version_id`. Gaps: allocation mechanism unnamed (SDAR-14), version-identity minting and hash canonicalization unspecified, and the literal exit-test wording ("PK enforces total order") is satisfied by `UNIQUE (tenant_id, revision)` instead — fine, but the design must not claim the PK enforces order if the UNIQUE does. Downgrade *detection* (revision comparison) and the no-broadening rule are stated but their evaluation point (execution-time verification under current policy vs snapshot-time policy) inherits the SDAR-19 ambiguity; §8's re-verification sentence covers it if SDAR-19's rule is pinned.

**Blocking condition:** none beyond SDAR-14's specification duty. Sequenced with SDAR-19.

---

## C-6 Verdict — Tenant Binding

**RESOLVED — APPROVE WITH CONDITIONS.**

`belief_tenant (tenant_id, belief_id)` with `UNIQUE(belief_id)` one-to-one mapping, explicit legacy-scenario backfill, refusal to equate `scenario_id` with tenant identity, and composite FKs from authority relationships are all correct, and preserving the frozen core is the right call. Conditions: FK `belief_id → belief(id)` must exist (SDAR-10); mapping immutability needs a mechanism; the composite-FK scaffolding on `principal`/`policy_version` required by the rest of the graph is missing (SDAR-7 — the tenant model's core promise is only as strong as those keys).

**Blocking condition:** SDAR-7 (shared with Principal/Policy verdicts).

---

## C-7 Verdict — Attestation Revocation

**RESOLVED AT THE SEMANTICS LEVEL — CONDITIONAL.**

The validity relation (issuance-time bounded: pre-revocation attestations survive; post-revocation issuance invalid; compromise invalidation honestly deferred; replay routed to C-3) matches the preceding review's F-5.2-10 requirements and correctly refuses to invent a credential table. Conditions: the temporal boundary must be committed to explicitly — `<` or `<=` — and justified as internally consistent with the attestation semantics (the review recommends `<` as fail-closed at the ambiguous instant but does not mandate it); §7's implied DB enforcement must be corrected to service enforcement (SDAR-13); and the revoked-discharger question must be answered.

**Blocking condition:** none blocking; SDAR-13 sequenced.

---

## Principal Verdict

**APPROVE WITH CONDITIONS.**

Stable `principal_id` with IdP-owned credential lifecycle, rotation updating issuer on the same row, immutable `principal_type` (making I-A4 relationally expressible), new-identity-with-linkage for type transitions, `revoked_at` forward-looking — all correct and consistent with D1 and F-5.2-11. Conditions: no `UNIQUE (tenant_id, principal_id)` exists, so every tenant-composite principal FK the design relies on is unwritable (SDAR-7); the type domain CHECK is unnamed; `revoked_at` write-once is unstated (SDAR-20); and the principal table's authority meaning/non-meaning boundary (attribution ≠ role authorization) is correctly drawn in §5.1 and §11 — preserved.

## AuthorityTarget Verdict

**REJECT — REDESIGN REQUIRED.**

The lifecycle pointer concept is right; the object as specified is not: homeless proposal tuple (SDAR-2), mutable authority pointer (SDAR-1), unenforced state machine (SDAR-9), missing CHECK for active-requires-snapshot, missing requester attribution (SDAR-11). The interrogation matrix's claims about this object are not supported by its definition.

## target_snapshot Verdict

**REJECT — REDESIGN REQUIRED (as specified).**

Right idea, right field set at the content level (five-tuple, epoch-bearing justification set, policy identity, approver, approval time; `snapshot_hash` correctly subordinated), and the privilege posture is a real mechanism against accidental mutation. But zero declared keys/FKs (SDAR-3), the `approver_principal_type` regression (SDAR-17), the unverifiable `justification_set` blob (see Second-Source-of-Truth Audit), and no declared one-authority-per-target rule leave the design's central object structurally undefined.

## Justification Verdict

**APPROVE WITH CONDITIONS.**

The live table's demotion to proposal/audit history with no authority-granting state is exactly right and cleanly implements the F-5.2-1 corollary. Composite routing through `belief_tenant` is correct. Conditions: epoch binding needs a real referent — the `belief_promotion` side table per the locked direction (SDAR-4); natural key for idempotent attach (SDAR-15); `attached_by` should be a tenant-composite principal FK (SDAR-7).

## Debt / Discharge Verdict

**APPROVE WITH CONDITIONS — with one blocking repair.**

Separation of obligation/discharge/instrument/debt-state is clean; the single-transaction rule (record + `array_remove` + commit) preserves D9; attribution (`discharged_by`) restored as the preceding review demanded; no destructive deletion. Blocking: the `retracted` status UPDATE path (SDAR-5). Sequenced: instrument identity minting (SDAR-16), C-4 consumption honesty (SDAR-6), revoked-discharger rule (SDAR-13).

## Policy Version Verdict

**APPROVE WITH CONDITIONS.**

Immutable, tenant-bound, revision-ordered, `effective_at` demoted to metadata, author attribution correctly separated from author authorization (F-5.2-5 honored), `policy_content` stored for historical interpretation. Conditions: allocation mechanism, identity minting, hash canonicalization (SDAR-14/21); `author_principal_id` needs the tenant-composite FK (SDAR-7). The policy floor (I-P1/I-P2) is correctly NOT claimed as schema enforcement — consistent with the preceding review's "NOT READY FOR SCHEMA ENFORCEMENT — DOCUMENTED CONTRACT only" verdict; the design does not sneak floor CHECKs in.

## Tenant Verdict

**APPROVE WITH CONDITIONS.**

Structural-isolation intent is present and the `belief_tenant` bridge is the right mechanism (see C-6). The decisive gap is key scaffolding: without `UNIQUE (tenant_id, principal_id)` and fully composite FKs on every principal/policy/snapshot edge, D3's "no cross-tenant authority relationship, ever" remains service discipline in exactly the edges where an identical UUID across tenants does the most damage (SDAR-7). `tenant_id` NOT NULL on every authority object is consistently maintained. No tenant table — correct per D3.

## Evidence Verdict

**APPROVE.**

The transitional posture is honest: belief-owned evidence referenced opaquely via `instrument_ref`, no claim that caller-supplied `content_sha256` is Solvent-verified (matches `kernel/kernel.go`'s caller-supplied hash and D12), Artifact/Citation split preserved as a migration target without building it, sharing semantics correctly deferred. Residual: identity canonicalization (SDAR-16) and the standing truth that `evidenceExists` dedup is per-belief today — which the design's C-3 rule will fix at the discharge layer, not the evidence layer; correctly scoped.

## Concurrency Verdict

**APPROVE WITH CONDITIONS — the matrix is directionally right but two rows hide real races, and the mandate's interleavings are only partially answerable from the design.**

Constructed interleavings, with the design's mechanisms:

| Race | Serialized on | Outcome | Verdict |
|---|---|---|---|
| Approve × Approve | UPDATE of `authority_target` row (state/snapshot) — row write conflict | First commit activates; loser gets `40001`, retries, reads `state='active'`, refuses | OK, provided the retry contract (SDAR-8) specifies refuse-on-fresh-state |
| Approve × Attach | `justification` insert vs approval read of the set | Loser is the *approver*: retry silently enlarges the approved set | **SDAR-8 — HIGH** |
| Approve × Retract | belief status/epoch read vs `RetractCascade` write | Serializable retry re-reads; refusal only if the approval tx actually validates belief state — unspecified | **SDAR-8** |
| Approve × Policy Change | snapshot's `policy_version_id` bind vs policy INSERT | Insert-only policy never conflicts destructively; approval binds the version it read; new versions cannot alter bound snapshots (append-only) | OK |
| Discharge × Promote | debt array row (`array_remove` vs `Promote`'s status UPDATE) | Both touch the same `belief` row; serial order; promotion sees committed debt state; `promoted_is_debt_free` holds | OK |
| Discharge × Discharge | partial UNIQUE `(tenant_id, obligation_key, policy_version_id, instrument_ref) WHERE accepted` | One accepted; loser unique-violation or `40001`→re-read | OK — modulo instrument identity minting (SDAR-16) and the revocation window (SDAR-5) |
| Revocation × Approve | `principal.revoked_at` write vs approval's read of principal state | Serial order decides; approval refuses if revocation committed first | OK at the DB layer — but the approval tx must actually read principal state (SDAR-8) |
| Target mutation × Approve | "mutation denied by privilege/security boundary" | As specified, the mutation is *not* denied — the pointer is updatable (SDAR-1) | **FAILS as written** |
| Snapshot creation × Approval | same transaction; FK gates activation | OK once the snapshot FKs exist (SDAR-3) | Conditional |
| Tenant association × relationship creation | `belief_tenant` existence via FK | Fail-closed on missing mapping | OK (SDAR-10 FK adds the phantom-mapping case) |

Write skew / stale reads: serializable isolation plus `crdb.ExecuteTx` retry covers the classic cases *if and only if* each transaction's precondition reads are specified — which the design does not do (SDAR-8). The two most dangerous observed races are the pointer mutation and the approval-set enlargement; both are failures of specification, not of isolation.

## Migration Verdict

**APPROVE WITH CONDITIONS.**

Fail-closed rules are exactly right and complete (missing dimension → NOT AUTHORIZED; the seven "never" wildcards; legacy action strings not silently translated; no shared authorization path; rollback cannot reinterpret dimensions as optional). `belief_tenant` backfill via explicit legacy mapping is correct and `scenario_id` ≠ tenant is preserved. Conditions: epoch backfill must be evidence-gated and fail-closed for existing promoted beliefs (SDAR-18) and the transition-window policy for the live `solvent_authorize_action` path must be explicit — the old path must not grant new production authority while new semantics are being introduced (SDAR-18). No migration SQL appears — correct.

## Service TCB Verdict

**CONDITIONS ON THE DESIGN'S OWN CONSISTENCY.**

The design accepts service-in-TCB (§22 assumptions) consistent with D2 — good. But §3 C-1 and §19 market the privilege boundary as making snapshot mutation impossible "by application," which blurs the threat model: against a *compromised* service the boundary is worthless (the service holds the app role; it can INSERT snapshots and UPDATE the pointer — full authority manufacture, accepted as TCB failure), while against a *buggy* service the boundary is real and valuable for snapshot content — and then defeated by the updatable pointer on `authority_target` (SDAR-1). The redesign should state plainly: privilege separation defends snapshot *content* against accidental application mutation; it defends nothing about *which* snapshot is authoritative; and a compromised service remains a full TCB failure with no schema pretense otherwise. The trust-model doc's rule ("Solvent does NOT claim to protect against a fully compromised Solvent service") is honored in §7's residual cells but contradicted in spirit by §3's mechanism framing.

## Executor Boundary Verdict

**APPROVE — CLEAR.**

I-12 remains a PROPOSED source-spec label with exact-equality server-side matching; I-13 remains FUTURE / UNSCOPED with no schema claim of execution proof; Executor Mediation is quoted verbatim as a DOCUMENTED CONTRACT deployment precondition (§19); `action_intent` is explicitly barred from becoming a warrant store (§13); no execution receipt, no consequence-verification columns. The boundary hygiene here is the design's strongest section.

## DocTrust Verdict

**APPROVE WITH CONDITIONS.**

Flows 1–6, 8–11 map to deterministic schema/transaction outcomes (activation, tuple mismatch DENY, unique-violation on duplicate discharge, refusal on revoked approver, downgrade non-broadening, cross-tenant FK rejection) *once* the blocking repairs land — today flows 6, 8, and 11 test constraints that do not exist in the design. Flow 7 (retraction re-evaluation) has no deterministic expected outcome until SDAR-19's rule is pinned. The "what DocTrust does not prove" list is honest and complete (domain-agnostic completeness, IdP correctness, semantic truth, execution effects, composition safety, production multi-tenancy, future executors). Add: DocTrust also cannot prove the privilege model (it is deployment posture, not interface behavior) and cannot prove snapshot immutability under the administrative role.

## Formal / Lean Verdict

**APPROVE — CLEAR.**

The mapping table (§18) is honest: existing Lean theorems cover only the three current ledger invariants plus cascade atomicity; nothing formalizes the new objects; every new property is labeled "FUTURE formal property" or "schema/service property"; and the design explicitly disclaims that current Lean proves the new schema. No `sorry`-level shortcuts, no borrowed invariant IDs. When formalization resumes, the epoch-revival property and the single-execution-authority property are the natural first candidates — the design correctly does not pretend they are proven.

---

## Security Property Matrix

| Property | Forbidden State | DB Prevention | Transaction | Service | Identity | Policy | Executor | Residual |
|---|---|---|---|---|---|---|---|---|
| Belief promotion | promoted + outstanding debt | `promoted_is_debt_free` (CURRENT) | existing Promote tx | N1–N4 (N1 only, today) | — | floor rules future | — | none for the invariant |
| Promotion epoch | stale justification revalidating across re-promote | **none declared** (SDAR-4) | epoch update in Promote tx (proposed) | epoch assertion | — | — | — | fabricated epochs until referent exists |
| Justification validity | justification for non-promoted/never-committed epoch | epoch FK missing (SDAR-4); tenant composite present | attach tx | AND evaluation | — | — | — | service-asserted epochs |
| Target immutability | active tuple mutation | **pointer mutable** (SDAR-1) | approval tx | refuse mutation | — | — | — | pointer swap by buggy service |
| Snapshot immutability | content mutation | app role no UPDATE/DELETE (proposed) | insert-only writes | no mutation path | — | — | — | admin role; compromised service (TCB) |
| Approval | approval without snapshot; agent self-approval | active⇒snapshot CHECK undeclared (SDAR-9); no approver-type CHECK | one ExecuteTx | role/policy checks (I-A4 service) | IdP type assertion | approver set | — | deceived approver (out of scope) |
| Principal revocation | revoked principal approves/discharges anew | none (cross-table) | revocation tx | refuse on fresh state | IdP signal | — | — | race closed by serial order |
| Tenant binding | cross-tenant edge | composite FKs **missing on principal/policy/snapshot edges** (SDAR-7) | — | tenant authz | tenant assertion | — | — | single-column FKs as written |
| Obligation identity | v1 discharge satisfies v2 obligation | FK present but **consumed by nothing** (SDAR-6) | discharge tx | qualification (future) | — | policy binding | — | transitional dual truth |
| Discharge uniqueness | same instrument, same obligation, twice | partial UNIQUE (proposed; SDAR-5 caveat) | discharge tx + array_remove one tx | instrument verification | — | — | — | identity minting (SDAR-16) |
| Policy ordering | revision regression | `UNIQUE (tenant_id, revision)` (proposed) | serialized allocation | reject stale | — | — | — | allocation mechanism unspecified |
| Policy downgrade | broadening of approved authority | append-only policy + snapshot binding | — | re-verification | — | no-broaden rule | — | SDAR-19 discretion |
| Migration fail-closed | legacy row → inferred authority | NOT NULLs (proposed) | — | refuse missing dimensions | — | — | — | mixed-path window (SDAR-18) |
| Warrant replay | warrant as second authority | no warrant table (correct) | — | reference re-verification | — | — | — | none in MVP |
| Tuple equality (I-12) | presented ≠ authorized executes | — | — | exact equality pre-execution | — | — | echo verbatim | executor contract (DOCUMENTED CONTRACT) |
| Execution effect (I-13) | presented ≠ world effect | — | — | — | — | — | — | FUTURE / UNSCOPED — no claim made (correct) |

### Mandatory-findings checklist (the 24 required determinations)

| # | Item | Safe as written? | Ref |
|---|---|---|---|
| 1 | target_snapshot sole authority | Semantics yes; anchoring no | SDAR-3 |
| 2 | target_snapshot immutability | Content yes; selection no | SDAR-1 |
| 3 | authority_target ↔ target_snapshot binding | No | SDAR-1, SDAR-3 |
| 4 | promotion_epoch correctness | No | SDAR-4 |
| 5 | justification ↔ promotion_epoch binding | No | SDAR-4 |
| 6 | retract → re-promote behavior | Direction yes; mechanism no | SDAR-4, SDAR-19 |
| 7 | discharge instrument uniqueness | Rule yes; lifecycle hole | SDAR-5, SDAR-16 |
| 8 | obligation identity + policy version | Representation yes; enforcement no | SDAR-6 |
| 9 | policy monotonic revision | Yes, conditions | SDAR-14 |
| 10 | policy downgrade semantics | Yes, pin SDAR-19 | SDAR-19 |
| 11 | principal stable identity | Yes | — |
| 12 | principal type lifecycle | Yes | SDAR-17 |
| 13 | principal revocation | Yes, service-side | SDAR-13 |
| 14 | attestation validity after revocation | Yes, conditions | SDAR-13 |
| 15 | belief_tenant one-to-one semantics | Yes | SDAR-10 |
| 16 | composite tenant FKs | No | SDAR-7 |
| 17 | migration fail-closed | Yes | SDAR-18 |
| 18 | absence of wildcard defaults | Mostly; empty-value probes open | SDAR-12 |
| 19 | absence of second authority source | Blob + pointer issues | SDAR-1, SDAR-17, audit below |
| 20 | executor mediation boundary | Yes (DOCUMENTED CONTRACT) | — |
| 21 | absence of execution-effect claims | Yes | — |
| 22 | evidence provenance boundary | Yes | — |
| 23 | concurrency semantics | Partially | SDAR-8 |
| 24 | DocTrust testability | Partially | SDAR-19 |

---

## Impossible-State Verdict

The §7 table is a good skeleton with three rows that fail verification:

- **"active target + mutable authority tuple"** — claimed prevented by "snapshot has no app UPDATE/DELETE." The tuple moved; the pointer did not. State is reachable (SDAR-1). Row must be rewritten after the SDAR-1 repair.
- **"active target + changed justification set"** — "execution reads snapshot only" is true, but the snapshot's set itself is unverifiable against any relational anchor (see audit below) and can be enlarged post-review by the retry race (SDAR-8). Reachable in the sense that matters: the *effective* set differs from the reviewed set.
- **"v1 discharge satisfies semantically different v2 obligation"** — claimed DB-prevented by the policy FK; the FK is on a row no enforcement path reads (SDAR-6). Reachable in the transitional model.

The remaining rows verify: the two inherited invariants (CURRENT, verified in `db/001_schema.sql`), cross-tenant justification (blocked through `belief_tenant` composite FK — this one edge is properly declared), discharge-without-debt-removal (single-tx rule), duplicate instrument (partial UNIQUE, with SDAR-5's revocation caveat), policy regression (UNIQUE), post-revocation attestation (service rule, honestly labeled after SDAR-13's correction), and the three NULL-wildcard rows (NOT NULLs are declared — extend per SDAR-12 to empty values).

Residuals claimed as "none" in two rows ("none for this invariant", "none under current model") are correct for the inherited invariants.

---

## Self-Attack Matrix

**The mandated attack chain (legitimate principal → promoted belief "customer identity was verified" → legitimate discharge → target "transfer treasury funds" → attach → valid policy → request → human approval → ALLOW → transfer):** every step is legitimate; the chain *succeeds*, and the architecture permits it — the authority was granted by an authorized human approver under a valid policy. The design correctly does not pretend otherwise (the deceived-but-legitimate approver is documented residual in the corpus). Strengthening the system layer by layer:

1. **Promotion epoch:** does not impede the chain (all states genuine) — but as written it also does not impede an attacker *fabricating* epochs (SDAR-4). After the `belief_promotion` repair: impotent against the honest chain, correctly fatal to epoch forgery.
2. **Immutable snapshot:** the chain's snapshot is honest. But the immutability mechanism as written does not stop the *pointer* swap — so a buggy revoke path, not the attacker, can hand the attacker's later target the earlier approval's authority (SDAR-1). This is the point where the design fails a test it set for itself.
3. **Policy revision:** the attacker's "valid customer policy" is genuine; revision ordering cannot help. The policy floor (HIGH_IMPACT ⇒ attested/quorum) is what should challenge step 6 — and it is DOCUMENTED CONTRACT / service-enforced only, correctly not claimed as schema.
4. **Tenant binding:** if the attacker stays in-tenant, no help — correctly so. Cross-tenant, the `justification → belief_tenant` edge holds but the principal/policy edges do not (SDAR-7): a cross-tenant principal *can* be attached to the chain's discharge or snapshot as written.
5. **Instrument uniqueness:** the chain's discharge is single-use; replay of it onto a second belief is blocked by the partial UNIQUE — except through the revocation window (SDAR-5).
6. **Revocation:** forward-looking; blocks the attacker's *future* approvals; does not impede the completed chain — by design (C-7).
7. **Exact tuple matching (I-12):** the executor presents what it was authorized to present; ALLOW is faithfully echoed. I-13 unclaimed — correct.

**Conclusion of the chain test:** the honest chain succeeds by design and the design says so. The manufactured-authority paths the architecture forbids are: pointer swap (SDAR-1 — design claims prevention, fails), epoch forgery (SDAR-4 — design claims prevention, fails), cross-tenant principal/policy attachment (SDAR-7 — design claims prevention, fails), replay-through-revocation (SDAR-5 — design sketch creates it), and unreviewed-set enlargement (SDAR-8). Each is precisely identified above; each is repairable without reopening the architecture.

---

## Second-Source-of-Truth Audit

| Duplicated fact | Authoritative copy | Historical/derived copy | Divergence impossible? | Verdict |
|---|---|---|---|---|
| Justification set: `justification` rows vs `justification_set` blob | blob (execution-time) | live rows (audit) | **No** — DB cannot compare blob to rows; the design declares this ("historical snapshot, not a second live authorization graph"), which is the right *label*, but nothing detects divergence between what was attached and what was snapshotted within the approval tx | Accepted-by-design; detection gap folded into SDAR-8's approval-contract requirement |
| Approver type: `principal.principal_type` vs `snapshot.approver_principal_type` | principal row (immutable) | snapshot copy | Divergence impossible *because* the source is immutable — which also makes the copy pure redundancy | SDAR-17: remove |
| Epoch: promotion occurrence (once `belief_promotion` lands) vs epochs inside the blob vs `justification.promotion_epoch` | `belief_promotion` row (after SDAR-4 repair) | justification FK value; blob copy | Blob copy unverifiable; justification value FK-verifiable after repair | Conditional |
| Debt: `belief.debt[]` vs `debt_discharge` ledger | array (promotion gate) | ledger (attribution/replay) | Single-tx rule makes *presence* divergence impossible; *meaning* divergence (version-blindness) unhandled | SDAR-6 |
| Belief status: `belief.status` vs `action_intent.belief_status` | belief.status | gate-cached copy, `ON UPDATE CASCADE` | Yes — CURRENT, verified constraint behavior | OK (inherited) |
| Target state vs snapshot existence | `authority_target.state` | — | No CHECK declared (`state='active' ⇒ snapshot_id NOT NULL` is prose) | SDAR-9 |
| Policy: `policy_content` vs `policy_hash` | content | hash | Canonicalization unspecified | SDAR-21 |
| Five-tuple: proposal location (unresolved, SDAR-2) vs snapshot | snapshot (after approval) | proposal columns (if added) | Requires SDAR-2's copy-then-freeze contract | Conditional |

No unresolved dual truth was found *outside* the items above. The design's declared single authority source (snapshot) is consistent across §8/§13/§19 — the consistency of the *prose* is not in question; the anchoring is.

---

## DDL Readiness Gate

## NOT SAFE TO BEGIN DDL

The design's own self-verdict ("NOT READY FOR DDL", §21) is **correct**, and this review sharpens why: the seven conditions as listed in §21 are framed as implementation mechanics ("finalize," "confirm," "validate"), but three of them are actually unresolved *semantic-structural* decisions (SDAR-1/2/3), and two of the seven claimed resolutions fail their own exit tests (C-2 → SDAR-4; C-4 → SDAR-6). The safety condition in §21 ("DDL must not be generated if any of the seven conditions is resolved only by a prose promise that has no enforceable schema/service boundary") — applied honestly — already prohibits DDL of this document as written, since C-1's mechanism and C-2's binding are exactly such prose promises.

---

## Required Conditions Before DDL

Blocking only; seven conditions covering the unique nine-finding blocking set (see BLOCKER-CROSS-TAB). Each names its finding and its exit test.

1. **Resolve the authority-mutation surface (SDAR-1, SDAR-9).** Choose and specify the transition structure (activation record / state-folded-into-snapshot / equivalent) such that no application-role UPDATE can change which snapshot an active target resolves to or resurrect a revoked target. Exit test: the direct SQL attacks in SDAR-1/SDAR-9 are structurally impossible.
2. **Give the proposal a representation (SDAR-2).** Specify where the candidate tuple lives from CreateTarget to Approve, which fields are mutable before approval, and prove the approval copy is sole authority thereafter. Exit test: the four-operation lifecycle is durably expressible; post-approval proposal mutation changes no decision.
3. **Declare the full key graph (SDAR-3, SDAR-7).** `target_snapshot` PK/FKs (target, principal, approver, policy — all tenant-composite), `UNIQUE (tenant_id, principal_id)` on `principal`, composite FKs on every principal/policy edge. Exit test: each cross-tenant/orphan insert in SDAR-3/SDAR-7 is rejected by a named constraint.
4. **Give the epoch a real referent (SDAR-4).** Implement the locked `belief_promotion` side-table direction (`UNIQUE(belief_id, promotion_epoch)`, FK to `belief`, frozen core preserved; the additive-column route only with explicit approved frozen-core exception); evidence-gated, fail-closed backfill; monotonicity CHECK. Exit test: epoch forgery rejected by constraint; backfill value stated.
5. **Close the discharge revocation path (SDAR-5).** Accepted rows immutable; revocation declared FUTURE / UNSCOPED for MVP (or, if required later, an insert-only counter-entry); final status domain with no speculative values. Exit test: no accept→revoke→re-accept sequence exists.
6. **Specify the approval transaction contract (SDAR-8).** Precondition reads (belief status + committed promotion occurrence, principal state, tenant, policy validity), AND evaluation point, set-pinning via RequestAuthorization with durable pin state, retry behavior that cannot silently enlarge the approved set. Exit test: the SDAR-8 interleaving ends in refusal or re-approval.
7. **Make C-4's claim true or relabel it (SDAR-6).** Consume the policy binding in the discharge transaction, or relabel transitional enforcement as service-level audit — and correct the §7/§19 rows that flow from this and from the other repairs (the attestation-row correction of SDAR-13 is sequenced, not blocking). Exit test: every §7/§19 row names the enforcement layer that actually consumes the binding, and a test demonstrates a v1 discharge against a v2-redefined obligation is either blocked or explicitly documented as an accepted transitional residual.

Non-blocking but sequenced before their consumers: SDAR-10 (belief_tenant FK + mapping authority), SDAR-11 (requester attribution), SDAR-12 (empty-value CHECK decision), SDAR-13 (C-7 boundary commitment), SDAR-14 (allocation mechanism), SDAR-15 (justification natural key), SDAR-16 (instrument identity minting), SDAR-17 (approver-type removal), SDAR-18 (backfill/window), SDAR-19 (retraction re-verification rule), SDAR-20/21 (spec tightening).

---

## Dogfood Evidence Ledger

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Only earned invariants are the four SQL constraints + I-7/I-8 | `db/001_schema.sql` constraints read verbatim; `kernel/` transaction discipline | High | — | CURRENT / VERIFIED |
| Design's semantic corpus (belief ≠ authority, snapshot-authoritative, epoch, instrument replay, revision ordering, belief_tenant, issuance-bounded attestation) survives attack | Findings SDAR-1..21 attack and confirm direction | High | structural repairs | PROPOSED |
| `target_snapshot` is the sole execution-time authority | §8/§13 prose consistent; anchoring absent | Medium | SDAR-1/3 | PROPOSED (contested anchoring) |
| Snapshot immutability is structurally enforced | Privilege prose guards content, not pointer | Low | SDAR-1 | CONTRADICTED (as claimed) |
| Justification epoch binding is DB-enforced | §5.4 "conceptually bound"; no referent exists | Low | SDAR-4 | CONTRADICTED (as claimed in §19) |
| Cross-tenant relationships are structurally impossible | One edge declared; principal/policy edges single-column | Low | SDAR-7 | CONTRADICTED (as claimed in §19) |
| v1 discharge cannot satisfy v2 obligation | Binding recorded, consumed by nothing | Low | SDAR-6 | CONTRADICTED (as claimed in §7/§19) |
| Discharge replay is prevented | Partial UNIQUE sound; revocation path reopens it | Medium | SDAR-5, SDAR-16 | PROPOSED |
| Policy revision ordering is deterministic | `UNIQUE (tenant_id, revision)`; allocation unspecified | Medium | SDAR-14 | PROPOSED |
| belief_tenant 1:1 mapping is structural | `UNIQUE(belief_id)`; FK-to-belief missing | Medium | SDAR-10 | PROPOSED |
| Migration is fail-closed | §16 rules complete and correct | High | SDAR-18 window | PROPOSED |
| I-12/I-13/Executor Mediation boundaries preserved | §19 verbatim; no execution claims anywhere | High | — | DOCUMENTED CONTRACT |
| Concurrency is fully specified | Matrix directionally right; two rows hide races; approval contract absent | Medium | SDAR-8 | OPEN |
| DocTrust can deterministically validate the model | 10 of 11 flows deterministic after repairs; flow 7 needs SDAR-19 | Medium | SDAR-19 | OPEN |
| Reasoning ≠ evidence: coherence of §1–§23 proves nothing | This review found 3 CRITICAL defects in a coherent document | — | — | — |

---

## Final Dogfood Test

Does the schema design obey Solvent's own methodology?

**Partially — and the failure mode is instructive.** The design's method section (derive from durable facts; the DB, not the LLM, decides; "the application proposes; the database decides") is applied faithfully at the *semantic* layer: every C-decision is derived, every authority claim is labeled, every deferred concept has a promotion condition, the invariant discipline is spotless, and the boundary honesty (executor, evidence, compromised service) is better than most of the corpus. Where it fails its own standard is the layer Solvent exists to defend: **three authority-critical properties are narrated as DB-enforced in §7/§19 while the relational machinery that would enforce them is absent from the object definitions** — the exact "receipt that cannot be traced to a specific row is decoration" failure the repository's own rules name, committed in structural form. A schema design that says "the DB prevents it" where the DB as specified does not, is doing to its reader what Solvent refuses to do to its agents.

The design is close. Its seven decisions are the right seven; five of the seven resolutions are directionally correct; the repairs are enumerable and do not reopen the architecture. It should be repaired against the seven blocking conditions above and resubmitted — the next artifact after that repair is the DDL-gate re-review, not another general architecture pass.

---

## Validation

Validation was executed for this revision. Results embedded below.

### BLOCKER-CROSS-TAB (mandatory — executed)

1. **Blocking SDAR IDs extracted exactly once:** from the finding list, every finding marked `DDL-blocking? YES`: SDAR-1, SDAR-2, SDAR-3, SDAR-4, SDAR-5, SDAR-6, SDAR-7, SDAR-8, SDAR-9. **Unique set size = 9.** Non-blocking: SDAR-10..21.
2. **"Required Conditions Before DDL" items extracted:** conditions 1–7 (Revision 2).
3. **Normalized to underlying SDAR IDs:**
   - Condition 1 → {SDAR-1, SDAR-9}
   - Condition 2 → {SDAR-2}
   - Condition 3 → {SDAR-3, SDAR-7}
   - Condition 4 → {SDAR-4}
   - Condition 5 → {SDAR-5}
   - Condition 6 → {SDAR-8}
   - Condition 7 → {SDAR-6}
4. **Set equality:** set(blocking SDARs) = {1,2,3,4,5,6,7,8,9}; set(SDARs represented by conditions) = {1,2,3,4,5,6,7,8,9}. **EQUAL — PASS.**
5. **No SDAR appears twice as an independent blocking condition:** Revision 1 contained condition 8 ("correct §7/§19 claims", citing SDAR-6 + SDAR-13) — SDAR-6 was already condition 7 (a double count) and SDAR-13 is non-blocking. Revision 2 removes that condition; the §7/§19 row corrections flowing from SDAR-6 are absorbed into condition 7, and the SDAR-13 attestation-row correction moves to the non-blocking sequenced list. **PASS.**
6. **Headline blocker count equals the unique set size:** the Executive Verdict and DDL Readiness Gate report **nine unique blocking findings** in **seven conditions**; the broad §7/§19 consistency sweep is a consequence of conditions 1–7, not an additional blocker. **PASS.**

### TAXONOMY / ID CHECK (executed)

- Every SDAR-N identifier is unique: SDAR-1..SDAR-21, no duplicates or gaps in references. **PASS.**
- Every DDL-blocking SDAR-N is counted once: verified by the BLOCKER-CROSS-TAB above. **PASS.**
- No proposed property is presented as an earned I-* invariant: this review creates no new I-* identifiers and cites only `promoted_is_debt_free`, `gate`, `live_requires_promoted`, `belief_id_status_key`, I-7, I-8 as repository-earned. **PASS.**
- I-12 remains a PROPOSED source-spec label: stated in Status, Executor Boundary Verdict, and Security Property Matrix. **PASS.**
- I-13 remains FUTURE / UNSCOPED: stated in the same locations. **PASS.**
- DOCUMENTED CONTRACT is not used as evidence of implementation: Executor Mediation appears only as a deployment precondition; the policy floor (I-P1/I-P2) is explicitly NOT claimed as schema enforcement. **PASS.**
- CURRENT / VERIFIED is reserved for repository-backed facts: used only in the Repository Truth section and the dogfood rows citing `db/001_schema.sql` / `kernel/`. All new-schema claims are PROPOSED. **PASS.**

### Other validation

1. Schema design modified? **No** — `docs/ARCHITECTURE/schema-design.md` untouched.
2. DDL created? **No.**
3. SQL changed? **No.**
4. Go changed? **No.**
5. Lean changed? **No.**
6. MCP changed? **No.**
7. REST changed? **No.**
8. ADR changed? **No.**
9. Prior review artifact (`SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW.md`) modified? **No** — this revision is a new file; Revision 1 is preserved verbatim.
10. `git status --short` executed post-write; output recorded in the delivery message; the only artifact added by this revision is this file.
11. No commit. No sync.
