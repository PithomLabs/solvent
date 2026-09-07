`phase4d_plan4.md` is now **very close to implementation-ready**, and the planning process is doing what it should: the two post-authorization races are explicitly separated, the test for the first race is deterministic, and the provider-outcome dependency is now correctly neutral.  

I would make **one substantive decision now: choose Option A for Race A for Phase 4D**, with explicit residual-risk documentation, rather than adding `AuthorizeAndClaimIntent`.

### Why I recommend Option A

The plan itself already establishes the distinction:

```text
Authorize → Claim
```

is a narrow database-to-database race with **no external side effect yet**, whereas:

```text
Claim → Provider
```

crosses the unavoidable external network boundary. 

Option B is technically defensible, but it introduces a composite authority/execution-ownership primitive into the kernel solely to eliminate a very small residual window. The plan's own Option A assessment says the external provider has not yet been invoked at that boundary and the window is normally very small. 

That is exactly the kind of case where Solvent's **small-kernel discipline** should prevail.

More importantly, Option A is now no longer being hidden or hand-waved. The plan explicitly names:

```text
RR-1 = Authorize → Claim race
RR-2 = Claim → Provider Window B
```

and gives them separate dispositions. 

That is enough to make the limitation honest.

### One correction to the acceptance wording

The current `TestExec55` definition is good, but the plan says:

> “Regardless of whether Option A or Option B is selected…”

and then changes the expected result based on the choice. 

That's fine as a **plan template**, but once the ADR chooses A, the acceptance test should become unconditionally:

```text
Authorize succeeds
→ real revocation commits
→ ClaimIntent succeeds
→ intent = executing
```

There should be no runtime/configuration-dependent acceptance criterion. The chosen architecture must determine the fixed expected result before implementation.

### One thing I would add to the plan

Add one explicit statement under Option A:

```text
Race A is accepted only because no external provider invocation occurs before
ClaimIntent. Once ClaimIntent succeeds, the durable `executing` state becomes
the execution-attempt record and the Claim→Provider race is governed by the
existing Window B limitation.
```

That prevents future readers from interpreting RR-1 as equivalent to the more consequential post-claim race.

---

## One subtle issue to preserve during implementation

The service must continue to distinguish:

```text
definitive rejection → executing → live
ambiguous outcome   → executing → executing
```

The new `adaptererrors.ProviderOutcome` design gets this right and keeps GitHub-specific HTTP classification in the adapter. 

Do not let implementation drift into:

```go
if err != nil {
    RollbackClaim(...)
}
```

because that would recreate the duplicate-execution vulnerability the phase is specifically intended to eliminate.

---

## My final decision

I would lock Phase 4D as:

```text
Race A: Authorize → Claim
    ACCEPTED RESIDUAL RISK

Race B: Claim → Provider
    ACCEPTED RESIDUAL RISK

Concurrency:
    CLOSED via ClaimIntent CAS

Ambiguous external outcome:
    FROZEN in executing until reconciliation

Definitive rejection:
    rollback to live

Approved parameters:
    snapshot remains sole source

Provider classification:
    generic adaptererrors contract

Kernel:
    ClaimIntent
    RollbackClaim
    CompleteIntent
    CancelIntent
```

The resulting state machine is coherent and explicitly documented. 

### Send this to the planning agent

```text
PHASE 4D — FINAL ADR DECISION

Choose OPTION A for the Authorize→ClaimIntent race.

Do NOT add AuthorizeAndClaimIntent in Phase 4D.

Rationale:

- The race is real and must remain explicitly documented.
- No external provider side effect has occurred at the ClaimIntent boundary.
- ClaimIntent provides durable execution ownership.
- Adding a composite kernel primitive here would increase trusted-kernel
  complexity for a narrow residual race.
- Solvent's smallest-trusted-kernel discipline favors explicit acceptance of
  this limitation over premature kernel growth.

Make TestExec55 deterministic and unconditional for the chosen architecture:

    final Authorize succeeds
        →
    real RevokeTarget commits
        →
    ClaimIntent succeeds
        →
    intent state == executing

No "depends on Option A/B" language should remain in the final acceptance
criteria after the ADR decision.

Add explicit wording that Race A is distinct from Claim→Provider Window B and
that no external provider invocation has occurred at the Race A boundary.

Keep all other decisions unchanged:

- ClaimIntent is the sole authoritative ownership gate
- live → executing is atomic CAS
- executing survives RetractCascade
- definitive provider rejection → live
- ambiguous outcome → executing
- ambiguous outcome is not retried automatically
- reconciliation requires executing source state
- ReconcileIntent remains internal and privileged
- ProviderOutcome/ProviderError remain in neutral adaptererrors
- GitHub-specific classification remains in adapter/github
- snapshot remains sole consequence-parameter source
- explicit HTTP timeout
- SetTestHook removed from production
- no new schema semantics beyond executing state
- no async execution
- no distributed transaction
- no exactly-once claim

Update the ADR, residual-risk table, TestExec55 acceptance criteria, and
security invariants accordingly.

STOP after the plan is finalized. Do not implement code yet.
```

After that, **I would lock the plan and implement**. The plan has reached the point where further abstract architecture iteration is likely to yield diminishing returns; the remaining risks are now explicit rather than hidden.



## claude

This closes it out properly, and the discipline in how it does so is worth confirming point by point rather than just approving.

**The ADR structure is exactly right.** Race A and Race B get separate identity, separate cause, and separate disposition tables rather than being collapsed back into one "post-authorization window" — the comparison table in §4 (what CAS checks, what Authorize checked, whether it can be closed atomically, whether external side effect is possible) is precisely the distinction that mattered, made explicit rather than left implicit.

**TestExec55 is genuinely deterministic, and it generalizes the fix correctly.** Channel-synchronized, binary assertions for both possible dispositions, a real `RevokeTarget` call rather than a fake, and — most importantly — a "Required properties" list that states "no 'may succeed', no 'may be allowed', no 'characterize the behavior'" as a standing rule. That's not just fixing this one test; it's converting the exact failure mode Kilo caught in the old Window B test into a written requirement future tests have to meet. Good.

**`adaptererrors` is now used consistently** throughout the classification and service-mapping code (lines 619-672) — the ambiguous "or a shared package" wording is gone from what I can see; this reads clean.

**On the actual decision — my recommendation is Option A, and here's a reason the document doesn't quite draw out itself:** both options' risk tables still carry Window B as an accepted v1 limitation, unconditionally — even Option B's own analysis (§4) concedes that Race A alone can't cause an external side effect, since the provider hasn't been invoked at the ClaimIntent boundary. That means choosing Option B closes the *lower-consequence* race (revocation racing a database-only state change) with a new kernel primitive, while leaving the *higher-consequence* race (revocation racing an actual external GitHub call) explicitly accepted in both options. That's a slightly incoherent prioritization if the goal is risk reduction rather than architectural completeness for its own sake — you'd be adding kernel surface to close the race that matters less, while the race that matters more stays open regardless. I'd take Option A for v1: document it, ship `TestExec55` proving it exists and stays narrow, and revisit both Race A and Window B together later if a real provider-side idempotency mechanism (which would help with Window B, not just Race A) ever becomes justified by actual operational experience. That's consistent with how this project has treated every other "we can imagine a race" question so far — Option B remains available and well-specified in §159-196 if a future incident or a customer requirement changes the calculus, but nothing here compels making that call now.

Everything else — the `TestExec55` entry correctly appearing in both the acceptance-test table and the implementation order regardless of which option is picked, `internal/m0/gate.go` correctly left untouched, the residual-risk tables correctly differing by option rather than being written once and hoped to cover both — is right. This is ready to proceed once Option A vs. B is actually decided; I don't have anything further blocking it.


## chatgpt

Yes. I agree with that reasoning.

**Choose Option A and lock the Phase 4D plan.**

The important strategic point is exactly what you identified: Option B spends trusted-kernel complexity to eliminate a race that cannot itself produce an external side effect, while the higher-consequence **ClaimIntent → provider** race remains in either design. That makes Option A the better risk/complexity tradeoff for v1.

The resulting position is clean:

```text
Race A
Authorize → ClaimIntent
    → accepted residual risk
    → deterministic TestExec55

Race B
ClaimIntent → Provider
    → accepted residual risk
    → existing Window-B characterization

Concurrent duplicate claims
    → CLOSED by atomic ClaimIntent CAS

Ambiguous external outcome
    → remains executing
    → reconciliation required
```

That is consistent with the small-kernel principle and avoids adding `AuthorizeAndClaimIntent` without a demonstrated operational need.

### Final instruction to OpenCode

```text
PHASE 4D — LOCK ARCHITECTURE AND IMPLEMENT

The Phase 4D architecture is now APPROVED with OPTION A.

FINAL DECISION:

Accept the Authorize→ClaimIntent race as an explicit v1 residual risk.

Do NOT implement AuthorizeAndClaimIntent.

The rationale is:

- Race A cannot itself cause an external provider side effect because the
  provider is not invoked until after ClaimIntent.
- Race B, which does border the actual external side effect, remains an
  unavoidable accepted v1 TOCTOU boundary in either Option A or Option B.
- Adding a composite kernel primitive would therefore increase trusted-kernel
  complexity without eliminating the higher-consequence provider race.
- Solvent's smallest-trusted-kernel discipline favors accepting and explicitly
  characterizing Race A for v1.
- Option B remains documented as a possible future response to a demonstrated
  security incident or concrete customer requirement.

==================================================
LOCKED ARCHITECTURE
==================================================

Race A:

    final Authorize
        ↓
    target may be revoked
        ↓
    ClaimIntent
        ↓
    claim may still succeed

Status:
    ACCEPTED V1 RESIDUAL RISK

TestExec55 MUST deterministically demonstrate exactly this behavior:

    Authorize succeeds
        →
    real RevokeTarget commits
        →
    ClaimIntent succeeds
        →
    intent state == executing

No probabilistic race.

Race B:

    ClaimIntent
        ↓
    target may be revoked
        ↓
    provider invocation

Status:
    ACCEPTED V1 TOCTOU LIMITATION

Do not claim atomic authorization + external execution.

==================================================
LOCKED STATE MODEL
==================================================

    live → executing → executed

Definitive provider rejection:

    executing → live

Ambiguous provider outcome:

    executing → executing

    No automatic retry.
    Reconciliation required.

RetractCascade:

    live → cancelled

    executing is NOT touched.

Reconciliation may resolve:

    executing → executed
    executing → live
    executing → cancelled

Only authenticated/privileged reconciliation may perform those resolution
operations, with source-state enforcement and audit.

==================================================
LOCKED KERNEL SCOPE
==================================================

Add only:

    ClaimIntent
    RollbackClaim
    CancelIntent
    updated CompleteIntent transition

Do NOT add:

    AuthorizeAndClaimIntent

The kernel must remain unaware of:
- GitHub
- provider HTTP semantics
- retries
- timeout classification
- reconciliation protocol
- workflow completion

==================================================
LOCKED PROVIDER CONTRACT
==================================================

Generic:

    adaptererrors.ProviderOutcome

with:

    ProviderAccepted
    ProviderRejected
    ProviderAmbiguous

GitHub adapter performs GitHub-specific classification.

Service consumes only generic ProviderOutcome.

Service must NOT inspect GitHub-specific HTTP status semantics.

==================================================
LOCKED EXECUTION FLOW
==================================================

ExecuteAction:

    1. PrepareForAction / kernel.Authorize
    2. Resolve fixed executor
    3. Reconstruct approved snapshot parameters
    4. Validate local parameters
    5. ClaimIntent(live → executing)
    6. adapter_invoked audit
    7. provider invocation

Provider result:

    definitive rejection
        → RollbackClaim
        → live

    ambiguous
        → remain executing

    accepted
        → CompleteIntent
        → executed

==================================================
LOCKED API/MCP MODEL
==================================================

REST/MCP authorization reads consequence_parameters from the approved target
snapshot.

Caller does NOT provide substitute consequence parameters.

The snapshot read is only input construction.

kernel.Authorize remains the sole authority oracle.

==================================================
LOCKED TEST REQUIREMENTS
==================================================

TestExec55 is mandatory and must be deterministic.

It must verify:

    Authorize allowed
    +
    RevokeTarget committed
    +
    ClaimIntent succeeded
    +
    intent == executing

No "may succeed" language.

Existing Phase 4C+ tests must continue passing.

Add Phase 4D acceptance/adversarial tests specified in the plan.

==================================================
IMPLEMENTATION
==================================================

Now implement the locked Phase 4D plan.

Do not introduce architecture beyond the approved plan.

Do not reopen the Option A/B decision.

Do not add idempotency beyond the defined executing-state/CAS model.

Do not add:
- distributed transactions
- exactly-once guarantees
- provider polling
- retries beyond definitive rejection retry
- generic workflow infrastructure
- additional providers
- RBAC/IAM
- multi-tenancy
- UI redesign

==================================================
VERIFICATION
==================================================

After implementation run:

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

Also run focused suites for:

    kernel
    service/authority
    adapter/github
    API/MCP

Verify TestExec55 genuinely forces the race and does not rely on timing.

Verify there is no:

    AuthorizeAndClaimIntent

Verify there is no:

    GetSnapshotConsequenceParams

Verify service does not inspect GitHub HTTP status codes directly.

Verify:
    caller params remain non-authoritative
    executor selection remains fixed
    exact intentID remains enforced
    executing survives RetractCascade
    ambiguous failures remain executing
    definitive failures return to live

==================================================
FINAL REPORT
==================================================

Report:

1. Implementation completed
2. Files changed
3. Schema changes
4. Kernel primitives added
5. Test counts
6. TestExec55 exact behavior
7. Provider outcome classification
8. Verification results
9. Remaining accepted v1 risks
10. Confirmation that Option A remains the final architectural decision

STOP after implementation and verification.
```

At this point I would **stop the planning loop**. Phase 4D has a coherent architectural boundary, an explicit risk decision, and deterministic tests for the remaining race. The implementation should now prove the design rather than repeatedly redesign it.
