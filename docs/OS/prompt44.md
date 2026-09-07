PHASE 4D — FINAL INDEPENDENT ADVERSARIAL SECURITY / ARCHITECTURE REVIEW

You are an INDEPENDENT senior security architect conducting the final acceptance
review of Solvent Phase 4D.

Phase 4C+ previously received independent GO.

Phase 4D is now implemented and the developer reports:

    go build ./...        PASS
    go vet ./...          PASS
    go test -count=1 ./... PASS
    go test -race ./...   PASS
    20 packages          PASS

Do NOT trust these claims. Verify the actual repository.

This is a READ-ONLY review.

DO NOT:
- modify files
- fix code
- refactor
- add tests
- change architecture
- rewrite plans
- weaken acceptance criteria
- create commits

You may inspect all source, SQL, migrations, tests, documentation, git state,
and run commands.

Your objective is to TRY TO BREAK THE IMPLEMENTATION.

==================================================
REPOSITORY
==================================================

Review:

    ~/Documents/go/solvent-main

First inspect:

    git status --short
    git branch --show-current
    git log -1 --oneline

Then inspect the complete Phase 4D implementation.

Do NOT rely on developer summaries.

==================================================
PHASE 4D ARCHITECTURAL CONTRACT
==================================================

The approved Phase 4D model is:

    live
      │
      │ ClaimIntent
      ▼
    executing
      │
      ├── definitive provider rejection
      │        ↓
      │      live
      │
      ├── provider acceptance
      │        ↓
      │      executed
      │
      └── ambiguous provider outcome / crash
               ↓
            executing
            (frozen until reconciliation)

RetractCascade:

    live → cancelled

RetractCascade MUST NOT erase an executing execution-attempt record.

Phase 4D does NOT claim exactly-once external execution.

It does claim:

    at most one provider attempt can be claimed concurrently for a given
    Solvent intent under Solvent-controlled concurrency.

==================================================
LOCKED DECISIONS
==================================================

### Race A — Authorize → ClaimIntent

This race is ACCEPTED as a v1 residual risk.

Sequence:

    final kernel.Authorize succeeds
        ↓
    target may be revoked
        ↓
    ClaimIntent(live → executing)

Because ClaimIntent checks intent state, not current authority, the claim may
still succeed.

This must be explicitly documented.

DO NOT treat this as a defect merely because the race exists.

DO flag:
- claims that Race A is impossible
- claims that Phase 4D atomically binds authorization to execution ownership
- accidental implementation changes that alter the documented semantics

### Race B — ClaimIntent → Provider

This remains the accepted v1 TOCTOU boundary.

Sequence:

    ClaimIntent succeeds
        ↓
    target may be revoked
        ↓
    provider invocation

This cannot be atomically coordinated with an external provider in the current
architecture.

### Concurrency

ClaimIntent is the authoritative ownership gate:

    UPDATE action_intent
    SET state = 'executing'
    WHERE id = ?
      AND scenario_id = ?
      AND state = 'live'

Exactly one concurrent claimant should succeed.

### Ambiguity

Definitive provider rejection:

    executing → live

Ambiguous outcome:

    executing → executing

No automatic retry from an ambiguous state.

Reconciliation is required.

### Retraction

Executing intents survive RetractCascade.

Retraction prevents creation of new intents by removing the authority basis;
it does NOT erase an already-started execution attempt.

### Provider outcome classification

Generic provider outcome types live in:

    adaptererrors

Expected outcomes:

    ProviderAccepted
    ProviderRejected
    ProviderAmbiguous

GitHub-specific HTTP classification belongs in:

    adapter/github

The service must consume the generic outcome rather than inspect GitHub-specific
HTTP semantics.

### Reconciliation

Reconciliation is:

- internal/service-level in Phase 4D
- privileged
- authenticated
- auditable
- source-state guarded
- only applicable to executing intents

No generic REST/MCP reconciliation endpoint exists.

### API/MCP consequence parameters

REST and MCP authorization obtain consequence_parameters from the approved target
snapshot.

Caller-supplied consequence parameters must NOT substitute for the snapshot.

The snapshot read is input construction only.

kernel.Authorize remains the sole authority oracle.

### HTTP provider

The provider should have:

- explicit configurable timeout
- sensible default, approximately 30 seconds
- bounded/truncated error response body
- generic wrapped API errors where appropriate
- credentials only from construction-time configuration
- no token logging
- no token in execution params
- no token in kernel/audit/API state

==================================================
MOST IMPORTANT NEW PHASE 4D INVARIANTS
==================================================

Verify these independently:

CI-4:
    At most one provider invocation can be claimed concurrently for one intent.

CI-5:
    Ambiguous execution requires reconciliation before retry.

CI-6:
    Consequence parameters always come from the approved snapshot.

CI-7:
    Executing intents survive retraction.

CI-8:
    Only definitive provider rejection allows retry.

CI-9:
    Provider outcome classification is adapter-specific while the outcome
    contract itself is provider-neutral.

CI-10:
    Reconciliation is source-state guarded.

==================================================
PART I — CLAIMINTENT ATOMICITY
==================================================

Inspect the actual SQL and transaction behavior.

Attempt to break:

- two concurrent claims
- ten concurrent claims
- claims from separate DB connections
- claims across separate processes
- stale reads before claim
- repeated claim attempts

Verify that:

    exactly one
        →
    live → executing

and every other concurrent claimant is refused.

Do NOT accept a test that merely runs two sequential calls.

Check:
- transaction boundaries
- RowsAffected handling
- CockroachDB retry behavior
- error handling
- scenario binding
- exact intent binding

Verify the pre-claim read, if any, has no authority or correctness role.

==================================================
PART II — AUTHORIZATION → CLAIM RACE
==================================================

Review TestExec55.

The test must genuinely establish:

    final Authorize succeeds
        ↓
    real RevokeTarget commits
        ↓
    ClaimIntent executes afterward

No:
- sleep
- time.After
- polling
- probabilistic scheduling

The expected Option A result is deterministic:

    ClaimIntent succeeds
    AND intent becomes executing

Verify the test proves the race rather than merely logging that it exists.

Also verify production documentation does not falsely claim this race is closed.

==================================================
PART III — CLAIM → PROVIDER WINDOW B
==================================================

Review TestExec15B.

Verify it still genuinely demonstrates:

    ClaimIntent
        ↓
    deterministic pause
        ↓
    RevokeTarget
        ↓
    provider invocation

Verify:
- revocation is real
- revocation commits before provider invocation
- provider invocation occurs exactly once
- no sleep/timing hacks
- behavior matches documented v1 limitation

==================================================
PART IV — AMBIGUOUS VS DEFINITIVE FAILURE
==================================================

This is a critical security area.

Verify:

    definitive rejection
        →
    executing → live

while:

    timeout/network/lost response/other ambiguous result
        →
    executing remains executing

Attempt to break this with:
- HTTP 400
- HTTP 401
- HTTP 403
- HTTP 404
- HTTP 408
- HTTP 409
- HTTP 429
- HTTP 500
- HTTP 502
- HTTP 503
- connection reset
- context timeout
- response lost after request acceptance

Determine whether each classification is semantically safe.

In particular, scrutinize whether ALL 4xx responses are truly definitive in the
context of the GitHub workflow_dispatch operation.

If an error means "request definitely not accepted", rollback to live may be safe.

If outcome remains uncertain, it MUST remain executing.

Do not allow a seemingly convenient classification rule to reintroduce duplicate
external execution.

==================================================
PART V — ROLLBACK SAFETY
==================================================

Attempt to cause:

    ambiguous failure → RollbackClaim → live

This must NOT be possible through normal service behavior.

Verify:
- only ProviderRejected reaches RollbackClaim
- ProviderAmbiguous does not
- provider-specific errors cannot accidentally lose their classification
- wrapped errors preserve classification through errors.As / errors.Is as intended

==================================================
PART VI — CRASH RECOVERY
==================================================

Review all crash-state assumptions.

Cases:

1. crash before ClaimIntent
   expected:
       live
       retry allowed

2. crash after ClaimIntent but before provider
   expected:
       executing
       retry forbidden
       reconciliation required

3. crash after provider acceptance but before CompleteIntent
   expected:
       executing
       retry forbidden
       reconciliation required

4. crash after CompleteIntent
   expected:
       executed

Attempt to identify any path where the implementation incorrectly changes:

    executing → live

after an ambiguous provider outcome.

==================================================
PART VII — RETRACTION
==================================================

Verify:

    RetractCascade
        ↓
    cancels live intents

but:

    executing
        ↓
    remains executing

Attempt concurrent:

    execution claim + retraction

Determine whether an executing intent can accidentally be cancelled or otherwise
lose its execution-attempt marker.

Check the database invariants and proof tests.

==================================================
PART VIII — RECONCILIATION SECURITY
==================================================

Inspect ReconcileIntent deeply.

Verify:
- only executing source state is accepted
- completed/cancelled/live intents cannot be reconciled
- exact intentID is bound
- scenario binding is enforced
- transitions are atomic
- operator identity is required
- operator identity is audited
- it is not exposed through generic REST/MCP
- an arbitrary caller cannot simply assert "completed"

Attempt:
- reconcile executed intent
- reconcile cancelled intent
- reconcile live intent
- cross-scenario reconciliation
- wrong intentID
- repeated reconciliation
- unauthorized operator

Most importantly, determine whether reconciliation can become an authority
bypass.

==================================================
PART IX — API / MCP SNAPSHOT SOURCE
==================================================

Inspect:

    api/authorization.go
    cmd/solvent-mcp/tools.go

Verify the snapshot query:

- retrieves the correct immutable approved snapshot
- uses the correct target activation relationship
- cannot be substituted by caller input
- does not silently create an authorization bypass
- still routes the final allow/deny decision through kernel.Authorize

IMPORTANT:
There has previously been a change to make snapshot queries graceful with a
fallback to `{}`.

Determine whether that fallback FAILS CLOSED.

Expected:

    snapshot lookup succeeds
        → use exact snapshot parameters

    snapshot lookup fails
        → authorization must NOT become more permissive

A missing/unavailable snapshot must never cause a valid consequential authority
decision merely because `{}` happens to compare successfully.

This is a high-priority adversarial check.

==================================================
PART X — CONSEQUENCE PARAMETER ATTACK
==================================================

Attempt:
- caller-supplied conflicting repo
- caller-supplied conflicting workflow
- caller-supplied conflicting ref
- caller-supplied extra provider fields
- malformed consequence parameters
- mutated maps
- alternative target IDs

Verify:

    approved snapshot
        →
    authoritative tuple
        →
    kernel.Authorize

and that caller data cannot rewrite the consequential tuple.

==================================================
PART XI — EXECUTOR SELECTION
==================================================

Attempt:
- params["tool_name"]
- executor name
- provider name
- alternate registry key
- malicious executor
- arbitrary action parameters

Verify fixed:

    authorized action → executor

and no caller-controlled capability selector.

==================================================
PART XII — EXACT INTENT BINDING
==================================================

Attempt:
- another intentID
- another scenario
- another belief
- another target
- cancelled intent
- executed intent
- duplicate execution
- fabricated UUID

Verify every state transition binds to the exact intended intent.

Search for:
    SELECT ... LIMIT 1

or equivalent arbitrary intent rediscovery.

==================================================
PART XIII — COMPLETEINTENT
==================================================

Inspect:

    CompleteIntent

Verify:
- exact ID
- exact scenario
- source state `executing`
- no authority mutation
- no belief mutation
- no target mutation
- no public REST/MCP endpoint
- only executed after provider acceptance
- provider output cannot directly cause authority changes

==================================================
PART XIV — HTTP PROVIDER SECURITY
==================================================

Review:

    adapter/github/http_provider.go
    adapter/github/provider.go
    adapter/github/executor.go
    adaptererrors

Inspect:

### Credentials
- construction-time only
- never in params
- never logged
- never audited
- never kernel state

### URL/path construction
- repo
- workflow
- ref
- injection
- traversal
- escaping

### HTTP semantics
- method
- endpoint
- headers
- request body
- response statuses
- malformed responses
- redirects
- timeout
- cancellation

### Error classification
Verify GitHub-specific error semantics are correctly converted to:
    ProviderRejected
    ProviderAmbiguous
    ProviderAccepted where appropriate

### Response leakage
Determine whether provider response bodies can leak sensitive information
through:
- API errors
- audit entries
- logs

Do not demand a large networking framework.

==================================================
PART XV — EXECUTOR ISOLATION
==================================================

Verify the executor cannot reach:

- kernel.Store
- authority mutations
- database
- audit state
- approval/promotion/revocation methods

Attempt to identify dependency inversion or hidden global state that makes such
access possible.

==================================================
PART XVI — AUDIT SEMANTICS
==================================================

Verify execution audit ordering and meaning:

    authorization_granted
    adapter_invoked
    executor_failed
    intent_completion_failed
    executor_completed

Verify:
- provider acceptance is not confused with workflow completion
- ambiguous provider state is not recorded as definitive failure
- audit failure does not rewrite established provider success
- reconciliation is auditable
- no contradictory event sequence is produced

==================================================
PART XVII — KERNEL MINIMALISM
==================================================

Inspect every Phase 4D kernel change.

Expected additions:

    ClaimIntent
    RollbackClaim
    CancelIntent
    updated CompleteIntent semantics

Race A remains accepted; therefore:

    AuthorizeAndClaimIntent

must NOT exist.

Verify the kernel still does NOT know:
- GitHub
- HTTP status semantics
- retry policy
- reconciliation protocol
- provider completion
- provider-specific errors

Determine whether each new primitive is justified as a durable atomic state
transition.

==================================================
PART XVIII — SCHEMA / MIGRATION SAFETY
==================================================

Inspect:

    db/008_executing_state.sql

Verify:
- migration actually changes the correct CHECK constraint
- existing schema is not silently left with an old constraint
- `executing` is allowed
- invalid states remain forbidden
- migration is compatible with the actual existing constraint name
- index is correct and useful
- migration consumers are synchronized
- M0 remains intentionally frozen

Attempt to identify:
- migration ordering issues
- destructive behavior
- partial migration problems
- false idempotency claims

==================================================
PART XIX — PROOF / TEST QUALITY
==================================================

Do not merely count tests.

Inspect whether new tests genuinely prove:

    TestExec39  concurrent duplicate prevention
    TestExec40  sequential duplicate prevention
    TestExec41  ClaimIntent atomicity
    TestExec42  definitive rejection rollback
    TestExec43  ambiguous failure remains executing
    TestExec44  crash recovery
    TestExec45  retraction preserves executing
    TestExec46  snapshot consequence parameters
    TestExec47  HTTP timeout
    TestExec48  no production SetTestHook
    TestExec49  adversarial concurrent race
    TestExec50  retry after executing
    TestExec51  retraction during execution
    TestExec52  parameter substitution
    TestExec53  execute after revocation
    TestExec54  ambiguous-to-live attack
    TestExec55  Authorize→Claim race

For each important test, ask:

    What implementation bug would make this test fail?

If the answer is "none" or "the test would still pass", flag it.

==================================================
PART XX — RACE DETECTOR
==================================================

Run:

    go test -race ./...

Do not rely on previous output.

Pay particular attention to:
- concurrent claim tests
- provider synchronization
- fake provider state
- reconciliation
- shared test database state
- goroutine cleanup
- channel closure
- data races on result variables

==================================================
PART XXI — FULL VERIFICATION
==================================================

Run:

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

Also:

    go test -count=1 ./adapter/github/...
    go test -count=1 ./service/authority/...
    go test -count=1 ./kernel/...
    go test -count=1 ./api/...

Verify the test counts independently.

==================================================
PART XXII — ADVERSARIAL MUTATION ANALYSIS
==================================================

For the important security tests, mentally simulate removal of:

1. ClaimIntent CAS
2. final authority check
3. exact intentID
4. snapshot parameter reconstruction
5. fixed executor mapping
6. ambiguous-state preservation
7. source-state predicate in reconciliation
8. RetractCascade protection for executing intents
9. authentication/attribution in reconciliation
10. provider outcome classification

Determine whether existing tests would detect each regression.

==================================================
FINDING CLASSIFICATION
==================================================

Classify findings:

    CRITICAL
    HIGH
    MEDIUM
    LOW
    INFO

CRITICAL/HIGH/MEDIUM => NO-GO.

LOW/INFO => do not automatically block GO.

For every finding provide:

- ID
- severity
- file
- function/symbol
- exact issue
- exploit/failure scenario
- violated invariant
- whether existing tests catch it
- remediation direction

Do NOT fix findings.

==================================================
IMPORTANT NON-FINDINGS
==================================================

Do NOT automatically report as defects:

- Race A itself, because it is an explicitly accepted v1 residual risk
- Claim→Provider Window B
- concurrent duplicate risk that ClaimIntent actually eliminates
- ambiguous execution requiring reconciliation
- lack of exactly-once external semantics
- lack of provider-side idempotency
- lack of async execution
- lack of retries for ambiguous outcomes
- lack of RBAC/IAM
- lack of multi-tenancy
- lack of additional providers

Only report them if implementation or documentation contradicts the approved
model.

==================================================
FINAL VERDICT
==================================================

Return:

    VERDICT: GO

ONLY if:

    0 CRITICAL
    0 HIGH
    0 MEDIUM

and the implementation actually satisfies the Phase 4D contract.

Otherwise:

    VERDICT: NO-GO

==================================================
FINAL REPORT FORMAT
==================================================

Start with:

    VERDICT: GO
or
    VERDICT: NO-GO

Then provide:

1. Executive assessment

2. Verification results

    go build ./...
    go vet ./...
    go test -count=1 ./...
    go test -race ./...

3. Previous Phase 4C+ boundary verification

4. Phase 4D invariant matrix

    CI-4 through CI-10:
        PASS / FAIL / UNPROVEN

5. Race A analysis

6. Race B / Window B analysis

7. ClaimIntent concurrency analysis

8. Ambiguous vs definitive provider analysis

9. Crash recovery analysis

10. RetractCascade analysis

11. Reconciliation security analysis

12. API/MCP snapshot-source analysis

13. Parameter-binding analysis

14. HTTP provider security analysis

15. CompleteIntent analysis

16. Kernel minimalism / architectural boundary analysis

17. Schema/migration analysis

18. Test-quality and mutation analysis

19. Findings table

20. Remaining accepted limitations

21. Final recommendation

The central question is:

    Does Phase 4D make consequential execution materially safer under
    concurrency and failure without turning Solvent's kernel into a generic
    orchestration system or falsely claiming guarantees it cannot provide?

Try to prove the answer is NO.

Only return GO if you fail.