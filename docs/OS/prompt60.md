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