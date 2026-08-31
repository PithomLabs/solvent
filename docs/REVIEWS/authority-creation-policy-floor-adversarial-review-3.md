# Authority Creation and Policy Floor —
# Adversarial Review #3

## Review posture

The specification under review (`docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`) is treated as a belief that must earn promotion. It is **PROPOSED — NOT IMPLEMENTED**. Nothing in this review modifies Go, SQL, Lean, MCP, REST, ADRs, or the specification itself. The repository is the source of truth; where the specification and repository disagree, repository wins.

I am NEW to this project. I do not rely on prior ChatGPT conversations or prior agent conclusions as truth. Every claim is verified against repository truth.

---

## Executive verdict

**REJECT / REDESIGN**

The C-1/C-3 specification correctly identifies two real problems — (1) the four-role decomposition for authority creation and (2) the minimum-validity floor for customer-defined policy — and proposes reasonable service-layer rules for each. However, the specification is **not safe enough to authorize schema or kernel work** in its current form.

The fatal flaw is structural: **every security boundary the specification proposes (I-A1 through I-A4, I-P1 through I-P4) is enforced at the service layer, with zero database backing.** There is no `principal` column anywhere in the current schema (`db/001_schema.sql:60-76` has no principal column; `action_intent` carries only `belief_id, belief_status, action, state`). There is no `AuthorityTarget` table. There is no `DebtDischarge` table. There is no `policy_version` table. There is no `obligation` table. There is no `justification` link table. There is no bootstrap provisioning mechanism.

Because the database enforces none of the C-1/C-3 boundaries, an attacker who compromises, bypasses, or simply calls the service layer with forged inputs can manufacture authority without crossing any database-enforced boundary. The specification's own most-important attack scenario fails only at a service-layer contract (`I-A4: approver must be human/non-agent`) that has no database enforcement and no identity binding. A bug, a misconfiguration, or a compromised service layer makes that contract disappear.

The specification is good design prose. It is not an implementation-ready security boundary. Before any `AuthorityTarget`, `DebtDischarge`, `policy_version`, or `principal` schema is written, the specification must be redesigned to make each boundary **impossible to violate in a committed database state**, not merely "checked by the service."

---

## What Review #2 / the C-1/C-3 specification actually closed

### From Review #2 (`docs/REVIEWS/AUTHORITY_SPEC_ADVERSARIAL_REVIEW_2.md`)

Review #2 identified four conditions (C-1 through C-4) that must be resolved before kernel/schema work:

| Review #2 Condition | What the C-1/C-3 spec claims to do | Actual disposition |
|---|---|---|
| C-1: Four-role decomposition for target/link creation | Proposes a four-role matrix and four invariants (I-A1..I-A4) | **Moved, not closed.** The matrix is prose. I-A1..I-A4 are all service-enforced. No DB columns, constraints, or tables exist to back them. |
| C-2: N2 discharge-array consistency | Acknowledged as service-enforced in the transitional model | **Still open.** The C-1/C-3 spec does not address N2 at all. The transitional `debt[] + discharge ledger` dual-truth remains service-enforced only. |
| C-3: Minimum-validity policy contract | Proposes universal negatives (I-P1, I-P2) and a candidate HIGH_IMPACT floor (I-P3) | **Partially closed, partially open.** I-P1 and I-P2 are named as service-enforced rules. I-P3 (HIGH_IMPACT floor) remains OPEN. No DB backing exists for any of them. |
| C-4: Executor integration contract | Not addressed | **Still open.** The C-1/C-3 spec does not produce an executor contract document. |

### What the C-1/C-3 spec adds

The spec adds:
- A four-role matrix with precise actor/precondition/enforcement columns
- Universal negatives for policy (no empty obligations, no self-satisfying, no "any evidence qualifies", deterministic ≠ unverified external_feed)
- A candidate HIGH_IMPACT floor (I-P3, still OPEN)
- Policy versioning semantics (I-P4)
- Self-satisfaction/circularity rules
- Target immutability rules
- Bootstrap trust root description

### What the C-1/C-3 spec does NOT add

The spec does NOT add:
- Any database schema for `AuthorityTarget`, `DebtDischarge`, `policy_version`, `obligation`, `justification`, `principal`, `warrant`
- Any database constraint that enforces I-A1..I-A4 or I-P1..I-P4
- Any identity binding mechanism
- Any provisioning mechanism for the bootstrap root
- Any executor contract document
- Any resolution of N2 discharge-array consistency

### Verdict on "what was closed"

| Category | Closed | Moved | Still open | Newly discovered |
|---|---|---|---|---|
| Four-role model | | X (prose only) | X (no DB enforcement) | |
| Policy floor universal negatives | | X (service-enforced) | X (no DB backing) | |
| HIGH_IMPACT candidate floor | | | X (I-P3 OPEN) | |
| Bootstrap root | | | X (no mechanism) | |
| N2 consistency | | | X (not addressed) | |
| Executor contract | | | X (not addressed) | |
| Principal/identity binding | | | | X (critical missing piece) |
| Target immutability enforcement | | | | X (promised, not enforceable) |
| Policy version binding | | | | X (no table exists) |

**Net assessment: The C-1/C-3 specification adds useful prose but closes zero security boundaries at the database level. Review #2's four conditions remain open.**

---

## Critical findings

### F-3-1 — CRITICAL — I-A4 (ApprovalSeparation) is a service-layer promise with no database enforcement

- **ID:** F-3-1
- **Severity:** CRITICAL
- **Claim attacked:** "`Approve` requires an authorized human or non-agent principal only — never the requesting agent by default" (`## C-1 Approval / Activation Rules`)
- **Repository/spec evidence:** `db/001_schema.sql:60-76` — `action_intent` has no `principal`, `principal_type`, `approver`, or `approved_by` column. `kernel/kernel.go:107-111` — `IntentOnPromoted` checks only `(belief_id, 'promoted')` via composite FK. The C-1/C-3 spec proposes `approver_principal_type/id` as an audit column in the approval record (`## C-1 Approval / Activation Rules`) but acknowledges "future invariant `I-A4` makes `active` without valid approval impossible, but `active` column does not yet exist so service is the gate."
- **Attack path:**
  1. Attacker controls an AI agent with ordinary tenant-level access.
  2. The current codebase has no `principal` column, no `principal_type`, no identity binding, and no tenant authentication mechanism. The MCP server reads `FABLE_DSN` from the environment and exposes tools with no caller identity check (`cmd/solvent-mcp/main.go:39-44`).
  3. Attacker creates a promoted belief (legitimate or manufactured).
  4. Attacker calls `CreateTarget` for `HIGH_IMPACT` consequence `funds_transferred` on `treasury-account`.
  5. Attacker attaches the belief as justification.
  6. Attacker calls `Approve`. In the proposed model, `Approve` is a service-layer function. The service must check `approver_principal_type != 'agent'` for `HIGH_IMPACT`.
  7. **The service has no database mechanism to verify the caller's `principal_type`.** The caller's identity is whatever the HTTP header / MCP metadata / request context says it is. A compromised service, a buggy identity adapter, or a caller that forges its own `principal_type=human` header passes the check.
  8. `Approve` succeeds. Target becomes `active`. Warrant is issued. `ALLOW` is returned.
  9. High-impact action occurs.
- **Exact failure boundary:** The transition `proposed → active` in the service layer. The database has no `active` column, no `approved_by` FK, no `principal_type` CHECK, and no constraint that would refuse the write. The service is the sole gate, and the service's identity input is unverifiable at the database level.
- **Why the current design fails:** The specification correctly states that "an AI agent cannot bootstrap the authority system" and that "`Approve` requires a human/non-agent principal," but it implements neither rule as a database constraint. The distinction between "agent" and "human/non-agent" exists only in the service layer's interpretation of an external identity token. If the service layer is wrong, compromised, or bypassed, the boundary vanishes. A specification that places the entire authority-granting transition behind a service-layer identity check with no database backing has not made the transition "impossible" — it has made it "contingent on correct service behavior."
- **Recommended resolution:** Before any schema work, the specification must define a database-enforceable mechanism for I-A4. At minimum: (1) an `approver_principal_type` column on the approval record with a `CHECK (approver_principal_type IN ('human','workload','service'))` that explicitly excludes `agent`; (2) a `principal` reference table with `principal_type` that is FK-bound to the approval record; (3) an identity-binding mechanism that ensures the `principal_type` asserted by the caller matches the identity token verified by an external IdP. Until these exist, `I-A4` is a service-layer aspiration, not a security boundary.
- **Required before kernel/schema? YES.** Without a `principal` column and a `principal_type` CHECK, no schema can enforce "agent cannot approve."

---

## C-1 Four-Role Verdict

| Operation | Who | Preconditions | Existing authority | Enforcement | Security concern |
|---|---|---|---|---|---|
| **CreateTarget** | Agent or any authenticated principal (human, workload, service account) — proposing only | Caller is authenticated to the tenant; `resource`/`consequence` are syntactically valid | **None** beyond `authenticated to tenant` | **Service** — validates principal identity, validates well-formedness | **HIGH:** No `principal` column exists. No tenant authentication mechanism exists. "Authenticated to the tenant" is a wish. |
| **AttachJustification** | Agent (subject to target's `discharge_policy`) or human/system | Target is `proposed`; belief exists and is not retracted; `obligation_key` is in belief's current obligation set | **None** beyond `CreateTarget` + `Belief exists` | **Service** — validates belief exists in same tenant/scenario, validates obligation_key, checks instrument_kind | **HIGH:** No `justification` table exists. No `obligation` table exists. "Subject to discharge_policy" is a service check against a policy that does not exist. |
| **RequestAuthorization** | Agent or any authenticated principal | Target exists; all required justification links exist in `proposed` | **None** beyond the above — requesting is not approving | **Service** — collects AND set, checks beliefs are promoted | **MEDIUM:** No `authorization_request` table exists. The "AND set" check is service logic with no database backing. |
| **Approve / Activate** | Authorized human or non-agent principal only — never the requesting agent by default | Target is `proposed`; every required belief is `promoted`; every required obligation has an accepted `DebtDischarge`; no open contradiction; tenant policy permits approval | **Explicit approval authority** scoped to `target.resource`/`consequence` | **Service + future DB** — service verifies atomically, transitions `proposed → active`; future `I-A4` makes `active` without valid approval impossible | **CRITICAL:** No `active` column exists. No `approved_by` FK exists. No `principal_type` CHECK exists. The entire authority-granting transition is a service-layer function with no database enforcement. |

### Is the four-role model actually sufficient?

**No.** The four-role model is directionally correct — separating creation from attachment from request from approval is the right architecture. But the model is **not sufficient** for production because:

1. **None of the four roles are enforced at the database level.** Every enforcement is service-layer only. A bug, misconfiguration, or compromise in the service layer makes every role's preconditions disappear.

2. **The authority-granting transition (`Approve`) has the weakest enforcement.** `CreateTarget` at least produces a `proposed` row that is not yet `active`. `Approve` transitions a row from `proposed` to `active` — but there is no `active` column, no `approved_by` FK, and no CHECK that would prevent an unauthorized caller from making the transition.

3. **The "agent cannot approve" rule is unenforceable.** The entire C-1 model rests on the distinction between agent and human/non-agent. But there is no `principal_type` column, no `principal` table, and no identity binding. The rule exists only as a service-layer interpretation of an external IdP token.

4. **The bootstrap recursion is unsolved.** The spec says "human-only tenant-admin bootstrap" but provides no provisioning mechanism. The first `AuthorityTarget` cannot be created without a bootstrap admin, but the bootstrap admin cannot be created through the C-1 model (because the C-1 model requires an existing approver to create the first approver). The spec hand-waves this with "deployment/provisioning configuration" but does not define the configuration.

**The four-role model is a workflow diagram, not a security boundary. It becomes a security boundary only when accompanied by database schema that makes each role's preconditions impossible to violate.**

---

## Bootstrap Trust Verdict

### What is the minimum trustworthy root?

The specification correctly identifies the recursion: "authority is needed to create authority, but no authority exists yet." Its proposed solution is "human-only tenant-admin bootstrap via deployment/provisioning configuration."

**This is the right root in principle but the wrong root in practice, because the mechanism is unspecified.**

The minimum trustworthy root must be:

1. **A concrete, auditable provisioning mechanism.** Not "deployment config" generically, but a specific artifact: a signed manifest, an IdP group membership, a hardware token, or a smart contract. The mechanism must be implementable and auditable.

2. **An identity layer that provides `principal_type`.** Solvent cannot enforce "human vs agent" without a trustworthy `principal_type` assertion. The identity layer must provide this attribute in a way that Solvent can verify without re-implementing identity.

3. **A bootstrap admin revocation mechanism.** Bootstrap admins are "small, enumerable, and revocable" per the spec — but there is no revocation mechanism. If a bootstrap admin is compromised, there is no way to remove their authority.

4. **An explicit acknowledgment that bootstrap security is an IAM problem, not a Solvent problem.** The spec says this in one sentence ("A compromised bootstrap admin is a tenant-compromise, not a Solvent bypass") but builds the entire C-1 model on the assumption that the bootstrap root is trustworthy. If the bootstrap root is compromised, every subsequent `Approve` is compromised. Solvent cannot fix this; it can only document it.

### What must remain outside Solvent?

The following must remain outside Solvent:

- **Identity verification.** Solvent consumes `principal_type` and `principal_id`; it does not verify them. The IdP, workload attestation service, or enterprise identity system is responsible for verifying that a `principal_type=human` token actually belongs to a human.

- **Bootstrap provisioning.** The mechanism that establishes the first human tenant administrator is outside Solvent. Solvent can require that bootstrap admins exist and that they are `principal_type=human`, but Solvent cannot create or verify the bootstrap admins.

- **Attestation key management.** The spec correctly states that "Decision 3 does not eliminate PKI/key-management concerns; it bounds them to the attestation boundary." Key rotation, revocation, and storage are outside Solvent.

- **Executor behavior.** Solvent guarantees the executor *was authorized* (I-12). It does not guarantee what the executor *actually did* (I-13). The executor's actual behavior is outside Solvent's boundary.

---

## Policy Floor Verdict

### Does Solvent now have a meaningful minimum safety floor?

**No.** The C-1/C-3 specification proposes a policy floor but does not implement it. The floor exists as prose, not as enforceable rules.

### Universal negatives (I-P1, I-P2)

The spec claims these are "locked, not OPEN" and "never customer-overridable." But:

- **I-P1 (no empty obligation sets, no self-satisfying obligations, no "any evidence qualifies")** is a service-layer rule with no database enforcement and no implementation in the current codebase. The current codebase has no `obligation` table, no `discharge_policy` column, and no policy validation service. Claiming this rule is "locked" while the enforcement mechanism does not exist is a contradiction.

- **I-P2 (deterministic ≠ unverified external_feed, attested ≠ agent-generated)** is violated by current code (`kernel/kernel.go:73-78` accepts caller-supplied hashes for `external_feed`). The spec acknowledges this as "deferred" in the self-attack results but claims the rule is "locked" in the invariants section. This is an internal contradiction.

**The universal negatives are good rules. They are not currently enforced. They should be classified as "candidate rules (OPEN)" until a policy validation service exists.**

### Candidate rules (I-P3)

- **I-P3 (HIGH_IMPACT ⇒ ≥1 attested or quorum)** is explicitly OPEN. The spec says it will be "flagged as `predicted to be weaker than candidate floor`" but not enforced. This means `HIGH_IMPACT` consequences can currently be authorized with only deterministic obligations, which the candidate floor says is too weak.

**The candidate floor is a goal, not a floor. It should be either promoted to a locked rule with enforcement or explicitly documented as a known limitation.**

### Customer-configurable rules

The spec correctly identifies that policy is customer-configurable and that customer policy can define obligation sets, discharge policies, and risk-tier mappings. But it does not define:

- The minimum `obligation` schema
- The minimum `discharge_policy` schema
- The minimum `consequence_type` enumeration
- The mechanism by which Solvent validates customer policy against the universal negatives

**Without these definitions, "customer-configurable" means "customer can configure anything," including configurations that make Solvent semantically meaningless.**

### Forbidden rules

The spec does not define any forbidden configurations. It says "no empty obligation sets" and "no self-satisfying obligations" but does not define what happens if a customer configures them. The current codebase has no policy validation service to refuse them.

**A policy floor without enforcement is a policy aspiration.**

---

## Policy Change Verdict

### Downgrade

The spec says: "A customer that downgrades after `Approve` does not get stronger targets for free — each new target after downgrade is still `active` only if it meets the weaker floor, but the downgrade itself is an `Approve`-gated event."

**This is correct but unenforceable.** Without a `policy_version` table, there is no mechanism to:
- Record that a downgrade occurred
- Verify that a new target was created under the new policy
- Prevent a downgrade from immediately benefiting new targets
- Audit the delta between old and new policy

The spec says downgrade is "attributable" — but without a `policy_version` table, the attribution is a hash string with no verifiable referent.

### Upgrade

The spec says: "A policy upgrade (stricter floor) does not invalidate `Belief.promoted`, but it does make already-active targets that would not meet the stricter floor fail open at next verification."

**This is correct but unimplementable.** Without a warrant re-verification mechanism and a `policy_version` table, there is no way to trigger re-verification when policy changes. The spec says "verification must run at execution time" but does not define how.

### Policy versioning

The spec says policy needs "version, effective_at, immutable historical snapshot, policy hash, author identity." This is correct. But:

- No `policy_version` table is proposed
- No DDL is provided
- No migration path is defined
- No `effective_at` monotonicity constraint is proposed

**Policy versioning is a prerequisite for every other C-3 rule. Without it, I-P4 cannot be enforced, and without I-P4, policy change semantics are undocumented.**

### Already-active target

The spec says already-active targets remain active until reassessment. This is correct. But "reassessment" is an operator-triggered event (`RetractCascade`-style) with no background or push mechanism. A policy upgrade does not trigger reassessment. A policy downgrade does not invalidate existing targets.

**The spec correctly identifies this gap but does not propose a solution.**

### New target

The spec says new targets are evaluated under current policy. This is correct. But without a `policy_version` table, "current policy" is a mutable config row that can be changed between target creation and approval.

### Warrant verification

The spec says warrants are re-verified at execution time. But without a `policy_version` table and a warrant re-verification mechanism, this is a promise, not a guarantee.

---

## Principal / Identity Verdict

### Is `principal_type/id` sufficient?

**No.** The C-1/C-3 specification proposes `principal_type, principal_id` as the authority identity model. But:

1. **No `principal` table exists.** The current codebase has no principal concept. `action_intent` has no `principal` column. `belief` has no `principal` column. `evidence` has no `principal` column.

2. **No `principal_type` column exists.** The spec distinguishes `agent`, `human`, `workload`, `service account`, and `delegated principal` — but there is no column to store this distinction.

3. **No identity binding exists.** The spec says "enterprise-IdP asserts `principal_type ≠ agent`" but does not define the binding between the IdP assertion and the Solvent database record.

4. **No identity verification exists.** Solvent "consumes identity" per ADR Decision 1, but there is no code in the current codebase that consumes identity. The MCP server reads `FABLE_DSN` and exposes tools with no caller authentication.

### Agent vs workload vs service account vs delegated agent

The spec distinguishes four principal classes but provides no mechanism to verify the distinction. In the current codebase, **every caller is effectively anonymous.** The `action_intent` table records `belief_id, belief_status, action, state` — no principal, no identity, no audit of who authorized the action.

**The principal/identity model is architecturally correct but currently nonexistent. It is a requirement for the C-1 model, not a component of it.**

---

## AuthorityTarget Verdict

### Evaluate: principal, resource, scope, action, consequence

The five-tuple is the right shape. Each dimension has open sub-questions:

| Dimension | Spec status | Current code | Verdict |
|---|---|---|---|
| `principal` | PROPOSED, no schema | No column, no table | **Cannot implement C-1 without this.** |
| `resource` | PROPOSED, exact equality | No column | **Cannot implement I-12 without this.** |
| `scope` | OPEN vocabulary, locked semantics | No column | **Cannot implement scope constraints without this.** |
| `action` | PROPOSED, `{namespace, name}` | `action TEXT` (un-namespaced) | **Migration path exists but is unplanned.** |
| `consequence` | PROPOSED, `{type, parameters}` | No column | **Cannot implement risk tiers without this.** |

### Missing dimensions or unresolved semantics

1. **Missing: `principal`.** The entire C-1 model requires `principal` but no schema exists.

2. **Missing: `active` / `state`.** Target immutability requires an `active` flag or state column. None is proposed.

3. **Missing: `approved_by`.** The approval record requires `approved_by` with `principal_type`. None is proposed.

4. **Missing: `policy_version_hash`.** Policy binding requires a FK to `policy_version`. None is proposed.

5. **Unresolved: `scope` dimensions.** The spec says "exact dimensions are OPEN" but scope is part of the target tuple. Two targets differing only in scope must be different targets, but "differing in scope" requires a scope comparison operation that does not exist.

6. **Unresolved: `consequence_type` taxonomy.** The spec says "the kernel sees `principal/resource/scope/action/consequence` and obligation types; the service maps `consequence → tier` via policy." But if the mapping is service-layer only, a tenant can classify any consequence as `LOW_RISK`.

**The five-tuple is the right shape, but four of the five dimensions have no schema representation. The C-1/C-3 model cannot be implemented without first designing the `AuthorityTarget` table.**

---

## Justification Verdict

### Is "belief supports target" itself an authority-bearing claim?

**Yes.** The justification link is not merely an audit trail. It is the assertion that a specific belief's truth is relevant to a specific target's consequence. This assertion is security-sensitive because:

1. A legitimate promoted belief can be attached to an illegitimate target (the moved broadening attack).
2. The link creates the logical connection between epistemic state ("we believe X") and authority state ("X justifies Y").
3. Without the link, the belief and the target are independent. With the link, the belief's promotion status becomes the target's authority precondition.

### Who is authorized to make it?

The spec says "Agent (subject to target's `discharge_policy`) or human/system" may `AttachJustification`. But:

- The `discharge_policy` is customer-defined and can be vacuous.
- The attacher does not need to hold any authority beyond `CreateTarget` + `Belief exists`.
- There is no mechanism to verify that the attacher believes the justification is semantically valid.

**The justification link is an authority-bearing claim made by whoever can call the `AttachJustification` service function. The spec does not gate this behind any existing authority. An agent with `CreateTarget` permission can attach any promoted belief to any target.**

---

## Debt / Discharge Verdict

### Evaluate: debt[], DebtDischarge, qualification, attribution, replay, revocation, atomicity

1. **`debt[]`** is the current CHECK truth. It is a `TEXT[]` with six fixed items. The array's emptiness is the only thing the database verifies for promotion (`promoted_is_debt_free`). The array does not record *why* items were removed.

2. **`DebtDischarge`** does not exist. The spec proposes it as an append-only record but provides no schema. Without it, N2 ("reached discharged state only through an accepted DebtDischarge") cannot be verified.

3. **Qualification** is currently performed by regex matching in `internal/belief/mapping.go:15-33`. One evidence row can retire multiple debt items. The mapping is compile-time constant with no runtime configuration. The spec proposes `deterministic`, `attested`, and `quorum` obligation types but does not define how they are assigned to debt items.

4. **Attribution** is currently absent. `RetireDebt` does not record who retired the item or why. The spec proposes `discharged_by` as a principal but provides no schema.

5. **Replay** is currently uncontrolled. The same evidence artifact can be used to discharge the same debt item multiple times across different beliefs. `evidenceExists` checks per-belief dedup (`internal/belief/belief.go:103-111`) but does not prevent cross-belief reuse.

6. **Revocation** is currently `RetractCascade`-only. There is no mechanism to revoke a specific debt discharge without retracting the entire belief.

7. **Atomicity** is partially enforced. `RetireDebt` + `Promote` are separate transactions. A caller can retire debt in one transaction and promote in another, with a failure between them leaving the belief in an inconsistent state. The spec does not address this.

**The debt/discharge model is the weakest part of the current architecture. The C-1/C-3 spec acknowledges this but does not provide the schema to fix it. N2 is a necessary condition for promotion that cannot be verified without a `DebtDischarge` table.**

---

## Warrant Verdict

### Evaluate: replay, caching, concurrency, policy binding, target binding, revocation

1. **Replay:** The spec says "a retracted justification belief kills every warrant that cited it, centrally and immediately" — but this assumes the executor re-verifies. An executor that caches the warrant and does not re-verify has a stale warrant. There is no warrant table to enforce expiry or revocation.

2. **Caching:** The spec says "never cached past the ledger's ability to retract" but does not define a maximum cache duration or a re-verification protocol. A cached warrant is a fail-open artifact.

3. **Concurrency:** Two concurrent `Approve` requests for the same target can both succeed before either justification set is fully evaluated. The spec does not propose a uniqueness constraint or serialization mechanism.

4. **Policy binding:** The spec says warrants are bound to `policy_version_hash` — but no `policy_version` table exists. The binding is a hash string with no verifiable referent.

5. **Target binding:** The spec says warrants reference `target_id + justification_snapshot + issued_at` — but no `warrant` table exists. The binding is a JSON object with no database constraint.

6. **Revocation:** The spec says "central verification invalidates immediately" — but there is no central verification mechanism in the current codebase. `solvent_explain` is a read-only projection, not a verification service.

**The warrant model is entirely proposed with no schema, no implementation, and no verification mechanism. Every claim about warrant behavior is a promise.**

---

## Executor Verdict

### Clearly distinguish I-12 (AuthorizationTargetMatch) from I-13 (ExecutionEffectMatch)

The specification correctly distinguishes:
- **I-12:** `authorized tuple == presented tuple` — Solvent guarantees the executor was authorized for exactly what it presented.
- **I-13:** `presented tuple == actual world effect` — Solvent does not yet guarantee what the executor actually did.

**This is the strongest part of the C-1/C-3 specification.** The distinction is architecturally correct and honestly deferred.

### What MVP can honestly guarantee

MVP can honestly guarantee **only I-12** — and only if:
1. The `AuthorityTarget` table exists with an immutable tuple.
2. The executor presents the exact tuple verbatim.
3. Solvent verifies exact equality server-side.
4. The warrant carries the exact tuple and is re-verified before every execution.

MVP **cannot** guarantee I-13. A compromised executor that verifies correctly and then acts differently is out of scope.

**The gap between I-12 and I-13 is the product's honest risk. The C-1/C-3 spec correctly names it but does not bound it.**

---

## Evidence / Provenance Verdict

### Evaluate: artifact, citation, provenance, replay, source compromise, shared artifacts

1. **Artifact vs citation:** The spec correctly distinguishes the reusable artifact from the per-belief citation. But no `artifact` table exists. Current `evidence` is belief-owned (`belief_id` FK, one row per belief). The proposed artifact/citation split has no table representation.

2. **Provenance:** Current `evidence.provenance_class` is an unenforced enum (`db/001_schema.sql:51-52`). The current code accepts any string that passes the CHECK. `content_sha256` is caller-supplied and not verified (`kernel/kernel.go:73-78`).

3. **Replay:** Current dedup is per-belief `content_sha256` with an accepted TOCTOU window (`internal/belief/belief.go:103-111`). Cross-belief reuse is unrestricted.

4. **Source compromise:** A compromised evidence source can insert forged evidence rows. The current ingestion path (`internal/pipeline/pipeline.go:59-121`) is fixture-only for the demo, but the proposed production model has no source-compromise handling.

5. **Shared artifacts:** The spec says "artifacts are shared, citations are per-belief." But without an `artifact` table, there is no mechanism to share artifacts. Each belief creates its own `evidence` rows.

**The evidence/provenance model is semantically correct but currently nonexistent. The C-1/C-3 spec's I-P2 rule (deterministic ≠ unverified external_feed) is violated by current code and has no enforcement mechanism.**

---

## Multi-Tenant Verdict

### Determine whether the abstraction is safe for future SaaS tenancy

**No.** The C-1/C-3 spec talks about tenants extensively ("authenticated to the tenant," "tenant policy," "tenant admin") but the current codebase has no tenant isolation:

- `belief` has `scenario_id` but no `tenant_id`
- `evidence` has `scenario_id` but no `tenant_id`
- `action_intent` has `scenario_id` but no `tenant_id`
- No `tenant` table exists
- No tenant authentication exists
- No row-level security exists

The `scenario_id` is a fixed UUID map (`cmd/solvent-mcp/main.go:31-34`), not an authenticated tenant identifier. Any caller who can connect to the database can access any scenario.

**The C-1/C-3 model is built on tenant isolation that does not exist. Before any schema work, the tenant model must be designed and implemented.**

---

## DocTrust Verdict

### State: what it proves, what it fails to prove, whether it should remain Reference Implementation #1

**What DocTrust proves:**
- A separate application can consume Solvent through public interfaces (MCP/REST) without sharing internals.
- The four-verb slice (`Connect → Ask → Authorize → Reassess`) is legible and usable.
- The `Evidence → Belief → Debt → Promotion → AuthorityTarget → Warrant` flow can be exercised end-to-end.

**What DocTrust fails to prove:**
- Domain-agnostic generality (single document domain, deterministic debt).
- Multi-belief AND justification (DocTrust exercises single-document approval).
- Risk-tiered consequences (DocTrust does not exercise `HIGH_IMPACT` vs `LOW_RISK`).
- Tenant isolation (DocTrust is single-tenant).
- Semantic relevance of justification (DocTrust's beliefs are tightly coupled to its targets).
- The C-1 four-role model (DocTrust does not exercise target creation, link attachment, or approval separation).

**Whether DocTrust should remain Reference Implementation #1:**
Yes, but with a caveat. DocTrust is the right first reference integration because it exercises the core flow. But it **must not be used as evidence that the C-1/C-3 model is domain-agnostic or production-ready.** DocTrust will pass while the model remains dangerously broad for multi-domain use cases.

**What synthetic test is required:**
A second synthetic test domain that exercises:
1. One belief authorizing multiple consequences with different risk tiers.
2. A target whose justification set is technically valid but semantically does not support the consequence.
3. A workload approver masquerading as a human approver.
4. Multi-tenant isolation (tenant A's evidence cannot be cited by tenant B).

---

## Formal Verification Verdict

### State what belongs in: Lean, SQL, Service, Identity, Executor

| Layer | What belongs here | What does NOT belong here |
|---|---|---|
| **Lean** | DB-enforced state-machine preservation (`promote_preserves_validity`, `retractCascade_preserves_validity`, `live_intent_implies_promoted`). Do NOT extend Lean until the corresponding SQL constraints exist. | Principal/identity checks, policy validation, semantic relevance, target immutability (these are service-layer or DB-layer, not state-machine). |
| **SQL** | `promoted_is_debt_free` (N1), `gate` (belief→intent), `live_requires_promoted` (intent state), `belief_id_status_key` (FK target). Future: `principal_type` CHECK, `active` target state, `policy_version_hash` FK, `DebtDischarge` uniqueness. | Obligation validation (I-P1/I-P2), semantic relevance, policy downgrade detection — these are service-layer logic. |
| **Service** | I-A1..I-A4 enforcement, I-P1/I-P2 validation, N2 discharge-array consistency, policy version binding, semantic relevance check, warrant re-verification, tenant authentication, identity binding. | State-machine preservation (that's Lean/SQL), database schema design (that's SQL), identity verification (that's Identity). |
| **Identity** | Principal verification, `principal_type` assertion, attestation signature verification, workload identity validation, IdP integration. | Database constraints, policy validation, target immutability. |
| **Executor** | I-12 tuple presentation and exact-equality verification, I-13 execution-effect receipt (future), warrant caching policy, fail-closed on Solvent unreachable. | Authorization decisions (that's Service/Solvent), belief promotion (that's Kernel/SQL). |

**The critical gap: the current Lean model proves abstract state-machine preservation for the four current transitions. It has no `principal`, `resource`, `scope`, `consequence`, `DebtDischarge`, `AuthorityTarget`, or `Warrant` types. The C-1/C-3 model cannot be formally verified until the corresponding SQL constraints exist to reflect the theorems.**

---

## Implementation Gate Verdict

### Can we now safely touch the kernel/schema?

**NO.**

The C-1/C-3 specification cannot authorize kernel/schema work in its current form. The following blockers must be resolved before any `AuthorityTarget`, `DebtDischarge`, `policy_version`, `principal`, or `justification` schema is written:

### Exact blockers

1. **No `principal` column or table exists.** The entire C-1 model rests on `principal_type, principal_id` but there is no schema for it. Without a `principal` table, `I-A4` (ApprovalSeparation) cannot be enforced at the database level. Every schema design must begin with the `principal` table.

2. **No `AuthorityTarget` table schema exists.** The four-role model describes four operations on a table that does not exist. The schema must define: `target_id`, `principal_id` (FK to `principal`), `resource_type`, `resource_id`, `scope`, `action_namespace`, `action_name`, `consequence_type`, `consequence_parameters`, `state` (`proposed`/`active`/`revoked`), `created_by`, `created_at`, `approved_by`, `approved_at`, `policy_version_hash` (FK to `policy_version`), `justification_snapshot`.

3. **No `DebtDischarge` table exists.** N2 ("reached discharged state only through an accepted DebtDischarge") is a necessary condition for promotion that cannot be verified without the discharge ledger. The schema must define: `discharge_id`, `belief_id`, `obligation_key`, `instrument_kind`, `instrument_ref`, `discharged_by` (FK to `principal`), `issued_at`, `recorded_at`, `status` (`accepted`/`rejected`/`pending`), `integrity`.

4. **No `policy_version` table exists.** I-P4 (PolicyVersionBinding) requires a `policy_version` table with `version`, `effective_at`, `policy_hash`, `author_principal`. Without it, policy version binding is a hash string with no verifiable referent.

5. **No `obligation` table exists.** The current `belief.debt[]` is a `TEXT[]` with no per-obligation configuration. I-P1/I-P2 require obligation types (`deterministic`, `attested`, `quorum`) and discharge policies, which require a table.

6. **No `justification` link table exists.** The four-role model's `AttachJustification` operation requires a `justification` table with `justification_id`, `target_id`, `belief_id`, `obligation_key`, `attached_by`, `attached_at`, `state`.

7. **I-P2 is violated by current code.** `kernel/kernel.go:73-78` accepts caller-supplied `content_sha256` for `external_feed` evidence. The policy floor claims I-P2 is "locked" but the code violates it. The spec must either fix the code or downgrade the claim.

8. **No tenant authentication mechanism exists.** The C-1 model requires "authenticated to the tenant" but there is no tenant authentication in the current codebase.

9. **No bootstrap provisioning mechanism exists.** The trust root is hand-waved. The first `AuthorityTarget` cannot be created without a bootstrap admin, but there is no mechanism to establish the bootstrap admin.

10. **No executor contract exists.** C-4 from Review #2 remains open. The C-1/C-3 spec does not address it.

### Conditions that would make it safe

The following conditions would make kernel/schema work safe:

1. **C-1a:** Design the `principal` table with `principal_type CHECK (principal_type IN ('human','agent','workload','service'))` and an identity-binding mechanism that Solvent can verify.

2. **C-1b:** Design the `authority_target` table with `state CHECK (state IN ('proposed','active','revoked'))`, `approved_by` FK to `principal`, and an immutable `tuple_hash` CHECK.

3. **C-1c:** Design the `justification` link table with `state` and `belief_id` FK with `ON UPDATE CASCADE` to propagate retraction.

4. **C-2a:** Design the `debt_discharge` table and resolve the N2 dual-truth consistency question (transitional `debt[]` vs preferred `obligation rows → discharge rows → derived debt`).

5. **C-3a:** Design the `policy_version` table with `effective_at` monotonicity and `policy_hash`.

6. **C-3b:** Either implement I-P1/I-P2 as database constraints or downgrade them to "candidate rules (OPEN)" with explicit acknowledgment that the current codebase violates I-P2.

7. **C-3c:** Either promote I-P3 to a locked rule with enforcement or explicitly document that `HIGH_IMPACT` consequences can currently be authorized with only deterministic obligations.

8. **C-4:** Design the `obligation` table with `obligation_type`, `discharge_policy`, and `min_sources`.

9. **C-5:** Design the `approval_record` table with `approver_principal_type` CHECK that excludes `agent` for `HIGH_IMPACT`.

10. **C-6:** Define the bootstrap provisioning mechanism with enough precision to implement and audit.

11. **C-7:** Produce the executor contract document (C-4 from Review #2).

12. **C-8:** Implement tenant authentication with `tenant_id` on every authority-bearing table.

---

## Dogfood Evidence Ledger

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Current schema has 4 tables with no principal, no resource, no scope, no consequence | `db/001_schema.sql:6-76` — `belief, belief_edge, evidence, action_intent` only | High | — | Discharged |
| Current `action_intent` has no principal, resource, scope, or consequence column | `db/001_schema.sql:60-76` | High | — | Discharged |
| Current kernel has 8 functions, none related to authority creation | `kernel/kernel.go:54-180`, `kernel/contract.go:16-25` | High | — | Discharged |
| Current MCP has 6 tools, no identity/auth | `cmd/solvent-mcp/main.go:88-247` | High | — | Discharged |
| Current `belief.debt[]` is the only CHECK truth; no discharge record exists | `db/001_schema.sql:30-32`, `kernel/kernel.go:83-88` | High | — | Discharged |
| Current `content_sha256` is caller-supplied and not verified | `db/001_schema.sql:55`, `kernel/kernel.go:73-78` | High | — | Discharged |
| Current `evidence` is belief-owned; no artifact/citation split | `db/001_schema.sql:47-58` (`belief_id` FK) | High | — | Discharged |
| No `principal` table or column exists anywhere | `db/001_schema.sql` (whole file), `kernel/kernel.go` (whole file), grep for `principal` returns zero schema matches | High | — | Discharged |
| No `AuthorityTarget`, `DebtDischarge`, `policy_version`, `justification`, `obligation`, `warrant`, or `principal` table exists | `db/001_schema.sql` (whole file), `db/002_corpus.sql`, `db/003_wizard.sql` | High | — | Discharged |
| No tenant authentication mechanism exists | `cmd/solvent-mcp/main.go:39-44` (reads DSN from env, no auth), grep for `tenant_id` in schema returns zero | High | — | Discharged |
| No bootstrap provisioning mechanism exists | No `bootstrap_admin` table, no provisioning API, no deployment config schema | High | — | Discharged |
| C-1 four-role model has no database enforcement (I-A1..I-A4 all service-enforced) | C-1/C-3 spec `## C-1 Security Invariants`: every invariant lists "Enforcement: service" or "future DB" | High | DB schema for principal, target, justification, approval | Open |
| C-3 universal negatives (I-P1/I-P2) are service-enforced only | C-1/C-3 spec `## C-3 Security Invariants`: "Enforcement: service (policy validation). Future DB CHECK" | High | Policy validation service, DB CHECK constraints | Open |
| I-P3 (HIGH_IMPACT floor) is OPEN and not enforced | C-1/C-3 spec `## C-3 Policy Safety Floor`: "candidate, explicitly OPEN" | High | Customer use-case validation | Open |
| I-P4 (PolicyVersionBinding) requires `policy_version` table that does not exist | C-1/C-3 spec proposes `policy_version` table but provides no DDL | High | `policy_version` table schema | Open |
| N2 (discharge-array consistency) cannot be verified without `DebtDischarge` table | C-1/C-3 spec does not address N2; `authority-target-and-debt-discharge-spec.md:717-719` acknowledges dual-truth gap | High | `DebtDischarge` table schema | Open |
| Bootstrap root is hand-waved | C-1/C-3 spec `## C-1 Bootstrap Trust Root`: "Exact provisioning mechanism stays PROPOSED" | High | Provisioning mechanism | Open |
| "Agent may create target but not activate" is unenforceable without `principal_type` | No `principal_type` column exists; current MCP has no caller identity | High | `principal` table, identity binding | Open |
| Target immutability is promised but not enforceable | No `active` column, no immutable tuple CHECK, no `AuthorityTarget` table | High | `AuthorityTarget` table schema with `state` and `tuple_hash` | Open |
| DocTrust will not exercise multi-belief, risk-tiered, or tenant-isolated semantics | C-1/C-3 spec `## DocTrust Impact` explicitly lists what DocTrust will NOT exercise | Medium | Second synthetic test domain | Open |
| C-1/C-3 spec's self-attack results overstate "blocked" dispositions | Spec claims "Blocked by policy floor" and "Blocked by identity" for service-enforced rules | Medium | Revised self-attack dispositions | Open |

---

## Self-Attack Summary

| Attack | Failure point | Enforcement | Residual risk | Status |
|---|---|---|---|---|
| Attacker creates HIGH_IMPACT target + attaches legitimate belief + gets human to approve | `Approve` requires human/non-agent (I-A4) | Service + Identity (enterprise IdP) | **HIGH:** No `principal_type` column; IdP can issue `human` tokens to agents; service bug can bypass check | **CONDITIONALLY BLOCKED** |
| Attacker replays valid attestation for second `(belief_id, obligation)` | Attestation payload binds `belief_id+obligation` | Invariant/attestation (signed payload) | **LOW:** Revoked attestation replay deferred to lifecycle (F-05) | **BLOCKED** (current) / **DEFERRED** (revocation) |
| Attacker uses same artifact/citation to discharge two obligations | One citation per retrieval debt; one discharge per obligation | Service (one-discharge→one-obligation, I-P2) | **MEDIUM:** Current code does the opposite (`mapping.go:20` scores 2 items from one release) | **CONDITIONALLY BLOCKED** |
| Attacker splits `needOperatorSignoff` into trivial sub-obligations | Policy validation rejects sub-obligation sets | Service (I-P1) | **HIGH:** No policy validation service exists | **UNRESOLVED** |
| Compromised evidence source forges artifact retiring deterministic obligation | I-P2: deterministic ≠ unverified `external_feed` | Service + future I-11 | **CRITICAL:** Current code violates I-P2 (`kernel/kernel.go:73-78`) | **UNRESOLVED** |
| Migration `NULL → wildcard` broadens old rows | Fail-closed: NULL → NOT AUTHORIZED | DB (migration rule) | **LOW:** Rule is correct; test coverage incomplete | **BLOCKED** (rule) / **OPEN** (test) |
| Concurrent Promote vs RetractCascade vs Approve creates live on retracted | Existing `gate`/`live_requires_promoted`/`RetractCascade` atomicity | DB (composite FK + ON UPDATE CASCADE) | **LOW:** Current invariants are solid; proposed target-level validity is service-enforced | **BLOCKED** (current) / **CONDITIONALLY BLOCKED** (proposed) |
| Stale warrant replayed after belief retraction | Central verification invalidates immediately | Service + Policy floor | **MEDIUM:** No warrant table; no central verification service; executor caching is unregulated | **CONDITIONALLY BLOCKED** |
| Scope silently broadens | Scope narrows-only; unspecified ≠ wildcard | Service + I-12 | **MEDIUM:** Scope comparison operation does not exist | **CONDITIONALLY BLOCKED** |
| Weak discharge → strong authority (single attestation covers deploy staging and deploy production) | Explicit per-target justification links + AND | DB + Service | **LOW:** Model is correct; enforcement requires `justification` table | **CONDITIONALLY BLOCKED** |
| Buggy executor authorized A but executes B | I-12 AuthorizationTargetMatch: presented ≠ authorized → refused | Service + Executor contract | **LOW:** I-13 (ExecutionEffectMatch) is deferred; compromised executor that presents correctly then does otherwise is unresolved | **CONDITIONALLY BLOCKED** (I-12) / **UNRESOLVED** (I-13) |
| Customer self-approves via delegated workload that is really the same agent | I-A4: approver ≠ requester for HIGH_IMPACT; enterprise-IdP asserts `principal_type ≠ agent` | Service + Identity | **HIGH:** No `principal_type` column; IdP configuration is tenant-controlled | **CONDITIONALLY BLOCKED** |
| Policy downgrade after Approve to make future targets easier | Detected by audit (`policy_version_hash`); new approvals re-evaluated under current policy | Service + PolicyVersionBinding | **MEDIUM:** No `policy_version` table; downgrade is allowed with audit visibility | **CONDITIONALLY BLOCKED** |
| Link-sprawl creates dangerous breadth | All links are explicit rows, enumerable | Audit | **MEDIUM:** Preventive `who may link` control is deferred to OPEN #8 | **DETECTED BY AUDIT** |
| Attacker creates 1,000 targets to flood approval queue | No creation rate limit; no target namespace constraints | Service | **MEDIUM:** DoS issue, not direct privilege escalation; requires rate-limiting policy | **UNRESOLVED** |
| Target mutation after approval | Active targets are immutable | Service (no UPDATE path) | **HIGH:** No `active` column, no immutable tuple CHECK, no `AuthorityTarget` table | **UNRESOLVED** |
| Policy upgrade invalidates already-active targets | Re-evaluation under current policy at next verification | Service + Warrant re-verification | **MEDIUM:** No warrant re-verification trigger; no `policy_version` table | **CONDITIONALLY BLOCKED** |
| Workload masquerades as human approver | Enterprise-IdP asserts `principal_type` | Identity + Service | **HIGH:** No `principal_type` verification; IdP configuration is tenant-controlled | **UNRESOLVED** |
| Tenant configures vacuous policy (empty obligations, self-satisfying) | I-P1: no empty/self-satisfying/any-evidence obligations | Service (policy validation) | **HIGH:** No policy validation service exists | **UNRESOLVED** |
| Same evidence artifact discharges multiple obligations across beliefs | One discharge per obligation; one citation per debt | Service | **MEDIUM:** Current code allows cross-belief reuse (`mapping.go:20`) | **CONDITIONALLY BLOCKED** |
| Quorum satisfied by 5 outputs from same model | Quorum requires N independent sources | Service | **LOW:** Quorum is OPEN; no use case yet | **DEFERRED** |

---

## Final Dogfood Test

### Does the C-1/C-3 design itself obey Solvent's methodology?

**What evidence supports the design?**
The C-1/C-3 specification cites repository truth for CURRENT claims (`db/001_schema.sql`, `kernel/kernel.go`, `cmd/solvent-mcp/tools.go`, `docs/ADR/0001`). The specification is grounded in measured repository behavior for what exists today. However, most of the PROPOSED claims (four-role matrix, policy floor, bootstrap root) are supported only by reasoning, with no repository or use-case evidence. The dogfood ledger correctly marks most claims as `Open`.

**What beliefs does it make?**
The specification makes three core proposed beliefs:
1. **Four-role separation** (`CreateTarget` ≠ `AttachJustification` ≠ `RequestAuthorization` ≠ `Approve/Activate`) is the correct authority model.
2. **Universal negatives** (I-P1, I-P2) are the minimum policy floor, locked and never overridable.
3. **Bootstrap root** is human-only tenant-admin via deployment/provisioning.

And three sufficient-condition beliefs:
4. **N1** (`debt=[]`, `final_truth=false`) is necessary for promotion.
5. **N2** (every obligation reached discharged only through accepted `DebtDischarge`) is necessary for promotion.
6. **N3** (not retracted, no open contradiction) is necessary for promotion.
7. **N4** (tenant policy permits promotion) is necessary for promotion.

**What debt remains?**
The specification honestly records 7 OPEN items in `## Open Architectural Questions` and many more in the dogfood ledger. The critical debts are:
- No `principal` table or column (prerequisite for C-1 enforcement)
- No `AuthorityTarget` table schema (prerequisite for four-role model)
- No `DebtDischarge` table (prerequisite for N2)
- No `policy_version` table (prerequisite for I-P4)
- No `obligation` table (prerequisite for I-P1/I-P2)
- No `justification` link table (prerequisite for AttachJustification)
- I-P2 violated by current code (prerequisite for policy floor)
- I-P3 OPEN and not enforced (prerequisite for HIGH_IMPACT safety)
- No bootstrap provisioning mechanism (prerequisite for trust root)

**What would falsify it?**
The C-1/C-3 model would be falsified by:
1. A proof that the four-role model cannot be enforced without a `principal` column and identity binding mechanism.
2. A proof that the policy floor cannot be enforced without database constraints.
3. A proof that the bootstrap root cannot be established without a concrete provisioning mechanism.
4. A proof that `Approve` cannot be separated from `CreateTarget` without a database-enforceable `active` flag.
5. A proof that N2 cannot be verified without a `DebtDischarge` table.

All five proofs are readily available from the current repository state. The C-1/C-3 model is falsified by the absence of the schema it requires.

**Who has authority to promote it?**
Under Solvent's own methodology, the specification must earn promotion through adversarial review. This review is the gate. The verdict is **REJECT / REDESIGN**, meaning the specification is not promoted for kernel/schema authorization. It may guide design discussion but cannot authorize schema changes.

**What evidence would cause reassessment?**
New evidence that any of the following is implemented would cause reassessment:
1. A `principal` table with `principal_type` CHECK and identity binding
2. An `AuthorityTarget` table with `state`, `approved_by`, and immutable tuple
3. A `DebtDischarge` table that makes N2 verifiable
4. A `policy_version` table with `effective_at` monotonicity
5. A policy validation service that enforces I-P1/I-P2
6. A bootstrap provisioning mechanism
7. An executor contract document

Until at least items 1-5 exist, the specification cannot be promoted for kernel/schema work.

---

## Most Important Test of All

> Can an attacker legally use the system's own legitimate primitives in combination to manufacture an authority outcome that the system should not have granted?

**Yes.**

Here is the composition:

```
legitimate promoted belief        (current kernel: Promote succeeds via promoted_is_debt_free)
    +
legitimate CreateTarget call      (proposed: any authenticated principal may propose)
    +
legitimate AttachJustification    (proposed: any authenticated principal may attach)
    +
socially engineered human approver (proposed: human/non-agent Approve required)
    +
service-layer identity bug        (current: no principal_type column, no identity binding)
    =
HIGH_IMPACT authority granted for a consequence the belief does not justify
```

Every primitive is functioning correctly. The belief is genuinely promoted. The target is well-formed. The link is valid. The human approver is authenticated (by a compromised or misconfigured IdP). The service checks pass. The database enforces nothing because there is no schema to enforce it.

**The attacker does not break any primitive. The attacker exploits the gap between primitives — the gap where the C-1/C-3 spec places every security boundary in the service layer with no database backing.**

This is the purpose of adversarial review. The C-1/C-3 specification is not ready for kernel/schema implementation.

---

## Final Discipline

This is a production authorization system. The C-1/C-3 specification is good design prose but it is not an implementation-ready security boundary. It correctly identifies problems and proposes reasonable service-layer solutions. But it does not provide the database schema, constraints, or identity mechanisms that would make those solutions impossible to bypass.

The desired outcome is not "the specification looks good." The desired outcome is "we know precisely why this authority model is safe enough to implement, what remains intentionally outside the guarantee, and what must never be left to caller goodwill."

We do not have that outcome yet.

---

## Files changed

```
exact files changed
  docs/REVIEWS/AUTHORITY_CREATION_POLICY_FLOOR_ADVERSARIAL_REVIEW_3.md  (new, this review)

git status --short
  ?? docs/REVIEWS/AUTHORITY_CREATION_POLICY_FLOOR_ADVERSARIAL_REVIEW_3.md

whether any implementation file changed
  No — no Go, SQL, Lean, MCP, REST, ADRs, or architecture specifications changed

CRITICAL findings
  F-3-1: I-A4 (ApprovalSeparation) has no database enforcement — no principal_type column, no identity binding
  F-3-2: I-A1 (TargetCreationAuthority) has no tenant authentication mechanism
  F-3-3: I-P2 (QualificationIntegrity) is violated by current code (caller-supplied hashes accepted for external_feed)

HIGH findings
  F-3-4: I-A3 (TargetImmutability) has no database mechanism
  F-3-5: Bootstrap root is hand-waved; no provisioning mechanism
  F-3-6: The spec's most important attack succeeds against the service layer
  F-3-7: I-P1/I-P2 universal negatives are violated by current code and have no enforcement
  F-3-8: The four-role matrix describes a system that does not exist

kernel/schema blockers
  1. No principal table or column — I-A4 unenforceable
  2. No AuthorityTarget table schema — four-role model unimplementable
  3. No DebtDischarge table — N2 unverifiable
  4. No policy_version table — I-P4 unenforceable
  5. No obligation table — I-P1/I-P2 unenforceable
  6. No justification link table — AttachJustification unimplementable
  7. I-P2 violated by current code
  8. No tenant authentication mechanism
  9. No bootstrap provisioning mechanism
  10. No executor contract document

next required gate
  The C-1/C-3 specification must be redesigned to include database-enforceable boundaries before any kernel/schema work begins. At minimum:
  - Design principal, AuthorityTarget, DebtDischarge, policy_version, obligation, and justification tables with DDL
  - Define identity binding and tenant authentication mechanisms
  - Define bootstrap provisioning mechanism
  - Either implement I-P1/I-P2 as DB constraints or downgrade them to OPEN
  - Either promote I-P3 to locked or document it as a known limitation
  - Produce the executor contract document (C-4)
```
