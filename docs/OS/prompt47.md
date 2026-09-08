Yes. This should be a **separate architectural/security audit of the entire 4A–4E evolution**, not another review of just the latest implementation.

The key difference is that the reviewer should look for **cumulative drift, contradictions between phases, and guarantees that became weaker or stronger accidentally as the system evolved**.

Use this prompt:

```text id="p4a4e-review"
SOLVENT PHASE 4A–4E
INDEPENDENT CUMULATIVE ADVERSARIAL ARCHITECTURE + SECURITY REVIEW

You are an INDEPENDENT senior security architect and systems architect.

Perform a fresh, read-only adversarial review of the COMPLETE Solvent evolution
from Phase 4A through Phase 4E.

This is NOT a review of only the latest implementation.

Your task is to determine whether the cumulative architecture developed across
Phases 4A, 4B, 4C, 4C+, 4D, and 4E is still coherent, secure, minimal, and
internally consistent.

The central question is:

    Did Solvent preserve its original authority-kernel thesis while evolving
    from a canonical API contract into a real externally consequential
    execution system exposed through REST and MCP?

Try to prove that the answer is NO.

Only return GO if you cannot establish a material security or architectural
failure.

==================================================
REVIEW MODE
==================================================

READ-ONLY.

DO NOT:
- modify source code
- modify tests
- modify plans
- modify documentation
- fix findings
- refactor
- create commits

You MAY:
- inspect repository history
- inspect current source
- inspect old plans/specifications
- inspect tests
- inspect OpenAPI/MCP contracts
- inspect SQL/migrations
- inspect proof artifacts
- run builds/tests/static analysis
- perform code search
- compare implementations across phases
- perform adversarial reasoning
- perform mutation analysis

Do not rely on developer summaries or "GO" reports.

Determine the actual state from the repository and available phase artifacts.

==================================================
REPOSITORY
==================================================

Repository:

    ~/Documents/go/solvent-main

First inspect:

    git status --short
    git branch --show-current
    git log --oneline --decorate -20

Determine whether phase-specific plans, ADRs, test targets, review reports,
OpenAPI files, proof artifacts, and implementation commits are available.

Search for:

    Phase 4A
    Phase 4B
    Phase 4C
    Phase 4C+
    Phase 4D
    Phase 4E
    ADR
    test target
    adversarial review
    OpenAPI
    MCP
    authority
    execution

==================================================
HISTORICAL REVIEW REQUIREMENT
==================================================

Do not review Phase 4E as though it appeared in isolation.

Build a timeline:

    Phase 4A
       ↓
    Phase 4B
       ↓
    Phase 4C
       ↓
    Phase 4C+
       ↓
    Phase 4D
       ↓
    Phase 4E

For each phase identify:

- intended objective
- architectural decisions
- new authority/security semantics
- new API semantics
- new state
- new trust boundaries
- new external side effects
- tests added
- known limitations
- deferred items

Then determine whether later phases accidentally invalidate earlier assumptions.

==================================================
ORIGINAL SOLVENT THESIS
==================================================

Treat this as the original architectural north star:

    Evidence ≠ Authority
    Agent claim ≠ Authority
    Confidence ≠ Authority
    TOKEN ≠ AUTHORITY
    Authorize ≠ Execute

The kernel is the smallest trusted authority core.

Canonical philosophy:

    External protocol/product → Adapter
    Policy/orchestration   → Service/Policy
    Execution/infrastructure → Executor/Deployment
    Product/UI/reporting    → Service/read model
    Durable invariant       → DB constraint
    New atomic security primitive → Kernel only when unavoidable
    Everything else        → Don't add it

Evaluate EVERY phase against this principle.

==================================================
PHASE 4A REVIEW
==================================================

Determine whether Phase 4A correctly established a canonical, language-neutral
API contract.

Review:

- HTTP/JSON
- OpenAPI
- MCP
- A2A posture
- API/service separation
- canonical semantics
- SDK status
- identity semantics
- forbidden endpoint design

Attack for:
- duplicate authorization semantics
- API-only security guarantees
- endpoint semantics not shared with service layer
- token-as-authority confusion
- caller-controlled security state
- API contracts that later became inconsistent with implementation

Question:

    Was Phase 4A's API contract actually a faithful projection of the authority
    model, or did it begin introducing a second interpretation of authority?

==================================================
PHASE 4B REVIEW
==================================================

Review the implementation of the canonical API/service architecture.

Verify:

- kernel.Authorize remains sole authority oracle
- Approve creates authority
- Authorize verifies authority
- RevokeTarget revokes authority
- exact action/target binding
- snapshot semantics
- principal semantics
- DB invariants
- REST/MCP convergence
- actor vs identity vs authentication
- audit semantics
- TOKEN != AUTHORITY
- no workflow-token authority bypass
- no second authorization engine

Inspect whether later phases relied on guarantees that Phase 4B never
actually established.

Especially investigate:

- identity authentication
- caller-supplied actor/principal values
- target activation
- snapshot immutability
- intent binding
- TOCTOU assumptions
- CockroachDB transaction assumptions

==================================================
PHASE 4C REVIEW
==================================================

Review:

- OpenAPI 3.1
- reference client
- GitHub reference integration
- API contract consistency
- integration boundaries

Verify no API contract drift was introduced.

Ask:

    Did Phase 4C turn documentation into an interface without proving the
    underlying security semantics?

Check:
- REST/MCP convergence
- server-side identity binding
- consequence parameter semantics
- target/action binding
- no forbidden endpoints
- extension-plane architecture

==================================================
PHASE 4C+ REVIEW
==================================================

This was the first real external side-effect phase.

Reconstruct the complete execution model:

    Authorize
      ↓
    exact target snapshot
      ↓
    exact intentID
      ↓
    fixed executor
      ↓
    GitHub provider
      ↓
    CompleteIntent
      ↓
    audit

Verify:

- caller parameters ignored
- snapshot is source of truth
- executor cannot mutate authority
- executor selection cannot be caller-controlled
- provider credentials are not authority
- provider acceptance != workflow completion
- exact intent binding
- CompleteIntent ordering
- CI-2 semantics
- audit semantics
- Window A
- Window B
- persistence-failure semantics
- no false atomicity claim

Review all original 4C+ findings and verify whether the final implementation
actually closed them.

Do NOT assume a previously reported GO is correct.

==================================================
PHASE 4D REVIEW
==================================================

Review the consequential-execution state model:

    live
      ↓
    executing
      ↓
    executed

and:

    definitive rejection → live
    ambiguous outcome   → executing
    executing survives retraction
    reconciliation required for ambiguity

Verify:

- ClaimIntent is atomic
- concurrent duplicate claims are prevented
- ambiguous provider state cannot accidentally become retryable
- crash recovery is honest
- CompleteIntent is source-state guarded
- RollbackClaim is source-state guarded
- CancelIntent is source-state guarded
- RetractCascade does not erase executing attempts
- provider outcome classification is generic at service boundary
- GitHub-specific semantics remain outside kernel

==================================================
PHASE 4D — RACE A / RACE B
==================================================

Explicitly analyze both races:

Race A:

    Authorize → ClaimIntent

Race B:

    ClaimIntent → Provider

Verify the project correctly distinguishes them.

Race A is an accepted v1 residual risk.

Race B is an accepted external TOCTOU limitation.

Do not automatically flag either one as a defect.

Instead determine whether the implementation or documentation accidentally
claims stronger guarantees than the architecture provides.

Review TestExec55 and TestExec15B.

Both must be deterministic and materially useful.

==================================================
PHASE 4E REVIEW
==================================================

Review the new public execution boundary:

    REST:
        POST /v1/authorizations/execute

    MCP:
        solvent_execute

and activity access:

    solvent_activity

Verify these do NOT create a second security model.

Trace:

    authentication / trusted boundary
        ↓
    handler
        ↓
    authority.Service
        ↓
    kernel
        ↓
    intent
        ↓
    executor
        ↓
    provider

==================================================
CROSS-PHASE PRINCIPAL / IDENTITY AUDIT
==================================================

This is a mandatory deep-dive.

Trace principal identity through 4A → 4B → 4C+ → 4D → 4E.

Determine:

- where identity originates
- where it is authenticated
- where it is merely attributed
- where it is trusted
- whether any later API weakened an earlier assumption
- whether REST and MCP intentionally have different trust models

Specifically inspect:

REST:
    Bearer token → authenticated principal

MCP:
    trusted-local-stdio → actor_id attribution

Verify the distinction is intentional and correctly documented.

Attempt identity-confusion attacks.

==================================================
CROSS-PHASE SNAPSHOT / PARAMETER AUDIT
==================================================

Trace consequence_parameters through every phase.

Required invariant:

    approved snapshot
        ↓
    authority decision
        ↓
    execution
        ↓
    provider

Caller input must never become the authoritative consequential tuple.

Attack with:
- repo substitution
- workflow substitution
- ref substitution
- arbitrary provider fields
- malformed parameter sets
- mutated maps
- target substitution
- stale caller state

Determine whether any API, MCP, or executor path weakens the snapshot invariant.

==================================================
CROSS-PHASE INTENT AUDIT
==================================================

Trace action intent semantics from introduction through Phase 4E.

Determine:

- who creates intents
- who can execute them
- what makes them live
- what binds them to principal/scenario/belief/action/target
- how ClaimIntent works
- how CompleteIntent works
- how cancellation works
- how reconciliation works
- whether intentID has effectively become a bearer capability

Attack:
- cross-principal intent execution
- cross-scenario intent execution
- wrong target
- wrong belief
- wrong action
- duplicate execution
- stale intent
- fabricated intentID
- cancelled intent
- executed intent
- ambiguous intent

==================================================
CROSS-PHASE AUTHORIZATION / EXECUTION SEPARATION
==================================================

Verify this remains true across every API surface:

    Authorize != Execute

The agent must not be able to transform:
- evidence into authority
- belief into authority
- confidence into authority
- token into authority
- intent into authority
- provider response into authority

Check whether Phase 4E's execution endpoints accidentally blur authorization
and execution semantics even if they call the same service.

==================================================
CROSS-PHASE KERNEL GROWTH AUDIT
==================================================

Produce the complete list of kernel primitives introduced during 4A–4E.

For each one:

- original reason
- phase introduced
- durable fact enforced
- transaction atomicity requirement
- whether it genuinely belongs in kernel
- whether it could safely have remained in service layer
- whether later phases made it unnecessary

Pay special attention to:

    CompleteIntent
    ClaimIntent
    RollbackClaim
    CancelIntent

Verify:

    AuthorizeAndClaimIntent does NOT exist

Determine whether the kernel has accumulated accidental product logic.

==================================================
CROSS-PHASE DATABASE / TRANSACTION AUDIT
==================================================

Review all schema changes and invariants introduced across 4A–4E.

Determine:

- which invariants are database-enforced
- which are service-enforced
- which are kernel-enforced
- which are merely tested

Look for:
- duplicated constraints
- stale assumptions about constraint names
- migration drift
- transaction misuse
- CockroachDB SERIALIZABLE misunderstandings
- TOCTOU assumptions
- state transitions without source-state predicates

Specifically verify the Phase 4D migration did not leave old constraints behind.

==================================================
CROSS-PHASE AUDIT / OBSERVABILITY AUDIT
==================================================

Trace all audit semantics.

Determine whether the system can distinguish:

    authorization
    intent creation
    execution claim
    provider invocation
    provider acceptance
    provider failure
    ambiguous outcome
    intent completion
    reconciliation

Verify later phases did not create contradictory event semantics.

Check:
- operator attribution
- actor attribution
- provider output
- credential leakage
- audit persistence failure
- reconciliation traceability

==================================================
CROSS-PHASE REST / MCP CONVERGENCE
==================================================

Compare equivalent REST and MCP operations across all phases.

They do NOT have to have identical authentication models.

They MUST have equivalent authority semantics once an effective trusted
principal/actor has been established.

Build a matrix:

    Operation
    REST behavior
    MCP behavior
    Same security semantics?
    Justified difference?

Include:

- authorize
- execute
- activity
- target operations
- belief operations
- evidence operations

Flag any unexplained semantic divergence.

==================================================
CROSS-PHASE PROVIDER / EXECUTOR BOUNDARY
==================================================

Verify that all GitHub-specific semantics remain outside the kernel and
service authority model.

Inspect:

    adapter/github
    service/executor
    service/authority

Verify:
- provider credentials
- HTTP behavior
- error classification
- executor registry
- fixed action mapping
- provider outcome semantics
- no authority mutation from provider
- no API handler directly invoking GitHub

==================================================
FULL END-TO-END ATTACKS
==================================================

Attempt these attack chains across the entire system:

1. Poisoned evidence → belief → approval → malicious execution

2. Fabricated agent claim → authorization

3. Caller-controlled consequence parameters → GitHub

4. Caller-controlled executor selection → GitHub

5. Cross-principal intent → execution

6. Wrong scenario → execution

7. Revoked authority → execution

8. Stale authorization → execution

9. Intent replay → duplicate provider call

10. Concurrent REST + MCP execution of same intent

11. Ambiguous GitHub response → retry → duplicate workflow

12. Reconciliation abuse → unauthorized state transition

13. Audit failure → false execution result

14. Provider response → authority mutation

15. Credential leakage → audit/API/log

16. MCP local-trust boundary → unintended remote exposure

==================================================
TEST SUITE INTEGRITY ACROSS PHASES
==================================================

Do not count tests.

Build a traceability matrix:

    Security invariant
        ↓
    Phase that introduced it
        ↓
    Current implementation
        ↓
    Current test
        ↓
    Does the test actually prove it?

Identify:

- stale tests
- tests that test deleted behavior
- placeholder tests
- tests that assert implementation rather than invariant
- test gaps hidden by higher-level tests
- tests that prove service behavior but not API boundary behavior
- tests that prove handler behavior but not actual provider behavior

==================================================
MUTATION ANALYSIS
==================================================

Across the complete 4A–4E system, simulate removal or corruption of:

- kernel.Authorize
- principal authentication
- snapshot parameter binding
- exact intent binding
- ClaimIntent
- CompleteIntent
- RollbackClaim
- reconciliation source-state guard
- fixed executor mapping
- provider outcome classification
- REST authentication
- MCP trust boundary
- activity scenario filter
- audit logging
- provider credential isolation

For each:

    Which tests fail?

If no test catches the regression, flag the coverage gap.

==================================================
ARCHITECTURAL DRIFT
==================================================

Search for evidence that the system has drifted from:

    small authority kernel
    +
    adapters/services/executors around it

Look specifically for:

- product logic in kernel
- GitHub logic in kernel
- policy duplicated across services
- API-specific security logic
- MCP-specific security logic
- hidden authority state in UI
- token-as-authority semantics
- arbitrary executor selection
- generic workflow-engine creep
- unnecessary abstractions
- dead infrastructure
- deprecated workflow token machinery
- abandoned services
- contradictory documentation

==================================================
SECURITY CLAIM AUDIT
==================================================

List every major security guarantee the project currently appears to make.

For each, classify:

    PROVEN
    PARTIALLY PROVEN
    TESTED BUT OVERSTATED
    DOCUMENTED BUT UNPROVEN
    FALSE

Pay special attention to phrases like:

- authorized
- safe
- secure
- prevented
- exactly once
- at most once
- immutable
- revocable
- authenticated
- trusted
- audited
- completed

Do not allow terminology to exceed what the implementation proves.

==================================================
DOCUMENTATION / IMPLEMENTATION DRIFT
==================================================

Cross-check:

- ADRs
- phase plans
- OpenAPI
- MCP descriptions
- README
- examples
- comments
- proof artifacts
- actual code

Look for:
- nonexistent endpoints
- stale method names
- old state names
- stale kernel primitives
- contradictory trust models
- claims of completion vs provider acceptance
- stale phase assumptions

==================================================
BUILD / VERIFICATION
==================================================

Run the strongest practical current verification:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task test

Also run relevant targeted suites for:

    kernel
    service/authority
    service/audit
    adapter/github
    api
    cmd/solvent-mcp

Run repository verification scripts if present.

Do not accept skipped tests caused by missing infrastructure.

==================================================
FINDING SEVERITY
==================================================

CRITICAL:
    bypasses authority
    arbitrary consequential execution
    cross-principal execution
    credential exposure
    material integrity failure

HIGH:
    significant security boundary failure
    serious authorization inconsistency
    consequential state corruption
    dangerous API/MCP bypass

MEDIUM:
    meaningful security weakness
    incomplete enforcement of an important invariant
    materially misleading security behavior

LOW:
    observability or defense-in-depth issue
    non-blocking architectural weakness

INFO:
    documentation/test-quality/hygiene

CRITICAL/HIGH/MEDIUM => NO-GO.

LOW/INFO => do not automatically block GO.

==================================================
FINAL VERDICT
==================================================

Return:

    VERDICT: GO

ONLY if:

- no CRITICAL
- no HIGH
- no MEDIUM
- cumulative 4A–4E architecture is coherent
- security claims do not materially exceed implementation guarantees
- later phases did not silently weaken earlier invariants

Otherwise:

    VERDICT: NO-GO

==================================================
FINAL REPORT
==================================================

Start with:

    VERDICT: GO
or
    VERDICT: NO-GO

Then produce:

1. Executive assessment

2. Phase-by-phase assessment:
   4A
   4B
   4C
   4C+
   4D
   4E

3. Cumulative architecture diagram

4. Security invariant evolution

5. Kernel growth audit

6. Identity/authentication evolution

7. Authority/execution separation audit

8. Snapshot/parameter binding audit

9. Intent lifecycle audit

10. REST/MCP convergence audit

11. Executor/provider boundary audit

12. Database/transaction invariant audit

13. Audit/observability audit

14. Residual risk inventory

15. Security-claim audit

16. Documentation/implementation drift

17. Test traceability matrix

18. Mutation analysis

19. Full verification results

20. Findings table

21. Detailed findings

22. Recommended remediation priority

23. Final architectural conclusion

==================================================
FINAL PRINCIPLE
==================================================

Do not judge Solvent by how much functionality it has accumulated.

Judge it by whether the accumulation has preserved the original idea:

    Untrusted intelligence
            ↓
    authoritative decision
            ↓
    exact authorized intent
            ↓
    controlled consequential action

without allowing the surrounding layers to quietly become a second authority
system.

Try to break that chain from ANY layer introduced during Phase 4A–4E.

Only return GO if it survives.
```

This review is worth doing separately because the strongest remaining risk is no longer necessarily a bug in one phase. It is **architectural drift across phases**—for example, a guarantee established in 4B being subtly weakened by a new REST/MCP path in 4E, or a state invariant being enforced in one path but merely assumed in another.

I would treat the result as the **architecture baseline audit for everything built through 4E**. If it comes back GO, that gives you a much stronger foundation for the next chapter than another phase-local review.
