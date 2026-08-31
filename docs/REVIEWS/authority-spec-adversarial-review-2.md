# Authority Target and Debt Discharge Specification —
# Adversarial Review #2

## Review posture

The specification under review (`docs/ARCHITECTURE/authority-target-and-debt-discharge-spec.md`) is treated as a belief that must earn promotion. It has not yet been implemented. Nothing in this review modifies Go, SQL, Lean, MCP, REST, ADRs, or the specification itself. The repository is source of truth; where the specification and repository disagree, repository wins.

## Executive verdict

**APPROVE WITH CONDITIONS**

The specification is structurally stronger than the production MVP plan it supersedes. It correctly separates epistemic promotion from authority, introduces an auditable justification boundary, and preserves the existing `promoted_is_debt_free` / `gate` / `live_requires_promoted` invariants as frozen. The five-tuple target, the M:N explicit-link model, and the two-boundary executor contract (`I-12` authorization vs reserved `I-13` execution-effect) are the right direction.

However, the specification leaves four security boundaries unresolved in a way that would allow an implementation to ship an authority system that can be exercised incorrectly without any schema-level detection. Those boundaries are:
1. link/target/justification creation authority (`OPEN #8`),
2. N2 enforcement layer (`OPEN #6`/`#7` — discharge-array consistency is service-layer only),
3. vacuous-policy defense (`OPEN #9`), and
4. executor contract as an integration boundary (no written contract exists).

None of these require kernel redesign, but all four must be closed before any production kernel or schema work begins. The conditions are listed explicitly in `## Conditional approval requirements`.

## What Review #1 fixed

| Finding from Review #1 | Disposition in this specification |
|---|---|
| Evidence Artifact vs Evidence Citation | **Closed.** Semantics are locked. Representation (`OPEN #13`) is an implementation choice, not a semantic gap. |
| `AuthorizationTargetMatch` vs reserved `I-13` `ExecutionEffectMatch` | **Closed.** The specification explicitly separates the authorization boundary from the execution boundary (`## Executor Binding`). |
| Link/target creation authority is a security boundary | **Moved, not closed.** `OPEN #8` records the four-role decomposition (`create target / attach belief / request authorization / approve`) as a prerequisite, but no enforcement is proposed. |
| Debt array + discharge ledger convergence | **Direction locked, representation open.** Preferred long-term direction (`Obligation rows → DebtDischarge rows → derived debt`) is stated. Exact schema mechanism is `OPEN #14`. |
| N4 placement contested | **Recorded as `OPEN #16`.** The specification conservatively keeps N4 inside the promotion conjunction but records the alternative (`Authority Eligibility` as target-level). |

## Critical findings

### F-2-1 — CRITICAL — Link and target creation is itself an authority operation with no proposed boundary

- **ID:** F-2-1
- **Severity:** CRITICAL
- **Title:** Target and justification-link creation can manufacture authority
- **Claim being attacked:** "M:N via explicit justification links" (`## Cardinality`) is the only model where every edge is a row someone created on purpose and an auditor can enumerate.
- **Evidence:** `OPEN #8` explicitly records that who may create a target, attach a belief, request authorization, and approve the target is unresolved. The specification states "preventive enforcement … is **OPEN #8** — four-role decomposition is the prerequisite" and defers it to schema design. The moved attack from Review #1 is recorded as `Self-Attack #15 (unresolved — moved broadening)`.
- **Attack scenario:** An attacker with the ability to create targets or links creates a legitimate-looking `AuthorityTarget("transfer funds, account=treasury_acme, consequence=funds_transferred, amount=high")` and attaches a single legitimate promoted belief ("customer identity was verified"). The target now has an AND justification set that is technically satisfied, and the system would authorize a high-impact consequence that the single belief does not actually justify. Audit visibility is not prevention.
- **Why it fails:** "Detected by audit" is not a preventative security control. A manufactured target that carries a valid justification set is indistinguishable from a legitimate target at the authorization boundary. The attack does not require breaking any invariant; it requires only the ability to create rows in tables that do not yet exist.
- **Recommended resolution:** Before any schema work, resolve the four-role decomposition: who may create a target, who may attach a justification link, who may request authorization, and who may approve. At least the target-creation and link-attachment roles must be gated by an existing authority (e.g., a policy-defined role or a pre-existing target). The simplest safe default is: target creation requires an existing authority-bearing operation; link attachment requires the target creator or a designated operator.
- **Required before kernel/schema work? YES**

### F-2-2 — HIGH — N2 is service-enforceable but not schema-enforceable as stated

- **ID:** F-2-2
- **Severity:** HIGH
- **Title:** Discharge-array consistency (N2 / I-10) cannot be enforced by schema in the proposed dual-representation model
- **Claim being attacked:** "No discharge without array change, no array change without discharge" (`## Promotion Preconditions`); "The array and the ledger cannot disagree in a committed state."
- **Evidence:** The specification itself acknowledges: "Rule 1 as a *schema* enforcement means a CHECK or trigger correlating `belief.debt` mutations with discharge rows — row-local SQL cannot cheaply assert 'this `array_remove` has a matching accepted discharge in this transaction'." The preferred long-term model (`Obligation rows → DebtDischarge rows → derived debt`) would make this enforceable, but the current transitional model keeps `belief.debt[]` as the CHECK truth and the discharge ledger as justification truth. The specification records this as OPEN but does not bound the transitional risk.
- **Attack scenario:** A service-layer bug or a compromised caller retires a debt item via `array_remove` without creating a corresponding `DebtDischarge` record (or creates a discharge record without retiring the array item). The two truths disagree. The schema CHECK (`promoted_is_debt_free`) sees only `debt=[]` and permits promotion. Audit visibility is post-hoc; the authority has already been granted.
- **Why it fails:** In the transitional model, N2 lives at the service layer and is *auditable*, not *impossible*. The specification is honest about this, but honesty is not enforcement. A production implementation that treats "auditable" as sufficient will recreate F-04 with more vocabulary.
- **Recommended resolution:** Either (a) make the transitional period short and bounded with an explicit migration test that proves the dual-truth consistency, or (b) accept that N2 is service-enforced during the transitional period and add a service-layer atomicity test that proves the discharge+array write is a single transaction. The specification should name which path it takes.
- **Required before kernel/schema work? YES**

### F-2-3 — HIGH — Vacuous-policy defense is unresolved (OPEN #9)

- **ID:** F-2-3
- **Severity:** HIGH
- **Title:** Customer-defined obligation sets can make Solvent semantically meaningless
- **Claim being attacked:** "The obligation set for a belief is owned by policy (tenancy configuration), not by the agent" and "minimum-validity rules are OPEN" (`## Debt and Discharge Semantics`).
- **Evidence:** `OPEN #9` records "minimum-validity rules for tenant obligation sets (the customer-defines-meaningless-obligations attack remains unresolved)." The specification explicitly lists attacks: empty obligations, self-satisfied obligations, policy that declares any evidence sufficient. No minimum semantic contract is proposed.
- **Attack scenario:** A customer configures their policy with a single obligation: `needProvenanceCheck` discharged by any `external_feed` evidence row with no source verification. The agent manufactures a synthetic evidence row, retires the obligation, promotes the belief, and creates an `AuthorityTarget` for `delete production database`. The system is semantically correct (debt-free, promoted, justified) and operationally wrong.
- **Why it fails:** The specification says "Do not assume customer configuration can be trusted merely because the customer is the customer" but does not propose a minimum-validity rule. Without one, the system's correctness is bounded by the weakest customer policy.
- **Recommended resolution:** Define a minimum-validity contract at the service layer before any production policy integration: at least one obligation must be `attested` or `quorum` type; `deterministic` obligations cannot be discharged by `external_feed` alone; a target for `HIGH_IMPACT` consequences must require at least one `attested` obligation. These are policy-layer rules, not kernel rules, but they must be named before the policy layer is exposed.
- **Required before kernel/schema work? YES**

### F-2-4 — HIGH — Executor contract exists as prose but not as a verifiable integration contract

- **ID:** F-2-4
- **Severity:** HIGH
- **Title:** I-12 executor binding has no written integration contract
- **Claim being attacked:** "The executor MUST present the full target tuple — verbatim, no normalization — for verification before executing" (`## Executor Binding`).
- **Evidence:** The specification lists five executor contract conditions but does not produce a written contract document, protocol specification, or executable test that an executor integration must satisfy. `## Recommended Next Decisions` lists "Executor contract document — the five conditions of `## Executor Binding` written as the integration-facing contract before any executor integration claims I-12" as decision 7. No such document exists.
- **Attack scenario:** An integration team reads the prose, interprets "exact tuple equality" loosely, and implements normalization (lowercasing action, trimming resource, defaulting empty scope to wildcard). The executor then presents a normalized tuple that passes Solvent's verification but executes a different consequence. Or the executor skips the pre-execution verification call entirely under high load, because the contract is prose, not a protocol requirement.
- **Why it fails:** A contract that exists only as prose in a specification document is not enforceable. The specification correctly identifies this as a required decision but does not produce the artifact.
- **Recommended resolution:** Produce the executor contract as a standalone document before any integration claims I-12. The document must specify: (1) the exact API call the executor must make before execution, (2) the exact tuple format (canonical serialization), (3) the exact failure behavior when Solvent returns DENY/REVIEW/UNAVAILABLE, and (4) the exact retry semantics.
- **Required before kernel/schema work? YES**

## Domain-model verdict

| Object | Definition | Current evidence | Proposed semantics | Open ambiguity | Security consequence |
|---|---|---|---|---|---|
| Evidence | Row in `evidence` table, belief-owned | `db/001_schema.sql:47-58`; `content_sha256` caller-supplied; `provenance_class` enum | Artifact vs Citation split | `OPEN #13`: table representation; whether artifact identity includes content hash; whether provenance attaches to artifact or citation | If artifact identity is not content-hashed, an external source can republish a modified artifact under the same ID, and citations survive the modification. |
| Evidence Artifact | The thing actually observed | `db/002_corpus.sql:6-9` corpus_issue is the closest analogue; no ledger-level artifact table exists | Shared, reusable object | Which table; whether `content_sha256` is server-verified; whether deduplication creates aliasing | Server-side hash verification is deferred to I-11. Until then, artifact identity is caller-attested. |
| Evidence Citation | Why an artifact is relevant to a specific belief | `belief_corpus_citation` (`db/002_corpus.sql:76-83`) is the closest analogue but is corpus-scoped, not ledger-scoped | Per-belief relevance claim | Whether a citation can carry a claim type (considered vs contradicts); whether one citation can discharge multiple obligations | The specification correctly says "records are never shared" — this is the strongest part of the artifact/citation design. |
| Belief | Epistemic proposition with lifecycle | `db/001_schema.sql:6-35`; six-item debt; `status ∈ {entered,promoted,retracted}` | No column changes proposed | None semantically | Clean. |
| Obligation | Typed condition attached to a belief at entry | `kernel.FullDebt` (`kernel/kernel.go:31-34`) is the current fixed vocabulary; no per-obligation row exists | Policy-owned, typed (`deterministic/attested/quorum`) | Whether obligation identity is the debt-item string or a row ID; whether a policy can redefine obligations after entry | If obligation identity is the string only, a policy change that renames a debt item retroactively changes the meaning of existing discharges. |
| DebtDischarge | Append-only fact that one obligation was satisfied | No table exists today. No discharge record of any kind. | `discharge_id, belief_id, obligation_key, instrument_kind, instrument_ref, discharged_by, issued_at, recorded_at, status, integrity` | Schema representation; whether `discharged_by` is a principal reference or a full identity row; whether `integrity` is a signature or a hash | If `discharged_by` is not attributable, the discharge is decoration. The specification correctly requires attribution, but the current codebase has no principal column anywhere. |
| AuthorityTarget | Five-tuple: principal, resource, scope, action, consequence | No table or column exists. `action_intent` carries only `belief_id, belief_status, action, state`. | Separate consequential object | All five dimensions have open sub-questions (principal representation, resource opacity, scope dimensions, action namespace, consequence taxonomy) | The five-tuple is the right shape, but if any dimension is under-specified (especially scope and consequence), the target's identity is ambiguous, and exact-equality verification cannot close the substitution attack. |
| Justification link | Explicit link between one AuthorityTarget and one belief it requires | No table exists. `belief_edge` is belief-to-belief only and cannot be reused (FKs both reference `belief(id)`). | Explicit auditable row | Schema shape; whether the link carries its own `belief_status` snapshot or references current belief status | If the link does not snapshot belief status at creation time, a target's authorization state silently drifts when a justification belief is retracted — the target remains in the database with a dead link. |
| Warrant | Portable reference to currently-valid authorization | No table or object exists. Authority is currently the live-intent relationship. | JSON protocol object, not a second source of truth | Portability vs central verification trade-off (ADR Decision 1); whether a warrant is an identifier or a self-contained token; expiry semantics | If a warrant is treated as bearer-scoped or cached past retraction, it becomes a fail-open artifact. The specification correctly says "possessing it grants nothing," but an integration that caches warrants must enforce bounded cache expiry. |
| Action / Execution | What the executor actually does | `action_intent.action TEXT` (`db/001_schema.sql:65`); `state ∈ {live,cancelled,executed}` | Execution claim must present the authorized tuple verbatim | I-13 deferred; no post-execution receipt | The MVP contract closes the authorization substitution window but not the execution-effect window. A compromised executor that verifies correctly and then acts differently is out of scope for the specification — this is honest, but it must be named as a product risk, not silently deferred. |
| Reassessment | Action → Reassessment lifecycle step | `RetractCascade` (`kernel/kernel.go:127-150`) is the only invalidation mechanism; it is explicit (operator calls `solvent_falsify`). No background or push mechanism. | Who triggers, what evidence, synchronous or asynchronous | All sub-questions in `## Open Architectural Questions` are unresolved | If reassessment is operator-triggered only, a promoted belief with a live intent can remain authorized after contradictory evidence arrives until an operator acts. The gap between evidence arrival and reassessment is a production SLO risk. |

## Cardinality verdict

| Relation | Specified cardinality | Safe? | Notes |
|---|---|---|---|
| Belief → AuthorityTarget | 0..* via justification links | **Conditionally** | Safe only if link-creation authority is gated (F-2-1). Without the gate, an attacker can attach a promoted belief to any target. |
| AuthorityTarget → Belief | 1..* required, AND semantics | **Conditionally** | Set-level AND enforcement is `OPEN #7`. If AND is service-enforced only, a bug in the service layer can authorize a target with one dead link. |
| Belief → Obligations | 1..* at entry, policy-owned | **At risk** | Safe only if minimum-validity rules exist (F-2-3). A policy that issues zero obligations makes promotion trivial. |
| Obligation → Discharges | exactly 0..1 accepted (0..* recorded) | **Conditionally** | Safe only if discharge-array consistency is atomic (F-2-2). In the transitional dual-truth model, a service-layer race can create disagreement. |
| Artifact → Citation | 1..* | **Safe** | The specification correctly says sharing is at the artifact level via distinct citations. This is the cleanest part of the model. |
| Citation → Belief | 1..1 | **Safe** | Each citation is per-belief. No ambiguity. |
| Target → Warrant | 0..* over time, ≤1 current | **At risk** | "≤1 current" is not enforced by any proposed mechanism. If two warrants are issued concurrently for the same target, both are "current" until one is invalidated. The specification does not propose a uniqueness constraint. |
| Warrant → Execution | MVP proposes 1; >1 is OPEN | **At risk** | If a warrant can be used for multiple executions without re-verification, a retracted belief's warrant can execute multiple times after retraction. The specification defers this but does not bound the risk. |

## Promotion verdict

### Is N1 ∧ N2 ∧ N3 ∧ N4 actually sufficient for promotion?

**No — not as currently specified.**

- **N1** (`debt=[]`, `final_truth=false`) is schema-enforced today and is sufficient as a necessary condition.
- **N2** (every obligation reached discharged state only through an accepted `DebtDischarge`) is proposed as service-enforced in the transitional model and is **not sufficient** without atomic discharge-array consistency (F-2-2). In the current codebase, `RetireDebt` is `array_remove` with no discharge record, and the specification's transitional model preserves this gap.
- **N3** (`not retracted`, no open contradiction) is half-enforced. Retraction is schema-real today. Open contradiction is deferred — contradictions are currently discarded with no ledger mutation (`internal/pipeline/pipeline.go:89-101`). N3's contradiction half is service-best-effort until it exists.
- **N4** (tenant policy permits promotion) is proposed as a policy layer with no representation. If the policy layer does not exist, N4 is vacuously true.

The conjunction N1 ∧ N2 ∧ N3 ∧ N4 is the right sufficient condition set, but the specification is honest that N2–N4 are not yet enforceable. The honest verdict is: **N1 is sufficient today; N1 ∧ N2 ∧ N3 ∧ N4 is the target, and the gap between today and the target is the entire attack surface of F-04.**

### Is promotion correctly separated from authority?

**Yes.** The specification correctly maintains that promotion grants eligibility only, and authority additionally requires an explicit `AuthorityTarget` with a fully satisfied justification set. This is the strongest part of the design. The anti-broadening example (Verified Customer Identity → three different targets) demonstrates the discipline correctly.

However, separation is not enforcement. If N2–N4 are service-layer only and the service layer has a bug, a promoted belief with an empty debt array and no valid discharges will still pass the schema's `promoted_is_debt_free` CHECK and appear eligible. The separation is correct; the enforcement gap is the risk.

## Authority-target verdict

**The five-tuple is sufficient in shape but not yet in enforcement.**

- **Principal:** `{principal_type, principal_id}` is the right abstraction. The current codebase has no principal anywhere. The specification correctly keeps identity resolution outside the kernel. The risk is that "outside the kernel" means "not enforced anywhere in MVP," which leaves authorization anonymous.
- **Resource:** Opaque `{resource_type, resource_id}` with exact equality is correct. The risk is that the current `action_intent` carries no resource column, and the migration path for old rows is fail-closed only if the service layer treats `NULL` as unknown, not wildcard.
- **Scope:** An open constraint set that narrows but never broadens is the right rule. The risk is that the exact dimensions and their comparison semantics are unresolved. Two targets differing only in scope must be different targets, but "differing in scope" requires a scope comparison operation that does not yet exist.
- **Action:** `{action_namespace, action_name}` is correct. The risk is that the current `action TEXT` is un-namespaced and adapter-side normalization is not yet prevented. The specification correctly says "no adapter-side silent normalization," but the current MCP handler (`cmd/solvent-mcp/tools.go:211`) accepts `action` as a plain string with no namespace validation.
- **Consequence:** `{consequence_type, consequence_parameters}` is correct. The risk is that exact-equality comparison of opaque JSON requires canonical serialization, which the specification does not specify. If two systems serialize the same consequence differently, the tuple comparison fails for legitimate authorizations and passes for illegitimate ones (if one system omits dangerous parameters).

## Executor-binding verdict

**The authorization boundary (I-12) is correctly identified. The execution boundary (I-13) is honestly deferred. The gap between them is the product's honest risk.**

The specification correctly states: "Solvent guarantees the executor *was authorized* for exactly what it presented (I-12); it does not yet guarantee what the executor *actually did* (I-13)." This is the right separation.

However, the specification does not address the availability dimension of the executor contract: what happens when Solvent is unreachable at execution time? The executor contract says "if Solvent is unreachable for a consequential action, the executor MUST NOT execute" — but this is a prose condition on the executor, not a Solvent-enforced guarantee. An executor that cannot reach Solvent and chooses to execute anyway has violated the contract, but Solvent cannot detect or prevent that violation. This is correctly deferred, but it must be named as a product risk in any customer-facing documentation.

## Warrant verdict

**The Warrant semantics create four concrete risks that must be bounded before production:**

1. **Replay:** A warrant is a portable reference. If an executor caches it and the underlying belief is retracted, the cached warrant authorizes a dead belief. The specification says "a retracted justification belief kills every warrant that cited it, centrally and immediately" — but this assumes the executor re-verifies. An executor that does not re-verify has a stale warrant.
2. **Stale authority:** Same as replay. Central verification is only effective if the executor calls it.
3. **Concurrency:** Two concurrent authorization requests for the same target can both receive valid warrants before either justification set is fully evaluated. The specification does not propose a uniqueness constraint or a serialization mechanism.
4. **Caching:** The specification says "never cached past the ledger's ability to retract" but does not define a maximum cache duration or a re-verification protocol.

The simplest mitigation is: warrants are single-use identifiers (not bearer tokens), and every executor must re-verify before every consequential action. This is a service-layer contract, not a schema invariant, and it must be written before any executor integration.

## Policy verdict

**Customer-defined policies can make Solvent semantically meaningless unless a minimum-validity contract is enforced.**

The specification correctly identifies the attack: "The customer configures Solvent into meaninglessness." The attacks listed (empty obligations, self-satisfied obligations, policy that declares any evidence sufficient) are real. The specification records these as OPEN but does not propose a minimum-validity rule.

The minimum-validity contract must include:
- At least one `attested` or `quorum` obligation for any target with a `HIGH_IMPACT` consequence.
- `deterministic` obligations cannot be discharged by `external_feed` evidence alone.
- An obligation set cannot be empty (every belief must carry at least one obligation at entry).
- A policy cannot define an obligation whose discharge policy is "any evidence of any kind."

These are service-layer rules, not kernel rules, but they are the boundary between "Solvent is correct" and "Solvent is being used correctly."

## Evidence/provenance verdict

**The Artifact/Citation distinction is semantically sufficient for I-11 but not yet enforceable.**

The specification correctly separates the reusable artifact from the per-belief citation. This resolves the hidden contradiction (a shared signed manifest should not be duplicated per belief, nor smuggled via duplicated references). The cardinality is safe: Artifact 1→* Citation, Citation →1 Belief.

However, the current `evidence` table is belief-owned (`belief_id` FK, one row → one belief), and the proposed artifact/citation split has no table representation yet (`OPEN #13`). Until I-11 (server-verified content hashing, provenance-class enforcement, replay protection) is implemented, the artifact's identity is caller-attested (`content_sha256` is caller-supplied in the current schema, `db/001_schema.sql:55`). The specification correctly defers I-11 but should name the artifact/citation split as blocked on I-11 for production use.

## Migration verdict

**The migration design is correct in principle but insufficiently tested.**

The fail-closed rule (`NULL → NOT AUTHORIZED`, never `NULL → wildcard`) is correctly stated. The additive-only column strategy matches the existing migration discipline (`db/004_debt_vocabulary.sql`). The required test (`TestMigration_OldActionIsNotOverAuthorized`) is named.

However, the specification does not address:
- Whether old `action_intent` rows with `action TEXT` only can be replayed under the new model via warrant references (F-11 in the MVP review).
- Whether a tenant can have old and new semantics simultaneously during a transition period.
- Whether `RetractCascade` semantics change when targets exist (target-scoped revocation is `OPEN #11`).

The migration design is sound; the test coverage is incomplete.

## Formal-verification verdict

**The Lean model is correctly bounded and should not be extended until the corresponding SQL exists.**

The current Lean model proves abstract state-machine preservation for the four current transitions (`promote`, `cancelIntent`, `authorizeIntent`, `retractCascade`). It has no `principal`, `resource`, `scope`, `consequence`, `DebtDischarge`, `AuthorityTarget`, or `Warrant` types.

The specification correctly follows the F-12 discipline: "Lean is not expanded for symmetry." Extending Lean to model target binding, AND justification, or retraction before the corresponding SQL constraints exist would prove an abstraction, not an implementation.

The refinement boundary is correctly identified: the critical DB behavior that Lean does not model is `ON UPDATE CASCADE` re-evaluation of `live_requires_promoted` and `crdb.ExecuteTx` retry semantics. Any future Lean extension that proves properties of the new target/warrant model must be accompanied by the corresponding SQL CHECKs or FKs — otherwise the Lean theorem proves a property the database does not enforce.

## DocTrust verdict

**DocTrust remains the right first reference integration, but it will not exercise the unresolved semantics.**

DocTrust is intended as Solvent's first independent reference integration, consuming Solvent through public interfaces without sharing internals. The specification correctly notes that DocTrust is independent and convergent, not a runtime dependency.

DocTrust will exercise:
- `Evidence → Belief → Debt → Promotion → AuthorityTarget → Warrant` flow
- Document-centric evidence with deterministic `needProvenanceCheck`-like debt
- Human review/attestation (if DocTrust implements the attestation channel)

DocTrust will **not** exercise:
- Multi-belief AND justification (DocTrust's domain is single-document approval)
- Risk-tiered consequences
- Tenant isolation
- Temporal validity
- Scope constraints beyond department-level
- Evidence artifact/citation split (DocTrust's evidence is document-hash-based, not corpus-scoped)

The risk is that a DocTrust-shaped kernel looks general while being document-shaped. The specification correctly identifies this as an unresolved generality question but does not require a second synthetic consumer before claiming domain-agnosticism.

## Production blockers

Must be resolved before touching the kernel or schema:

1. **F-2-1 (CRITICAL):** Resolve the four-role decomposition for target/link creation authority (`OPEN #8`). At minimum: who may create a target, who may attach a justification link. Without this boundary, the M:N explicit-link model moves the broadening attack from implicit promotion to explicit linking without closing it.
2. **F-2-2 (HIGH):** Decide whether N2 / I-10 discharge-array consistency is schema-enforced or service-enforced during the transitional period, and add an atomicity test that proves the discharge+array write is a single transaction. Do not ship "auditable" as "impossible."
3. **F-2-3 (HIGH):** Define minimum-validity rules for tenant obligation sets before any production policy integration. At least one `attested` or `quorum` obligation for `HIGH_IMPACT` targets; no empty obligation sets; no self-satisfying discharge policies.
4. **F-2-4 (HIGH):** Write the executor contract as a standalone integration-facing document before any executor integration claims I-12. The contract must specify the exact API call, tuple format, failure behavior, and retry semantics.

## Conditional approval requirements

Each condition must have: finding, required decision, required evidence, and exit test.

| Condition | Finding | Required decision | Required evidence | Exit test |
|---|---|---|---|---|
| C-1 | F-2-1 | Resolve four-role decomposition for target/link creation | Written security boundary document naming the four roles and their gating mechanism | Schema design review confirms target/link tables have creation-authority FK or service-layer gate |
| C-2 | F-2-2 | Choose N2 enforcement layer for transitional period | Service-layer atomicity test proving discharge+array write is single transaction; OR schema migration to obligation rows | Test passes: concurrent discharge + array_remove either both commit or both rollback |
| C-3 | F-2-3 | Define minimum-validity policy contract | Written policy contract naming minimum obligation types per risk tier | Service-layer test rejects empty obligation set, rejects `external_feed`-only discharge for `attested` obligations |
| C-4 | F-2-4 | Produce executor integration contract | Standalone contract document with API spec, tuple format, failure behavior, retry semantics | Integration test against mock executor verifies contract conditions 1–4 (`## Executor Binding`) |

## Revised next-step ordering

Minimum changes to the existing roadmap. Do not rewrite the entire plan.

```
Current:  Semantic contract → Authority abstraction → Kernel hardening
          → Attestation → Temporal → Warrant+REST → ...

Revised:  Semantic contract
          → Resolve C-1 through C-4 (security boundary, N2 enforcement,
            minimum-validity policy, executor contract)
          → Authority abstraction (with link/target creation authority gated)
          → Kernel hardening (preserving existing I-1..I-8)
          → Attestation
          → Temporal
          → Warrant+REST
```

The four conditions are a single prerequisite phase before the authority abstraction work begins. They do not change the kernel design; they close the security boundaries that the current specification leaves open.

## Dogfood Evidence Ledger

| Claim | Evidence | Confidence | Open Debt | Status |
|---|---|---|---|---|
| Current `belief.debt[]` is the only CHECK truth; no discharge record exists | `db/001_schema.sql:25-32` (`promoted_is_debt_free` checks `array_length(debt,1)=0`); `kernel/kernel.go:83-88` (`RetireDebt` is `array_remove`); no `debt_discharge` table anywhere in repository | High | — | Discharged |
| Current `action_intent` carries no principal, resource, scope, or consequence | `db/001_schema.sql:60-76`; `docs/POST_HACKATHON_ARCHITECTURE.md:399-419` proposes them as future, unimplemented | High | — | Discharged |
| Current evidence is belief-owned; no reusable artifact object exists | `db/001_schema.sql:47-58` (`belief_id` FK, one row → one belief); `db/002_corpus.sql:6-9` corpus is shared but is NOT ledger evidence | High | — | Discharged |
| Current contradictions are detected and discarded without ledger mutation | `internal/pipeline/pipeline.go:89-101`; `internal/belief/belief.go:38-48` (`slog.Warn`, return nil) | High | Contradiction relation deferred | Discharged |
| Current MCP surface has 7 tools including `solvent_explain` | `cmd/solvent-mcp/main.go:88-247`; `docs/FOUR_VERB.md:25-31` | High | — | Discharged |
| Lean model is abstract state-machine preservation, not a refinement proof | `formal/lean/Solvent/Types.lean:47-51` (`ActionIntent` carries only `state, beliefId, beliefStatus`); `formal/lean/README.md` (explicitly not a refinement proof) | High | — | Discharged |
| M:N explicit links can manufacture authority if link creation is ungated | Design reasoning (`## Cardinality` moved attack, `Self-Attack #15`); `OPEN #8` records the gap | High | C-1 required | Open |
| N2 discharge-array consistency is not schema-enforceable in the transitional dual-truth model | `## Promotion Preconditions` ("Rule 1 as a schema enforcement … is OPEN and deliberately not made in a semantics-first document") | High | C-2 required | Open |
| Vacuous-policy defense is unresolved | `OPEN #9`; `## Debt and Discharge Semantics` ("minimum-validity rule for tenant-defined vacuous sets: unresolved") | High | C-3 required | Open |
| Executor contract exists as prose but not as a verifiable integration document | `## Executor Binding` (five conditions as prose); `## Recommended Next Decisions` (decision 7: "Executor contract document … written as the integration-facing contract before any executor integration claims I-12") | High | C-4 required | Open |
| Artifact/Citation semantics are locked; representation is OPEN #13 | `## Domain Objects → Evidence` ("semantics-only, no table sketch yet (OPEN #13)"); `## Open Architectural Questions` #13 | Medium | Table representation OPEN #13 | Open |
| N4 placement as 4th promotion conjunct vs target-level Authority Eligibility is OPEN #16 | `## Promotion Preconditions` ("N4's placement is contested (OPEN #16)"); conservative choice `Promotion = N1∧N2∧N3∧N4` kept | Medium | Placement OPEN #16 | Open |
| Target → Warrant "≤1 current" is not enforced by any proposed mechanism | `## Cardinality` ("0..* over time, ≤1 current"); no uniqueness constraint or serialization mechanism proposed | Medium | Warrant concurrency risk | Open |
| Warrant → Execution "1; >1 is OPEN" creates stale-authority risk if warrants are cached | `## Cardinality` ("Warrant → Execution: MVP proposes 1; >1 is OPEN"); no re-verification protocol proposed | Medium | Warrant caching/replay risk | Open |
| Migration fail-closed rule is correct; test coverage is incomplete | `## Migration` (fail-closed rule, `TestMigration_OldActionIsNotOverAuthorized` named); F-11 in MVP review (replay of old warrants under new model) | Medium | Migration test debt | Open |
| Lean model should not be extended before corresponding SQL exists | F-12 discipline (`docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:146-153`); `## Lean / SQL / Service Responsibility` ("Lean is not expanded for symmetry") | High | — | Deferred |
| Evidence ingress hardening (I-11) is deferred and correctly so | `docs/ADR/0001:222-240` (evidence gateway as OUT OF SCOPE for hackathon); current ingestion is fixture-only (`internal/pipeline/pipeline.go:41-57`) | High | — | Deferred |
| Temporal authority / freshness (N5) is deferred | `docs/ADR/0001:543` defers temporal authority; `docs/POST_HACKATHON_ARCHITECTURE.md:2.4` sketches four-layer defense | High | — | Deferred |
| DocTrust is the right first reference integration but does not exercise multi-belief, risk-tiered, or tenant-isolated semantics | `docs/ADR/0001:313-322` (DocTrust is independent, convergent, not a runtime dependency); `docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md:117-124` (F-09) | Medium | Second synthetic domain to prove generality | Deferred |
| A2A is correctly deferred | `docs/ADR/0001:300-311` (Decision 7 sequencing); `docs/POST_HACKATHON_ARCHITECTURE.md:3.4` defers A2A extension | High | — | Deferred |

## Final dogfood test

> Does the Authority Target / Debt Discharge specification itself obey the Solvent methodology?

- **What evidence supports it?** The specification cites repository truth for every CURRENT claim (`db/001_schema.sql`, `kernel/kernel.go`, `internal/belief/belief.go`, `internal/pipeline/pipeline.go`, `cmd/solvent-mcp/tools.go`, `formal/lean/Solvent/Preservation.lean`, `docs/ADR/0001`, `docs/POST_HACKATHON_ARCHITECTURE.md`, `docs/REVIEWS/MVP_PLAN_ADVERSARIAL_REVIEW.md`). The specification is grounded in measured repository behavior, not strategy documents or prior approvals.
- **What beliefs does it make?** It makes three proposed architecture beliefs (Decision A: belief is never authority; Decision B: AuthorityTarget is a separate consequential object with M:N explicit links; Decision C: debt-free is necessary, not sufficient) and four sufficient-condition beliefs (N1–N4 for promotion).
- **What debt remains?** Sixteen OPEN items (`OPEN #1`–`#16`), four of which are production blockers (C-1 through C-4 above). The specification honestly records these as OPEN rather than promoting them by rhetoric.
- **Who would be authorized to promote it?** Under the specification's own methodology, the specification itself must earn promotion through adversarial review. This review is that gate. The verdict is APPROVE WITH CONDITIONS, meaning the specification is promoted with open debt that must be discharged (C-1 through C-4) before consequential implementation action.
- **What would falsify it?** A proof that any of the following is possible: (1) an ungated link/target creation creates a valid authorization for a consequence the linked belief does not justify; (2) the dual-representation debt model permits a committed state where `debt=[]` but no accepted discharge exists for an issued obligation; (3) a customer policy with empty or vacuous obligations produces a promoted belief that authorizes a high-impact consequence; (4) an executor integration that follows the prose contract verbatim still permits a substitution attack because canonical serialization is undefined.
- **What evidence would cause reassessment?** New evidence that any of the four conditions (C-1 through C-4) cannot be closed without kernel redesign would cause reassessment of the entire proposal. Evidence that the M:N explicit-link model is structurally unable to prevent link-sprawl at scale (even with gated creation) would cause reassessment of Decision B. Evidence that N2 cannot be made atomic in any representation would cause reassessment of the transitional debt model.
- **Which unresolved questions prevent promotion?** `OPEN #8` (link/target creation authority), `OPEN #6`/`#7` (N2 enforceability), `OPEN #9` (vacuous-policy defense), and the missing executor contract document (C-4). These are the four conditions. Until they are discharged, the specification is promoted with open debt — it can guide design discussion but cannot authorize kernel or schema changes.

---

*This review document does not modify any Go source, SQL schema, Lean proof, MCP implementation, REST endpoint, ADR, or the specification itself. The review was produced by an independent adversarial reading of the specification against the repository, following the 20-attack structure prescribed in `/home/chaschel/Desktop/biz/solvent/v2/adv_plan2_review.md`.*
