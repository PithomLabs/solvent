ARCHITECTURAL DECISION — FINAL
==================================================

The invariant is:

    DEBT IS OPAQUE TO SOLVENT.

Solvent must store and structurally enforce debt, but must not own debt vocabulary.

Final ownership:

    Solvent Kernel
        - stores debt TEXT[]
        - knows only empty vs non-empty
        - promotion requires zero debt
        - retires opaque identifiers
        - does NOT enumerate, validate, normalize, or interpret debt strings

    API / Application Boundary
        - may perform structural input validation
        - omitted JSON debt maps to empty []string
        - malformed identifiers such as "" are rejected here
        - does NOT impose arbitrary domain-neutral vocabulary rules

    Wizard Domain
        - owns its existing six deployment-review debt identifiers
        - owns the single source of truth for wizardDebt
        - owns DebtMapping
        - must not depend on kernel.FullDebt

    EBP Coordinator
        - owns/defines ebpInitialDebt
        - MUST attach EBP starting debt during packet compilation
        - must not rely on Solvent defaults

No new universal/default debt vocabulary is to be introduced.

Do NOT replace the old Solvent six-item vocabulary with:
- ebp:* defaults
- generic default debt
- universal literal vocabulary
- "sane" kernel defaults
- a shared Solvent debt taxonomy
- any other substitute default list

The database default is intentionally empty.

==================================================
IMPORTANT — PLAN ONLY
==================================================

Do not modify source code.

First inspect the repository and revise the implementation plan against the actual codebase.

The revised plan must explicitly identify:
- exact files to change;
- exact existing APIs and call sites;
- exact migration path;
- exact tests to replace/add;
- exact ownership of wizard debt;
- exact Coordinator enforcement point;
- exact re-freeze procedure.

Do not assume filenames, migration numbers, or relationships until inspected.

==================================================
1. RESOLVE mapping.go BEFORE FINALIZING PLAN
==================================================

Inspect the actual wizard DebtMapping implementation.

Determine which of these is true:

A. mapping.go currently imports/references kernel.FullDebt

OR

B. mapping.go independently defines the six wizard debt identifiers

OR

C. another relationship exists.

The final plan MUST resolve this empirically.

Desired end state:

    wizard/domain vocabulary
            |
            +--> wizardDebt
            |
            +--> DebtMapping
            |
            v
         Solvent
       opaque []string

kernel.FullDebt must cease to be the source of wizard vocabulary.

If mapping.go references kernel.FullDebt:
    move the six-item vocabulary into the wizard/domain layer and
    make both wizardDebt and DebtMapping consume that one domain-owned source.

If mapping.go already owns an independent six-item list:
    verify that it is equivalent to the current wizard vocabulary,
    consolidate it into one domain-owned source of truth,
    and eliminate duplicate hand-maintained copies.

Record which branch was found and exactly how it is resolved.

Do not leave this conditional or unresolved in the final plan.

==================================================
2. SOLVENT DATABASE CONTRACT
==================================================

Change the belief debt default from the hardcoded deployment vocabulary to an empty array.

Conceptually:

    debt TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[]

Use the repository's actual CockroachDB/Postgres-compatible syntax.

Do NOT create:
- debt table
- debt vocabulary table
- debt type table
- debt definition table
- Solvent debt registry

Do not alter unrelated schema semantics.

Promotion must remain database-authoritative:

    promoted
    requires debt empty
    and final_truth = false

Existing:
- belief_edge
- evidence
- action_intent
- retraction cascade
- refusal logging

must remain semantically unchanged.

==================================================
3. MIGRATION PATH MUST BE TRACED
==================================================

Inspect the actual migration chain and bootstrap mechanisms.

Determine whether modifying an old schema such as 001 is actually necessary.

If a later additive migration already establishes the empty default, do not make a historical migration change merely for appearance.

Trace all real paths:
    clean install
    migration chain
    test bootstrap
    any alternate schema loader

The final plan must state why each schema file is or is not modified.

Do not leave duplicate or dead schema edits without explaining the path they serve.

==================================================
4. EnterBelief API
==================================================

Change EnterBelief so callers can supply initial debt.

Conceptually:

    EnterBelief(ctx, scenarioID, claim, claimType, debt)

Use the repository's actual idiom.

API behavior:

    omitted debt -> []string{}
    explicit [] -> []
    explicit ["foo","bar"] -> exactly those opaque identifiers

The JSON `debt` field should remain OPTIONAL.

Do not make it a required field merely to compensate for the loss of the old Solvent default.

The responsibility for meaningful initial debt belongs to the caller/domain layer.

The database empty default remains correct.

==================================================
5. STRUCTURAL INPUT VALIDATION
==================================================

Keep structural validation separate from semantic vocabulary validation.

Accept arbitrary non-empty debt identifiers.

Reject malformed empty-string identifiers:

    ""

at the API/application boundary.

Do NOT:
- lowercase identifiers
- trim identifiers in Solvent
- impose a universal regex vocabulary
- impose arbitrary max-length rules
- impose arbitrary max-number-of-debts rules

Solvent must not decide what a valid domain vocabulary is.

The domain/application layer may enforce stronger constraints appropriate to its own vocabulary.

==================================================
6. DEGENERATE CASE CONTRACT
==================================================

Define and test these explicitly.

Empty string:

    "" -> reject at API/application boundary

Duplicates:

    ["foo","foo"] -> structurally permitted

Retirement of duplicate values:

    preserve actual array_remove behavior
    and document/test that behavior

Retirement of an existing belief's missing debt item:

    idempotent no-op, preserving existing behavior

Retirement against a nonexistent belief:

    MUST remain an error

Do not accidentally collapse "belief does not exist" and
"debt item does not exist."

Add explicit regression coverage for both cases.

==================================================
7. RETIRE DEBT
==================================================

RetireDebt remains generic.

It must not:
- consult a vocabulary
- validate known debt names
- normalize identifiers
- reject previously unknown debt strings simply because they are unknown

Preserve current transactional/error semantics.

==================================================
8. REPLACE OLD TEST CONTRACTS
==================================================

Do not preserve obsolete tests as commented-out documentation.

Replace old B-17/B-23-style tests with debt-agnostic contract tests.

At minimum cover:

    arbitrary debt accepted
    omitted debt => empty
    explicit empty debt accepted
    unknown debt accepted
    arbitrary debt can be retired
    promotion blocked while any debt remains
    promotion succeeds when debt empty
    final-truth prohibition unchanged
    action_intent gate unchanged
    retraction cascade unchanged
    refusal logging unchanged
    nonexistent debt item is a no-op
    nonexistent belief still errors
    empty-string rejected at application boundary
    duplicates behave according to actual array_remove semantics

Also explicitly search for old tests that assert rejection of
"unknown debt" or vocabulary membership, even when those tests do
not reference kernel.FullDebt.

Replace/invert them deliberately.

Do not merely let the suite fail and then patch failures opportunistically.

==================================================
9. REPOSITORY-WIDE VOCABULARY SWEEP
==================================================

Plan an explicit repository-wide search for all six legacy identifiers.

Verify where they remain.

Expected ownership:
- wizard/domain implementation
- wizard DebtMapping
- relevant examples/tests/fixtures where historically intentional

Unexpected locations:
- Solvent kernel
- Solvent default constants
- generic Solvent validators
- database default definitions
- generic MCP enum/schema that claims they are universally valid

Use the sweep as evidence for the final "what changed" claim.

==================================================
10. MCP / API SCHEMA
==================================================

If the MCP/API schema currently exposes the old six-item enum:

    remove the Solvent-level vocabulary enum.

Do not replace it with another universal enum.

Update tool/schema descriptions so callers understand:

- debt identifiers are opaque;
- the currently attached debt values can be discovered from the belief;
- retirement must use one of the actual debt identifiers attached to that belief.

The schema must not pretend Solvent knows a universal vocabulary.

Note any client-schema/cache impact in the implementation plan.

==================================================
11. WIZARD DOMAIN OWNERSHIP
==================================================

Preserve the existing wizard's six deployment-review names.

Move ownership out of Solvent and into the wizard/domain layer.

The plan should show one domain-owned source of truth consumed by:

    wizardDebt
    DebtMapping
    any wizard-specific validation/tests

The wizard may reject typoed debt identifiers because it owns their meaning.

This is intentional.

The kernel must not perform this check.

==================================================
12. EBP COORDINATOR — THIS IS REQUIRED
==================================================

This is a critical architectural requirement.

The Coordinator MUST explicitly attach the EBP initial debt set when compiling research beliefs.

Do not merely state this in prose.

Make it mechanically checkable.

The plan must define:

    ebpInitialDebt

at the Coordinator/domain layer.

The Coordinator's compile step MUST ensure every EBP belief it creates receives the required EBP starting obligations, even when the research packet contains no debt values.

Agents may propose debt.

Agents do NOT control the final starting debt attached to the canonical belief.

The Coordinator is authoritative for this translation.

Required negative regression test:

    input packet:
        valid belief
        no debts supplied by agent

    compiled Solvent belief:
        contains ebpInitialDebt

This test exists specifically to prevent accidental fail-open research ingestion.

Also define what happens when the packet proposes additional debt:
    preserve/merge according to the actual Coordinator contract,
    without silently deleting required EBP initial debt.

Do not let omission from an agent packet translate into an immediately
promotable EBP belief.

==================================================
13. SAFETY-POSTURE CHANGE
==================================================

The plan must explicitly acknowledge:

Before:
    Solvent itself injected six deployment-specific obligations.

After:
    Solvent starts empty unless the caller supplies debt.

This is intentional kernel decoupling.

The compensating safety property is:

    domain adapters MUST attach their own starting obligations.

For EBP:
    Coordinator enforces this.

For Wizard:
    Wizard owns its own initial debt.

Do not "fix" this by restoring a universal Solvent default.

==================================================
14. LEGACY / GRANDFATHERED BELIEFS
==================================================

Make the migration decision explicit:

Existing database rows retain their historical debt arrays.

This migration must NOT:
- rewrite old beliefs;
- translate old debt names;
- reinterpret historical debt;
- auto-retire historical obligations.

State clearly how old wizard beliefs remain retireable after the vocabulary moves to the wizard/domain layer.

==================================================
15. FUNCTIONAL TAXONOMY — GUIDANCE ONLY
==================================================

Preserve the useful cross-domain insight, but do NOT encode it into Solvent.

Create or update documentation outside the kernel repositories describing
functional debt roles such as:

    provenance
    consistency
    scope
    verification
    impact
    recovery
    human authority where policy requires it

Make clear that these are:
- conceptual guidance;
- not a universal literal vocabulary;
- not mandatory Solvent debt values;
- not kernel policy.

Do NOT create:

    ebp:need_provenance
    ebp:need_scope
    etc.

as Solvent defaults.

Do not turn the taxonomy into a shared enforced enum.

If documentation belongs in Conductor/coordinator/application docs or another
non-kernel documentation location, specify that location.

The goal is to preserve the architectural insight without recreating coupling.

==================================================
16. DOMAIN VOCABULARY CONFORMANCE
==================================================

The final plan may recommend a domain-level rule:

    each domain defines its own debt vocabulary
    and maps its obligations to the functional questions
    that actually apply.

This is guidance, not Solvent enforcement.

For EBP, the Coordinator owns the EBP vocabulary.

For Wizard, the Wizard owns the wizard vocabulary.

==================================================
17. FULL REGRESSION PRESERVATION
==================================================

Plan to run:

    all Solvent tests
    all affected wizard tests
    Coordinator tests
    API/MCP tests
    relevant integration tests

Verify unchanged semantics for:

    Promote
    IntentOnPromoted
    RetractCascade
    refusal_log
    AuditLiveOnNonPromoted
    evidence handling
    belief_edge handling

No unrelated refactoring.

==================================================
18. FREEZE / GOVERNANCE SEQUENCE
==================================================

The implementation plan MUST end with an explicit freeze process.

Required sequence:

    implement
    -> targeted tests
    -> full suite
    -> migration/bootstrap verification
    -> repository-wide vocabulary sweep
    -> decision record
    -> new commit
    -> explicitly tagged frozen commit
    -> update freeze/Test-H/baseline references
    -> record supersession of previous freeze

The previous freeze at 7602699 becomes historical/superseded once this kernel change lands.

Do NOT leave the new baseline as floating HEAD.

Use the repository's established freeze/tag convention after inspecting it.

The decision record must lead with the architectural rationale:

    this change restores the documented doctrine that debt
    vocabulary belongs to the application, policy, or deployment;
    it does not expand kernel scope.

Record:
- why the kernel was reopened;
- alternatives considered;
- why no universal vocabulary was introduced;
- wizard vocabulary ownership;
- EBP Coordinator obligation;
- grandfathering decision;
- tests;
- final frozen commit/hash;
- supersession of the prior freeze.

==================================================
19. ROLLBACK
==================================================

Include a minimal rollback note.

Rollback must be understood as:
- revert the implementation commit/migration according to repository conventions;
- restore the previous frozen baseline if required.

Do not create an elaborate rollback subsystem.

==================================================
20. NON-GOALS
==================================================

The final plan must explicitly state that this change does NOT:

- redesign Solvent;
- redesign Conductor;
- introduce a debt table;
- introduce a debt vocabulary table;
- introduce universal EBP debt into Solvent;
- introduce a third research database;
- change belief lifecycle semantics;
- change authority semantics;
- add automatic promotion;
- add automatic debt retirement;
- introduce semantic reasoning into Solvent;
- make Solvent understand Wizard or EBP;
- replace domain vocabulary with generic placeholders.

==================================================
21. REQUIRED PLAN OUTPUT
==================================================

Return a revised implementation plan with these sections:

1. Executive decision
2. Actual repository findings
3. Ownership/boundary model
4. Schema/migration changes
5. EnterBelief/API changes
6. Wizard vocabulary migration
7. EBP Coordinator integration
8. Test changes
9. Migration/bootstrap verification
10. Documentation/taxonomy guidance
11. Decision record
12. Re-freeze procedure
13. Risks and mitigations
14. Exact files expected to change
15. Acceptance criteria

For every file, explain WHY it changes.

Do not implement anything.

Before presenting the final plan, resolve all conditional questions that can be answered
by repository inspection, especially:

    mapping.go relationship to kernel.FullDebt
    actual schema bootstrap/migration path
    existing unknown-debt rejection tests
    existing RetireDebt behavior for missing belief/item
    existing freeze/tag convention
    existing Coordinator packet compilation/validation path

The final plan must be internally consistent and implementation-ready.
||||||| base
=======
Perform a fresh, implementation-level adversarial security review of the CURRENT Solvent repository at HEAD after the latest Post-Kernel-Freeze Service Authorization Cleanup.

REVIEW ONLY.
Do not modify code, schema, migrations, tests, documentation, or configuration.

The review must be based on the actual repository and actual command results, not the implementation summary.

============================================================
1. REVIEW SCOPE
============================================================

The immediately preceding change set was intended to close the remaining
service-layer authorization gap involving:

- REST RetireDebt
- REST Discharge / discharged_by impersonation
- nil initialDebt documentation
- verification of the post-kernel-freeze boundary

The intended implementation was:

RetireDebt:
    authenticated principal
        ↓
    verify principal exists and is not revoked
        ↓
    kernel RetireDebt

Discharge:
    authenticated principal
        ↓
    validate request.discharged_by == authenticated PrincipalID
        ↓
    override/use authenticated principal as effective discharged_by
        ↓
    kernel Discharge

The kernel is intentionally frozen.

============================================================
2. PRIMARY OBJECTIVES
============================================================

Determine whether the implementation:

A. actually prevents discharged_by impersonation;
B. actually rejects revoked/nonexistent principals before debt mutation;
C. preserves correct successful RetireDebt and Discharge behavior;
D. preserves audit integrity;
E. introduces no accidental kernel/schema changes;
F. honestly documents the remaining RetireDebt TOCTOU race;
G. preserves the exact authority binding and debt-domain-agnostic properties;
H. leaves no unresolved CRITICAL/HIGH/MEDIUM defect.

The architectural post-freeze rule remains:

> New capabilities default to service, adapter, executor, deployment,
> policy, demo, or documentation layers.

A kernel change is justified only by a genuinely new durable security fact
or atomic security transition that cannot safely be enforced outside the kernel.

============================================================
3. RETIREDEBT AUTHORIZATION REVIEW
============================================================

Inspect the complete REST RetireDebt path.

Verify:

1. Authentication is present.
2. Authenticated PrincipalID is obtained from trusted request context.
3. Principal existence is verified.
4. Principal revocation state is verified.
5. The checks occur before the debt mutation.
6. No caller-supplied actor identity can bypass these checks.
7. Cross-scenario protections remain intact.
8. Successful active-principal retirement still works.

Attempt:

- nonexistent principal
- revoked principal
- active principal
- malformed principal
- cross-scenario belief
- repeated retirement
- concurrent revoke vs RetireDebt

The implementation currently intends to use a best-effort liveness pre-check
because the kernel owns its internal transaction boundary.

Verify that the code/docs do NOT falsely claim:

    "revocation and mutation are atomic"

unless the actual implementation proves that.

The accepted residual race is:

    principal active at pre-check
        ↓
    principal revoked
        ↓
    kernel RetireDebt still executes

Determine whether that race is:
- documented,
- accurately described,
- actually acceptable under the stated architecture.

Do not recommend kernel growth merely to eliminate it.

============================================================
4. DISCHARGE IMPERSONATION REVIEW
============================================================

Inspect the complete REST Discharge path.

Verify:

- authenticated principal is derived from request context
- caller-supplied discharged_by cannot impersonate another principal
- mismatched discharged_by is rejected
- correct authenticated identity is used for the actual kernel call
- successful discharge stores the authenticated identity
- failed impersonation causes zero discharge rows
- failed impersonation causes zero debt mutation

Attempt:

    authenticated principal = P1
    request discharged_by = P2

Expected:

    HTTP 403
    explicit discharged_by mismatch error
    no debt_discharge row
    no debt mutation

Also test:

    authenticated principal = P1
    request discharged_by = P1

Expected:

    success
    persisted discharged_by = P1

Do not substitute response-body validation for durable database verification.

============================================================
5. AUDIT INTEGRITY
============================================================

The durable debt_discharge row is audit evidence.

For successful discharge verify:

- exactly one appropriate row
- scenario_id correct
- belief_id correct
- obligation_key correct
- discharged_by equals authenticated principal
- no caller-controlled impersonation

For rejected requests verify:

- no false "discharged" audit event
- no false "debt_retired" event
- no misleading successful audit entry
- no audit row created when the operation never occurred

Do not conflate:
- authorization refusal
- state conflict
- successful mutation

============================================================
6. VERIFY THE PREVIOUS TESTING FAILURE CANNOT RECUR
============================================================

Earlier investigation discovered a stale test binary / debug-only replacement issue.

Confirm that the CURRENT repository contains the intended assertions.

Specifically inspect the relevant discharge test.

Verify:

- test source contains real assertions
- no debug-only replacement has removed them
- the test queries the current database
- the test verifies the persisted discharged_by value
- the test is built fresh before execution

Do NOT accept stale-binary output as evidence.

Where practical:

    go clean -testcache
    rebuild tests
    rerun targeted test

Use actual current source and current binary.

============================================================
7. NIL initialDebt REVIEW
============================================================

The current intended semantics are:

    nil initialDebt
        → []string{}
        → empty debt

    []string{}
        → empty debt

    DDL DEFAULT
        → not reached by supported kernel creation paths

Verify this against CURRENT HEAD.

Inspect:

- EnterBelief
- EnsureBelief
- SQL encoding
- nil guards
- DDL default
- callers

Determine whether:

- nil and empty are intentionally equivalent
- no accidental FullDebt fallback occurs
- all supported callers pass explicit arrays
- the documentation accurately describes actual behavior

Do not modify code.

============================================================
8. DEBT DOMAIN-AGNOSTICITY REGRESSION
============================================================

Re-check after the service authorization changes that:

- EnterBelief requires caller-supplied initialDebt
- EnsureBelief requires caller-supplied initialDebt
- kernel does not reference FullDebt internally
- RetireDebt accepts arbitrary opaque identifiers
- promotion depends on debt emptiness, not vocabulary
- no kernel validation against FullDebt exists

Verify that no service cleanup accidentally reintroduced
deployment-specific assumptions into kernel creation.

============================================================
9. EXACT AUTHORITY BINDING REGRESSION
============================================================

Re-check Plan 10.1 integrity.

Verify:

- action_intent.target_id
- action_intent.snapshot_id
- composite FK
- live intent unique index
- ClaimIntent 7-field CAS
- T1/S1 → T2/S2 confused-deputy rejection
- executor not invoked
- intent remains live on failed claim

Run the canonical confused-deputy test.

============================================================
10. API / SERVICE CALL-GRAPH REVIEW
============================================================

Trace all authority-changing paths.

At minimum inspect:

- EnterBelief
- EnsureBelief
- AddEvidence
- RetireDebt
- Discharge
- Promote
- AuthorizeAndCreateIntent
- ClaimIntent
- CompleteIntent
- RollbackClaim
- CancelIntent
- RetractCascade
- RevokeTarget

For each path determine:

    authentication
    authorization
    scenario binding
    database transaction boundary
    audit behavior
    caller-controlled fields

Look for alternate internal paths that bypass the newly added service checks.

============================================================
11. DIRECT DB ACCESS REVIEW
============================================================

Confirm the latest changes did NOT introduce a new raw DB write path.

Run:

    bash scripts/check_i7.sh

Inspect any changed SQL.

The desired state remains:

- kernel/service owns production writes
- swarm/demo/test raw SQL is clearly isolated
- application-level authorization is not mistaken for DB-level isolation

Remember:

> Anyone with database write credentials can bypass application authorization.

Classify this as deployment/credential security unless repository evidence
shows an application design flaw.

============================================================
12. SCENARIO ISOLATION REGRESSION
============================================================

Re-test:

- cross-scenario RetireDebt
- cross-scenario Discharge
- cross-scenario Promote
- cross-scenario evidence
- cross-scenario intent
- cross-scenario execution

Ensure the service authorization changes did not reorder validation
in a way that weakens scenario isolation.

============================================================
13. CONCURRENCY REVIEW
============================================================

Test or inspect:

### RetireDebt
    concurrent revoke
    concurrent retirement

### Discharge
    concurrent discharge
    duplicate discharge

### Authority
    concurrent claim
    concurrent intent creation
    concurrent revocation
    concurrent authorization

The RetireDebt revocation pre-check race is already acknowledged.

Do not report the existence of that known residual race as a new finding
unless the current implementation or documentation makes it materially
worse than previously accepted.

============================================================
14. HTTP / ERROR SEMANTICS
============================================================

Verify:

RetireDebt:
- unauthenticated → existing auth failure
- revoked/nonexistent principal → 403 or existing repository-defined
  principal error
- valid active principal → success

Discharge:
- mismatched discharged_by → 403 `discharged_by_mismatch`
- matching discharged_by → success

Ensure the implementation does not misclassify:

    authentication failure
    authorization refusal
    state conflict
    resource not found

Do not invent new error abstractions.

============================================================
15. KERNEL-FREEZE PROTECTION
============================================================

Check git diff / repository state.

Confirm:

    kernel primitives added       = 0
    kernel invariants changed    = 0
    schema changes               = 0
    migration changes            = 0
    kernel SQL changes           = 0
    kernel signatures changed    = 0

A comment-only kernel documentation change is acceptable if it was part
of this cleanup.

If any actual kernel semantic change exists, identify it explicitly.

Do NOT recommend keeping it merely because it is small.

============================================================
16. VERIFICATION COMMANDS
============================================================

Run all of these where available:

    git diff
    gofmt -l cmd internal kernel api service adapter
    go clean -testcache
    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task db:reset
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Also run targeted tests for:

- RetireDebt active principal
- RetireDebt revoked principal
- RetireDebt cross-scenario
- RetireDebt concurrent revoke race
- Discharge own identity
- Discharge impersonation
- persisted discharged_by
- no mutation on rejected discharge
- duplicate discharge
- exact authority confused deputy
- arbitrary debt vocabulary
- EnsureBelief preservation
- nil vs empty initialDebt

Report actual commands and actual results.

============================================================
17. FINDING DISCIPLINE
============================================================

Do not manufacture findings.

For every substantive finding provide:

- ID
- Severity
- exact file/symbol/line
- attack/failure path
- concrete evidence
- why existing controls fail
- security impact
- correct architectural layer
- whether kernel growth is required

Before reporting a finding, try to falsify it.

Use:

CRITICAL
HIGH
MEDIUM
LOW
INFO

Do not elevate an already accepted residual limitation without new evidence.

Known accepted limitation:
- RetireDebt principal liveness check is not transactionally atomic with
  kernel mutation because the kernel owns its internal ExecuteTx.

Reopen it only if current implementation creates a materially different
or exploitable condition.

============================================================
18. SPECIAL CHECK: SERVICE AUTHORIZATION VS. KERNEL RESPONSIBILITY
============================================================

For any authorization finding ask:

1. Is this identity/authorization policy?
2. Is it already enforceable at service/API level?
3. Does the DB need to know actor semantics?
4. Does the kernel need to know actor semantics?
5. Is there a genuinely new durable security fact?
6. Is there a genuinely new atomic transition?
7. Can the existing frozen kernel safely remain unaware?

The default answer should be:

    service/policy layer

unless evidence proves otherwise.

============================================================
19. FINAL VERDICT
============================================================

Conclude with exactly one:

    GREEN
    GREEN WITH ACCEPTED LOW
    NO-GO

GREEN:
- no unresolved CRITICAL/HIGH/MEDIUM security defect
- service authorization behavior correct
- audit provenance correct
- kernel remains frozen
- verification complete

GREEN WITH ACCEPTED LOW:
- only genuinely low-risk limitations remain
- no kernel growth required

NO-GO:
- unresolved security/integrity defect exists
- or the implementation violates the frozen kernel boundary
- or a real new atomic/durable kernel invariant has been demonstrated

============================================================
20. FINAL OUTPUT FORMAT
============================================================

### 1. Verdict

### 2. Executive assessment

Is the post-kernel-freeze cleanup actually correct?

### 3. Findings

Only substantive findings.

### 4. RetireDebt security results

### 5. Discharge security + persisted audit results

### 6. Nil initialDebt results

### 7. Exact authority binding regression results

### 8. Scenario/concurrency results

### 9. Kernel-freeze integrity

Explicitly state whether kernel/schema/migration/signature/SQL changes occurred.

### 10. Verification results

Actual commands and results.

### 11. Remaining accepted risks

Only risks actually supported by evidence.

### 12. Final freeze status

If GREEN or GREEN WITH ACCEPTED LOW:

    SOLVENT KERNEL: FROZEN
    POST-FREEZE SERVICE AUTHORIZATION: VERIFIED
    NO KERNEL GROWTH REQUIRED

Do not treat Oracle, physics verification, swarm, harness,
semantic applicability, or domain policy as part of this review.
Those remain separate post-freeze work.
