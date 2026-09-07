Agreed. The next planning revision should make that characterization test **deterministic and binary in its acceptance criterion**, before implementation begins.

Send this to the planning agent:

```text id="p8v6ds"
PHASE 4D — FINAL PLAN REVISION: AUTHORIZE→CLAIM RACE

Do not implement code.

Keep the current Phase 4D architecture unless the analysis below proves that
a kernel change is actually necessary.

==================================================
1. SPLIT THE POST-AUTHORIZATION WINDOW
==================================================

Explicitly distinguish two separate races:

A. Authorize → ClaimIntent gap

    final kernel.Authorize succeeds
        ↓
    authority may be revoked
        ↓
    ClaimIntent(live → executing)

This race exists because ClaimIntent's CAS predicate checks intent state, not
current target authority.

B. ClaimIntent → Provider gap (existing Window B)

    ClaimIntent succeeds
        ↓
    target may be revoked
        ↓
    provider invocation

These have different causes and dispositions.

A is caused by the separation between authority validation and durable
execution ownership.

B is the unavoidable external-side-effect/network boundary already accepted
as a v1 TOCTOU limitation.

Do not describe them as one undifferentiated "Window B."

==================================================
2. ADD EXPLICIT DECISION FOR AUTHORIZE→CLAIM RACE
==================================================

Add an ADR-style decision fork:

Option A:
    Accept the Authorize→ClaimIntent race as a documented v1 limitation.

Option B:
    Close it with an atomic kernel operation that validates current authority
    and claims the intent in the same transaction.

Do NOT select an option implicitly.

For each option document:

- security property gained/lost
- kernel complexity
- schema impact
- service impact
- test impact
- operational implications
- whether the property is materially important for Phase 4D
- whether the change is justified under Solvent's "smallest trusted kernel"
  principle

Reuse the same decision discipline as ADR-0002:
    kernel growth requires a durable security/atomicity justification,
    not merely the existence of a theoretically possible race.

==================================================
3. REQUIRED CHARACTERIZATION TEST — DETERMINISTIC
==================================================

Regardless of whether Option A or Option B is ultimately selected, define the
test BEFORE implementation.

The test must NOT be a prose assertion that behavior "may happen."

It must force the ordering.

Required structure:

    Goroutine A:
        perform final kernel.Authorize
        signal authorization-complete
        wait

    Goroutine B:
        wait for authorization-complete
        perform real target revocation
        signal revocation-complete

    Goroutine A:
        proceed to ClaimIntent

The test must establish, deterministically:

    T1: Authorize returned Allowed=true
    T2: revocation committed
    T3: ClaimIntent executes after revocation

Then the expected outcome MUST be a concrete binary assertion.

For Option A (accept race), the acceptance criterion is:

    ClaimIntent succeeds
    AND intent state becomes `executing`

This proves that the Authorize→Claim gap actually exists.

For Option B (close race), the acceptance criterion is:

    ClaimIntent fails
    AND intent remains non-executing
    AND no provider invocation occurs

Do not use:
    "may succeed"
    "may be allowed"
    "characterize the behavior"
    "best effort"

The test must have an unambiguous expected outcome.

==================================================
4. REQUIRE REAL COMMITTED REVOCATION
==================================================

The revocation must be a real committed target revocation using the existing
kernel mechanism.

Do not fake the authority state in memory.

The test must verify independently that the revocation exists/committed before
ClaimIntent proceeds.

Do not use:
    time.Sleep
    time.After
    polling
    arbitrary scheduling

Use channels or equivalent deterministic synchronization.

==================================================
5. IF OPTION A IS SELECTED
==================================================

Add a named residual-risk entry:

    Authorize→ClaimIntent authority race

State precisely:

    Final authorization and durable execution claim are separate operations.
    Revocation committed between them may not prevent ClaimIntent because
    ClaimIntent validates intent state, not current target authority.

Explicitly distinguish this from:

    ClaimIntent→provider Window B

The two are separate residual races.

Do NOT claim that Phase 4D provides:
    atomic authorization + execution claim

unless Option B is actually selected and implemented.

==================================================
6. IF OPTION B IS SELECTED
==================================================

Do not invent an implementation immediately.

First demonstrate that the atomic operation cannot be expressed safely using
existing kernel transaction machinery without duplicating authority semantics.

If a new kernel primitive is justified, specify:

- exact inputs
- exact transaction boundary
- exact authority facts checked
- exact intent state transition
- exact SQL predicate
- why existing primitives are insufficient
- why this belongs in kernel

The kernel must still remain unaware of:
- GitHub
- provider timeout semantics
- retries
- reconciliation protocols
- workflow completion

==================================================
7. PROVIDER OUTCOME CONTRACT
==================================================

Retain the current generic provider outcome design:

    ProviderAccepted
    ProviderRejected
    ProviderAmbiguous

Generic types live in the neutral `adaptererrors` package.

GitHub adapter:
    GitHub-specific HTTP semantics
        ↓
    adaptererrors.ProviderError

Service:
    consumes generic ProviderOutcome
    ↓
    chooses kernel transition

Do not move GitHub HTTP status inspection into service/authority.

==================================================
8. RECONCILIATION
==================================================

Retain the current reconciliation model:

- only executing intents are reconcilable
- source-state must be enforced atomically
- operator identity is authenticated/audited
- reconciliation remains internal/service-level in Phase 4D
- no generic REST/MCP reconciliation endpoint

Keep:
    executing → executed
    executing → live
    executing → cancelled

and make clear that CancelIntent is an execution-reconciliation transition,
not an authority-revocation primitive.

==================================================
9. API/MCP
==================================================

Keep the approved snapshot as the sole consequence-parameter source.

REST/MCP:
    read approved target snapshot
        ↓
    construct exact AuthorityTuple
        ↓
    existing kernel authority check

Do not add caller-supplied consequence parameters.

Do not create a second authorization engine.

==================================================
10. FINAL PLAN DELIVERABLE
==================================================

Update the Phase 4D plan to include:

1. Distinct Authorize→Claim and Claim→Provider races
2. ADR-style Option A / Option B decision
3. Deterministic acceptance test for Authorize→Claim
4. Explicit binary expected outcome for each option
5. Residual-risk entry if Option A is selected
6. Explicit justification if Option B is selected
7. Generic adaptererrors ProviderOutcome contract
8. Existing reconciliation constraints
9. Existing API/MCP snapshot semantics

Do not implement code.

STOP after the revised plan is complete.
```

This is the right final planning gate. It prevents the exact methodological mistake that already surfaced with the earlier Window B test: **a test should not merely describe an allowed race; it must force the race and have a deterministic expected result**.

After the agent produces this revision, the key decision will be whether we accept the Authorize→Claim race for Phase 4D or spend kernel complexity to close it. That decision should be made explicitly, rather than accidentally encoded by whichever implementation happens to be easiest.
