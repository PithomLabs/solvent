PHASE 4E — END-TO-END AGENT WORKFLOW + PRODUCTION HARDENING
PLAN ONLY — DO NOT IMPLEMENT

Phase 4C+ and Phase 4D are COMPLETE and independently passed adversarial review.

Phase 4D final verdict:

    GO

The current Solvent authority/execution architecture is now trusted.

The purpose of Phase 4E is NOT to redesign the kernel.

The purpose is to turn the proven authority → intent → execution architecture
into a coherent, usable, end-to-end agent workflow using the real GitHub
integration, while applying a small amount of production hardening.

==================================================
LOCKED FOUNDATIONS
==================================================

Treat these as fixed unless a genuine security defect is discovered:

- kernel.Authorize is the sole authority oracle
- TOKEN != AUTHORITY
- Authorize != Execute
- approved snapshot consequence_parameters are authoritative
- caller execution parameters do not determine consequential provider params
- fixed authorized-action → executor mapping
- exact intentID binding
- ClaimIntent is the atomic execution-ownership gate
- live → executing → executed
- definitive provider rejection → live
- ambiguous provider outcome → executing
- ambiguous outcomes require reconciliation
- executing intents survive RetractCascade
- CompleteIntent only transitions executing → executed
- provider acceptance does not mean workflow completion
- GitHub semantics remain outside kernel
- provider outcome classification remains adapter-specific
- ReconcileIntent remains internal and privileged
- no public CompleteIntent endpoint
- no public generic reconciliation endpoint
- no AuthorizeAndClaimIntent
- no GetSnapshotConsequenceParams
- no second authorization engine
- kernel remains minimal

Accepted v1 residual races remain:

    Race A:
    Authorize → ClaimIntent

    Race B:
    ClaimIntent → Provider

Do NOT attempt to eliminate these in Phase 4E unless a concrete production
requirement or security incident establishes that the current design is
insufficient.

==================================================
PHASE 4E OBJECTIVE
==================================================

Prove that a real autonomous workflow can use Solvent from proposal through
real external execution without bypassing the authority model.

The canonical experience should become:

    evidence / agent reasoning
        ↓
    propose consequential action
        ↓
    create/identify target
        ↓
    obtain approval
        ↓
    authorize
        ↓
    create exact intent
        ↓
    execute exact intent
        ↓
    GitHub accepts workflow dispatch
        ↓
    Solvent records execution state
        ↓
    audit trail explains what happened

The workflow must be understandable to both:
- an AI agent
- a human operator/developer

==================================================
PRIMARY DELIVERABLE
==================================================

Design one flagship end-to-end GitHub deployment workflow.

Use a safe test repository/workflow.

Do not add multiple providers.

The flagship scenario should demonstrate:

    "AI proposes a deployment, but Solvent—not the AI—decides whether
     the exact consequential action is authorized."

The workflow should show:
- evidence
- proposed action
- approved target
- exact consequence parameters
- authorization
- intent
- execution
- GitHub provider acceptance
- audit/activity
- resulting Solvent state

==================================================
TRACK A — AGENT-FACING API EXPERIENCE
==================================================

Evaluate the current REST and MCP surfaces.

The goal is:

    Any capable agent can understand:
        what it may ask Solvent to do
        what Solvent requires
        what Solvent decided
        what exact action will execute

Review the complete lifecycle:

    target creation
    approval
    authorization
    intent creation
    execution

Determine whether the current contracts are:
- coherent
- discoverable
- minimally verbose
- deterministic
- language-neutral

Do NOT add an SDK yet.

Canonical interface remains:

    HTTP/JSON API
    OpenAPI
    MCP
    A2A where already applicable

SDKs remain convenience layers.

==================================================
TRACK B — MCP WORKFLOW
==================================================

Review the current MCP tool set.

Construct the minimal tool sequence needed for the flagship GitHub workflow.

For each step document:

- tool
- inputs
- authoritative state created/used
- output
- whether action is read-only or consequential
- whether human approval is required
- whether the tool can trigger an external side effect

Verify:
- no tool bypasses kernel authority
- no tool exposes CompleteIntent
- no tool exposes ReconcileIntent
- no arbitrary executor selector
- no caller-controlled provider parameters
- no duplicate authorization engine

The MCP flow should be understandable without reading Solvent source code.

==================================================
TRACK C — REST WORKFLOW
==================================================

Verify the REST equivalents of the flagship workflow.

Confirm that REST and MCP produce semantically equivalent authorization and
execution behavior.

Do not duplicate business/security semantics between REST and MCP handlers.

The underlying service semantics must remain shared.

==================================================
TRACK D — GITHUB EXECUTION EXPERIENCE
==================================================

Use the existing real HTTP GitHub provider.

Do NOT introduce:
- another GitHub SDK
- additional providers
- workflow polling
- asynchronous orchestration
- retry framework

The flagship example should make clear:

    provider accepted
    ≠
    workflow completed

Return and display the GitHub run reference when available.

Do not claim completion unless an actual completion mechanism exists.

==================================================
TRACK E — AUDIT / ACTIVITY EXPERIENCE
==================================================

Review the execution audit trail from the perspective of a human operator.

A person should be able to answer:

    What action was proposed?
    Who/what was the actor?
    What target was approved?
    What exact parameters were frozen?
    Was authorization granted?
    Was execution claimed?
    Was the provider invoked?
    Did the provider accept?
    Was the intent completed?
    Was there an ambiguity?
    Was reconciliation performed?
    Who reconciled it?

The audit trail must distinguish:

    authority decision
    execution claim
    provider invocation
    provider outcome
    intent completion
    reconciliation

Do not introduce a second event system.

==================================================
TRACK F — RECONCILIATION HARDENING
==================================================

The Phase 4D review left one LOW issue:

    ReconcileIntent does not audit operatorID.

Plan the minimal fix.

The design should record:
- operator identity
- intentID
- scenarioID
- reconciliation outcome
- timestamp
- relevant external/provider reference if available

Keep reconciliation internal in this phase unless there is a compelling reason
to expose it.

Do NOT introduce an admin platform.

==================================================
TRACK G — HTTP PROVIDER HARDENING
==================================================

Review the current timeout/error-handling work from Phase 4D.

Confirm:
- explicit timeout
- context cancellation
- bounded error bodies
- no credential leakage
- correct provider outcome classification

Do not expand the provider beyond the GitHub workflow_dispatch use case.

==================================================
TRACK H — DEVELOPER EXPERIENCE
==================================================

The end-to-end example should be runnable by a technically competent developer
with minimal interpretation.

Design:

    examples/github/...

so that it demonstrates the complete flow.

The example must make configuration explicit without embedding secrets.

Document:
- required environment variables
- required GitHub repository/workflow setup
- what the workflow does
- what Solvent protects
- expected success/failure behavior

Do not create a large tutorial site.

==================================================
TRACK I — SECURITY DEMONSTRATION
==================================================

The flagship workflow should contain at least three explicit adversarial
demonstrations using the same architecture:

1. Caller tries to change the approved repo/workflow/ref.
   Expected:
       snapshot wins.

2. Caller tries to select another executor.
   Expected:
       fixed action→executor mapping wins.

3. Authority is revoked before execution.
   Expected:
       authorization denies.

Use the existing tests and service path.

Do NOT create special demo-only security mechanisms.

==================================================
TRACK J — UI DECISION
==================================================

Do NOT automatically build a UI.

First determine whether the API/MCP workflow is already understandable enough.

If a UI is proposed, it must be:
- an operational/reference client
- non-authoritative
- replaceable
- unable to bypass service semantics

Prefer no UI expansion unless it materially improves the flagship demonstration.

==================================================
TRACK K — A2A
==================================================

Evaluate whether Phase 4E should expose the flagship workflow through A2A.

Do not implement A2A merely for completeness.

The question is whether an agent-to-agent protocol provides a concrete benefit
to the current product thesis.

If not, explicitly defer it.

==================================================
TRACK L — DEMO SCRIPT
==================================================

Design one polished end-to-end demonstration.

Suggested narrative:

    1. Agent gathers evidence.
    2. Agent proposes deployment.
    3. Solvent shows exact target/action/parameters.
    4. Human approval is obtained.
    5. Authorization succeeds.
    6. Intent is created.
    7. Execution is attempted.
    8. GitHub accepts workflow dispatch.
    9. Solvent records the execution.
   10. Audit shows the complete chain.
   11. Attempt a parameter substitution attack.
   12. Attempt an unauthorized execution.
   13. Show both being denied.

The demo should make the central thesis obvious:

    "The agent can propose.
     Solvent decides.
     GitHub executes."

==================================================
TRACK M — TEST STRATEGY
==================================================

Design an end-to-end acceptance suite.

At minimum include:

- REST happy path
- MCP happy path
- REST/MCP semantic equivalence
- real GitHub provider integration test where credentials are available
- snapshot parameter binding
- unauthorized execution
- wrong target
- revoked authority
- malicious executor selector
- audit completeness
- reconciliation audit
- provider acceptance vs workflow completion

Also retain all Phase 4C+/4D regression tests.

Do not delete strong existing security tests.

==================================================
TRACK N — PRODUCTION HARDENING
==================================================

Evaluate:
- configuration validation
- credential handling
- logging
- audit safety
- error messages
- timeout behavior
- graceful missing-GITHUB_TOKEN behavior
- startup behavior
- malformed provider responses
- operational diagnostics

Do not expand this into an enterprise deployment system.

==================================================
NON-GOALS
==================================================

Do NOT add:

- additional providers
- Kubernetes
- AWS
- generic workflow orchestration
- distributed transactions
- exactly-once external execution
- broad retry mechanisms
- provider polling
- multi-tenancy
- RBAC/IAM
- compliance platform
- large UI
- SDK ecosystem
- agent memory system
- generic event sourcing
- kernel redesign

==================================================
ARCHITECTURAL DECISION RULE
==================================================

Before adding any new primitive, ask:

    Is this a durable security fact?
    Is this an atomic state transition?
    Is it impossible to express safely above the kernel?

If not:

    keep it above the kernel.

==================================================
DELIVERABLE
==================================================

DO NOT MODIFY CODE YET.

Produce:

1. Phase 4E objective

2. Current end-to-end workflow assessment

3. Flagship GitHub workflow

4. REST lifecycle

5. MCP lifecycle

6. Proposed service/API contract changes

7. Audit/activity improvements

8. Reconciliation audit fix

9. GitHub integration experience

10. Security demonstration

11. Developer example

12. End-to-end acceptance tests

13. Adversarial tests

14. Production-hardening items

15. UI decision

16. A2A decision

17. Demo script

18. Migration/backward-compatibility considerations

19. Explicit non-goals

20. Risk analysis

21. Exact files expected to change

22. Exact files that must NOT change

23. Implementation order

24. Definition of done

==================================================
IMPORTANT
==================================================

Phase 4D is complete.

Do not reopen Phase 4D architecture.

The purpose of this plan is to turn the proven security kernel/execution model
into a compelling, usable product workflow.

STOP after producing the plan.