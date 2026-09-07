PHASE 4E — FINAL INDEPENDENT ADVERSARIAL SECURITY / ARCHITECTURE REVIEW

You are an INDEPENDENT senior security architect performing the final acceptance
review of Solvent Phase 4E.

Phase 4C+ and Phase 4D have already passed independent adversarial review.

Phase 4E is now implemented.

The developer reports that the following are green:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task test
    API adversarial tests
    MCP verification
    formatting / repository checks

Do NOT trust the report.

Verify the actual repository state and rerun the relevant commands yourself.

This is a READ-ONLY review.

DO NOT:
- modify source files
- fix code
- refactor
- add tests
- rewrite architecture
- change API contracts
- weaken acceptance criteria
- create commits

Your job is to TRY TO BREAK PHASE 4E.

==================================================
REPOSITORY
==================================================

Review:

    ~/Documents/go/solvent-main

First inspect:

    git status --short
    git branch --show-current
    git log -1 --oneline

Inspect the actual implementation, tests, OpenAPI specification, MCP tools,
examples, and service wiring.

Do not rely on screenshots or developer summaries.

==================================================
PHASE 4E OBJECTIVE
==================================================

Phase 4E's purpose is to turn the proven:

    authority → intent → execution

architecture into a coherent end-to-end agent workflow.

The canonical flow is:

    evidence / agent reasoning
        ↓
    proposed consequential action
        ↓
    target + frozen parameters
        ↓
    approval
        ↓
    authorization
        ↓
    live intent
        ↓
    execution
        ↓
    ClaimIntent
        ↓
    GitHub provider
        ↓
    CompleteIntent
        ↓
    audit/activity

The central product/security thesis is:

    "The agent can propose. Solvent decides. GitHub executes."

==================================================
LOCKED FOUNDATIONS
==================================================

Do NOT reopen these unless there is a genuine security contradiction:

- kernel.Authorize is the sole authority oracle
- TOKEN != AUTHORITY
- Authorize != Execute
- approved snapshot consequence_parameters are authoritative
- caller execution parameters are non-authoritative
- fixed action → executor mapping
- exact intentID binding
- ClaimIntent is the execution ownership gate
- live → executing → executed
- definitive rejection → live
- ambiguous outcome → executing
- ambiguous outcomes require reconciliation
- executing intents survive RetractCascade
- CompleteIntent only transitions executing → executed
- provider acceptance != workflow completion
- GitHub semantics remain outside kernel
- provider outcome classification is adapter-specific
- ReconcileIntent is internal and privileged
- no public CompleteIntent
- no public generic reconciliation endpoint
- no AuthorizeAndClaimIntent
- no GetSnapshotConsequenceParams
- no second authorization engine
- kernel remains minimal

Accepted v1 races:

    Race A:
        Authorize → ClaimIntent

    Race B:
        ClaimIntent → Provider

Both must remain explicitly documented.

==================================================
PART I — NEW REST EXECUTION BOUNDARY
==================================================

Review:

    POST /v1/authorizations/execute

Trace:

    HTTP request
        ↓
    authentication
        ↓
    principal derivation
        ↓
    request validation
        ↓
    authority.Service.ExecuteAction
        ↓
    kernel.Authorize
        ↓
    ClaimIntent
        ↓
    executor
        ↓
    provider

Verify the endpoint does NOT create a second authorization engine.

### Principal attack

Attempt:

    authenticated principal A
    +
    intent belonging to principal B
    →
    execution

Expected:

    DENIED

Verify:
- principal comes from the existing authentication boundary
- request body contains no trusted principal override
- actor_id/principal_id cannot be substituted
- authenticated identity is the only effective identity

### Context substitution attacks

With a valid intent, change:

    scenario_id
    belief_id
    action
    target_id
    intent_id

Expected:

    DENIED

Verify exact authoritative intent binding remains intact.

### Consequence parameter attack

Attempt to send:

    consequence_parameters
    repo
    workflow
    ref
    provider-specific fields

Expected:

    caller values ignored

Approved snapshot remains the sole execution source.

### Executor attack

Attempt:

    tool_name
    executor
    provider
    alternate registry key

Expected:

    caller cannot select another executor

==================================================
PART II — MCP EXECUTION BOUNDARY
==================================================

Review:

    solvent_execute

Trace the full MCP path.

Verify:
- same service semantics as REST
- no bypass of kernel.Authorize
- no public CompleteIntent
- no public ReconcileIntent
- no caller-controlled executor
- no caller-controlled consequence parameters

### Principal/trust model

Determine exactly where the effective MCP principal comes from.

Do NOT assume the MCP actor field is equivalent to REST authentication.

Verify whether the current trusted-local-stdio model is intentional and correctly
documented.

Attempt:
- actor_id substitution
- scenario substitution
- intent substitution
- cross-principal execution

Determine whether an MCP caller can execute an intent belonging to another
principal.

If MCP intentionally operates as a trusted local administrative surface,
verify that this trust boundary is explicit and consistent with the architecture.

==================================================
PART III — MCP ACTIVITY ACCESS
==================================================

Review:

    solvent_activity

Verify:
- scenario-scoped access
- no cross-scenario disclosure
- same semantics as GET /v1/activity
- no second audit-access authorization model

Attempt:
- omitting scenario
- invalid scenario
- another scenario
- another actor's scenario
- arbitrary activity type filtering

Check whether the MCP tool leaks audit information beyond the intended boundary.

==================================================
PART IV — REST / MCP SEMANTIC EQUIVALENCE
==================================================

The REST and MCP execution interfaces should represent the same underlying
service semantics.

Compare:
- authorization
- execution
- identity
- snapshot parameter handling
- executor selection
- error semantics
- intent binding
- audit behavior

Flag any case where REST and MCP produce materially different security
decisions for equivalent requests.

==================================================
PART V — EXECUTION FLOW
==================================================

Verify the exact production flow:

    kernel.Authorize
        ↓
    approved snapshot params
        ↓
    local preparation
        ↓
    ClaimIntent
        ↓
    adapter_invoked
        ↓
    executor/provider
        ↓
    provider outcome
        ↓
    CompleteIntent / RollbackClaim / remain executing
        ↓
    audit

Verify no API or MCP path bypasses this flow.

==================================================
PART VI — PHASE 4D REGRESSION
==================================================

Verify Phase 4D invariants remain intact:

CI-4:
    Concurrent claims permit only one provider attempt.

CI-5:
    Ambiguous results remain executing and cannot be retried automatically.

CI-6:
    Snapshot parameters remain authoritative.

CI-7:
    Executing intents survive RetractCascade.

CI-8:
    Only definitive rejection returns executing → live.

CI-9:
    Provider outcome classification remains provider-neutral at the service
    boundary.

CI-10:
    Reconciliation transitions remain source-state guarded.

Attempt to break each through both REST and MCP where applicable.

==================================================
PART VII — RECONCILIATION AUDIT
==================================================

Inspect:

    ReconcileIntent

Verify the Phase 4D LOW finding was fixed:

    operator identity
        +
    intent
        +
    scenario
        +
    outcome
        +
    timestamp

must be auditable.

Verify:
- audit is actually written
- correct operator is recorded
- reconciliation cannot silently modify a non-executing intent
- no public REST/MCP route exposes reconciliation
- audit failure does not create contradictory execution semantics

==================================================
PART VIII — AUDIT / ACTIVITY INTEGRITY
==================================================

Verify the event sequence can distinguish:

    authorization_granted
    adapter_invoked
    executor_failed
    intent_completion_failed
    executor_completed
    reconciliation_completed

Verify:
- provider acceptance != workflow completion
- provider failure != audit failure
- CompleteIntent persistence failure != provider failure
- reconciliation is visible
- no contradictory success/failure chain
- sensitive credentials never enter audit

==================================================
PART IX — GITHUB REAL SIDE EFFECT
==================================================

Verify the new public API path actually reaches the existing GitHub executor.

Trace:

    REST/MCP
        ↓
    ExecuteAction
        ↓
    fixed action mapping
        ↓
    github executor
        ↓
    HTTPProvider
        ↓
    workflow_dispatch

The API/MCP layer must not implement its own GitHub execution logic.

Verify:
- provider output is returned meaningfully
- RunID is surfaced when available
- system does not claim workflow completion
- missing GITHUB_TOKEN produces a clear error
- credentials remain construction-time configuration

==================================================
PART X — FLAGSHIP EXAMPLE
==================================================

Review:

    examples/github/

The example should demonstrate the complete lifecycle:

    evidence
    →
    belief
    →
    debt discharge
    →
    promotion
    →
    target
    →
    justification
    →
    request
    →
    approval
    →
    authorization
    →
    execution
    →
    audit

Verify the example is actually runnable.

Do not accept "documentation says runnable."

Check:
- configuration
- environment variables
- API availability
- target/workflow setup
- error handling
- no embedded secrets
- no direct authority bypass

Also verify the example's adversarial demonstrations are truthful.

IMPORTANT:
A caller supplying conflicting repo/workflow/ref should demonstrate:

    snapshot wins

not falsely claim the request itself is denied if the execution proceeds using
the approved snapshot.

==================================================
PART XI — PRODUCTION CONFIGURATION
==================================================

Review configuration validation.

Verify:

    GITHUB_TOKEN present
        →
    provider registered

    GITHUB_TOKEN absent
        →
    provider not registered

Do NOT require GitHub token prefixes such as ghp_ or github_pat_.

Verify:
- no secret logging
- no secret in audit
- no secret in API response
- no secret in execution params

Check startup configuration errors for leakage.

==================================================
PART XII — ERROR HANDLING
==================================================

Review new REST/MCP handlers.

Attempt:
- malformed JSON
- missing required fields
- invalid UUIDs
- nonexistent intent
- nonexistent target
- revoked target
- wrong principal
- wrong scenario
- wrong action
- missing executor
- GitHub provider failure
- ambiguous provider outcome

Verify:
- meaningful HTTP status
- no raw secret leakage
- no stack traces
- no accidental authorization bypass
- consistent REST/MCP semantics

==================================================
PART XIII — AUTHENTICATION BOUNDARY
==================================================

Inspect the existing authentication implementation and how Phase 4E uses it.

Verify the execution endpoint does NOT trust:

    actor_id
    principal_id
    created_by

from the request body as authentication proof.

Attempt identity confusion attacks.

Pay special attention to whether `intent_id` alone could be used as a bearer
capability by an unauthenticated or wrong-principal caller.

==================================================
PART XIV — API CONTRACT
==================================================

Inspect:

    docs/openapi/solvent.yaml

Verify the OpenAPI contract accurately reflects implementation:

- required fields
- response fields
- errors
- authentication expectations
- no fields that imply caller-controlled consequence parameters
- no false execution-completion claims

Check for specification/implementation drift.

==================================================
PART XV — BACKWARD COMPATIBILITY
==================================================

Verify:
- existing endpoints still behave identically
- existing MCP tools remain valid
- no unintended breaking changes
- new execution endpoint is additive
- authority semantics did not change
- kernel/schema remain unchanged

==================================================
PART XVI — TEST QUALITY
==================================================

Do not merely count tests.

Inspect newly added:

    Exec11–Exec19
    REST execution tests
    MCP execution tests
    E2E tests

Determine whether each security test genuinely exercises the boundary it claims
to test.

For each major property ask:

    Would this test fail if the handler were changed to bypass the service?

    Would this test fail if the handler trusted request principal_id?

    Would this test fail if caller consequence parameters were forwarded?

    Would this test fail if caller-selected executor were honored?

    Would this test fail if the wrong intent were accepted?

    Would this test fail if authentication were removed?

Identify tests that only validate HTTP shape or error codes without proving
security semantics.

==================================================
PART XVII — MUTATION ANALYSIS
==================================================

Perform mental/code-level mutation analysis.

At minimum simulate:

1. REST handler passes caller identity instead of authenticated principal.
2. REST handler trusts request consequence_parameters.
3. REST handler bypasses ExecuteAction.
4. MCP handler bypasses ExecuteAction.
5. MCP tool accepts arbitrary executor.
6. Intent context validation is removed.
7. Snapshot parameter lookup is removed.
8. ClaimIntent is bypassed.
9. Revoked authority is accepted.
10. Reconciliation audit is removed.
11. Activity tool returns cross-scenario data.
12. GitHub executor mapping becomes caller-controlled.

Determine whether existing tests would detect each mutation.

==================================================
PART XVIII — CONCURRENCY / RACE
==================================================

Run:

    go test -race ./...

Pay attention to:
- REST concurrent execution
- MCP concurrent execution
- same intent through REST and MCP simultaneously
- fake provider
- audit
- test DB state
- goroutine cleanup

Attempt:

    REST ExecuteAction
    +
    MCP solvent_execute

simultaneously against the same intent.

Expected:

    ClaimIntent allows only one provider attempt.

==================================================
PART XIX — BUILD / TEST / REPOSITORY GATES
==================================================

Independently run:

    go build ./...
    go vet ./...
    go test -count=1 -p 1 ./...
    go test -race -count=1 -p 1 ./...
    task test

Also:

    go test -count=1 -race ./api/...
    go test -count=1 -race ./cmd/solvent-mcp/...
    go test -count=1 -race ./service/authority/...
    go test -count=1 -race ./kernel/...
    go test -count=1 -race ./adapter/github/...

Verify repository checks:

    check_17.sh
    mcp_verify.sh

if present.

Do not accept a green result that was obtained only because tests skipped due
to an unavailable DB.

==================================================
PART XX — ARCHITECTURAL BOUNDARY
==================================================

Verify Phase 4E did NOT introduce:

- a second authority engine
- new kernel primitives
- new schema semantics
- public CompleteIntent
- public ReconcileIntent
- GitHub logic in kernel
- caller-selected executor
- caller-controlled provider parameters
- MCP-only security semantics
- REST-only security semantics
- a new audit system
- generic orchestration infrastructure
- hidden authentication bypass

The intended architecture remains:

    API / MCP
         ↓
    service layer
         ↓
    kernel
         ↓
    executor
         ↓
    provider

==================================================
PART XXI — FINDING CLASSIFICATION
==================================================

Classify findings as:

    CRITICAL
    HIGH
    MEDIUM
    LOW
    INFO

CRITICAL/HIGH/MEDIUM => NO-GO.

LOW/INFO => do not automatically block GO.

For each finding provide:

- ID
- severity
- file
- function/symbol
- exact issue
- attack/failure scenario
- violated invariant
- whether current tests detect it
- remediation direction

Do not modify code.

==================================================
IMPORTANT NON-FINDINGS
==================================================

Do NOT automatically classify these as defects:

- Race A
- Race B / Window B
- ambiguous outcome requiring reconciliation
- lack of exactly-once provider execution
- lack of provider polling
- lack of broad retries
- lack of RBAC/IAM
- lack of multi-tenancy
- lack of A2A
- lack of large UI
- internal ReconcileIntent

These are intentional architectural decisions.

Only report them when implementation contradicts the approved contract.

==================================================
FINAL VERDICT
==================================================

Return:

    VERDICT: GO

ONLY when:

    0 CRITICAL
    0 HIGH
    0 MEDIUM

and the actual implementation satisfies the Phase 4E contract.

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

2. Actual verification results

3. REST execution boundary analysis

4. MCP execution boundary analysis

5. REST/MCP semantic equivalence

6. Authentication/principal analysis

7. Intent/context binding analysis

8. Snapshot parameter analysis

9. Executor-selection analysis

10. Audit/reconciliation analysis

11. GitHub external-side-effect analysis

12. Example/demo analysis

13. OpenAPI contract analysis

14. Phase 4D regression analysis

15. Concurrency/race analysis

16. Test-quality assessment

17. Mutation analysis

18. Architectural-boundary assessment

19. Findings table

20. Remaining accepted limitations

21. Final recommendation

The central question is:

    Can an untrusted agent now reach a real consequential GitHub action through
    REST or MCP while Solvent remains the authoritative decision and execution
    boundary?

Do not approve because the code is clean or tests are green.

Try to make the answer NO.

Only return GO if the implementation survives the attack.