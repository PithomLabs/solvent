Phase 4C+ — Final Implementation-Plan Reconciliation Before Coding

STOP. Do not implement production code yet.

The Test Engineering Gate has already PASSED:

    Kilo Code → GO

Authoritative test contract:

    docs/OS/phase4c_plus_test_target.md

Current implementation plan:

    phase4c_plus_imp.md

Your task is to revise ONLY the implementation plan so it is fully consistent
with the already-approved test contract.

Do not weaken, reinterpret, or silently change the approved test semantics.

==================================================
1. CRITICAL — RESOLVE CI-2 / CompleteIntent CONTRADICTION
==================================================

The approved test target defines:

    CI-2:
    ExecutionResult.Success is true iff the executor function returned nil error.

The current implementation plan instead proposes:

    executor returns nil
    → provider accepted
    → CompleteIntent fails
    → result.Success = false

This contradicts the approved acceptance contract.

Fix the implementation plan.

Preferred semantics:

    executor returns nil
        →
    provider accepted
        →
    ExecutionResult.Success remains true

If CompleteIntent then fails:
    - preserve the truthful execution result
    - do NOT redefine provider acceptance as execution failure
    - explicitly record the persistence failure separately
    - explicitly document that the intent may remain live
    - explicitly document the resulting duplicate-execution risk
    - do NOT claim that sequential duplicate prevention still holds in this
      failure case

The approved contract must remain authoritative.

Do not modify CI-2.

Do not create a new semantic meaning for ExecutionResult.Success.

Document the distinction:

    Provider/executor outcome
        versus
    Solvent persistence of the execution fact

Any audit event for CompleteIntent failure must not falsely say that the
provider execution failed when the provider already accepted it.

==================================================
2. COMPLETEINTENT FAILURE TESTING
==================================================

The approved test target does not currently contain a dedicated
CompleteIntent-persistence-failure test.

Do not invent a new mandatory security test unless necessary.

Instead, in the implementation plan:

- explicitly classify the scenario as a known v1 limitation
- state the exact resulting state:
      provider accepted
      result.Success == true
      CompleteIntent persistence failed
      intent may remain live
- state that a subsequent invocation MAY cause duplicate external execution
- state that this is deferred to future idempotency work

Do not call this an authorization failure or provider failure.

It is a post-provider persistence failure.

==================================================
3. EXACT intent_id PROPAGATION
==================================================

Keep the improved design:

    ExecuteAction(... intentID ...)
        →
    provider invocation
        →
    CompleteIntent(exact intentID)

Do NOT revert to:

    SELECT ... LIMIT 1

Verify that the revised call graph consistently uses the exact intent ID.

Also verify that the source of intentID is explicit and legitimate in the
current architecture.

==================================================
4. GetSnapshotConsequenceParams — VERIFY ACTUAL SQL SHAPE
==================================================

The current plan proposes reusing `sqlAuthorizeResolve` with:

    QueryRowContext(...).Scan(nil, nil, ..., &paramsJSON, nil)

Do NOT assume this works.

Inspect the actual definition of `sqlAuthorizeResolve` and its SELECT column
order in the repository.

Determine the smallest correct implementation that:
- retrieves the approved/non-revoked snapshot's consequence_parameters
- does not duplicate snapshot-resolution semantics outside the kernel
- does not use invalid database/sql Scan destinations
- preserves the kernel's authority semantics

If reusing `sqlAuthorizeResolve` is technically valid, document the exact
column position and correct scan strategy.

If it is not valid, choose the smallest correct alternative.

Do not merely copy the previous plan.

==================================================
5. REVIEW THE POST-AUTHORIZATION WINDOW
==================================================

The current call flow now contains:

    final authorization check
        →
    intent-state read
        →
    snapshot parameter read
        →
    executor invocation

The approved test target's Window B rationale describes the narrow boundary
between the final authorization check and executor invocation.

Because the revised implementation adds these reads, make the implementation
plan's TOCTOU description match the actual call graph.

Do not claim:

    final authorization → immediate executor call

unless that is actually true.

Preserve the accepted v1 Window B limitation, but make the exact boundary
honest.

Do NOT solve this by inventing a new transaction/locking architecture unless
the existing design truly requires it.

==================================================
6. PRESERVE PARAMETER-BINDING SEMANTICS
==================================================

The approved contract establishes two distinct levels:

A. Kernel-level tests:
    TestExec10-13
    deliberately construct wrong AuthorityTuples
    → kernel.Authorize rejects them

B. Execution-path test:
    TestExec36
    caller execution params are ignored
    → service reconstructs from approved snapshot
    → executor receives approved values only

Ensure the implementation plan reflects this distinction.

The actual ExecuteAction path must not use caller-supplied provider parameters
as its source of truth.

The executor remains a thin provider adapter.

==================================================
7. PRESERVE EXECUTOR SELECTION SECURITY
==================================================

Keep:

    authorized action
        →
    fixed executor mapping

For Phase 4C+:

    "deploy" → "github_trigger_workflow"

Never use caller-controlled:

    params["tool_name"]

for consequential executor selection.

==================================================
8. PRESERVE COMPLETEINTENT PROVENANCE
==================================================

The approved contract requires:

    CompleteIntent
    exists on Contract

but:

    no REST handler invokes it
    no MCP tool invokes it
    no caller-controlled path invokes it

The only production path is:

    ExecuteAction
      →
    current authorization
      →
    snapshot reconstruction
      →
    executor
      →
    provider acceptance
      →
    CompleteIntent(exact intentID)

Keep this exact provenance.

==================================================
9. AUDIT SEMANTICS
==================================================

The implementation plan must distinguish:

    authorization granted
    executor invoked
    provider accepted
    CompleteIntent persistence succeeded/failed

In particular:

If provider accepted but CompleteIntent persistence failed:
- do NOT emit a provider-failure event
- do NOT report the external provider as failed
- record the persistence problem truthfully
- preserve the truthful provider/executor result

Do not conflate persistence failure with provider failure.

==================================================
10. FIX PLAN BOOKKEEPING
==================================================

Clean up any inconsistencies in:
- number of files changed
- number of tests
- references to test target sections
- track dependencies
- execution order

The implementation plan currently has references such as "all 54 tests".
Verify the actual approved test target's test count and use consistent numbers.

==================================================
11. SCOPE
==================================================

Do NOT add:

- new services
- schema migrations
- generic executor framework
- multiple providers
- Kubernetes/AWS
- async polling
- retries
- RBAC
- multi-tenancy
- general remote execution API
- Phase 4D idempotency infrastructure

Kernel additions remain minimal and must be justified.

==================================================
DELIVERABLE
==================================================

Update:

    phase4c_plus_imp.md

Do NOT implement code.

Before stopping, perform a consistency review against:

    docs/OS/phase4c_plus_test_target.md

Explicitly report the final decisions for:

A. CompleteIntent failure semantics
B. exact intentID propagation
C. snapshot-parameter retrieval mechanism
D. actual post-authorization TOCTOU boundary
E. ExecutionResult.Success semantics
F. audit semantics for provider acceptance + persistence failure

Then STOP.

The revised implementation plan will be reviewed before implementation
begins.