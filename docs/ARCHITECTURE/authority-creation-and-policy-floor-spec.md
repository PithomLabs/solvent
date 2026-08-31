# Authority Creation and Policy Floor Specification

## Status

**PROPOSED — NOT IMPLEMENTED**

This is a design specification, not a description of shipped behaviour. It proposes semantics for **who may create, justify, and activate authority** (`C-1`) and for the **minimum safety floor below which Solvent must refuse to operate** (`C-3`). Nothing in this document has been implemented. No table, column, constraint, kernel function, MCP tool, REST endpoint, Lean theorem, or migration described here exists.

- Every claim about what exists **today** is labelled **CURRENT** and cites repository truth (`file:line`). Where a cited document is itself unimplemented future design (e.g. `docs/ADR/0001` decisions), the citation is to the *decision*, not to behaviour.
- Every claim about what is **proposed** is labelled **PROPOSED**.
- Every detail that survives only on reasoning, with no repository or use-case evidence, is labelled **OPEN**.

This specification is itself treated as a belief: it carries a **Self-Attack Results** section and a **Dogfood Evidence Ledger** in which nothing is marked `Discharged` without evidence. It must survive **Adversarial Review #3** (attacking the bootstrap root, target/link creation authority, policy downgrade, and whether “agent may create target but not activate it” is enforceable) before any `AuthorityTarget`/policy kernel or schema change begins (`## Implementation Gate`).

*Revision note:* incorporates the Q2 `scope = open constraint set` decision — scope may narrow but never broaden resource/consequence, dimensions remain OPEN.

---

## Purpose

The production MVP plan (`mvp_plan2.md`) and its adversarial review (`docs/REVIEWS/AUTHORITY_SPEC_ADVERSARIAL_REVIEW_2.md` as authoritative Review #2; `plan3_imp_review.md` and `mvp_plan2_review.md` as supporting earlier context — there is no `review3.md` in the repository) identified two conditions that must be resolved **before** the `AuthorityTarget`/`DebtDischarge` abstraction touches the kernel:

- **C-1** — Authority to create authority: who may `CREATE TARGET`, `ATTACH JUSTIFICATION`, `REQUEST AUTHORIZATION`, `APPROVE/ACTIVATE`, and how to close the recursion `what authorizes the first target?`
- **C-3** — Minimum validity: below what customer-configured policy Solvent must refuse to call anything *authorized*, because `belief → debt → discharge → promotion → target → warrant` would otherwise be internally consistent while semantically meaningless.

This document answers those two questions precisely enough that an engineer could implement them without inventing missing rules, adversarially enough that its weaknesses are recorded as debt, and minimally enough that only the smallest kernel-enforced semantics become impossible states.

It directly implements the **locked choices** from the review synthesis:

- **Bootstrap:** human-only tenant-admin bootstrap via deployment/provisioning configuration. Exact provisioning mechanism stays `PROPOSED`; existence of a non-agent root is **locked**, not `OPEN`. `AI agent cannot bootstrap the authority system that will subsequently authorize that same agent.`
- **Policy floor:** `HIGH_IMPACT ⇒ ≥1 attested or quorum obligation` is the **candidate** minimum, explicitly `OPEN` pending a real high-impact customer use case. **Universal negatives are locked now:** no empty obligation sets, no self-satisfying obligations, no “any evidence qualifies,” deterministic obligations cannot accept unverified `external_feed` alone.
- **Agent capabilities:** `CREATE TARGET YES` (proposing a candidate object, **not** granting authority), `ATTACH JUSTIFICATION YES` subject to policy, `REQUEST AUTHORIZATION YES`, `APPROVE/ACTIVATE NO` by default. `APPROVE` requires an authorized human/non-agent principal. High-impact approval is human/non-agent unless evidence later justifies an automated path. **Creating a target or link does not create authority.**

---

## Background

**Product context:** Solvent is a production SaaS authority layer for AI agents. Agents can reason, propose, consume evidence, and be wrong or manipulated (`docs/ADR/0001:187` bounded PKI, `docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:1` 22 sections). Therefore an agent's conclusion must not silently become durable authority.

Conceptual lifecycle:

```
Evidence → Belief → Obligations/Debt → Verified Discharge → Promotion → AuthorityTarget → Warrant → Execution → Reassessment
```

Current kernel is simpler:

```
Evidence → Belief → Debt → Promotion → ActionIntent (`db/001_schema.sql:1` 4 tables)
```

Current invariants (`promoted_is_debt_free` `23514`, `gate` `23503`, `live_requires_promoted` `23514`) are valuable and must not be weakened (`AGENTS.md:52` frozen, `kernel/kernel.go:54` `FullDebt`).

**Already proposed abstraction** (`docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`):

- Belief = *“What do we currently hold to be sufficiently supported?”* — epistemic, never authority.
- AuthorityTarget = *“What exact real-world consequence are we permitting?”* — `principal + resource + scope + action + consequence`. A promoted belief is **eligible to justify**, not authority. Belief↔Target is many-to-many via explicit justification links, `AND` for MVP (`mvp_plan2.md:402`).

The main spec is `PROPOSED — NOT IMPLEMENTED` and has survived one revision and one adversarial review (`docs/REVIEWS/AUTHORITY_SPEC_ADVERSARIAL_REVIEW_2.md` verdict `APPROVE WITH CONDITIONS` `C-1..C-4`). This spec focuses **only** on `C-1` and `C-3`.

---

## C-1 Problem — Authority to Create Authority

The M:N justification model fixes **wildcard permission** (`promoted belief ≠ permission`) but creates a **moved broadening attack** (`plan3_imp_review.md:182`):

```
legitimate promoted belief
    +
attacker-created AuthorityTarget / justification link
    ↓
apparently valid authorization
    ↓
high-impact action
```

Example: legitimate belief `"customer identity was verified"` + attacker creates `AuthorityTarget{principal=attacker-agent, resource=treasury-account, action=transfer, consequence=funds_transferred}` and attaches the legitimate belief. The belief is genuinely promoted, the database has no invalid state, the relationship is nevertheless illegitimate.

The prior spec correctly states:

> Authorization to create an authorization object is itself an authorization problem.

but identifies four operations without defining who may perform each:

```
1. create target
2. attach belief / create justification link
3. request authorization
4. approve / activate
```

This is an unresolved production security boundary. Until it is resolved, `N beliefs → 1 target` with explicit links only moves the wildcard from implicit promotion to explicit linking.

---

## C-1 Four-Role Model

### Four distinct authority transitions

Do **not** conflate:

```
creating a target          — proposing a candidate authorization object
attaching a justification  — claiming “this belief supports this target”
requesting authorization   — asking “may this target become authority?”
approving / activating     — causing the target to become authority
executing authorization    — (separate, out of scope for C-1/C-3)
```

Each is a different transition, with different actor, preconditions, and audit.

### Proposed four-role matrix (PROPOSED)

| Operation | Permitted principal (PROPOSED) | Preconditions | Required existing authority | Enforcement (proposed) | Audit (proposed) |
|---|---|---|---|---|---|
| **CreateTarget** | **Agent** or any authenticated principal (human, workload, service account) — **proposing only** | Caller is authenticated to the tenant; `resource`/`consequence` are syntactically valid opaque identities; no `*` wildcard unless policy explicitly lists allowed wildcards (none for MVP) | **None** beyond `authenticated to tenant`. Creation does **not** imply any belief, any link, or any authorization. Creation alone is never authority. | **Service** — validates principal identity (workload-attested or tenant-scoped), validates `principal/resource/scope/action/consequence` are well-formed and not wildcard, writes target in state `proposed`/`draft` | `target_id, created_by, created_at, principal, resource, scope, action, consequence, state=proposed` |
| **AttachJustification** | **Agent** (subject to target's `discharge_policy`) **or** human/system | Target exists in `proposed`; belief exists and is not retracted; `obligation` belongs to that belief per policy; instrument (evidence/attestation) not yet used to discharge a *different* obligation instance (one-discharge→one-obligation, `I-P2`); attachment does not auto-discharge — it only proposes the link | **None** beyond `CreateTarget` + `Belief exists`. Attachment creates a `Justification` row in state `proposed`; it does **not** promote, discharge, or authorize. | **Service** — validates `belief_id` exists in same tenant/scenario, validates `obligation_key` is in that belief's current obligation set, checks `instrument_kind` qualifies per `discharge_policy`; single-transaction link + idempotency check | `justification_id, target_id, belief_id, obligation_key, attached_by, attached_at, state=proposed` |
| **RequestAuthorization** | **Agent** or any authenticated principal | Target exists; all required justification links exist in `proposed`; caller may be different from creator/attacher | **None** beyond the above — requesting is not approving. The request is the *question* “is this target now justified?”, not the answer. | **Service** — collects the target's required `AND` set (`docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md:449` AND semantics), checks that each linked belief is currently `promoted` (per `gate` FK shape) — but does not yet decide | `request_id, target_id, requested_by, requested_at, justification_snapshot` |
| **Approve / Activate** | **Authorized human or non-agent principal only** — **never the requesting agent** by default; must be a principal that the tenant's approval policy lists for the target's `consequence` class (e.g. `HIGH_IMPACT ⇒ treasury-operator`) | Target is `proposed`; every required belief is `promoted`; every required obligation has an `accepted` `DebtDischarge` satisfying its `discharge_policy` ( `N1∧N2∧N3∧N4` per `authority-target-and-debt-discharge-spec.md:688`); no open contradiction; tenant policy permits approval for this `consequence` | **Explicit approval authority** scoped to `target.resource`/`consequence` — not generic “can approve anything.” The approver's principal must be independently authenticated (enterprise identity, `docs/ADR/0001:499` outside kernel) and must differ from the agent that requested when `HIGH_IMPACT` (separation of duties, see below). | **Service + future DB (see invariants)** — service verifies all preconditions atomically, then transitions target `proposed → active` in one transaction; future invariant `I-A4` makes `active` without valid approval impossible | `target_id, approved_by, approved_at, prior_state, new_state=active, approver_principal_type/id, policy_version_hash, justification_snapshot` |

**Key distinctions enforced by the matrix:**

- **Creating an authorization object is not granting authorization.** The `CreateTarget` row has no authority until `Approve`. A `proposed` target cannot be `ALLOW`ed, even though it is already stored. This is the crucial boundary (`mvp_plan2_review.md:194` — creating a target must not mean creating authority).
- **Attaching is not discharging.** A justification link is a *claim* that `Belief B supports Target T`. The claim becomes discharge-relevant only when its obligation's `DebtDischarge` is `accepted` with an integrity-bound instrument (C-3 floor).
- **Requesting is not approving.** The agent's most useful role is to propose well-formed targets, attach plausible justifications, and *ask*. The decision is elsewhere.
- **Auditability ≠ prevention, but prevention starts with audit.** Every link is an **auditable row** (`docs/ARCHITECTURE/...:432` sprawl is visible), but audit alone does not prevent link-sprawl — hence the creation/attach authority column and the `Approve` gate.

---

## C-1 Bootstrap Trust Root

### The recursion

If `creating AuthorityTarget requires authority`, what authorizes the first target?

```
authority is needed to create authority
    but
no authority exists yet
```

Hand-waving this away guarantees that the first real customer has a `root` principal that can do anything, forever.

### Proposed root — human-only tenant-admin bootstrap

**Locked:** use a **human-only tenant-admin bootstrap via deployment/provisioning configuration** as the minimum trust root. Exact provisioning mechanism stays `PROPOSED` (e.g., initial tenant admin list in deployment config, seeded via enterprise identity provider sync, or system-level provisioning API with human operator cert), but the *existence* of a non-agent root is **locked, not `OPEN`**.

```
Deployment / provisioning (system-level, human-operated)
        ↓
Human tenant administrator(s) (individuals, not agents)
        ↓
policy + authority configuration (obligation sets, `principal/resource` namespaces, approval policies)
        ↓
Solvent service
```

**Principle (locked):**

> **An AI agent cannot bootstrap the authority system that will subsequently authorize that same agent.**

- The bootstrap tenant admin(s) are humans (or human-attested workload identities managed outside Solvent, `AGENTS.md:56` evidence-source abstraction).
- Bootstrap principals are provisioned **out-of-band** of Solvent's own `CreateTarget` path. Their creation does not go through `AuthorityTarget` authorization; it goes through the deployment's provisioning control plane, which is itself human-authorized and audited.
- The set of bootstrap admins is **small, enumerable, and revokable**, and any change to it is itself an `Approve`-level event (same human/non-agent gate).
- Once bootstrap exists, **all** subsequent `AuthorityTarget` creation is governed by the matrix above; no “root can do anything via `action TEXT`” wildcard survives beyond provisioning.

**Attack on recursion — how this closes it:**

- Attacker who controls an agent but not the bootstrap cannot create a target that self-authorizes: `CreateTarget` succeeds (proposed), but `Approve` requires a human/non-agent principal that the attacker does not control, and `Attach` still requires `Belief.promoted` and `DebtDischarge.accepted`. The recursion terminates at a human.
- A compromised bootstrap admin is a **tenant-compromise**, not a Solvent bypass — same as a compromised IdP admin. Mitigation is enterprise IdP controls (revocation, MFA, audit), not Solvent claiming to be its own IdP (consistent with `docs/ADR/0001:499` Solvent consumes identity).

*Do not* treat the first `AuthorityTarget` as self-authorizing, agent-bootstrapped, or tenant-wildcard. The alternative — `agent may bootstrap` — is the authority equivalent of `debt=[]` checkbox theater: formally `ALLOW`ed but semantically meaningless, and therefore rejected as a universal rule (see C-3 floor).

---

## C-1 Target Creation Rules

- **Creation is proposing.** Any **authenticated** principal (human, agent, workload, service account) may `CreateTarget` in state `proposed`. The only precondition is `authenticated to tenant` + syntactically valid opaque `resource`/`consequence` identities (no `*`, no empty, no normalization by the adapter — adapters must not lowercase/alias, `docs/REVIEWS/...:62`). See matrix.
- **Creation is never authority.** A `proposed` target has no warrant, no `ALLOW`, and cannot be executed. The transition `proposed → active` is the only authority-granting transition, via `Approve`.
- **Policy-controlled dimensions:** Customer policy may restrict which principals may create targets for which `resource` prefixes or `consequence` types (e.g., only `treasury-agent` may propose `funds_transferred`), but for MVP the *creation* restriction is **policy, not kernel** — the kernel currently has no `resource` column at all (`db/001_schema.sql:60-76`), and adding a kernel `CHECK` for creation would be premature before the authority-target spec is validated by DocTrust. Keep it service-enforced with audit, make the policy dimension *configurable* (customer), not *hardcoded*.
- **Enforcement:** service validates principal identity and target well-formedness, writes `state=proposed`, emits audit. No DB `CHECK` yet beyond future additive columns (`docs/POST_HACKATHON_ARCHITECTURE.md:491-503` discipline).

---

## C-1 Justification-Link Rules

- **Link = claim that a specific belief supports a specific target.** `AttachJustification(target_id, belief_id, obligation_key)` creates a `Justification` row in state `proposed`. The link does not discharge an obligation; it only asserts that `Belief B` is *candidate* justification for `Target T`.
- **Attaching is link-sprawl capable — hence auditable.** An operator or automation that links every promoted belief to every target “to be safe” recreates wildcard via diligence failure. The mitigation is **not a new mechanism but explicit rows**: every link is an auditable row, so sprawl is enumerable (`docs/ARCHITECTURE/...:432`). Preventive control beyond audit is `Approve` — a sprawled target still cannot become `active` without approval.
- **Attachment preconditions (service):** `target` is `proposed`, `belief` exists and is not `retracted` (`db/001_schema.sql:12`), `obligation_key` is in that belief's current obligation set, `instrument_kind` (evidence vs attestation) qualifies per `discharge_policy`. One `DebtDischarge` → one `obligation` (see C-3); the same `attestation_id` cannot be attached to two `(belief_id, obligation)` pairs — binding is inside the signed payload (`docs/ADR/0001:175-178`).
- **Q: Should attaching a belief require authority scoped to the target?** For MVP, **yes as a service check**: the attacher must be `authenticated to tenant` and must not be *expressly forbidden* by a target-creation policy (e.g., `treasury-account` targets require `treasury-agent`). Full scoped authority (“attaching to `treasury-account` requires `treasury` authority”) is the natural next tightening after DocTrust proves the `principal/resource` abstraction, not the first gate — hence **deferred to `OPEN #8`** as the four-role refinement, but the *existence* of the check is locked now.
- **Q: Should target creation and attaching be separate privileges?** Yes — they are already separate operations in the matrix; keeping them separate is the cheapest way to require at least two auditable actions for the moved-broadening attack. Coalescing them would let `CreateTarget` implicitly attach its justifications without a second auditable row.

---

## C-1 Approval / Activation Rules

- **Approval is the authority-granting transition.** `Approve` moves `proposed → active`. Only an **authorized human or non-agent principal** (per `docs/ADR/0001:187` bounded PKI: the attestation signer's identity is established out-of-band) may approve. The approver's principal must be independently authenticated (enterprise IdP, `docs/ADR/0001:499`) and must **differ from the requesting agent** when the target's `consequence` is `HIGH_IMPACT` (separation of duties, see below).
- **High-impact separation of duties:** For `consequence ∈ {funds_transferred, production_deployment, production_dns_record_modified, clinical_treatment_record_modified}` (the four cross-domain examples in `docs/ARCHITECTURE/...:1025`), `approver ≠ requester` and `approver` must be in the `HIGH_IMPACT` approver set (e.g., `treasury-operator`, `on-call human`). For `READ`/`DRAFT`/`LOW_RISK_EXECUTE` the same principal may both propose and approve — dual control is not justified there. This mirrors `docs/ADR/0001:113-134` risk tiers while keeping the rule policy-controlled, not kernel-hardcoded.
- **What approval checks (service, atomically):** every required belief (the `AND` set, `docs/ARCHITECTURE/...:402`) is currently `promoted`; every required obligation has an `accepted` `DebtDischarge` satisfying its `discharge_policy` and passing the C-3 floor; no open contradiction is recorded against any justification belief; tenant policy version in effect at approval is recorded (`policy_version_hash`); `Approve` is idempotent-safe (re-approval does not create a second active target).
- **What is recorded:** `target_id, approved_by, approved_at, prior_state=proposed, new_state=active, approver_principal_type/id, policy_version_hash, justification_snapshot` — the snapshot is what later reassessment and the warrant verification will compare against.

---

## C-1 Target Immutability

- **Target identity is the exact tuple.** `principal + resource + scope + action + consequence` is the identity (see `docs/ARCHITECTURE/...:449-575`). Any change to any field — including adding a scope constraint — produces a **different target** requiring **new authorization**. There is no in-place mutation of an `active` target.
- **Target fields must be immutable after creation.** A `proposed` target may be edited before any justification is attached; once a justification link exists, or once `Approve` has been requested, the target is **frozen**. Edits after that point must create a **new target** (new `target_id`). This prevents `time-of-check/time-of-use` drift (`adv_plan_prompt.md:342-361`).
- **Links may not change after authorization.** Once a target is `active`, its `Justification` set is immutable. Adding a `Justification` link after `active` is not an update — it is a request for a **new authorization** (new `Warrant` over the new snapshot). Removing a link or retracting a justification belief invalidates the target via reassessment (`kernel/kernel.go:114` `RetractCascade` analog for targets, future).
- **Warrant identity follows.** A warrant references `target_id` + `justification_snapshot` + `issued_at`. Changing the target changes the identity; the old warrant remains bound to the old tuple and fails closed on verification (central verification, `docs/ADR/0001:95-111`).

---

## C-1 Agent vs Human Capabilities

| Principal class | CreateTarget | AttachJustification | RequestAuthorization | Approve / Activate | Notes |
|---|---|---|---|---|---|
| **Agent (model, tool, workload)** | **YES** — proposing only | **YES** — subject to target's `discharge_policy`;  `agent:` never satisfies an attested obligation (`docs/ADR/0001:160-179`) | **YES** | **NO** — by default | The agent's value is in proposing well-formed targets and plausible justifications; its limitation is not being able to self-approve. This is the production form of `docs/ADR/0001:181-183` “agent may request, never manufacture.” |
| **Human / authorized non-agent** (tenant-admin-provisioned, enterprise-IdP-attested, `docs/ADR/0001:187-228` attestation lifecycle) | YES | YES | YES | **YES** — when they are in the `HIGH_IMPACT` approver set and differ from requester | Bootstrap admins are this class. For `LOW_RISK` the same human may both propose and approve; for `HIGH_IMPACT` separation of duties applies. |
| **Service account / delegated workload** | YES | YES | YES | **Conditional** — only when its `principal_type` is explicitly listed in the target's approval policy as a non-agent approver (e.g., `workload:payment-processor` for `LOW_RISK`); never for `HIGH_IMPACT` unless evidence later justifies an automated approval path (see `## Open Architectural Questions`). | Delegation = independently verifiable relationship to delegating principal (`docs/ARCHITECTURE/...:475`); full chain is `OPEN` and out of MVP. |

**Prominence rule for any agent-facing surface** (MCP `solvent_explain`, future REST):

> `agent may create target` means `agent may propose a candidate authorization object`. It does **not** mean `agent may grant itself authority`. A `proposed` target has no authority until an authorized human/non-agent has `Approve`d it, and the lifecycle `Create → Attach → Request → Approve → Authority exists` must appear verbatim in the UX, not just in the audit.

---

## C-1 Security Invariants

*Provisional IDs, challenged; only the description is normative. Enforcement column is `PROPOSED`.*

- **I-A1 `TargetCreationAuthority`** — `CreateTarget` is allowed only for principals authenticated to the tenant and never confers authority. A `proposed` target is not `active`, not warrantable, and not executable. *Why service + audit, not DB?* `principal` does not yet exist as a column (`db/001_schema.sql:60-76`); making `CreateTarget` fail-closed via service is sufficient for MVP, while DB-level `CHECK` for creation remains `OPEN` until `principal/resource` columns exist.
- **I-A2 `JustificationLinkAuthority`** — A `Justification` link may be created only for an existing `proposed` target and an existing, non-retracted belief, with an `obligation_key` that is in that belief's current obligation set. The link is `proposed` until the target is approved; it grants nothing by existing. Cross-belief/cross-item replay is impossible because the attestation/evidence payload binds `belief_id + obligation` inside the signed/verified payload (`docs/ADR/0001:175-178`). *Enforcement: service.*
- **I-A3 `TargetImmutability`** — An `active` target's tuple is immutable. Any field change, including scope, produces a different target. Justification set is immutable after `active`; late addition invalidates. *Enforcement: service now, future DB additive columns with no `UPDATE` path (only new-row = new-target) — additive-only discipline `docs/POST_HACKATHON_ARCHITECTURE.md:491-503`.*
- **I-A4 `ApprovalSeparation`** — `Approve` requires an authorized human/non-agent principal that is independently authenticated and, for `HIGH_IMPACT` consequences, is not the `Request` principal. An agent alone cannot make a target `active`. `Tenant-admin bootstrap` is the only human-only root that may provision the first approvers. *Enforcement: service + enterprise IdP; no kernel bypass; DB `active` without valid `approved_by` is the future impossible state, but `active` column does not yet exist so service is the gate.*

---

## C-3 Problem — Minimum Policy Validity

Solvent lets tenant policy determine `obligation set, obligation type, discharge qualification, potentially risk-related requirements` (`adv_plan_prompt.md`). Without a floor:

```
Customer policy → meaningless obligations → trivial discharge → promotion → AuthorityTarget → valid gate → wrong action
```

Example: `needProvenanceCheck` qualified as `any external_feed record`; attacker manufactures a weak external evidence row, belief promotes, high-impact target is created, `gate` passes, the system is internally consistent while semantically meaningless (`adv_plan_prompt.md`).

Tenant policy itself becomes an attack surface: the tenant's `CreateTarget` principal may define the obligation that `CreateTarget` satisfies — `target creator controls policy → defines obligation → discharges it → target authorized` (`adv_plan_prompt.md` circularity).

Therefore Solvent needs a **minimum safety contract beneath customer policy** — the difference between *configurable* and *unsafe*.

---

## C-3 Policy Safety Floor

### Always required — universal negatives (locked, not `OPEN`)

These are **never customer-overridable**, not even with “we explicitly want weaker”:

1. **No empty obligation sets.** Every new belief must enter with at least one obligation; a tenant configuration that yields zero obligations is invalid at write time (service). Evidence: a belief with no debt would be promotable on creation (`promoted_is_debt_free` `db/001_schema.sql:30` would immediately allow it), collapsing the entire debt idea to a no-op.
2. **No self-satisfying obligations.** An `obligation` whose `discharge_policy` is “`anything counts`” / `always satisfied` is invalid. The policy that defines obligations is itself validated; a tenant cannot define an obligation whose only qualifier is existence.
3. **No `any evidence qualifies` policy.** Similarly, a discharge rule of `any provenance_class` with no further qualification is invalid for the purpose of promotion. At minimum the rule must name an allowed `provenance_class` set, not the universal set.
4. **Deterministic obligations cannot be satisfied by unverified `external_feed` evidence alone.** `deterministic` (machine-verifiable, `docs/POST_HACKATHON_ARCHITECTURE.md:371`) requires `provenance_class ∈ {'reproducible_artifact','live_scan', …}` with **server-verified** content hashing (future `I-11`), never `external_feed` with caller-supplied `content_sha256` (`db/001_schema.sql:50` + `kernel/kernel.go:73` pass-through). Until server verification exists, deterministic obligations are satisfied only by `attested` or by a `reproducible_artifact` whose hash was independently fetched by Solvent — not asserted by the evidence producer.

*Where enforced:* service layer (policy validation) now, because obligation configuration does not yet have a DB table; future DB `CHECK` on `obligation_type` vs `provenance_class` membership is the promotable strengthening, but the rule is already testable without it.

### Candidate minimum for HIGH_IMPACT — explicitly `OPEN`

> For high-impact consequences, Solvent should require stronger evidence than deterministic evidence alone; `≥1 attested or quorum obligation` is the current **candidate** floor, **pending validation against a real high-impact customer use case**.

- **Not hard-coded as a universal kernel invariant yet.** Reason (per guidance): Solvent is general-purpose; some environments have genuinely stronger deterministic controls than human approval (e.g., hardware-attested reproducible builds). Hard-coding “human approval for every `HIGH_IMPACT`” would make Solvent a workflow gate, not an authority layer, for those customers.
- **Until validated:** keep the candidate as `OPEN` debt in the dogfood ledger and enforce the universal negatives above as the floor that already blocks the dangerous case (`unverified external_feed → deterministic discharge`). Any tenant that configures `HIGH_IMPACT` with only one deterministic obligation and no attested/quorum obligation is **allowed by the floor but flagged** in `solvent_explain` as `predicted to be weaker than the candidate floor` — visible, not blocked, until the candidate is promoted.
- **What is locked:** the *existence* of a stronger floor for high-impact will remain in the spec; its exact shape (`≥1 attested OR quorum`, vs `≥2 independent deterministic`, vs `attested only for funds_transferred`) is the `OPEN` part.

---

## C-3 Risk-Tier Requirements

Should minimum requirements depend on consequence class?

**Proposed (consistent with `docs/ADR/0001:113-134` fail-closed tiers, now applied as a policy floor):**

```
READ:
    no approval / no attestation — allowed, but scoped to resource
    (reading a patient record ≠ reading the cache)

DRAFT:
    no human attestation — allowed

LOW_RISK_EXECUTE:
    policy-dependent — at least the universal negatives (above);
    deterministic may be sufficient if I-11 provenance holds

HIGH_IMPACT_EXECUTE:
    stronger obligation floor — candidate ≥1 attested or quorum (OPEN);
    explicit human/non-agent Approve required (I-A4);
    scope/resource equality is still I-12 (same for all tiers)
```

- **Architecture:** keep risk tiers in **policy** (`Scope = …` + `obligation set` + `discharge_policy`), not as a kernel `risk_tier` column (the earlier plan correctly deferred `risk_tier TEXT` as `docs/ADR/0001:113` illustration only). The kernel sees `principal/resource/scope/action/consequence` and obligation types; the service maps `consequence → tier` via policy. This avoids hard-coding consequence taxonomy into the ledger (`docs/ARCHITECTURE/...:569` correctly avoids ontology).
- **Attack if tiers are kernel-hardcoded:** a new consequence type (`clinical_record_modified` vs `ephemeral_cache_state_modified`) would require a kernel migration to change its tier, coupling product policy to schema. Keeping tiers in policy keeps the kernel `Evidence → Belief → Obligation → Discharge → Target` generic.
- **Requirement on tier definition:** tiers must be **customer-visible enumerations with audit**, not free-text; tier assignment must itself be policy-controlled (see C-1 approver scoping).

---

## C-3 Obligation Qualification

Per `obligation_type` (`docs/ARCHITECTURE/...:319` `deterministic | attested | quorum`):

- **deterministic** — machine-verifiable. Minimum definition: `provenance_class` is **not** `external_feed` with caller-supplied hash; it is `reproducible_artifact`/`live_scan`/etc. with a hash that Solvent fetched or recomputed itself (future server verification, `I-11`). An `external_feed` row alone never discharges deterministic — universal negative above. `quorum` is not deterministic-by-aggregation; five outputs from the same model are one instrument. Minimum independence: different `source_url` prefixes and `provenance_class` diversity or cryptographic distinctness, not count alone.
- **attested** — independently attributable human/system attestation (`docs/ADR/0001:160-179`: bound to exact `(belief_id, obligation)` inside signed payload, attributable principal, independent channel, integrity proof, one attestation per debt item `docs/ADR/0001:425`). **Tenant may not configure `attested = agent-generated text`** — that redefines the type to mean nothing; the service must reject any policy that maps `attested` to `provenance_class=external_feed` with `attested_by=agent:`.
- **quorum** — `N independent sources` (`docs/POST_HACKATHON_ARCHITECTURE.md:371-377` direction). Minimum: `N` distinct principals **and** distinct provenance channels (not five outputs from one model). Until quorum has a customer, keep as direction, `OPEN`.

Each label is **meaningful only if the qualifier is enforced**. The service validates that the obligation's `discharge_policy` names a **restricted** `provenance_class` set, not the universal set.

---

## C-3 Self-Satisfaction / Circularity Rules

Every circular policy must be structurally impossible, not merely discouraged:

1. **Policy may not be self-satisfying.** `policy says agent may attest → agent attests → attestation satisfies policy` is circular. Rule: `attested` obligations are never satisfiable by `discharged_by` whose principal is `agent:` (or any principal that the obligation's own policy lists as its discharger without an independent attestation channel). The discharge's `attested_by` must be independently authenticated, not the same agent that requested.
2. **Evidence source may not define its own qualification.** `evidence source defines its own qualification → produces evidence → satisfies its own qualification`. Rule: `discharge_policy` is **policy-owned**, never `evidence`-owned (`docs/ARCHITECTURE/...:324` — owner is customer policy, never the benefiting agent; same for the source). The source that produced the row and the policy that qualifies the row are different principals.
3. **Target creator may not control the policy that makes their target trivial.** `target creator controls policy → defines light obligation → discharges it → authorized` (the most important for C-1/C-3 interaction). Rule: `tenant policy` that defines obligation sets and `discharge_policy` is writable only via the `Approve`-gated bootstrap/human-admin path (see `## Implementation Gate`), not via `CreateTarget` or `AttachJustification`. An agent that can `CreateTarget` cannot `WritePolicy` — different authority, different role, auditable as `policy_version_hash` in the target's approval record.

Violation of any circularity rule is a **policy-validation refusal** at write time, not a later audit finding.

---

## C-3 Policy Versioning

Policy needs: **version, effective_at, immutable historical snapshot, policy hash, author identity** — **YES for MVP** for the minimal production safety contract, but minimal.

- **What is stored:** `policy_version` (monotonic integer or content-hash), `effective_at` (when this version became the active floor), `policy_hash` (hash of the obligation-set + `discharge_policy` + risk-tier mapping that was evaluated), `author_principal` (who last changed the policy — a human/non-agent, never an agent). This is the smallest set that lets an auditor answer `Can an auditor reconstruct why an authority decision was allowed under the policy that was actually in force?`
- **Why not defer:** without it, a `belief promoted under Policy A` later re-read under `Policy B` (weaker) appears to satisfy `B`'s lighter floor, and a downgrade after obtaining authority (C-3 policy-change attack) is indistinguishable from legitimate authority. A mutable `current policy` row is insufficient — the target's `policy_version_hash` must be **snapshotted at approval** (as already required in the `Approve` audit row).
- **What is not needed for MVP:** full policy history UI, policy diff, or policy rollback beyond the deployment's own config rollback. The ledger of policy versions can be an append-only `policy_version` table with the hash and author; the full policy document may live outside Solvent as long as the hash is verifiable.

---

## C-3 Policy Change Semantics

```
belief promoted under Policy A
        ↓
tenant changes to Policy B (weaker)
        ↓
what happens to the already-promoted belief?
        ↓
what happens to already-active AuthorityTargets / warrants?
        ↓
what happens to new authorizations?
```

**Minimum safe semantics (locked, not `OPEN`):**

- **Belief remains `promoted` under the policy that promoted it** — `promoted_is_debt_free` is a *belief-local* CHECK (`db/001_schema.sql:30`), not a policy re-evaluation. A later weaker policy does not retroactively demote, and a later stronger policy does not retroactively invalidate a belief's `promoted` status alone. The belief's `status` is epistemic, not policy-recomputation.
- **Already-`active` AuthorityTargets remain `active` until reassessment, but `active` ≠ `permanently ALLOW`.** `RequestAuthorization`/`warrant verification` is the point that re-evaluates under **current** policy. An already-active target whose justification set would **fail the new, stronger** policy becomes `REVIEW`/`DENY` at next verification (new `Request` or warrant re-check), not automatically retracted. This mirrors `docs/ADR/0001:201-219` central verification: `Warrant is not authority; current verification decides`. The audit shows `policy_version_hash@approval ≠ current_policy_hash`, so the delta is visible.
- **Policy applies to new authorizations from `effective_at` forward.** Any `CreateTarget`/`Approve` after `effective_at` is evaluated under `Policy B`. Back-dating `effective_at` is forbidden (must be ≥ last version's `effective_at`); the service rejects it.
- **Downgrade after obtaining authority:** a customer that downgrades after `Approve` does **not** get stronger targets for free — each new target after downgrade is still `active` only if it meets the weaker floor, but the downgrade itself is an `Approve`-gated event (see Bootstrap: tenant policy write requires human/admin approval), is audited with `author_principal`, and is immediately visible to any auditor that replays `policy_version_hash` at approval. The *ability* to downgrade is not blocked (customer owns policy within the floor), but the *fact* of downgrade is attributable and reassessment will re-evaluate already-active targets at next use.
- **Upgrade invalidates?** A policy **upgrade** (stricter floor) does not invalidate `Belief.promoted`, but it does make already-active targets that would not meet the stricter floor **fail open at next verification** — which is why verification must run at execution time for `HIGH_IMPACT` (`docs/ADR/0001:113-134` fail-closed), not on a cached `ALLOW`.

**Customer strategy check:** a customer that weakens policy after `Approve` to make a future high-impact target easier has not bypassed the floor — the universal negatives (non-empty, non-self-satisfying, deterministic≠unverified `external_feed`) are still enforced even under weaker policy, and any `HIGH_IMPACT` candidate that relied solely on a now-permitted weaker obligation would still be subject to the `candidate ≥1 attested|quorum` flag in `solvent_explain` as `predicted to be weaker than candidate floor` until that candidate is promoted to a lock.

---

## C-3 Security Invariants

*Provisional IDs, challenged; only the description is normative. Enforcement column is `PROPOSED`.*

- **I-P1 `NonVacuousPolicy`** — No tenant policy is valid with an empty obligation set, an empty `discharge_policy` (`any evidence qualifies`), or a self-satisfying obligation. Any `CreateTarget` under such policy is refused at policy-validation time, not after promotion. *Enforcement: service (policy validation). Future DB `CHECK` on `obligation` rows when they exist.*
- **I-P2 `QualificationIntegrity`** — A `deterministic` obligation is never discharged by `external_feed` with caller-supplied `content_sha256` alone; an `attested` obligation is never discharged by `agent:` principal; `quorum` requires `N` *independent* sources (distinct principals + distinct provenance channels, not count). A `discharge_policy` that maps a type to a universal set is rejected. *Enforcement: service + future I-11 server-verified hashing.*
- **I-P3 `HighImpactMinimum` (candidate, OPEN)** — The candidate `HIGH_IMPACT ⇒ ≥1 attested or quorum` is the minimum **safety** floor but remains `OPEN` pending a real high-impact customer use case. Universal negatives of I-P1/I-P2 are enforced regardless; the candidate is enforced as `predicted_*` warning in `solvent_explain` until promoted. *Enforcement: service (policy) now, candidate DB constraint after validation.*
- **I-P4 `PolicyVersionBinding`** — Every `active` target is bound to `policy_version_hash` and `effective_at` at approval; any `RequestAuthorization`/warrant verification re-evaluates under **current** policy and records the delta. A mutable `current policy` row alone is insufficient for audit reconstruction. *Enforcement: service + append-only `policy_version` ledger.*

---

## C-1 + C-3 Interaction

**Attack 1 — weak policy + legitimate belief + attacker-created target (the most dangerous for MVP):**

```
1. Attacker controls an agent + ordinary tenant-level access (not bootstrap admin)
2. Attacker obtains a legitimate promoted belief (e.g., "customer identity was verified" — genuinely discharged via deterministic artifact before compromise)
3. Tenant policy is weak but formally valid: {needProvenanceCheck: any external_feed} — passes the universal negatives? No — this exact policy is **rejected by I-P1/I-P2** because `any external_feed` is “any evidence qualifies” and `deterministic` cannot be satisfied by unverified external_feed alone. So the attack already fails at policy-validation. Suppose instead the tenant has a *minimally valid* weak policy: {needProvenanceCheck: reproducible_artifact, needContradictionSweep: external_feed} — this *is* valid under the universal floor, but any HIGH_IMPACT target needing only these two deterministic obligations would be flagged by the candidate floor.
4. Attacker creates AuthorityTarget{principal=attacker-agent, resource=treasury-account, action=transfer, consequence=funds_transferred}
   → succeeds: CreateTarget is proposing only (`I-A1`), any authenticated principal may propose (matrix).
5. Attacker attaches the legitimate belief as justification
   → succeeds as a proposed link: AttachJustification is `proposed`, belief is promoted, obligation exists. The link is visible but not yet authority.
6. Tenant policy says target is eligible with just the two deterministic obligations
   → policy says eligible (if weak policy is minimally valid). This is the point where the candidate floor matters: if the target is HIGH_IMPACT and policy says `eligible with 0 attested`, the service would mark it eligible under universal negatives, but `solvent_explain` would flag `predicted to be weaker than candidate floor` and the Approve step is still required.
7. Solvent evaluates target (Request → Approve):
   → **Fails at Approve/Activate (I-A4):** the target is HIGH_IMPACT (`funds_transferred`), so Approve requires an **authorized human/non-agent principal distinct from the attacker agent**, in the `HIGH_IMPACT` approver set, with `I-A4` separation of duties. The attacker-agent calling Approve is refused (`isError true, predicted_*` not emitted — real service error, not DB SQLSTATE). The target remains `proposed`, never `active`, no warrant, no execution. The legitimate belief stays promoted — the belief was never the problem; the target's approver was.
```

**Precise failure boundary for Attack 1:** `Approve`, by `I-A4` (agent never approves `HIGH_IMPACT`) **and** by the universal floor (I-P1/I-P2) for any `any evidence qualifies` variant. The attack never reaches `AuthorityTargetMatch` (I-12) because it never becomes `active`. This is why `CREATE TARGET = YES` is safe: creation is not grant.

**Attack 2 — strong policy + unauthorized target/link creation:**

```
Strong policy: HIGH_IMPACT requires needOperatorSignoff with attested discharge; needProvenanceCheck with reproducible_artifact.
Legitimate target: {treasury-account, transfer, funds_transferred} is correctly created and approved.
Attacker (agent) creates a second target {treasury-account, transfer, funds_transferred} with a different justification link: the same legitimate belief but with a forged attestation whose payload does not bind (belief_id, obligation).

→ Fails at AttachJustification (I-A2): the attestation's signed payload does not contain the exact (belief_id, obligation) binding (`docs/ADR/0001:175-178`), so the DebtDischarge is refused by the attestation mechanism. Even if the attacker bypasses the attestation and calls RetireDebt directly, that path is the current weak path (`docs/ADR/0001:140-148`) and — under the proposed N1∧N2∧N3∧N4 promotion — the belief would not stay promoted without an accepted DebtDischarge, so the link's belief is no longer promoted at the Approve check (I-A2 per-link composite-FK half). The target remains proposed.
→ If the attacker instead creates a legitimate target correctly and then mutates it to treasury-account before execution, the exact-tuple check fails at I-12 (AuthorizationTargetMatch, docs/ARCHITECTURE/...:581) — tuple inequality, not the authorized target, refused.
```

**Precise failure boundary for Attack 2:** `AttachJustification` / `Approve` (I-A2/I-A4) for the forged attestation case; `I-12 AuthorizationTargetMatch` for the post-authorization mutation case. Strong policy alone is not sufficient without `I-A1..I-A4`; unauthorized link creation alone also fails without a weak policy. Hence:

> **Both trustworthy policy floor *and* trustworthy authority-object creation are required; neither compensates for the other.**

---

## Kernel / Service / Policy / Identity Responsibility

*Use `impossible→database, interpretation→service, enterprise identity→identity layer, policy→policy/service, state-machine→Lean` as the rule, but challenge where necessary.*

| Property | Enforcement (proposed) | Why |
|---|---|---|
| `promoted ⇒ debt empty` (N1) | **DB** `promoted_is_debt_free` `23514` (`db/001_schema.sql:30`) + Lean `promote_preserves_validity` | Impossible state, row-local |
| `live intent ⇒ promoted` (gate) | **DB** `gate` `23503` + `live_requires_promoted` `23514` + `ON UPDATE CASCADE` re-evaluation (`db/001_schema.sql:68-75`) | Row-local, already proven |
| `Justification link cites only promoted belief` (per-link half of I-9) | **DB** candidate: same composite FK shape as `gate` (PROPOSED, I-A2); Lean only after SQL exists | Per-link, row-local — provable |
| `Target valid ⇒ all justification links valid` (set-level AND of I-9) | **Service** now; DB `CHECK` OPEN | “Every row of my set satisfies P” not row-local |
| `Discharge ↔ array consistency` (I-P2/I-10, 1:1) | **Service** + append-only ledger now; `UNIQUE(belief_id, obligation)` on `accepted` candidate later | Correlating array + ledger across tables not row-local in MVP |
| `TargetImmutability` (I-A3) | **Service** now (no UPDATE path; new target = new row) | Additive-only discipline; DB `CHECK` for tuple immutability is future |
| `ApprovalSeparation` (I-A4, agent never approves HIGH_IMPACT) | **Service + Identity** (enterprise IdP asserts human/non-agent, service enforces `approver ≠ requester` for HIGH_IMPACT) | Authenticated principal class, not DB principal |
| `NonVacuousPolicy` (I-P1) | **Service** (policy validation at write) — future `CHECK` on obligation rows | Empty/any/self-satisfying is a configuration refusal, not a ledger state |
| `QualificationIntegrity` (I-P2) | **Service** + future `I-11` server-verified hash | `deterministic ≠ external_feed` is a policy/content check, not a FK |
| `HighImpactMinimum` (I-P3 candidate) | **Service** (policy) now, warned as `predicted_*` in `solvent_explain` until promoted | Candidate, not universal |
| `PolicyVersionBinding` (I-P4) | **Service + append-only policy_version ledger** (immutable snapshot) | History, not current-row `CHECK` |
| `Authorized tuple == presented tuple` (I-12 AuthorizationTargetMatch) | **Service** (server-side equality, idempotent `Warrant→Execution` 1:1 for MVP) + executor contract | Comparison, not ledger impossibility; world effect is I-13 |
| `World effect == presented tuple` (I-13 ExecutionEffectMatch) | **Deferred** (trusted executor / signed receipt) | Outside Solvent's verification boundary today |

**Smallest kernel to make unsafe authority impossible:** keep the three existing DB invariants plus the two proposed per-link/per-discharge row-local checks (`Justification→Belief` composite FK, `Discharge` unique on `accepted`) — everything else stays service/policy/identity with Lean only after the DB constraint exists (per `mvp_plan2_review.md:12` “if security-critical, prefer DB”).

---

## DocTrust Impact

*Do not implement DocTrust; state its contract.*

- **What DocTrust must prove:** that a *separate* application with a *separate* database can: `provide evidence` via ingestion (not shared kernel), `create/request` beliefs/targets through `MCP/REST` public interfaces (`mvp_plan2.md:193`), `obtain review` via the attestation lifecycle (human/non-agent approve, not agent self-approve), `ask` via `solvent_explain` (`docs/FOUR_VERB.md`), `request` and receive an authoritative `DENY/ALLOW/REVIEW` that reflects the current ledger + current policy, and `act only after Solvent's decision`. It must obey the four-role matrix and the published discharge policies; it must never `import Solvent internals`, `write Solvent tables directly`, `bypass Solvent for consequential actions`, or `reimplement Solvent authorization` (`mvp_plan2.md:888`).

- **What DocTrust cannot prove:** domain-agnostic generality (document workflow tests provenance attestation but not, e.g., cloud-infrastructure `funds_transferred`), evidence-poisoning resilience (its evidence is controlled fixtures), quorum or temporal scope, or `I-13` execution-effect. The `customer identity → view/issue refund/transfer` 1→3 anti-broadening demo must be exercised separately to prove `promoted belief ≠ permission` for the same belief across resources.

- **What integration contract it must obey:** same tuple the executor will present — `principal/resource/scope/action/consequence` exact equality (`I-12`), `scope never broadens resource`, `unspecified ≠ wildcard`, `target immutability` (new target on mutation), `agent may create/request but not approve`, tenant policy is **versioned** (`policy_version_hash` snapshotted at `Approve`), and any `HIGH_IMPACT tool` execution verifies that the presented tuple `==` the approved tuple before Bets.

- **What test scenarios should exist (minimum, proposed):**
  ```
  legitimate target creation → attach promoted belief → request → denied (not yet approved)
  unauthorized target creation (anonymous agent) → refused at CreateTarget or remains proposed
  legitimate belief + unauthorized link (attacker attaches to treasury target) → link refused (I-A2) or target never activates (I-A4)
  weak policy proposed (empty / any-evidence / self-satisfying) → policy validation refused at write (I-P1/I-P2)
  high-impact action requiring stronger proof (deterministic-only policy) → explain warns candidate floor, Approve refuses until attested discharge added
  ```

---

## Open Architectural Questions

Only genuinely unresolved questions (each blocks its schema decision):

1. **Target/link creation policy granularity** — exact shape of `allowed_principals` per `resource`/`consequence` prefix for MVP: a full `principal→resource` ACL table vs a short allow-list in tenant config (`OPEN #8` incarnation).
2. **Scope dimensions final vocabulary** — which constraint dimensions are canon for `Scope` equality (tenant/environment/region/time-window/field) and their comparison semantics; time depends on deferred temporal authority.
3. **Attestation key/identity provisioning** — exact provisioning mechanism for bootstrap human-admin keys vs managed keys vs enterprise-identity-backed vs WebAuthn (`docs/ADR/0001:187` choices remain `OPEN`).
4. **Evidence artifact vs citation table shape** — `artifact` + `artifact_citation` vs alternative, prerequisite to `I-11` (`OPEN #13`).
5. **Debt array convergence** — whether to keep transitional `debt[] + ledger` dual truth or migrate to preferred `obligation rows → discharge rows → derived debt` while preserving `promoted_is_debt_free` (`OPEN #14`).
6. **N4 placement** — fourth promotion conjunct vs target-level `Authority Eligibility` (`OPEN #16`).
7. **Execution-effect scope** — whether `I-13 ExecutionEffectMatch` belongs in MVP via receipts or stays deferred; where the verification boundary ends and the trust boundary begins.

---

## Self-Attack Results

*For every major rule, disposition ∈ {`blocked by invariant` / `blocked by service contract` / `blocked by policy floor` / `blocked by identity` / `detected by audit` / `deferred` / `unresolved`}.*

| # | Attack | Disposition |
|---|---|---|
| 1 | Attacker with ordinary tenant access + agent + legitimate promoted belief creates a high-impact `AuthorityTarget` (treasury `transfer`) and attaches the belief | **Blocked by policy floor** (`I-P1`/`I-P2` for any `any evidence qualifies` variant) **and** **blocked by identity** (`I-A4`: `HIGH_IMPACT` requires human/non-agent approver ≠ attacker; agent never approves). Target stays `proposed`; `I-12` never reached. See `C-1+C-3 Interaction` Attack 1. |
| 2 | Attacker replays a valid attestation for a second `(belief_id, obligation)` | **Blocked by invariant/attestation** (`DebtDischarge` payload binds `belief_id+obligation` inside signed payload, cross-use is mismatch). Revoked replay: **deferred** to lifecycle (F-05) / `PolicyVersionBinding`. |
| 3 | Attacker uses same artifact/citation to discharge two obligations | **Blocked by invariant** (`I-P2` 1:1: one discharge → one obligation; shared instrument only via distinct citations with explicit policy). CURRENT `mapping.go:20` does the opposite — scored `Open` in ledger until I-11. |
| 4 | Attacker splits `needOperatorSignoff` into `Part1..6` trivial sub-obligations | **Blocked by policy floor** (`I-P1` minimum-validity rejects sub-obligation sets that are not the tenant's declared set; audit shows split as visible `CreateTarget`). Self-graded tenant still **unresolved** (OPEN #9). |
| 5 | Compromised evidence source forges artifact that retires a deterministic obligation | **Blocked by policy floor** (`I-P2` deterministic ≠ unverified `external_feed`; server-verified hash required, I-11). Until I-11 exists: **deferred** / `detected by audit` (fixture-only ingress) — scored `Open`. |
| 6 | Migration `NULL → wildcard` (`resource=''` treated as `*`) broadens old rows | **Blocked by invariant** (fail-closed: `NULL/missing ⇒ NOT AUTHORIZED`, tested by `TestMigration_OldActionIsNotOverAuthorized`), plus `I-A3` immutability. |
| 7 | Concurrent `Promote` vs `RetractCascade` vs `Approve` creates live on retracted | **Blocked by invariant** (existing `gate`/`live_requires_promoted`/`RetractCascade` atomicity, `40001` retries) for old semantics; proposed target-level validity `all links promoted` is **blocked by service contract** until row-local set-level `CHECK` exists (`OPEN` #7). |
| 8 | Stale warrant replayed after belief retraction | **Blocked by service contract** (central verification invalidates immediately) + **blocked by policy floor** (re-verification under current policy); caching beyond verification **deferred** (`review I-12`). |
| 9 | Scope silently broadens (`environment=production` → all production resources) | **Blocked by invariant** (`Scope narrows-only; unspecified ≠ wildcard` per Q2 lock, I-12). Enforcement representation still `OPEN` but rule is locked. |
| 10 | Weak discharge → strong authority (single attestation covers `deploy staging` and `deploy production`) | **Blocked by invariant** (Decision B: explicit per-target `Justification` links + `AND`; `authority strength ≤ justification strength` by construction, target immutability). |
| 11 | Buggy executor `authorized A` but executes `B` (`patient/999 delete` vs `patient/123 modify`) | **Blocked by service contract** (`I-12 AuthorizationTargetMatch`: presented ≠ authorized → refused). Residual compromised executor that presents correctly then does otherwise: `unresolved` until I-13 receipts (deferred). |
| 12 | Customer self-approves via delegated workload that is really the same agent | **Blocked by identity** (`I-A4` `approver ≠ requester` for `HIGH_IMPACT`, enterprise-IdP asserts `principal_type ≠ agent`). Policy that lists `agent` as `HIGH_IMPACT` approver is rejected by service (I-A4). |
| 13 | Policy downgrade after `Approve` to make future targets easier | **Detected by audit** (`policy_version_hash` snapshotted at approval ≠ current) + **blocked by service contract** on *new* approvals (re-evaluated under current policy); universal floor still enforced. `PolicyVersionBinding` (I-P4) makes it attributable. |
| 14 | Link-sprawl (thousands of belief→target links) creates dangerous breadth | **Detected by audit** (all links are explicit rows, enumerable). Preventive `who may link` control is **blocked by identity** / `I-A2` creator check (`OPEN #8` as the four-role gate). |

---

## Dogfood Evidence Ledger

*Strict rule: future design claims without repository/use-case evidence are `OPEN`. Only `Discharged` has implementation, measured behavior, or an accepted-and-cited decision **as a decision**.*

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Existing kernel 4 tables + `promoted_is_debt_free` / `gate` / `live_requires_promoted` are DB-enforced | `db/001_schema.sql:34` `belief_id_status_key`, `:68` `live_requires_promoted` `23514`, `:74` `gate` `23503` + `kernel/kernel_test.go:249` B-09, `internal/view/explain_test.go` | High | — | **Discharged** |
| `solvent_explain` is read-only and never invents SQLSTATEs | `internal/view/explain.go` `predicted_*` vs real `23503`/`23514`; `Taskfile.yml:224` MCP boundary grep no `INSERT`/`UPDATE` in `internal/view`/`cmd/solvent-mcp`; `scripts/mcp_verify.sh` 7 tools deterministic | High | — | **Discharged** |
| Agent output is not authority — `gate` is the authority boundary | `docs/ADR/0001:99` Decision 1 central verification + `README.md:111` “The database decides” | High | — | **Discharged** |
| Bootstrap recursion terminated at human tenant-admin | Repository has **no** bootstrap table yet; inference from design principle + enterprise identity outside kernel (`docs/ADR/0001:499`) — principle is argument | Low | Provisioning mechanism not specified; no `bootstrap_admin` table | **Open** |
| “Same principal can perform everything” (Model A) is safe | **Contradicted** — `Mvp_plan2_review.md:15` moved-broadening attack (`Belief A → Target("move $10M")`) demonstrates that legitimate belief + attacker-created link becomes authority if no creation/approval separation. DocTrust must not share kernel and must use MCP/REST. | Low | — | **Contradicted** |
| “Target creator and approver may differ optionally” (Model B weak) is safe | **Contradicted** for `HIGH_IMPACT` — Review #2 requires `HIGH_IMPACT` ⇒ human/non-agent approver ≠ requester; Model B optional fails `Self-Attack #1` | Low | — | **Contradicted** |
| “Policy-controlled roles” (Model C) alone without universal floor is safe | **Contradicted** — weak policy `any evidence qualifies` would pass Model C if customer so configures; requires universal `I-P1`/`I-P2` floor to be safe (see `C-3 Policy Safety Floor`). | Low | — | **Contradicted** |
| “Existing authority required to create a new target” (Model D) solves bootstrap | **Contradicted** as infinite recursion (`adv_plan_prompt.md:301`); requires human bootstrap exception, which is `I-A4`'s human-root. Model D without bootstrap is a livelock. | Low | — | **Contradicted** |
| `CreateTarget`/`AttachJustification`/`Request`/`Approve` as four distinct transitions | Design reasoning in `## C-1 Four-Role Model`; no `AuthorityTarget` table exists (`db/001_schema.sql` has no such table) | Medium | Who may create/attach/approve (I-A1/I-A2/I-A4 `OPEN #8` incarnation) | **Open** |
| Target immutability — exact tuple equality, no in-place mutation | Design reasoning `## C-1 Target Immutability` + additive-only discipline `docs/POST_HACKATHON_ARCHITECTURE.md:491-503` | Medium | No tuple column exists | **Open** |
| `NonVacuousPolicy` universal negatives (`I-P1`) are sufficient to block meaningless `debt=[]` | Design reasoning from `F-04` checkbox theater; consequences demonstrated with synthetic `operator_asserted` rows; no `obligation` table yet | Medium | `obligation` table & policy-validation implementation `OPEN` | **Open** |
| `QualificationIntegrity` (`I-P2` deterministic≠unverified `external_feed`) | Design reasoning; current `kernel.go:73` pass-through `content_sha256` shows why verification is needed; I-11 deferred | Medium | Server-verified hash implementation `Deferred` | **Open** |
| Candidate `HIGH_IMPACT ⇒ ≥1 attested|quorum` (`I-P3`) | **Candidate**, `OPEN` pending real high-impact customer (locked choice 2) | Medium | Customer use-case validation | **Open** |
| `PolicyVersionBinding` (`I-P4` version/hash/snapshot) | Design reasoning from audit requirement `adv_plan_prompt.md`; no `policy_version` ledger today | Medium | Ledger schema `OPEN` | **Open** |
| Central Warrant verification remains authoritative | Accepted decision `docs/ADR/0001:95` `Warrant is not authority` | High (as decision) | — | **Discharged** (as decision) |
| MCP Elicitation is channel, not signature | Accepted decision `docs/ADR/0001:187` bounded PKI; `internal/view/explain.go` `predicted_*` | High (as decision) | — | **Discharged** (as decision) |
| Four-verb slice `Connect→Ask→Authorize→Reassess` is legible and DB-enforced | `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh` 10 steps green, `mcp_verify.sh` 7 tools | High | — | **Discharged** |
| Evidence ingress hardening (`I-11` server-verified hash, replay) | Not begun; fixture-only (`internal/pipeline/pipeline.go:41-57`) | — | Gateway design (F-02) | **Deferred** |
| Execution-effect verification (`I-13`) | Not in scope; receipts/trusted executor deferred | — | Post-execution attestation | **Deferred** |
| Temporal scope/freshness (`N5`) | `docs/ADR/0001:543` defers validity-horizon machinery | — | `validity_horizon`/`expires_at` machinery | **Deferred** |
| Warrant as portable reference, not second source of truth | Accepted decision `docs/ADR/0001:479` | High (as decision) | — | **Discharged** (as decision) |

---

## Implementation Gate

Exact conditions that must be met before `AuthorityTarget` schema design, policy schema design, or kernel changes may begin (`PROPOSED — NOT IMPLEMENTED` stays until all hold):

1. **This C-1/C-3 specification is promoted through Adversarial Review #3** — the review that attacks the bootstrap root, target/link creation authority, policy downgrade path, and whether “agent may create target but not activate it” is enforceable rather than contractual, and whose verdict is `APPROVE` or `APPROVE WITH CONDITIONS` with those conditions matching this gate.
2. **Every `OPEN` in `## Open Architectural Questions` is either resolved, or explicitly bounded out of the first implementation phase with its deferral recorded as debt** (exactly as `docs/ARCHITECTURE/...` does for `OPEN #1..#13`).
3. **Every proposed invariant (`I-A1..I-A4`, `I-P1..I-P4`, `I-9..I-13`) has an assigned enforcement layer** (`Lean` / `SQL` / `Service` / `Policy` / `Identity`) with rationale — no invariant left “somewhere.”
4. **Bootstrap trust root is provisioned as `human-only tenant-admin via deployment config`, with the exact mechanism `PROPOSED` but the principle locked** — an AI agent cannot bootstrap (`## C-1 Bootstrap Trust Root`), and any change to bootstrap admins is itself an `Approve`-gated event.
5. **Policy floor universal negatives are specified as `I-P1` + `I-P2` and are test-enforceable at the service layer** (empty/self-satisfying/`any evidence` rejected; `deterministic` rejects unverified `external_feed`), ready to become `CHECK`s when `obligation` rows exist.
6. **Agent vs human matrix is written as the integration-facing contract** (`docs/ARCHITECTURE/...:415` four-role matrix) and is enforced at the service layer before any `HIGH_IMPACT` `Approve` (`I-A4`).
7. **Executor contract is written before any executor integration claims `I-12`** (`AuthorizationTargetMatch`: exact tuple equality, no mutation, `active` target immutable, one decision → one tuple, fail-closed on `unavailable`).
8. **Dogfood ledger is re-scored after Review #3:** nothing may move to `Discharged` without repository/use-case evidence.
9. **Exact files changed:** `git status --short` shows only `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md` (and its review) — no `Go`/`SQL`/`Lean`/`MCP`/`REST`/`ADRs`/`authority-target-and-debt-discharge-spec.md` edits.
10. **No commit, no sync:** the spec is the gate; implementation is not.

---

## Important Distinctions

Do not confuse:

```
creating a target ≠ attaching a justification ≠ requesting authorization ≠ approving authorization ≠ executing authorization
customer policy flexibility ≠ permission to make Solvent meaningless
auditability ≠ prevention
promoted belief ≠ permission
exact tuple equality (I-12 AuthorizationTargetMatch) ≠ proof of execution (I-13 ExecutionEffectMatch)
central verification ≠ replay/concurrency solved
```

Do not use a second source of truth. `belief.debt[]` (current CHECK truth) + future `DebtDischarge` row truth remains `OPEN` dual truth until rows-derived is chosen — but `promoted_is_debt_free` `23514` remains the impossibility that survives the migration.

---

## Most Important Adversarial Test

Try to make this attack succeed:

```
1. Attacker controls an AI agent.
2. Attacker has ordinary tenant-level access (not bootstrap admin).
3. Attacker obtains a legitimate promoted belief ("customer identity was verified").
4. Attacker creates or modifies an AuthorityTarget (treasury transfer).
5. Attacker attaches the legitimate belief as justification (explicit link).
6. Tenant policy is weak but formally valid (passes universal floor I-P1/I-P2: e.g., {reproducible_artifact for needProvenanceCheck}).
7. Solvent evaluates the target (RequestAuthorization).
8. Solvent returns ALLOW.
9. Attacker causes a HIGH_IMPACT consequence.
```

**If your design cannot identify a precise point where the attack fails, the design is not complete.**

**Where this attack fails in the proposed design — exactly:**

- **`Attach` or `Approve`, not `Create`.** `CreateTarget` succeeds as `proposed` (any authenticated principal may propose, `I-A1`), but the target is **not authority**. `AttachJustification` proposing the link succeeds as `proposed` (visible audit row). **Failure is at `Approve/Activate` (`I-A4`):** the target is `HIGH_IMPACT` (`funds_transferred`), so `Approve` requires an **authorized human/non-agent principal in the `HIGH_IMPACT` approver set, distinct from the attacker agent**, whose principal is enterprise-IdP-attested (`I-A4` + bootstrap). The attacker-agent calling `Approve` is refused with a service error (`predicted_*` not emitted — real `I-A4` check). The target stays `proposed`, no warrant, no `I-12` verification. Strong conclusion: `proposed target + legitimate belief + weak-but-valid policy` is **never `ALLOW`** without human approval.

**Then test the inverse:**

```
strong policy (HIGH_IMPACT requires ≥1 attested per candidate floor)
    +
unauthorized target/link creation (agent forges attestation by calling RetireDebt directly)
```

- **Fails at `Discharge` and `Approve`.** The forged `DebtDischarge` payload does not bind `(belief_id, obligation)` inside the attestation (`I-A2`, `I-P2`), so `AttachJustification` is refused or `Approve` finds `N2` not satisfied (`N1∧N2∧N3∧N4` proposed). Even with strong policy, the unauthorized link cannot make `belief.promoted` suffice for authority (Decision A: `promotion ⇒ eligibility only`). The failure boundary is **identical**: `Approve` checks `N2` (one accepted discharge per obligation) and `I-A4` approver, not just policy strength.

---

## Final Rule

This is a production authority system. The purpose of this document is not to maximize configurability, minimize tables, or make the architecture aesthetically elegant. It is to answer:

> **Who is allowed to manufacture authority, and what is the minimum safety floor below which Solvent refuses to call something authorized?**

**Answers (locked as PROPOSED, adversarially challenged):**

- **Manufacturing authority:** only a **human-only bootstrap admin** can provision approvers; only an **authorized human/non-agent distinct from the requester** can `Approve` `HIGH_IMPACT`; the **agent may propose but never approve**, and proposing is not authority.
- **Minimum floor:** `I-P1` universal negatives (non-empty, non-self-satisfying, no `any evidence qualifies`, `deterministic ≠ unverified external_feed`) are **locked as service refusals**; the candidate `HIGH_IMPACT ⇒ ≥1 attested|quorum` stays `OPEN` pending a real high-impact customer, flagged as `predicted to be weaker than candidate floor` in `solvent_explain` until promoted.

Be skeptical. Challenge your own recommendations. If you cannot support a claim with repository evidence (`db/001_schema.sql:50` belief-owned, `:65` caller-asserted `action`, `kernel/kernel.go:73` hash pass-through) or a concrete use case (the 1→3 anti-broadening `customer identity → view/transfer` demo), **mark it `OPEN`**.

---

*Report:*

```
exact files changed
  docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md  (new, PROPOSED)
git status --short
  ?? docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md
  ?? docs/REVIEWS/  (pre-existing, not part of this change)
whether any implementation file changed
  No — no Go, SQL, Lean, MCP, REST, or ADRs changed; no schema; no tables; authority-target-and-debt-discharge-spec.md untouched
unresolved blockers
  OPEN #1 Scope vocabulary, OPEN #3 consequence taxonomy, OPEN #5 principal delegation chain, OPEN #6 N2 schema enforceability, OPEN #7 set-level AND, OPEN #8 four-role incarnation (target creation policy shape), OPEN #9 vacuous-policy defense, OPEN #13 artifact/citation table, OPEN #14 debt rows, OPEN #16 N4 placement, candidate I-P3 HIGH_IMPACT floor pending customer, I-13 execution-effect deferred, attestation key lifecycle (F-05), temporal N5
implementation gate status
  Gate is for the spec itself — it must survive Adversarial Review #3 attacking the four items above before any AuthorityTarget/policy schema, kernel changes, or DocTrust integration may begin.
recommended next gate
  Adversarial Review #3, scoped to the four bullets above: bootstrap trust root, target/link creation authority (agent may create but not activate — enforceable?), policy downgrade (P_A→P_B weaker), and whether exact-tuple equality is enforceable rather than contractual.
```

Only created: `docs/ARCHITECTURE/authority-creation-and-policy-floor-spec.md`. No other changes. Do not commit. Do not sync.
